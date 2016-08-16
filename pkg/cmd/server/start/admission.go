package start

import (
	"io"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/util/sets"

	// Admission control plug-ins used by OpenShift
	_ "github.com/openshift/origin/pkg/build/admission/defaults"
	_ "github.com/openshift/origin/pkg/build/admission/jenkinsbootstrapper"
	_ "github.com/openshift/origin/pkg/build/admission/overrides"
	_ "github.com/openshift/origin/pkg/build/admission/strategyrestrictions"
	_ "github.com/openshift/origin/pkg/image/admission"
	_ "github.com/openshift/origin/pkg/image/admission/imagepolicy"
	_ "github.com/openshift/origin/pkg/project/admission/lifecycle"
	_ "github.com/openshift/origin/pkg/project/admission/nodeenv"
	_ "github.com/openshift/origin/pkg/project/admission/requestlimit"
	_ "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	_ "github.com/openshift/origin/pkg/quota/admission/clusterresourcequota"
	_ "github.com/openshift/origin/pkg/quota/admission/resourcequota"
	_ "github.com/openshift/origin/pkg/quota/admission/runonceduration"
	_ "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
	_ "github.com/openshift/origin/pkg/security/admission"
	_ "k8s.io/kubernetes/plugin/pkg/admission/admit"
	_ "k8s.io/kubernetes/plugin/pkg/admission/alwayspullimages"
	_ "k8s.io/kubernetes/plugin/pkg/admission/exec"
	_ "k8s.io/kubernetes/plugin/pkg/admission/limitranger"
	_ "k8s.io/kubernetes/plugin/pkg/admission/namespace/exists"
	_ "k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"
	_ "k8s.io/kubernetes/plugin/pkg/admission/persistentvolume/label"
	_ "k8s.io/kubernetes/plugin/pkg/admission/resourcequota"
	_ "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

var (
	defaultOnPlugins  = sets.String{}
	defaultOffPlugins = sets.String{}
)

func init() {
	defaultOffPlugins.Insert("AlwaysPullImages")
	defaultOnPlugins.Insert("openshift.io/ClusterResourceQuota")
	admission.PluginEnabledFn = IsAdmissionPluginActivated
}

func IsAdmissionPluginActivated(name string, config io.Reader) bool {
	// only intercept if we have an explicit enable or disable.  If the check fails in any way,
	// assume that the config was a different type and let the actual admission plugin check it
	if defaultOnPlugins.Has(name) {
		if enabled, err := configlatest.IsAdmissionPluginActivated(config, true); err == nil && !enabled {
			return false
		}
	} else if defaultOffPlugins.Has(name) {
		if enabled, err := configlatest.IsAdmissionPluginActivated(config, false); err == nil && !enabled {
			return false
		}
	}

	return true
}
