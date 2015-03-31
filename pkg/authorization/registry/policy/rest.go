package policy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for policies.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New creates a new Policy object
func (r *REST) New() runtime.Object {
	return &authorizationapi.Policy{}
}
func (r *REST) NewList() runtime.Object {
	return &authorizationapi.PolicyList{}
}

// List obtains a list of Policies that match selector.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	policies, err := r.registry.ListPolicies(ctx, label, field)
	if err != nil {
		return nil, err
	}
	return policies, err

}

// Get obtains the policy specified by its id.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	policy, err := r.registry.GetPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	return policy, err
}

// Delete asynchronously deletes the Policy specified by its id.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeletePolicy(ctx, id)
}

// Watch begins watching for new, changed, or deleted PolicyBindings.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.registry.WatchPolicies(ctx, label, field, resourceVersion)
}
