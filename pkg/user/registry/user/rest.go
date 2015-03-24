package user

import (
	"errors"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST returns a new REST.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New returns a new UserIdentityMapping for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.User{}
}

// Get retrieves an UserIdentityMapping by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	// "~" means the currently authenticated user
	if id == "~" {
		user, ok := kapi.UserFrom(ctx)
		if !ok || user.GetName() == "" {
			return nil, kerrs.NewForbidden("user", "~", errors.New("Requests to ~ must be authenticated"))
		}
		id = user.GetName()
	}
	if ok, details := validation.ValidateUserName(id, false); !ok {
		return nil, fielderrors.NewFieldInvalid("metadata.name", id, details)
	}
	return s.registry.GetUser(id)
}
