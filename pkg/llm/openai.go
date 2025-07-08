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
	"context"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	oai "github.com/sashabaranov/go-openai"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	runtimeutil "github.com/glidea/zenfeed/pkg/util/runtime"
)

type openai struct {
	*component.Base[Config, struct{}]
	text
}

func newOpenAI(c *Config) LLM {
	config := oai.DefaultConfig(c.APIKey)
	config.BaseURL = c.Endpoint
	client := oai.NewClientWithConfig(config)
	embeddingSpliter := newEmbeddingSpliter(1536, 64)

	base := component.New(&component.BaseConfig[Config, struct{}]{
		Name:     "LLM/openai",
		Instance: c.Name,
		Config:   c,
	})

	return &openai{
		Base: base,
		text: &openaiText{
			Base:             base,
			client:           client,
			embeddingSpliter: embeddingSpliter,
		},
	}
}

func (o *openai) WAV(ctx context.Context, text string, speakers []Speaker) (r io.ReadCloser, err error) {
	return nil, errors.New("not supported")
}

type openaiText struct {
	*component.Base[Config, struct{}]

	client           *oai.Client
	embeddingSpliter embeddingSpliter
}

func (o *openaiText) String(ctx context.Context, messages []string) (value string, err error) {
	ctx = telemetry.StartWith(ctx, append(o.TelemetryLabels(), telemetrymodel.KeyOperation, "String")...)
	defer func() { telemetry.End(ctx, err) }()

	config := o.Config()
	if config.Model == "" {
		return "", errors.New("model is not set")
	}
	msgs := make([]oai.ChatCompletionMessage, 0, len(messages))
	for _, m := range messages {
		msgs = append(msgs, oai.ChatCompletionMessage{
			Role:    oai.ChatMessageRoleUser,
			Content: m,
		})
	}

	req := oai.ChatCompletionRequest{
		Model:       config.Model,
		Messages:    msgs,
		Temperature: config.Temperature,
	}

	resp, err := o.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", errors.Wrap(err, "create chat completion")
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no completion choices returned")
	}

	lvs := []string{o.Name(), o.Instance(), "String"}
	promptTokens.WithLabelValues(lvs...).Add(float64(resp.Usage.PromptTokens))
	completionTokens.WithLabelValues(lvs...).Add(float64(resp.Usage.CompletionTokens))
	totalTokens.WithLabelValues(lvs...).Add(float64(resp.Usage.TotalTokens))

	return resp.Choices[0].Message.Content, nil
}

func (o *openaiText) EmbeddingLabels(ctx context.Context, labels model.Labels) (value [][]float32, err error) {
	ctx = telemetry.StartWith(ctx, append(o.TelemetryLabels(), telemetrymodel.KeyOperation, "EmbeddingLabels")...)
	defer func() { telemetry.End(ctx, err) }()

	config := o.Config()
	if config.EmbeddingModel == "" {
		return nil, errors.New("embedding model is not set")
	}
	splits, err := o.embeddingSpliter.Split(labels)
	if err != nil {
		return nil, errors.Wrap(err, "split embedding")
	}

	vecs := make([][]float32, 0, len(splits))
	for _, split := range splits {
		text := runtimeutil.Must1(json.Marshal(split))
		vec, err := o.Embedding(ctx, string(text))
		if err != nil {
			return nil, errors.Wrap(err, "embedding")
		}
		vecs = append(vecs, vec)
	}

	return vecs, nil
}

func (o *openaiText) Embedding(ctx context.Context, s string) (value []float32, err error) {
	ctx = telemetry.StartWith(ctx, append(o.TelemetryLabels(), telemetrymodel.KeyOperation, "Embedding")...)
	defer func() { telemetry.End(ctx, err) }()

	config := o.Config()
	if config.EmbeddingModel == "" {
		return nil, errors.New("embedding model is not set")
	}
	vec, err := o.client.CreateEmbeddings(ctx, oai.EmbeddingRequest{
		Input:          []string{s},
		Model:          oai.EmbeddingModel(config.EmbeddingModel),
		EncodingFormat: oai.EmbeddingEncodingFormatFloat,
	})
	if err != nil {
		return nil, errors.Wrap(err, "create embeddings")
	}
	if len(vec.Data) == 0 {
		return nil, errors.New("no embedding data returned")
	}

	lvs := []string{o.Name(), o.Instance(), "Embedding"}
	promptTokens.WithLabelValues(lvs...).Add(float64(vec.Usage.PromptTokens))
	completionTokens.WithLabelValues(lvs...).Add(float64(vec.Usage.CompletionTokens))
	totalTokens.WithLabelValues(lvs...).Add(float64(vec.Usage.TotalTokens))

	return vec.Data[0].Embedding, nil
}
