package kubernetes

import (
	"reflect"
	"testing"
	"time"

	proxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app/options"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/kubelet/qos"
)

func TestProxyConfig(t *testing.T) {
	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in buildKubeProxyConfig(), update this expected default to match the new upstream defaults
	oomScoreAdj := qos.KubeProxyOOMScoreAdj
	ipTablesMasqueratebit := 14
	expectedDefaultConfig := &proxyoptions.ProxyServerConfig{
		KubeProxyConfiguration: componentconfig.KubeProxyConfiguration{
			BindAddress:        "0.0.0.0",
			HealthzPort:        10249,
			HealthzBindAddress: "127.0.0.1",
			OOMScoreAdj:        &oomScoreAdj,
			ResourceContainer:  "/kube-proxy",
			IPTablesSyncPeriod: unversioned.Duration{Duration: 30 * time.Second},
			// from k8s.io/kubernetes/cmd/kube-proxy/app/options/options.go
			// defaults to 14.
			IPTablesMasqueradeBit:          &ipTablesMasqueratebit,
			UDPIdleTimeout:                 unversioned.Duration{Duration: 250 * time.Millisecond},
			ConntrackMax:                   256 * 1024,                                          // 4x default (64k)
			ConntrackTCPEstablishedTimeout: unversioned.Duration{Duration: 86400 * time.Second}, // 1 day (1/5 default)
		},
		ConfigSyncPeriod: 15 * time.Minute,
		KubeAPIQPS:       5.0,
		KubeAPIBurst:     10,
	}

	actualDefaultConfig := proxyoptions.NewProxyConfig()

	if !reflect.DeepEqual(expectedDefaultConfig, actualDefaultConfig) {
		t.Errorf("Default kube proxy config has changed. Adjust buildKubeProxyConfig() as needed to disable or make use of additions.")
		t.Logf("Expected default config:\n%#v\n\n", expectedDefaultConfig)
		t.Logf("Actual default config:\n%#v\n\n", actualDefaultConfig)
	}

}
