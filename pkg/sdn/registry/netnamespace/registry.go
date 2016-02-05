package netnamespace

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/sdn/api"
)

// Registry is an interface implemented by things that know how to store sdn objects.
type Registry interface {
	// ListNetNamespaces obtains a list of NetNamespaces
	ListNetNamespaces(ctx kapi.Context) (*api.NetNamespaceList, error)
	// GetNetNamespace returns a specific NetNamespace
	GetNetNamespace(ctx kapi.Context, name string) (*api.NetNamespace, error)
	// CreateNetNamespace creates a NetNamespace
	CreateNetNamespace(ctx kapi.Context, nn *api.NetNamespace) (*api.NetNamespace, error)
	// DeleteNetNamespace deletes a netnamespace
	DeleteNetNamespace(ctx kapi.Context, name string) error
}

// Storage is an interface for a standard REST Storage backend
// TODO: move me somewhere common
type Storage interface {
	rest.Lister
	rest.Getter

	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
	Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error)
	Delete(ctx kapi.Context, name string, opts *kapi.DeleteOptions) (runtime.Object, error)
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

func (s *storage) ListNetNamespaces(ctx kapi.Context) (*api.NetNamespaceList, error) {
	obj, err := s.List(ctx, &kapi.ListOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*api.NetNamespaceList), nil
}

func (s *storage) GetNetNamespace(ctx kapi.Context, name string) (*api.NetNamespace, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.NetNamespace), nil
}

func (s *storage) CreateNetNamespace(ctx kapi.Context, nn *api.NetNamespace) (*api.NetNamespace, error) {
	obj, err := s.Create(ctx, nn)
	if err != nil {
		return nil, err
	}
	return obj.(*api.NetNamespace), nil
}

func (s *storage) DeleteNetNamespace(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
