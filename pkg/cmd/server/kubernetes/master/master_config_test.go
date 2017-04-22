package master

import (
	"net"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	utilconfig "k8s.io/apiserver/pkg/util/flag"
	kubeapiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kapi "k8s.io/kubernetes/pkg/api"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	extensionsapiv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kubeoptions "k8s.io/kubernetes/pkg/kubeapiserver/options"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	scheduleroptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func TestAPIServerDefaults(t *testing.T) {
	defaults := kubeapiserveroptions.NewServerRunOptions()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &kubeapiserveroptions.ServerRunOptions{
		ServiceNodePortRange: kubeapiserveroptions.DefaultServiceNodePortRange,
		MasterCount:          1,
		GenericServerRunOptions: &apiserveroptions.ServerRunOptions{
			MaxRequestsInFlight:         400,
			MaxMutatingRequestsInFlight: 200,
			MinRequestTimeout:           1800,
			AdmissionControl:            "AlwaysAdmit",
		},
		Etcd: &apiserveroptions.EtcdOptions{
			StorageConfig: storagebackend.Config{
				ServerList: nil,
				Prefix:     "/registry",
				DeserializationCacheSize: 0,
				Copier: kapi.Scheme,
			},
			DefaultStorageMediaType: "application/vnd.kubernetes.protobuf",
			DeleteCollectionWorkers: 1,
			EnableGarbageCollection: true,
			EnableWatchCache:        true,
		},
		SecureServing: &apiserveroptions.SecureServingOptions{
			ServingOptions: apiserveroptions.ServingOptions{
				BindAddress: net.ParseIP("0.0.0.0"),
				BindPort:    6443,
			},
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: "/var/run/kubernetes",
				PairName:      "apiserver",
			},
		},
		InsecureServing: &apiserveroptions.ServingOptions{
			BindAddress: net.ParseIP("127.0.0.1"),
			BindPort:    8080,
		},
		EventTTL: 1 * time.Hour,
		KubeletConfig: kubeletclient.KubeletClientConfig{
			Port:         10250,
			ReadOnlyPort: 10255,
			PreferredAddressTypes: []string{
				string(apiv1.NodeHostName),
				string(apiv1.NodeInternalDNS),
				string(apiv1.NodeInternalIP),
				string(apiv1.NodeExternalDNS),
				string(apiv1.NodeExternalIP),
				string(apiv1.NodeLegacyHostIP),
			},
			EnableHttps: true,
			HTTPTimeout: time.Duration(5) * time.Second,
		},
		Audit: &apiserveroptions.AuditLogOptions{},
		Features: &apiserveroptions.FeatureOptions{
			EnableProfiling: true,
		},
		Authentication: &kubeoptions.BuiltInAuthenticationOptions{
			Anonymous:       &kubeoptions.AnonymousAuthenticationOptions{Allow: true},
			AnyToken:        &kubeoptions.AnyTokenAuthenticationOptions{},
			BootstrapToken:  &kubeoptions.BootstrapTokenAuthenticationOptions{},
			ClientCert:      &apiserveroptions.ClientCertAuthenticationOptions{},
			Keystone:        &kubeoptions.KeystoneAuthenticationOptions{},
			OIDC:            &kubeoptions.OIDCAuthenticationOptions{},
			PasswordFile:    &kubeoptions.PasswordFileAuthenticationOptions{},
			RequestHeader:   &apiserveroptions.RequestHeaderAuthenticationOptions{},
			ServiceAccounts: &kubeoptions.ServiceAccountAuthenticationOptions{},
			TokenFile:       &kubeoptions.TokenFileAuthenticationOptions{},
			WebHook:         &kubeoptions.WebHookAuthenticationOptions{CacheTTL: 2 * time.Minute},
		},
		Authorization: &kubeoptions.BuiltInAuthorizationOptions{
			Mode: "AlwaysAllow",
			WebhookCacheAuthorizedTTL:   5 * time.Minute,
			WebhookCacheUnauthorizedTTL: 30 * time.Second,
		},
		CloudProvider: &kubeoptions.CloudProviderOptions{},
		StorageSerialization: &kubeoptions.StorageSerializationOptions{
			StorageVersions:        kapi.Registry.AllPreferredGroupVersions(),
			DefaultStorageVersions: kapi.Registry.AllPreferredGroupVersions(),
		},
		APIEnablement: &kubeoptions.APIEnablementOptions{
			RuntimeConfig: utilconfig.ConfigurationMap{},
		},
	}

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesMasterConfig and update expectedDefaults")
	}
}

func TestCMServerDefaults(t *testing.T) {
	defaults := cmapp.NewCMServer()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &cmapp.CMServer{
		KubeControllerManagerConfiguration: componentconfig.KubeControllerManagerConfiguration{
			Port:                              10252, // disabled
			Address:                           "0.0.0.0",
			ConcurrentEndpointSyncs:           5,
			ConcurrentRCSyncs:                 5,
			ConcurrentRSSyncs:                 5,
			ConcurrentDaemonSetSyncs:          2,
			ConcurrentJobSyncs:                5,
			ConcurrentResourceQuotaSyncs:      5,
			ConcurrentDeploymentSyncs:         5,
			ConcurrentNamespaceSyncs:          2,
			ConcurrentSATokenSyncs:            5,
			ConcurrentServiceSyncs:            1,
			ConcurrentGCSyncs:                 20,
			LookupCacheSizeForRC:              4096,
			LookupCacheSizeForRS:              4096,
			LookupCacheSizeForDaemonSet:       1024,
			ConfigureCloudRoutes:              true,
			NodeCIDRMaskSize:                  24,
			ServiceSyncPeriod:                 metav1.Duration{Duration: 5 * time.Minute},
			ResourceQuotaSyncPeriod:           metav1.Duration{Duration: 5 * time.Minute},
			NamespaceSyncPeriod:               metav1.Duration{Duration: 5 * time.Minute},
			PVClaimBinderSyncPeriod:           metav1.Duration{Duration: 15 * time.Second},
			HorizontalPodAutoscalerSyncPeriod: metav1.Duration{Duration: 30 * time.Second},
			DeploymentControllerSyncPeriod:    metav1.Duration{Duration: 30 * time.Second},
			MinResyncPeriod:                   metav1.Duration{Duration: 12 * time.Hour},
			RegisterRetryCount:                10,
			RouteReconciliationPeriod:         metav1.Duration{Duration: 10 * time.Second},
			PodEvictionTimeout:                metav1.Duration{Duration: 5 * time.Minute},
			NodeMonitorGracePeriod:            metav1.Duration{Duration: 40 * time.Second},
			NodeStartupGracePeriod:            metav1.Duration{Duration: 60 * time.Second},
			NodeMonitorPeriod:                 metav1.Duration{Duration: 5 * time.Second},
			ClusterName:                       "kubernetes",
			TerminatedPodGCThreshold:          12500,
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
			ContentType:  "application/vnd.kubernetes.protobuf",
			KubeAPIQPS:   20.0,
			KubeAPIBurst: 30,
			LeaderElection: componentconfig.LeaderElectionConfiguration{
				LeaderElect:   true,
				LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
				RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
				RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
			},
			ClusterSigningCertFile:            "/etc/kubernetes/ca/ca.pem",
			ClusterSigningKeyFile:             "/etc/kubernetes/ca/ca.key",
			EnableGarbageCollector:            true,
			DisableAttachDetachReconcilerSync: false,
			ReconcilerSyncLoopPeriod:          metav1.Duration{Duration: 60 * time.Second},
			Controllers:                       []string{"*"},
			EnableTaintManager:                true,
		},
	}

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesMasterConfig and update expectedDefaults")
	}
}

func TestSchedulerServerDefaults(t *testing.T) {
	defaults := scheduleroptions.NewSchedulerServer()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &scheduleroptions.SchedulerServer{
		KubeSchedulerConfiguration: componentconfig.KubeSchedulerConfiguration{
			Port:                           10251, // disabled
			Address:                        "0.0.0.0",
			AlgorithmProvider:              "DefaultProvider",
			ContentType:                    "application/vnd.kubernetes.protobuf",
			KubeAPIQPS:                     50,
			KubeAPIBurst:                   100,
			SchedulerName:                  "default-scheduler",
			HardPodAffinitySymmetricWeight: 1,
			FailureDomains:                 "kubernetes.io/hostname,failure-domain.beta.kubernetes.io/zone,failure-domain.beta.kubernetes.io/region",
			LeaderElection: componentconfig.LeaderElectionConfiguration{
				LeaderElect: true,
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
		},
	}

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", diff.ObjectReflectDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesMasterConfig and update expectedDefaults")
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
		config := configapi.MasterConfig{KubernetesMasterConfig: &configapi.KubernetesMasterConfig{DisabledAPIGroupVersions: tc.DisabledVersions}}
		overrides := getAPIResourceConfig(config)

		for _, expected := range tc.ExpectedDisabledVersions {
			if overrides.AnyResourcesForVersionEnabled(expected) {
				t.Errorf("%s: Expected %v", k, expected)
			}
		}

		for _, expected := range tc.ExpectedEnabledVersions {
			if !overrides.AllResourcesForVersionEnabled(expected) {
				t.Errorf("%s: Expected %v", k, expected)
			}
		}
	}
}
