package etcd

import (
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	etcdgeneric "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
	"github.com/openshift/origin/pkg/user/registry/user"
)

// rest implements a RESTStorage for users against etcd
type REST struct {
	etcdgeneric.Etcd
}

const EtcdPrefix = "/registry/users"

// NewREST returns a RESTStorage object that will work against users
func NewREST(h tools.EtcdHelper) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.User{} },
		NewListFunc: func() runtime.Object { return &api.UserList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return etcdgeneric.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.User).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return user.MatchUser(label, field)
		},
		EndpointName: "users",

		Helper: h,
	}

	store.CreateStrategy = user.Strategy
	store.UpdateStrategy = user.Strategy

	return &REST{*store}
}

// Get retrieves the item from etcd.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	// "~" means the currently authenticated user
	if name == "~" {
		user, ok := kapi.UserFrom(ctx)
		if !ok || user.GetName() == "" {
			return nil, kerrs.NewForbidden("user", "~", errors.New("Requests to ~ must be authenticated"))
		}
		name = user.GetName()
	}
	if ok, details := validation.ValidateUserName(name, false); !ok {
		return nil, fielderrors.NewFieldInvalid("metadata.name", name, details)
	}

	return r.Etcd.Get(ctx, name)
}
