package integration

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var expectedGroups = map[string]bool{
	"apiregistration.k8s.io/v1beta1":       false,
	"extensions/v1beta1":                   false,
	"apps/v1":                              false,
	"apps/v1beta1":                         false,
	"apps/v1beta2":                         false,
	"events.k8s.io/v1beta1":                false,
	"authentication.k8s.io/v1":             false,
	"authentication.k8s.io/v1beta1":        false,
	"authorization.k8s.io/v1":              false,
	"authorization.k8s.io/v1beta1":         false,
	"autoscaling/v1":                       false,
	"autoscaling/v2beta1":                  false,
	"batch/v1":                             false,
	"batch/v1beta1":                        false,
	"batch/v2alpha1":                       false,
	"certificates.k8s.io/v1beta1":          false,
	"networking.k8s.io/v1":                 false,
	"policy/v1beta1":                       false,
	"authorization.openshift.io/v1":        false,
	"rbac.authorization.k8s.io/v1":         false,
	"rbac.authorization.k8s.io/v1beta1":    false,
	"storage.k8s.io/v1":                    false,
	"storage.k8s.io/v1beta1":               false,
	"admissionregistration.k8s.io/v1beta1": false,
	"apiextensions.k8s.io/v1beta1":         false,
	"apps.openshift.io/v1":                 false,
	"build.openshift.io/v1":                false,
	"image.openshift.io/v1":                false,
	"network.openshift.io/v1":              false,
	"oauth.openshift.io/v1":                false,
	"project.openshift.io/v1":              false,
	"quota.openshift.io/v1":                false,
	"route.openshift.io/v1":                false,
	"security.openshift.io/v1":             false,
	"template.openshift.io/v1":             false,
	"user.openshift.io/v1":                 false,
	"/v1": false,
}

var expectedCategories = map[string][]string{
	"DaemonSet/extensions/v1beta1":                {"all"},
	"Deployment/extensions/v1beta1":               {"all"},
	"ReplicaSet/extensions/v1beta1":               {"all"},
	"DaemonSet/apps/v1":                           {"all"},
	"Deployment/apps/v1":                          {"all"},
	"ReplicaSet/apps/v1":                          {"all"},
	"StatefulSet/apps/v1":                         {"all"},
	"Deployment/apps/v1beta1":                     {"all"},
	"StatefulSet/apps/v1beta1":                    {"all"},
	"DaemonSet/apps/v1beta2":                      {"all"},
	"Deployment/apps/v1beta2":                     {"all"},
	"ReplicaSet/apps/v1beta2":                     {"all"},
	"StatefulSet/apps/v1beta2":                    {"all"},
	"HorizontalPodAutoscaler/autoscaling/v1":      {"all"},
	"HorizontalPodAutoscaler/autoscaling/v2beta1": {"all"},
	"Job/batch/v1":                                {"all"},
	"DeploymentConfig/apps.openshift.io/v1":       {"all"}, // FIX: discovered with no categories
	"BuildConfig/build.openshift.io/v1":           {"all"}, // FIX: discovered with no categories
	"Build/build.openshift.io/v1":                 {"all"}, // FIX: discovered with no categories
	"ImageStream/image.openshift.io/v1":           {"all"}, // FIX: discovered with no categories
	"Route/route.openshift.io/v1":                 {"all"}, // FIX: discovered with no categories
	"Pod/v1":                                      {"all"},
	"CronJob/batch/v1beta1":                       {"all"},
	"CronJob/batch/v2alpha1":                      {"all"},
	"ReplicationController/v1":                    {"all"},
	"Service/v1":                                  {"all"},
	"BuildConfig/v1":                              {"all"}, // FIX: discovered with no categories
	"Build/v1":                                    {"all"}, // FIX: discovered with no categories
	"DeploymentConfig/v1":                         {"all"}, // FIX: discovered with no categories
	"ImageStream/v1":                              {"all"}, // FIX: discovered with no categories
	"Route/v1":                                    {"all"}, // FIX: discovered with no categories
}

func TestDiscoveryCategoriesContainExpectedResources(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	masterConfig.OAuthConfig = nil
	clusterAdminKubeConfig, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer testserver.CleanupMasterEtcd(t, masterConfig)

	kclientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resources, err := kclientset.Discovery().ServerResources()
	if err != nil {
		t.Fatal(err)
	}

	actualCategories := map[string][]string{}

	for _, resourceList := range resources {
		gv, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			t.Fatal(err)
		}

		groupVersion := ""
		if len(gv.Group) > 0 {
			groupVersion += gv.Group
		}
		if len(gv.Version) > 0 {
			groupVersion += "/" + gv.Version
		}

		if _, exists := expectedGroups[groupVersion]; exists {
			expectedGroups[groupVersion] = true
		} else {
			t.Fatalf("unexpected discoverable resource group %q", groupVersion)
		}

		for _, resource := range resourceList.APIResources {
			if len(resource.Categories) == 0 {
				continue
			}

			kindGroupVersion := resource.Kind
			if len(gv.Group) > 0 {
				kindGroupVersion += "/" + gv.Group
			}
			if len(gv.Version) > 0 {
				kindGroupVersion += "/" + gv.Version
			}

			actualCategories[kindGroupVersion] = resource.Categories
		}
	}

	for groupVersion, seen := range expectedGroups {
		if !seen {
			t.Fatalf("expected group/version %q to be discoverable", groupVersion)
		}
	}

	if len(actualCategories) != len(expectedCategories) {
		t.Fatalf("expected\n\n %v\n to contain categories, but got\n\n %v", expectedCategories, actualCategories)
	}

	for expected, expectedAliases := range expectedCategories {
		actualAliases, exists := actualCategories[expected]
		if !exists {
			t.Fatalf("expected resource %v to contain at least one category", expected)
		}

		if len(actualAliases) != len(expectedAliases) {
			t.Fatalf("expected %v to contain categories %v but got %v", expected, expectedAliases, actualAliases)
		}

		for _, actualAlias := range actualAliases {
			for _, expectedAlias := range expectedAliases {
				if actualAlias != expectedAlias {
					t.Fatalf("expected category for %v to be %v, but got %v", expectedAlias, expected, actualAlias)
				}
			}
		}
	}
}
