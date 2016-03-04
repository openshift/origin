package api

import "testing"

func TestInjectionListSet(t *testing.T) {
	table := map[string][]InjectPath{
		"/test:":             {{SourcePath: "/test", DestinationDir: "."}},
		"/test:/test":        {{SourcePath: "/test", DestinationDir: "/test"}},
		"/test/foo:/etc/ssl": {{SourcePath: "/test/foo", DestinationDir: "/etc/ssl"}},
		":/foo":              {{SourcePath: ".", DestinationDir: "/foo"}},
		"/foo":               {{SourcePath: "/foo", DestinationDir: "."}},
		":":                  {{SourcePath: ".", DestinationDir: "."}},
		"/t est/foo:":        {{SourcePath: "/t est/foo", DestinationDir: "."}},
		`"/test":"/foo"`:     {{SourcePath: "/test", DestinationDir: "/foo"}},
		`'/test':"/foo"`:     {{SourcePath: "/test", DestinationDir: "/foo"}},
		`"/te"st":"/foo"`:    {},
		"/test/foo:/ss;ss":   {},
		"/test;foo:/ssss":    {},
	}
	for v, expected := range table {
		got := InjectionList{}
		err := got.Set(v)
		if len(expected) == 0 {
			if err == nil {
				t.Errorf("Expected error for %q, got %#v", v, got)
			} else {
				continue
			}
		}
		if len(got) != len(expected) {
			t.Errorf("Expected %d injection in the list for %q, got %d", len(expected), v, len(got))
		}
		for _, exp := range expected {
			found := false
			for _, g := range got {
				if g.SourcePath == exp.SourcePath && g.DestinationDir == exp.DestinationDir {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected %+v injection found in %#v list", exp, got)
			}
		}
	}
}
