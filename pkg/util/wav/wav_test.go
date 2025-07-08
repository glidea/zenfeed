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

package wav

import (
	"bytes"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/glidea/zenfeed/pkg/test"
)

func TestWriteHeader(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}

	type whenDetail struct {
		header      *Header
		pcmDataSize uint32
	}

	type thenExpected struct {
		expectedBytes []byte
		expectError   bool
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario:    "Standard CD quality audio",
			Given:       "a header for CD quality audio and a non-zero data size",
			When:        "writing the header",
			Then:        "should produce a valid 44-byte WAV header and no error",
			GivenDetail: givenDetail{},
			WhenDetail: whenDetail{
				header: &Header{
					SampleRate:  44100,
					BitDepth:    16,
					NumChannels: 2,
				},
				pcmDataSize: 176400,
			},
			ThenExpected: thenExpected{
				expectedBytes: []byte{
					'R', 'I', 'F', 'F',
					0x34, 0xB1, 0x02, 0x00, // ChunkSize = 36 + 176400 = 176436
					'W', 'A', 'V', 'E',
					'f', 'm', 't', ' ',
					0x10, 0x00, 0x00, 0x00, // Subchunk1Size = 16
					0x01, 0x00, // AudioFormat = 1 (PCM)
					0x02, 0x00, // NumChannels = 2
					0x44, 0xAC, 0x00, 0x00, // SampleRate = 44100
					0x10, 0xB1, 0x02, 0x00, // ByteRate = 176400
					0x04, 0x00, // BlockAlign = 4
					0x10, 0x00, // BitsPerSample = 16
					'd', 'a', 't', 'a',
					0x10, 0xB1, 0x02, 0x00, // Subchunk2Size = 176400
				},
				expectError: false,
			},
		},
		{
			Scenario:    "Mono audio for speech",
			Given:       "a header for mono speech audio and a non-zero data size",
			When:        "writing the header",
			Then:        "should produce a valid 44-byte WAV header and no error",
			GivenDetail: givenDetail{},
			WhenDetail: whenDetail{
				header: &Header{
					SampleRate:  16000,
					BitDepth:    16,
					NumChannels: 1,
				},
				pcmDataSize: 32000,
			},
			ThenExpected: thenExpected{
				expectedBytes: []byte{
					'R', 'I', 'F', 'F',
					0x24, 0x7D, 0x00, 0x00, // ChunkSize = 36 + 32000 = 32036
					'W', 'A', 'V', 'E',
					'f', 'm', 't', ' ',
					0x10, 0x00, 0x00, 0x00, // Subchunk1Size = 16
					0x01, 0x00, // AudioFormat = 1
					0x01, 0x00, // NumChannels = 1
					0x80, 0x3E, 0x00, 0x00, // SampleRate = 16000
					0x00, 0x7D, 0x00, 0x00, // ByteRate = 32000
					0x02, 0x00, // BlockAlign = 2
					0x10, 0x00, // BitsPerSample = 16
					'd', 'a', 't', 'a',
					0x00, 0x7D, 0x00, 0x00, // Subchunk2Size = 32000
				},
				expectError: false,
			},
		},
		{
			Scenario:    "8-bit mono audio with zero data size",
			Given:       "a header for 8-bit mono audio and a zero data size",
			When:        "writing the header for an empty file",
			Then:        "should produce a valid 44-byte WAV header with data size 0",
			GivenDetail: givenDetail{},
			WhenDetail: whenDetail{
				header: &Header{
					SampleRate:  8000,
					BitDepth:    8,
					NumChannels: 1,
				},
				pcmDataSize: 0,
			},
			ThenExpected: thenExpected{
				expectedBytes: []byte{
					'R', 'I', 'F', 'F',
					0x24, 0x00, 0x00, 0x00, // ChunkSize = 36 + 0 = 36
					'W', 'A', 'V', 'E',
					'f', 'm', 't', ' ',
					0x10, 0x00, 0x00, 0x00, // Subchunk1Size = 16
					0x01, 0x00, // AudioFormat = 1
					0x01, 0x00, // NumChannels = 1
					0x40, 0x1F, 0x00, 0x00, // SampleRate = 8000
					0x40, 0x1F, 0x00, 0x00, // ByteRate = 8000
					0x01, 0x00, // BlockAlign = 1
					0x08, 0x00, // BitsPerSample = 8
					'd', 'a', 't', 'a',
					0x00, 0x00, 0x00, 0x00, // Subchunk2Size = 0
				},
				expectError: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			var buf bytes.Buffer

			// When.
			err := WriteHeader(&buf, tt.WhenDetail.header, tt.WhenDetail.pcmDataSize)

			// Then.
			if tt.ThenExpected.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(buf.Bytes()).To(Equal(tt.ThenExpected.expectedBytes))
			}
		})
	}
}
