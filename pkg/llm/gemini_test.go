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
	"strings"
	"testing"

	"github.com/glidea/zenfeed/pkg/util/wav"
)

type geminiWAVRoundTripper func(*http.Request) (*http.Response, error)

func (f geminiWAVRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGeminiWAVReturnsOneHeaderForBothResponseFormats(t *testing.T) {
	rawPCM := []byte{1, 0, 2, 0}
	var completeWAV bytes.Buffer
	if err := wav.WriteHeader(&completeWAV, geminiWavHeader, uint32(len(rawPCM))); err != nil {
		t.Fatal(err)
	}
	completeWAV.Write(rawPCM)

	for _, tc := range []struct {
		model    string
		mimeType string
		audio    []byte
	}{
		{"gemini-2.5-flash-preview-tts", "audio/L16;codec=pcm;rate=24000", rawPCM},
		{"gemini-3.8-flash-tts", "audio/wav", completeWAV.Bytes()},
	} {
		t.Run(tc.model, func(t *testing.T) {
			g := newGemini(&Config{
				Endpoint: "https://generativelanguage.googleapis.com/v1beta",
				TTSModel: tc.model,
			}).(*gemini)
			g.hc.Transport = geminiWAVRoundTripper(func(req *http.Request) (*http.Response, error) {
				if got, want := req.URL.Path, "/v1beta/models/"+tc.model+":generateContent"; got != want {
					t.Errorf("unexpected request path: got %q, want %q", got, want)
				}

				if !strings.Contains(req.Header.Get("Content-Type"), "application/json") {
					t.Error("request does not advertise JSON")
				}

				response := geminiResponse{
					Candidates: []*geminiResponseCandidate{{
						Content: &geminiResponseContent{
							Parts: []*geminiResponsePart{{
								InlineData: &geminiResponseInlineData{
									MimeType: tc.mimeType,
									Data:     base64.StdEncoding.EncodeToString(tc.audio),
								},
							}},
						},
					}},
				}
				body, err := json.Marshal(response)
				if err != nil {
					return nil, err
				}

				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}, nil
			})

			r, err := g.WAV(context.Background(), "Hello", []Speaker{{Name: "Narrator", Voice: "Kore"}})
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()

			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, completeWAV.Bytes()) {
				t.Errorf("expected one WAV header and unchanged samples, got %x", got)
			}
		})
	}
}
