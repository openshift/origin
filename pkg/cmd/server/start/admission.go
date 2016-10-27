package start

import (
	"io"

	"github.com/golang/glog"

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

	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storageclass/default"

	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imagepolicy "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	quotaadmission "github.com/openshift/origin/pkg/quota/admission/resourcequota"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"
	"k8s.io/kubernetes/plugin/pkg/admission/namespace/lifecycle"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

var (
	defaultOnPlugins = sets.NewString(
		"OriginNamespaceLifecycle",
		"openshift.io/JenkinsBootstrapper",
		"BuildByStrategy",
		storageclassdefaultadmission.PluginName,
		imageadmission.PluginName,
		lifecycle.PluginName,
		"OriginPodNodeEnvironment",
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		"LimitRanger",
		"ServiceAccount",
		"SecurityContextConstraint",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"DefaultStorageClass",
		"OwnerReferencesPermissionEnforcement",
		quotaadmission.PluginName,
		"openshift.io/ClusterResourceQuota",
	)

	// defaultOffPlugins includes plugins which require explicit configuration to run
	// if you wire them incorrectly, they may prevent the server from starting
	defaultOffPlugins = sets.NewString(
		"ProjectRequestLimit",
		"RunOnceDuration",
		"PodNodeConstraints",
		overrideapi.PluginName,
		imagepolicy.PluginName,
		"AlwaysPullImages",
		"ImagePolicyWebhook",
	)
)

func init() {
	admission.PluginEnabledFn = IsAdmissionPluginActivated
}

func IsAdmissionPluginActivated(name string, config io.Reader) bool {
	// only intercept if we have an explicit enable or disable.  If the check fails in any way,
	// assume that the config was a different type and let the actual admission plugin check it
	if defaultOnPlugins.Has(name) {
		if enabled, err := configlatest.IsAdmissionPluginActivated(config, true); err == nil && !enabled {
			glog.V(2).Infof("Admission plugin %v is disabled.  It will not be started.", name)
			return false
		}
	} else if defaultOffPlugins.Has(name) {
		if enabled, err := configlatest.IsAdmissionPluginActivated(config, false); err == nil && !enabled {
			glog.V(2).Infof("Admission plugin %v is not enabled.  It will not be started.", name)
			return false
		}
	}

	return true
}
