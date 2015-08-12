package ipfailover

import (
	"fmt"
	"io"
	"os"
	"testing"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

var callCounterMap = make(map[string]int, 0)

func GetCallCount(name string) int {
	counter, ok := callCounterMap[name]
	if ok {
		return counter
	}

	return 0
}

func IncrementCallCount(name string) {
	callCounterMap[name] = GetCallCount(name) + 1
}

type MockPlugin struct {
	Name             string
	Factory          *clientcmd.Factory
	Options          *IPFailoverConfigCmdOptions
	DeploymentConfig *deployapi.DeploymentConfig
	TestError        error
}

func (p MockPlugin) GetWatchPort() (int, error) {
	IncrementCallCount(p.Name)
	return p.Options.WatchPort, p.TestError
}

func (p MockPlugin) GetSelector() (map[string]string, error) {
	IncrementCallCount(p.Name)
	return map[string]string{DefaultName: p.Name}, p.TestError
}

func (p MockPlugin) GetNamespace() (string, error) {
	IncrementCallCount(p.Name)
	return "mock", p.TestError
}

func (p MockPlugin) GetDeploymentConfig() (*deployapi.DeploymentConfig, error) {
	IncrementCallCount(p.Name)
	return p.DeploymentConfig, p.TestError
}

func (p MockPlugin) Generate() (*kapi.List, error) {
	IncrementCallCount(p.Name)
	return &kapi.List{}, p.TestError
}

func (p MockPlugin) Create(out io.Writer) error {
	IncrementCallCount(p.Name)
	return p.TestError
}

func MakeMockPlugin(name string, err error) *MockPlugin {
	return &MockPlugin{
		Name:             name,
		Options:          &IPFailoverConfigCmdOptions{},
		DeploymentConfig: &deployapi.DeploymentConfig{},
		TestError:        err,
	}
}

type PluginCallback func(name string, p IPFailoverConfiguratorPlugin) error

func runPluginCallCountTest(t *testing.T, name string, idx int, err error, expectation int, cb PluginCallback) {
	testUnitName := fmt.Sprintf("%s-%d", name, idx)
	plugin := MakeMockPlugin(testUnitName, err)
	cb(testUnitName, *plugin)
	callCount := GetCallCount(testUnitName)
	if callCount != expectation {
		t.Errorf("Plugin test %q:%d failed - got call count %d, expected %d", name, idx, callCount, expectation)
	}
}

func TestNewPlugin(t *testing.T) {
	plugin := MakeMockPlugin("NewPlugin", nil)
	if nil == plugin {
		t.Errorf("Test for NewPlugin failed - got nil, expected a new plugin instance")
	}
}

func RunPluginInterfaceTest(t *testing.T, name string, cb PluginCallback) {
	idx := 0
	expectedErrors := []error{nil, fmt.Errorf("error-test-%f", name)}
	for _, err := range expectedErrors {
		for _, val := range []int{1, 1, 2, 3, 5, 8} {
			idx = idx + 1
			runPluginCallCountTest(t, name, idx, err, val, func(n string, p IPFailoverConfiguratorPlugin) error {
				for cnt := 0; cnt < val; cnt++ {
					reterr := cb(n, p)
					if nil != err && nil == reterr {
						t.Errorf("Test %q failed - got no error, expected %v", err)
					}
				}
				return nil
			})
		}
	}
}

func TestPluginGetWatchPort(t *testing.T) {
	cb := func(n string, p IPFailoverConfiguratorPlugin) error {
		_, err := p.GetWatchPort()
		return err
	}

	RunPluginInterfaceTest(t, "PluginGetWatchPort", cb)
}

func TestPluginGetSelector(t *testing.T) {
	cb := func(n string, p IPFailoverConfiguratorPlugin) error {
		_, err := p.GetSelector()
		return err
	}

	RunPluginInterfaceTest(t, "PluginGetSelector", cb)
}

func TestPluginGetNamespace(t *testing.T) {
	cb := func(n string, p IPFailoverConfiguratorPlugin) error {
		_, err := p.GetNamespace()
		return err
	}

	RunPluginInterfaceTest(t, "PluginGetNamespace", cb)
}

func TestPluginGetDeploymentConfig(t *testing.T) {
	cb := func(n string, p IPFailoverConfiguratorPlugin) error {
		_, err := p.GetDeploymentConfig()
		return err
	}

	RunPluginInterfaceTest(t, "PluginGetDeploymentConfig", cb)
}

func TestPluginGenerate(t *testing.T) {
	cb := func(n string, p IPFailoverConfiguratorPlugin) error {
		_, err := p.Generate()
		return err
	}

	RunPluginInterfaceTest(t, "PluginGenerate", cb)
}

func TestPluginCreate(t *testing.T) {
	cb := func(n string, p IPFailoverConfiguratorPlugin) error {
		return p.Create(os.Stderr)
	}

	RunPluginInterfaceTest(t, "PluginCreateStderr", cb)
}
