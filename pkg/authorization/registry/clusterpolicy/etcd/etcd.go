package etcd

import (
	"fmt"
	gruntime "runtime"
	"runtime/debug"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/util"
	"github.com/openshift/origin/pkg/util/restoptions"
)

const ClusterPolicyPath = "/authorization/cluster/policies"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {

	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &authorizationapi.ClusterPolicy{} },
		NewListFunc:       func() runtime.Object { return &authorizationapi.ClusterPolicyList{} },
		QualifiedResource: authorizationapi.Resource("clusterpolicies"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return ClusterPolicyPath
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, ClusterPolicyPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*authorizationapi.ClusterPolicy).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return clusterpolicy.Matcher(label, field)
		},

		CreateStrategy: clusterpolicy.Strategy,
		UpdateStrategy: clusterpolicy.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, ClusterPolicyPath); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}

func (e *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	found := false
	for _, apiCallStack := range allowedStacks {
		failed := false
		for i := range apiCallStack {
			_, file, line, ok := gruntime.Caller(i + 1)
			if !ok {
				failed = true
				break
			}
			if !strings.Contains(file, apiCallStack[i].file) {
				failed = true
				break
			}
			if apiCallStack[i].line != line {
				failed = true
				break
			}
		}
		if !failed {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("#### DISALLOWED!\n")
		debug.PrintStack()
	}

	return e.Store.Get(ctx, name)
}

var allowedStacks = [][]struct {
	file string
	line int
}{
	// REST API "get"
	{
		{"pkg/authorization/registry/clusterpolicy/registry.go", 77},
		{"pkg/authorization/registry/clusterpolicy/registry.go", 111},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 65},
		{"pkg/authorization/registry/clusterrole/proxy/proxy.go", 48},
		{"k8s.io/kubernetes/pkg/apiserver/resthandler.go", 145},
	},
	// REST API "delete"
	{
		{"pkg/authorization/registry/clusterpolicy/registry.go", 77},
		{"pkg/authorization/registry/clusterpolicy/registry.go", 111},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 83},
		{"pkg/authorization/registry/clusterrole/proxy/proxy.go", 56},
		{"k8s.io/kubernetes/pkg/apiserver/resthandler.go", 764},
	},
	// REST API "create"
	{
		{"pkg/authorization/registry/clusterpolicy/registry.go", 77},
		{"pkg/authorization/registry/clusterpolicy/registry.go", 111},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 196},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 124},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 105},
		{"pkg/authorization/registry/clusterrole/proxy/proxy.go", 68},
		{"k8s.io/kubernetes/pkg/apiserver/resthandler.go", 439},
	},
	// REST API "update"
	{
		{"pkg/authorization/registry/clusterpolicy/registry.go", 77},
		{"pkg/authorization/registry/clusterpolicy/registry.go", 111},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 171},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 144},
		{"pkg/authorization/registry/clusterrole/proxy/proxy.go", 80},
		{"k8s.io/kubernetes/pkg/apiserver/resthandler.go", 682},
	},
	// REST API "update"
	{
		{"pkg/authorization/registry/clusterpolicy/registry.go", 77},
		{"pkg/authorization/registry/clusterpolicy/registry.go", 111},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 65},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 156},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 144},
		{"pkg/authorization/registry/clusterrole/proxy/proxy.go", 80},
		{"k8s.io/kubernetes/pkg/apiserver/resthandler.go", 682},
	},
	// REST API "patch"
	{
		{"pkg/authorization/registry/clusterpolicy/registry.go", 77},
		{"pkg/authorization/registry/clusterpolicy/registry.go", 111},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 65},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 156},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 144},
		{"pkg/authorization/registry/clusterrole/proxy/proxy.go", 80},
		{"k8s.io/kubernetes/pkg/apiserver/resthandler.go", 552},
	},
	// REST API "patch"
	{
		{"pkg/authorization/registry/clusterpolicy/registry.go", 77},
		{"pkg/authorization/registry/clusterpolicy/registry.go", 111},
		{"pkg/authorization/registry/role/policybased/virtual_storage.go", 65},
		{"pkg/authorization/registry/clusterrole/proxy/proxy.go", 48},
		{"k8s.io/kubernetes/pkg/apiserver/resthandler.go", 524},
	},
}
