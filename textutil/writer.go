// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import (
	"bytes"
	"io"
	"unicode/utf8"
)

// WriteFlusher is the interface that groups the basic Write and Flush methods.
//
// Flush is typically provided when Write calls perform buffering; Flush
// immediately outputs the buffered data.  Flush must be called after the last
// call to Write, and may be called an arbitrary number of times before the last
// Write.
//
// If the type is also a Closer, Close implies a Flush call.
type WriteFlusher interface {
	io.Writer
	Flush() error
}

// WriteFlushCloser is the interface that groups the basic Write, Flush and
// Close methods.
type WriteFlushCloser interface {
	WriteFlusher
	io.Closer
}

// PrefixWriter returns an io.Writer that wraps w, where the prefix is written
// out immediately before the first non-empty Write call.
func PrefixWriter(w io.Writer, prefix string) io.Writer {
	return &prefixWriter{w, []byte(prefix)}
}

type prefixWriter struct {
	w      io.Writer
	prefix []byte
}

func (w *prefixWriter) Write(data []byte) (int, error) {
	if w.prefix != nil && len(data) > 0 {
		w.w.Write(w.prefix)
		w.prefix = nil
	}
	return w.w.Write(data)
}

// PrefixLineWriter returns a WriteFlushCloser that wraps w.  Any occurrence of
// EOL (\f, \n, \r, \v, LineSeparator or ParagraphSeparator) causes the
// preceeding line to be written to w, with the given prefix.  Data without EOL
// is buffered until the next EOL, or Flush or Close call.  A single Write call
// may result in zero or more Write calls on the underlying writer.
func PrefixLineWriter(w io.Writer, prefix string) WriteFlushCloser {
	return &prefixLineWriter{w, []byte(prefix), nil}
}

type prefixLineWriter struct {
	w      io.Writer
	prefix []byte
	buf    []byte
}

const eolRunesAsString = "\f\n\r\v" + string(LineSeparator) + string(ParagraphSeparator)

func (w *prefixLineWriter) Write(data []byte) (int, error) {
	totalLen := len(data)
	for len(data) > 0 {
		index := bytes.IndexAny(data, eolRunesAsString)
		if index == -1 {
			// No EOL: buffer remaining data.
			// TODO(toddw): Flush at a max size, to avoid unbounded growth?
			w.buf = append(w.buf, data...)
			return totalLen, nil
		}
		// Saw EOL: write prefix, buffer, and data including EOL.
		if _, err := w.w.Write(w.prefix); err != nil {
			return totalLen - len(data), err
		}
		if _, err := w.w.Write(w.buf); err != nil {
			return totalLen - len(data), err
		}
		w.buf = w.buf[:0]
		_, eolSize := utf8.DecodeRune(data[index:])
		n, err := w.w.Write(data[:index+eolSize])
		data = data[n:]
		if err != nil {
			return totalLen - len(data), err
		}
	}
	return totalLen, nil
}

func (w *prefixLineWriter) Flush() error {
	if len(w.buf) > 0 {
		if _, err := w.w.Write(w.prefix); err != nil {
			return err
		}
		if _, err := w.w.Write(w.buf); err != nil {
			return err
		}
		w.buf = w.buf[:0]
	}
	if f, ok := w.w.(WriteFlusher); ok {
		return f.Flush()
	}
	return nil
}

func (w *prefixLineWriter) Close() error {
	firstErr := w.Flush()
	if c, ok := w.w.(io.Closer); ok {
		if err := c.Close(); firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ByteReplaceWriter returns an io.Writer that wraps w, where all occurrences of
// the old byte are replaced with the new string on Write calls.
func ByteReplaceWriter(w io.Writer, old byte, new string) io.Writer {
	return &byteReplaceWriter{w, []byte{old}, []byte(new)}
}

type byteReplaceWriter struct {
	w        io.Writer
	old, new []byte
}

func (w *byteReplaceWriter) Write(data []byte) (int, error) {
	replaced := bytes.Replace(data, w.old, w.new, -1)
	if len(replaced) == 0 {
		return len(data), nil
	}
	// Write the replaced data, and return the number of bytes in data that were
	// written out, based on the proportion of replaced data written.  The
	// important boundary cases are:
	//   If all replaced data was written, we return n=len(data).
	//   If not all replaced data was written, we return n<len(data).
	n, err := w.w.Write(replaced)
	return n * len(data) / len(replaced), err
}

// TODO(toddw): Add ReplaceWriter, which performs arbitrary string replacements.
// This will need to buffer data and have an extra Flush() method, since the old
// string may match across successive Write calls.
