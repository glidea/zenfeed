// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package scraper

import (
	"context"
	"strconv"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/pkg/errors"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/storage/feed"
	"github.com/glidea/zenfeed/pkg/storage/kv"
	"github.com/glidea/zenfeed/pkg/telemetry"
	"github.com/glidea/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	hashutil "github.com/glidea/zenfeed/pkg/util/hash"
	"github.com/glidea/zenfeed/pkg/util/retry"
	timeutil "github.com/glidea/zenfeed/pkg/util/time"
)

var clk = clock.New()

// --- Interface code block ---
type Scraper interface {
	component.Component
	Config() *Config
}

type Config struct {
	Past     time.Duration
	Interval time.Duration
	Name     string
	Labels   model.Labels
	RSS      *ScrapeSourceRSS
}

const maxPast = 15 * 24 * time.Hour

func (c *Config) Validate() error {
	if c.Past <= 0 {
		c.Past = timeutil.Day
	}
	if c.Past > maxPast {
		c.Past = maxPast
	}
	if c.Interval <= 0 {
		c.Interval = time.Hour
	}
	if c.Interval < 10*time.Minute {
		c.Interval = 10 * time.Minute
	}
	if c.Name == "" {
		return errors.New("name cannot be empty")
	}
	if c.RSS != nil {
		if err := c.RSS.Validate(); err != nil {
			return errors.Wrap(err, "invalid RSS config")
		}
	}

	return nil
}

type Dependencies struct {
	FeedStorage feed.Storage
	KVStorage   kv.Storage
}

// --- Factory code block ---
type Factory component.Factory[Scraper, Config, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Scraper, Config, Dependencies](
			func(instance string, config *Config, dependencies Dependencies) (Scraper, error) {
				m := &mockScraper{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Scraper, Config, Dependencies](new)
}

func new(instance string, config *Config, dependencies Dependencies) (Scraper, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid scraper config")
	}

	source, err := newReader(config)
	if err != nil {
		return nil, errors.Wrap(err, "creating source")
	}

	return &scraper{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "Scraper",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		source: source,
	}, nil
}

// --- Implementation code block ---

type scraper struct {
	*component.Base[Config, Dependencies]

	source reader
}

func (s *scraper) Run() (err error) {
	ctx := telemetry.StartWith(s.Context(), append(s.TelemetryLabels(), telemetrymodel.KeyOperation, "Run")...)
	defer func() { telemetry.End(ctx, err) }()

	// Add random offset to avoid synchronized scraping.
	offset := timeutil.Random(time.Minute)
	log.Debug(ctx, "computed scrape offset", "offset", offset)

	timer := time.NewTimer(offset)
	defer timer.Stop()
	s.MarkReady()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.scrapeUntilSuccess(ctx)
			timer.Reset(s.Config().Interval)
		}
	}
}

func (s *scraper) scrapeUntilSuccess(ctx context.Context) {
	_ = retry.Backoff(ctx, func() (err error) {
		opCtx := telemetry.StartWith(ctx, append(s.TelemetryLabels(), telemetrymodel.KeyOperation, "scrape")...)
		defer func() { telemetry.End(opCtx, err) }()
		timeout := 20 * time.Minute // For llm rewrite, it may take a long time.
		opCtx, cancel := context.WithTimeout(opCtx, timeout)
		defer cancel()

		// Read feeds from source.
		feeds, err := s.source.Read(opCtx)
		if err != nil {
			return errors.Wrap(err, "reading source feeds")
		}
		log.Debug(opCtx, "reading source feeds success", "count", len(feeds))

		// Process feeds.
		processed := s.processFeeds(ctx, feeds)
		log.Debug(opCtx, "processed feeds", "count", len(processed))
		if len(processed) == 0 {
			return nil
		}

		// Save processed feeds.
		if err := s.Dependencies().FeedStorage.Append(opCtx, processed...); err != nil {
			return errors.Wrap(err, "saving feeds")
		}
		log.Debug(opCtx, "appending feeds success")

		return nil
	}, &retry.Options{
		MinInterval: time.Minute,
		MaxInterval: 16 * time.Minute,
		MaxAttempts: retry.InfAttempts,
	})
}

func (s *scraper) processFeeds(ctx context.Context, feeds []*model.Feed) []*model.Feed {
	feeds = s.filterPasted(feeds)
	feeds = s.addAdditionalMetaLabels(feeds)
	feeds = s.fillIDs(feeds)
	feeds = s.filterExists(ctx, feeds)

	return feeds
}

func (s *scraper) filterPasted(feeds []*model.Feed) (filtered []*model.Feed) {
	now := clk.Now()
	for _, feed := range feeds {
		t := timeutil.MustParse(feed.Labels.Get(model.LabelPubTime))
		if timeutil.InRange(t, now.Add(-s.Config().Past), now) {
			filtered = append(filtered, feed)
		}
	}

	return filtered
}

func (s *scraper) fillIDs(feeds []*model.Feed) []*model.Feed {
	for _, feed := range feeds {
		// We can not use the pub time to join the hash,
		// because the pub time is dynamic for some sources.
		//
		// title may be changed for some sources... so...
		source := feed.Labels.Get(model.LabelSource)
		link := feed.Labels.Get(model.LabelLink)
		feed.ID = hashutil.Sum64s([]string{source, link})
	}

	return feeds
}

const (
	keyPrefix = "scraper.feed.try-append."
	ttl       = maxPast + time.Minute // Ensure the key is always available util the feed is pasted.
)

func (s *scraper) filterExists(ctx context.Context, feeds []*model.Feed) (filtered []*model.Feed) {
	appendToResult := func(feed *model.Feed) {
		key := keyPrefix + strconv.FormatUint(feed.ID, 10)
		value := timeutil.Format(feed.Time)
		if err := s.Dependencies().KVStorage.Set(ctx, []byte(key), []byte(value), ttl); err != nil {
			log.Error(ctx, err, "set last try store time")
		}
		filtered = append(filtered, feed)
	}

	for _, feed := range feeds {
		key := keyPrefix + strconv.FormatUint(feed.ID, 10)

		lastTryStored, err := s.Dependencies().KVStorage.Get(ctx, []byte(key))
		switch {
		default:
			log.Error(ctx, err, "get last stored time, fallback to continue writing")
			appendToResult(feed)

		case errors.Is(err, kv.ErrNotFound):
			appendToResult(feed)

		case err == nil:
			t, err := timeutil.Parse(string(lastTryStored))
			if err != nil {
				log.Error(ctx, err, "parse last try stored time, fallback to continue writing")
				appendToResult(feed)
			}

			exists, err := s.Dependencies().FeedStorage.Exists(ctx, feed.ID, t)
			if err != nil {
				log.Error(ctx, err, "check feed exists, fallback to continue writing")
				appendToResult(feed)
			}
			if !exists {
				appendToResult(feed)
			}
		}
	}

	return filtered
}

func (s *scraper) addAdditionalMetaLabels(feeds []*model.Feed) []*model.Feed {
	for _, feed := range feeds {
		feed.Labels = append(
			feed.Labels,
			append(s.Config().Labels, model.Label{Key: model.LabelSource, Value: s.Config().Name})...,
		)
		feed.Labels.EnsureSorted()
	}

	return feeds
}

type mockScraper struct {
	component.Mock
}

func (s *mockScraper) Config() *Config {
	args := s.Called()

	return args.Get(0).(*Config)
}
