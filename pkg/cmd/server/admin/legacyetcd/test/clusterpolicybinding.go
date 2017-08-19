package test

import (
	"errors"
	"fmt"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

type ClusterPolicyBindingRegistry struct {
	// clusterPolicyBindings is a of namespace->name->ClusterPolicyBinding
	clusterPolicyBindings map[string]map[string]authorizationapi.ClusterPolicyBinding
	Err                   error
}

func NewClusterPolicyBindingRegistry(bindings []authorizationapi.ClusterPolicyBinding, err error) *ClusterPolicyBindingRegistry {
	bindingMap := make(map[string]map[string]authorizationapi.ClusterPolicyBinding)

	for _, binding := range bindings {
		addClusterPolicyBinding(bindingMap, binding)
	}

	return &ClusterPolicyBindingRegistry{bindingMap, err}
}

func (r *ClusterPolicyBindingRegistry) List(label labels.Selector) ([]*authorizationapi.ClusterPolicyBinding, error) {
	list, err := r.ListClusterPolicyBindings(apirequest.NewContext(), &metainternal.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}
	var items []*authorizationapi.ClusterPolicyBinding
	for i := range list.Items {
		items = append(items, &list.Items[i])
	}
	return items, nil
}
func (r *ClusterPolicyBindingRegistry) Get(name string) (*authorizationapi.ClusterPolicyBinding, error) {
	return r.GetClusterPolicyBinding(apirequest.NewContext(), name, &metav1.GetOptions{})
}

// ListClusterPolicyBindings obtains list of clusterPolicyBindings that match a selector.
func (r *ClusterPolicyBindingRegistry) ListClusterPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	list := make([]authorizationapi.ClusterPolicyBinding, 0)

	if namespace == metav1.NamespaceAll {
		for _, curr := range r.clusterPolicyBindings {
			for _, binding := range curr {
				list = append(list, binding)
			}
		}

	} else {
		if namespacedBindings, ok := r.clusterPolicyBindings[namespace]; ok {
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
func (r *ClusterPolicyBindingRegistry) GetClusterPolicyBinding(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.ClusterPolicyBinding, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return nil, errors.New("invalid request.  Namespace parameter disallowed.")
	}

	if namespacedBindings, ok := r.clusterPolicyBindings[namespace]; ok {
		if binding, ok := namespacedBindings[id]; ok {
			return &binding, nil
		}
	}

	return nil, kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicybinding"), id)
}

// CreateClusterPolicyBinding creates a new policyBinding.
func (r *ClusterPolicyBindingRegistry) CreateClusterPolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}
	if existing, _ := r.GetClusterPolicyBinding(ctx, policyBinding.Name, &metav1.GetOptions{}); existing != nil {
		return kapierrors.NewAlreadyExists(authorizationapi.Resource("clusterpolicybinding"), policyBinding.Name)
	}

	addClusterPolicyBinding(r.clusterPolicyBindings, *policyBinding)

	return nil
}

// UpdateClusterPolicyBinding updates a policyBinding.
func (r *ClusterPolicyBindingRegistry) UpdateClusterPolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.ClusterPolicyBinding) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}
	if existing, _ := r.GetClusterPolicyBinding(ctx, policyBinding.Name, &metav1.GetOptions{}); existing == nil {
		return kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicybinding"), policyBinding.Name)
	}

	addClusterPolicyBinding(r.clusterPolicyBindings, *policyBinding)

	return nil
}

// DeleteClusterPolicyBinding deletes a policyBinding.
func (r *ClusterPolicyBindingRegistry) DeleteClusterPolicyBinding(ctx apirequest.Context, id string) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}

	namespacedBindings, ok := r.clusterPolicyBindings[namespace]
	if ok {
		delete(namespacedBindings, id)
	}

	return nil
}

func (r *ClusterPolicyBindingRegistry) WatchClusterPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
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
