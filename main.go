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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"

	"github.com/glidea/zenfeed/pkg/api"
	"github.com/glidea/zenfeed/pkg/api/http"
	"github.com/glidea/zenfeed/pkg/api/mcp"
	"github.com/glidea/zenfeed/pkg/api/rss"
	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/config"
	"github.com/glidea/zenfeed/pkg/llm"
	"github.com/glidea/zenfeed/pkg/notify"
	"github.com/glidea/zenfeed/pkg/notify/channel"
	"github.com/glidea/zenfeed/pkg/notify/route"
	"github.com/glidea/zenfeed/pkg/rewrite"
	"github.com/glidea/zenfeed/pkg/schedule"
	"github.com/glidea/zenfeed/pkg/schedule/rule"
	"github.com/glidea/zenfeed/pkg/scrape"
	"github.com/glidea/zenfeed/pkg/scrape/scraper"
	"github.com/glidea/zenfeed/pkg/storage/feed"
	"github.com/glidea/zenfeed/pkg/storage/feed/block"
	"github.com/glidea/zenfeed/pkg/storage/feed/block/chunk"
	"github.com/glidea/zenfeed/pkg/storage/feed/block/index/inverted"
	"github.com/glidea/zenfeed/pkg/storage/feed/block/index/primary"
	"github.com/glidea/zenfeed/pkg/storage/feed/block/index/vector"
	"github.com/glidea/zenfeed/pkg/storage/kv"
	"github.com/glidea/zenfeed/pkg/storage/object"
	"github.com/glidea/zenfeed/pkg/telemetry/log"
	telemetryserver "github.com/glidea/zenfeed/pkg/telemetry/server"
	timeutil "github.com/glidea/zenfeed/pkg/util/time"
)

var version = "dev" // Will be set by the build process.

var disclaimer = `
# Disclaimer

**Before using the zenfeed software (hereinafter referred to as "the Software"), please read and understand this disclaimer carefully. Your download, installation, or use of the Software or any related services signifies that you have read, understood, and agreed to be bound by all terms of this disclaimer. If you do not agree with any part of this disclaimer, please cease using the Software immediately.**

1.  **Provided "AS IS":** The Software is provided on an "AS IS" and "AS AVAILABLE" basis, without any warranties of any kind, either express or implied. The authors and contributors make no warranties or representations regarding the Software's merchantability, fitness for a particular purpose, non-infringement, accuracy, completeness, reliability, security, timeliness, or performance.

2.  **User Responsibility:** You are solely responsible for all actions taken using the Software. This includes, but is not limited to:
    *   **Data Source Selection:** You are responsible for selecting and configuring the data sources (e.g., RSS feeds, potential future Email sources) you connect to the Software. You must ensure you have the right to access and process the content from these sources and comply with their respective terms of service, copyright policies, and applicable laws and regulations.
    *   **Content Compliance:** You must not use the Software to process, store, or distribute any content that is unlawful, infringing, defamatory, obscene, or otherwise objectionable.
    *   **API Key and Credential Security:** You are responsible for safeguarding the security of any API keys, passwords, or other credentials you configure within the Software. The authors and contributors are not liable for any loss or damage arising from your failure to maintain proper security.
    *   **Configuration and Use:** You are responsible for correctly configuring and using the Software's features, including content processing pipelines, filtering rules, notification settings, etc.

3.  **Third-Party Content and Services:** The Software may integrate with or rely on third-party data sources and services (e.g., RSSHub, LLM providers, SMTP service providers). The authors and contributors are not responsible for the availability, accuracy, legality, security, or terms of service of such third-party content or services. Your interactions with these third parties are governed by their respective terms and policies. Copyright for third-party content accessed or processed via the Software (including original articles, summaries, classifications, scores, etc.) belongs to the original rights holders, and you assume all legal liability arising from your use of such content.

4.  **No Warranty on Content Processing:** The Software utilizes technologies like Large Language Models (LLMs) to process content (e.g., summarization, classification, scoring, filtering). These processed results may be inaccurate, incomplete, or biased. The authors and contributors are not responsible for any decisions made or actions taken based on these processed results. The accuracy of semantic search results is also affected by various factors and is not guaranteed.

5.  **No Liability for Indirect or Consequential Damages:** In no event shall the authors or contributors be liable under any legal theory (whether contract, tort, or otherwise) for any direct, indirect, incidental, special, exemplary, or consequential damages arising out of the use or inability to use the Software. This includes, but is not limited to, loss of profits, loss of data, loss of goodwill, business interruption, or other commercial damages or losses, even if advised of the possibility of such damages.

6.  **Open Source Software:** The Software is licensed under the AGPLv3 License. You are responsible for understanding and complying with the terms of this license.

7.  **Not Legal Advice:** This disclaimer does not constitute legal advice. If you have any questions regarding the legal implications of using the Software, you should consult a qualified legal professional.

8.  **Modification and Acceptance:** The authors reserve the right to modify this disclaimer at any time. Continued use of the Software following any modifications will be deemed acceptance of the revised terms.

**Please be aware: Using the Software to fetch, process, and distribute copyrighted content may carry legal risks. Users are responsible for ensuring their usage complies with all applicable laws, regulations, and third-party terms of service. The authors and contributors assume no liability for any legal disputes or losses arising from user misuse or improper use of the Software.**

`

func main() {
	ctx := context.Background()

	// Parse Flags.
	configPath := flag.String("config", "./config.yaml", "path to the config file")
	justVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	// Print Disclaimer & Version.
	fmt.Println(disclaimer)
	fmt.Println("version:", version)
	if *justVersion {
		return
	}

	// Create App.
	app := newApp(*configPath)

	// Setup App.
	if err := app.setup(); err != nil {
		log.Fatal(ctx, err, "setup application")
	}
	log.Info(ctx, "setup application complete")

	// Run App.
	if err := app.run(ctx); err != nil {
		log.Fatal(ctx, err, "run application")
	}

	log.Info(ctx, "exiting application")
}

// App holds the application's components and manages its lifecycle.
type App struct {
	configPath string
	configMgr  config.Manager
	conf       *config.App
	telemetry  telemetryserver.Server

	kvStorage     kv.Storage
	llmFactory    llm.Factory
	rewriter      rewrite.Rewriter
	feedStorage   feed.Storage
	objectStorage object.Storage
	api           api.API
	http          http.Server
	mcp           mcp.Server
	rss           rss.Server
	scraperMgr    scrape.Manager
	scheduler     schedule.Scheduler
	notifier      notify.Notifier
	notifyChan    chan *rule.Result
}

// newApp creates a new application instance.
func newApp(configPath string) *App {
	return &App{
		configPath: configPath,
		notifyChan: make(chan *rule.Result, 1000),
	}
}

// setup initializes all application components.
func (a *App) setup() error {
	if err := a.setupConfig(); err != nil {
		return errors.Wrap(err, "setup config")
	}

	if err := a.applyGlobals(a.conf); err != nil {
		return errors.Wrap(err, "apply initial global settings")
	}
	a.configMgr.Subscribe(config.WatcherFunc(func(newConf *config.App) error {
		return a.applyGlobals(newConf)
	}))

	if err := a.setupTelemetryServer(); err != nil {
		return errors.Wrap(err, "setup telemetry server")
	}

	if err := a.setupKVStorage(); err != nil {
		return errors.Wrap(err, "setup kv storage")
	}
	if err := a.setupObjectStorage(); err != nil {
		return errors.Wrap(err, "setup object storage")
	}
	if err := a.setupLLMFactory(); err != nil {
		return errors.Wrap(err, "setup llm factory")
	}
	if err := a.setupRewriter(); err != nil {
		return errors.Wrap(err, "setup rewriter")
	}
	if err := a.setupFeedStorage(); err != nil {
		return errors.Wrap(err, "setup feed storage")
	}
	if err := a.setupAPI(); err != nil {
		return errors.Wrap(err, "setup api")
	}
	if err := a.setupHTTPServer(); err != nil {
		return errors.Wrap(err, "setup http server")
	}
	if err := a.setupMCPServer(); err != nil {
		return errors.Wrap(err, "setup mcp server")
	}
	if err := a.setupRSSServer(); err != nil {
		return errors.Wrap(err, "setup rss server")
	}
	if err := a.setupScraper(); err != nil {
		return errors.Wrap(err, "setup scraper")
	}
	if err := a.setupScheduler(); err != nil {
		return errors.Wrap(err, "setup scheduler")
	}
	if err := a.setupNotifier(); err != nil {
		return errors.Wrap(err, "setup notifier")
	}

	return nil
}

// setupConfig loads the configuration manager.
func (a *App) setupConfig() (err error) {
	a.configMgr, err = config.NewFactory().New(component.Global, &config.Config{Path: a.configPath}, config.Dependencies{})
	if err != nil {
		return err
	}

	a.conf = a.configMgr.AppConfig()
	a.configMgr.Subscribe(config.WatcherFunc(func(newConf *config.App) error {
		a.conf = newConf

		return nil
	}))

	return nil
}

// applyGlobals sets global settings based on config.
func (a *App) applyGlobals(conf *config.App) error {
	if err := timeutil.SetLocation(conf.Timezone); err != nil {
		return errors.Wrapf(err, "set timezone to %s", conf.Timezone)
	}
	if err := log.SetLevel(log.Level(conf.Telemetry.Log.Level)); err != nil {
		return errors.Wrapf(err, "set log level to %s", conf.Telemetry.Log.Level)
	}

	return nil
}

// setupKVStorage initializes the Key-Value storage.
func (a *App) setupKVStorage() (err error) {
	a.kvStorage, err = kv.NewFactory().New(component.Global, a.conf, kv.Dependencies{})

	return err
}

// setupLLMFactory initializes the LLM factory.
func (a *App) setupLLMFactory() (err error) {
	a.llmFactory, err = llm.NewFactory(component.Global, a.conf, llm.FactoryDependencies{
		KVStorage: a.kvStorage,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.llmFactory)

	return nil
}

// setupRewriter initializes the Rewriter factory.
func (a *App) setupRewriter() (err error) {
	a.rewriter, err = rewrite.NewFactory().New(component.Global, a.conf, rewrite.Dependencies{
		LLMFactory:    a.llmFactory,
		ObjectStorage: a.objectStorage,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.rewriter)

	return nil
}

// setupFeedStorage initializes the Feed storage.
func (a *App) setupFeedStorage() (err error) {
	a.feedStorage, err = feed.NewFactory().New(component.Global, a.conf, feed.Dependencies{
		LLMFactory:      a.llmFactory,
		Rewriter:        a.rewriter,
		BlockFactory:    block.NewFactory(),
		ChunkFactory:    chunk.NewFactory(),
		PrimaryFactory:  primary.NewFactory(),
		InvertedFactory: inverted.NewFactory(),
		VectorFactory:   vector.NewFactory(),
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.feedStorage)

	return nil
}

// setupObjectStorage initializes the Object storage.
func (a *App) setupObjectStorage() (err error) {
	a.objectStorage, err = object.NewFactory().New(component.Global, a.conf, object.Dependencies{})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.objectStorage)

	return nil
}

// setupTelemetryServer initializes the Telemetry server.
func (a *App) setupTelemetryServer() (err error) {
	a.telemetry, err = telemetryserver.NewFactory().New(component.Global, a.conf, telemetryserver.Dependencies{})
	if err != nil {
		return err
	}

	return nil
}

// setupAPI initializes the API service.
func (a *App) setupAPI() (err error) {
	a.api, err = api.NewFactory().New(component.Global, a.conf, api.Dependencies{
		ConfigManager: a.configMgr,
		FeedStorage:   a.feedStorage,
		LLMFactory:    a.llmFactory,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.api)

	return nil
}

// setupHTTPServer initializes the HTTP server.
func (a *App) setupHTTPServer() (err error) {
	a.http, err = http.NewFactory().New(component.Global, a.conf, http.Dependencies{
		API: a.api,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.http)

	return nil
}

// setupMCPServer initializes the MCP server.
func (a *App) setupMCPServer() (err error) {
	a.mcp, err = mcp.NewFactory().New(component.Global, a.conf, mcp.Dependencies{
		API: a.api,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.mcp)

	return nil
}

// setupRSSServer initializes the RSS server.
func (a *App) setupRSSServer() (err error) {
	a.rss, err = rss.NewFactory().New(component.Global, a.conf, rss.Dependencies{
		API: a.api,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.rss)

	return nil
}

// setupScraper initializes the Scraper manager.
func (a *App) setupScraper() (err error) {
	a.scraperMgr, err = scrape.NewFactory().New(component.Global, a.conf, scrape.Dependencies{
		ScraperFactory: scraper.NewFactory(),
		FeedStorage:    a.feedStorage,
		KVStorage:      a.kvStorage,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.scraperMgr)

	return nil
}

// setupScheduler initializes the Scheduler.
func (a *App) setupScheduler() (err error) {
	a.scheduler, err = schedule.NewFactory().New(component.Global, a.conf, schedule.Dependencies{
		RuleFactory: rule.NewFactory(),
		FeedStorage: a.feedStorage,
		Out:         a.notifyChan,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.scheduler)

	return nil
}

// setupNotifier initializes the Notifier.
func (a *App) setupNotifier() (err error) {
	a.notifier, err = notify.NewFactory().New(component.Global, a.conf, notify.Dependencies{
		In:             a.notifyChan, // Receive from the channel.
		RelatedScore:   vector.Score,
		RouterFactory:  route.NewFactory(),
		ChannelFactory: channel.NewFactory(),
		KVStorage:      a.kvStorage,
		LLMFactory:     a.llmFactory,
	})
	if err != nil {
		return err
	}

	a.configMgr.Subscribe(a.notifier)

	return nil
}

// run starts the application components and blocks until shutdown.
func (a *App) run(ctx context.Context) error {
	defer close(a.notifyChan) // Close channel when Run finishes.

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info(ctx, "received signal, shutting down", "signal", sig.String())
		cancel()
	}()

	log.Info(ctx, "starting application components...")
	if err := component.Run(ctx,
		component.Group{a.configMgr},
		component.Group{a.llmFactory, a.objectStorage, a.telemetry},
		component.Group{a.rewriter},
		component.Group{a.feedStorage},
		component.Group{a.kvStorage},
		component.Group{a.notifier, a.api},
		component.Group{a.http, a.mcp, a.rss, a.scraperMgr, a.scheduler},
	); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	log.Info(ctx, "Application stopped gracefully")

	return nil
}
