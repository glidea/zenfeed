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
	"io"

	"github.com/pkg/errors"

	binaryutil "github.com/glidea/zenfeed/pkg/util/binary"
)

// Header contains the WAV header information.
type Header struct {
	SampleRate  uint32
	BitDepth    uint16
	NumChannels uint16
}

// WriteHeader writes the WAV header to a writer.
// pcmDataSize is the size of the raw PCM data.
func WriteHeader(w io.Writer, h *Header, pcmDataSize uint32) error {
	// RIFF Header.
	if err := writeRIFFHeader(w, pcmDataSize); err != nil {
		return errors.Wrap(err, "write RIFF header")
	}

	// fmt chunk.
	if err := writeFMTChunk(w, h); err != nil {
		return errors.Wrap(err, "write fmt chunk")
	}

	// data chunk.
	if _, err := w.Write([]byte("data")); err != nil {
		return errors.Wrap(err, "write data chunk marker")
	}
	if err := binaryutil.WriteUint32(w, pcmDataSize); err != nil {
		return errors.Wrap(err, "write pcm data size")
	}

	return nil
}

func writeRIFFHeader(w io.Writer, pcmDataSize uint32) error {
	if _, err := w.Write([]byte("RIFF")); err != nil {
		return errors.Wrap(err, "write RIFF")
	}
	if err := binaryutil.WriteUint32(w, uint32(36+pcmDataSize)); err != nil {
		return errors.Wrap(err, "write file size")
	}
	if _, err := w.Write([]byte("WAVE")); err != nil {
		return errors.Wrap(err, "write WAVE")
	}

	return nil
}

func writeFMTChunk(w io.Writer, h *Header) error {
	if _, err := w.Write([]byte("fmt ")); err != nil {
		return errors.Wrap(err, "write fmt")
	}
	if err := binaryutil.WriteUint32(w, uint32(16)); err != nil { // PCM chunk size.
		return errors.Wrap(err, "write pcm chunk size")
	}
	if err := binaryutil.WriteUint16(w, uint16(1)); err != nil { // PCM format.
		return errors.Wrap(err, "write pcm format")
	}
	if err := binaryutil.WriteUint16(w, h.NumChannels); err != nil {
		return errors.Wrap(err, "write num channels")
	}
	if err := binaryutil.WriteUint32(w, h.SampleRate); err != nil {
		return errors.Wrap(err, "write sample rate")
	}
	byteRate := h.SampleRate * uint32(h.NumChannels) * uint32(h.BitDepth) / 8
	if err := binaryutil.WriteUint32(w, byteRate); err != nil {
		return errors.Wrap(err, "write byte rate")
	}
	blockAlign := h.NumChannels * h.BitDepth / 8
	if err := binaryutil.WriteUint16(w, blockAlign); err != nil {
		return errors.Wrap(err, "write block align")
	}
	if err := binaryutil.WriteUint16(w, h.BitDepth); err != nil {
		return errors.Wrap(err, "write bit depth")
	}

	return nil
}
