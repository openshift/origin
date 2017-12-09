package keepalived

import (
	"testing"

	"github.com/openshift/origin/pkg/oc/experimental/ipfailover/ipfailover"
)

func TestNewIPFailoverConfiguratorPlugin(t *testing.T) {
	tests := []struct {
		Name             string
		Options          *ipfailover.IPFailoverConfigCmdOptions
		ErrorExpectation bool
	}{
		{
			Name:             "selector",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{Selector: "ipfailover=test-nodes"},
			ErrorExpectation: false,
		},
		{
			Name:             "empty-selector",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{Selector: ""},
			ErrorExpectation: false,
		},
		{
			Name: "vips",
			Options: &ipfailover.IPFailoverConfigCmdOptions{
				VirtualIPs: "1.2.3.4,5.6.7.8-10,11.0.0.12",
			},
			ErrorExpectation: false,
		},
		{
			Name:             "empty-vips",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{VirtualIPs: ""},
			ErrorExpectation: false,
		},
		{
			Name:             "interface",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{NetworkInterface: "eth0"},
			ErrorExpectation: false,
		},
		{
			Name:             "empty-interface",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{NetworkInterface: ""},
			ErrorExpectation: false,
		},
		{
			Name:             "watch-port",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{WatchPort: 999},
			ErrorExpectation: false,
		},
		{
			Name:             "replicas",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{Replicas: 2},
			ErrorExpectation: false,
		},
		{
			Name:             "vrid-base",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{VRRPIDOffset: 30},
			ErrorExpectation: false,
		},
		{
			Name: "all-options",
			Options: &ipfailover.IPFailoverConfigCmdOptions{
				Selector:         "ipf=v1",
				VirtualIPs:       "9.8.7.6,5.4.3.2-5",
				NetworkInterface: "ipf0",
				WatchPort:        12345,
				VRRPIDOffset:     70,
				Replicas:         1,
			},
			ErrorExpectation: false,
		},
		{
			Name:             "no-options",
			Options:          &ipfailover.IPFailoverConfigCmdOptions{},
			ErrorExpectation: false,
		},
		{
			Name:             "", // empty
			Options:          &ipfailover.IPFailoverConfigCmdOptions{},
			ErrorExpectation: false,
		},
	}

	for _, tc := range tests {
		p, err := NewIPFailoverConfiguratorPlugin(tc.Name, nil, tc.Options)
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
			Name:      "service2",
			WatchPort: 65535,
			Expected:  65535,
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
			Name:      "invalid-port-3",
			WatchPort: 65536,
			Expected:  80,
		},
		{
			Name:      "zero-port",
			WatchPort: 0,
			Expected:  80,
		},
	}

	for _, tc := range tests {
		options := &ipfailover.IPFailoverConfigCmdOptions{WatchPort: tc.WatchPort}
		p, err := NewIPFailoverConfiguratorPlugin(tc.Name, nil, options)
		if err != nil {
			t.Errorf("Error creating IPFailoverConfigurator plugin - test=%q, error: %v", tc.Name, err)
		}

		port, err := p.GetWatchPort()
		if err != nil {
			t.Errorf("Error getting watch port - test=%q, error: %v", tc.Name, err)
		}

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
			Selector:    "ipf=router",
			ExpectedKey: "ipf",
		},
		{
			Name:        "service1",
			Selector:    "service1=us-west",
			ExpectedKey: "service1",
		},
		{
			Name:        "default-selector",
			Selector:    ipfailover.DefaultSelector,
			ExpectedKey: ipfailover.DefaultName,
		},
	}

	for _, tc := range tests {
		options := &ipfailover.IPFailoverConfigCmdOptions{Selector: tc.Selector}
		p, err := NewIPFailoverConfiguratorPlugin(tc.Name, nil, options)
		if err != nil {
			t.Errorf("Error creating IPFailoverConfigurator plugin - test=%q, error: %v", tc.Name, err)
		}

		selector, err := p.GetSelector()
		if err != nil {
			t.Errorf("Error getting selector - test=%q, error: %v", tc.Name, err)
		}

		if len(tc.ExpectedKey) > 0 {
			if _, ok := selector[tc.ExpectedKey]; !ok {
				t.Errorf("Test case %q expected key %q was not found",
					tc.Name, tc.ExpectedKey)
			}
		}
	}
}

// TODO: tests for Create, Generate, GetService, GetNamespace.
