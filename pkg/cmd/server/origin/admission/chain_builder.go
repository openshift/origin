package admission

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	noderestriction "k8s.io/kubernetes/plugin/pkg/admission/noderestriction"
	saadmit "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	storageclassdefaultadmission "k8s.io/kubernetes/plugin/pkg/admission/storageclass/setdefault"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	imageadmission "github.com/openshift/origin/pkg/image/admission"
	imagepolicy "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	ingressadmission "github.com/openshift/origin/pkg/ingress/admission"
	overrideapi "github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"

	serviceadmit "github.com/openshift/origin/pkg/service/admission"
)

var (
	// openshiftAdmissionControlPlugins gives the in-order default admission chain for openshift resources.
	openshiftAdmissionControlPlugins = []string{
		"ProjectRequestLimit",
		"OriginNamespaceLifecycle",
		"openshift.io/RestrictSubjectBindings",
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		imageadmission.PluginName,
		"OwnerReferencesPermissionEnforcement",
		"Initializers",
		"GenericAdmissionWebhook",
		"ResourceQuota",
	}

	// KubeAdmissionPlugins gives the in-order default admission chain for kube resources.
	KubeAdmissionPlugins = []string{
		lifecycle.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		overrideapi.PluginName,
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		imagepolicy.PluginName,
		"ImagePolicyWebhook",
		"PodPreset",
		"LimitRanger",
		"ServiceAccount",
		noderestriction.PluginName,
		"SecurityContextConstraint",
		storageclassdefaultadmission.PluginName,
		"AlwaysPullImages",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		ingressadmission.IngressAdmission,
		"DefaultTolerationSeconds",
		"Initializers",
		"GenericAdmissionWebhook",
		"PodTolerationRestriction",
		// NOTE: ResourceQuota and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}

	// CombinedAdmissionControlPlugins gives the in-order default admission chain for all resources resources.
	// When possible, this list is used.  The set of openshift+kube chains must exactly match this set.  In addition,
	// the order specified in the openshift and kube chains must match the order here.
	CombinedAdmissionControlPlugins = []string{
		lifecycle.PluginName,
		"ProjectRequestLimit",
		"OriginNamespaceLifecycle",
		"openshift.io/RestrictSubjectBindings",
		"PodNodeConstraints",
		"openshift.io/JenkinsBootstrapper",
		"openshift.io/BuildConfigSecretInjector",
		"BuildByStrategy",
		imageadmission.PluginName,
		"RunOnceDuration",
		"PodNodeConstraints",
		"OriginPodNodeEnvironment",
		"PodNodeSelector",
		overrideapi.PluginName,
		serviceadmit.ExternalIPPluginName,
		serviceadmit.RestrictedEndpointsPluginName,
		imagepolicy.PluginName,
		"ImagePolicyWebhook",
		"PodPreset",
		"LimitRanger",
		"ServiceAccount",
		noderestriction.PluginName,
		"SecurityContextConstraint",
		storageclassdefaultadmission.PluginName,
		"AlwaysPullImages",
		"LimitPodHardAntiAffinityTopology",
		"SCCExecRestrictions",
		"PersistentVolumeLabel",
		"OwnerReferencesPermissionEnforcement",
		ingressadmission.IngressAdmission,
		"DefaultTolerationSeconds",
		"Initializers",
		"GenericAdmissionWebhook",
		"PodTolerationRestriction",
		// NOTE: ResourceQuota and ClusterResourceQuota must be the last 2 plugins.
		// DO NOT ADD ANY PLUGINS AFTER THIS LINE!
		"ResourceQuota",
		"openshift.io/ClusterResourceQuota",
	}
)

// fixupAdmissionPlugins fixes the input plugins to handle deprecation and duplicates.
func fixupAdmissionPlugins(plugins []string) []string {
	result := replace(plugins, "openshift.io/OriginResourceQuota", "ResourceQuota")
	result = dedupe(result)
	return result
}

func NewAdmissionChains(
	options configapi.MasterConfig,
	kubeClientSet kclientsetinternal.Interface,
	admissionInitializer admission.PluginInitializer,
) (admission.Interface /*origin*/, admission.Interface /*kube*/, error) {
	// check to see if they've taken explicit control of the kube admission chain
	// this happens when any of the following are true:
	// 1. extended kube server args are used to change the admission plugin list
	// 2. kube explicit config changes the admission plugin list
	// 3. extended kube server args are used to change the admission config file
	// 4. openshift explicit config changes the admission plugins list
	// 5. kube and openshift plugin config try to configure the same plugin differently
	// TODO: one release from now, I think we should start failing on setting the kube admission config
	//       two releases from now, I think we should start removing it
	//       two releases from now, I think we should remove the PluginOverrideOrder entirely
	hasSeparateKubeAdmissionChain := false
	KubeAdmissionPlugins := KubeAdmissionPlugins
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.APIServerArguments["admission-control"]) > 0 {
		KubeAdmissionPlugins = strings.Split(options.KubernetesMasterConfig.APIServerArguments["admission-control"][0], ",")
		hasSeparateKubeAdmissionChain = true
	}
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride) > 0 {
		KubeAdmissionPlugins = options.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride
		hasSeparateKubeAdmissionChain = true
	}
	KubeAdmissionPlugins = fixupAdmissionPlugins(KubeAdmissionPlugins)

	kubeAdmissionPluginConfigFilename := ""
	if options.KubernetesMasterConfig != nil && len(options.KubernetesMasterConfig.APIServerArguments["admission-control-config-file"]) > 0 {
		kubeAdmissionPluginConfigFilename = options.KubernetesMasterConfig.APIServerArguments["admission-control-config-file"][0]
		hasSeparateKubeAdmissionChain = true
	}

	openshiftAdmissionPlugins := openshiftAdmissionControlPlugins
	if len(options.AdmissionConfig.PluginOrderOverride) > 0 {
		openshiftAdmissionPlugins = options.AdmissionConfig.PluginOrderOverride
		hasSeparateKubeAdmissionChain = true
	}
	openshiftAdmissionPlugins = fixupAdmissionPlugins(openshiftAdmissionPlugins)

	if options.KubernetesMasterConfig != nil && !hasSeparateKubeAdmissionChain {
		// check for collisions between openshift and kube plugin config
		for pluginName, kubeConfig := range options.KubernetesMasterConfig.AdmissionConfig.PluginConfig {
			if openshiftConfig, exists := options.AdmissionConfig.PluginConfig[pluginName]; exists && !reflect.DeepEqual(kubeConfig, openshiftConfig) {
				hasSeparateKubeAdmissionChain = true
				break
			}
		}
	}

	if hasSeparateKubeAdmissionChain {
		// build kube admission
		var kubeAdmission admission.Interface
		if options.KubernetesMasterConfig != nil {
			var err error
			kubeAdmission, err = newAdmissionChainFunc(KubeAdmissionPlugins, kubeAdmissionPluginConfigFilename, options.KubernetesMasterConfig.AdmissionConfig.PluginConfig, options, kubeClientSet, admissionInitializer)
			if err != nil {
				return nil, nil, err
			}
		}

		// build openshift admission
		openshiftAdmission, err := newAdmissionChainFunc(openshiftAdmissionPlugins, "", options.AdmissionConfig.PluginConfig, options, kubeClientSet, admissionInitializer)
		if err != nil {
			return nil, nil, err
		}

		return openshiftAdmission, kubeAdmission, nil
	}

	// if we have a unified chain, build the combined config
	pluginConfig := map[string]configapi.AdmissionPluginConfig{}
	if options.KubernetesMasterConfig != nil {
		for pluginName, config := range options.KubernetesMasterConfig.AdmissionConfig.PluginConfig {
			pluginConfig[pluginName] = config
		}
	}
	for pluginName, config := range options.AdmissionConfig.PluginConfig {
		pluginConfig[pluginName] = config
	}

	admissionChain, err := newAdmissionChainFunc(CombinedAdmissionControlPlugins, "", pluginConfig, options, kubeClientSet, admissionInitializer)
	if err != nil {
		return nil, nil, err
	}

	return admissionChain, admissionChain, err
}

// newAdmissionChainFunc is for unit testing only.  You should NEVER OVERRIDE THIS outside of a unit test.
var newAdmissionChainFunc = newAdmissionChain

func newAdmissionChain(pluginNames []string, admissionConfigFilename string, pluginConfig map[string]configapi.AdmissionPluginConfig, options configapi.MasterConfig, kubeClientSet kclientsetinternal.Interface, admissionInitializer admission.PluginInitializer) (admission.Interface, error) {
	plugins := []admission.Interface{}
	for _, pluginName := range pluginNames {
		var (
			plugin             admission.Interface
			skipInitialization bool
		)

		switch pluginName {
		case lifecycle.PluginName:
			// We need to include our infrastructure and shared resource namespaces in the immortal namespaces list
			immortalNamespaces := sets.NewString(metav1.NamespaceDefault)
			if len(options.PolicyConfig.OpenShiftSharedResourcesNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftSharedResourcesNamespace)
			}
			if len(options.PolicyConfig.OpenShiftInfrastructureNamespace) > 0 {
				immortalNamespaces.Insert(options.PolicyConfig.OpenShiftInfrastructureNamespace)
			}
			lc, err := lifecycle.NewLifecycle(immortalNamespaces)
			if err != nil {
				return nil, err
			}
			admissionInitializer.Initialize(lc)
			if err := lc.(admission.Validator).Validate(); err != nil {
				return nil, err
			}
			plugin = lc

		case serviceadmit.ExternalIPPluginName:
			// this needs to be moved upstream to be part of core config
			reject, admit, err := serviceadmit.ParseRejectAdmitCIDRRules(options.NetworkConfig.ExternalIPNetworkCIDRs)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			allowIngressIP := false
			if _, ipNet, err := net.ParseCIDR(options.NetworkConfig.IngressIPNetworkCIDR); err == nil && !ipNet.IP.IsUnspecified() {
				allowIngressIP = true
			}
			plugin = serviceadmit.NewExternalIPRanger(reject, admit, allowIngressIP)

		case serviceadmit.RestrictedEndpointsPluginName:
			// we need to set some customer parameters, so create by hand
			var restricted []string
			restricted = append(restricted, options.NetworkConfig.ServiceNetworkCIDR)
			for _, cidr := range options.NetworkConfig.ClusterNetworks {
				restricted = append(restricted, cidr.CIDR)
			}
			restrictedNetworks, err := serviceadmit.ParseSimpleCIDRRules(restricted)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			plugin = serviceadmit.NewRestrictedEndpointsAdmission(restrictedNetworks)

		case saadmit.PluginName:
			// we need to set some custom parameters on the service account admission controller, so create that one by hand
			saAdmitter := saadmit.NewServiceAccount()
			saAdmitter.SetInternalKubeClientSet(kubeClientSet)
			saAdmitter.LimitSecretReferences = options.ServiceAccountConfig.LimitSecretReferences
			plugin = saAdmitter

		default:
			configFile, err := pluginconfig.GetAdmissionConfigurationFile(pluginConfig, pluginName, admissionConfigFilename)
			if err != nil {
				return nil, err
			}
			configReader, err := admission.ReadAdmissionConfiguration([]string{pluginName}, configFile)
			if err != nil {
				return nil, err
			}
			pluginConfigReader, err := configReader.ConfigFor(pluginName)
			if err != nil {
				return nil, err
			}

			plugin, err = OriginAdmissionPlugins.InitPlugin(pluginName, pluginConfigReader, admissionInitializer)
			if err != nil {
				// should have been caught with validation
				return nil, err
			}
			if plugin == nil {
				continue
			}

			// skip initialization below because admission.InitPlugin does all the work
			skipInitialization = true
		}

		plugins = append(plugins, plugin)

		if !skipInitialization {
			admissionInitializer.Initialize(plugin)
		}
	}

	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}

	return admission.NewChainHandler(plugins...), nil
}

// replace returns a slice where each instance of the input that is x is replaced with y
func replace(input []string, x, y string) []string {
	result := []string{}
	for i := range input {
		if input[i] == x {
			result = append(result, y)
		} else {
			result = append(result, input[i])
		}
	}
	return result
}

// dedupe removes duplicate items from the input list.
// the last instance of a duplicate is kept in the input list.
func dedupe(input []string) []string {
	items := sets.NewString()
	result := []string{}
	for i := len(input) - 1; i >= 0; i-- {
		if items.Has(input[i]) {
			continue
		}
		items.Insert(input[i])
		result = append([]string{input[i]}, result...)
	}
	return result
}

func init() {
	// add a filter that will remove DefaultAdmissionConfig
	admission.FactoryFilterFn = filterEnableAdmissionConfigs
}

func filterEnableAdmissionConfigs(delegate admission.Factory) admission.Factory {
	return func(config io.Reader) (admission.Interface, error) {
		config1, config2, err := splitStream(config)
		if err != nil {
			return nil, err
		}
		// if the config isn't a DefaultAdmissionConfig, then assume we're enabled (we were called after all)
		// if the config *is* a DefaultAdmissionConfig and it explicitly said
		obj, err := configlatest.ReadYAML(config1)
		// if we can't read it, let the plugin deal with it
		if err != nil {
			return delegate(config2)
		}
		// if nothing was there, let the plugin deal with it
		if obj == nil {
			return delegate(config2)
		}
		// if it wasn't a DefaultAdmissionConfig object, let the plugin deal with it
		if _, ok := obj.(*configapi.DefaultAdmissionConfig); !ok {
			return delegate(config2)
		}

		// if it was a DefaultAdmissionConfig, then it must have said "enabled" and it wasn't really meant for the
		// admission plugin
		return delegate(nil)
	}
}

// splitStream reads the stream bytes and constructs two copies of it.
func splitStream(config io.Reader) (io.Reader, io.Reader, error) {
	if config == nil || reflect.ValueOf(config).IsNil() {
		return nil, nil, nil
	}

	configBytes, err := ioutil.ReadAll(config)
	if err != nil {
		return nil, nil, err
	}

	return bytes.NewBuffer(configBytes), bytes.NewBuffer(configBytes), nil
}
