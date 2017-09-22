package admission

import (
	"bytes"
	"io"
	"io/ioutil"
	"reflect"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kubeapiserver "k8s.io/kubernetes/cmd/kube-apiserver/app"

	// Admission control plug-ins used by OpenShift
	authorizationrestrictusers "github.com/openshift/origin/pkg/authorization/admission/restrictusers"
	buildjenkinsbootstrapper "github.com/openshift/origin/pkg/build/admission/jenkinsbootstrapper"
	buildsecretinjector "github.com/openshift/origin/pkg/build/admission/secretinjector"
	buildstrategyrestrictions "github.com/openshift/origin/pkg/build/admission/strategyrestrictions"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imagepolicy "github.com/openshift/origin/pkg/image/admission/imagepolicy"
	ingressadmission "github.com/openshift/origin/pkg/ingress/admission"
	projectlifecycle "github.com/openshift/origin/pkg/project/admission/lifecycle"
	projectnodeenv "github.com/openshift/origin/pkg/project/admission/nodeenv"
	projectrequestlimit "github.com/openshift/origin/pkg/project/admission/requestlimit"
	quotaclusterresourceoverride "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride"
	quotaclusterresourcequota "github.com/openshift/origin/pkg/quota/admission/clusterresourcequota"
	quotarunonceduration "github.com/openshift/origin/pkg/quota/admission/runonceduration"
	schedulerpodnodeconstraints "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints"
	securityadmission "github.com/openshift/origin/pkg/security/admission"
	serviceadmit "github.com/openshift/origin/pkg/service/admission"

	"k8s.io/kubernetes/plugin/pkg/admission/noderestriction"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storageclass/setdefault"

	imagepolicyapi "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

// TODO register this per apiserver or at least per process
var OriginAdmissionPlugins = &admission.Plugins{}

func init() {
	RegisterAllAdmissionPlugins(OriginAdmissionPlugins)
}

// RegisterAllAdmissionPlugins registers all admission plugins
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	kubeapiserver.RegisterAllAdmissionPlugins(plugins)
	genericapiserver.RegisterAllAdmissionPlugins(plugins)
	authorizationrestrictusers.Register(plugins)
	buildjenkinsbootstrapper.Register(plugins)
	buildsecretinjector.Register(plugins)
	buildstrategyrestrictions.Register(plugins)
	imageadmission.Register(plugins)
	imagepolicy.Register(plugins)
	ingressadmission.Register(plugins)
	projectlifecycle.Register(plugins)
	projectnodeenv.Register(plugins)
	projectrequestlimit.Register(plugins)
	quotaclusterresourceoverride.Register(plugins)
	quotaclusterresourcequota.Register(plugins)
	quotarunonceduration.Register(plugins)
	schedulerpodnodeconstraints.Register(plugins)
	securityadmission.Register(plugins)
	securityadmission.RegisterSCCExecRestrictions(plugins)
	serviceadmit.RegisterExternalIP(plugins)
	serviceadmit.RegisterRestrictedEndpoints(plugins)
}

var (
	DefaultOnPlugins = sets.NewString(
		"OriginNamespaceLifecycle",
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		storageclassdefaultadmission.PluginName,
		imageadmission.PluginName,
		lifecycle.PluginName,
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		"LimitRanger",
		"ServiceAccount",
		noderestriction.PluginName,
		"SecurityContextConstraint",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"DefaultStorageClass",
		"OwnerReferencesPermissionEnforcement",
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
		"openshift.io/IngressAdmission",
	)

	// DefaultOffPlugins includes plugins which require explicit configuration to run
	// if you wire them incorrectly, they may prevent the server from starting
	DefaultOffPlugins = sets.NewString(
		"ProjectRequestLimit",
		"RunOnceDuration",
		"PodNodeConstraints",
		overrideapi.PluginName,
		imagepolicyapi.PluginName,
		"AlwaysPullImages",
		"ImagePolicyWebhook",
		"openshift.io/RestrictSubjectBindings",
		"LimitPodHardAntiAffinityTopology",
		"DefaultTolerationSeconds",
		"PodPreset", // default to off while PodPreset is alpha

		// these are new, reassess post-rebase
		"Initializers",
		"GenericAdmissionWebhook",
		"PodTolerationRestriction",
	)
)

// IsAdmissionPluginActivated returns true if the admission plugin has to be enabled otherwise false.
// It also returns plugin config reader as nill if DefaultAdmissionConfig is passed, otherwise it
// returns the same copy of passed plugin config reader.
func IsAdmissionPluginActivated(name string, pluginConfigReader io.Reader) (bool, io.Reader) {
	input, output, err := splitStream(pluginConfigReader)
	if err != nil {
		return false, nil
	}

	// only intercept if we have an explicit enable or disable.  If the check fails in any way,
	// assume that the config was a different type and let the actual admission plugin check it
	var initMode configlatest.AdmissionPluginInitMode
	if DefaultOnPlugins.Has(name) {
		if initMode = configlatest.IsAdmissionPluginActivated(input, true); initMode == configlatest.Disabled {
			glog.V(2).Infof("Admission plugin %v is disabled.  It will not be started.", name)
		}
	} else if DefaultOffPlugins.Has(name) {
		if initMode = configlatest.IsAdmissionPluginActivated(input, false); initMode == configlatest.Disabled {
			glog.V(2).Infof("Admission plugin %v is not enabled.  It will not be started.", name)
		}
	}

	if initMode == configlatest.EnabledWithDefaultAdmissionConfig {
		output = nil
	}

	return initMode == configlatest.Disabled, output
}

// splitStream reads the stream bytes and constructs two copies of it.
// This is copied from kubernetes
func splitStream(config io.Reader) (io.Reader, io.Reader, error) {
	if config == nil || reflect.ValueOf(config).IsNil() {
		return nil, nil, nil
	}

	configBytes, err := ioutil.ReadAll(config)
	if err != nil {
		glog.Errorf("error reading admission plugin config: %v", err)
		return nil, nil, err
	}

	return bytes.NewBuffer(configBytes), bytes.NewBuffer(configBytes), nil
}
