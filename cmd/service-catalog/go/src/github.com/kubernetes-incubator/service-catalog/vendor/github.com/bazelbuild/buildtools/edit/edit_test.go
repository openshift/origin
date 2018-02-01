/*
Copyright 2016 Google Inc. All Rights Reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
package edit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bazelbuild/buildtools/build"
)

var parseLabelTests = []struct {
	in   string
	repo string
	pkg  string
	rule string
}{
	{"//devtools/buildozer:rule", "", "devtools/buildozer", "rule"},
	{"devtools/buildozer:rule", "", "devtools/buildozer", "rule"},
	{"//devtools/buildozer", "", "devtools/buildozer", "buildozer"},
	{"//base", "", "base", "base"},
	{"//base:", "", "base", "base"},
	{"@r//devtools/buildozer:rule", "r", "devtools/buildozer", "rule"},
	{"@r//devtools/buildozer", "r", "devtools/buildozer", "buildozer"},
	{"@r//base", "r", "base", "base"},
	{"@r//base:", "r", "base", "base"},
	{"@foo", "foo", "", "foo"},
	{":label", "", "", "label"},
	{"label", "", "", "label"},
	{"/abs/path/to/WORKSPACE:rule", "", "/abs/path/to/WORKSPACE", "rule"},
}

func TestParseLabel(t *testing.T) {
	for i, tt := range parseLabelTests {
		repo, pkg, rule := ParseLabel(tt.in)
		if repo != tt.repo || pkg != tt.pkg || rule != tt.rule {
			t.Errorf("%d. ParseLabel(%q) => (%q, %q, %q), want (%q, %q, %q)",
				i, tt.in, repo, pkg, rule, tt.repo, tt.pkg, tt.rule)
		}
	}
}

var shortenLabelTests = []struct {
	in     string
	pkg    string
	result string
}{
	{"//devtools/buildozer:rule", "devtools/buildozer", ":rule"},
	{"//devtools/buildozer:rule", "devtools", "//devtools/buildozer:rule"},
	{"//base:rule", "devtools", "//base:rule"},
	{"//base:base", "devtools", "//base"},
	{"//base", "base", ":base"},
	{":local", "", ":local"},
	{"something else", "", "something else"},
	{"/path/to/file", "path/to", "/path/to/file"},
}

func TestShortenLabel(t *testing.T) {
	for i, tt := range shortenLabelTests {
		result := ShortenLabel(tt.in, tt.pkg)
		if result != tt.result {
			t.Errorf("%d. ShortenLabel(%q, %q) => %q, want %q",
				i, tt.in, tt.pkg, result, tt.result)
		}
	}
}

var labelsEqualTests = []struct {
	label1   string
	label2   string
	pkg      string
	expected bool
}{
	{"//devtools/buildozer:rule", "rule", "devtools/buildozer", true},
	{"//devtools/buildozer:rule", "rule:jar", "devtools", false},
}

func TestLabelsEqual(t *testing.T) {
	for i, tt := range labelsEqualTests {
		if got := LabelsEqual(tt.label1, tt.label2, tt.pkg); got != tt.expected {
			t.Errorf("%d. LabelsEqual(%q, %q, %q) => %v, want %v",
				i, tt.label1, tt.label2, tt.pkg, got, tt.expected)
		}
	}
}

var splitOnSpacesTests = []struct {
	in  string
	out []string
}{
	{"a", []string{"a"}},
	{"  abc def ", []string{"abc", "def"}},
	{`  abc\ def `, []string{"abc def"}},
}

func TestSplitOnSpaces(t *testing.T) {
	for i, tt := range splitOnSpacesTests {
		result := SplitOnSpaces(tt.in)
		if !reflect.DeepEqual(result, tt.out) {
			t.Errorf("%d. SplitOnSpaces(%q) => %q, want %q",
				i, tt.in, result, tt.out)
		}
	}
}

func TestInsertLoad(t *testing.T) {
	tests := []struct{ input, expected string }{
		{``, `load("location", "symbol")`},
		{`load("location", "symbol")`, `load("location", "symbol")`},
		{`load("location", "other", "symbol")`, `load("location", "other", "symbol")`},
		{`load("location", "other")`, `load("location", "other", "symbol")`},
		{
			`load("other loc", "symbol")`,
			`load("location", "symbol")
load("other loc", "symbol")`,
		},
	}

	for _, tst := range tests {
		bld, err := build.Parse("BUILD", []byte(tst.input))
		if err != nil {
			t.Error(err)
			continue
		}
		bld.Stmt = InsertLoad(bld.Stmt, []string{"location", "symbol"})
		got := strings.TrimSpace(string(build.Format(bld)))
		if got != tst.expected {
			t.Errorf("maybeInsertLoad(%s): got %s, expected %s", tst.input, got, tst.expected)
		}
	}
}

func TestAddValueToListAttribute(t *testing.T) {
	tests := []struct{ input, expected string }{
		{`rule(name="rule")`, `rule(name="rule", attr=["foo"])`},
		{`rule(name="rule", attr=["foo"])`, `rule(name="rule", attr=["foo"])`},
		{`rule(name="rule", attr=IDENT)`, `rule(name="rule", attr=IDENT+["foo"])`},
		{`rule(name="rule", attr=["foo"] + IDENT)`, `rule(name="rule", attr=["foo"] + IDENT)`},
		{`rule(name="rule", attr=["bar"] + IDENT)`, `rule(name="rule", attr=["bar", "foo"] + IDENT)`},
		{`rule(name="rule", attr=IDENT + ["foo"])`, `rule(name="rule", attr=IDENT + ["foo"])`},
		{`rule(name="rule", attr=IDENT + ["bar"])`, `rule(name="rule", attr=IDENT + ["bar", "foo"])`},
	}

	for _, tst := range tests {
		bld, err := build.Parse("BUILD", []byte(tst.input))
		if err != nil {
			t.Error(err)
			continue
		}
		rule := bld.RuleAt(1)
		AddValueToListAttribute(rule, "attr", "", &build.StringExpr{Value: "foo"}, nil)
		got := strings.TrimSpace(string(build.Format(bld)))

		wantBld, err := build.Parse("BUILD", []byte(tst.expected))
		if err != nil {
			t.Error(err)
			continue
		}
		want := strings.TrimSpace(string(build.Format(wantBld)))
		if got != want {
			t.Errorf("AddValueToListAttribute(%s): got %s, expected %s", tst.input, got, want)
		}
	}
}

func TestUseImplicitName(t *testing.T) {
	tests := []struct {
		input            string
		expectedRuleLine int
		wantErr          bool
		wantRootErr      bool
		description      string
	}{
		{`rule()`, 1, false, false, `Use an implicit name for one rule.`},
		{`rule(name="a")
		  rule(name="b")
		  rule()`, 3, false, false, `Use an implicit name for the one unnamed rule`},
		{`rule() rule() rule()`, 1, true, false, `Error for multiple unnamed rules`},
		{`rule()`, 1, true, true, `Error for the root package`},
	}

	for _, tst := range tests {
		path := "foo/BUILD"
		if tst.wantRootErr {
			path = "BUILD"
		}
		bld, err := build.Parse(path, []byte(tst.input))
		if err != nil {
			t.Error(tst.description, err)
			continue
		}
		got := UseImplicitName(bld, "foo")

		if !tst.wantErr {
			want := bld.RuleAt(tst.expectedRuleLine)
			if got.Kind() != want.Kind() || got.Name() != want.Name() {
				t.Errorf("UseImplicitName(%s): got %s, expected %s. %s", tst.input, got, want, tst.description)
			}
		} else {
			if got != nil {
				t.Errorf("UseImplicitName(%s): got %s, expected nil. %s", tst.input, got, tst.description)
			}
		}
	}
}
