package etcd

import (
	"errors"
	"strings"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	"github.com/openshift/origin/pkg/user/apis/user/validation"
	"github.com/openshift/origin/pkg/user/registry/user"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// rest implements a RESTStorage for users against etcd
type REST struct {
	*registry.Store
}

var _ rest.StandardStorage = &REST{}

// NewREST returns a RESTStorage object that will work against users
func NewREST(optsGetter restoptions.Getter) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &userapi.User{} },
		NewListFunc:              func() runtime.Object { return &userapi.UserList{} },
		DefaultQualifiedResource: userapi.Resource("users"),

		CreateStrategy: user.Strategy,
		UpdateStrategy: user.Strategy,
		DeleteStrategy: user.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{store}, nil
}

// Get retrieves the item from etcd.
func (r *REST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	// "~" means the currently authenticated user
	if name == "~" {
		user, ok := apirequest.UserFrom(ctx)
		if !ok || user.GetName() == "" {
			return nil, kerrs.NewForbidden(userapi.Resource("user"), "~", errors.New("requests to ~ must be authenticated"))
		}
		name = user.GetName()

		// remove the known virtual groups from the list if they are present
		contextGroups := sets.NewString(user.GetGroups()...)
		contextGroups.Delete(bootstrappolicy.UnauthenticatedGroup, bootstrappolicy.AuthenticatedGroup)

		// build a virtual user object using the context data
		virtualUser := &userapi.User{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(user.GetUID())}, Groups: contextGroups.List()}

		if reasons := validation.ValidateUserName(name, false); len(reasons) != 0 {
			// The user the authentication layer has identified cannot be a valid persisted user
			// Return an API representation of the virtual user
			return virtualUser, nil
		}

		// see if the context user exists in storage
		obj, err := r.Store.Get(ctx, name, options)

		// valid persisted user
		if err == nil {
			return obj, nil
		}

		// server is broken
		if !kerrs.IsNotFound(err) {
			return nil, kerrs.NewInternalError(err)
		}

		// impersonation, remote token authn, etc
		return virtualUser, nil
	}

	// do not bother looking up users that cannot be persisted
	if reasons := validation.ValidateUserName(name, false); len(reasons) != 0 {
		return nil, field.Invalid(field.NewPath("metadata", "name"), name, strings.Join(reasons, ", "))
	}

	return r.Store.Get(ctx, name, options)
}
