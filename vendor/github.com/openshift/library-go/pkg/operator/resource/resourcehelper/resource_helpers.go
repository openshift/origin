package resourcehelper

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/openshift/api"
)

var (
	openshiftScheme = runtime.NewScheme()
)

func init() {
	if err := api.Install(openshiftScheme); err != nil {
		panic(err)
	}
}

// FormatResourceForCLIWithNamespace generates a string that can be copy/pasted for use with oc get that includes
// specifying the namespace with the -n option (e.g., `ConfigMap/cluster-config-v1 -n kube-system`).
func FormatResourceForCLIWithNamespace(obj runtime.Object) string {
	gvk := GuessObjectGroupVersionKind(obj)
	kind := gvk.Kind
	group := gvk.Group
	var name, namespace string
	accessor, err := meta.Accessor(obj)
	if err != nil {
		name = "<unknown>"
		namespace = "<unknown>"
	} else {
		name = accessor.GetName()
		namespace = accessor.GetNamespace()
	}
	if len(group) > 0 {
		group = "." + group
	}
	if len(namespace) > 0 {
		namespace = " -n " + namespace
	}
	return kind + group + "/" + name + namespace
}

// FormatResourceForCLI generates a string that can be copy/pasted for use with oc get.
func FormatResourceForCLI(obj runtime.Object) string {
	gvk := GuessObjectGroupVersionKind(obj)
	kind := gvk.Kind
	group := gvk.Group
	var name string
	accessor, err := meta.Accessor(obj)
	if err != nil {
		name = "<unknown>"
	} else {
		name = accessor.GetName()
	}
	if len(group) > 0 {
		group = "." + group
	}
	return kind + group + "/" + name
}

// GuessObjectGroupVersionKind returns a human readable for the passed runtime object.
func GuessObjectGroupVersionKind(object runtime.Object) schema.GroupVersionKind {
	if gvk := object.GetObjectKind().GroupVersionKind(); len(gvk.Kind) > 0 {
		return gvk
	}
	if kinds, _, _ := scheme.Scheme.ObjectKinds(object); len(kinds) > 0 {
		return kinds[0]
	}
	if kinds, _, _ := openshiftScheme.ObjectKinds(object); len(kinds) > 0 {
		return kinds[0]
	}
	return schema.GroupVersionKind{Kind: "<unknown>"}
}
