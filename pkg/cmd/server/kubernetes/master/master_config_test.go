package master

import (
	"net"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/cfssl/helpers"

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

var expectedGroupPreferredVersions []string = []string{
	// keep this sorted:
	"admission.k8s.io/v1alpha1",
	"admissionregistration.k8s.io/v1alpha1",
	"apps/v1beta1,authentication.k8s.io/v1",
	"authorization.k8s.io/v1",
	"autoscaling/v1",
	"batch/v1",
	"certificates.k8s.io/v1beta1",
	"componentconfig/v1alpha1",
	"extensions/v1beta1",
	"federation/v1beta1",
	"imagepolicy.k8s.io/v1alpha1",
	"networking.k8s.io/v1",
	"policy/v1beta1",
	"rbac.authorization.k8s.io/v1beta1",
	"settings.k8s.io/v1alpha1",
	"storage.k8s.io/v1",
	"v1",
}

func TestPreferredGroupVersions(t *testing.T) {
	s := kapi.Registry.AllPreferredGroupVersions()
	expected := strings.Join(expectedGroupPreferredVersions, ",")
	if s != expected {
		t.Logf("expected: %#v", expected)
		t.Logf("got: %#v", s)
		t.Errorf("unexpected preferred group versions: %v", diff.StringDiff(expected, s))
	}
}

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
		},
		Admission: &apiserveroptions.AdmissionOptions{
			PluginNames: []string{"AlwaysAdmit"},
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
			DefaultWatchCacheSize:   100,
		},
		SecureServing: &apiserveroptions.SecureServingOptions{
			BindAddress: net.ParseIP("0.0.0.0"),
			BindPort:    6443,
			ServerCert: apiserveroptions.GeneratableKeyCert{
				CertDirectory: "/var/run/kubernetes",
				PairName:      "apiserver",
			},
		},
		InsecureServing: &kubeoptions.InsecureServingOptions{
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
			},
			EnableHttps: true,
			HTTPTimeout: time.Duration(5) * time.Second,
		},
		Audit: &apiserveroptions.AuditOptions{
			WebhookOptions: apiserveroptions.AuditWebhookOptions{
				Mode: "batch",
			},
		},
		Features: &apiserveroptions.FeatureOptions{
			EnableProfiling: true,
		},
		Authentication: &kubeoptions.BuiltInAuthenticationOptions{
			Anonymous:      &kubeoptions.AnonymousAuthenticationOptions{Allow: true},
			AnyToken:       &kubeoptions.AnyTokenAuthenticationOptions{},
			BootstrapToken: &kubeoptions.BootstrapTokenAuthenticationOptions{},
			ClientCert:     &apiserveroptions.ClientCertAuthenticationOptions{},
			Keystone:       &kubeoptions.KeystoneAuthenticationOptions{},
			OIDC:           &kubeoptions.OIDCAuthenticationOptions{},
			PasswordFile:   &kubeoptions.PasswordFileAuthenticationOptions{},
			RequestHeader:  &apiserveroptions.RequestHeaderAuthenticationOptions{},
			ServiceAccounts: &kubeoptions.ServiceAccountAuthenticationOptions{
				Lookup: true,
			},
			TokenFile: &kubeoptions.TokenFileAuthenticationOptions{},
			WebHook:   &kubeoptions.WebHookAuthenticationOptions{CacheTTL: 2 * time.Minute},
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
		EnableLogsHandler: true, // we disable this
	}

	// clear the non-serializeable bit
	defaults.Admission.Plugins = nil

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
	defaults := cmapp.NewCMServer()
	// We need to sort GCIgnoredResources because it's built from a map, which means the insertion
	// order is random.
	sort.Sort(sortedGCIgnoredResources(defaults.GCIgnoredResources))

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &cmapp.CMServer{
		KubeControllerManagerConfiguration: componentconfig.KubeControllerManagerConfiguration{
			Port:                                            10252, // disabled
			Address:                                         "0.0.0.0",
			ConcurrentEndpointSyncs:                         5,
			ConcurrentRCSyncs:                               5,
			ConcurrentRSSyncs:                               5,
			ConcurrentDaemonSetSyncs:                        2,
			ConcurrentJobSyncs:                              5,
			ConcurrentResourceQuotaSyncs:                    5,
			ConcurrentDeploymentSyncs:                       5,
			ConcurrentNamespaceSyncs:                        10,
			ConcurrentSATokenSyncs:                          5,
			ConcurrentServiceSyncs:                          1,
			ConcurrentGCSyncs:                               20,
			LookupCacheSizeForRC:                            4096,
			LookupCacheSizeForRS:                            4096,
			LookupCacheSizeForDaemonSet:                     1024,
			ConfigureCloudRoutes:                            true,
			NodeCIDRMaskSize:                                24,
			ServiceSyncPeriod:                               metav1.Duration{Duration: 5 * time.Minute},
			ResourceQuotaSyncPeriod:                         metav1.Duration{Duration: 5 * time.Minute},
			NamespaceSyncPeriod:                             metav1.Duration{Duration: 5 * time.Minute},
			PVClaimBinderSyncPeriod:                         metav1.Duration{Duration: 15 * time.Second},
			HorizontalPodAutoscalerSyncPeriod:               metav1.Duration{Duration: 30 * time.Second},
			DeploymentControllerSyncPeriod:                  metav1.Duration{Duration: 30 * time.Second},
			MinResyncPeriod:                                 metav1.Duration{Duration: 12 * time.Hour},
			RegisterRetryCount:                              10,
			RouteReconciliationPeriod:                       metav1.Duration{Duration: 10 * time.Second},
			PodEvictionTimeout:                              metav1.Duration{Duration: 5 * time.Minute},
			NodeMonitorGracePeriod:                          metav1.Duration{Duration: 40 * time.Second},
			NodeStartupGracePeriod:                          metav1.Duration{Duration: 60 * time.Second},
			NodeMonitorPeriod:                               metav1.Duration{Duration: 5 * time.Second},
			HorizontalPodAutoscalerUpscaleForbiddenWindow:   metav1.Duration{Duration: 3 * time.Minute},
			HorizontalPodAutoscalerDownscaleForbiddenWindow: metav1.Duration{Duration: 5 * time.Minute},
			ClusterName:              "kubernetes",
			TerminatedPodGCThreshold: 12500,
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
				ResourceLock:  "endpoints",
				LeaderElect:   true,
				LeaseDuration: metav1.Duration{Duration: 15 * time.Second},
				RenewDeadline: metav1.Duration{Duration: 10 * time.Second},
				RetryPeriod:   metav1.Duration{Duration: 2 * time.Second},
			},
			ClusterSigningCertFile: "/etc/kubernetes/ca/ca.pem",
			ClusterSigningKeyFile:  "/etc/kubernetes/ca/ca.key",
			ClusterSigningDuration: metav1.Duration{Duration: helpers.OneYear},
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
				{Group: "apiregistration.k8s.io", Resource: "apiservices"},
				{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"},
			},
			DisableAttachDetachReconcilerSync: false,
			ReconcilerSyncLoopPeriod:          metav1.Duration{Duration: 60 * time.Second},
			Controllers:                       []string{"*"},
			EnableTaintManager:                true,
		},
	}

	// Because we sorted the defaults, we need to sort the expectedDefaults too.
	sort.Sort(sortedGCIgnoredResources(expectedDefaults.GCIgnoredResources))

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
			LockObjectNamespace:      "kube-system",
			LockObjectName:           "kube-scheduler",
			PolicyConfigMapNamespace: "kube-system",
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
