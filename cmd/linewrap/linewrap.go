// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -h

package main

import (
	"io"
	"strconv"
	"strings"

	"v.io/x/lib/cmdline"
	"v.io/x/lib/textutil"
)

func main() {
	cmdline.Main(cmdLineWrap)
}

var cmdLineWrap = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runLineWrap),
	Name:   "linewrap",
	Short:  "formats text into pretty output",
	Long: `
Command linewrap formats text from stdin into pretty output on stdout.

The input text is expected to consist of words, defined as sequences of letters.
Sequences of words form paragraphs, where paragraphs are separated by either
blank lines (that contain no letters), or an explicit U+2029 ParagraphSeparator.
Input lines with leading spaces are treated verbatim.

Paragraphs are output as word-wrapped lines; line breaks only occur at word
boundaries.  Output lines are usually no longer than the target width,
defaulting to the terminal width.  The exceptions are single words longer than
the target width, which are output on their own line, and verbatim lines, which
may be arbitrarily longer or shorter than the width.

Output lines never contain trailing spaces.  Only verbatim output lines may
contain leading spaces.  Spaces separating input words are output verbatim,
unless it would result in a line with leading or trailing spaces.

Example usage in a unix terminal:
  $ cat myfile.txt | linewrap

See http://godoc.org/v.io/x/lib/textutil#WrapWriter for details on the
formatting algorithm.
`,
}

var (
	flagWidth    int
	flagLineTerm string
	flagParaSep  string
	flagIndents  string
)

func init() {
	_, width, err := textutil.TerminalSize()
	if err != nil {
		width = 80 // reasonable default for unknown terminal width
	}
	cmdLineWrap.Flags.IntVar(&flagWidth, "width", width, `
Target line width in runes.  If negative the line width is unlimited; each
paragraph is output as a single line.  If 0 each word is output on its own line.
Defaults to the terminal width.
`)
	cmdLineWrap.Flags.StringVar(&flagLineTerm, "line-term", `\n`, `
Line terminator.  Every output line is terminated with this string.  The format
is a Go interpreted string literal, where \n means newline.
   https://golang.org/ref/spec#String_literals
`)
	cmdLineWrap.Flags.StringVar(&flagParaSep, "para-sep", `\n`, `
Paragraph separator.  Every consecutive pair of non-empty paragraphs is
separated with this string.  The format is a Go interpreted string literal,
where \n menas newline.
   https://golang.org/ref/spec#String_literals
`)
	cmdLineWrap.Flags.StringVar(&flagIndents, "indents", ``, `
Comma-separated indentation prefixes.  Each entry specifes the prefix to use for
the corresponding paragraph line, or all subsequent paragraph lines if there are
no more entries.  E.g. "AA,BBB,C" means the first line in each paragraph is
indented with "AA", the second line with "BBB", and all subsequent lines with
"C".  The format of each indent prefix is a Go interpreted string literal.
   https://golang.org/ref/spec#String_literals
`)
	// Override default value, so that the godoc style shows good defaults.
	cmdLineWrap.Flags.Lookup("width").DefValue = "<terminal width>"
}

func runLineWrap(env *cmdline.Env, args []string) error {
	lineTerm, err := strconv.Unquote(`"` + flagLineTerm + `"`)
	if err != nil {
		return err
	}
	paraSep, err := strconv.Unquote(`"` + flagParaSep + `"`)
	if err != nil {
		return err
	}
	var indents []string
	if flagIndents != "" {
		for _, quoted := range strings.Split(flagIndents, ",") {
			indent, err := strconv.Unquote(`"` + quoted + `"`)
			if err != nil {
				return err
			}
			indents = append(indents, indent)
		}
	}
	w := textutil.NewUTF8WrapWriter(env.Stdout, flagWidth)
	if err := w.SetLineTerminator(lineTerm); err != nil {
		return err
	}
	if err := w.SetParagraphSeparator(paraSep); err != nil {
		return err
	}
	if err := w.SetIndents(indents...); err != nil {
		return err
	}
	if _, err := io.Copy(w, env.Stdin); err != nil {
		return err
	}
	return w.Flush()
}
