package openshiftrestmapper

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// defaultRESTMappings contains enough RESTMappings to have enough of the kube-controller-manager succeed when running
// against a kube-apiserver that cannot reach aggregated APIs to do a full mapping.  This happens when the OwnerReferencesPermissionEnforcement
// admission plugin runs to confirm permissions.  Don't add things just because you don't want to fail.  These are here so that
// we can start enough back up to get the rest of the system working correctly.
var defaultRESTMappings = []meta.RESTMapping{
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ReplicationController"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ServiceAccount"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ControllerRevision"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "controllerrevisions"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"},
	},
	// This is created so that cluster-bootstrap can always map securitycontextconstraints since the CRD doesn't have
	// discovery. Discovery is delegated to the openshift-apiserver which doesn't not exist early in the bootstrapping
	// phase.  This leads to SCC related failures that we don't need to have.
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "security.openshift.io", Version: "v1", Kind: "SecurityContextConstraints"},
		Scope:            meta.RESTScopeRoot,
		Resource:         schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints"},
	},
	{
		GroupVersionKind: schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
		Scope:            meta.RESTScopeNamespace,
		Resource:         schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
	},
}

func NewOpenShiftHardcodedRESTMapper(delegate meta.RESTMapper) meta.RESTMapper {
	ret := HardCodedFirstRESTMapper{
		Mapping:    map[schema.GroupVersionKind]meta.RESTMapping{},
		RESTMapper: delegate,
	}
	for i := range defaultRESTMappings {
		curr := defaultRESTMappings[i]
		ret.Mapping[curr.GroupVersionKind] = curr
	}
	return ret
}

// HardCodedFirstRESTMapper is a RESTMapper that will look for hardcoded mappings first, then delegate.
// This is done in service to `OwnerReferencesPermissionEnforcement` and for cluster-bootstrap.
type HardCodedFirstRESTMapper struct {
	Mapping map[schema.GroupVersionKind]meta.RESTMapping
	meta.RESTMapper
}

var _ meta.RESTMapper = HardCodedFirstRESTMapper{}

func (m HardCodedFirstRESTMapper) String() string {
	return fmt.Sprintf("HardCodedRESTMapper{\n\t%v\n%v\n}", m.Mapping, m.RESTMapper)
}

// RESTMapping is the only function called today.  The first hit openshiftrestmapper ought to make this work right.  OwnerReferencesPermissionEnforcement
// only ever calls with one version.
func (m HardCodedFirstRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	// not exactly one version, delegate
	if len(versions) != 1 {
		return m.RESTMapper.RESTMapping(gk, versions...)
	}
	gvk := gk.WithVersion(versions[0])

	single, ok := m.Mapping[gvk]
	// not handled, delegate
	if !ok {
		return m.RESTMapper.RESTMapping(gk, versions...)
	}

	return &single, nil
}

// RESTMapping is the only function called today.  The firsthit openshiftrestmapper ought to make this work right.  OwnerReferencesPermissionEnforcement
// only ever calls with one version.
func (m HardCodedFirstRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	// not exactly one version, delegate
	if len(versions) != 1 {
		return m.RESTMapper.RESTMappings(gk, versions...)
	}
	gvk := gk.WithVersion(versions[0])

	single, ok := m.Mapping[gvk]
	// not handled, delegate
	if !ok {
		return m.RESTMapper.RESTMappings(gk, versions...)
	}

	return []*meta.RESTMapping{&single}, nil
}
