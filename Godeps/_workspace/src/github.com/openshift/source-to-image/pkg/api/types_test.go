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

func TestEnvironmentSet(t *testing.T) {
	table := map[string][]EnvironmentSpec{
		"FOO=bar":  {{Name: "FOO", Value: "bar"}},
		"FOO=":     {{Name: "FOO", Value: ""}},
		"FOO":      {},
		"=":        {},
		"FOO=bar,": {{Name: "FOO", Value: "bar,"}},
		// Users should get a deprecation warning in this case
		// TODO: Create fake glog interface to be able to verify this.
		"FOO=bar,BAR=foo": {{Name: "FOO", Value: "bar,BAR=foo"}},
	}

	for v, expected := range table {
		got := EnvironmentList{}
		err := got.Set(v)
		if len(expected) == 0 && err == nil {
			t.Errorf("Expected error for env %q", v)
			continue
		}
		if len(expected) != len(got) {
			t.Errorf("got %d items, expected %d items in the list for %q", len(got), len(expected), v)
			continue
		}
		for _, exp := range expected {
			found := false
			for _, g := range got {
				if g.Name == exp.Name && g.Value == exp.Value {
					found = true
				}
			}
			if !found {
				t.Errorf("Expected %+v environment found in %#v list", exp, got)
			}
		}
	}
}
