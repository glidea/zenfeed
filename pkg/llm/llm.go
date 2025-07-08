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

package llm

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/mock"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/config"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/storage/kv"
	"github.com/glidea/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	binaryutil "github.com/glidea/zenfeed/pkg/util/binary"
	"github.com/glidea/zenfeed/pkg/util/buffer"
	"github.com/glidea/zenfeed/pkg/util/hash"
)

// --- Interface code block ---
type LLM interface {
	component.Component
	text
	audio
}

type text interface {
	String(ctx context.Context, messages []string) (string, error)
	EmbeddingLabels(ctx context.Context, labels model.Labels) ([][]float32, error)
	Embedding(ctx context.Context, text string) ([]float32, error)
}

type audio interface {
	WAV(ctx context.Context, text string, speakers []Speaker) (io.ReadCloser, error)
}

type Speaker struct {
	Name  string
	Voice string
}

type Config struct {
	Name                            string
	Default                         bool
	Provider                        ProviderType
	Endpoint                        string
	APIKey                          string
	Model, EmbeddingModel, TTSModel string
	Temperature                     float32
}

type ProviderType string

const (
	ProviderTypeOpenAI      ProviderType = "openai"
	ProviderTypeOpenRouter  ProviderType = "openrouter"
	ProviderTypeDeepSeek    ProviderType = "deepseek"
	ProviderTypeGemini      ProviderType = "gemini"
	ProviderTypeVolc        ProviderType = "volc" // Rename MaaS to ARK. 😄
	ProviderTypeSiliconFlow ProviderType = "siliconflow"
)

var defaultEndpoints = map[ProviderType]string{
	ProviderTypeOpenAI:      "https://api.openai.com/v1",
	ProviderTypeOpenRouter:  "https://openrouter.ai/api/v1",
	ProviderTypeDeepSeek:    "https://api.deepseek.com/v1",
	ProviderTypeGemini:      "https://generativelanguage.googleapis.com/v1beta",
	ProviderTypeVolc:        "https://ark.cn-beijing.volces.com/api/v3",
	ProviderTypeSiliconFlow: "https://api.siliconflow.cn/v1",
}

func (c *Config) Validate() error { //nolint:cyclop
	if c.Name == "" {
		return errors.New("name is required")
	}

	switch c.Provider {
	case "":
		c.Provider = ProviderTypeOpenAI
	case ProviderTypeOpenAI, ProviderTypeOpenRouter, ProviderTypeDeepSeek,
		ProviderTypeGemini, ProviderTypeVolc, ProviderTypeSiliconFlow:
	default:
		return errors.Errorf("invalid provider: %s", c.Provider)
	}

	if c.Endpoint == "" {
		c.Endpoint = defaultEndpoints[c.Provider]
	}
	if c.APIKey == "" {
		return errors.New("api key is required")
	}
	if c.Model == "" && c.EmbeddingModel == "" && c.TTSModel == "" {
		return errors.New("model or embedding model or tts model is required")
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		return errors.Errorf("invalid temperature: %f, should be in range [0, 2]", c.Temperature)
	}

	return nil
}

var (
	promptTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: model.AppName,
			Subsystem: "llm",
			Name:      "prompt_tokens",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, telemetrymodel.KeyOperation},
	)
	completionTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: model.AppName,
			Subsystem: "llm",
			Name:      "completion_tokens",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, telemetrymodel.KeyOperation},
	)
	totalTokens = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: model.AppName,
			Subsystem: "llm",
			Name:      "total_tokens",
		},
		[]string{telemetrymodel.KeyComponent, telemetrymodel.KeyComponentInstance, telemetrymodel.KeyOperation},
	)
)

// --- Factory code block ---
type FactoryConfig struct {
	LLMs       []Config
	defaultLLM string
}

func (c *FactoryConfig) Validate() error {
	if len(c.LLMs) == 0 {
		return errors.New("no llm config")
	}

	for i := range c.LLMs {
		if err := (&c.LLMs[i]).Validate(); err != nil {
			return errors.Wrapf(err, "validate llm config %s", c.LLMs[i].Name)
		}
	}

	if len(c.LLMs) == 1 {
		c.LLMs[0].Default = true
		c.defaultLLM = c.LLMs[0].Name

		return nil
	}

	defaults := 0
	for _, llm := range c.LLMs {
		if llm.Default {
			c.defaultLLM = llm.Name
			defaults++
		}
	}
	if defaults > 1 {
		return errors.New("multiple llm configs are default")
	}

	return nil
}

func (c *FactoryConfig) From(app *config.App) {
	for _, llm := range app.LLMs {
		c.LLMs = append(c.LLMs, Config{
			Name:           llm.Name,
			Default:        llm.Default,
			Provider:       ProviderType(llm.Provider),
			Endpoint:       llm.Endpoint,
			APIKey:         llm.APIKey,
			Model:          llm.Model,
			EmbeddingModel: llm.EmbeddingModel,
			TTSModel:       llm.TTSModel,
			Temperature:    llm.Temperature,
		})
	}
}

type FactoryDependencies struct {
	KVStorage kv.Storage
}

// Factory is a factory for creating LLM instances.
// If name is empty or not found, it will return the default.
type Factory interface {
	component.Component
	config.Watcher
	Get(name string) LLM
}

func NewFactory(
	instance string,
	app *config.App,
	dependencies FactoryDependencies,
	mockOn ...component.MockOption,
) (Factory, error) {
	if len(mockOn) > 0 {
		mf := &mockFactory{}
		m := &mockLLM{}
		component.MockOptions(mockOn).Apply(&m.Mock)
		mf.On("Get", mock.Anything).Return(m)
		mf.On("Reload", mock.Anything).Return(nil)

		return mf, nil
	}

	config := &FactoryConfig{}
	config.From(app)
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}
	f := &factory{
		Base: component.New(&component.BaseConfig[FactoryConfig, FactoryDependencies]{
			Name:         "LLMFactory",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		llms: make(map[string]LLM),
	}
	f.initLLMs()

	return f, nil
}

type factory struct {
	*component.Base[FactoryConfig, FactoryDependencies]

	defaultLLM LLM
	llms       map[string]LLM
	mu         sync.Mutex
}

func (f *factory) Run() error {
	for _, llm := range f.llms {
		if err := component.RunUntilReady(f.Context(), llm, 10*time.Second); err != nil {
			return errors.Wrapf(err, "run llm %s", llm.Name())
		}
	}
	f.MarkReady()
	<-f.Context().Done()

	return nil
}

func (f *factory) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, llm := range f.llms {
		_ = llm.Close()
	}

	return nil
}

func (f *factory) Reload(app *config.App) error {
	newConfig := &FactoryConfig{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "validate config")
	}
	if reflect.DeepEqual(f.Config(), newConfig) {
		log.Debug(f.Context(), "no changes in llm config")

		return nil
	}

	// Reload the LLMs.
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SetConfig(newConfig)

	// Close the old LLMs.
	for _, llm := range f.llms {
		_ = llm.Close()
	}

	// Recreate the LLMs.
	f.initLLMs()

	return nil
}

func (f *factory) Get(name string) LLM {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name == "" {
		return f.defaultLLM
	}

	for _, llmC := range f.Config().LLMs {
		if llmC.Name != name {
			continue
		}

		return f.llms[name]
	}

	return f.defaultLLM
}

func (f *factory) new(c *Config) LLM {
	switch c.Provider {
	case ProviderTypeOpenAI, ProviderTypeOpenRouter, ProviderTypeDeepSeek, ProviderTypeVolc, ProviderTypeSiliconFlow: //nolint:lll
		return newCached(newOpenAI(c), f.Dependencies().KVStorage)

	case ProviderTypeGemini:
		return newCached(newGemini(c), f.Dependencies().KVStorage)

	default:
		return newCached(newOpenAI(c), f.Dependencies().KVStorage)
	}
}

func (f *factory) initLLMs() {
	var (
		config     = f.Config()
		llms       = make(map[string]LLM, len(config.LLMs))
		defaultLLM LLM
	)

	for _, llmC := range config.LLMs {
		llm := f.new(&llmC)

		llms[llmC.Name] = llm

		if llmC.Name == config.defaultLLM {
			defaultLLM = llm
		}
	}

	f.llms = llms
	f.defaultLLM = defaultLLM
}

type mockFactory struct {
	component.Mock
}

func (m *mockFactory) Get(name string) LLM {
	args := m.Called(name)

	return args.Get(0).(LLM)
}

func (m *mockFactory) Reload(app *config.App) error {
	args := m.Called(app)

	return args.Error(0)
}

// --- Implementation code block ---
type cached struct {
	LLM
	kvStorage kv.Storage
}

func newCached(llm LLM, kvStorage kv.Storage) LLM {
	return &cached{
		LLM:       llm,
		kvStorage: kvStorage,
	}
}

func (c *cached) String(ctx context.Context, messages []string) (string, error) {
	key := hash.Sum64s(messages)
	keyStr := strconv.FormatUint(key, 10) // for human readable & compatible.

	valueBs, err := c.kvStorage.Get(ctx, []byte(keyStr))
	switch {
	case err == nil:
		return string(valueBs), nil
	case errors.Is(err, kv.ErrNotFound):
		break
	default:
		return "", errors.Wrap(err, "get from kv storage")
	}

	value, err := c.LLM.String(ctx, messages)
	if err != nil {
		return "", err
	}

	// TODO: reduce copies.
	if err = c.kvStorage.Set(ctx, []byte(keyStr), []byte(value), 65*time.Minute); err != nil {
		log.Error(ctx, err, "set to kv storage")
	}

	return value, nil
}

var (
	toBytes = func(v []float32) ([]byte, error) {
		buf := buffer.Get()
		defer buffer.Put(buf)

		for _, fVal := range v {
			if err := binaryutil.WriteFloat32(buf, fVal); err != nil {
				return nil, errors.Wrap(err, "write float32")
			}
		}

		// Must copy data, as the buffer will be reused.
		bs := make([]byte, buf.Len())
		copy(bs, buf.Bytes())

		return bs, nil
	}

	toF32s = func(bs []byte) ([]float32, error) {
		if len(bs)%4 != 0 {
			return nil, errors.New("embedding data is corrupted, length not multiple of 4")
		}

		r := bytes.NewReader(bs)
		floats := make([]float32, len(bs)/4)

		for i := range floats {
			f, err := binaryutil.ReadFloat32(r)
			if err != nil {
				return nil, errors.Wrap(err, "deserialize float32")
			}
			floats[i] = f
		}

		return floats, nil
	}
)

func (c *cached) Embedding(ctx context.Context, text string) ([]float32, error) {
	key := hash.Sum64(text)
	keyStr := strconv.FormatUint(key, 10)

	valueBs, err := c.kvStorage.Get(ctx, []byte(keyStr))
	switch {
	case err == nil:
		return toF32s(valueBs)
	case errors.Is(err, kv.ErrNotFound):
		break
	default:
		return nil, errors.Wrap(err, "get from kv storage")
	}

	value, err := c.LLM.Embedding(ctx, text)
	if err != nil {
		return nil, err
	}

	valueBs, err = toBytes(value)
	if err != nil {
		return nil, errors.Wrap(err, "serialize embedding")
	}

	if err = c.kvStorage.Set(ctx, []byte(keyStr), valueBs, 65*time.Minute); err != nil {
		log.Error(ctx, err, "set to kv storage")
	}

	return value, nil
}

type mockLLM struct {
	component.Mock
}

func (m *mockLLM) String(ctx context.Context, messages []string) (string, error) {
	args := m.Called(ctx, messages)

	return args.Get(0).(string), args.Error(1)
}

func (m *mockLLM) EmbeddingLabels(ctx context.Context, labels model.Labels) ([][]float32, error) {
	args := m.Called(ctx, labels)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([][]float32), args.Error(1)
}

func (m *mockLLM) Embedding(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]float32), args.Error(1)
}

func (m *mockLLM) WAV(ctx context.Context, text string, speakers []Speaker) (io.ReadCloser, error) {
	args := m.Called(ctx, text, speakers)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(io.ReadCloser), args.Error(1)
}
