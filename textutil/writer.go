package textutil

import (
	"bytes"
	"io"
)

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
