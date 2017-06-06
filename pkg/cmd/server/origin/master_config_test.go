package origin

import (
	"reflect"
	"testing"
)

func TestQuotaAdmissionPluginsAreLast(t *testing.T) {
	kubeLen := len(KubeAdmissionPlugins)
	if kubeLen < 2 {
		t.Fatalf("must have at least the 2 quota plugins")
	}

	if KubeAdmissionPlugins[kubeLen-2] != "ResourceQuota" {
		t.Errorf("KubeAdmissionPlugins must have %s as the next to last plugin", "ResourceQuota")
	}

	if KubeAdmissionPlugins[kubeLen-1] != "openshift.io/ClusterResourceQuota" {
		t.Errorf("KubeAdmissionPlugins must have ClusterResourceQuota as the last plugin")
	}

	combinedLen := len(CombinedAdmissionControlPlugins)
	if CombinedAdmissionControlPlugins[combinedLen-2] != "ResourceQuota" {
		t.Errorf("CombinedAdmissionControlPlugins must have %s as the next to last plugin", "ResourceQuota")
	}

	if CombinedAdmissionControlPlugins[combinedLen-1] != "openshift.io/ClusterResourceQuota" {
		t.Errorf("CombinedAdmissionControlPlugins must have ClusterResourceQuota as the last plugin")
	}
}
func TestFixupAdmissionPlugins(t *testing.T) {
	inputList := []string{"DefaultTolerationSeconds", "openshift.io/OriginResourceQuota", "OwnerReferencesPermissionEnforcement", "ResourceQuota", "openshift.io/ClusterResourceQuota"}
	expectedList := []string{"DefaultTolerationSeconds", "OwnerReferencesPermissionEnforcement", "ResourceQuota", "openshift.io/ClusterResourceQuota"}
	actualList := fixupAdmissionPlugins(inputList)
	if !reflect.DeepEqual(expectedList, actualList) {
		t.Errorf("Expected: %v, but got: %v", expectedList, actualList)
	}
}
