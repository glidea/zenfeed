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

package scrape

import (
	"reflect"
	"time"

	"github.com/pkg/errors"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/config"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/scrape/scraper"
	"github.com/glidea/zenfeed/pkg/storage/feed"
	"github.com/glidea/zenfeed/pkg/storage/kv"
	"github.com/glidea/zenfeed/pkg/telemetry"
	"github.com/glidea/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
)

// --- Interface code block ---
type Manager interface {
	component.Component
	config.Watcher
}

type Config struct {
	Scrapers []scraper.Config
}

func (c *Config) Validate() error {
	nameUnique := make(map[string]struct{})
	for i := range c.Scrapers {
		scraperCfg := &c.Scrapers[i]
		if _, exists := nameUnique[scraperCfg.Name]; exists {
			return errors.New("scraper name must be unique")
		}
		nameUnique[scraperCfg.Name] = struct{}{}
	}

	for i := range c.Scrapers {
		scraperCfg := &c.Scrapers[i]
		if err := scraperCfg.Validate(); err != nil {
			return errors.Wrapf(err, "invalid scraper %s", scraperCfg.Name)
		}
	}

	return nil
}

func (c *Config) From(app *config.App) {
	c.Scrapers = make([]scraper.Config, len(app.Scrape.Sources))
	for i := range app.Scrape.Sources {
		c.Scrapers[i] = scraper.Config{
			Past:     time.Duration(app.Scrape.Past),
			Name:     app.Scrape.Sources[i].Name,
			Interval: time.Duration(app.Scrape.Sources[i].Interval),
			Labels:   model.Labels{},
		}
		c.Scrapers[i].Labels.FromMap(app.Scrape.Sources[i].Labels)
		if c.Scrapers[i].Interval <= 0 {
			c.Scrapers[i].Interval = time.Duration(app.Scrape.Interval)
		}
		if app.Scrape.Sources[i].RSS != nil {
			c.Scrapers[i].RSS = &scraper.ScrapeSourceRSS{
				URL:             app.Scrape.Sources[i].RSS.URL,
				RSSHubEndpoint:  app.Scrape.RSSHubEndpoint,
				RSSHubRoutePath: app.Scrape.Sources[i].RSS.RSSHubRoutePath,
			}
		}
	}
}

type Dependencies struct {
	ScraperFactory scraper.Factory
	FeedStorage    feed.Storage
	KVStorage      kv.Storage
}

// --- Factory code block ---
type Factory component.Factory[Manager, config.App, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Manager, config.App, Dependencies](
			func(instance string, app *config.App, dependencies Dependencies) (Manager, error) {
				m := &mockManager{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Manager, config.App, Dependencies](new)
}

func new(instance string, app *config.App, dependencies Dependencies) (Manager, error) {
	config := &Config{}
	config.From(app)
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid configuration")
	}

	m := &manager{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "ScrapeManager",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		scrapers: make(map[string]scraper.Scraper, len(config.Scrapers)),
	}

	for i := range config.Scrapers {
		c := &config.Scrapers[i]
		s, err := m.newScraper(c)
		if err != nil {
			return nil, errors.Wrapf(err, "creating scraper %s", c.Name)
		}
		m.scrapers[c.Name] = s
	}

	return m, nil
}

// --- Implementation code block ---
type manager struct {
	*component.Base[Config, Dependencies]

	scrapers map[string]scraper.Scraper
}

func (m *manager) Run() (err error) {
	ctx := telemetry.StartWith(m.Context(), append(m.TelemetryLabels(), telemetrymodel.KeyOperation, "Run")...)
	defer func() { telemetry.End(ctx, err) }()

	for _, s := range m.scrapers {
		if err := component.RunUntilReady(ctx, s, 10*time.Second); err != nil {
			return errors.Wrapf(err, "running scraper %s", s.Config().Name)
		}
	}

	m.MarkReady()
	<-ctx.Done()

	return nil
}

func (m *manager) Reload(app *config.App) error {
	newConfig := &Config{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "invalid configuration")
	}
	if reflect.DeepEqual(m.Config(), newConfig) {
		log.Debug(m.Context(), "no changes in scrape config")

		return nil
	}

	return m.reload(newConfig)
}

func (m *manager) Close() error {
	if err := m.Base.Close(); err != nil {
		return errors.Wrap(err, "closing base")
	}

	return m.stopAllScrapers()
}

func (m *manager) newScraper(c *scraper.Config) (scraper.Scraper, error) {
	return m.Dependencies().ScraperFactory.New(
		c.Name,
		c,
		scraper.Dependencies{
			FeedStorage: m.Dependencies().FeedStorage,
			KVStorage:   m.Dependencies().KVStorage,
		},
	)
}

func (m *manager) reload(config *Config) (err error) {
	ctx := telemetry.StartWith(m.Context(), append(m.TelemetryLabels(), telemetrymodel.KeyOperation, "reload")...)
	defer func() { telemetry.End(ctx, err) }()

	newScrapers := make(map[string]scraper.Scraper, len(m.scrapers))
	if err := m.runOrRestartScrapers(config, newScrapers); err != nil {
		return errors.Wrap(err, "run or restart RSS scrapers")
	}
	if err := m.stopObsoleteScrapers(newScrapers); err != nil {
		return errors.Wrap(err, "stop obsolete scrapers")
	}

	m.scrapers = newScrapers
	m.SetConfig(config)

	return nil
}

func (m *manager) runOrRestartScrapers(config *Config, newScrapers map[string]scraper.Scraper) error {
	for i := range config.Scrapers {
		c := &config.Scrapers[i]
		if err := c.Validate(); err != nil {
			return errors.Wrapf(err, "validate scraper %s", c.Name)
		}

		if err := m.runOrRestartScraper(c, newScrapers); err != nil {
			return errors.Wrapf(err, "run or restart scraper %s", c.Name)
		}
	}

	return nil
}

func (m *manager) runOrRestartScraper(c *scraper.Config, newScrapers map[string]scraper.Scraper) error {
	if existing, exists := m.scrapers[c.Name]; exists {
		if reflect.DeepEqual(existing.Config(), c) {
			newScrapers[c.Name] = existing

			// No changed.
			return nil
		}

		// Config updated.
		if err := existing.Close(); err != nil {
			return errors.Wrapf(err, "closing")
		}
	}

	// Recreate & Run.
	if _, exists := newScrapers[c.Name]; !exists {
		s, err := m.newScraper(c)
		if err != nil {
			return errors.Wrap(err, "creating")
		}
		newScrapers[c.Name] = s
		if err := component.RunUntilReady(m.Context(), s, 10*time.Second); err != nil {
			return errors.Wrap(err, "running")
		}
	}

	return nil
}

func (m *manager) stopObsoleteScrapers(newScrapers map[string]scraper.Scraper) error {
	for id, old := range m.scrapers {
		if _, exists := newScrapers[id]; !exists {
			if err := old.Close(); err != nil {
				return errors.Wrapf(err, "closing scraper %s", id)
			}
		}
	}

	return nil
}

func (m *manager) stopAllScrapers() error {
	for _, s := range m.scrapers {
		if err := s.Close(); err != nil {
			return errors.Wrapf(err, "closing scraper %s", s.Config().Name)
		}
	}

	return nil
}

type mockManager struct {
	component.Mock
}

func (m *mockManager) Reload(config *config.App) error {
	args := m.Called(config)

	return args.Error(0)
}
