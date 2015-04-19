package keepalived

import (
	"testing"

	haconfig "github.com/openshift/origin/pkg/haconfig"
)

func TestNewHAConfiguratorPlugin(t *testing.T) {
	tests := []struct {
		Name             string
		Options          *haconfig.HAConfigCmdOptions
		ErrorExpectation bool
	}{
		{
			Name:             "selector",
			Options:          &haconfig.HAConfigCmdOptions{Selector: "haconfig=test-nodes"},
			ErrorExpectation: false,
		},
		{
			Name:             "empty-selector",
			Options:          &haconfig.HAConfigCmdOptions{Selector: ""},
			ErrorExpectation: false,
		},
		{
			Name: "vips",
			Options: &haconfig.HAConfigCmdOptions{
				VirtualIPs: "1.2.3.4,5.6.7.8-10,11.0.0.12",
			},
			ErrorExpectation: false,
		},
		{
			Name:             "empty-vips",
			Options:          &haconfig.HAConfigCmdOptions{VirtualIPs: ""},
			ErrorExpectation: false,
		},
		{
			Name:             "interface",
			Options:          &haconfig.HAConfigCmdOptions{NetworkInterface: "eth0"},
			ErrorExpectation: false,
		},
		{
			Name:             "empty-interface",
			Options:          &haconfig.HAConfigCmdOptions{NetworkInterface: ""},
			ErrorExpectation: false,
		},
		{
			Name:             "watch-port",
			Options:          &haconfig.HAConfigCmdOptions{WatchPort: 999},
			ErrorExpectation: false,
		},
		{
			Name:             "replicas",
			Options:          &haconfig.HAConfigCmdOptions{Replicas: 2},
			ErrorExpectation: false,
		},
		{
			Name: "all-options",
			Options: &haconfig.HAConfigCmdOptions{
				Selector:         "hac=v1",
				VirtualIPs:       "9.8.7.6,5.4.3.2-5",
				NetworkInterface: "ha0",
				WatchPort:        12345,
				Replicas:         1,
			},
			ErrorExpectation: false,
		},
		{
			Name:             "no-options",
			Options:          &haconfig.HAConfigCmdOptions{},
			ErrorExpectation: false,
		},
		{
			Name:             "", // empty
			Options:          &haconfig.HAConfigCmdOptions{},
			ErrorExpectation: false,
		},
	}

	for _, tc := range tests {
		p, err := NewHAConfiguratorPlugin(tc.Name, nil, tc.Options)
		if err != nil && !tc.ErrorExpectation {
			t.Errorf("Test case for %s got an error where none was expected", tc.Name)
		}

		if nil == err && nil == p {
			t.Errorf("Test case for %s got no error but plugin was not found", tc.Name)
		}
	}
}

func TestPluginGetWatchPort(t *testing.T) {
	tests := []struct {
		Name      string
		WatchPort int
		Expected  int
	}{
		{
			Name:      "router",
			WatchPort: 80,
			Expected:  80,
		},
		{
			Name:      "service1",
			WatchPort: 9999,
			Expected:  9999,
		},
		{
			Name:      "invalid-port",
			WatchPort: -12345,
			Expected:  80,
		},
		{
			Name:      "invalid-port-2",
			WatchPort: -1,
			Expected:  80,
		},
		{
			Name:      "zero-port",
			WatchPort: 0,
			Expected:  80,
		},
	}

	for _, tc := range tests {
		options := &haconfig.HAConfigCmdOptions{WatchPort: tc.WatchPort}
		p, err := NewHAConfiguratorPlugin(tc.Name, nil, options)
		if err != nil {
			t.Errorf("Error creating HAConfigurator plugin - test=%q, error: %v", tc.Name, err)
		}

		port := p.GetWatchPort()
		if tc.Expected != port {
			t.Errorf("Test case %q expected watch port = %d, got %d",
				tc.Name, tc.Expected, port)
		}

	}
}

func TestPluginGetSelector(t *testing.T) {
	tests := []struct {
		Name        string
		Selector    string
		ExpectedKey string
	}{
		{
			Name:        "router",
			Selector:    "hac=router",
			ExpectedKey: "hac",
		},
		{
			Name:        "service1",
			Selector:    "service1=us-west",
			ExpectedKey: "service1",
		},
		{
			Name:        "default-selector",
			Selector:    haconfig.DefaultSelector,
			ExpectedKey: haconfig.DefaultName,
		},
	}

	for _, tc := range tests {
		options := &haconfig.HAConfigCmdOptions{Selector: tc.Selector}
		p, err := NewHAConfiguratorPlugin(tc.Name, nil, options)
		if err != nil {
			t.Errorf("Error creating HAConfigurator plugin - test=%q, error: %v", tc.Name, err)
		}

		selector := p.GetSelector()
		if len(tc.ExpectedKey) > 0 {
			if _, ok := selector[tc.ExpectedKey]; !ok {
				t.Errorf("Test case %q expected key %q was not found",
					tc.Name, tc.ExpectedKey)
			}
		}
	}
}

// TODO: tests for Delete, Create, Generate, GetService, GetNamespace.
