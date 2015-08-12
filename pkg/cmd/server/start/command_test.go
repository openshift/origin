package start

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/spf13/cobra"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

// this groups of methods force all the unit tests to share the same config directory
// the non-cert parts of the directory are cleaned up with every execution, but the certs
// remain in order to speed up the tests.
var configDir = ""

const nodeConfigGlob = "node-*/node-config.yaml"

func getNodeConfigGlob() string {
	return path.Join(getNodeConfigDir(), nodeConfigGlob)
}

func getConfigDir() string {
	if len(configDir) == 0 {
		configDir, _ = ioutil.TempDir("", "")
	}

	return configDir
}

func getAllInOneConfigDir() string {
	return getConfigDir()
}

func getMasterConfigDir() string {
	return path.Join(getConfigDir(), "master")
}

func getCleanAllInOneConfigDir() string {
	cleanupMasterConfigDir()
	return getAllInOneConfigDir()
}

func getCleanMasterConfigDir() string {
	cleanupMasterConfigDir()
	cleanupNodeConfigDirs()
	return getMasterConfigDir()
}

func getNodeConfigDir() string {
	return path.Join(getConfigDir(), "node")
}

func cleanupMasterConfigDir() {
	os.Remove(path.Join(getMasterConfigDir(), "policy.json"))
	os.Remove(path.Join(getMasterConfigDir(), "master-config.yaml"))
}
func cleanupNodeConfigDirs() {
	// no errors reported, just best effort
	nodeConfigs, _ := filepath.Glob(getNodeConfigGlob())
	for _, file := range nodeConfigs {
		os.Remove(file)
	}
}

func TestCommandBindingListen(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeMasterCommand([]string{"--listen=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.ListenArg.ListenAddr.Set(valueToSet)

	if expectedArgs.ListenArg.ListenAddr.String() != actualCfg.ListenArg.ListenAddr.String() {
		t.Errorf("expected %v, got %v", expectedArgs.ListenArg.ListenAddr.String(), actualCfg.ListenArg.ListenAddr.String())
	}
}

func TestCommandBindingMaster(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeMasterCommand([]string{"--master=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.MasterAddr.Set(valueToSet)

	if expectedArgs.MasterAddr.String() != actualCfg.MasterAddr.String() {
		t.Errorf("expected %v, got %v", expectedArgs.MasterAddr.String(), actualCfg.MasterAddr.String())
	}
}

func TestCommandBindingMasterPublic(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeMasterCommand([]string{"--public-master=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.MasterPublicAddr.Set(valueToSet)

	if expectedArgs.MasterPublicAddr.String() != actualCfg.MasterPublicAddr.String() {
		t.Errorf("expected %v, got %v", expectedArgs.MasterPublicAddr.String(), actualCfg.MasterPublicAddr.String())
	}
}

func TestCommandBindingEtcd(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeMasterCommand([]string{"--etcd=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.EtcdAddr.Set(valueToSet)

	if expectedArgs.EtcdAddr.String() != actualCfg.EtcdAddr.String() {
		t.Errorf("expected %v, got %v", expectedArgs.EtcdAddr.String(), actualCfg.EtcdAddr.String())
	}
}

func TestCommandBindingPortalNet(t *testing.T) {
	valueToSet := "192.168.0.0/16"
	actualCfg := executeMasterCommand([]string{"--portal-net=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.PortalNet.Set(valueToSet)

	if expectedArgs.PortalNet.String() != actualCfg.PortalNet.String() {
		t.Errorf("expected %v, got %v", expectedArgs.PortalNet.String(), actualCfg.PortalNet.String())
	}
}

func TestCommandBindingImageTemplateFormat(t *testing.T) {
	valueToSet := "some-format-string"
	actualCfg := executeMasterCommand([]string{"--images=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.ImageFormatArgs.ImageTemplate.Format = valueToSet

	if expectedArgs.ImageFormatArgs.ImageTemplate.Format != actualCfg.ImageFormatArgs.ImageTemplate.Format {
		t.Errorf("expected %v, got %v", expectedArgs.ImageFormatArgs.ImageTemplate.Format, actualCfg.ImageFormatArgs.ImageTemplate.Format)
	}
}

func TestCommandBindingImageLatest(t *testing.T) {
	expectedArgs := NewDefaultMasterArgs()

	valueToSet := strconv.FormatBool(!expectedArgs.ImageFormatArgs.ImageTemplate.Latest)
	actualCfg := executeMasterCommand([]string{"--latest-images=" + valueToSet})

	expectedArgs.ImageFormatArgs.ImageTemplate.Latest = !expectedArgs.ImageFormatArgs.ImageTemplate.Latest

	if expectedArgs.ImageFormatArgs.ImageTemplate.Latest != actualCfg.ImageFormatArgs.ImageTemplate.Latest {
		t.Errorf("expected %v, got %v", expectedArgs.ImageFormatArgs.ImageTemplate.Latest, actualCfg.ImageFormatArgs.ImageTemplate.Latest)
	}
}

func TestCommandBindingEtcdDir(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeMasterCommand([]string{"--etcd-dir=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.EtcdDir = valueToSet

	if expectedArgs.EtcdDir != actualCfg.EtcdDir {
		t.Errorf("expected %v, got %v", expectedArgs.EtcdDir, actualCfg.EtcdDir)
	}
}

// explicit start master never modifies the NodeList
func TestCommandBindingNodesForMaster(t *testing.T) {
	valueToSet := "first,second,third"
	actualCfg := executeMasterCommand([]string{"master", "--nodes=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.NodeList.Set(valueToSet)

	if expectedArgs.NodeList.String() != actualCfg.NodeList.String() {
		t.Errorf("expected %v, got %v", expectedArgs.NodeList, actualCfg.NodeList)
	}
}

// explicit start master never modifies the NodeList
func TestCommandBindingNodesDefaultingMaster(t *testing.T) {
	actualCfg := executeMasterCommand([]string{"master"})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.NodeList.Set("")

	if expectedArgs.NodeList.String() != actualCfg.NodeList.String() {
		t.Errorf("expected %v, got %v", expectedArgs.NodeList, actualCfg.NodeList)
	}
}

func TestCommandBindingCors(t *testing.T) {
	valueToSet := "first,second,third"
	actualCfg := executeMasterCommand([]string{"--cors-allowed-origins=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.CORSAllowedOrigins.Set(valueToSet)

	if expectedArgs.CORSAllowedOrigins.String() != actualCfg.CORSAllowedOrigins.String() {
		t.Errorf("expected %v, got %v", expectedArgs.CORSAllowedOrigins, actualCfg.CORSAllowedOrigins)
	}
}

func executeMasterCommand(args []string) *MasterArgs {
	argsToUse := make([]string, 0, 4+len(args))
	argsToUse = append(argsToUse, "master")
	argsToUse = append(argsToUse, args...)
	argsToUse = append(argsToUse, "--write-config="+getCleanMasterConfigDir())
	argsToUse = append(argsToUse, "--create-certs=false")

	root := &cobra.Command{
		Use:   "openshift",
		Short: "test",
		Long:  "",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	openshiftStartCommand, cfg := NewCommandStartMaster("openshift", os.Stdout)
	root.AddCommand(openshiftStartCommand)
	root.SetArgs(argsToUse)
	root.Execute()

	return cfg.MasterArgs
}

func executeAllInOneCommand(args []string) (*MasterArgs, *NodeArgs) {
	masterArgs, _, _, nodeArgs, _, _ := executeAllInOneCommandWithConfigs(args)
	return masterArgs, nodeArgs
}

func executeAllInOneCommandWithConfigs(args []string) (*MasterArgs, *configapi.MasterConfig, error, *NodeArgs, *configapi.NodeConfig, error) {
	argsToUse := make([]string, 0, 4+len(args))
	argsToUse = append(argsToUse, "start")
	argsToUse = append(argsToUse, args...)
	argsToUse = append(argsToUse, "--write-config="+getCleanAllInOneConfigDir())
	argsToUse = append(argsToUse, "--create-certs=false")

	root := &cobra.Command{
		Use:   "openshift",
		Short: "test",
		Long:  "",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	openshiftStartCommand, cfg := NewCommandStartAllInOne("openshift start", os.Stdout)
	root.AddCommand(openshiftStartCommand)
	root.SetArgs(argsToUse)
	root.Execute()

	masterCfg, masterErr := configapilatest.ReadAndResolveMasterConfig(path.Join(getAllInOneConfigDir(), "master", "master-config.yaml"))

	var nodeCfg *configapi.NodeConfig
	var nodeErr error

	nodeConfigs, nodeErr := filepath.Glob(getNodeConfigGlob())
	if nodeErr == nil {
		if len(nodeConfigs) != 1 {
			nodeErr = fmt.Errorf("found wrong number of node configs: %v", nodeConfigs)
		} else {
			nodeCfg, nodeErr = configapilatest.ReadAndResolveNodeConfig(nodeConfigs[0])
		}
	}

	if nodeCfg == nil && nodeErr == nil {
		nodeErr = errors.New("did not find node config")
	}
	return cfg.MasterOptions.MasterArgs, masterCfg, masterErr, cfg.NodeArgs, nodeCfg, nodeErr
}
