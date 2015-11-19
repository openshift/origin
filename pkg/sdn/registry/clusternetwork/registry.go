package clusternetwork

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/sdn/api"
)

// Registry is an interface implemented by things that know how to store sdn's ClusterNetwork objects.
type Registry interface {
	// GetClusterNetwork returns a specific network
	GetClusterNetwork(ctx kapi.Context, name string) (*api.ClusterNetwork, error)
	// CreateClusterNetwork creates a cluster network
	CreateClusterNetwork(ctx kapi.Context, hs *api.ClusterNetwork) (*api.ClusterNetwork, error)
}

// Storage is an interface for a standard REST Storage backend
// TODO: move me somewhere common
type Storage interface {
	rest.Getter

	Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error)
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

func (s *storage) GetClusterNetwork(ctx kapi.Context, name string) (*api.ClusterNetwork, error) {
	obj, err := s.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ClusterNetwork), nil
}

func (s *storage) CreateClusterNetwork(ctx kapi.Context, hs *api.ClusterNetwork) (*api.ClusterNetwork, error) {
	obj, err := s.Create(ctx, hs)
	if err != nil {
		return nil, err
	}
	return obj.(*api.ClusterNetwork), nil
}
