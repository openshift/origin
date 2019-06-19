package openshift_kube_apiserver

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog"
	"k8s.io/kube-aggregator/pkg/apiserver"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	"k8s.io/kubernetes/pkg/capabilities"
	"k8s.io/kubernetes/pkg/kubeapiserver/options"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation/customresourcevalidationregistration"
	"k8s.io/kubernetes/openshift-kube-apiserver/kubeadmission"
	"k8s.io/kubernetes/openshift-kube-apiserver/openshiftkubeapiserver"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

func RunOpenShiftKubeAPIServerServer(kubeAPIServerConfig *kubecontrolplanev1.KubeAPIServerConfig, stopCh <-chan struct{}) error {
	// This allows to move crqs, sccs, and rbrs to CRD
	apiserver.AddAlwaysLocalDelegateForPrefix("/apis/quota.openshift.io/v1/clusterresourcequotas")
	apiserver.AddAlwaysLocalDelegateForPrefix("/apis/security.openshift.io/v1/securitycontextconstraints")
	apiserver.AddAlwaysLocalDelegateForPrefix("/apis/authorization.openshift.io/v1/rolebindingrestrictions")
	apiserver.AddAlwaysLocalDelegateGroupResource(schema.GroupResource{Group: "authorization.openshift.io", Resource: "rolebindingrestrictions"})

	// This allows the CRD registration to avoid fighting with the APIService from the operator
	apiserver.AddOverlappingGroupVersion(schema.GroupVersion{Group: "authorization.openshift.io", Version: "v1"})

	// Allow privileged containers
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
		PrivilegedSources: capabilities.PrivilegedSources{
			HostNetworkSources: []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostPIDSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostIPCSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
		},
	})

	bootstrappolicy.ClusterRoles = bootstrappolicy.OpenshiftClusterRoles
	bootstrappolicy.ClusterRoleBindings = bootstrappolicy.OpenshiftClusterRoleBindings

	options.AllOrderedPlugins = kubeadmission.NewOrderedKubeAdmissionPlugins(options.AllOrderedPlugins)
	kubeRegisterAdmission := options.RegisterAllAdmissionPlugins
	options.RegisterAllAdmissionPlugins = func(plugins *admission.Plugins) {
		kubeRegisterAdmission(plugins)
		kubeadmission.RegisterOpenshiftKubeAdmissionPlugins(plugins)
		customresourcevalidationregistration.RegisterCustomResourceValidation(plugins)
	}
	options.DefaultOffAdmissionPlugins = kubeadmission.NewDefaultOffPluginsFunc(options.DefaultOffAdmissionPlugins())

	configPatchFn, serverPatchContext := openshiftkubeapiserver.NewOpenShiftKubeAPIServerConfigPatch(genericapiserver.NewEmptyDelegate(), kubeAPIServerConfig)
	app.OpenShiftKubeAPIServerConfigPatch = configPatchFn
	app.OpenShiftKubeAPIServerServerPatch = serverPatchContext.PatchServer

	cmd := app.NewAPIServerCommand(stopCh)
	args, err := openshiftkubeapiserver.ConfigToFlags(kubeAPIServerConfig)
	if err != nil {
		return err
	}
	if err := cmd.ParseFlags(args); err != nil {
		return err
	}
	klog.Infof("`kube-apiserver %v`", args)
	if err := cmd.RunE(cmd, nil); err != nil {
		return err
	}

	return nil
}
