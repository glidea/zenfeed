| Field       | Type     | Description                                                                                                                                                                                                                                                                                                                    | Default Value         | Required         |
| :---------- | :------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------- | :--------------- |
| `timezone`  | `string` | The application's timezone. E.g., `Asia/Shanghai`.                                                                                                                                                                                                                                                                             | Server local time     | No               |
| `telemetry` | `object` | Telemetry configuration. See the **Telemetry Configuration** section below.                                                                                                                                                                                                                                                    | (See specific fields) | No               |
| `api`       | `object` | API configuration. See the **API Configuration** section below.                                                                                                                                                                                                                                                                | (See specific fields) | No               |
| `llms`      | `list`   | Large Language Model (LLM) configuration. Referenced by other configuration sections. See the **LLM Configuration** section below.                                                                                                                                                                                             | `[]`                  | Yes (at least 1) |
| `jina`      | `object` | Jina AI configuration. See the **Jina AI Configuration** section below.                                                                                                                                                                                                                                                        | (See specific fields) | No               |
| `scrape`    | `object` | Scrape configuration. See the **Scrape Configuration** section below.                                                                                                                                                                                                                                                          | (See specific fields) | No               |
| `storage`   | `object` | Storage configuration. See the **Storage Configuration** section below.                                                                                                                                                                                                                                                        | (See specific fields) | No               |
| `scheduls`  | `object` | Scheduling configuration for monitoring feeds (also known as monitoring rules). See the **Scheduling Configuration** section below.                                                                                                                                                                                            | (See specific fields) | No               |
| `notify`    | `object` | Notification configuration. It receives results from the scheduling module, groups them via routing configuration, and sends them to notification receivers via notification channels. See the **Notification Configuration**, **Notification Routing**, **Notification Receivers**, **Notification Channels** sections below. | (See specific fields) | Yes              |

### Telemetry Configuration (`telemetry`)

| Field                 | Type     | Description                                                                        | Default Value         | Required |
| :-------------------- | :------- | :--------------------------------------------------------------------------------- | :-------------------- | :------- |
| `telemetry.address`   | `string` | Exposes Prometheus metrics & pprof.                                                | `:9090`               | No       |
| `telemetry.log`       | `object` | Log configuration related to telemetry.                                            | (See specific fields) | No       |
| `telemetry.log.level` | `string` | Log level for telemetry-related messages, one of `debug`, `info`, `warn`, `error`. | `info`                | No       |

### API Configuration (`api`)

| Field              | Type     | Description                                                                                                                  | Default Value                 | Required               |
| :----------------- | :------- | :--------------------------------------------------------------------------------------------------------------------------- | :---------------------------- | :--------------------- |
| `api.http`         | `object` | HTTP API configuration.                                                                                                      | (See specific fields)         | No                     |
| `api.http.address` | `string` | Address for the HTTP API (`[host]:port`). E.g., `0.0.0.0:1300`. Cannot be changed after the application starts.              | `:1300`                       | No                     |
| `api.mcp`          | `object` | MCP API configuration.                                                                                                       | (See specific fields)         | No                     |
| `api.mcp.address`  | `string` | Address for the MCP API (`[host]:port`). E.g., `0.0.0.0:1301`. Cannot be changed after the application starts.               | `:1301`                       | No                     |
| `api.llm`          | `string` | Name of the LLM used for summarizing feeds. E.g., `my-favorite-gemini-king`. Refers to an LLM defined in the `llms` section. | Default LLM in `llms` section | Yes (if using summary) |

### LLM Configuration (`llms[]`)

This section defines the list of available Large Language Models. At least one LLM configuration is required.

| Field                    | Type      | Description                                                                                                                                                                                                                                          | Default Value               | Required                                                   |
| :----------------------- | :-------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :-------------------------- | :--------------------------------------------------------- |
| `llms[].name`            | `string`  | Name (or 'id') of the LLM. E.g., `my-favorite-gemini-king`. Used to refer to this LLM in other configuration sections (e.g., `api.llm`, `storage.feed.embedding_llm`).                                                                               |                             | Yes                                                        |
| `llms[].default`         | `bool`    | Whether this LLM is the default LLM. Only one LLM can be the default.                                                                                                                                                                                | `false`                     | No (but one must be `true` if relying on default behavior) |
| `llms[].provider`        | `string`  | Provider of the LLM, one of `openai`, `openrouter`, `deepseek`, `gemini`, `volc`, `siliconflow`. E.g., `openai`.                                                                                                                                     |                             | Yes                                                        |
| `llms[].endpoint`        | `string`  | Custom endpoint for the LLM. E.g., `https://api.openai.com/v1`.                                                                                                                                                                                      | (Provider-specific default) | No                                                         |
| `llms[].api_key`         | `string`  | API key for the LLM.                                                                                                                                                                                                                                 |                             | Yes                                                        |
| `llms[].model`           | `string`  | Model of the LLM. E.g., `gpt-4o-mini`. Cannot be empty if used for generation tasks (e.g., summarization). If this LLM is used, cannot be empty along with `embedding_model`.                                                                        |                             | Conditionally Required                                     |
| `llms[].embedding_model` | `string`  | Embedding model of the LLM. E.g., `text-embedding-3-small`. Cannot be empty if used for embedding. If this LLM is used, cannot be empty along with `model`. **Note:** Do not modify directly after initial use; add a new LLM configuration instead. |                             | Conditionally Required                                     |
| `llms[].tts_model`       | `string`  | The Text-to-Speech (TTS) model of the LLM.                                                                                                                                                                                                           |                             | No                                                         |
| `llms[].temperature`     | `float32` | Temperature of the LLM (0-2).                                                                                                                                                                                                                        | `0.0`                       | No                                                         |

### Jina AI Configuration (`jina`)

This section configures parameters related to the Jina AI Reader API, primarily used by the `crawl_by_jina` type in rewrite rules.

| Field        | Type     | Description                                                                                                                                                                                                                        | Default Value | Required |
| :----------- | :------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------------ | :------- |
| `jina.token` | `string` | API Token for Jina AI. Obtain from [Jina AI API Dashboard](https://jina.ai/api-dashboard/). Providing a token grants higher service rate limits. If left empty, requests will be made as an anonymous user with lower rate limits. |               | No       |

### Scrape Configuration (`scrape`)

| Field                    | Type              | Description                                                                                                                                                                            | Default Value | Required                             |
| :----------------------- | :---------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------------ | :----------------------------------- |
| `scrape.past`            | `time.Duration`   | Time window to look back when scraping feeds. E.g., `1h` means only scrape feeds from the past 1 hour.                                                                                 | `24h`         | No                                   |
| `scrape.interval`        | `time.Duration`   | Frequency to scrape each source (global default). E.g., `1h`.                                                                                                                          | `1h`          | No                                   |
| `scrape.rsshub_endpoint` | `string`          | Endpoint for RSSHub. You can deploy your own RSSHub server or use a public instance (see [RSSHub Documentation](https://docs.rsshub.app/guide/instances)). E.g., `https://rsshub.app`. |               | Yes (if `rsshub_route_path` is used) |
| `scrape.sources`         | `list of objects` | List of sources to scrape feeds from. See **Scrape Source Configuration** below.                                                                                                       | `[]`          | Yes (at least one)                   |

### Scrape Source Configuration (`scrape.sources[]`)

Describes each source to be scraped.

| Field                       | Type                | Description                                                                                                                           | Default Value     | Required                    |
| :-------------------------- | :------------------ | :------------------------------------------------------------------------------------------------------------------------------------ | :---------------- | :-------------------------- |
| `scrape.sources[].interval` | `time.Duration`     | Frequency to scrape this specific source. Overrides global `scrape.interval`.                                                         | Global `interval` | No                          |
| `scrape.sources[].name`     | `string`            | Name of the source. Used to tag feeds.                                                                                                |                   | Yes                         |
| `scrape.sources[].labels`   | `map[string]string` | Additional key-value labels to attach to feeds from this source.                                                                      | `{}`              | No                          |
| `scrape.sources[].rss`      | `object`            | RSS configuration for this source. See **Scrape Source RSS Configuration** below. Each source can only have one type set (e.g., RSS). | `nil`             | Yes (if source type is RSS) |

### Scrape Source RSS Configuration (`scrape.sources[].rss`)

| Field                                    | Type     | Description                                                                                                                                                    | Default Value | Required                                                  |
| :--------------------------------------- | :------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------------ | :-------------------------------------------------------- |
| `scrape.sources[].rss.url`               | `string` | Full URL of the RSS feed. E.g., `http://localhost:1200/github/trending/daily/any`. Cannot be set if `rsshub_route_path` is set.                                |               | Yes (unless `rsshub_route_path` is set)                   |
| `scrape.sources[].rss.rsshub_route_path` | `string` | RSSHub route path. E.g., `github/trending/daily/any`. Will be concatenated with `scrape.rsshub_endpoint` to form the final URL. Cannot be set if `url` is set. |               | Yes (unless `url` is set, and requires `rsshub_endpoint`) |

### Storage Configuration (`storage`)

| Field            | Type     | Description                                                                                               | Default Value         | Required |
| :--------------- | :------- | :-------------------------------------------------------------------------------------------------------- | :-------------------- | :------- |
| `storage.dir`    | `string` | Base directory for all storage. Cannot be changed after the application starts.                           | `./data`              | No       |
| `storage.feed`   | `object` | Feed storage configuration. See **Feed Storage Configuration** below.                                     | (See specific fields) | No       |
| `storage.object` | `object` | Object storage configuration for storing files like podcasts. See **Object Storage Configuration** below. | (See specific fields) | No       |

### Feed Storage Configuration (`storage.feed`)

| Field                         | Type              | Description                                                                                                                                                                                                                                                               | Default Value                 | Required |
| :---------------------------- | :---------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | :---------------------------- | :------- |
| `storage.feed.rewrites`       | `list of objects` | How to process each feed before storing it. Inspired by Prometheus relabeling. See **Rewrite Rule Configuration** below.                                                                                                                                                  | `[]`                          | No       |
| `storage.feed.flush_interval` | `time.Duration`   | Frequency to flush feed storage to the database. Higher values risk more data loss but reduce disk operations and improve performance.                                                                                                                                    | `200ms`                       | No       |
| `storage.feed.embedding_llm`  | `string`          | Name of the LLM used for feed embedding (from `llms` section). Significantly impacts semantic search accuracy. **Note:** If switching, keep the old LLM configuration as past data is implicitly associated with it, otherwise past data cannot be semantically searched. | Default LLM in `llms` section | No       |
| `storage.feed.retention`      | `time.Duration`   | Retention duration for feeds.                                                                                                                                                                                                                                             | `8d`                          | No       |
| `storage.feed.block_duration` | `time.Duration`   | Retention duration for each time-based feed storage block (similar to Prometheus TSDB Block).                                                                                                                                                                             | `25h`                         | No       |

### Object Storage Configuration (`storage.object`)

| Field                              | Type     | Description                                  | Default Value | Required                       |
| :--------------------------------- | :------- | :------------------------------------------- | :------------ | :----------------------------- |
| `storage.object.endpoint`          | `string` | The endpoint of the object storage.          |               | Yes (if using podcast feature) |
| `storage.object.access_key_id`     | `string` | The access key id of the object storage.     |               | Yes (if using podcast feature) |
| `storage.object.secret_access_key` | `string` | The secret access key of the object storage. |               | Yes (if using podcast feature) |
| `storage.object.bucket`            | `string` | The bucket of the object storage.            |               | Yes (if using podcast feature) |
| `storage.object.bucket`        | `string` | The URL of the object storage bucket.        |               | No                             |

### Rewrite Rule Configuration (`storage.feed.rewrites[]`)

Defines rules to process feeds before storage. Rules are applied sequentially.

| Field                                    | Type              | Description                                                                                                                                                                                                                                 | Default Value            | Required                                      |
| :--------------------------------------- | :---------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | :----------------------- | :-------------------------------------------- |
| `...rewrites[].if`                       | `list of strings` | Conditions to match feeds. If not set, matches all feeds. Similar to label filters, e.g., `["source=github", "title!=xxx"]`. If conditions are not met, this rule is skipped.                                                               | `[]` (matches all)       | No                                            |
| `...rewrites[].source_label`             | `string`          | Feed label used as the source text for transformation. Default labels include: `type`, `source`, `title`, `link`, `pub_time`, `content`.                                                                                                    | `content`                | No                                            |
| `...rewrites[].skip_too_short_threshold` | `*int`            | If set, feeds where the `source_label` text length is below this threshold will be skipped by this rule (processing continues to the next rule, or feed storage if no more rules). Helps filter out feeds that are too short/uninformative. | `300`                    | No                                            |
| `...rewrites[].transform`                | `object`          | Configures how to transform the `source_label` text. See **Rewrite Rule Transform Configuration** below. If not set, the `source_label` text is used directly for matching.                                                                 | `nil`                    | No                                            |
| `...rewrites[].match`                    | `string`          | Simple string to match against the (transformed) text. Cannot be set with `match_re`.                                                                                                                                                       |                          | No (use `match` or `match_re`)                |
| `...rewrites[].match_re`                 | `string`          | Regular expression to match against the (transformed) text.                                                                                                                                                                                 | `.*` (matches all)       | No (use `match` or `match_re`)                |
| `...rewrites[].action`                   | `string`          | Action to perform on match: `create_or_update_label` (adds/updates a label with the matched/transformed text), `drop_feed` (discards the feed entirely).                                                                                    | `create_or_update_label` | No                                            |
| `...rewrites[].label`                    | `string`          | Name of the feed label to create or update.                                                                                                                                                                                                 |                          | Yes (if `action` is `create_or_update_label`) |
| `...transform.to_text`                   | `object`          | Transforms source text to text using an LLM. See **Rewrite Rule To Text Configuration** below.                                                                                                                                              | `nil`                    | No                                            |
| `...transform.to_podcast`                | `object`          | Transforms source text to a podcast. See **Rewrite Rule To Podcast Configuration** below.                                                                                                                                                   | `nil`                    | No                                            |

### Rewrite Rule To Text Configuration (`storage.feed.rewrites[].transform.to_text`)

This configuration defines how to transform the text from `source_label`.

| Field               | Type     | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | Default Value                 | Required                    |
| :------------------ | :------- | :---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------- | :-------------------------- |
| `...to_text.type`   | `string` | Type of transformation. Options: <ul><li>`prompt` (default): Uses an LLM and a specified prompt to transform the source text.</li><li>`crawl`: Treats the source text as a URL, directly crawls the web page content pointed to by the URL, and converts it to Markdown format. This method performs local crawling and attempts to follow `robots.txt`.</li><li>`crawl_by_jina`: Treats the source text as a URL, crawls and processes web page content via the [Jina AI Reader API](https://jina.ai/reader/), and returns Markdown. Potentially more powerful, e.g., for handling dynamic pages, but relies on the Jina AI service.</li></ul> | `prompt`                      | No                          |
| `...to_text.llm`    | `string` | **Only valid if `type` is `prompt`.** Name of the LLM used for transformation (from `llms` section). If not specified, the LLM marked as `default: true` in the `llms` section will be used.                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Default LLM in `llms` section | No                          |
| `...to_text.prompt` | `string` | **Only valid if `type` is `prompt`.** Prompt used for transformation. The source text will be injected. You can use Go template syntax to reference built-in prompts: `{{ .summary }}`, `{{ .category }}`, `{{ .tags }}`, `{{ .score }}`, `{{ .comment_confucius }}`, `{{ .summary_html_snippet }}`, `{{ .summary_html_snippet_for_small_model }}`.                                                                                                                                                                                                                                                                                             |                               | Yes (if `type` is `prompt`) |

### Rewrite Rule To Podcast Configuration (`storage.feed.rewrites[].transform.to_podcast`)

This configuration defines how to transform the text from `source_label` into a podcast.

| Field                                        | Type              | Description                                                                                                                                    | Default Value                 | Required |
| :------------------------------------------- | :---------------- | :--------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------- | :------- |
| `...to_podcast.llm`                          | `string`          | The name of the LLM (from the `llms` section) to use for generating the podcast script.                                                        | Default LLM in `llms` section | No       |
| `...to_podcast.transcript_additional_prompt` | `string`          | Additional instructions to append to the prompt for generating the podcast script.                                                             |                               | No       |
| `...to_podcast.tts_llm`                      | `string`          | The name of the LLM (from the `llms` section) to use for Text-to-Speech (TTS). **Note: Currently only supports LLMs with `provider: gemini`**. | Default LLM in `llms` section | No       |
| `...to_podcast.speakers`                     | `list of objects` | A list of speakers for the podcast. See **Speaker Configuration** below.                                                                       | `[]`                          | Yes      |

#### Speaker Configuration (`...to_podcast.speakers[]`)

| Field                 | Type     | Description                          | Default Value | Required |
| :-------------------- | :------- | :----------------------------------- | :------------ | :------- |
| `...speakers[].name`  | `string` | The name of the speaker.             |               | Yes      |
| `...speakers[].role`  | `string` | The role description of the speaker. |               | No       |
| `...speakers[].voice` | `string` | The voice of the speaker.            |               | Yes      |

### Scheduling Configuration (`scheduls`)

Defines rules for querying and monitoring feeds.

| Field            | Type              | Description                                                                                                                                                    | Default Value | Required |
| :--------------- | :---------------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------------ | :------- |
| `scheduls.rules` | `list of objects` | List of rules for scheduling feeds. The results of each rule (matched feeds) will be sent to notification routes. See **Scheduling Rule Configuration** below. | `[]`          | No       |

### Scheduling Rule Configuration (`scheduls.rules[]`)

| Field                             | Type                | Description                                                                                                                                                                                  | Default Value | Required                                 |
| :-------------------------------- | :------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :------------ | :--------------------------------------- |
| `scheduls.rules[].name`           | `string`            | Name of the rule.                                                                                                                                                                            |               | Yes                                      |
| `scheduls.rules[].query`          | `string`            | Semantic query to find relevant feeds. Optional.                                                                                                                                             |               | No                                       |
| `scheduls.rules[].threshold`      | `float32`           | Relevance score threshold (0-1) for filtering semantic query results. Only effective if `query` is set.                                                                                      | `0.6`         | No                                       |
| `scheduls.rules[].label_filters`  | `list of strings`   | Filters based on feed labels (equals or not equals). E.g., `["category=tech", "source!=github"]`.                                                                                            | `[]`          | No                                       |
| `scheduls.rules[].labels`         | `map[string]string` | Additional key-value labels attached to this source Feed.                                                                                                                                    | `{}`          | No                                       |
| `scheduls.rules[].every_day`      | `string`            | Query range relative to the end of each day. Format: `start~end` (HH:MM). E.g., `00:00~23:59` (today), `-22:00~07:00` (yesterday 22:00 to today 07:00). Cannot be set with `watch_interval`. |               | No (use `every_day` or `watch_interval`) |
| `scheduls.rules[].watch_interval` | `time.Duration`     | Frequency to run the query. E.g., `10m`. Cannot be set with `every_day`.                                                                                                                     | `10m`         | No (use `every_day` or `watch_interval`) |

### Notification Configuration (`notify`)

| Field              | Type              | Description                                                                                                     | Default Value         | Required                |
| :----------------- | :---------------- | :-------------------------------------------------------------------------------------------------------------- | :-------------------- | :---------------------- |
| `notify.route`     | `object`          | Main notification routing configuration. See **Notification Routing Configuration** below.                      | (See specific fields) | Yes                     |
| `notify.receivers` | `list of objects` | Defines notification receivers (e.g., email addresses). See **Notification Receiver Configuration** below.      | `[]`                  | Yes (at least one)      |
| `notify.channels`  | `object`          | Configures notification channels (e.g., email SMTP settings). See **Notification Channel Configuration** below. | (See specific fields) | Yes (if using channels) |

### Notification Routing Configuration (`notify.route` and `notify.route.sub_routes[]`)

This structure can be nested using `sub_routes`. Feeds will first try to match sub-routes; if no sub-route matches, the parent route's configuration is applied.

| Field                              | Type              | Description                                                                                                                                                        | Default Value                 | Required              |
| :--------------------------------- | :---------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------- | :---------------------------- | :-------------------- |
| `...matchers` (sub-routes only)    | `list of strings` | Label matchers to determine if a feed belongs to this sub-route. E.g., `["category=tech", "source!=github"]`.                                                      | `[]`                          | Yes (sub-routes only) |
| `...receivers`                     | `list of strings` | List of receiver names (defined in `notify.receivers`) to send notifications for feeds matching this route.                                                        | `[]`                          | Yes (at least one)    |
| `...group_by`                      | `list of strings` | List of labels to group feeds by before sending notifications. Each group results in a separate notification. E.g., `["source", "category"]`.                      | `[]`                          | Yes (at least one)    |
| `...source_label`                  | `string`          | Source label to extract content from each feed for summarization. Defaults to all labels. Strongly recommended to set to 'summary' to reduce context length.       | All labels                    | No                    |
| `...summary_prompt`                | `string`          | Prompt to summarize feeds for each group.                                                                                                                          |                               | No                    |
| `...llm`                           | `string`          | Name of the LLM to use. Defaults to the default LLM in the `llms` section. Recommended to use an LLM with a large context length.                                  | Default LLM in `llms` section | No                    |
| `...compress_by_related_threshold` | `*float32`        | If set, compresses highly similar feeds within a group based on semantic relatedness, sending only one representative. Threshold (0-1), higher means more similar. | `0.85`                        | No                    |
| `...sub_routes`                    | `list of objects` | List of nested routes. Allows defining more specific routing rules. Each object follows **Notification Routing Configuration**.                                    | `[]`                          | No                    |

### Notification Receiver Configuration (`notify.receivers[]`)

Defines *who* receives notifications.

| Field                        | Type     | Description                                                              | Default Value | Required               |
| :--------------------------- | :------- | :----------------------------------------------------------------------- | :------------ | :--------------------- |
| `notify.receivers[].name`    | `string` | Unique name of the receiver. Used in routes.                             |               | Yes                    |
| `notify.receivers[].email`   | `string` | Email address of the receiver.                                           |               | Yes (if using Email)   |
| `notify.receivers[].webhook` | `object` | Webhook configuration for the receiver. E.g. `webhook: { "url": "xxx" }` |               | Yes (if using Webhook) |

### Notification Channel Configuration (`notify.channels`)

Configures *how* notifications are sent.

| Field                   | Type     | Description                                                                                 | Default Value | Required             |
| :---------------------- | :------- | :------------------------------------------------------------------------------------------ | :------------ | :------------------- |
| `notify.channels.email` | `object` | Global Email channel configuration. See **Notification Channel Email Configuration** below. | `nil`         | Yes (if using Email) |

### Notification Channel Email Configuration (`notify.channels.email`)

| Field                                 | Type     | Description                                                                                                                                                                                         | Default Value    | Required |
| :------------------------------------ | :------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | :--------------- | :------- |
| `...email.smtp_endpoint`              | `string` | SMTP server endpoint. E.g., `smtp.gmail.com:587`.                                                                                                                                                   |                  | Yes      |
| `...email.from`                       | `string` | Sender's email address.                                                                                                                                                                             |                  | Yes      |
| `...email.password`                   | `string` | App-specific password for the sender's email. (For Gmail, see [Google App Passwords](https://support.google.com/mail/answer/185833)).                                                               |                  | Yes      |
| `...email.feed_markdown_template`     | `string` | Markdown template for formatting each feed in the email body. Renders feed content by default. Cannot be set with `feed_html_snippet_template`. Available template variables depend on feed labels. | `{{ .content }}` | No       |
| `...email.feed_html_snippet_template` | `string` | HTML snippet template for formatting each feed. Cannot be set with `feed_markdown_template`. Available template variables depend on feed labels.                                                    |                  | No       |
