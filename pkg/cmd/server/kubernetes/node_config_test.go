package kubernetes

import (
	"net"
	"reflect"
	"testing"
	"time"

	proxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app/options"
	"k8s.io/kubernetes/pkg/kubelet/qos"
)

func TestProxyConfig(t *testing.T) {
	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in buildKubeProxyConfig(), update this expected default to match the new upstream defaults
	expectedDefaultConfig := &proxyoptions.ProxyServerConfig{
		BindAddress:                    net.ParseIP("0.0.0.0"),
		HealthzPort:                    10249,
		HealthzBindAddress:             net.ParseIP("127.0.0.1"),
		OOMScoreAdj:                    qos.KubeProxyOOMScoreAdj,
		ResourceContainer:              "/kube-proxy",
		IptablesSyncPeriod:             30 * time.Second,
		ConfigSyncPeriod:               15 * time.Minute,
		KubeAPIQPS:                     5.0,
		KubeAPIBurst:                   10,
		UDPIdleTimeout:                 250 * time.Millisecond,
		ConntrackMax:                   256 * 1024, // 4x default (64k)
		ConntrackTCPTimeoutEstablished: 86400,      // 1 day (1/5 default)
	}

	actualDefaultConfig := proxyoptions.NewProxyConfig()

	if !reflect.DeepEqual(expectedDefaultConfig, actualDefaultConfig) {
		t.Errorf("Default kube proxy config has changed. Adjust buildKubeProxyConfig() as needed to disable or make use of additions.")
		t.Logf("Expected default config:\n%#v\n\n", expectedDefaultConfig)
		t.Logf("Actual default config:\n%#v\n\n", actualDefaultConfig)
	}

}
