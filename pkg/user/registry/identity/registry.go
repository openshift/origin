package identity

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// Registry is an interface implemented by things that know how to store Identity objects.
type Registry interface {
	// ListIdentities obtains a list of Identities having labels which match selector.
	ListIdentities(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.IdentityList, error)
	// GetIdentity returns a specific Identity
	GetIdentity(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.Identity, error)
	// CreateIdentity creates a Identity
	CreateIdentity(ctx apirequest.Context, Identity *userapi.Identity) (*userapi.Identity, error)
	// UpdateIdentity updates an existing Identity
	UpdateIdentity(ctx apirequest.Context, Identity *userapi.Identity) (*userapi.Identity, error)
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

	Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error)
	Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error)
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

func (s *storage) ListIdentities(ctx apirequest.Context, options *metainternal.ListOptions) (*userapi.IdentityList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.IdentityList), nil
}

func (s *storage) GetIdentity(ctx apirequest.Context, name string, options *metav1.GetOptions) (*userapi.Identity, error) {
	obj, err := s.Get(ctx, name, options)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.Identity), nil
}

func (s *storage) CreateIdentity(ctx apirequest.Context, identity *userapi.Identity) (*userapi.Identity, error) {
	obj, err := s.Create(ctx, identity, false)
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.Identity), nil
}

func (s *storage) UpdateIdentity(ctx apirequest.Context, identity *userapi.Identity) (*userapi.Identity, error) {
	obj, _, err := s.Update(ctx, identity.Name, rest.DefaultUpdatedObjectInfo(identity, kapi.Scheme))
	if err != nil {
		return nil, err
	}
	return obj.(*userapi.Identity), nil
}
