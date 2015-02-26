package server

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/openshift/origin/pkg/cmd/util"
)

func TestMasterPublicAddressDefaulting(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetMasterPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestMasterPublicAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.MasterPublicAddr.Set(expected)

	actual, err := cfg.GetMasterPublicAddress()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressDefaultToKubernetesAddress(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.KubernetesAddr.Set(expected)
	cfg.MasterPublicAddr.Set("unexpectedpublicmaster")
	cfg.MasterAddr.Set("unexpectedmaster")

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressDefaultToPublicMasterAddress(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterPublicAddr.Set(expected)
	cfg.MasterAddr.Set("unexpectedmaster")

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressDefaultToMasterAddress(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesPublicAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.KubernetesAddr.Set("http://internal.com:9013")
	cfg.MasterPublicAddr.Set("http://internal.com:9014")
	cfg.KubernetesPublicAddr.Set(expected)

	actual, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesAddressDefaulting(t *testing.T) {
	expected := "http://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubernetesAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.KubernetesAddr.Set(expected)

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdAddressDefaulting(t *testing.T) {
	expected := "http://example.com:4001"
	master := "https://example.com:9012"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(master)

	actual, err := cfg.GetEtcdAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set("http://internal.com:9012")
	cfg.EtcdAddr.Set(expected)

	actual, err := cfg.GetEtcdAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdBindAddressDefault(t *testing.T) {
	expected := "0.0.0.0:4001"

	cfg := NewDefaultConfig()
	actual := cfg.GetEtcdBindAddress()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdPeerAddressDefault(t *testing.T) {
	expected := "0.0.0.0:7001"

	cfg := NewDefaultConfig()
	actual := cfg.GetEtcdPeerBindAddress()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestEtcdBindAddressDefaultToBind(t *testing.T) {
	expected := "1.2.3.4:4001"

	cfg := NewDefaultConfig()
	cfg.BindAddr.Set("https://1.2.3.4:8080")

	actual := cfg.GetEtcdBindAddress()
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

	cfg := NewDefaultConfig()
	cfg.StartMaster = true
	cfg.BindAddr.Set("http://0.0.0.0:9012")

	actual, err := cfg.GetMasterAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestMasterAddressExplicit(t *testing.T) {
	expected := "http://external.com:12445"

	cfg := NewDefaultConfig()
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetMasterAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestKubeClientForExternalKubernetesMasterWithNoConfig(t *testing.T) {
	expected := "https://localhost:8443"

	cfg := NewDefaultConfig()
	cfg.StartMaster = true
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}

	_, config, err := cfg.GetKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != config.Host {
		t.Fatalf("expected %v, got %v", expected, config.Host)
	}
}

func TestKubeClientForNodeWithNoConfig(t *testing.T) {
	expected := "https://localhost:8443"

	cfg := NewDefaultConfig()
	cfg.StartNode = true
	cfg.MasterAddr.Set(expected)

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != actual.String() {
		t.Fatalf("expected %v, got %v", expected, actual)
	}

	_, config, err := cfg.GetKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expected != config.Host {
		t.Fatalf("expected %v, got %v", expected, config.Host)
	}
}

func TestKubeClientForExternalKubernetesMasterWithConfig(t *testing.T) {
	expectedServer := "https://some-other-server:1234"
	expectedUser := "myuser"

	cfg := NewDefaultConfig()
	cfg.StartMaster = true
	cfg.ClientConfigLoadingRules, cfg.ClientConfig = makeKubeconfig(expectedServer, expectedUser)

	actualPublic, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actualPublic.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actualPublic)
	}

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actual.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actual)
	}

	_, config, err := cfg.GetKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Host != expectedServer {
		t.Fatalf("expected %v, got %v", expectedServer, config.Host)
	}
	if config.Username != expectedUser {
		t.Fatalf("expected %v, got %v", expectedUser, config.Username)
	}
}

func TestKubeClientForNodeWithConfig(t *testing.T) {
	expectedServer := "https://some-other-server:1234"
	expectedUser := "myuser"

	cfg := NewDefaultConfig()
	cfg.StartNode = true
	cfg.ClientConfigLoadingRules, cfg.ClientConfig = makeKubeconfig(expectedServer, expectedUser)

	actualPublic, err := cfg.GetKubernetesPublicAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actualPublic.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actualPublic)
	}

	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actual.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actual)
	}

	_, config, err := cfg.GetKubeClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Host != expectedServer {
		t.Fatalf("expected %v, got %v", expectedServer, config.Host)
	}
	if config.Username != expectedUser {
		t.Fatalf("expected %v, got %v", expectedUser, config.Username)
	}
}

func TestKubeClientForExternalKubernetesMasterWithErrorKubeconfig(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.StartMaster = true
	cfg.ClientConfigLoadingRules, cfg.ClientConfig = makeErrorKubeconfig()

	// GetKubernetesPublicAddress hits the invalid kubeconfig in the fallback chain
	_, err := cfg.GetKubernetesPublicAddress()
	if err == nil {
		t.Fatalf("expected error, got none")
	}

	// GetKubernetesAddress hits the invalid kubeconfig in the fallback chain
	_, err = cfg.GetKubernetesAddress()
	if err == nil {
		t.Fatalf("expected error, got none")
	}

	// Should not get a client
	if _, _, err = cfg.GetKubeClient(); err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestKubeClientForExternalKubernetesMasterWithConflictingKubernetesAddress(t *testing.T) {
	expectedServer := "https://some-other-server:1234"
	expectedUser := "myuser"

	cfg := NewDefaultConfig()
	cfg.StartMaster = true
	// Explicitly set --kubernetes must match --kubeconfig or return an error
	cfg.KubernetesAddr.Set(expectedServer)
	cfg.ClientConfigLoadingRules, cfg.ClientConfig = makeKubeconfig("https://another-server:2345", expectedUser)

	// GetKubernetesAddress returns the explicitly set address
	actual, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actual.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actual)
	}

	// Should not get a client that might let us send credentials to the wrong server
	if _, _, err := cfg.GetKubeClient(); err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestKubeClientForNodeWithConflictingKubernetesAddress(t *testing.T) {
	expectedServer := "https://some-other-server:1234"
	expectedUser := "myuser"

	cfg := NewDefaultConfig()
	cfg.StartNode = true
	cfg.KubernetesAddr.Set(expectedServer)
	cfg.ClientConfigLoadingRules, cfg.ClientConfig = makeKubeconfig("https://another-server:2345", expectedUser)

	// GetKubernetesAddress returns the explicitly set address
	actualServer, err := cfg.GetKubernetesAddress()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expectedServer != actualServer.String() {
		t.Fatalf("expected %v, got %v", expectedServer, actualServer)
	}

	// Should not get a client that might let us send credentials to the wrong server
	if _, _, err := cfg.GetKubeClient(); err == nil {
		t.Fatalf("expected error, got none")
	}
}

func makeEmptyKubeconfig() (clientcmd.ClientConfigLoadingRules, clientcmd.ClientConfig) {
	// Set a non-empty CommandLinePath to trigger loading
	loadingRules := clientcmd.ClientConfigLoadingRules{CommandLinePath: "specified"}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		// Set empty loading rules to avoid missing file errors
		&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{},
	)
	return loadingRules, clientConfig
}

func makeErrorKubeconfig() (clientcmd.ClientConfigLoadingRules, clientcmd.ClientConfig) {
	// Set a non-empty CommandLinePath to trigger loading
	loadingRules := clientcmd.ClientConfigLoadingRules{CommandLinePath: "missing-file"}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&loadingRules,
		&clientcmd.ConfigOverrides{},
	)
	return loadingRules, clientConfig
}

func makeKubeconfig(server, user string) (clientcmd.ClientConfigLoadingRules, clientcmd.ClientConfig) {
	// Set a non-empty CommandLinePath to trigger loading
	loadingRules := clientcmd.ClientConfigLoadingRules{CommandLinePath: "specified"}

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
