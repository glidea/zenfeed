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
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"

	"github.com/pkg/errors"
	oai "github.com/sashabaranov/go-openai"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	"github.com/glidea/zenfeed/pkg/util/wav"
)

type gemini struct {
	*component.Base[Config, struct{}]
	text
	hc *http.Client

	embeddingSpliter embeddingSpliter
}

func newGemini(c *Config) LLM {
	config := oai.DefaultConfig(c.APIKey)
	config.BaseURL = filepath.Join(c.Endpoint, "openai") // OpenAI compatible endpoint.
	client := oai.NewClientWithConfig(config)
	embeddingSpliter := newEmbeddingSpliter(1536, 64)

	base := component.New(&component.BaseConfig[Config, struct{}]{
		Name:     "LLM/gemini",
		Instance: c.Name,
		Config:   c,
	})

	return &gemini{
		Base: base,
		text: &openaiText{
			Base:   base,
			client: client,
		},
		hc:               &http.Client{},
		embeddingSpliter: embeddingSpliter,
	}
}

func (g *gemini) WAV(ctx context.Context, text string, speakers []Speaker) (r io.ReadCloser, err error) {
	ctx = telemetry.StartWith(ctx, append(g.TelemetryLabels(), telemetrymodel.KeyOperation, "WAV")...)
	defer func() { telemetry.End(ctx, err) }()

	if g.Config().TTSModel == "" {
		return nil, errors.New("tts model is not set")
	}

	reqPayload, err := buildWAVRequestPayload(text, speakers)
	if err != nil {
		return nil, errors.Wrap(err, "build wav request payload")
	}

	pcmData, err := g.doWAVRequest(ctx, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "do wav request")
	}

	return streamWAV(pcmData), nil
}

func (g *gemini) doWAVRequest(ctx context.Context, reqPayload *geminiRequest) ([]byte, error) {
	config := g.Config()
	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "marshal tts request")
	}

	url := config.Endpoint + "/models/" + config.TTSModel + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, "new tts request")
	}
	req.Header.Set("x-goog-api-key", config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.hc.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "do tts request")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := io.ReadAll(resp.Body)

		return nil, errors.Errorf("tts request failed with status %d: %s", resp.StatusCode, string(errMsg))
	}

	var ttsResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&ttsResp); err != nil {
		return nil, errors.Wrap(err, "decode tts response")
	}
	if len(ttsResp.Candidates) == 0 || len(ttsResp.Candidates[0].Content.Parts) == 0 || ttsResp.Candidates[0].Content.Parts[0].InlineData == nil {
		return nil, errors.New("no audio data in tts response")
	}

	audioDataB64 := ttsResp.Candidates[0].Content.Parts[0].InlineData.Data
	pcmData, err := base64.StdEncoding.DecodeString(audioDataB64)
	if err != nil {
		return nil, errors.Wrap(err, "decode base64")
	}

	return pcmData, nil
}

func buildWAVRequestPayload(text string, speakers []Speaker) (*geminiRequest, error) {
	reqPayload := geminiRequest{
		Contents: []*geminiRequestContent{{Parts: []*geminiRequestPart{{Text: text}}}},
		Config: &geminiRequestConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig:       &geminiRequestSpeechConfig{},
		},
	}

	switch len(speakers) {
	case 0:
		return nil, errors.New("no speakers")
	case 1:
		reqPayload.Config.SpeechConfig.VoiceConfig = &geminiRequestVoiceConfig{
			PrebuiltVoiceConfig: &geminiRequestPrebuiltVoiceConfig{VoiceName: speakers[0].Voice},
		}
	default:
		multiSpeakerConfig := &geminiRequestMultiSpeakerVoiceConfig{}
		for _, s := range speakers {
			multiSpeakerConfig.SpeakerVoiceConfigs = append(multiSpeakerConfig.SpeakerVoiceConfigs, &geminiRequestSpeakerVoiceConfig{
				Speaker: s.Name,
				VoiceConfig: &geminiRequestVoiceConfig{
					PrebuiltVoiceConfig: &geminiRequestPrebuiltVoiceConfig{VoiceName: s.Voice},
				},
			})
		}
		reqPayload.Config.SpeechConfig.MultiSpeakerVoiceConfig = multiSpeakerConfig
	}

	return &reqPayload, nil
}

func streamWAV(pcmData []byte) io.ReadCloser {
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		defer func() { _ = pipeWriter.Close() }()
		if err := wav.WriteHeader(pipeWriter, geminiWavHeader, uint32(len(pcmData))); err != nil {
			pipeWriter.CloseWithError(errors.Wrap(err, "write wav header"))

			return
		}
		if _, err := io.Copy(pipeWriter, bytes.NewReader(pcmData)); err != nil {
			pipeWriter.CloseWithError(errors.Wrap(err, "write pcm data"))

			return
		}
	}()

	return pipeReader
}

var geminiWavHeader = &wav.Header{
	SampleRate:  24000,
	BitDepth:    16,
	NumChannels: 1,
}

type geminiRequest struct {
	Contents []*geminiRequestContent `json:"contents"`
	Config   *geminiRequestConfig    `json:"generationConfig"`
}

type geminiRequestContent struct {
	Parts []*geminiRequestPart `json:"parts"`
}

type geminiRequestPart struct {
	Text string `json:"text"`
}

type geminiRequestConfig struct {
	ResponseModalities []string                   `json:"responseModalities"`
	SpeechConfig       *geminiRequestSpeechConfig `json:"speechConfig"`
}

type geminiRequestSpeechConfig struct {
	VoiceConfig             *geminiRequestVoiceConfig             `json:"voiceConfig,omitempty"`
	MultiSpeakerVoiceConfig *geminiRequestMultiSpeakerVoiceConfig `json:"multiSpeakerVoiceConfig,omitempty"`
}

type geminiRequestVoiceConfig struct {
	PrebuiltVoiceConfig *geminiRequestPrebuiltVoiceConfig `json:"prebuiltVoiceConfig,omitempty"`
}

type geminiRequestPrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName,omitempty"`
}

type geminiRequestMultiSpeakerVoiceConfig struct {
	SpeakerVoiceConfigs []*geminiRequestSpeakerVoiceConfig `json:"speakerVoiceConfigs,omitempty"`
}

type geminiRequestSpeakerVoiceConfig struct {
	Speaker     string                    `json:"speaker,omitempty"`
	VoiceConfig *geminiRequestVoiceConfig `json:"voiceConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []*geminiResponseCandidate `json:"candidates"`
}

type geminiResponseCandidate struct {
	Content *geminiResponseContent `json:"content"`
}

type geminiResponseContent struct {
	Parts []*geminiResponsePart `json:"parts"`
}

type geminiResponsePart struct {
	InlineData *geminiResponseInlineData `json:"inlineData"`
}

type geminiResponseInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // Base64 encoded.
}
