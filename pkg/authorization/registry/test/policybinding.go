package test

import (
	"errors"
	"fmt"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	"github.com/openshift/origin/pkg/client"
)

type PolicyBindingRegistry struct {
	// policyBindings is a of namespace->name->PolicyBinding
	policyBindings map[string]map[string]authorizationapi.PolicyBinding
	Err            error
}

func NewPolicyBindingRegistry(bindings []authorizationapi.PolicyBinding, err error) *PolicyBindingRegistry {
	bindingMap := make(map[string]map[string]authorizationapi.PolicyBinding)

	for _, binding := range bindings {
		addPolicyBinding(bindingMap, binding)
	}

	return &PolicyBindingRegistry{bindingMap, err}
}

func (r *PolicyBindingRegistry) PolicyBindings(namespace string) client.PolicyBindingLister {
	return policyBindingLister{registry: r, namespace: namespace}
}

type policyBindingLister struct {
	registry  policybindingregistry.Registry
	namespace string
}

func (s policyBindingLister) List(options metav1.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	optint := metainternal.ListOptions{}
	if err := metainternal.Convert_v1_ListOptions_To_internalversion_ListOptions(&options, &optint, nil); err != nil {
		return nil, err
	}
	return s.registry.ListPolicyBindings(apirequest.WithNamespace(apirequest.NewContext(), s.namespace), &optint)
}

func (s policyBindingLister) Get(name string, options metav1.GetOptions) (*authorizationapi.PolicyBinding, error) {
	return s.registry.GetPolicyBinding(apirequest.WithNamespace(apirequest.NewContext(), s.namespace), name, &options)
}

// ListPolicyBindings obtains a list of policyBinding that match a selector.
func (r *PolicyBindingRegistry) ListPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	list := make([]authorizationapi.PolicyBinding, 0)

	if namespace == metav1.NamespaceAll {
		for _, curr := range r.policyBindings {
			for _, binding := range curr {
				list = append(list, binding)
			}
		}

	} else {
		if namespacedBindings, ok := r.policyBindings[namespace]; ok {
			for _, curr := range namespacedBindings {
				list = append(list, curr)
			}
		}
	}

	return &authorizationapi.PolicyBindingList{
			Items: list,
		},
		nil
}

// GetPolicyBinding retrieves a specific policyBinding.
func (r *PolicyBindingRegistry) GetPolicyBinding(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.PolicyBinding, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return nil, errors.New("invalid request.  Namespace parameter required.")
	}

	if namespacedBindings, ok := r.policyBindings[namespace]; ok {
		if binding, ok := namespacedBindings[id]; ok {
			return &binding, nil
		}
	}

	return nil, kapierrors.NewNotFound(authorizationapi.Resource("policybinding"), id)
}

// CreatePolicyBinding creates a new policyBinding.
func (r *PolicyBindingRegistry) CreatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return errors.New("invalid request.  Namespace parameter required.")
	}
	if existing, _ := r.GetPolicyBinding(ctx, policyBinding.Name, &metav1.GetOptions{}); existing != nil {
		return fmt.Errorf("PolicyBinding %v::%v already exists", namespace, policyBinding.Name)
	}

	addPolicyBinding(r.policyBindings, *policyBinding)

	return nil
}

// UpdatePolicyBinding updates a policyBinding.
func (r *PolicyBindingRegistry) UpdatePolicyBinding(ctx apirequest.Context, policyBinding *authorizationapi.PolicyBinding) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return errors.New("invalid request.  Namespace parameter required.")
	}
	if existing, _ := r.GetPolicyBinding(ctx, policyBinding.Name, &metav1.GetOptions{}); existing == nil {
		return kapierrors.NewNotFound(authorizationapi.Resource("policybinding"), policyBinding.Name)
	}

	addPolicyBinding(r.policyBindings, *policyBinding)

	return nil
}

// DeletePolicyBinding deletes a policyBinding.
func (r *PolicyBindingRegistry) DeletePolicyBinding(ctx apirequest.Context, id string) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) == 0 {
		return errors.New("invalid request.  Namespace parameter required.")
	}

	namespacedBindings, ok := r.policyBindings[namespace]
	if ok {
		delete(namespacedBindings, id)
	}

	return nil
}

func (r *PolicyBindingRegistry) WatchPolicyBindings(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return nil, errors.New("unsupported action for test registry")
}

func addPolicyBinding(bindings map[string]map[string]authorizationapi.PolicyBinding, binding authorizationapi.PolicyBinding) {
	resourceVersion += 1
	binding.ResourceVersion = fmt.Sprintf("%d", resourceVersion)

	namespacedBindings, ok := bindings[binding.Namespace]
	if !ok {
		namespacedBindings = make(map[string]authorizationapi.PolicyBinding)
		bindings[binding.Namespace] = namespacedBindings
	}

	namespacedBindings[binding.Name] = binding
}
