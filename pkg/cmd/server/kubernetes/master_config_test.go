package kubernetes

import (
	"net"
	"reflect"
	"testing"
	"time"

	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	"k8s.io/kubernetes/pkg/genericapiserver"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

func TestAPIServerDefaults(t *testing.T) {
	defaults := apiserveroptions.NewAPIServer()

	// This is a snapshot of the default config
	// If the default changes (new fields are added, or default values change), we want to know
	// Once we've reacted to the changes appropriately in BuildKubernetesMasterConfig(), update this expected default to match the new upstream defaults
	expectedDefaults := &apiserveroptions.APIServer{
		ServerRunOptions: &genericapiserver.ServerRunOptions{
			BindAddress:          net.ParseIP("0.0.0.0"),
			CertDirectory:        "/var/run/kubernetes",
			InsecureBindAddress:  net.ParseIP("127.0.0.1"),
			InsecurePort:         8080,
			LongRunningRequestRE: "(/|^)((watch|proxy)(/|$)|(logs?|portforward|exec|attach)/?$)",
			MaxRequestsInFlight:  400,
			SecurePort:           6443,
		},
		APIGroupPrefix:          "/apis",
		APIPrefix:               "/api",
		AdmissionControl:        "AlwaysAdmit",
		AuthorizationMode:       "AlwaysAllow",
		DeleteCollectionWorkers: 1,
		EnableLogsSupport:       true,
		EnableProfiling:         true,
		EnableWatchCache:        true,
		EtcdConfig: etcdstorage.EtcdConfig{
			Prefix: "/registry",
		},
		EventTTL:               1 * time.Hour,
		MasterCount:            1,
		MasterServiceNamespace: "default",
		MinRequestTimeout:      1800,
		RuntimeConfig:          util.ConfigurationMap{},
		StorageVersions:        registered.AllPreferredGroupVersions(),
		DefaultStorageVersions: registered.AllPreferredGroupVersions(),
		KubeletConfig: kubeletclient.KubeletClientConfig{
			Port:        10250,
			EnableHttps: true,
			HTTPTimeout: time.Duration(5) * time.Second,
		},
	}

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", util.ObjectGoPrintDiff(expectedDefaults, defaults))
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
			LookupCacheSizeForRC:              4096,
			LookupCacheSizeForRS:              4096,
			LookupCacheSizeForDaemonSet:       1024,
			ServiceSyncPeriod:                 unversioned.Duration{Duration: 5 * time.Minute},
			NodeSyncPeriod:                    unversioned.Duration{Duration: 10 * time.Second},
			ResourceQuotaSyncPeriod:           unversioned.Duration{Duration: 5 * time.Minute},
			NamespaceSyncPeriod:               unversioned.Duration{Duration: 5 * time.Minute},
			PVClaimBinderSyncPeriod:           unversioned.Duration{Duration: 10 * time.Minute},
			HorizontalPodAutoscalerSyncPeriod: unversioned.Duration{Duration: 30 * time.Second},
			DeploymentControllerSyncPeriod:    unversioned.Duration{Duration: 30 * time.Second},
			MinResyncPeriod:                   unversioned.Duration{Duration: 12 * time.Hour},
			RegisterRetryCount:                10,
			PodEvictionTimeout:                unversioned.Duration{Duration: 5 * time.Minute},
			NodeMonitorGracePeriod:            unversioned.Duration{Duration: 40 * time.Second},
			NodeStartupGracePeriod:            unversioned.Duration{Duration: 60 * time.Second},
			NodeMonitorPeriod:                 unversioned.Duration{Duration: 5 * time.Second},
			ClusterName:                       "kubernetes",
			TerminatedPodGCThreshold:          12500,
			VolumeConfiguration: componentconfig.VolumeConfiguration{
				EnableHostPathProvisioning: false,
				PersistentVolumeRecyclerConfiguration: componentconfig.PersistentVolumeRecyclerConfiguration{
					MaximumRetry:             3,
					MinimumTimeoutNFS:        300,
					IncrementTimeoutNFS:      30,
					MinimumTimeoutHostPath:   60,
					IncrementTimeoutHostPath: 30,
				},
			},
			KubeAPIQPS:   20.0,
			KubeAPIBurst: 30,
			LeaderElection: componentconfig.LeaderElectionConfiguration{
				LeaderElect:   false,
				LeaseDuration: unversioned.Duration{Duration: 15 * time.Second},
				RenewDeadline: unversioned.Duration{Duration: 10 * time.Second},
				RetryPeriod:   unversioned.Duration{Duration: 2 * time.Second},
			},
		},
	}

	if !reflect.DeepEqual(defaults, expectedDefaults) {
		t.Logf("expected defaults, actual defaults: \n%s", util.ObjectGoPrintDiff(expectedDefaults, defaults))
		t.Errorf("Got different defaults than expected, adjust in BuildKubernetesMasterConfig and update expectedDefaults")
	}
}

func TestGetAPIGroupVersionOverrides(t *testing.T) {
	testcases := map[string]struct {
		DisabledVersions  map[string][]string
		ExpectedOverrides map[string]genericapiserver.APIGroupVersionOverride
	}{
		"empty": {
			DisabledVersions:  nil,
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{},
		},
		"* -> v1": {
			DisabledVersions:  map[string][]string{"": {"*"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"api/v1": {Disable: true}},
		},
		"v1": {
			DisabledVersions:  map[string][]string{"": {"v1"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"api/v1": {Disable: true}},
		},
		"* -> v1beta1": {
			DisabledVersions:  map[string][]string{"extensions": {"*"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"extensions/v1beta1": {Disable: true}},
		},
		"extensions/v1beta1": {
			DisabledVersions:  map[string][]string{"extensions": {"v1beta1"}},
			ExpectedOverrides: map[string]genericapiserver.APIGroupVersionOverride{"extensions/v1beta1": {Disable: true}},
		},
	}

	for k, tc := range testcases {
		config := configapi.MasterConfig{KubernetesMasterConfig: &configapi.KubernetesMasterConfig{DisabledAPIGroupVersions: tc.DisabledVersions}}
		overrides := getAPIGroupVersionOverrides(config)
		if !reflect.DeepEqual(overrides, tc.ExpectedOverrides) {
			t.Errorf("%s: Expected\n%#v\ngot\n%#v", k, tc.ExpectedOverrides, overrides)
		}
	}
}
