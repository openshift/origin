// This file is dual licensed under CC0 and The gonum license.
//
// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Copyright ©2017 Robin Eklind.
// This file is made available under a Creative Commons CC0 1.0
// Universal Public Domain Dedication.

package lexer_test

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"gonum.org/v1/gonum/graph/formats/dot"
)

func TestParseFile(t *testing.T) {
	golden := []struct {
		in  string
		out string
	}{
		{
			in:  "testdata/tokens.dot",
			out: "testdata/tokens.golden",
		},
	}
	for _, g := range golden {
		file, err := dot.ParseFile(g.in)
		if err != nil {
			t.Errorf("%q: unable to parse file; %v", g.in, err)
			continue
		}
		// If no output path is specified, the input is already golden.
		out := g.in
		if len(g.out) > 0 {
			out = g.out
		}
		buf, err := ioutil.ReadFile(out)
		if err != nil {
			t.Errorf("%q: unable to read file; %v", g.in, err)
			continue
		}
		got := file.String()
		// Remove trailing newline.
		want := string(bytes.TrimSpace(buf))
		if got != want {
			t.Errorf("%q: graph mismatch; expected %q, got %q", g.in, want, got)
		}
	}
}

func TestParseFuzz(t *testing.T) {
	r, err := zip.OpenReader("../../fuzz/corpus.zip")
	if err != nil {
		if os.IsNotExist(err) {
			t.Skip("no corpus")
		}
		t.Fatalf("failed to open corpus: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open %q: %v", f.Name, err)
		}
		func() {
			defer func() {
				p := recover()
				if p != nil {
					t.Errorf("unexpected panic parsing %q: %v", f.Name, p)
				}
			}()

			_, err = dot.Parse(rc)
			if err != nil {
				t.Errorf("unexpected error parsing %q: %v", f.Name, err)
			}
		}()
		rc.Close()
	}
}
