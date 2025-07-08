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

package rewrite

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/config"
	"github.com/glidea/zenfeed/pkg/llm"
	"github.com/glidea/zenfeed/pkg/llm/prompt"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/storage/object"
	"github.com/glidea/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	"github.com/glidea/zenfeed/pkg/util/buffer"
	"github.com/glidea/zenfeed/pkg/util/crawl"
	hashutil "github.com/glidea/zenfeed/pkg/util/hash"
)

// --- Interface code block ---
type Rewriter interface {
	component.Component
	config.Watcher

	// Labels applies rewrite rules to the given labels and returns the modified labels.
	// Note: this method modifies the input labels in place.
	// If a rule's action is ActionDropFeed, it returns nil to indicate the item should be dropped.
	Labels(ctx context.Context, labels model.Labels) (model.Labels, error)
}

type Config []Rule

func (c *Config) Validate() error {
	for i := range *c {
		if err := (*c)[i].Validate(); err != nil {
			return errors.Wrapf(err, "validate and adjust rewrite config")
		}
	}

	return nil
}

func (c *Config) From(app *config.App) {
	for _, r := range app.Storage.Feed.Rewrites {
		var rc Rule
		rc.From(&r)
		*c = append(*c, rc)
	}
}

type Dependencies struct {
	LLMFactory    llm.Factory // NOTE: String() with cache.
	ObjectStorage object.Storage
}

type Rule struct {
	// If is the condition to check before applying the rule.
	// If not set, the rule will be applied.
	If  []string
	if_ model.LabelFilters

	// SourceLabel specifies which label's value to use as source text.
	// Default is model.LabelContent.
	SourceLabel string

	// SkipTooShortThreshold is the threshold of the source text length.
	// If the source text is shorter than this threshold, it will be skipped.
	SkipTooShortThreshold *int

	// Transform used to transform the source text.
	// If not set, transform to original source text.
	Transform *Transform

	// Match used to match the text after transform.
	// If not set, match all.
	Match   string
	matchRE *regexp.Regexp

	// Action determines what to do if matchs.
	Action Action

	// Label is the label to create or update.
	Label string
}

func (r *Rule) Validate() error { //nolint:cyclop,gocognit,funlen
	// If.
	if len(r.If) > 0 {
		if_, err := model.NewLabelFilters(r.If)
		if err != nil {
			return errors.Wrapf(err, "invalid if %q", r.If)
		}
		r.if_ = if_
	}

	// Source label.
	if r.SourceLabel == "" {
		r.SourceLabel = model.LabelContent
	}
	if r.SkipTooShortThreshold == nil {
		r.SkipTooShortThreshold = ptr.To(0)
	}

	// Transform.
	if r.Transform != nil { //nolint:nestif
		if r.Transform.ToText != nil && r.Transform.ToPodcast != nil {
			return errors.New("to_text and to_podcast can not be set at same time")
		}
		if r.Transform.ToText == nil && r.Transform.ToPodcast == nil {
			return errors.New("either to_text or to_podcast must be set when transform is set")
		}

		if r.Transform.ToText != nil {
			switch r.Transform.ToText.Type {
			case ToTextTypePrompt:
				if r.Transform.ToText.Prompt == "" {
					return errors.New("to text prompt is required for prompt type")
				}
				tmpl, err := template.New("").Parse(r.Transform.ToText.Prompt)
				if err != nil {
					return errors.Wrapf(err, "parse prompt template %s", r.Transform.ToText.Prompt)
				}

				buf := buffer.Get()
				defer buffer.Put(buf)
				if err := tmpl.Execute(buf, prompt.Builtin); err != nil {
					return errors.Wrapf(err, "execute prompt template %s", r.Transform.ToText.Prompt)
				}
				r.Transform.ToText.promptRendered = buf.String()

			case ToTextTypeCrawl, ToTextTypeCrawlByJina:
				// No specific validation for crawl type here, as the source text itself is the URL.
			default:
				return errors.Errorf("unknown transform type: %s", r.Transform.ToText.Type)
			}
		}

		if r.Transform.ToPodcast != nil {
			if len(r.Transform.ToPodcast.Speakers) == 0 {
				return errors.New("at least one speaker is required for to_podcast")
			}

			r.Transform.ToPodcast.speakers = make([]llm.Speaker, len(r.Transform.ToPodcast.Speakers))
			var speakerDescs []string
			var speakerNames []string
			for i, s := range r.Transform.ToPodcast.Speakers {
				if s.Name == "" {
					return errors.New("speaker name is required")
				}
				if s.Voice == "" {
					return errors.New("speaker voice is required")
				}
				r.Transform.ToPodcast.speakers[i] = llm.Speaker{Name: s.Name, Voice: s.Voice}

				desc := s.Name
				if s.Role != "" {
					desc += " (" + s.Role + ")"
				}
				speakerDescs = append(speakerDescs, desc)
				speakerNames = append(speakerNames, s.Name)
			}

			speakersDesc := "- " + strings.Join(speakerDescs, "\n- ")
			exampleSpeaker1 := speakerNames[0]
			exampleSpeaker2 := exampleSpeaker1
			if len(speakerNames) > 1 {
				exampleSpeaker2 = speakerNames[1]
			}

			promptSegments := []string{
				"Please convert the following article into a podcast dialogue script.",
				"The speakers are:\n" + speakersDesc,
			}

			if r.Transform.ToPodcast.EstimateMaximumDuration > 0 {
				wordsPerMinute := 200
				totalMinutes := int(r.Transform.ToPodcast.EstimateMaximumDuration.Minutes())
				estimatedWords := totalMinutes * wordsPerMinute
				promptSegments = append(promptSegments, fmt.Sprintf("The script should be approximately %d words to fit within a %d-minute duration. If the original content is not sufficient, the script can be shorter as appropriate.", estimatedWords, totalMinutes))
			}

			if r.Transform.ToPodcast.TranscriptAdditionalPrompt != "" {
				promptSegments = append(promptSegments, "Additional instructions: "+r.Transform.ToPodcast.TranscriptAdditionalPrompt)
			}

			promptSegments = append(promptSegments,
				"The output format MUST be a script where each line starts with the speaker's name followed by a colon and a space.",
				"Do NOT include any other text, explanations, or formatting before or after the script.",
				"Do NOT use background music in the script.",
				"Do NOT include any greetings or farewells (e.g., 'Hello everyone', 'Welcome to our show', 'Goodbye').",
				fmt.Sprintf("Example of the required format:\n%s: Today we are discussing the article's main points.\n%s: Let's start with the first one.", exampleSpeaker1, exampleSpeaker2),
				"Now, convert the article.",
			)

			r.Transform.ToPodcast.transcriptPrompt = strings.Join(promptSegments, "\n\n")
			r.Transform.ToPodcast.speakersDesc = speakersDesc
		}
	}

	// Match.
	if r.Match == "" {
		r.Match = ".*"
	}
	re, err := regexp.Compile(r.Match)
	if err != nil {
		return errors.Wrapf(err, "compile match regex %s", r.Match)
	}
	r.matchRE = re

	// Action.
	if r.Action == "" {
		r.Action = ActionCreateOrUpdateLabel
	}
	switch r.Action {
	case ActionCreateOrUpdateLabel:
		if r.Label == "" {
			return errors.New("label is required for create or update label action")
		}
	case ActionDropFeed:
	default:
		return errors.Errorf("invalid action: %s", r.Action)
	}

	return nil
}

func (r *Rule) From(c *config.RewriteRule) {
	r.If = c.If
	r.SourceLabel = c.SourceLabel
	r.SkipTooShortThreshold = c.SkipTooShortThreshold
	if c.Transform != nil { //nolint:nestif
		t := &Transform{}
		if c.Transform.ToText != nil {
			toText := &ToText{
				LLM:    c.Transform.ToText.LLM,
				Prompt: c.Transform.ToText.Prompt,
			}
			toText.Type = ToTextType(c.Transform.ToText.Type)
			if toText.Type == "" {
				toText.Type = ToTextTypePrompt // Default to prompt if not specified.
			}
			t.ToText = toText
		}
		if c.Transform.ToPodcast != nil {
			toPodcast := &ToPodcast{
				LLM:                        c.Transform.ToPodcast.LLM,
				EstimateMaximumDuration:    time.Duration(c.Transform.ToPodcast.EstimateMaximumDuration),
				TranscriptAdditionalPrompt: c.Transform.ToPodcast.TranscriptAdditionalPrompt,
				TTSLLM:                     c.Transform.ToPodcast.TTSLLM,
			}
			if toPodcast.EstimateMaximumDuration == 0 {
				toPodcast.EstimateMaximumDuration = 3 * time.Minute
			}
			for _, s := range c.Transform.ToPodcast.Speakers {
				toPodcast.Speakers = append(toPodcast.Speakers, Speaker{
					Name:  s.Name,
					Role:  s.Role,
					Voice: s.Voice,
				})
			}
			t.ToPodcast = toPodcast
		}
		r.Transform = t
	}
	r.Match = c.Match
	if r.Match == "" {
		r.Match = c.MatchRE
	}
	r.Action = Action(c.Action)
	r.Label = c.Label
}

type Transform struct {
	ToText    *ToText
	ToPodcast *ToPodcast
}

type ToText struct {
	Type ToTextType

	// LLM is the name of the LLM to use.
	// Only used when Type is ToTextTypePrompt.
	LLM string

	// Prompt is the prompt for LLM completion.
	// The source text will automatically be injected into the prompt.
	// Only used when Type is ToTextTypePrompt.
	Prompt         string
	promptRendered string
}

type ToPodcast struct {
	LLM                        string
	EstimateMaximumDuration    time.Duration
	TranscriptAdditionalPrompt string
	TTSLLM                     string
	Speakers                   []Speaker

	transcriptPrompt string
	speakersDesc     string
	speakers         []llm.Speaker
}

type Speaker struct {
	Name  string
	Role  string
	Voice string
}

type ToTextType string

const (
	ToTextTypePrompt      ToTextType = "prompt"
	ToTextTypeCrawl       ToTextType = "crawl"
	ToTextTypeCrawlByJina ToTextType = "crawl_by_jina"
)

type Action string

const (
	ActionDropFeed            Action = "drop_feed"
	ActionCreateOrUpdateLabel Action = "create_or_update_label"
)

// --- Factory code block ---
type Factory component.Factory[Rewriter, config.App, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Rewriter, config.App, Dependencies](func(instance string, app *config.App, dependencies Dependencies) (Rewriter, error) {
			m := &mockRewriter{}
			component.MockOptions(mockOn).Apply(&m.Mock)

			return m, nil
		})
	}

	return component.FactoryFunc[Rewriter, config.App, Dependencies](new)
}

func new(instance string, app *config.App, dependencies Dependencies) (Rewriter, error) {
	c := &Config{}
	c.From(app)
	if err := c.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate and adjust rewrite config")
	}

	return &rewriter{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "Rewriter",
			Instance:     instance,
			Config:       c,
			Dependencies: dependencies,
		}),
		crawler:     crawl.NewLocal(),
		jinaCrawler: crawl.NewJina(app.Jina.Token),
	}, nil
}

// --- Implementation code block ---

type rewriter struct {
	*component.Base[Config, Dependencies]

	crawler     crawl.Crawler
	jinaCrawler crawl.Crawler
}

func (r *rewriter) Reload(app *config.App) error {
	newConfig := &Config{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "validate and adjust rewrite config")
	}
	r.SetConfig(newConfig)

	r.jinaCrawler = crawl.NewJina(app.Jina.Token)

	return nil
}

func (r *rewriter) Labels(ctx context.Context, labels model.Labels) (rewritten model.Labels, err error) {
	ctx = telemetry.StartWith(ctx, append(r.TelemetryLabels(), telemetrymodel.KeyOperation, "Labels")...)
	defer func() { telemetry.End(ctx, err) }()

	rules := *r.Config()
	for _, rule := range rules {
		// If.
		if !rule.if_.Match(labels) {
			continue
		}

		// Get source text based on source label.
		sourceText := labels.Get(rule.SourceLabel)
		if utf8.RuneCountInString(sourceText) < *rule.SkipTooShortThreshold {
			continue
		}

		// Transform text if configured.
		text, err := r.transform(ctx, rule.Transform, sourceText)
		if err != nil {
			return nil, errors.Wrap(err, "transform")
		}

		// Check if text matches the rule.
		if !rule.matchRE.MatchString(text) {
			continue
		}

		// Handle actions.
		switch rule.Action {
		case ActionDropFeed:
			return nil, nil
		case ActionCreateOrUpdateLabel:
			labels.Put(rule.Label, text, false)
		}
	}

	labels.EnsureSorted()

	return labels, nil
}

func (r *rewriter) transform(ctx context.Context, transform *Transform, sourceText string) (string, error) {
	if transform == nil {
		return sourceText, nil
	}

	if transform.ToText != nil {
		return r.transformText(ctx, transform.ToText, sourceText)
	}

	if transform.ToPodcast != nil {
		return r.transformPodcast(ctx, transform.ToPodcast, sourceText)
	}

	return sourceText, nil
}

// transformText transforms text using configured LLM or by crawling a URL.
func (r *rewriter) transformText(ctx context.Context, toText *ToText, text string) (string, error) {
	switch toText.Type {
	case ToTextTypeCrawl:
		return r.transformTextCrawl(ctx, r.crawler, text)
	case ToTextTypeCrawlByJina:
		return r.transformTextCrawl(ctx, r.jinaCrawler, text)

	case ToTextTypePrompt:
		return r.transformTextPrompt(ctx, toText, text)
	default:
		return r.transformTextPrompt(ctx, toText, text)
	}
}

func (r *rewriter) transformTextCrawl(ctx context.Context, crawler crawl.Crawler, url string) (string, error) {
	mdBytes, err := crawler.Markdown(ctx, url)
	if err != nil {
		return "", errors.Wrapf(err, "crawl %s", url)
	}

	return string(mdBytes), nil
}

// transformTextPrompt transforms text using configured LLM.
func (r *rewriter) transformTextPrompt(ctx context.Context, toText *ToText, text string) (string, error) {
	// Get LLM instance.
	llm := r.Dependencies().LLMFactory.Get(toText.LLM)

	// Call completion.
	result, err := llm.String(ctx, []string{
		toText.promptRendered,
		text, // TODO: may place to first line to hit the model cache in different rewrite rules.
	})
	if err != nil {
		return "", errors.Wrap(err, "llm completion")
	}

	return r.transformTextHack(result), nil
}

func (r *rewriter) transformTextHack(text string) string {
	// TODO: optimize this.
	text = strings.ReplaceAll(text, "```html", "")
	text = strings.ReplaceAll(text, "```markdown", "")
	text = strings.ReplaceAll(text, "```", "")

	return text
}

var audioKey = func(transcript, ext string) string {
	hash := hashutil.Sum64(transcript)
	file := strconv.FormatUint(hash, 10) + "." + ext

	return "podcasts/" + file
}

func (r *rewriter) transformPodcast(ctx context.Context, toPodcast *ToPodcast, sourceText string) (url string, err error) {
	transcript, err := r.generateTranscript(ctx, toPodcast, sourceText)
	if err != nil {
		return "", errors.Wrap(err, "generate podcast transcript")
	}

	audioKey := audioKey(transcript, "wav")
	url, err = r.Dependencies().ObjectStorage.Get(ctx, audioKey)
	switch {
	case err == nil:
		// May canceled at last time by reload, fast return.
		return url, nil
	case errors.Is(err, object.ErrNotFound):
		// Not found, generate new audio.
	default:
		return "", errors.Wrap(err, "get audio")
	}

	audioStream, err := r.generateAudio(ctx, toPodcast, transcript)
	if err != nil {
		return "", errors.Wrap(err, "generate podcast audio")
	}
	defer func() {
		if closeErr := audioStream.Close(); closeErr != nil {
			err = errors.Wrap(err, "close audio stream")
		}
	}()

	url, err = r.Dependencies().ObjectStorage.Put(ctx, audioKey, audioStream, "audio/wav")
	if err != nil {
		return "", errors.Wrap(err, "store podcast audio")
	}

	return url, nil
}

func (r *rewriter) generateTranscript(ctx context.Context, toPodcast *ToPodcast, sourceText string) (string, error) {
	transcript, err := r.Dependencies().LLMFactory.Get(toPodcast.LLM).
		String(ctx, []string{toPodcast.transcriptPrompt, sourceText})
	if err != nil {
		return "", errors.Wrap(err, "llm completion")
	}

	return toPodcast.speakersDesc +
		"\n\nFollowed by the actual dialogue script:\n" +
		transcript, nil
}

func (r *rewriter) generateAudio(ctx context.Context, toPodcast *ToPodcast, transcript string) (io.ReadCloser, error) {
	audioStream, err := r.Dependencies().LLMFactory.Get(toPodcast.TTSLLM).
		WAV(ctx, transcript, toPodcast.speakers)
	if err != nil {
		return nil, errors.Wrap(err, "calling tts llm")
	}

	return audioStream, nil
}

type mockRewriter struct {
	component.Mock
}

func (r *mockRewriter) Reload(app *config.App) error {
	args := r.Called(app)

	return args.Error(0)
}

func (r *mockRewriter) Labels(ctx context.Context, labels model.Labels) (model.Labels, error) {
	args := r.Called(ctx, labels)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(model.Labels), args.Error(1)
}
