package api

import "testing"

func TestVolumeListSet(t *testing.T) {
	table := map[string][]VolumeSpec{
		"/test:":             {{Source: "/test", Destination: "."}},
		"/test:/test":        {{Source: "/test", Destination: "/test"}},
		"/test/foo:/etc/ssl": {{Source: "/test/foo", Destination: "/etc/ssl"}},
		":/foo":              {{Source: ".", Destination: "/foo"}},
		"/foo":               {{Source: "/foo", Destination: "."}},
		":":                  {{Source: ".", Destination: "."}},
		"/t est/foo:":        {{Source: "/t est/foo", Destination: "."}},
		`"/test":"/foo"`:     {{Source: "/test", Destination: "/foo"}},
		`'/test':"/foo"`:     {{Source: "/test", Destination: "/foo"}},
		`"/te"st":"/foo"`:    {},
		"/test/foo:/ss;ss":   {},
		"/test;foo:/ssss":    {},
	}
	for v, expected := range table {
		got := VolumeList{}
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
				if g.Source == exp.Source && g.Destination == exp.Destination {
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
