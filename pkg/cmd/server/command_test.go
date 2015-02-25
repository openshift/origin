package server

import (
	"strconv"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandBindingListen(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeCommand([]string{"--listen=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.BindAddr.Set(valueToSet)

	if expectedConfig.BindAddr.String() != actualCfg.BindAddr.String() {
		t.Errorf("expected %v, got %v", expectedConfig.BindAddr.String(), actualCfg.BindAddr.String())
	}
}

func TestCommandBindingMaster(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeCommand([]string{"--master=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.MasterAddr.Set(valueToSet)

	if expectedConfig.MasterAddr.String() != actualCfg.MasterAddr.String() {
		t.Errorf("expected %v, got %v", expectedConfig.MasterAddr.String(), actualCfg.MasterAddr.String())
	}
}

func TestCommandBindingMasterPublic(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeCommand([]string{"--public-master=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.MasterPublicAddr.Set(valueToSet)

	if expectedConfig.MasterPublicAddr.String() != actualCfg.MasterPublicAddr.String() {
		t.Errorf("expected %v, got %v", expectedConfig.MasterPublicAddr.String(), actualCfg.MasterPublicAddr.String())
	}
}

func TestCommandBindingEtcd(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeCommand([]string{"--etcd=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.EtcdAddr.Set(valueToSet)

	if expectedConfig.EtcdAddr.String() != actualCfg.EtcdAddr.String() {
		t.Errorf("expected %v, got %v", expectedConfig.EtcdAddr.String(), actualCfg.EtcdAddr.String())
	}
}

func TestCommandBindingKubernetes(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeCommand([]string{"--kubernetes=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.KubernetesAddr.Set(valueToSet)

	if expectedConfig.KubernetesAddr.String() != actualCfg.KubernetesAddr.String() {
		t.Errorf("expected %v, got %v", expectedConfig.KubernetesAddr.String(), actualCfg.KubernetesAddr.String())
	}
}

func TestCommandBindingKubernetesPublic(t *testing.T) {
	valueToSet := "http://example.org:9123"
	actualCfg := executeCommand([]string{"--public-kubernetes=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.KubernetesPublicAddr.Set(valueToSet)

	if expectedConfig.KubernetesPublicAddr.String() != actualCfg.KubernetesPublicAddr.String() {
		t.Errorf("expected %v, got %v", expectedConfig.KubernetesPublicAddr.String(), actualCfg.KubernetesPublicAddr.String())
	}
}

func TestCommandBindingPortalNet(t *testing.T) {
	valueToSet := "192.168.0.0/16"
	actualCfg := executeCommand([]string{"--portal-net=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.PortalNet.Set(valueToSet)

	if expectedConfig.PortalNet.String() != actualCfg.PortalNet.String() {
		t.Errorf("expected %v, got %v", expectedConfig.PortalNet.String(), actualCfg.PortalNet.String())
	}
}

func TestCommandBindingImageTemplateFormat(t *testing.T) {
	valueToSet := "some-format-string"
	actualCfg := executeCommand([]string{"--images=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.ImageTemplate.Format = valueToSet

	if expectedConfig.ImageTemplate.Format != actualCfg.ImageTemplate.Format {
		t.Errorf("expected %v, got %v", expectedConfig.ImageTemplate.Format, actualCfg.ImageTemplate.Format)
	}
}

func TestCommandBindingImageLatest(t *testing.T) {
	expectedConfig := NewDefaultConfig()

	valueToSet := strconv.FormatBool(!expectedConfig.ImageTemplate.Latest)
	actualCfg := executeCommand([]string{"--latest-images=" + valueToSet})

	expectedConfig.ImageTemplate.Latest = !expectedConfig.ImageTemplate.Latest

	if expectedConfig.ImageTemplate.Latest != actualCfg.ImageTemplate.Latest {
		t.Errorf("expected %v, got %v", expectedConfig.ImageTemplate.Latest, actualCfg.ImageTemplate.Latest)
	}
}

func TestCommandBindingVolumeDir(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeCommand([]string{"--volume-dir=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.VolumeDir = valueToSet

	if expectedConfig.VolumeDir != actualCfg.VolumeDir {
		t.Errorf("expected %v, got %v", expectedConfig.VolumeDir, actualCfg.VolumeDir)
	}
}

func TestCommandBindingEtcdDir(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeCommand([]string{"--etcd-dir=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.EtcdDir = valueToSet

	if expectedConfig.EtcdDir != actualCfg.EtcdDir {
		t.Errorf("expected %v, got %v", expectedConfig.EtcdDir, actualCfg.EtcdDir)
	}
}

func TestCommandBindingCertDir(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeCommand([]string{"--cert-dir=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.CertDir = valueToSet

	if expectedConfig.CertDir != actualCfg.CertDir {
		t.Errorf("expected %v, got %v", expectedConfig.CertDir, actualCfg.CertDir)
	}
}

func TestCommandBindingHostname(t *testing.T) {
	valueToSet := "some-string"
	actualCfg := executeCommand([]string{"--hostname=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.Hostname = valueToSet

	if expectedConfig.Hostname != actualCfg.Hostname {
		t.Errorf("expected %v, got %v", expectedConfig.Hostname, actualCfg.Hostname)
	}
}

func TestCommandBindingNodes(t *testing.T) {
	valueToSet := "first,second,third"
	actualCfg := executeCommand([]string{"--nodes=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.NodeList.Set(valueToSet)

	if expectedConfig.NodeList.String() != actualCfg.NodeList.String() {
		t.Errorf("expected %v, got %v", expectedConfig.NodeList, actualCfg.NodeList)
	}
}

func TestCommandBindingCors(t *testing.T) {
	valueToSet := "first,second,third"
	actualCfg := executeCommand([]string{"--cors-allowed-origins=" + valueToSet})

	expectedConfig := NewDefaultConfig()
	expectedConfig.CORSAllowedOrigins.Set(valueToSet)

	if expectedConfig.CORSAllowedOrigins.String() != actualCfg.CORSAllowedOrigins.String() {
		t.Errorf("expected %v, got %v", expectedConfig.CORSAllowedOrigins, actualCfg.CORSAllowedOrigins)
	}
}

func TestCommandCompletionNode(t *testing.T) {
	commandCompletionTest{
		args: []string{"node"},

		StartNode: true,
	}.run(t)
}
func TestCommandCompletionMaster(t *testing.T) {
	commandCompletionTest{
		args: []string{"master"},

		StartMaster: true,
		StartKube:   true,
		StartEtcd:   true,
	}.run(t)
}
func TestCommandCompletionAllInOne(t *testing.T) {
	commandCompletionTest{
		StartNode:   true,
		StartMaster: true,
		StartKube:   true,
		StartEtcd:   true,
	}.run(t)
}

type commandCompletionTest struct {
	args []string

	StartNode   bool
	StartMaster bool
	StartKube   bool
	StartEtcd   bool
}

func executeCommand(args []string) *Config {
	argsToUse := make([]string, 0, 1+len(args))
	argsToUse = append(argsToUse, "start")
	argsToUse = append(argsToUse, args...)
	argsToUse = append(argsToUse, "--write-config-and-walk-away")

	root := &cobra.Command{
		Use:   "openshift",
		Short: "test",
		Long:  "",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	openshiftStartCommand, cfg := NewCommandStartServer("start")
	root.AddCommand(openshiftStartCommand)
	root.SetArgs(argsToUse)
	root.Execute()

	return cfg
}

func (test commandCompletionTest) run(t *testing.T) {
	actualCfg := executeCommand(test.args)

	if test.StartNode != actualCfg.StartNode {
		t.Errorf("expected %v, got %v", test.StartNode, actualCfg.StartNode)
	}
	if test.StartMaster != actualCfg.StartMaster {
		t.Errorf("expected %v, got %v", test.StartMaster, actualCfg.StartMaster)
	}
	if test.StartKube != actualCfg.StartKube {
		t.Errorf("expected %v, got %v", test.StartKube, actualCfg.StartKube)
	}
	if test.StartEtcd != actualCfg.StartEtcd {
		t.Errorf("expected %v, got %v", test.StartEtcd, actualCfg.StartEtcd)
	}

}
