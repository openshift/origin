package etcd

import (
	"errors"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
	"github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for users against etcd
type REST struct {
	registry.Store
}

// NewREST returns a RESTStorage object that will work against users
func NewREST(optsGetter restoptions.Getter) (*REST, error) {

	store := &registry.Store{
		NewFunc:     func() runtime.Object { return &api.User{} },
		NewListFunc: func() runtime.Object { return &api.UserList{} },
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.User).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) *generic.SelectionPredicate {
			return user.Matcher(label, field)
		},
		QualifiedResource: api.Resource("users"),

		CreateStrategy: user.Strategy,
		UpdateStrategy: user.Strategy,
	}

	if err := restoptions.ApplyOptions(optsGetter, store, false, storage.NoTriggerPublisher); err != nil {
		return nil, err
	}

	return &REST{*store}, nil
}

// Get retrieves the item from etcd.
func (r *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	// "~" means the currently authenticated user
	if name == "~" {
		user, ok := kapi.UserFrom(ctx)
		if !ok || user.GetName() == "" {
			return nil, kerrs.NewForbidden(api.Resource("user"), "~", errors.New("requests to ~ must be authenticated"))
		}
		name = user.GetName()

		// remove the known virtual groups from the list if they are present
		contextGroups := sets.NewString(user.GetGroups()...)
		contextGroups.Delete(bootstrappolicy.UnauthenticatedGroup, bootstrappolicy.AuthenticatedGroup)

		if reasons := validation.ValidateUserName(name, false); len(reasons) != 0 {
			// The user the authentication layer has identified cannot be a valid persisted user
			// Return an API representation of the virtual user
			return &api.User{ObjectMeta: kapi.ObjectMeta{Name: name}, Groups: contextGroups.List()}, nil
		}

		obj, err := r.Store.Get(ctx, name)
		if err == nil {
			return obj, nil
		}

		if !kerrs.IsNotFound(err) {
			return nil, err
		}

		return &api.User{ObjectMeta: kapi.ObjectMeta{Name: name}, Groups: contextGroups.List()}, nil
	}

	if reasons := validation.ValidateUserName(name, false); len(reasons) != 0 {
		return nil, field.Invalid(field.NewPath("metadata", "name"), name, strings.Join(reasons, ", "))
	}

	return r.Store.Get(ctx, name)
}
