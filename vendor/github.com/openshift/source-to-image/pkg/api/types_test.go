package api

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestVolumeListSet(t *testing.T) {
	table := []struct {
		Input    string
		Expected VolumeList
	}{
		{"/test:", VolumeList{{Source: "/test", Destination: "."}}},
		{"/test:/test", VolumeList{{Source: "/test", Destination: "/test"}}},
		{"/test/foo:/etc/ssl", VolumeList{{Source: "/test/foo", Destination: "/etc/ssl"}}},
		{":/foo", VolumeList{{Source: ".", Destination: "/foo"}}},
		{"/foo", VolumeList{{Source: "/foo", Destination: "."}}},
		{":", VolumeList{{Source: ".", Destination: "."}}},
		{"/t est/foo:", VolumeList{{Source: "/t est/foo", Destination: "."}}},
		{`"/test":"/foo"`, VolumeList{{Source: "/test", Destination: "/foo"}}},
		{`'/test':"/foo"`, VolumeList{{Source: "/test", Destination: "/foo"}}},
		{`C:\test:/bar`, VolumeList{{Source: `C:\test`, Destination: "/bar"}}},
		{`C:\test:bar`, VolumeList{{Source: `C:\test`, Destination: "bar"}}},
		{`"/te"st":"/foo"`, VolumeList{}},
		{"/test/foo:/ss;ss", VolumeList{}},
		{"/test;foo:/ssss", VolumeList{}},
	}
	for _, test := range table {
		if len(test.Expected) != 0 {
			test.Expected[0].Source = filepath.FromSlash(test.Expected[0].Source)
		}
		got := VolumeList{}
		got.Set(test.Input)
		if !reflect.DeepEqual(got, test.Expected) {
			t.Errorf("On test %s, got %#v, expected %#v", test.Input, got, test.Expected)
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
					break
				}
			}
			if !found {
				t.Errorf("Expected %+v environment found in %#v list", exp, got)
			}
		}
	}
}
