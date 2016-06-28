package clusterresourcequota

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	quotaapi "github.com/openshift/origin/pkg/quota/api"
	"github.com/openshift/origin/pkg/util"
	"github.com/openshift/origin/pkg/util/restoptions"
)

const ClusterResourceQuotaPath = "/" + quotaapi.GroupName + "/clusterresourcequotas"

type REST struct {
	*registry.Store
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:           func() runtime.Object { return &quotaapi.ClusterResourceQuota{} },
		NewListFunc:       func() runtime.Object { return &quotaapi.ClusterResourceQuotaList{} },
		QualifiedResource: quotaapi.Resource("clusterresourcequotas"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return ClusterResourceQuotaPath
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, ClusterResourceQuotaPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*quotaapi.ClusterResourceQuota).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return Matcher(label, field)
		},

		CreateStrategy:      Strategy,
		UpdateStrategy:      Strategy,
		DeleteStrategy:      Strategy,
		ReturnDeletedObject: false,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, ClusterResourceQuotaPath); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}
