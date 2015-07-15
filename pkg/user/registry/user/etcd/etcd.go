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
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

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
func NewREST(h tools.EtcdHelper) *REST {
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
			return nil, kerrs.NewForbidden("user", "~", errors.New("requests to ~ must be authenticated"))
		}
		name = user.GetName()

		// remove the known virtual groups from the list if they are present
		contextGroups := kutil.NewStringSet(user.GetGroups()...)
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
