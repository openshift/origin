package node

import (
	"reflect"
	goruntime "runtime"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	kubeproxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet/apis/kubeletconfig"
	"k8s.io/kubernetes/pkg/kubelet/config"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	kubeletutil "k8s.io/kubernetes/pkg/kubelet/util"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"
)

func TestKubeletDefaults(t *testing.T) {
	defaults, _ := kubeletoptions.NewKubeletServer()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesNodeConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &kubeletoptions.KubeletServer{
		KubeletFlags: kubeletoptions.KubeletFlags{
			KubeConfig: "", // overridden
			ContainerRuntimeOptions: config.ContainerRuntimeOptions{
				DockershimRootDirectory:   "/var/lib/dockershim",
				DockerEndpoint:            "unix:///var/run/docker.sock",
				ImagePullProgressDeadline: metav1.Duration{Duration: 1 * time.Minute},
				PodSandboxImage:           "k8s.gcr.io/pause:3.1", // overridden
				DockerDisableSharedPID:    true,
				ContainerRuntime:          "docker",
				RuntimeCgroups:            "",
			},
			CloudProvider:           "", // now disabled
			RootDirectory:           "/var/lib/kubelet",
			CertDirectory:           "/var/lib/kubelet/pki",
			RegisterNode:            true,
			RemoteRuntimeEndpoint:   "unix:///var/run/dockershim.sock", // overridden
			Containerized:           false,                             // overridden based on OPENSHIFT_CONTAINERIZED
			VolumePluginDir:         "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/",
			SeccompProfileRoot:      "/var/lib/kubelet/seccomp",
			MaxContainerCount:       -1,
			MasterServiceNamespace:  "default",
			NodeLabels:              map[string]string{},
			MaxPerPodContainerCount: 1,
			NodeStatusMaxImages:     50,
			RegisterSchedulable:     true,
			NonMasqueradeCIDR:       "10.0.0.0/8",
			AllowPrivileged:         true, // overridden
			CAdvisorPort:            0,    // disabled
			EnableServer:            true,
			HostNetworkSources:      []string{"*"}, // overridden
			HostPIDSources:          []string{"*"}, // overridden
			HostIPCSources:          []string{"*"}, // overridden
		},

		KubeletConfiguration: kubeletconfig.KubeletConfiguration{
			Address: "0.0.0.0", // overridden
			Authentication: kubeletconfig.KubeletAuthentication{
				Webhook: kubeletconfig.KubeletWebhookAuthentication{
					CacheTTL: metav1.Duration{Duration: 2 * time.Minute},
				},
				Anonymous: kubeletconfig.KubeletAnonymousAuthentication{
					Enabled: true,
				},
			},
			Authorization: kubeletconfig.KubeletAuthorization{
				Mode: kubeletconfig.KubeletAuthorizationModeAlwaysAllow,
				Webhook: kubeletconfig.KubeletWebhookAuthorization{
					CacheAuthorizedTTL:   metav1.Duration{Duration: 5 * time.Minute},
					CacheUnauthorizedTTL: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			VolumeStatsAggPeriod: metav1.Duration{Duration: time.Minute},
			CgroupRoot:           "",
			CgroupDriver:         "cgroupfs",
			ClusterDNS:           nil,  // overridden
			ClusterDomain:        "",   // overridden
			CPUCFSQuota:          true, // forced to true

			EventBurst:     10,
			EventRecordQPS: 5.0,

			EnableDebuggingHandlers: true,
			EvictionHard: map[string]string{
				"memory.available":  "100Mi",
				"nodefs.available":  "10%",
				"nodefs.inodesFree": "5%",
				"imagefs.available": "15%",
			},
			FileCheckFrequency:          metav1.Duration{Duration: 20 * time.Second}, // overridden
			HealthzBindAddress:          "127.0.0.1",                                 // disabled
			HealthzPort:                 10248,                                       // disabled
			HTTPCheckFrequency:          metav1.Duration{Duration: 20 * time.Second}, // disabled
			ImageMinimumGCAge:           metav1.Duration{Duration: 120 * time.Second},
			ImageGCHighThresholdPercent: 85,
			ImageGCLowThresholdPercent:  80,
			IPTablesMasqueradeBit:       14,
			IPTablesDropBit:             15,
			PodPidsLimit:                -1,
			FailSwapOn:                  true, // overridden
			ContainerLogMaxSize:         "10Mi",
			ContainerLogMaxFiles:        5,

			MakeIPTablesUtilChains:    true,
			MaxOpenFiles:              1000000,
			MaxPods:                   110, // overridden
			NodeStatusUpdateFrequency: metav1.Duration{Duration: 10 * time.Second},
			OOMScoreAdj:               -999,
			Port:                      10250, // overridden
			ReadOnlyPort:              10255, // disabled
			RegistryBurst:             10,
			RegistryPullQPS:           5.0,
			ResolverConfig:            kubetypes.ResolvConfDefault,
			KubeletCgroups:            "",
			CgroupsPerQOS:             true,

			SerializeImagePulls:            true,
			StreamingConnectionIdleTimeout: metav1.Duration{Duration: 4 * time.Hour},
			SyncFrequency:                  metav1.Duration{Duration: 1 * time.Minute},
			SystemCgroups:                  "",
			TLSCertFile:                    "", // overridden to prevent cert generation
			TLSPrivateKeyFile:              "", // overridden to prevent cert generation
			KubeAPIQPS:                     5.0,
			KubeAPIBurst:                   10,
			HairpinMode:                    "promiscuous-bridge",
			RuntimeRequestTimeout:          metav1.Duration{Duration: 2 * time.Minute},
			ContentType:                    "application/vnd.kubernetes.protobuf",
			EnableControllerAttachDetach:   true,

			EvictionPressureTransitionPeriod: metav1.Duration{Duration: 5 * time.Minute},

			SystemReserved: nil,
			KubeReserved:   nil,

			EnforceNodeAllocatable: []string{"pods"},

			CPUManagerReconcilePeriod: metav1.Duration{Duration: 10 * time.Second},
			CPUManagerPolicy:          "none",
		},
	}

	if goruntime.GOOS == "darwin" {
		expectedDefaults.KubeletFlags.RemoteRuntimeEndpoint = ""
	}

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesNodeConfig and update expectedDefaults")
	}
}

func TestProxyConfig(t *testing.T) {
	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in buildKubeProxyConfig(), update this expected default to match the new upstream defaults
	oomScoreAdj := int32(-999)
	ipTablesMasqueratebit := int32(14)
	conntrackMin := int32(128 * 1024)
	conntrackMaxPerCore := int32(32 * 1024)

	expectedProxyConfig := &kubeproxyconfig.KubeProxyConfiguration{
		BindAddress:        "0.0.0.0",
		HealthzBindAddress: "0.0.0.0:10256",   // disabled
		MetricsBindAddress: "127.0.0.1:10249", // disabled
		FeatureGates:       map[string]bool{},
		ClientConnection: kubeproxyconfig.ClientConnectionConfiguration{
			ContentType: "application/vnd.kubernetes.protobuf",
			QPS:         5,
			Burst:       10,
		},
		IPTables: kubeproxyconfig.KubeProxyIPTablesConfiguration{
			MasqueradeBit: &ipTablesMasqueratebit,
			SyncPeriod:    metav1.Duration{Duration: 30 * time.Second},
		},
		IPVS: kubeproxyconfig.KubeProxyIPVSConfiguration{
			SyncPeriod: metav1.Duration{Duration: 30 * time.Second},
		},
		OOMScoreAdj:       &oomScoreAdj,  // disabled
		ResourceContainer: "/kube-proxy", // disabled
		UDPIdleTimeout:    metav1.Duration{Duration: 250 * time.Millisecond},
		Conntrack: kubeproxyconfig.KubeProxyConntrackConfiguration{
			Min:                   &conntrackMin,
			MaxPerCore:            &conntrackMaxPerCore,
			TCPEstablishedTimeout: &metav1.Duration{Duration: 86400 * time.Second}, // 1 day (1/5 default)
			TCPCloseWaitTimeout:   &metav1.Duration{Duration: 1 * time.Hour},
		},
		ConfigSyncPeriod: metav1.Duration{Duration: 15 * time.Minute},
	}

	actualDefaultConfig := kubeproxyoptions.NewOptions()
	actualConfig, _ := actualDefaultConfig.ApplyDefaults(actualDefaultConfig.GetConfig())

	if !reflect.DeepEqual(expectedProxyConfig, actualConfig) {
		t.Errorf("Default kube proxy config has changed. Adjust buildKubeProxyConfig() as needed to disable or make use of additions.")
		t.Logf("Difference %s", diff.ObjectReflectDiff(expectedProxyConfig, actualConfig))
	}

}

func TestRuntimeEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		expectError  bool
		expectedAddr string
	}{
		{
			// Current default format
			name:         "Endpoint format that includes protocol",
			endpoint:     "unix:///var/run/dockershim.sock",
			expectedAddr: "/var/run/dockershim.sock",
		},
		{
			// Deprecated format, this ensures old/existing configs are not broken
			name:         "Endpoint format that does not include protocol",
			endpoint:     "/var/run/dockershim.sock",
			expectedAddr: "/var/run/dockershim.sock",
		},
		{
			name:        "Endpoint with invalid protocol",
			endpoint:    "tcp1://foo",
			expectError: true,
		},
		{
			name:        "Invalid endpoint",
			endpoint:    "foo bar",
			expectError: true,
		},
	}

	for _, test := range tests {
		addr, _, err := kubeletutil.GetAddressAndDialer(test.endpoint)
		if err != nil && !test.expectError {
			t.Errorf("Test: %q expected to pass but received error %#v, endpoint: %s", test.name, err, test.endpoint)
			continue
		} else if err == nil && test.expectError {
			t.Errorf("Test: %q expected to fail but did not receive an error, endpoint: %s", test.name, test.endpoint)
		}

		if test.expectedAddr != addr {
			t.Errorf("Test: %q expected addr %s but got %s", test.name, test.expectedAddr, addr)
		}
	}
}
