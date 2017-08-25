package integration

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/intstr"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	"github.com/openshift/origin/pkg/deploy/generated/clientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var (
	nonDefaultRevisionHistoryLimit = deployapi.DefaultRevisionHistoryLimit + 42
)

func minimalDC(name string, generation int64) *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Generation: generation,
		},
		Spec: deployapi.DeploymentConfigSpec{
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

func setEssentialDefaults(dc *deployapi.DeploymentConfig) *deployapi.DeploymentConfig {
	dc.Spec.Strategy.Type = deployapi.DeploymentStrategyTypeRolling
	dc.Spec.Strategy.RollingParams = &deployapi.RollingDeploymentStrategyParams{
		IntervalSeconds:     int64ptr(1),
		UpdatePeriodSeconds: int64ptr(1),
		TimeoutSeconds:      int64ptr(600),
		MaxUnavailable:      intstr.FromString("25%"),
		MaxSurge:            intstr.FromString("25%"),
	}
	dc.Spec.Strategy.ActiveDeadlineSeconds = int64ptr(21600)
	dc.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{}
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

func clearTransient(dc *deployapi.DeploymentConfig) {
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

	legacyClient, err := client.New(adminClientConfig)
	if err != nil {
		t.Fatalf("Failed to create legacyClient: %v", err)
	}

	appsClient, err := clientset.NewForConfig(adminClientConfig)
	if err != nil {
		t.Fatalf("Failed to create appsClient: %v", err)
	}

	ttLegacy := []struct {
		obj    *deployapi.DeploymentConfig
		legacy *deployapi.DeploymentConfig
	}{
		{
			obj: func() *deployapi.DeploymentConfig {
				dc := minimalDC("test-legacy-01", 0)
				dc.Spec.RevisionHistoryLimit = nil
				return dc
			}(),
			legacy: func() *deployapi.DeploymentConfig {
				dc := minimalDC("test-legacy-01", 1)
				setEssentialDefaults(dc)
				// Legacy API shall not default RevisionHistoryLimit to maintain backwards compatibility
				dc.Spec.RevisionHistoryLimit = nil
				return dc
			}(),
		},
		{
			obj: func() *deployapi.DeploymentConfig {
				dc := minimalDC("test-legacy-02", 0)
				dc.Spec.RevisionHistoryLimit = &nonDefaultRevisionHistoryLimit
				return dc
			}(),
			legacy: func() *deployapi.DeploymentConfig {
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
				legacyDC, err := legacyClient.DeploymentConfigs(namespace).Create(tc.obj)
				if err != nil {
					t.Fatalf("Failed to create DC: %v", err)
				}

				clearTransient(legacyDC)
				if !reflect.DeepEqual(legacyDC, tc.legacy) {
					t.Errorf("Legacy DC differs from expected output: %s", diff.ObjectReflectDiff(legacyDC, tc.legacy))
				}
			})
		}
	})

	ttApps := []struct {
		obj  *deployapi.DeploymentConfig
		apps *deployapi.DeploymentConfig
	}{
		{
			obj: func() *deployapi.DeploymentConfig {
				dc := minimalDC("test-apps-01", 0)
				dc.Spec.RevisionHistoryLimit = nil
				return dc
			}(),
			apps: func() *deployapi.DeploymentConfig {
				dc := minimalDC("test-apps-01", 1)
				setEssentialDefaults(dc)
				// Group API should default RevisionHistoryLimit
				dc.Spec.RevisionHistoryLimit = int32ptr(10)
				return dc
			}(),
		},
		{
			obj: func() *deployapi.DeploymentConfig {
				dc := minimalDC("test-apps-02", 0)
				dc.Spec.RevisionHistoryLimit = &nonDefaultRevisionHistoryLimit
				return dc
			}(),
			apps: func() *deployapi.DeploymentConfig {
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
				var objV1 deployapiv1.DeploymentConfig
				err := kapi.Scheme.Convert(tc.obj, &objV1, nil)
				if err != nil {
					t.Fatalf("Failed to convert internal DC to v1: %v", err)
				}
				appsDCV1, err := appsClient.AppsV1().DeploymentConfigs(namespace).Create(&objV1)
				if err != nil {
					t.Fatalf("Failed to create DC: %v", err)
				}

				var appsDC deployapi.DeploymentConfig
				err = kapi.Scheme.Convert(appsDCV1, &appsDC, nil)
				if err != nil {
					t.Fatalf("Failed to convert v1 to internal DC: %v", err)
				}
				clearTransient(&appsDC)
				if !reflect.DeepEqual(&appsDC, tc.apps) {
					t.Errorf("Apps DC differs from expected output: %s", diff.ObjectReflectDiff(&appsDC, tc.apps))
				}
			})
		}
	})
}
