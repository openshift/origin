package hostsubnet

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/sdn/api"
)

// Registry is an interface implemented by things that know how to store sdn objects.
type Registry interface {
	// ListSubnets obtains a list of subnets
	ListSubnets(ctx kapi.Context) (*api.HostSubnetList, error)
	// GetSubnet returns a specific subnet
	GetSubnet(ctx kapi.Context, name string) (*api.HostSubnet, error)
	// CreateSubnet creates a HostSubnet
	CreateSubnet(ctx kapi.Context, hs *api.HostSubnet) (*api.HostSubnet, error)
	// DeleteSubnet deletes a hostsubnet
	DeleteSubnet(ctx kapi.Context, name string) error
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

func (s *storage) ListSubnets(ctx kapi.Context) (*api.HostSubnetList, error) {
	obj, err := s.List(ctx, &kapi.ListOptions{})
	if err != nil {
		return nil, err
	}
	return obj.(*api.HostSubnetList), nil
}

func (s *storage) GetSubnet(ctx kapi.Context, name string) (*api.HostSubnet, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.HostSubnet), nil
}

func (s *storage) CreateSubnet(ctx kapi.Context, hs *api.HostSubnet) (*api.HostSubnet, error) {
	obj, err := s.Create(ctx, hs)
	if err != nil {
		return nil, err
	}
	return obj.(*api.HostSubnet), nil
}

func (s *storage) DeleteSubnet(ctx kapi.Context, name string) error {
	_, err := s.Delete(ctx, name, nil)
	if err != nil {
		return err
	}
	return nil
}
