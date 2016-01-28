package test

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type ClusterPolicyBindingRegistry struct {
	// ClusterPolicyBindings is a of namespace->name->ClusterPolicyBinding
	ClusterPolicyBindings map[string]map[string]authorizationapi.ClusterPolicyBinding
	Err                   error
}

func NewClusterPolicyBindingRegistry(bindings []authorizationapi.ClusterPolicyBinding, err error) *ClusterPolicyBindingRegistry {
	bindingMap := make(map[string]map[string]authorizationapi.ClusterPolicyBinding)

	for _, binding := range bindings {
		addClusterPolicyBinding(bindingMap, binding)
	}

	return &ClusterPolicyBindingRegistry{bindingMap, err}
}

// ListClusterPolicyBindings obtains list of clusterPolicyBindings that match a selector.
func (r *ClusterPolicyBindingRegistry) ListClusterPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	list := make([]authorizationapi.ClusterPolicyBinding, 0)

	if namespace == kapi.NamespaceAll {
		for _, curr := range r.ClusterPolicyBindings {
			for _, binding := range curr {
				list = append(list, binding)
			}
		}

	} else {
		if namespacedBindings, ok := r.ClusterPolicyBindings[namespace]; ok {
			for _, curr := range namespacedBindings {
				list = append(list, curr)
			}
		}
	}

	return &authorizationapi.ClusterPolicyBindingList{
			Items: list,
		},
		nil
}

// GetClusterPolicyBinding retrieves a specific policyBinding.
func (r *ClusterPolicyBindingRegistry) GetClusterPolicyBinding(ctx kapi.Context, id string) (*authorizationapi.ClusterPolicyBinding, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return nil, errors.New("invalid request.  Namespace parameter disallowed.")
	}

	if namespacedBindings, ok := r.ClusterPolicyBindings[namespace]; ok {
		if binding, ok := namespacedBindings[id]; ok {
			return &binding, nil
		}
	}

	return nil, kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicybinding"), id)
}

// CreateClusterPolicyBinding creates a new policyBinding.
func (r *ClusterPolicyBindingRegistry) CreateClusterPolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}
	if existing, _ := r.GetClusterPolicyBinding(ctx, policyBinding.Name); existing != nil {
		return kapierrors.NewAlreadyExists(authorizationapi.Resource("clusterpolicybinding"), policyBinding.Name)
	}

	addClusterPolicyBinding(r.ClusterPolicyBindings, *policyBinding)

	return nil
}

// UpdateClusterPolicyBinding updates a policyBinding.
func (r *ClusterPolicyBindingRegistry) UpdateClusterPolicyBinding(ctx kapi.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}
	if existing, _ := r.GetClusterPolicyBinding(ctx, policyBinding.Name); existing == nil {
		return kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicybinding"), policyBinding.Name)
	}

	addClusterPolicyBinding(r.ClusterPolicyBindings, *policyBinding)

	return nil
}

// DeleteClusterPolicyBinding deletes a policyBinding.
func (r *ClusterPolicyBindingRegistry) DeleteClusterPolicyBinding(ctx kapi.Context, id string) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := kapi.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}

	namespacedBindings, ok := r.ClusterPolicyBindings[namespace]
	if ok {
		delete(namespacedBindings, id)
	}

	return nil
}

func (r *ClusterPolicyBindingRegistry) WatchClusterPolicyBindings(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return nil, errors.New("unsupported action for test registry")
}

func addClusterPolicyBinding(bindings map[string]map[string]authorizationapi.ClusterPolicyBinding, binding authorizationapi.ClusterPolicyBinding) {
	resourceVersion += 1
	binding.ResourceVersion = fmt.Sprintf("%d", resourceVersion)

	namespacedBindings, ok := bindings[binding.Namespace]
	if !ok {
		namespacedBindings = make(map[string]authorizationapi.ClusterPolicyBinding)
		bindings[binding.Namespace] = namespacedBindings
	}

	namespacedBindings[binding.Name] = binding
}
