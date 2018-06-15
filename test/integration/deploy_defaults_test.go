package integration

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapiv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appsclientscheme "github.com/openshift/origin/pkg/apps/generated/internalclientset/scheme"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	"github.com/openshift/origin/pkg/api/legacy"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
)

var (
	nonDefaultRevisionHistoryLimit = appsapi.DefaultRevisionHistoryLimit + 42
)

func minimalDC(name string, generation int64) *appsapi.DeploymentConfig {
	return &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: generation,
		},
		Spec: appsapi.DeploymentConfigSpec{
			Selector: map[string]string{
				"app": name,
			},
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name:  "a",
							Image: " ",
						},
					},
				},
			},
		},
	}
}

func int64ptr(v int64) *int64 {
	return &v
}

func int32ptr(v int32) *int32 {
	return &v
}

func setEssentialDefaults(dc *appsapi.DeploymentConfig) *appsapi.DeploymentConfig {
	dc.Spec.Strategy.Type = appsapi.DeploymentStrategyTypeRolling
	dc.Spec.Strategy.RollingParams = &appsapi.RollingDeploymentStrategyParams{
		IntervalSeconds:     int64ptr(1),
		UpdatePeriodSeconds: int64ptr(1),
		TimeoutSeconds:      int64ptr(600),
		MaxUnavailable:      intstr.FromString("25%"),
		MaxSurge:            intstr.FromString("25%"),
	}
	dc.Spec.Strategy.ActiveDeadlineSeconds = int64ptr(21600)
	dc.Spec.Triggers = []appsapi.DeploymentTriggerPolicy{
		{Type: appsapi.DeploymentTriggerOnConfigChange},
	}
	dc.Spec.Template.Spec.Containers[0].TerminationMessagePath = "/dev/termination-log"
	dc.Spec.Template.Spec.Containers[0].TerminationMessagePolicy = "File"
	dc.Spec.Template.Spec.Containers[0].ImagePullPolicy = "IfNotPresent"
	dc.Spec.Template.Spec.RestartPolicy = "Always"
	dc.Spec.Template.Spec.TerminationGracePeriodSeconds = int64ptr(30)
	dc.Spec.Template.Spec.DNSPolicy = "ClusterFirst"
	dc.Spec.Template.Spec.SecurityContext = &kapi.PodSecurityContext{
		HostNetwork: false,
		HostPID:     false,
		HostIPC:     false,
	}
	dc.Spec.Template.Spec.SchedulerName = "default-scheduler"

	return dc
}

func clearTransient(dc *appsapi.DeploymentConfig) {
	dc.ObjectMeta.Namespace = ""
	dc.ObjectMeta.SelfLink = ""
	dc.ObjectMeta.UID = ""
	dc.ObjectMeta.ResourceVersion = ""
	dc.ObjectMeta.CreationTimestamp.Time = time.Time{}
}

func TestDeploymentConfigDefaults(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("Failed to start master: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	namespace := "default"

	adminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("Failed to get cluster admin client config: %v", err)
	}

	appsClient, err := appsclient.NewForConfig(adminClientConfig)
	if err != nil {
		t.Fatalf("Failed to create appsClient: %v", err)
	}
	// install the legacy types into the client for decoding
	legacy.InstallLegacy(appsapi.GroupName, appsapi.AddToSchemeInCoreGroup, appsapiv1.AddToSchemeInCoreGroup,
		sets.NewString(),
		appsclientscheme.Registry, appsclientscheme.Scheme,
	)

	ttLegacy := []struct {
		obj    *appsapi.DeploymentConfig
		legacy *appsapi.DeploymentConfig
	}{
		{
			obj: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-legacy-01", 0)
				dc.Spec.RevisionHistoryLimit = nil
				return dc
			}(),
			legacy: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-legacy-01", 1)
				setEssentialDefaults(dc)
				// Legacy API shall not default RevisionHistoryLimit to maintain backwards compatibility
				dc.Spec.RevisionHistoryLimit = nil
				return dc
			}(),
		},
		{
			obj: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-legacy-02", 0)
				dc.Spec.RevisionHistoryLimit = &nonDefaultRevisionHistoryLimit
				return dc
			}(),
			legacy: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-legacy-02", 1)
				setEssentialDefaults(dc)
				dc.Spec.RevisionHistoryLimit = &nonDefaultRevisionHistoryLimit
				return dc
			}(),
		},
	}
	t.Run("Legacy API", func(t *testing.T) {
		for _, tc := range ttLegacy {
			t.Run("", func(t *testing.T) {
				dcBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Version: "v1"}), tc.obj)
				if err != nil {
					t.Fatal(err)
				}
				legacyObj, err := appsClient.Apps().RESTClient().Post().AbsPath("/oapi/v1/namespaces/" + namespace + "/deploymentconfigs").Body(dcBytes).Do().Get()
				if err != nil {
					t.Fatalf("Failed to create DC: %v", err)
				}
				legacyDC := legacyObj.(*appsapi.DeploymentConfig)

				clearTransient(legacyDC)
				if !reflect.DeepEqual(legacyDC, tc.legacy) {
					t.Errorf("Legacy DC differs from expected output: %s", diff.ObjectReflectDiff(legacyDC, tc.legacy))
				}
			})
		}
	})

	ttApps := []struct {
		obj  *appsapi.DeploymentConfig
		apps *appsapi.DeploymentConfig
	}{
		{
			obj: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-apps-01", 0)
				dc.Spec.RevisionHistoryLimit = nil
				return dc
			}(),
			apps: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-apps-01", 1)
				setEssentialDefaults(dc)
				// Group API should default RevisionHistoryLimit
				dc.Spec.RevisionHistoryLimit = int32ptr(10)
				return dc
			}(),
		},
		{
			obj: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-apps-02", 0)
				dc.Spec.RevisionHistoryLimit = &nonDefaultRevisionHistoryLimit
				return dc
			}(),
			apps: func() *appsapi.DeploymentConfig {
				dc := minimalDC("test-apps-02", 1)
				setEssentialDefaults(dc)
				dc.Spec.RevisionHistoryLimit = &nonDefaultRevisionHistoryLimit
				return dc
			}(),
		},
	}
	t.Run("apps.openshift.io", func(t *testing.T) {
		for _, tc := range ttApps {
			t.Run("", func(t *testing.T) {
				appsDC, err := appsClient.Apps().DeploymentConfigs(namespace).Create(tc.obj)
				if err != nil {
					t.Fatalf("Failed to create DC: %v", err)
				}

				clearTransient(appsDC)
				if !reflect.DeepEqual(appsDC, tc.apps) {
					t.Errorf("Apps DC differs from expected output: %s", diff.ObjectReflectDiff(appsDC, tc.apps))
				}
			})
		}
	})
}
