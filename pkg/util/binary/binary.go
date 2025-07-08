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

package binary

import (
	"encoding/binary"
	"io"
	"math"
	"sync"

	"github.com/pkg/errors"

	"github.com/glidea/zenfeed/pkg/util/buffer"
)

// WriteString writes a string to a writer.
func WriteString(w io.Writer, str string) error {
	len := len(str)
	if len > math.MaxUint32 {
		return errors.New("length exceeds maximum uint32")
	}

	if err := WriteUint32(w, uint32(len)); err != nil {
		return errors.Wrap(err, "write length")
	}
	if _, err := io.WriteString(w, str); err != nil {
		return errors.Wrap(err, "write data")
	}

	return nil
}

// ReadString reads a string from a reader.
func ReadString(r io.Reader) (string, error) {
	len, err := ReadUint32(r)
	if err != nil {
		return "", errors.Wrap(err, "read length")
	}

	bb := buffer.Get()
	defer buffer.Put(bb)
	// bb.EnsureRemaining(int(len))

	if _, err := io.CopyN(bb, r, int64(len)); err != nil {
		return "", errors.Wrap(err, "read data")
	}

	return bb.String(), nil
}

var smallBufPool = sync.Pool{
	New: func() any {
		// 8 bytes is enough for uint64, uint32, float32.
		b := make([]byte, 8)

		return &b
	},
}

// WriteUint64 writes a uint64 using a pooled buffer.
func WriteUint64(w io.Writer, v uint64) error {
	bp := smallBufPool.Get().(*[]byte)
	defer smallBufPool.Put(bp)
	b := *bp

	binary.LittleEndian.PutUint64(b, v)
	_, err := w.Write(b[:8])

	return err
}

// ReadUint64 reads a uint64 using a pooled buffer.
func ReadUint64(r io.Reader) (uint64, error) {
	bp := smallBufPool.Get().(*[]byte)
	defer smallBufPool.Put(bp)
	b := (*bp)[:8]

	// Read exactly 8 bytes into the slice.
	if _, err := io.ReadFull(r, b); err != nil {
		return 0, errors.Wrap(err, "read uint64")
	}

	return binary.LittleEndian.Uint64(b), nil
}

// WriteUint32 writes a uint32 using a pooled buffer.
func WriteUint32(w io.Writer, v uint32) error {
	bp := smallBufPool.Get().(*[]byte)
	defer smallBufPool.Put(bp)
	b := *bp

	binary.LittleEndian.PutUint32(b, v)
	_, err := w.Write(b[:4])

	return err
}

// ReadUint32 reads a uint32 using a pooled buffer.
func ReadUint32(r io.Reader) (uint32, error) {
	bp := smallBufPool.Get().(*[]byte)
	defer smallBufPool.Put(bp)
	b := (*bp)[:4]

	// Read exactly 4 bytes into the slice.
	if _, err := io.ReadFull(r, b); err != nil {
		return 0, errors.Wrap(err, "read uint32")
	}

	return binary.LittleEndian.Uint32(b), nil
}

// WriteUint16 writes a uint16 using a pooled buffer.
func WriteUint16(w io.Writer, v uint16) error {
	bp := smallBufPool.Get().(*[]byte)
	defer smallBufPool.Put(bp)
	b := *bp

	binary.LittleEndian.PutUint16(b, v)
	_, err := w.Write(b[:2])

	return err
}

// ReadUint16 reads a uint16 using a pooled buffer.
func ReadUint16(r io.Reader) (uint16, error) {
	bp := smallBufPool.Get().(*[]byte)
	defer smallBufPool.Put(bp)
	b := (*bp)[:2]

	// Read exactly 2 bytes into the slice.
	if _, err := io.ReadFull(r, b); err != nil {
		return 0, errors.Wrap(err, "read uint16")
	}

	return binary.LittleEndian.Uint16(b), nil
}

// WriteFloat32 writes a float32 using a pooled buffer.
func WriteFloat32(w io.Writer, v float32) error {
	return WriteUint32(w, math.Float32bits(v))
}

// ReadFloat32 reads a float32 using a pooled buffer.
func ReadFloat32(r io.Reader) (float32, error) {
	// Read the uint32 bits first.
	bits, err := ReadUint32(r)
	if err != nil {
		return 0, err
	}

	// Convert bits to float32.
	return math.Float32frombits(bits), nil
}
