package master

import (
	"net"
	"reflect"
	"sort"
	"testing"
	"time"

	apiv1 "k8s.io/api/core/v1"
	extensionsapiv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	utilconfig "k8s.io/apiserver/pkg/util/flag"
	pluginbuffered "k8s.io/apiserver/plugin/pkg/audit/buffered"
	plugintruncate "k8s.io/apiserver/plugin/pkg/audit/truncate"
	cmoptions "k8s.io/kubernetes/cmd/controller-manager/app/options"
	kubeapiservercmdoptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	scheduleroptions "k8s.io/kubernetes/cmd/kube-scheduler/app/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kubeapiserveroptions "k8s.io/kubernetes/pkg/kubeapiserver/options"
	kubeoptions "k8s.io/kubernetes/pkg/kubeapiserver/options"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"

	"github.com/cloudflare/cfssl/helpers"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

func TestAPIServerDefaults(t *testing.T) {
	defaults := kubeapiservercmdoptions.NewServerRunOptions()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &kubeapiservercmdoptions.ServerRunOptions{
		ServiceClusterIPRange: net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}, // overridden
		ServiceNodePortRange:  utilnet.PortRange{Base: 30000, Size: 2768},                         // overridden
		MasterCount:           1,                                                                  // ignored
		GenericServerRunOptions: &apiserveroptions.ServerRunOptions{
			MaxRequestsInFlight:         400,
			MaxMutatingRequestsInFlight: 200,
			MinRequestTimeout:           1800,
			RequestTimeout:              time.Duration(60) * time.Second,
			JSONPatchMaxCopyBytes:       int64(100 * 1024 * 1024),
			MaxRequestBodyBytes:         int64(100 * 1024 * 1024),
		},
		Admission: &kubeapiserveroptions.AdmissionOptions{
			PluginNames: nil, // ignored
			GenericAdmission: &apiserveroptions.AdmissionOptions{
				RecommendedPluginOrder: kubeapiserveroptions.AllOrderedPlugins,            // ignored
				DefaultOffPlugins:      kubeapiserveroptions.DefaultOffAdmissionPlugins(), // ignored
			},
		},
		Etcd: &apiserveroptions.EtcdOptions{
			StorageConfig: storagebackend.Config{
				ServerList: nil,
				Prefix:     "/registry",
				DeserializationCacheSize: 0,
				Quorum:                true,
				CompactionInterval:    300000000000, // five minutes
				CountMetricPollPeriod: 60000000000,  // one minute
			},
			DefaultStorageMediaType: "application/vnd.kubernetes.protobuf",
			DeleteCollectionWorkers: 1,
			EnableGarbageCollection: true,
			EnableWatchCache:        true,
			DefaultWatchCacheSize:   100,
		},
		SecureServing: &apiserveroptions.SecureServingOptionsWithLoopback{
			SecureServingOptions: &apiserveroptions.SecureServingOptions{
				BindAddress: net.ParseIP("0.0.0.0"),
				BindPort:    6443,
				ServerCert: apiserveroptions.GeneratableKeyCert{
					CertDirectory: "/var/run/kubernetes",
					PairName:      "apiserver",
				},
			},
		},
		InsecureServing: &kubeoptions.InsecureServingOptions{
			BindAddress: net.ParseIP("127.0.0.1"),
			BindPort:    8080,
		},
		EndpointReconcilerType: "lease", //ignored
		EventTTL:               1 * time.Hour,
		KubeletConfig: kubeletclient.KubeletClientConfig{
			Port:         10250,
			ReadOnlyPort: 10255,
			PreferredAddressTypes: []string{
				string(apiv1.NodeHostName),
				string(apiv1.NodeInternalDNS),
				string(apiv1.NodeInternalIP),
				string(apiv1.NodeExternalDNS),
				string(apiv1.NodeExternalIP),
			},
			EnableHttps: true,
			HTTPTimeout: time.Duration(5) * time.Second,
		},
		// we currently overwrite this entire stanza, but we should be trying to collapse onto the upstream
		// flag or config mechanism for kube.
		Audit: &apiserveroptions.AuditOptions{
			LogOptions: apiserveroptions.AuditLogOptions{
				GroupVersionString: "audit.k8s.io/v1beta1",
				Format:             "json",
				BatchOptions: apiserveroptions.AuditBatchOptions{
					Mode: "blocking",
					BatchConfig: pluginbuffered.BatchConfig{
						BufferSize:    10000,
						MaxBatchSize:  400,
						MaxBatchWait:  time.Duration(30000000000),
						ThrottleQPS:   10,
						ThrottleBurst: 15,
					},
				},
				TruncateOptions: apiserveroptions.AuditTruncateOptions{
					TruncateConfig: plugintruncate.Config{
						MaxEventSize: 102400,
						MaxBatchSize: 10485760,
					},
				},
			},
			WebhookOptions: apiserveroptions.AuditWebhookOptions{
				GroupVersionString: "audit.k8s.io/v1beta1",
				BatchOptions: apiserveroptions.AuditBatchOptions{
					Mode: "batch",
					BatchConfig: pluginbuffered.BatchConfig{
						BufferSize:     10000,
						MaxBatchSize:   400,
						MaxBatchWait:   time.Duration(30000000000),
						ThrottleEnable: true,
						ThrottleQPS:    10,
						ThrottleBurst:  15,
					},
				},
				TruncateOptions: apiserveroptions.AuditTruncateOptions{
					TruncateConfig: plugintruncate.Config{
						MaxEventSize: 102400,
						MaxBatchSize: 10485760,
					},
				},
				InitialBackoff: time.Duration(10000000000),
			},
		},
		Features: &apiserveroptions.FeatureOptions{
			EnableProfiling: true,
		},
		Authentication: &kubeoptions.BuiltInAuthenticationOptions{
			Anonymous:      &kubeoptions.AnonymousAuthenticationOptions{Allow: true},
			BootstrapToken: &kubeoptions.BootstrapTokenAuthenticationOptions{},
			ClientCert:     &apiserveroptions.ClientCertAuthenticationOptions{},
			OIDC:           &kubeoptions.OIDCAuthenticationOptions{},
			PasswordFile:   &kubeoptions.PasswordFileAuthenticationOptions{},
			RequestHeader:  &apiserveroptions.RequestHeaderAuthenticationOptions{},
			ServiceAccounts: &kubeoptions.ServiceAccountAuthenticationOptions{
				Lookup: true,
			},
			TokenFile: &kubeoptions.TokenFileAuthenticationOptions{},
			WebHook:   &kubeoptions.WebHookAuthenticationOptions{CacheTTL: 2 * time.Minute},

			TokenSuccessCacheTTL: 10 * time.Second,
			TokenFailureCacheTTL: 0,
		},
		Authorization: &kubeoptions.BuiltInAuthorizationOptions{
			Modes: []string{"AlwaysAllow"},
			WebhookCacheAuthorizedTTL:   5 * time.Minute,
			WebhookCacheUnauthorizedTTL: 30 * time.Second,
		},
		CloudProvider: &kubeoptions.CloudProviderOptions{},
		StorageSerialization: &kubeoptions.StorageSerializationOptions{
			StorageVersions:        kubeapiserveroptions.ToPreferredVersionString(legacyscheme.Scheme.PreferredVersionAllGroups()),
			DefaultStorageVersions: kubeapiserveroptions.ToPreferredVersionString(legacyscheme.Scheme.PreferredVersionAllGroups()),
		},
		APIEnablement: &apiserveroptions.APIEnablementOptions{
			RuntimeConfig: utilconfig.ConfigurationMap{},
		},
		EnableLogsHandler: true, // we disable this
	}

	// clear the non-serializeable bit
	defaults.Admission.GenericAdmission.Plugins = nil

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesMasterConfig and update expectedDefaults")
	}
}

// sortedGCIgnoredResources sorts by Group, then Resource.
type sortedGCIgnoredResources []componentconfig.GroupResource

func (r sortedGCIgnoredResources) Len() int {
	return len(r)
}

func (r sortedGCIgnoredResources) Less(i, j int) bool {
	if r[i].Group < r[j].Group {
		return true
	} else if r[i].Group > r[j].Group {
		return false
	}

	return r[i].Resource < r[j].Resource
}

func (r sortedGCIgnoredResources) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func TestCMServerDefaults(t *testing.T) {
	defaults, err := cmapp.NewKubeControllerManagerOptions()
	if err != nil {
		t.Fatal(err)
	}

	// We need to sort GCIgnoredResources because it's built from a map, which means the insertion
	// order is random.
	sort.Sort(sortedGCIgnoredResources(defaults.GarbageCollectorController.GCIgnoredResources))

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &cmapp.KubeControllerManagerOptions{
		SecureServing: &apiserveroptions.SecureServingOptions{
			BindAddress: net.ParseIP("0.0.0.0"),
			BindPort:    0,
			ServerCert: apiserveroptions.GeneratableKeyCert{
				PairName:      "kube-controller-manager",
				CertDirectory: "/var/run/kubernetes",
			},
		},
		InsecureServing: &cmoptions.InsecureServingOptions{
			BindAddress: net.ParseIP("0.0.0.0"),
			BindPort:    10252,
			BindNetwork: "tcp",
		},
		Controllers:   []string{"*"},
		CloudProvider: &cmoptions.CloudProviderOptions{},
		Debugging:     &cmoptions.DebuggingOptions{},
		GenericComponent: &cmoptions.GenericComponentConfigOptions{
			KubeAPIQPS:      20.0,
			KubeAPIBurst:    30,
			ContentType:     "application/vnd.kubernetes.protobuf",
			MinResyncPeriod: metav1.Duration{Duration: 12 * time.Hour},
			LeaderElection: componentconfig.LeaderElectionConfiguration{
				ResourceLock:  "endpoints",
				LeaderElect:   true,
				LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
				RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
				RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
			},
		},
		KubeCloudShared: &cmoptions.KubeCloudSharedOptions{
			Address: "0.0.0.0",
			Port:    10252, // disabled
			RouteReconciliationPeriod: metav1.Duration{Duration: 10 * time.Second},
			NodeMonitorPeriod:         metav1.Duration{Duration: 5 * time.Second},
			ClusterName:               "kubernetes",
			ConfigureCloudRoutes:      true,
		},

		AttachDetachController: &cmoptions.AttachDetachControllerOptions{
			ReconcilerSyncLoopPeriod: metav1.Duration{Duration: 60 * time.Second},
		},
		CSRSigningController: &cmoptions.CSRSigningControllerOptions{
			ClusterSigningCertFile: "/etc/kubernetes/ca/ca.pem",
			ClusterSigningKeyFile:  "/etc/kubernetes/ca/ca.key",
			ClusterSigningDuration: metav1.Duration{Duration: helpers.OneYear},
		},
		DaemonSetController: &cmoptions.DaemonSetControllerOptions{
			ConcurrentDaemonSetSyncs: 2,
		},
		DeploymentController: &cmoptions.DeploymentControllerOptions{
			ConcurrentDeploymentSyncs:      5,
			DeploymentControllerSyncPeriod: metav1.Duration{Duration: 30 * time.Second},
		},
		DeprecatedFlags: &cmoptions.DeprecatedControllerOptions{
			RegisterRetryCount: 10,
		},
		EndPointController: &cmoptions.EndPointControllerOptions{
			ConcurrentEndpointSyncs: 5,
		},
		GarbageCollectorController: &cmoptions.GarbageCollectorControllerOptions{
			ConcurrentGCSyncs:      20,
			EnableGarbageCollector: true,
			GCIgnoredResources: []componentconfig.GroupResource{
				{Group: "extensions", Resource: "replicationcontrollers"},
				{Group: "", Resource: "bindings"},
				{Group: "", Resource: "componentstatuses"},
				{Group: "", Resource: "events"},
				{Group: "authentication.k8s.io", Resource: "tokenreviews"},
				{Group: "authorization.k8s.io", Resource: "subjectaccessreviews"},
				{Group: "authorization.k8s.io", Resource: "selfsubjectaccessreviews"},
				{Group: "authorization.k8s.io", Resource: "localsubjectaccessreviews"},
				{Group: "authorization.k8s.io", Resource: "selfsubjectrulesreviews"},
			},
		},
		HPAController: &cmoptions.HPAControllerOptions{
			HorizontalPodAutoscalerSyncPeriod:               metav1.Duration{Duration: 30 * time.Second},
			HorizontalPodAutoscalerUpscaleForbiddenWindow:   metav1.Duration{Duration: 3 * time.Minute},
			HorizontalPodAutoscalerDownscaleForbiddenWindow: metav1.Duration{Duration: 5 * time.Minute},
			HorizontalPodAutoscalerTolerance:                0.1,
			HorizontalPodAutoscalerUseRESTClients:           true, // we ignore this for now
		},
		JobController: &cmoptions.JobControllerOptions{
			ConcurrentJobSyncs: 5,
		},
		NamespaceController: &cmoptions.NamespaceControllerOptions{
			ConcurrentNamespaceSyncs: 10,
			NamespaceSyncPeriod:      metav1.Duration{Duration: 5 * time.Minute},
		},
		NodeIpamController: &cmoptions.NodeIpamControllerOptions{
			NodeCIDRMaskSize: 24,
		},
		NodeLifecycleController: &cmoptions.NodeLifecycleControllerOptions{
			NodeMonitorGracePeriod: metav1.Duration{Duration: 40 * time.Second},
			NodeStartupGracePeriod: metav1.Duration{Duration: 60 * time.Second},
			PodEvictionTimeout:     metav1.Duration{Duration: 5 * time.Minute},
			EnableTaintManager:     true,
		},
		PersistentVolumeBinderController: &cmoptions.PersistentVolumeBinderControllerOptions{
			PVClaimBinderSyncPeriod: metav1.Duration{Duration: 15 * time.Second},
			VolumeConfiguration: componentconfig.VolumeConfiguration{
				EnableDynamicProvisioning:  true,
				EnableHostPathProvisioning: false,
				FlexVolumePluginDir:        "/usr/libexec/kubernetes/kubelet-plugins/volume/exec/",
				PersistentVolumeRecyclerConfiguration: componentconfig.PersistentVolumeRecyclerConfiguration{
					MaximumRetry:             3,
					MinimumTimeoutNFS:        300,
					IncrementTimeoutNFS:      30,
					MinimumTimeoutHostPath:   60,
					IncrementTimeoutHostPath: 30,
				},
			},
		},
		PodGCController: &cmoptions.PodGCControllerOptions{
			TerminatedPodGCThreshold: 12500,
		},
		ReplicaSetController: &cmoptions.ReplicaSetControllerOptions{
			ConcurrentRSSyncs: 5,
		},
		ReplicationController: &cmoptions.ReplicationControllerOptions{
			ConcurrentRCSyncs: 5,
		},
		ResourceQuotaController: &cmoptions.ResourceQuotaControllerOptions{
			ResourceQuotaSyncPeriod:      metav1.Duration{Duration: 5 * time.Minute},
			ConcurrentResourceQuotaSyncs: 5,
		},
		SAController: &cmoptions.SAControllerOptions{
			ConcurrentSATokenSyncs: 5,
		},
		ServiceController: &cmoptions.ServiceControllerOptions{
			ConcurrentServiceSyncs: 1,
		},
	}

	// Because we sorted the defaults, we need to sort the expectedDefaults too.
	sort.Sort(sortedGCIgnoredResources(expectedDefaults.GarbageCollectorController.GCIgnoredResources))

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesMasterConfig and update expectedDefaults")
	}
}

func TestSchedulerServerDefaults(t *testing.T) {
	defaults, err := scheduleroptions.NewOptions()
	if err != nil {
		t.Fatal(err)
	}

	provider := "DefaultProvider"

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &componentconfig.KubeSchedulerConfiguration{
		SchedulerName: "default-scheduler",
		AlgorithmSource: componentconfig.SchedulerAlgorithmSource{
			Provider: &provider,
		},
		HardPodAffinitySymmetricWeight: 1,
		LeaderElection: componentconfig.KubeSchedulerLeaderElectionConfiguration{
			LeaderElectionConfiguration: componentconfig.LeaderElectionConfiguration{
				ResourceLock: "endpoints",
				LeaderElect:  true,
				LeaseDuration: metav1.Duration{
					Duration: 15 * time.Second,
				},
				RenewDeadline: metav1.Duration{
					Duration: 10 * time.Second,
				},
				RetryPeriod: metav1.Duration{
					Duration: 2 * time.Second,
				},
			},
			LockObjectNamespace: "kube-system",
			LockObjectName:      "kube-scheduler",
		},
		ClientConnection: componentconfig.ClientConnectionConfiguration{
			ContentType: "application/vnd.kubernetes.protobuf",
			QPS:         50,
			Burst:       100,
		},
		HealthzBindAddress: "0.0.0.0:10251", // we disable this
		MetricsBindAddress: "0.0.0.0:10251",
		FailureDomains:     "kubernetes.io/hostname,failure-domain.beta.kubernetes.io/zone,failure-domain.beta.kubernetes.io/region",
	}

	if !reflect.DeepEqual(defaults.ComponentConfig, *expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(*expectedDefaults, defaults.ComponentConfig))
		t.Errorf("Got different defaults than expected, adjust in computeSchedulerArgs and update expectedDefaults")
	}
}

func TestGetAPIGroupVersionOverrides(t *testing.T) {
	testcases := map[string]struct {
		DisabledVersions         map[string][]string
		ExpectedDisabledVersions []schema.GroupVersion
		ExpectedEnabledVersions  []schema.GroupVersion
	}{
		"empty": {
			DisabledVersions:         nil,
			ExpectedDisabledVersions: []schema.GroupVersion{},
			ExpectedEnabledVersions:  []schema.GroupVersion{apiv1.SchemeGroupVersion, extensionsapiv1beta1.SchemeGroupVersion},
		},
		"* -> v1": {
			DisabledVersions:         map[string][]string{"": {"*"}},
			ExpectedDisabledVersions: []schema.GroupVersion{apiv1.SchemeGroupVersion},
			ExpectedEnabledVersions:  []schema.GroupVersion{extensionsapiv1beta1.SchemeGroupVersion},
		},
		"v1": {
			DisabledVersions:         map[string][]string{"": {"v1"}},
			ExpectedDisabledVersions: []schema.GroupVersion{apiv1.SchemeGroupVersion},
			ExpectedEnabledVersions:  []schema.GroupVersion{extensionsapiv1beta1.SchemeGroupVersion},
		},
		"* -> v1beta1": {
			DisabledVersions:         map[string][]string{"extensions": {"*"}},
			ExpectedDisabledVersions: []schema.GroupVersion{extensionsapiv1beta1.SchemeGroupVersion},
			ExpectedEnabledVersions:  []schema.GroupVersion{apiv1.SchemeGroupVersion},
		},
		"extensions/v1beta1": {
			DisabledVersions:         map[string][]string{"extensions": {"v1beta1"}},
			ExpectedDisabledVersions: []schema.GroupVersion{extensionsapiv1beta1.SchemeGroupVersion},
			ExpectedEnabledVersions:  []schema.GroupVersion{apiv1.SchemeGroupVersion},
		},
	}

	for k, tc := range testcases {
		config := configapi.MasterConfig{KubernetesMasterConfig: configapi.KubernetesMasterConfig{DisabledAPIGroupVersions: tc.DisabledVersions}}
		overrides := getAPIResourceConfig(config)

		for _, expected := range tc.ExpectedDisabledVersions {
			if overrides.VersionEnabled(expected) {
				t.Errorf("%s: Expected %v", k, expected)
			}
		}

		for _, expected := range tc.ExpectedEnabledVersions {
			if !overrides.VersionEnabled(expected) {
				t.Errorf("%s: Expected %v", k, expected)
			}
		}
	}
}
