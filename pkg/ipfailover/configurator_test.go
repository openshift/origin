package ipfailover

import (
	"fmt"
	"testing"
)

type ConfiguratorCallback func(name string, c *Configurator) error

func runConfiguratorCallCountTest(t *testing.T, name string, idx int, err error, expectation int, cb ConfiguratorCallback) {
	testUnitName := fmt.Sprintf("%s-%d", name, idx)
	plugin := MakeMockPlugin(testUnitName, err)
	c := NewConfigurator(testUnitName, plugin, nil)
	cb(name, c)
	callCount := GetCallCount(testUnitName)
	if callCount != expectation {
		t.Errorf("Configurator test %q:%d failed - got call count %d, expected %d", name, idx, callCount, expectation)
	}
}

func RunConfiguratorTest(t *testing.T, name string, cb ConfiguratorCallback) {
	idx := 0
	expectedErrors := []error{nil, fmt.Errorf("error-test-%s", name)}
	for _, err := range expectedErrors {
		for _, val := range []int{1, 1, 2, 3, 5, 8} {
			idx = idx + 1
			runConfiguratorCallCountTest(t, name, idx, err, val, func(n string, c *Configurator) error {
				for cnt := 0; cnt < val; cnt++ {
					reterr := cb(n, c)
					if nil != err && nil == reterr {
						t.Errorf("Configurator test %q failed - got no error, expected %v", name, err)
					}
				}
				return nil
			})
		}
	}
}

func TestNewConfigurator(t *testing.T) {
	plugin := &MockPlugin{}
	c := NewConfigurator("test-configurator", plugin, nil)
	if nil == c {
		t.Errorf("TestNewConfigurator failed - got nil, expected a new configurator instance")
	}
}

func TestConfiguratorGenerate(t *testing.T) {
	cb := func(n string, c *Configurator) error {
		_, err := c.Generate()
		return err
	}

	RunConfiguratorTest(t, "TestConfiguratorGenerate", cb)
}

func TestConfiguratorCreate(t *testing.T) {
	cb := func(n string, c *Configurator) error {
		return c.Create()
	}

	RunConfiguratorTest(t, "TestConfiguratorCreate", cb)
}
