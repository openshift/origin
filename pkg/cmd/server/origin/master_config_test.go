package origin

import (
	"testing"

	quotaadmission "github.com/openshift/origin/pkg/quota/admission/resourcequota"
)

func TestQuotaAdmissionPluginsAreLast(t *testing.T) {
	kubeLen := len(KubeAdmissionPlugins)
	if kubeLen < 2 {
		t.Fatalf("must have at least the 2 quota plugins")
	}

	if KubeAdmissionPlugins[kubeLen-2] != quotaadmission.PluginName {
		t.Errorf("KubeAdmissionPlugins must have %s as the next to last plugin", quotaadmission.PluginName)
	}

	if KubeAdmissionPlugins[kubeLen-1] != "openshift.io/ClusterResourceQuota" {
		t.Errorf("KubeAdmissionPlugins must have ClusterResourceQuota as the last plugin")
	}

	combinedLen := len(CombinedAdmissionControlPlugins)
	if CombinedAdmissionControlPlugins[combinedLen-2] != quotaadmission.PluginName {
		t.Errorf("CombinedAdmissionControlPlugins must have %s as the next to last plugin", quotaadmission.PluginName)
	}

	if CombinedAdmissionControlPlugins[combinedLen-1] != "openshift.io/ClusterResourceQuota" {
		t.Errorf("CombinedAdmissionControlPlugins must have ClusterResourceQuota as the last plugin")
	}
}
