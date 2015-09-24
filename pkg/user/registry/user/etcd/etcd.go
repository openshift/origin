package etcd

import (
	"errors"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
	"github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/util"
)

// rest implements a RESTStorage for users against etcd
type REST struct {
	etcdgeneric.Etcd
}

const EtcdPrefix = "/users"

// NewREST returns a RESTStorage object that will work against users
func NewREST(s storage.Interface) *REST {
	store := &etcdgeneric.Etcd{
		NewFunc:     func() runtime.Object { return &api.User{} },
		NewListFunc: func() runtime.Object { return &api.UserList{} },
		KeyRootFunc: func(ctx kapi.Context) string {
			return EtcdPrefix
		},
		KeyFunc: func(ctx kapi.Context, name string) (string, error) {
			return util.NoNamespaceKeyFunc(ctx, EtcdPrefix, name)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.User).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return user.MatchUser(label, field)
		},
		EndpointName: "users",

		Storage: s,
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
			return nil, kerrs.NewForbidden("user", "~", errors.New("requests to ~ must be authenticated"))
		}
		name = user.GetName()

		// remove the known virtual groups from the list if they are present
		contextGroups := sets.NewString(user.GetGroups()...)
		contextGroups.Delete(bootstrappolicy.UnauthenticatedGroup, bootstrappolicy.AuthenticatedGroup)

		if ok, _ := validation.ValidateUserName(name, false); !ok {
			// The user the authentication layer has identified cannot possibly be a persisted user
			// Return an API representation of the virtual user
			return &api.User{ObjectMeta: kapi.ObjectMeta{Name: name}, Groups: contextGroups.List()}, nil
		}

		obj, err := r.Etcd.Get(ctx, name)
		if err == nil {
			return obj, nil
		}

		if !kerrs.IsNotFound(err) {
			return nil, err
		}

		return &api.User{ObjectMeta: kapi.ObjectMeta{Name: name}, Groups: contextGroups.List()}, nil
	}

	if ok, details := validation.ValidateUserName(name, false); !ok {
		return nil, fielderrors.NewFieldInvalid("metadata.name", name, details)
	}

	return r.Etcd.Get(ctx, name)
}
