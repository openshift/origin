package etcd

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/buildconfig"
)

const BuildConfigPath = "/buildconfigs"

type REST struct {
	*etcdgeneric.Etcd
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewStorage(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:      func() runtime.Object { return &api.BuildConfig{} },
		NewListFunc:  func() runtime.Object { return &api.BuildConfigList{} },
		EndpointName: "buildconfig",
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, BuildConfigPath)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, BuildConfigPath, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.BuildConfig).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return buildconfig.Matcher(label, field)
		},

		CreateStrategy:      buildconfig.Strategy,
		UpdateStrategy:      buildconfig.Strategy,
		DeleteStrategy:      buildconfig.Strategy,
		ReturnDeletedObject: false,
		Storage:             s,
	}

	return &REST{store}
}

func (r *REST) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	bcObj, err := r.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	bc := bcObj.(*api.BuildConfig)

	// Just update DeletionTimestamp so Builds get deleted
	if bc.DeletionTimestamp.IsZero() {
		now := unversioned.Now()
		bc.DeletionTimestamp = &now
		bcObj, _, err = r.Update(ctx, bc)
		return bcObj, err
	}

	if !bc.Status.CanDelete {
		err = kapierrors.NewConflict("BuildConfig", fmt.Sprintf("%s/%s", bc.Namespace, bc.Name), fmt.Errorf("The system is ensuring all Builds instantiated from this BuildConfig are removed. Upon completion, this BuildConfig will automatically be purged by the system."))
		return nil, err
	}

	key, err := r.KeyFunc(ctx, name)
	if err != nil {
		return nil, err
	}
	out := r.NewFunc()
	err = r.Storage.Delete(ctx, key, out)
	return out, err
}
