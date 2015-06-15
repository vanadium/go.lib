// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//+build ignore

// This command is used by "go generate" for (re)generating the
// implementation of this package.
package main

import (
	"fmt"
	"os"
	"text/template"
	"unicode"
	"unicode/utf8"
)

var fns = template.FuncMap{
	"capitalize": capitalize,
	"suffix":     suffix,
	"value":      value,
}

var implTemplate = template.Must(template.New("impl").Funcs(fns).Parse(`// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

var {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}} = {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}T{}

type {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}T struct{}

// FromSlice transforms the given slice to a set.
func ({{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}T) FromSlice(els []{{.KeyType}}) map[{{.KeyType}}]{{.ValueType}} {
	if len(els) == 0 {
		return nil
	}
	result := map[{{.KeyType}}]{{.ValueType}}{}
	for _, el := range els {
		result[el] = {{value .ValueType}}
	}
	return result
}

// ToSlice transforms the given set to a slice.
func ({{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}T) ToSlice(s map[{{.KeyType}}]{{.ValueType}}) []{{.KeyType}} {
	var result []{{.KeyType}}
	for el, _ := range s {
		result = append(result, el)
	}
	return result
}

// Difference subtracts s2 from s1, storing the result in s1.
func ({{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}T) Difference(s1, s2 map[{{.KeyType}}]{{.ValueType}}) {
	for el, _ := range s1 {
		if _, ok := s2[el]; ok {
			delete(s1, el)
		}
	}
}

// Intersection intersects s1 and s2, storing the result in s1.
func ({{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}T) Intersection(s1, s2 map[{{.KeyType}}]{{.ValueType}}) {
	for el, _ := range s1 {
		if _, ok := s2[el]; !ok {
			delete(s1, el)
		}
	}
}

// Union merges s1 and s2, storing the result in s1.
func ({{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}T) Union(s1, s2 map[{{.KeyType}}]{{.ValueType}}) {
	for el, _ := range s2 {
		s1[el] = {{value .ValueType}}
	}
}
`))

var implTestTemplate = template.Must(template.New("impl-test").Funcs(fns).Parse(`// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package set

import (
	"testing"
)

func Test{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}(t *testing.T) {
	slice := []{{.KeyType}}{}
{{range $el := .Elements}}
	slice = append(slice, {{$el}}){{end}}

	// Test conversion from a slice.
	s1 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
	for i, want := range []bool{true, true} {
		if _, got := s1[slice[i]]; got != want {
			t.Errorf("index %d: got %v, want %v", i, got, want)
		}
	}
	if got, want := len(s1), len(slice); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test conversion to a slice.
	slice2 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.ToSlice(s1)
	for i, got := range []bool{true, true} {
		if _, want := s1[(slice2[i])]; got != want {
			t.Errorf("index %d: got %v, want %v", i, got, want)
		}
	}
	if got, want := len(slice2), len(s1); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test set difference.
	{
		s1 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		s2 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Difference(s1, s2)
		for i, want := range []bool{true, false} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		s4 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Difference(s3, s4)
		for i, want := range []bool{false, false} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		s6 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[:1])
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Difference(s5, s6)
		for i, want := range []bool{false, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		s8 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Difference(s7, s8)
		for i, want := range []bool{false, false} {
			if _, got := s7[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s7), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	// Test set intersection.
	{
		s1 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		s2 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Intersection(s1, s2)
		for i, want := range []bool{false, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		s4 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Intersection(s3, s4)
		for i, want := range []bool{false, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		s6 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[:1])
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Intersection(s5, s6)
		for i, want := range []bool{false, false} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		s8 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Intersection(s7, s8)
		for i, want := range []bool{true, true} {
			if _, got := s7[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s7), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	// Test set union.
	{
		s1 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[:1])
		s2 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Union(s1, s2)
		for i, want := range []bool{true, true} {
			if _, got := s1[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s1), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s3 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		s4 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Union(s3, s4)
		for i, want := range []bool{true, true} {
			if _, got := s3[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s3), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s5 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[1:])
		s6 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice[:1])
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Union(s5, s6)
		for i, want := range []bool{true, true} {
			if _, got := s5[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s5), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		s7 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		s8 := {{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.FromSlice(slice)
		{{capitalize .KeyType}}{{capitalize (suffix .ValueType)}}.Union(s7, s8)
		for i, want := range []bool{true, true} {
			if _, got := s7[(slice[i])]; got != want {
				t.Errorf("index %d: got %v, want %v", i, got, want)
			}
		}
		if got, want := len(s7), 2; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
`))

type builtin struct {
	KeyType   string
	ValueType string
	Elements  []string
}

type generator struct {
	Suffix   string
	Template *template.Template
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	rune, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(rune)) + s[size:]
}

func suffix(ty string) string {
	switch ty {
	case "bool":
		return "bool"
	case "struct{}":
		return ""
	default:
		panic(fmt.Errorf("unexpected type %v", ty))
	}
}

func value(ty string) string {
	switch ty {
	case "bool":
		return "true"
	case "struct{}":
		return "struct{}{}"
	default:
		panic(fmt.Errorf("unexpected type %v", ty))
	}
}

func main() {
	builtins := []builtin{
		builtin{
			KeyType:  "complex64",
			Elements: []string{"complex(-1.0, 1.0)", "complex(1.0, -1.0)"},
		},
		builtin{
			KeyType:  "complex128",
			Elements: []string{"complex(-1.0, 1.0)", "complex(1.0, -1.0)"},
		},
		builtin{
			KeyType:  "float32",
			Elements: []string{"-1.0", "1.0"},
		},
		builtin{
			KeyType:  "float64",
			Elements: []string{"-1.0", "1.0"},
		},
		builtin{
			KeyType:  "int",
			Elements: []string{"-1", "1"},
		},
		builtin{
			KeyType:  "int8",
			Elements: []string{"-1", "1"},
		},
		builtin{
			KeyType:  "int16",
			Elements: []string{"-1", "1"},
		},
		builtin{
			KeyType:  "int32",
			Elements: []string{"-1", "1"},
		},
		builtin{
			KeyType:  "int64",
			Elements: []string{"-1", "1"},
		},
		builtin{
			KeyType:  "string",
			Elements: []string{`"a"`, `"b"`},
		},
		builtin{
			KeyType:  "uint",
			Elements: []string{"0", "1"},
		},
		builtin{
			KeyType:  "uint8",
			Elements: []string{"0", "1"},
		},
		builtin{
			KeyType:  "uint16",
			Elements: []string{"0", "1"},
		},
		builtin{
			KeyType:  "uint32",
			Elements: []string{"0", "1"},
		},
		builtin{
			KeyType:  "uint64",
			Elements: []string{"0", "1"},
		},
		builtin{
			KeyType:  "uintptr",
			Elements: []string{"0", "1"},
		},
	}
	generators := []generator{
		generator{
			Suffix:   ".go",
			Template: implTemplate,
		},
		generator{
			Suffix:   "_test.go",
			Template: implTestTemplate,
		},
	}
	for _, b := range builtins {
		for _, ty := range []string{"struct{}", "bool"} {
			b.ValueType = ty
			for _, g := range generators {
				fileName := b.KeyType + suffix(ty) + g.Suffix
				file, err := os.Create(fileName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Create(%v) failed: %v", fileName, err)
					os.Exit(1)
				}
				if err := g.Template.Execute(file, b); err != nil {
					fmt.Fprintf(os.Stderr, "Execute(%v) failed: %v", b, err)
					os.Exit(1)
				}
			}
		}
	}
}
