package etcd

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/clusterquota"
)

type REST struct {
	*etcdgeneric.Etcd
}

// NewStorage returns a RESTStorage object that will work against Build objects.
func NewREST(s storage.Interface) (*REST, *StatusREST) {
	prefix := "/clusterresourcequotas"

	store := &etcdgeneric.Etcd{
		NewFunc:           func() runtime.Object { return &api.ClusterResourceQuota{} },
		NewListFunc:       func() runtime.Object { return &api.ClusterResourceQuotaList{} },
		QualifiedResource: api.Resource("clusterresourcequotas"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return prefix
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NoNamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.ClusterResourceQuota).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return clusterquota.Matcher(label, field)
		},
		CreateStrategy: clusterquota.Strategy,
		UpdateStrategy: clusterquota.Strategy,
		DeleteStrategy: clusterquota.Strategy,
		Storage:        s,
	}

	statusStore := *store
	statusStore.UpdateStrategy = clusterquota.StatusStrategy

	return &REST{store}, &StatusREST{store: &statusStore}
}

// StatusREST implements the REST endpoint for changing the status of a resourcequota.
type StatusREST struct {
	store *etcdgeneric.Etcd
}

func (r *StatusREST) New() runtime.Object {
	return &api.ClusterResourceQuota{}
}

// Update alters the status subset of an object.
func (r *StatusREST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return r.store.Update(ctx, obj)
}
