package start

import (
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	utilerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
)

func TestCommandBindingListenHttp(t *testing.T) {
	valueToSet := "http://example.org:9123"
	masterArgs, masterCfg, masterErr, nodeArgs, nodeCfg, nodeErr := executeAllInOneCommandWithConfigs([]string{"--listen=" + valueToSet})

	if masterErr != nil {
		t.Fatalf("Unexpected error: %v", masterErr)
	}
	if nodeErr != nil {
		t.Fatalf("Unexpected error: %v", nodeErr)
	}

	if configapi.UseTLS(masterCfg.ServingInfo) {
		t.Errorf("Unexpected TLS: %v", masterCfg.ServingInfo)
	}
	if configapi.UseTLS(masterCfg.AssetConfig.ServingInfo) {
		t.Errorf("Unexpected TLS: %v", masterCfg.AssetConfig.ServingInfo)
	}
	if configapi.UseTLS(nodeCfg.ServingInfo) {
		t.Errorf("Unexpected TLS: %v", nodeCfg.ServingInfo)
	}

	if masterArgs.ListenArg.ListenAddr.String() != valueToSet {
		t.Errorf("Expected %v, got %v", valueToSet, masterArgs.ListenArg.ListenAddr.String())
	}
	if nodeArgs.ListenArg.ListenAddr.String() != valueToSet {
		t.Errorf("Expected %v, got %v", valueToSet, nodeArgs.ListenArg.ListenAddr.String())
	}

	// Ensure there are no errors other than missing client kubeconfig files and missing bootstrap policy files
	masterErrs := validation.ValidateMasterConfig(masterCfg).Filter(func(e error) bool {
		return strings.Contains(e.Error(), "masterClients.") || strings.Contains(e.Error(), "policyConfig.bootstrapPolicyFile")
	})
	if len(masterErrs) != 0 {
		t.Errorf("Unexpected validation errors: %v", utilerrors.NewAggregate(masterErrs))
	}

	nodeErrs := validation.ValidateNodeConfig(nodeCfg).Filter(func(e error) bool {
		return strings.Contains(e.Error(), "masterKubeConfig")
	})
	if len(nodeErrs) != 0 {
		t.Errorf("Unexpected validation errors: %v", utilerrors.NewAggregate(nodeErrs))
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

func TestCommandBindingVolumeDir(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeNodeCommand([]string{"--volume-dir=" + valueToSet})

	expectedArgs := NewDefaultNodeArgs()
	expectedArgs.VolumeDir = valueToSet

	if expectedArgs.VolumeDir != actualCfg.VolumeDir {
		t.Errorf("expected %v, got %v", expectedArgs.VolumeDir, actualCfg.VolumeDir)
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

func TestCommandBindingCertDir(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeMasterCommand([]string{"--cert-dir=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.CertArgs.CertDir = valueToSet

	if expectedArgs.CertArgs.CertDir != actualCfg.CertArgs.CertDir {
		t.Errorf("expected %v, got %v", expectedArgs.CertArgs.CertDir, actualCfg.CertArgs.CertDir)
	}
}

func TestCommandBindingHostname(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeNodeCommand([]string{"--hostname=" + valueToSet})

	expectedArgs := NewDefaultNodeArgs()
	expectedArgs.NodeName = valueToSet

	if expectedArgs.NodeName != actualCfg.NodeName {
		t.Errorf("expected %v, got %v", expectedArgs.NodeName, actualCfg.NodeName)
	}
}

// AllInOne always adds the default hostname
func TestCommandBindingNodesForAllInOneAppend(t *testing.T) {
	valueToSet := "first,second,third"
	actualMasterCfg, actualNodeConfig := executeAllInOneCommand([]string{"--nodes=" + valueToSet})

	expectedArgs := NewDefaultMasterArgs()

	stringList := util.StringList{}
	stringList.Set(valueToSet + "," + strings.ToLower(actualNodeConfig.NodeName))
	expectedArgs.NodeList.Set(strings.Join(util.NewStringSet(stringList...).List(), ","))

	if expectedArgs.NodeList.String() != actualMasterCfg.NodeList.String() {
		t.Errorf("expected %v, got %v", expectedArgs.NodeList, actualMasterCfg.NodeList)
	}
}

// AllInOne always adds the default hostname
func TestCommandBindingNodesForAllInOneAppendNoDupes(t *testing.T) {
	valueToSet := "first,localhost,second,third"
	actualMasterCfg, _ := executeAllInOneCommand([]string{"--nodes=" + valueToSet, "--hostname=LOCALHOST"})

	expectedArgs := NewDefaultMasterArgs()
	expectedArgs.NodeList.Set(valueToSet)

	util.NewStringSet()

	if expectedArgs.NodeList.String() != actualMasterCfg.NodeList.String() {
		t.Errorf("expected %v, got %v", expectedArgs.NodeList, actualMasterCfg.NodeList)
	}
}

// AllInOne always adds the default hostname
func TestCommandBindingNodesDefaultingAllInOne(t *testing.T) {
	actualMasterCfg, _ := executeAllInOneCommand([]string{})

	expectedArgs := NewDefaultMasterArgs()
	expectedNodeArgs := NewDefaultNodeArgs()
	expectedArgs.NodeList.Set(strings.ToLower(expectedNodeArgs.NodeName))

	if expectedArgs.NodeList.String() != actualMasterCfg.NodeList.String() {
		t.Errorf("expected %v, got %v", expectedArgs.NodeList, actualMasterCfg.NodeList)
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
	fakeConfigFile, _ := ioutil.TempFile("", "")
	defer os.Remove(fakeConfigFile.Name())

	argsToUse := make([]string, 0, 4+len(args))
	argsToUse = append(argsToUse, "master")
	argsToUse = append(argsToUse, args...)
	argsToUse = append(argsToUse, "--write-config")
	argsToUse = append(argsToUse, "--create-policy-file=false")
	argsToUse = append(argsToUse, "--create-certs=false")
	argsToUse = append(argsToUse, "--config="+fakeConfigFile.Name())

	root := &cobra.Command{
		Use:   "openshift",
		Short: "test",
		Long:  "",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	openshiftStartCommand, cfg := NewCommandStartMaster()
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
	fakeMasterConfigFile, _ := ioutil.TempFile("", "")
	defer os.Remove(fakeMasterConfigFile.Name())
	fakeNodeConfigFile, _ := ioutil.TempFile("", "")
	defer os.Remove(fakeNodeConfigFile.Name())

	argsToUse := make([]string, 0, 4+len(args))
	argsToUse = append(argsToUse, "start")
	argsToUse = append(argsToUse, args...)
	argsToUse = append(argsToUse, "--write-config")
	argsToUse = append(argsToUse, "--create-certs=false")
	argsToUse = append(argsToUse, "--create-policy-file=false")
	argsToUse = append(argsToUse, "--master-config="+fakeMasterConfigFile.Name())
	argsToUse = append(argsToUse, "--node-config="+fakeNodeConfigFile.Name())

	root := &cobra.Command{
		Use:   "openshift",
		Short: "test",
		Long:  "",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	openshiftStartCommand, cfg := NewCommandStartAllInOne()
	root.AddCommand(openshiftStartCommand)
	root.SetArgs(argsToUse)
	root.Execute()

	masterCfg, masterErr := configapilatest.ReadAndResolveMasterConfig(fakeMasterConfigFile.Name())
	nodeCfg, nodeErr := configapilatest.ReadAndResolveNodeConfig(fakeNodeConfigFile.Name())

	return cfg.MasterArgs, masterCfg, masterErr, cfg.NodeArgs, nodeCfg, nodeErr
}

func executeNodeCommand(args []string) *NodeArgs {
	fakeConfigFile, _ := ioutil.TempFile("", "")
	defer os.Remove(fakeConfigFile.Name())

	argsToUse := make([]string, 0, 4+len(args))
	argsToUse = append(argsToUse, "node")
	argsToUse = append(argsToUse, args...)
	argsToUse = append(argsToUse, "--write-config")
	argsToUse = append(argsToUse, "--create-certs=false")
	argsToUse = append(argsToUse, "--config="+fakeConfigFile.Name())

	root := &cobra.Command{
		Use:   "openshift",
		Short: "test",
		Long:  "",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	openshiftStartCommand, cfg := NewCommandStartNode()
	root.AddCommand(openshiftStartCommand)
	root.SetArgs(argsToUse)
	root.Execute()

	return cfg.NodeArgs
}
