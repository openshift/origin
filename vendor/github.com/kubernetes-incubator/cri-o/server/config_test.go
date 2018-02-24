package server

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/kubernetes-incubator/cri-o/lib"
)

const fixturePath = "fixtures/crio.conf"

func must(t *testing.T, err error) {
	if err != nil {
		t.Error(err)
	}
}

func assertAllFieldsEquality(t *testing.T, c Config) {
	testCases := []struct {
		fieldValue, expected interface{}
	}{
		{c.RootConfig.Root, "/var/lib/containers/storage"},
		{c.RootConfig.RunRoot, "/var/run/containers/storage"},
		{c.RootConfig.Storage, "overlay"},
		{c.RootConfig.StorageOptions[0], "overlay.override_kernel_check=1"},

		{c.APIConfig.Listen, "/var/run/crio.sock"},
		{c.APIConfig.StreamPort, "10010"},
		{c.APIConfig.StreamAddress, "localhost"},

		{c.RuntimeConfig.Runtime, "/usr/local/bin/runc"},
		{c.RuntimeConfig.RuntimeUntrustedWorkload, "untrusted"},
		{c.RuntimeConfig.DefaultWorkloadTrust, "trusted"},
		{c.RuntimeConfig.Conmon, "/usr/local/libexec/crio/conmon"},
		{c.RuntimeConfig.ConmonEnv[0], "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		{c.RuntimeConfig.SELinux, true},
		{c.RuntimeConfig.SeccompProfile, "/etc/crio/seccomp.json"},
		{c.RuntimeConfig.ApparmorProfile, "crio-default"},
		{c.RuntimeConfig.CgroupManager, "cgroupfs"},
		{c.RuntimeConfig.PidsLimit, int64(1024)},

		{c.ImageConfig.DefaultTransport, "docker://"},
		{c.ImageConfig.PauseImage, "kubernetes/pause"},
		{c.ImageConfig.PauseCommand, "/pause"},
		{c.ImageConfig.SignaturePolicyPath, "/tmp"},
		{c.ImageConfig.ImageVolumes, lib.ImageVolumesType("mkdir")},
		{c.ImageConfig.InsecureRegistries[0], "insecure-registry:1234"},
		{c.ImageConfig.Registries[0], "registry:4321"},

		{c.NetworkConfig.NetworkDir, "/etc/cni/net.d/"},
		{c.NetworkConfig.PluginDir, "/opt/cni/bin/"},
	}
	for _, tc := range testCases {
		if tc.fieldValue != tc.expected {
			t.Errorf(`Expecting: "%s", got: "%s"`, tc.expected, tc.fieldValue)
		}
	}
}

func TestUpdateFromFile(t *testing.T) {
	c := Config{}

	must(t, c.UpdateFromFile(fixturePath))

	assertAllFieldsEquality(t, c)
}

func TestToFile(t *testing.T) {
	configFromFixture := Config{}

	must(t, configFromFixture.UpdateFromFile(fixturePath))

	f, err := ioutil.TempFile("", "crio.conf")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())

	must(t, configFromFixture.ToFile(f.Name()))

	writtenConfig := Config{}
	err = writtenConfig.UpdateFromFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	assertAllFieldsEquality(t, writtenConfig)
}
