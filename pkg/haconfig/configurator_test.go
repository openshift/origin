package haconfig

import (
	"io"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

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

func TestNewConfigurator(t *testing.T) {
	plugin := &MockPlugin{}
	c := NewConfigurator("test-configurator", plugin, nil)
	if nil == c {
		t.Errorf("Test for NewConfigurator failed - got nil, expected a new configurator instance")
	}
}

func makeMockPlugin(name string) *MockPlugin {
	return &MockPlugin{
		Name:      name,
		Options:   &HAConfigCmdOptions{},
		Service:   &kapi.Service{},
		CallCount: make(map[string]int, 0),
	}
}

type callback func(name string, c *Configurator)

func runCallCountTest(t *testing.T, name string, expectation int, cb callback) {
	plugin := makeMockPlugin(name)
	c := NewConfigurator(name, plugin, nil)
	cb(name, c)
	callCount := plugin.CallCount[name]
	if callCount != expectation {
		t.Errorf("Test for Generate failed - got call count %d, expected %d", callCount, expectation)
	}
}

func TestConfiguratorGenerate(t *testing.T) {
	runCallCountTest(t, "Generate", 1, func(n string, c *Configurator) {
		c.Generate()
	})
}

func TestConfiguratorCreate(t *testing.T) {
	runCallCountTest(t, "Create", 1, func(n string, c *Configurator) {
		c.Create()
	})
}

func TestConfiguratorDelete(t *testing.T) {
	runCallCountTest(t, "Delete", 1, func(n string, c *Configurator) {
		c.Delete()
	})
}
