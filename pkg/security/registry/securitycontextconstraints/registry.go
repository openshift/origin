package securitycontextconstraints

import (
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

// Registry is an interface implemented by things that know how to store SecurityContextConstraints objects.
type Registry interface {
	// ListSecurityContextConstraints obtains a list of SecurityContextConstraints having labels which match selector.
	ListSecurityContextConstraints(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*securityapi.SecurityContextConstraintsList, error)
	// Watch for new/changed/deleted SecurityContextConstraints
	WatchSecurityContextConstraints(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error)
	// Get a specific SecurityContextConstraints
	GetSecurityContextConstraint(ctx genericapirequest.Context, name string) (*securityapi.SecurityContextConstraints, error)
	// Create a SecurityContextConstraints based on a specification.
	CreateSecurityContextConstraint(ctx genericapirequest.Context, scc *securityapi.SecurityContextConstraints) error
	// Update an existing SecurityContextConstraints
	UpdateSecurityContextConstraint(ctx genericapirequest.Context, scc *securityapi.SecurityContextConstraints) error
	// Delete an existing SecurityContextConstraints
	DeleteSecurityContextConstraint(ctx genericapirequest.Context, name string) error
}

// storage puts strong typing around storage calls
type storage struct {
	rest.StandardStorage
}

// NewRegistry returns a new Registry interface for the given Storage. Any mismatched
// types will panic.
func NewRegistry(s rest.StandardStorage) Registry {
	return &storage{s}
}

func (s *storage) ListSecurityContextConstraints(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (*securityapi.SecurityContextConstraintsList, error) {
	obj, err := s.List(ctx, options)
	if err != nil {
		return nil, err
	}
	return obj.(*securityapi.SecurityContextConstraintsList), nil
}

func (s *storage) WatchSecurityContextConstraints(ctx genericapirequest.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return s.Watch(ctx, options)
}

func (s *storage) GetSecurityContextConstraint(ctx genericapirequest.Context, name string) (*securityapi.SecurityContextConstraints, error) {
	obj, err := s.Get(ctx, name, nil)
	if err != nil {
		return nil, err
	}
	return obj.(*securityapi.SecurityContextConstraints), nil
}

func (s *storage) CreateSecurityContextConstraint(ctx genericapirequest.Context, scc *securityapi.SecurityContextConstraints) error {
	_, err := s.Create(ctx, scc, false)
	return err
}

func (s *storage) UpdateSecurityContextConstraint(ctx genericapirequest.Context, scc *securityapi.SecurityContextConstraints) error {
	_, _, err := s.Update(ctx, scc.Name, rest.DefaultUpdatedObjectInfo(scc, api.Scheme))
	return err
}

func (s *storage) DeleteSecurityContextConstraint(ctx genericapirequest.Context, name string) error {
	_, _, err := s.Delete(ctx, name, nil)
	return err
}
