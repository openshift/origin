package haconfig

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func TestValidateIPAddress(t *testing.T) {
	validIPs := []string{"1.1.1.1", "1.1.1.255", "255.255.255.255",
		"8.8.8.8", "0.1.2.3", "255.254.253.252",
	}

	for _, ip := range validIPs {
		if err := ValidateIPAddress(ip); err != nil {
			t.Errorf("Test valid ip=%q got error %s expected: no error.", ip, err)
		}
	}

	invalidIPs := []string{"1.1.1.256", "256.256.256.256",
		"1024.512.256.128", "a.b.c.d", "1.2.3.4.abc", "5.6.7.8def",
		"a.12.13.14", "9999.888.77.6",
	}

	for _, ip := range invalidIPs {
		if err := ValidateIPAddress(ip); err == nil {
			t.Errorf("Test invalid ip=%q got no error expected: error.", ip)
		}
	}
}

func TestValidateIPAddressRange(t *testing.T) {
	validRanges := []string{"1.1.1.1-1", "1.1.1.1-7", "1.1.1.250-255",
		"255.255.255.255-255", "8.8.8.4-8", "0.1.2.3-255",
		"255.254.253.252-255",
	}

	for _, iprange := range validRanges {
		if err := ValidateIPAddressRange(iprange); err != nil {
			t.Errorf("Test valid iprange=%q got error %s expected: no error.", iprange, err)
		}
	}

	invalidRanges := []string{"1.1.1.256-250", "1.1.1.1-0",
		"1.1.1.5-1", "255.255.255.255-259", "1024.512.256.128-255",
		"a.b.c.d-e", "1.2.3.4.abc-def", "5.6.7.8def-1.2.3.4abc",
		"a.12.13.14-55", "9999.888.77.6-66",
	}

	for _, iprange := range invalidRanges {
		if err := ValidateIPAddressRange(iprange); err == nil {
			t.Errorf("Test invalid iprange=%q got no error expected: error.", iprange)
		}
	}
}

func TestValidateVirtualIPs(t *testing.T) {
	validVIPs := []string{"", "1.1.1.1-1,2.2.2.2", "4.4.4.4-8",
		"1.1.1.1-7,2.2.2.2,3.3.3.3-5",
		"1.1.1.250-255,255.255.255.255-255", "4.4.4.4-8,8.8.8.4-8",
		"0.1.2.3-255,4.5.6.7,8.9.10.11,12.13.14.15-20",
		"255.254.253.252-255,1.1.1.1",
	}

	for _, vips := range validVIPs {
		if err := ValidateVirtualIPs(vips); err != nil {
			t.Errorf("Test valid vips=%q got error %s expected: no error.",
				vips, err)
		}
	}

	invalidVIPs := []string{"1.1.1.256-250,2.2.2.2", "1.1.1.1,2.2.2.2-0",
		"1.1.1.1-5,2.2.2.2,3.3.3.3-1", "255.255.255.255-259",
		"1.2.3.4-5,1024.512.256.128-255", "1.1.1.1,a.b.c.d-e",
		"a.b.c.d-e,5.4.3.2", "1.2.3.4.abc-def",
		"5.6.7.8def-1.2.3.4abc", "4.1.1.1,a.12.13.14-55",
		"8.8.8.8,9999.888.77.6-66,4.4.4.4-8",
	}

	for _, vips := range invalidVIPs {
		if err := ValidateVirtualIPs(vips); err == nil {
			t.Errorf("Test invalid vips=%q got no error expected: error.", vips)
		}
	}
}

/*
type MockPlugin struct {
	Name      string
	Factory   *clientcmd.Factory
	Options   *HAConfigCmdOptions
	Service   *kapi.Service
	CallCount map[string]int
}

func (p *MockPlugin) IncrementCallCount(name string) {
	value, ok := p.CallCount[name]
	if !ok {
		value = 0
	}
	value += 1
	p.CallCount[name] = value
}

func (p *MockPlugin) GetWatchPort() int {
	p.IncrementCallCount("GetWatchPort")
	return p.Options.WatchPort
}

func (p *MockPlugin) GetSelector() map[string]string {
	p.IncrementCallCount("GetSelector")
	return map[string]string{DefaultName: p.Name}
}

func (p *MockPlugin) GetNamespace() string {
	p.IncrementCallCount("GetNamespace")
	return "mock"
}

func (p *MockPlugin) GetService() *kapi.Service {
	p.IncrementCallCount("GetService")
	return p.Service
}

func (p *MockPlugin) Generate() *kapi.List {
	p.IncrementCallCount("Generate")
	return &kapi.List{}
}

func (p *MockPlugin) Create(out io.Writer) {
	p.IncrementCallCount("Create")
}

func (p *MockPlugin) Delete() {
	p.IncrementCallCount("Delete")
}
*/

func getMockConfigurator(options *HAConfigCmdOptions, service *kapi.Service) *Configurator {
	p := &MockPlugin{
		Name:      "mock",
		Options:   options,
		Service:   service,
		CallCount: make(map[string]int, 0),
	}
	return NewConfigurator("mock-plugin", p, nil)
}

func TestValidateCmdOptionsForCreate(t *testing.T) {
	tests := []struct {
		Name             string
		Create           bool
		Service          *kapi.Service
		ErrorExpectation bool
	}{
		{
			Name:             "create-with-no-service",
			Create:           true,
			ErrorExpectation: false,
		},
		{
			Name:             "create-with-service",
			Create:           true,
			Service:          &kapi.Service{},
			ErrorExpectation: true,
		},
		{
			Name:             "no-create-option-and-service",
			ErrorExpectation: false,
		},
		{
			Name:             "no-create-option-with-service",
			Service:          &kapi.Service{},
			ErrorExpectation: false,
		},
	}

	for _, tc := range tests {
		options := &HAConfigCmdOptions{Create: tc.Create}
		plugin := &MockPlugin{
			Name:      "mock",
			Options:   options,
			Service:   tc.Service,
			CallCount: make(map[string]int, 0),
		}
		c := NewConfigurator(tc.Name, plugin, nil)

		err := ValidateCmdOptions(options, c)
		if err != nil && !tc.ErrorExpectation {
			t.Errorf("Test case %q got an error: %v where none was expected.",
				tc.Name, err)
		}
		if nil == err && tc.ErrorExpectation {
			t.Errorf("Test case %q got no error - expected an error.", tc.Name)
		}
	}
}

func TestValidateCmdOptionsForDelete(t *testing.T) {
	tests := []struct {
		Name             string
		Delete           bool
		Service          *kapi.Service
		ErrorExpectation bool
	}{
		{
			Name:             "delete-with-service",
			Delete:           true,
			Service:          &kapi.Service{},
			ErrorExpectation: false,
		},
		{
			Name:             "delete-with-no-service",
			Delete:           true,
			ErrorExpectation: true,
		},
		{
			Name:             "no-delete-option-and-service",
			ErrorExpectation: false,
		},
		{
			Name:             "no-delete-option-with-service",
			Service:          &kapi.Service{},
			ErrorExpectation: false,
		},
	}

	for _, tc := range tests {
		options := &HAConfigCmdOptions{Delete: tc.Delete}
		plugin := &MockPlugin{
			Name:      "mock",
			Options:   options,
			Service:   tc.Service,
			CallCount: make(map[string]int, 0),
		}
		c := NewConfigurator(tc.Name, plugin, nil)

		err := ValidateCmdOptions(options, c)
		if err != nil && !tc.ErrorExpectation {
			t.Errorf("Test case %q got an error: %v where none was expected.",
				tc.Name, err)
		}
		if nil == err && tc.ErrorExpectation {
			t.Errorf("Test case %q got no error - expected an error.", tc.Name)
		}
	}
}

func TestValidateCmdOptionsVIPs(t *testing.T) {
	validVIPs := []string{"", "1.1.1.1-1,2.2.2.2", "4.4.4.4-8",
		"1.1.1.1-7,2.2.2.2,3.3.3.3-5",
		"1.1.1.250-255,255.255.255.255-255", "4.4.4.4-8,8.8.8.4-8",
		"0.1.2.3-255,4.5.6.7,8.9.10.11,12.13.14.15-20",
		"255.254.253.252-255,1.1.1.1",
	}

	for _, vips := range validVIPs {
		options := &HAConfigCmdOptions{VirtualIPs: vips}
		c := getMockConfigurator(options, nil)
		if err := ValidateCmdOptions(options, c); err != nil {
			t.Errorf("Test command options valid vips=%q got error %s expected: no error.",
				vips, err)
		}
	}

	invalidVIPs := []string{"1.1.1.256-250,2.2.2.2", "1.1.1.1,2.2.2.2-0",
		"1.1.1.1-5,2.2.2.2,3.3.3.3-1", "255.255.255.255-259",
		"1.2.3.4-5,1024.512.256.128-255", "1.1.1.1,a.b.c.d-e",
		"a.b.c.d-e,5.4.3.2", "1.2.3.4.abc-def",
		"5.6.7.8def-1.2.3.4abc", "4.1.1.1,a.12.13.14-55",
		"8.8.8.8,9999.888.77.6-66,4.4.4.4-8",
	}

	for _, vips := range invalidVIPs {
		options := &HAConfigCmdOptions{VirtualIPs: vips}
		c := getMockConfigurator(options, nil)
		if err := ValidateCmdOptions(options, c); err == nil {
			t.Errorf("Test command options invalid vips=%q got no error expected: error.", vips)
		}
	}
}
