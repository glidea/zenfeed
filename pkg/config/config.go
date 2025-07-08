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

package config

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/telemetry"
	"github.com/glidea/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	timeutil "github.com/glidea/zenfeed/pkg/util/time"
)

// --- Interface code block ---
type Manager interface {
	component.Component
	AppConfig() *App
	SaveAppConfig(app *App) error
	Subscribe(w Watcher)
}

type Config struct {
	Path string
}

type App struct {
	Timezone  string `yaml:"timezone,omitempty" json:"timezone,omitempty" desc:"The timezone of the app. e.g. Asia/Shanghai. Default: server's local timezone"`
	Telemetry struct {
		Address string `yaml:"address,omitempty" json:"address,omitempty" desc:"The address ([host]:port) of the telemetry server. e.g. 0.0.0.0:9090. Default: :9090. It can not be changed after the app is running."`
		Log     struct {
			Level string `yaml:"level,omitempty" json:"level,omitempty" desc:"Log level, one of debug, info, warn, error. Default: info"`
		} `yaml:"log,omitempty" json:"log,omitempty" desc:"The log config."`
	} `yaml:"telemetry,omitempty" json:"telemetry,omitempty" desc:"The telemetry config."`
	API struct {
		HTTP struct {
			Address string `yaml:"address,omitempty" json:"address,omitempty" desc:"The address ([host]:port) of the HTTP API. e.g. 0.0.0.0:1300. Default: :1300. It can not be changed after the app is running."`
		} `yaml:"http,omitempty" json:"http,omitempty" desc:"The HTTP API config."`
		MCP struct {
			Address string `yaml:"address,omitempty" json:"address,omitempty" desc:"The address ([host]:port) of the MCP API. e.g. 0.0.0.0:1300. Default: :1301. It can not be changed after the app is running."`
		} `yaml:"mcp,omitempty" json:"mcp,omitempty" desc:"The MCP API config."`
		RSS struct {
			Address             string `yaml:"address,omitempty" json:"address,omitempty" desc:"The address ([host]:port) of the RSS API. e.g. 0.0.0.0:1300. Default: :1302. It can not be changed after the app is running."`
			ContentHTMLTemplate string `yaml:"content_html_template,omitempty" json:"content_html_template,omitempty" desc:"The template to render the RSS content for each item. Default is {{ .summary_html_snippet }}."`
		} `yaml:"rss,omitempty" json:"rss,omitempty" desc:"The RSS config."`
		LLM string `yaml:"llm,omitempty" json:"llm,omitempty" desc:"The LLM name for summarizing feeds. e.g. my-favorite-gemini-king. Default is the default LLM in llms section."`
	} `yaml:"api,omitempty" json:"api,omitempty" desc:"The API config."`
	LLMs []LLM `yaml:"llms,omitempty" json:"llms,omitempty" desc:"The LLMs config. It is required, at least one LLM is needed, refered by other config sections."`
	Jina struct {
		Token string `yaml:"token,omitempty" json:"token,omitempty" desc:"The token of the Jina server."`
	} `yaml:"jina,omitempty" json:"jina,omitempty" desc:"The Jina config."`
	Scrape   Scrape  `yaml:"scrape,omitempty" json:"scrape,omitempty" desc:"The scrape config."`
	Storage  Storage `yaml:"storage,omitempty" json:"storage,omitempty" desc:"The storage config."`
	Scheduls struct {
		Rules []SchedulsRule `yaml:"rules,omitempty" json:"rules,omitempty" desc:"The rules for scheduling feeds. Each rule may query out multiple feeds as a 'Result', the result will be sended to the notify route, finally to notify receivers."`
	} `yaml:"scheduls,omitempty" json:"scheduls,omitempty" desc:"The scheduls config for monitoring feeds. Aka monitoring rules."`
	Notify struct {
		Route     NotifyRoute      `yaml:"route,omitempty" json:"route,omitempty" desc:"The notify route config. It is required."`
		Receivers []NotifyReceiver `yaml:"receivers,omitempty" json:"receivers,omitempty" desc:"The notify receivers config. It is required, at least one receiver is needed."`
		Channels  NotifyChannels   `yaml:"channels,omitempty" json:"channels,omitempty" desc:"The notify channels config. e.g. email"`
	} `yaml:"notify,omitempty" json:"notify,omitempty" desc:"The notify config. It will receive the results from the scheduls module, and group them by the notify route config, and send to the notify receivers via the notify channels."`
}

type LLM struct {
	Name           string  `yaml:"name,omitempty" json:"name,omitempty" desc:"The name (or call it 'id') of the LLM. e.g. my-favorite-gemini-king. It is required when api.llm is set."`
	Default        bool    `yaml:"default,omitempty" json:"default,omitempty" desc:"Whether this LLM is the default LLM. Only one LLM can be the default."`
	Provider       string  `yaml:"provider,omitempty" json:"provider,omitempty" desc:"The provider of the LLM, one of openai, openrouter, deepseek, gemini, volc, siliconflow. e.g. openai"`
	Endpoint       string  `yaml:"endpoint,omitempty" json:"endpoint,omitempty" desc:"The custom endpoint of the LLM. e.g. https://api.openai.com/v1"`
	APIKey         string  `yaml:"api_key,omitempty" json:"api_key,omitempty" desc:"The API key of the LLM. It is required when api.llm is set."`
	Model          string  `yaml:"model,omitempty" json:"model,omitempty" desc:"The model of the LLM. e.g. gpt-4o-mini. Can not be empty with embedding_model at same time when api.llm is set."`
	EmbeddingModel string  `yaml:"embedding_model,omitempty" json:"embedding_model,omitempty" desc:"The embedding model of the LLM. e.g. text-embedding-3-small. Can not be empty with model at same time when api.llm is set. NOTE: Once used, do not modify it directly, instead, add a new LLM configuration."`
	TTSModel       string  `yaml:"tts_model,omitempty" json:"tts_model,omitempty" desc:"The TTS model of the LLM."`
	Temperature    float32 `yaml:"temperature,omitempty" json:"temperature,omitempty" desc:"The temperature (0-2) of the LLM. Default: 0.0"`
}

type Scrape struct {
	Past           timeutil.Duration `yaml:"past,omitempty" json:"past,omitempty" desc:"The lookback time window for scraping feeds. e.g. 1h means only scrape feeds in the past 1 hour. Default: 3d"`
	Interval       timeutil.Duration `yaml:"interval,omitempty" json:"interval,omitempty" desc:"How often to scrape each source, it is a global interval. e.g. 1h. Default: 1h"`
	RSSHubEndpoint string            `yaml:"rsshub_endpoint,omitempty" json:"rsshub_endpoint,omitempty" desc:"The endpoint of the RSSHub. You can deploy your own RSSHub server or use the public one (https://docs.rsshub.app/guide/instances). e.g. https://rsshub.app. It is required when sources[].rss.rsshub_route_path is set."`
	Sources        []ScrapeSource    `yaml:"sources,omitempty" json:"sources,omitempty" desc:"The sources for scraping feeds."`
}

type Storage struct {
	Dir    string        `yaml:"dir,omitempty" json:"dir,omitempty" desc:"The base directory of the all storages. Default: ./data. It can not be changed after the app is running."`
	Feed   FeedStorage   `yaml:"feed,omitempty" json:"feed,omitempty" desc:"The feed storage config."`
	Object ObjectStorage `yaml:"object,omitempty" json:"object,omitempty" desc:"The object storage config."`
}

type FeedStorage struct {
	Rewrites      []RewriteRule     `yaml:"rewrites,omitempty" json:"rewrites,omitempty" desc:"How to process each feed before storing it. It inspired by Prometheus relabeling (https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config), this implements a very strong flexibility and loose coupling."`
	FlushInterval timeutil.Duration `yaml:"flush_interval,omitempty" json:"flush_interval,omitempty" desc:"How often to flush the feed storage to the database, higher value will cause high data loss risk, but on the other hand, it will reduce the number of disk operations and improve performance. Default: 200ms"`
	EmbeddingLLM  string            `yaml:"embedding_llm,omitempty" json:"embedding_llm,omitempty" desc:"The embedding LLM for the feed storage. It will significantly affect the accuracy of semantic search, please be careful to choose. If you want to switch, please note to keep the old llm configuration, because the past data is still implicitly associated with it, otherwise it will cause the past data to be unable to be semantically searched. Default is the default LLM in llms section."`
	Retention     timeutil.Duration `yaml:"retention,omitempty" json:"retention,omitempty" desc:"How long to keep a feed. Default: 8d"`
	BlockDuration timeutil.Duration `yaml:"block_duration,omitempty" json:"block_duration,omitempty" desc:"How long to keep the feed storage block. Block is time-based, like Prometheus TSDB Block. Default: 25h"`
}

type ObjectStorage struct {
	Endpoint        string `yaml:"endpoint,omitempty" json:"endpoint,omitempty" desc:"The endpoint of the object storage."`
	AccessKeyID     string `yaml:"access_key_id,omitempty" json:"access_key_id,omitempty" desc:"The access key id of the object storage."`
	SecretAccessKey string `yaml:"secret_access_key,omitempty" json:"secret_access_key,omitempty" desc:"The secret access key of the object storage."`
	Bucket          string `yaml:"bucket,omitempty" json:"bucket,omitempty" desc:"The bucket of the object storage."`
	BucketURL       string `yaml:"bucket_url,omitempty" json:"bucket_url,omitempty" desc:"The public URL of the object storage bucket."`
}

type ScrapeSource struct {
	Interval timeutil.Duration `yaml:"interval,omitempty" json:"interval,omitempty" desc:"How often to scrape this source. Default: global interval"`
	Name     string            `yaml:"name,omitempty" json:"name,omitempty" desc:"The name of the source. It is required."`
	Labels   map[string]string `yaml:"labels,omitempty" json:"labels,omitempty" desc:"The additional labels to add to the feed of this source."`
	RSS      *ScrapeSourceRSS  `yaml:"rss,omitempty" json:"rss,omitempty" desc:"The RSS config of the source."`
}

type ScrapeSourceRSS struct {
	URL             string `yaml:"url,omitempty" json:"url,omitempty" desc:"The URL of the RSS feed. e.g. http://localhost:1200/github/trending/daily/any. You can not set it when rsshub_route_path is set."`
	RSSHubRoutePath string `yaml:"rsshub_route_path,omitempty" json:"rsshub_route_path,omitempty" desc:"The RSSHub route path of the RSS feed. e.g. github/trending/daily/any. It will be joined with the rsshub_endpoint as the final URL."`
}

type RewriteRule struct {
	If                    []string              `yaml:"if,omitempty" json:"if,omitempty" desc:"The condition config to match the feed. If not set, that means match all feeds. Like label filters, e.g. [source=github, title!=xxx]"`
	SourceLabel           string                `yaml:"source_label,omitempty" json:"source_label,omitempty" desc:"The feed label of the source text to transform. Default is the 'content' label. The feed is essentially a label set (similar to Prometheus metric data). The default labels are type (rss, email (in future), etc), source (the source name), title (feed title), link (feed link), pub_time (feed publish time), and content (feed content)."`
	SkipTooShortThreshold *int                  `yaml:"skip_too_short_threshold,omitempty" json:"skip_too_short_threshold,omitempty" desc:"The threshold of the source text length to skip. Default is 300. It helps we to filter out some short feeds."`
	Transform             *RewriteRuleTransform `yaml:"transform,omitempty" json:"transform,omitempty" desc:"The transform config to transform the source text. If not set, that means transform nothing, so the source text is the transformed text."`
	Match                 string                `yaml:"match,omitempty" json:"match,omitempty" desc:"The match config to match the transformed text (if transform is not set, then source text is the transformed text). It can not be set with match_re at same time."`
	MatchRE               string                `yaml:"match_re,omitempty" json:"match_re,omitempty" desc:"The match regular expression config to match the transformed text. Default is .*"`
	Action                string                `yaml:"action,omitempty" json:"action,omitempty" desc:"The action config to perform when matched. It can be one of create_or_update_label, drop_feed. Default is create_or_update_label."`
	Label                 string                `yaml:"label,omitempty" json:"label,omitempty" desc:"The feed label to add to the transformed text. Only effective when action is create_or_update_label. It is required when action is create_or_update_label."`
}

type RewriteRuleTransform struct {
	ToText    *RewriteRuleTransformToText    `yaml:"to_text,omitempty" json:"to_text,omitempty" desc:"The transform config to transform the source text to text."`
	ToPodcast *RewriteRuleTransformToPodcast `yaml:"to_podcast,omitempty" json:"to_podcast,omitempty" desc:"The transform config to transform the source text to podcast."`
}

type RewriteRuleTransformToText struct {
	Type   string `yaml:"type,omitempty" json:"type,omitempty" desc:"The type of the transform. It can be one of prompt, crawl, crawl_by_jina. Default is prompt. For crawl, the source text will be as the url to crawl the page, and the page will be converted to markdown. crawl vs crawl_by_jina: crawl is local, more stable; crawl_by_jina is powered by https://jina.ai, more powerful."`
	LLM    string `yaml:"llm,omitempty" json:"llm,omitempty" desc:"The LLM name to use. Default is the default LLM in llms section."`
	Prompt string `yaml:"prompt,omitempty" json:"prompt,omitempty" desc:"The prompt to transform the source text. The source text will be injected into the prompt above. And you can use go template syntax to refer some built-in prompts, like {{ .summary }}. Available built-in prompts: category, tags, score, comment_confucius, summary, summary_html_snippet."`
}

type RewriteRuleTransformToPodcast struct {
	LLM                        string                                 `yaml:"llm,omitempty" json:"llm,omitempty" desc:"The LLM name to use. Default is the default LLM in llms section."`
	EstimateMaximumDuration    timeutil.Duration                      `yaml:"estimate_maximum_duration,omitempty" json:"estimate_maximum_duration,omitempty" desc:"The estimated maximum duration of the podcast. It will affect the length of the generated transcript. e.g. 5m. Default is 5m."`
	TranscriptAdditionalPrompt string                                 `yaml:"transcript_additional_prompt,omitempty" json:"transcript_additional_prompt,omitempty" desc:"The additional prompt to add to the transcript. It is optional."`
	TTSLLM                     string                                 `yaml:"tts_llm,omitempty" json:"tts_llm,omitempty" desc:"The LLM name to use for TTS. Only supports gemini now. Default is the default LLM in llms section."`
	Speakers                   []RewriteRuleTransformToPodcastSpeaker `yaml:"speakers,omitempty" json:"speakers,omitempty" desc:"The speakers to use. It is required, at least one speaker is needed."`
}

type RewriteRuleTransformToPodcastSpeaker struct {
	Name  string `yaml:"name,omitempty" json:"name,omitempty" desc:"The name of the speaker. It is required."`
	Role  string `yaml:"role,omitempty" json:"role,omitempty" desc:"The role description of the speaker. You can think of it as a character setting."`
	Voice string `yaml:"voice,omitempty" json:"voice,omitempty" desc:"The voice of the speaker. It is required."`
}

type SchedulsRule struct {
	Name          string            `yaml:"name,omitempty" json:"name,omitempty" desc:"The name of the rule. It is required."`
	Query         string            `yaml:"query,omitempty" json:"query,omitempty" desc:"The semantic query to get the feeds. NOTE it is optional"`
	Threshold     float32           `yaml:"threshold,omitempty" json:"threshold,omitempty" desc:"The threshold to filter the query result by relevance (with 'query') score. It does not work when query is not set. Default is 0.6."`
	LabelFilters  []string          `yaml:"label_filters,omitempty" json:"label_filters,omitempty" desc:"The label filters (equal or not equal) to match the feeds. e.g. [category=tech, source!=github]"`
	Labels        map[string]string `yaml:"labels,omitempty" json:"labels,omitempty" desc:"The labels to attach to the feeds."`
	EveryDay      string            `yaml:"every_day,omitempty" json:"every_day,omitempty" desc:"The query range at the end time of every day. Format: start~end, e.g. 00:00~23:59, or -22:00~7:00 (yesterday 22:00 to today 07:00)."`
	WatchInterval timeutil.Duration `yaml:"watch_interval,omitempty" json:"watch_interval,omitempty" desc:"The run and query interval to watch the rule. Default is 10m. It can not be set with every_day at same time."`
}

type NotifyRoute struct {
	Receivers                  []string         `yaml:"receivers,omitempty" json:"receivers,omitempty" desc:"The notify receivers. It is required, at least one receiver is needed."`
	GroupBy                    []string         `yaml:"group_by,omitempty" json:"group_by,omitempty" desc:"The group by config to group the feeds, each group will be notified individually. It is required, at least one group by is needed."`
	SourceLabel                string           `yaml:"source_label,omitempty" json:"source_label,omitempty" desc:"The source label to extract the content from each feed, and summarize them. Default are all labels. It is very recommended to set it to 'summary' to reduce context length."`
	SummaryPrompt              string           `yaml:"summary_prompt,omitempty" json:"summary_prompt,omitempty" desc:"The prompt to summarize the feeds of each group."`
	LLM                        string           `yaml:"llm,omitempty" json:"llm,omitempty" desc:"The LLM name to use. Default is the default LLM in llms section. A large context length LLM is recommended."`
	CompressByRelatedThreshold *float32         `yaml:"compress_by_related_threshold,omitempty" json:"compress_by_related_threshold,omitempty" desc:"The threshold to compress the feeds by relatedness, that is, if the feeds are too similar, only one will be notified. Default is 0.85."`
	SubRoutes                  []NotifySubRoute `yaml:"sub_routes,omitempty" json:"sub_routes,omitempty" desc:"The sub routes to notify the feeds. A feed prefers to be matched by the sub routes, if not matched, it will be matched by the parent route."`
}

type NotifySubRoute struct {
	Matchers []string `yaml:"matchers,omitempty" json:"matchers,omitempty" desc:"The matchers to match the feeds. A feed prefers to be matched by the sub routes, if not matched, it will be matched by the parent route. e.g. [category=tech, source!=github]"`

	Receivers                  []string         `yaml:"receivers,omitempty" json:"receivers,omitempty" desc:"The notify receivers. It is required, at least one receiver is needed."`
	GroupBy                    []string         `yaml:"group_by,omitempty" json:"group_by,omitempty" desc:"The group by config to group the feeds, each group will be notified individually. It is required, at least one group by is needed."`
	SourceLabel                string           `yaml:"source_label,omitempty" json:"source_label,omitempty" desc:"The source label to extract the content from each feed, and summarize them. Default are all labels. It is very recommended to set it to 'summary' to reduce context length."`
	SummaryPrompt              string           `yaml:"summary_prompt,omitempty" json:"summary_prompt,omitempty" desc:"The prompt to summarize the feeds of each group."`
	LLM                        string           `yaml:"llm,omitempty" json:"llm,omitempty" desc:"The LLM name to use. Default is the default LLM in llms section. A large context length LLM is recommended."`
	CompressByRelatedThreshold *float32         `yaml:"compress_by_related_threshold,omitempty" json:"compress_by_related_threshold,omitempty" desc:"The threshold to compress the feeds by relatedness, that is, if the feeds are too similar, only one will be notified. Default is 0.85."`
	SubRoutes                  []NotifySubRoute `yaml:"sub_routes,omitempty" json:"sub_routes,omitempty" desc:"The sub routes to notify the feeds. A feed prefers to be matched by the sub routes, if not matched, it will be matched by the parent route."`
}

type NotifyReceiver struct {
	Name    string                 `yaml:"name,omitempty" json:"name,omitempty" desc:"The name of the receiver. It is required."`
	Email   string                 `yaml:"email,omitempty" json:"email,omitempty" desc:"The email of the receiver."`
	Webhook *NotifyReceiverWebhook `yaml:"webhook" json:"webhook" desc:"The webhook of the receiver."`
}

type NotifyReceiverWebhook struct {
	URL string `yaml:"url"`
}

type NotifyChannels struct {
	Email *NotifyChannelEmail `yaml:"email,omitempty" json:"email,omitempty" desc:"The global email channel config."`
}

type NotifyChannelEmail struct {
	SmtpEndpoint            string `yaml:"smtp_endpoint,omitempty" json:"smtp_endpoint,omitempty" desc:"The SMTP endpoint of the email channel. e.g. smtp.gmail.com:587"`
	From                    string `yaml:"from,omitempty" json:"from,omitempty" desc:"The sender email of the email channel."`
	Password                string `yaml:"password,omitempty" json:"password,omitempty" desc:"The application password of the sender email. If gmail, see https://support.google.com/mail/answer/185833"`
	FeedMarkdownTemplate    string `yaml:"feed_markdown_template,omitempty" json:"feed_markdown_template,omitempty" desc:"The markdown template of the feed. Default is {{ .content }}."`
	FeedHTMLSnippetTemplate string `yaml:"feed_html_snippet_template,omitempty" json:"feed_html_snippet_template,omitempty" desc:"The HTML snippet template of the feed. It can not be set with feed_markdown_template at same time."`
}

type Dependencies struct{}

type Watcher interface {
	Name() string
	Reload(app *App) error
}

type WatcherFunc func(app *App) error

func (f WatcherFunc) Name() string {
	return "Anonymous"
}

func (f WatcherFunc) Reload(app *App) error {
	return f(app)
}

// --- Factory code block ---
type Factory component.Factory[Manager, Config, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Manager, Config, Dependencies](func(instance string, config *Config, dependencies Dependencies) (Manager, error) {
			m := &mockManager{}
			component.MockOptions(mockOn).Apply(&m.Mock)

			return m, nil
		})
	}

	return component.FactoryFunc[Manager, Config, Dependencies](new)
}

func new(instance string, config *Config, dependencies Dependencies) (Manager, error) {
	m := &manager{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "ConfigManager",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		changedByAPI:    make(chan struct{}, 1),
		apiReloadResult: make(chan error, 1),
	}
	if err := m.tryReloadAppConfig(m.Context()); err != nil {
		return nil, errors.Wrap(err, "reload config")
	}

	return m, nil
}

// --- Implementation code block ---
type manager struct {
	*component.Base[Config, Dependencies]

	app         *App
	subscribers []Watcher
	mu          sync.RWMutex

	changedByAPI    chan struct{}
	apiReloadResult chan error
}

func (m *manager) Run() (err error) {
	ctx := telemetry.StartWith(m.Context(), append(m.TelemetryLabels(), telemetrymodel.KeyOperation, "Run")...)
	defer func() { telemetry.End(ctx, err) }()

	m.MarkReady()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if err := m.tryReloadAppConfig(ctx); err != nil {
				log.Error(ctx, err, "try reload app config on tick")
			}
		case <-m.changedByAPI:
			err := m.tryReloadAppConfig(ctx)
			if err != nil {
				log.Error(ctx, err, "try reload app config on api change")
			}
			m.apiReloadResult <- err
		case <-ctx.Done():
			return nil
		}
	}
}

func (m *manager) AppConfig() *App {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.app
}
func (m *manager) SaveAppConfig(app *App) error {
	b, err := yaml.Marshal(app)
	if err != nil {
		return errors.Wrap(err, "marshal app config")
	}

	// Create temp file in the same directory.
	dir := filepath.Dir(m.Config().Path)
	tmpFile, err := os.CreateTemp(dir, "*.tmp.yaml")
	if err != nil {
		return errors.Wrap(err, "create temp file")
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	// Write to temp file.
	if err := os.WriteFile(tmpPath, b, 0644); err != nil {
		return errors.Wrap(err, "write temp config")
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, m.Config().Path); err != nil {
		return errors.Wrap(err, "rename config file")
	}

	select {
	case m.changedByAPI <- struct{}{}:
	default:
	}
	if err := <-m.apiReloadResult; err != nil {
		return errors.Wrap(err, "reload app config")
	}

	return nil
}

func (m *manager) Subscribe(w Watcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscribers = append(m.subscribers, w)
}

func (m *manager) tryReloadAppConfig(ctx context.Context) (err error) {
	ctx = telemetry.StartWith(ctx, append(m.TelemetryLabels(), telemetrymodel.KeyOperation, "tryReloadAppConfig")...)
	defer func() { telemetry.End(ctx, err) }()
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read the config file.
	b, err := os.ReadFile(m.Config().Path)
	if err != nil {
		return errors.Wrap(err, "read config file")
	}
	var newConfig App
	if err := yaml.Unmarshal(b, &newConfig); err != nil {
		return errors.Wrap(err, "parse config file")
	}

	// Diff the new config with the old one.
	if reflect.DeepEqual(m.app, &newConfig) {
		log.Debug(ctx, "config is the same, skipping reload")

		return nil
	}

	// Notify the subscribers.
	for _, s := range m.subscribers {
		log.Debug(ctx, "notifying subscriber", "subscriber", s.Name())
		if err := s.Reload(&newConfig); err != nil {
			return errors.Wrap(err, "notify subscribers")
		}
	}

	// Update the config.
	m.app = &newConfig

	return nil
}

type mockManager struct {
	component.Mock
}

func (m *mockManager) AppConfig() *App {
	args := m.Called()

	return args.Get(0).(*App)
}

func (m *mockManager) SaveAppConfig(app *App) error {
	args := m.Called(app)

	return args.Error(0)
}

func (m *mockManager) Subscribe(w Watcher) {
	m.Called(w)
}
