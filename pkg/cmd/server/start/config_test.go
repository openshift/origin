package start

import (
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/cmd/util"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
)

func TestMasterURLNoPathAllowed(t *testing.T) {
	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set("http://example.com:9012/")
	err := masterArgs.Validate()

	if err == nil || !strings.Contains(err.Error(), "may not include a path") {
		t.Errorf("expected %v, got %v", "may not include a path", err)
	}
}

func TestMasterPublicURLNoPathAllowed(t *testing.T) {
	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterPublicAddr.Set("http://example.com:9012/")
	err := masterArgs.Validate()

	if err == nil || !strings.Contains(err.Error(), "may not include a path") {
		t.Errorf("expected %v, got %v", "may not include a path", err)
	}
}

func TestMasterPublicAddressDefaulting(t *testing.T) {
	expected := "http://example.com:9012"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set(expected)

	actual, err := masterArgs.GetMasterPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestMasterPublicAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set("http://internal.com:9012")
	masterArgs.MasterPublicAddr.Set(expected)

	actual, err := masterArgs.GetMasterPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestAssetPublicAddressDefaulting(t *testing.T) {
	master := "http://example.com:9011"
	expected := "http://example.com:9011/console/"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set(master)

	actual, err := masterArgs.GetAssetPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesAddressDefaulting(t *testing.T) {
	expected := "http://example.com:9012"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set(expected)
	masterAddr, _ := masterArgs.GetMasterAddress()

	actual, err := masterArgs.KubeConnectionArgs.GetKubernetesAddress(masterAddr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdAddressDefaulting(t *testing.T) {
	expected := "https://example.com:4001"
	master := "http://example.com:9012"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set(master)

	actual, err := masterArgs.GetEtcdAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdAddressUseListenScheme(t *testing.T) {
	// Scheme from --listen arg
	// Host from --master arg
	// Port from default
	expected := "http://example.com:4001"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set("https://example.com:9012")
	masterArgs.ListenArg.ListenAddr.Set("http://0.0.0.0:8043")

	actual, err := masterArgs.GetEtcdAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set("http://internal.com:9012")
	masterArgs.EtcdAddr.Set(expected)

	actual, err := masterArgs.GetEtcdAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdBindAddressDefault(t *testing.T) {
	expected := "0.0.0.0:4001"

	masterArgs := NewDefaultMasterArgs()
	actual := masterArgs.GetEtcdBindAddress()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdPeerAddressDefault(t *testing.T) {
	expected := "0.0.0.0:7001"

	masterArgs := NewDefaultMasterArgs()
	actual := masterArgs.GetEtcdPeerBindAddress()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdBindAddressDefaultToBind(t *testing.T) {
	expected := "1.2.3.4:4001"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.ListenArg.ListenAddr.Set("https://1.2.3.4:8080")

	actual := masterArgs.GetEtcdBindAddress()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestMasterAddressDefaultingToBindValues(t *testing.T) {
	defaultIP, err := util.DefaultLocalIP4()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "http://" + defaultIP.String() + ":9012"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.ListenArg.ListenAddr.Set("http://0.0.0.0:9012")

	actual, err := masterArgs.GetMasterAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestMasterAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set(expected)

	actual, err := masterArgs.GetMasterAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubeClientForExternalKubernetesMasterWithNoConfig(t *testing.T) {
	expected := "https://localhost:8443"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set(expected)
	masterAddr, _ := masterArgs.GetMasterAddress()

	actual, err := masterArgs.KubeConnectionArgs.GetKubernetesAddress(masterAddr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubeClientForNodeWithNoConfig(t *testing.T) {
	expected := "https://localhost:8443"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.MasterAddr.Set(expected)
	masterAddr, _ := masterArgs.GetMasterAddress()

	actual, err := masterArgs.KubeConnectionArgs.GetKubernetesAddress(masterAddr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubeClientForExternalKubernetesMasterWithConfig(t *testing.T) {
	expectedServer := "https://some-other-server:1234"
	expectedUser := "myuser"

	masterArgs := NewDefaultMasterArgs()
	masterArgs.KubeConnectionArgs.ClientConfigLoadingRules, masterArgs.KubeConnectionArgs.ClientConfig = makeKubeconfig(expectedServer, expectedUser)

	masterAddr, _ := masterArgs.GetMasterAddress()

	actual, err := masterArgs.KubeConnectionArgs.GetKubernetesAddress(masterAddr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actual.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actual)
	}
}

func TestKubeClientForNodeWithConfig(t *testing.T) {
	expectedServer := "https://some-other-server:1234"
	expectedUser := "myuser"

	nodeArgs := NewDefaultNodeArgs()
	nodeArgs.KubeConnectionArgs.ClientConfigLoadingRules, nodeArgs.KubeConnectionArgs.ClientConfig = makeKubeconfig(expectedServer, expectedUser)

	actual, err := nodeArgs.KubeConnectionArgs.GetKubernetesAddress(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actual.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actual)
	}
}

func TestKubeClientForExternalKubernetesMasterWithErrorKubeconfig(t *testing.T) {
	masterArgs := NewDefaultMasterArgs()
	masterArgs.KubeConnectionArgs.ClientConfigLoadingRules, masterArgs.KubeConnectionArgs.ClientConfig = makeErrorKubeconfig()

	// GetKubernetesAddress hits the invalid kubeconfig in the fallback chain
	masterAddr, _ := masterArgs.GetMasterAddress()
	_, err := masterArgs.KubeConnectionArgs.GetKubernetesAddress(masterAddr)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func makeEmptyKubeconfig() (clientcmd.ClientConfigLoadingRules, clientcmd.ClientConfig) {
	// Set a non-empty CommandLinePath to trigger loading
	loadingRules := clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = "specified"

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		// Set empty loading rules to avoid missing file errors
		&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{},
	)
	return loadingRules, clientConfig
}

func makeErrorKubeconfig() (clientcmd.ClientConfigLoadingRules, clientcmd.ClientConfig) {
	// Set a non-empty CommandLinePath to trigger loading
	loadingRules := clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = "missing-file"

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&loadingRules,
		&clientcmd.ConfigOverrides{},
	)
	return loadingRules, clientConfig
}

func makeKubeconfig(server, user string) (clientcmd.ClientConfigLoadingRules, clientcmd.ClientConfig) {
	// Set a non-empty CommandLinePath to trigger loading
	loadingRules := clientcmd.ClientConfigLoadingRules{}
	loadingRules.ExplicitPath = "specified"

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		// Set empty loading rules to avoid missing file errors
		&clientcmd.ClientConfigLoadingRules{},
		// Override the server and user in client config to simulate loading from a file
		&clientcmd.ConfigOverrides{
			ClusterInfo: clientcmdapi.Cluster{Server: server},
			AuthInfo:    clientcmdapi.AuthInfo{Username: user},
		},
	)

	return loadingRules, clientConfig
}
