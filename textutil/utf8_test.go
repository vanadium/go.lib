// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textutil

import (
	"reflect"
	"testing"
)

func TestUTF8ChunkDecoder(t *testing.T) {
	r2 := "Δ"
	r3 := "王"
	r4 := "\U0001F680"
	tests := []struct {
		Text string
		Want []rune
	}{
		{"", nil},
		{"a", []rune{'a'}},
		{"abc", []rune{'a', 'b', 'c'}},
		{"abc def ghi", []rune{'a', 'b', 'c', ' ', 'd', 'e', 'f', ' ', 'g', 'h', 'i'}},
		// 2-byte runes.
		{"ΔΘΠΣΦ", []rune{'Δ', 'Θ', 'Π', 'Σ', 'Φ'}},
		// 3-byte runes.
		{"王普澤世界", []rune{'王', '普', '澤', '世', '界'}},
		// 4-byte runes.
		{"\U0001F680\U0001F681\U0001F682\U0001F683", []rune{'\U0001F680', '\U0001F681', '\U0001F682', '\U0001F683'}},
		// Mixed-bytes.
		{"aΔ王\U0001F680普Θb", []rune{'a', 'Δ', '王', '\U0001F680', '普', 'Θ', 'b'}},
		// Error runes translated to U+FFFD.
		{"\uFFFD", []rune{'\uFFFD'}},
		{"a\uFFFDb", []rune{'a', '\uFFFD', 'b'}},
		{"\xFF", []rune{'\uFFFD'}},
		{"a\xFFb", []rune{'a', '\uFFFD', 'b'}},
		// Multi-byte full runes.
		{r2, []rune{[]rune(r2)[0]}},
		{r3, []rune{[]rune(r3)[0]}},
		{r4, []rune{[]rune(r4)[0]}},
		// Partial runes translated to U+FFFD.
		{r2[:1], []rune{'\uFFFD'}},
		{r3[:1], []rune{'\uFFFD'}},
		{r3[:2], []rune{'\uFFFD', '\uFFFD'}},
		{r4[:1], []rune{'\uFFFD'}},
		{r4[:2], []rune{'\uFFFD', '\uFFFD'}},
		{r4[:3], []rune{'\uFFFD', '\uFFFD', '\uFFFD'}},
		// Leading partial runes translated to U+FFFD.
		{r2[:1] + "b", []rune{'\uFFFD', 'b'}},
		{r3[:1] + "b", []rune{'\uFFFD', 'b'}},
		{r3[:2] + "b", []rune{'\uFFFD', '\uFFFD', 'b'}},
		{r4[:1] + "b", []rune{'\uFFFD', 'b'}},
		{r4[:2] + "b", []rune{'\uFFFD', '\uFFFD', 'b'}},
		{r4[:3] + "b", []rune{'\uFFFD', '\uFFFD', '\uFFFD', 'b'}},
		// Trailing partial runes translated to U+FFFD.
		{"a" + r2[:1], []rune{'a', '\uFFFD'}},
		{"a" + r3[:1], []rune{'a', '\uFFFD'}},
		{"a" + r3[:2], []rune{'a', '\uFFFD', '\uFFFD'}},
		{"a" + r4[:1], []rune{'a', '\uFFFD'}},
		{"a" + r4[:2], []rune{'a', '\uFFFD', '\uFFFD'}},
		{"a" + r4[:3], []rune{'a', '\uFFFD', '\uFFFD', '\uFFFD'}},
		// Bracketed partial runes translated to U+FFFD.
		{"a" + r2[:1] + "b", []rune{'a', '\uFFFD', 'b'}},
		{"a" + r3[:1] + "b", []rune{'a', '\uFFFD', 'b'}},
		{"a" + r3[:2] + "b", []rune{'a', '\uFFFD', '\uFFFD', 'b'}},
		{"a" + r4[:1] + "b", []rune{'a', '\uFFFD', 'b'}},
		{"a" + r4[:2] + "b", []rune{'a', '\uFFFD', '\uFFFD', 'b'}},
		{"a" + r4[:3] + "b", []rune{'a', '\uFFFD', '\uFFFD', '\uFFFD', 'b'}},
	}
	for _, test := range tests {
		// Run with a variety of chunk sizes.
		for _, sizes := range [][]int{nil, {1}, {2}, {1, 2}, {2, 1}, {3}, {1, 2, 3}} {
			got := runeChunkWriteFlush(t, test.Text, sizes)
			if want := test.Want; !reflect.DeepEqual(got, want) {
				t.Errorf("%q got %v, want %v", test.Text, got, want)
			}
		}
	}
}

func runeChunkWriteFlush(t *testing.T, text string, sizes []int) []rune {
	var dec UTF8ChunkDecoder
	var runes []rune
	addRune := func(r rune) error {
		runes = append(runes, r)
		return nil
	}
	// Write chunks of different sizes until we've exhausted the input text.
	remain := text
	for ix := 0; len(remain) > 0; ix++ {
		var chunk []byte
		chunk, remain = nextChunk(remain, sizes, ix)
		got, err := RuneChunkWrite(&dec, addRune, chunk)
		if want := len(chunk); got != want || err != nil {
			t.Errorf("%q RuneChunkWrite(%q) got (%d,%v), want (%d,nil)", text, chunk, got, err, want)
		}
	}
	// Flush the decoder.
	if err := RuneChunkFlush(&dec, addRune); err != nil {
		t.Errorf("%q RuneChunkFlush got %v, want nil", text, err)
	}
	return runes
}

func nextChunk(text string, sizes []int, index int) (chunk []byte, remain string) {
	if len(sizes) == 0 {
		return []byte(text), ""
	}
	size := sizes[index%len(sizes)]
	if size >= len(text) {
		return []byte(text), ""
	}
	return []byte(text[:size]), text[size:]
}
