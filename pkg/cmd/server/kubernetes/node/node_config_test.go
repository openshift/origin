package node

import (
	"io"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apiserver/pkg/util/flag"
	kubeproxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/fake"
	"k8s.io/kubernetes/pkg/kubelet/rkt"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
)

func TestKubeletDefaults(t *testing.T) {
	defaults := kubeletoptions.NewKubeletServer()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesNodeConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &kubeletoptions.KubeletServer{
		KubeletFlags: kubeletoptions.KubeletFlags{
			KubeConfig: flag.NewStringFlag("/var/lib/kubelet/kubeconfig"),
			ContainerRuntimeOptions: kubeletoptions.ContainerRuntimeOptions{
				DockershimRootDirectory:   "/var/lib/dockershim",
				DockerExecHandlerName:     "native",
				DockerEndpoint:            "unix:///var/run/docker.sock",
				ImagePullProgressDeadline: metav1.Duration{Duration: 1 * time.Minute},
				RktAPIEndpoint:            rkt.DefaultRktAPIServiceEndpoint,
				PodSandboxImage:           "gcr.io/google_containers/pause-amd64:3.0", // overridden
			},
		},

		KubeletConfiguration: componentconfig.KubeletConfiguration{
			Address:         "0.0.0.0", // overridden
			AllowPrivileged: false,     // overridden
			Authentication: componentconfig.KubeletAuthentication{
				Webhook: componentconfig.KubeletWebhookAuthentication{
					CacheTTL: metav1.Duration{Duration: 2 * time.Minute},
				},
				Anonymous: componentconfig.KubeletAnonymousAuthentication{
					Enabled: true,
				},
			},
			Authorization: componentconfig.KubeletAuthorization{
				Mode: componentconfig.KubeletAuthorizationModeAlwaysAllow,
				Webhook: componentconfig.KubeletWebhookAuthorization{
					CacheAuthorizedTTL:   metav1.Duration{Duration: 5 * time.Minute},
					CacheUnauthorizedTTL: metav1.Duration{Duration: 30 * time.Second},
				},
			},
			CAdvisorPort:         4194, // disabled
			VolumeStatsAggPeriod: metav1.Duration{Duration: time.Minute},
			CertDirectory:        "/var/run/kubernetes",
			CgroupRoot:           "",
			CgroupDriver:         "cgroupfs",
			ClusterDNS:           nil, // overridden
			ClusterDomain:        "",  // overridden
			ContainerRuntime:     "docker",
			Containerized:        false, // overridden based on OPENSHIFT_CONTAINERIZED
			CPUCFSQuota:          true,  // forced to true

			EventBurst:                     10,
			EventRecordQPS:                 5.0,
			EnableCustomMetrics:            false,
			EnableDebuggingHandlers:        true,
			EnableServer:                   true,
			EvictionHard:                   "memory.available<100Mi,nodefs.available<10%,nodefs.inodesFree<5%",
			FileCheckFrequency:             metav1.Duration{Duration: 20 * time.Second}, // overridden
			HealthzBindAddress:             "127.0.0.1",                                 // disabled
			HealthzPort:                    10248,                                       // disabled
			HostNetworkSources:             []string{"*"},                               // overridden
			HostPIDSources:                 []string{"*"},                               // overridden
			HostIPCSources:                 []string{"*"},                               // overridden
			HTTPCheckFrequency:             metav1.Duration{Duration: 20 * time.Second}, // disabled
			ImageMinimumGCAge:              metav1.Duration{Duration: 120 * time.Second},
			ImageGCHighThresholdPercent:    85,
			ImageGCLowThresholdPercent:     80,
			IPTablesMasqueradeBit:          14,
			IPTablesDropBit:                15,
			LowDiskSpaceThresholdMB:        0, // used to be 256.  Overriden to have old behavior. 3.7
			MakeIPTablesUtilChains:         true,
			MasterServiceNamespace:         "default",
			MaxContainerCount:              -1,
			MaxPerPodContainerCount:        1,
			MaxOpenFiles:                   1000000,
			MaxPods:                        110, // overridden
			MinimumGCAge:                   metav1.Duration{},
			NonMasqueradeCIDR:              "10.0.0.0/8",
			VolumePluginDir:                "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/",
			NodeStatusUpdateFrequency:      metav1.Duration{Duration: 10 * time.Second},
			NodeLabels:                     nil,
			OOMScoreAdj:                    -999,
			LockFilePath:                   "",
			Port:                           10250, // overridden
			ReadOnlyPort:                   10255, // disabled
			RegisterNode:                   true,
			RegisterSchedulable:            true,
			RegistryBurst:                  10,
			RegistryPullQPS:                5.0,
			RemoteRuntimeEndpoint:          "unix:///var/run/dockershim.sock", // overridden
			ResolverConfig:                 kubetypes.ResolvConfDefault,
			KubeletCgroups:                 "",
			CgroupsPerQOS:                  true,
			RootDirectory:                  "/var/lib/kubelet", // overridden
			RuntimeCgroups:                 "",
			SerializeImagePulls:            true,
			StreamingConnectionIdleTimeout: metav1.Duration{Duration: 4 * time.Hour},
			SyncFrequency:                  metav1.Duration{Duration: 1 * time.Minute},
			SystemCgroups:                  "",
			TLSCertFile:                    "", // overridden to prevent cert generation
			TLSPrivateKeyFile:              "", // overridden to prevent cert generation
			KubeAPIQPS:                     5.0,
			KubeAPIBurst:                   10,
			OutOfDiskTransitionFrequency:   metav1.Duration{Duration: 5 * time.Minute},
			HairpinMode:                    "promiscuous-bridge",
			SeccompProfileRoot:             "/var/lib/kubelet/seccomp",
			CloudProvider:                  "auto-detect",
			RuntimeRequestTimeout:          metav1.Duration{Duration: 2 * time.Minute},
			ContentType:                    "application/vnd.kubernetes.protobuf",
			EnableControllerAttachDetach:   true,
			ExperimentalQOSReserved:        componentconfig.ConfigurationMap{},

			EvictionPressureTransitionPeriod:    metav1.Duration{Duration: 5 * time.Minute},
			ExperimentalKernelMemcgNotification: false,

			SystemReserved: componentconfig.ConfigurationMap{},
			KubeReserved:   componentconfig.ConfigurationMap{},

			EnforceNodeAllocatable: []string{"pods"},
		},
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

	expectedProxyConfig := &componentconfig.KubeProxyConfiguration{
		BindAddress:        "0.0.0.0",
		HealthzBindAddress: "0.0.0.0:10256",   // disabled
		MetricsBindAddress: "127.0.0.1:10249", // disabled
		ClientConnection: componentconfig.ClientConnectionConfiguration{
			ContentType: "application/vnd.kubernetes.protobuf",
			QPS:         5,
			Burst:       10,
		},
		IPTables: componentconfig.KubeProxyIPTablesConfiguration{
			MasqueradeBit: &ipTablesMasqueratebit,
			SyncPeriod:    metav1.Duration{Duration: 30 * time.Second},
		},
		OOMScoreAdj:       &oomScoreAdj,  // disabled
		ResourceContainer: "/kube-proxy", // disabled
		UDPIdleTimeout:    metav1.Duration{Duration: 250 * time.Millisecond},
		Conntrack: componentconfig.KubeProxyConntrackConfiguration{
			Min:                   128 * 1024,
			MaxPerCore:            32 * 1024,
			TCPEstablishedTimeout: metav1.Duration{Duration: 86400 * time.Second}, // 1 day (1/5 default)
			TCPCloseWaitTimeout:   metav1.Duration{Duration: 1 * time.Hour},
		},
		ConfigSyncPeriod: metav1.Duration{Duration: 15 * time.Minute},
	}

	actualDefaultConfig, _ := kubeproxyoptions.NewOptions()
	actualConfig, _ := actualDefaultConfig.ApplyDefaults(actualDefaultConfig.GetConfig())

	if !reflect.DeepEqual(expectedProxyConfig, actualConfig) {
		t.Errorf("Default kube proxy config has changed. Adjust buildKubeProxyConfig() as needed to disable or make use of additions.")
		t.Logf("Difference %s", diff.ObjectReflectDiff(expectedProxyConfig, actualConfig))
	}

}

func TestBuildCloudProviderFake(t *testing.T) {
	providerName := "fake"
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		return &fake.FakeCloud{}, nil
	})

	server := kubeletoptions.NewKubeletServer()
	server.CloudProvider = providerName

	cloud, err := buildCloudProvider(server)
	if err != nil {
		t.Errorf("buildCloudProvider failed: %v", err)
	}
	if cloud == nil {
		t.Errorf("buildCloudProvider returned nil cloud provider")
	} else {
		if cloud.ProviderName() != providerName {
			t.Errorf("Invalid cloud provider returned, expected %q, got %q", providerName, cloud.ProviderName())
		}
	}
}

func TestBuildCloudProviderNone(t *testing.T) {
	server := kubeletoptions.NewKubeletServer()
	server.CloudProvider = ""
	cloud, err := buildCloudProvider(server)
	if err != nil {
		t.Errorf("buildCloudProvider failed: %v", err)
	}
	if cloud != nil {
		t.Errorf("buildCloudProvider returned cloud provider %q where nil was expected", cloud.ProviderName())
	}
}

func TestBuildCloudProviderError(t *testing.T) {
	server := kubeletoptions.NewKubeletServer()
	server.CloudProvider = "unknown-provider-name"
	cloud, err := buildCloudProvider(server)
	if err == nil {
		t.Errorf("buildCloudProvider returned no error when one was expected")
	}
	if cloud != nil {
		t.Errorf("buildCloudProvider returned cloud provider %q where nil was expected", cloud.ProviderName())
	}
}
