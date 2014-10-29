package textutil

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

// UTF8Encoder implements RuneEncoder for the UTF-8 encoding.
type UTF8Encoder struct{}

var _ RuneEncoder = UTF8Encoder{}

// Encode encodes r into buf in the UTF-8 encoding.
func (UTF8Encoder) Encode(r rune, buf *bytes.Buffer) { buf.WriteRune(r) }

// UTF8ChunkDecoder implements RuneChunkDecoder for a stream of UTF-8 data that
// is arbitrarily chunked.
//
// UTF-8 is a byte-wise encoding that may use multiple bytes to encode a single
// rune.  This decoder buffers partial runes that have been split across chunks,
// so that a full rune is returned when the subsequent data chunk is provided.
//
// This is commonly used to implement an io.Writer wrapper over UTF-8 text.  It
// is useful since the data provided to Write calls may be arbitrarily chunked.
//
// The zero UTF8ChunkDecoder is a decoder with an empty buffer.
type UTF8ChunkDecoder struct {
	// The only state we keep is the last partial rune we've encountered.
	partial    [utf8.UTFMax]byte
	partialLen int
}

var _ RuneChunkDecoder = (*UTF8ChunkDecoder)(nil)

// Decode returns a RuneStreamDecoder that decodes the data chunk.  Call Next
// repeatedly on the returned stream until it returns EOF to decode the chunk.
//
// If the data is chunked in the middle of an encoded rune, the final partial
// rune in the chunk will be buffered, and the next call to Decode will continue
// by combining the buffered data with the next chunk.
//
// Invalid encodings are transformed into U+FFFD, one byte at a time.  See
// unicode/utf8.DecodeRune for details.
func (d *UTF8ChunkDecoder) Decode(chunk []byte) RuneStreamDecoder {
	return &utf8Stream{d, chunk, 0}
}

// DecodeLeftover returns a RuneStreamDecoder that decodes leftover buffered
// data.  Call Next repeatedly on the returned stream until it returns EOF to
// ensure all buffered data is processed.
//
// Since the only data that is buffered is the final partial rune, the returned
// RuneStreamDecoder will only contain U+FFFD or EOF.
func (d *UTF8ChunkDecoder) DecodeLeftover() RuneStreamDecoder {
	return &utf8LeftoverStream{d, 0}
}

// nextRune decodes the next rune, logically combining any previously buffered
// data with the data chunk.  It returns the decoded rune and the byte size of
// the data that was used for the decoding.
//
// The returned size may be > 0 even if the returned rune == EOF, if a partial
// rune was detected and buffered.  The returned size may be 0 even if the
// returned rune != EOF, if previously buffered data was decoded.
func (d *UTF8ChunkDecoder) nextRune(data []byte) (rune, int) {
	if d.partialLen > 0 {
		return d.nextRunePartial(data)
	}
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError && !utf8.FullRune(data) {
		// Initialize the partial rune buffer with remaining data.
		d.partialLen = copy(d.partial[:], data)
		return d.verifyPartial(d.partialLen, data)
	}
	return r, size
}

// nextRunePartial implements nextRune when there is a previously buffered
// partial rune.
func (d *UTF8ChunkDecoder) nextRunePartial(data []byte) (rune, int) {
	// Append as much data as we can to the partial rune, and see if it's full.
	oldLen := d.partialLen
	d.partialLen += copy(d.partial[oldLen:], data)
	if !utf8.FullRune(d.partial[:d.partialLen]) {
		// We still don't have a full rune - keep waiting.
		return d.verifyPartial(d.partialLen-oldLen, data)
	}
	// We finally have a full rune.
	r, size := utf8.DecodeRune(d.partial[:d.partialLen])
	if size < oldLen {
		// This occurs when we have a multi-byte rune that has the right number of
		// bytes, but is an invalid code point.
		//
		// Say oldLen=2, and we just received the third byte of a 3-byte rune which
		// isn't a UTF-8 trailing byte.  In this case utf8.DecodeRune returns U+FFFD
		// and size=1, to indicate we should skip the first byte.
		//
		// We shift the unread portion of the old partial data forward, and update
		// the partial len so that it's strictly decreasing.  The strictly
		// decreasing property isn't necessary for correctness, but helps avoid
		// repeatedly copying data into the partial buffer unecessarily.
		copy(d.partial[:], d.partial[size:oldLen])
		d.partialLen = oldLen - size
		return r, 0
	}
	// We've used all the old buffered data; start decoding directly from data.
	d.partialLen = 0
	return r, size - oldLen
}

// verifyPartial is called when we don't have a full rune, and ncopy bytes have
// been copied from data into the decoder partial rune buffer.  We expect that
// all data has been buffered and we return EOF and the total size of the data.
func (d *UTF8ChunkDecoder) verifyPartial(ncopy int, data []byte) (rune, int) {
	if ncopy < len(data) {
		// Something's very wrong if we managed to fill d.partial without copying
		// all the data; any sequence of utf8.UTFMax bytes must be a full rune.
		panic(fmt.Errorf("UTF8ChunkDecoder: partial rune %v with leftover data %v", d.partial[:d.partialLen], data[ncopy:]))
	}
	return EOF, len(data)
}

// utf8Stream implements UTF8ChunkDecoder.Decode.
type utf8Stream struct {
	d    *UTF8ChunkDecoder
	data []byte
	pos  int
}

var _ RuneStreamDecoder = (*utf8Stream)(nil)

func (s *utf8Stream) Next() rune {
	if s.pos == len(s.data) {
		return EOF
	}
	r, size := s.d.nextRune(s.data[s.pos:])
	s.pos += size
	return r
}

func (s *utf8Stream) BytePos() int {
	return s.pos
}

// utf8LeftoverStream implements UTF8ChunkDecoder.DecodeLeftover.
type utf8LeftoverStream struct {
	d   *UTF8ChunkDecoder
	pos int
}

var _ RuneStreamDecoder = (*utf8LeftoverStream)(nil)

func (s *utf8LeftoverStream) Next() rune {
	if s.d.partialLen == 0 {
		return EOF
	}
	r, size := utf8.DecodeRune(s.d.partial[:s.d.partialLen])
	copy(s.d.partial[:], s.d.partial[size:])
	s.d.partialLen -= size
	s.pos += size
	return r
}

func (s *utf8LeftoverStream) BytePos() int {
	return s.pos
}
