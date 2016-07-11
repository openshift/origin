package identity

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/user/api"
)

// Registry is an interface implemented by things that know how to store Identity objects.
type Registry interface {
	// ListIdentities obtains a list of Identities having labels which match selector.
	ListIdentities(ctx kapi.Context, options *kapi.ListOptions) (*api.IdentityList, error)
	// GetIdentity returns a specific Identity
	GetIdentity(ctx kapi.Context, name string) (*api.Identity, error)
	// CreateIdentity creates a Identity
	CreateIdentity(ctx kapi.Context, Identity *api.Identity) (*api.Identity, error)
	// UpdateIdentity updates an existing Identity
	UpdateIdentity(ctx kapi.Context, Identity *api.Identity) (*api.Identity, error)
}

func identityName(provider, identity string) string {
	// TODO: normalize?
	return provider + ":" + identity
}

// Storage is an interface for a standard REST Storage backend
// TODO: move me somewhere common
type Storage interface {
	rest.Lister
	rest.Getter

	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
	Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error)
}

// storage puts strong typing around storage calls
type storage struct {
	Storage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s Storage) Registry {
	return &storage{s}
}

func (s *storage) ListIdentities(ctx kapi.Context, options *kapi.ListOptions) (*api.IdentityList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*api.IdentityList), nil
}

func (s *storage) GetIdentity(ctx kapi.Context, name string) (*api.Identity, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Identity), nil
}

func (s *storage) CreateIdentity(ctx kapi.Context, identity *api.Identity) (*api.Identity, error) {
	obj, err := s.Create(ctx, identity)
	if err != nil {
		return nil, err
	}
	return obj.(*api.Identity), nil
}

func (s *storage) UpdateIdentity(ctx kapi.Context, identity *api.Identity) (*api.Identity, error) {
	obj, _, err := s.Update(ctx, identity.Name, rest.DefaultUpdatedObjectInfo(identity, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*api.Identity), nil
}
