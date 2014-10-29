package textutil

import (
	"bytes"
)

// TODO(toddw): Add UTF16 support.

const (
	EOF                = rune(-1) // Indicates the end of a rune stream.
	LineSeparator      = '\u2028' // Unicode line separator rune.
	ParagraphSeparator = '\u2029' // Unicode paragraph separator rune.
)

// RuneEncoder is the interface to an encoder of a stream of runes into
// bytes.Buffer.
type RuneEncoder interface {
	// Encode encodes r into buf.
	Encode(r rune, buf *bytes.Buffer)
}

// RuneStreamDecoder is the interface to a decoder of a contiguous stream of
// runes.
type RuneStreamDecoder interface {
	// Next returns the next rune.  Invalid encodings are returned as U+FFFD.
	// Returns EOF at the end of the stream.
	Next() rune
	// BytePos returns the current byte position in the original data buffer.
	BytePos() int
}

// RuneChunkDecoder is the interface to a decoder of a stream of encoded runes
// that may be arbitrarily chunked.
//
// Implementations of RuneChunkDecoder are commonly used to implement io.Writer
// wrappers, to handle buffering when chunk boundaries may occur in the middle
// of an encoded rune.
type RuneChunkDecoder interface {
	// Decode returns a RuneStreamDecoder that decodes the data chunk.  Call Next
	// repeatedly on the returned stream until it returns EOF to decode the chunk.
	Decode(chunk []byte) RuneStreamDecoder
	// DecodeLeftover returns a RuneStreamDecoder that decodes leftover buffered
	// data.  Call Next repeatedly on the returned stream until it returns EOF to
	// ensure all buffered data is processed.
	DecodeLeftover() RuneStreamDecoder
}

// RuneChunkWrite is a helper that calls d.Decode(data) and repeatedly calls
// Next in a loop, calling fn for every rune that is decoded.  Returns the
// number of bytes in data that were successfully processed.  If fn returns an
// error, Write will return with that error, without processing any more data.
//
// This is a convenience for implementing io.Writer, given a RuneChunkDecoder.
func RuneChunkWrite(d RuneChunkDecoder, fn func(rune) error, data []byte) (int, error) {
	stream := d.Decode(data)
	for r := stream.Next(); r != EOF; r = stream.Next() {
		if err := fn(r); err != nil {
			return stream.BytePos(), err
		}
	}
	return stream.BytePos(), nil
}

// RuneChunkFlush is a helper that calls d.DecodeLeftover and repeatedly calls
// Next in a loop, calling fn for every rune that is decoded.  If fn returns an
// error, Flush will return with that error, without processing any more data.
//
// This is a convenience for implementing an additional Flush() call on an
// implementation of io.Writer, given a RuneChunkDecoder.
func RuneChunkFlush(d RuneChunkDecoder, fn func(rune) error) error {
	stream := d.DecodeLeftover()
	for r := stream.Next(); r != EOF; r = stream.Next() {
		if err := fn(r); err != nil {
			return err
		}
	}
	return nil
}

// bytePos and runePos distinguish positions that are used in either domain;
// we're trying to avoid silly mistakes like adding a bytePos to a runePos.
type bytePos int
type runePos int

// byteRuneBuffer maintains a buffer with both byte and rune based positions.
type byteRuneBuffer struct {
	enc     RuneEncoder
	buf     bytes.Buffer
	runeLen runePos
}

func (b *byteRuneBuffer) ByteLen() bytePos { return bytePos(b.buf.Len()) }
func (b *byteRuneBuffer) RuneLen() runePos { return b.runeLen }
func (b *byteRuneBuffer) Bytes() []byte    { return b.buf.Bytes() }

func (b *byteRuneBuffer) Reset() {
	b.buf.Reset()
	b.runeLen = 0
}

// WriteRune writes r into b.
func (b *byteRuneBuffer) WriteRune(r rune) {
	b.enc.Encode(r, &b.buf)
	b.runeLen++
}

// WriteString writes str into b.
func (b *byteRuneBuffer) WriteString(str string) {
	for _, r := range str {
		b.WriteRune(r)
	}
}

// WriteString0Runes writes str into b, not incrementing the rune length.
func (b *byteRuneBuffer) WriteString0Runes(str string) {
	for _, r := range str {
		b.enc.Encode(r, &b.buf)
	}
}
