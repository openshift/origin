package start

import (
	"testing"

	// this package has imports for all the admission controllers used in the kube api server
	// it causes all the admission plugins to be registered, giving us a full listing.
	_ "k8s.io/kubernetes/cmd/kube-apiserver/app"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

var admissionPluginsNotUsedByKube = sets.NewString(
	"AlwaysAdmit",            // from kube, no need for this by default
	"AlwaysDeny",             // from kube, no need for this by default
	"NamespaceAutoProvision", // from kube, it creates a namespace if a resource is created in a non-existent namespace.  We don't want this behavior
	"SecurityContextDeny",    // from kube, it denies pods that want to use SCC capabilities.  We have different rules to allow this in openshift.
	"DenyExecOnPrivileged",   // from kube, it denies exec to pods that have certain privileges.  This is superceded in origin by SCCExecRestrictions that checks against SCC rules.

	"BuildByStrategy",          // from origin, only needed for managing builds, not kubernetes resources
	"OriginNamespaceLifecycle", // from origin, only needed for rejecting openshift resources, so not needed by kube

	"NamespaceExists",  // superceded by NamespaceLifecycle
	"InitialResources", // do we want this? https://github.com/kubernetes/kubernetes/blob/master/docs/proposals/initial-resources.md
)

func TestKubeAdmissionControllerUsage(t *testing.T) {
	registeredKubePlugins := sets.NewString(admission.GetPlugins()...)

	usedAdmissionPlugins := sets.NewString(kubernetes.AdmissionPlugins...)

	if missingPlugins := usedAdmissionPlugins.Difference(registeredKubePlugins); len(missingPlugins) != 0 {
		t.Errorf("%v not found", missingPlugins.List())
	}

	if notUsed := registeredKubePlugins.Difference(usedAdmissionPlugins); len(notUsed) != 0 {
		for pluginName := range notUsed {
			if !admissionPluginsNotUsedByKube.Has(pluginName) {
				t.Errorf("%v not used", pluginName)
			}
		}
	}
}
