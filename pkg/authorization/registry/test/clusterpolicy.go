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

var resourceVersion = 1

type ClusterPolicyRegistry struct {
	// ClusterPolicies is a of namespace->name->ClusterPolicy
	clusterPolicies map[string]map[string]authorizationapi.ClusterPolicy
	Err             error
}

func NewClusterPolicyRegistry(policies []authorizationapi.ClusterPolicy, err error) *ClusterPolicyRegistry {
	policyMap := make(map[string]map[string]authorizationapi.ClusterPolicy)

	for _, policy := range policies {
		addClusterPolicy(policyMap, policy)
	}

	return &ClusterPolicyRegistry{policyMap, err}
}

func (r *ClusterPolicyRegistry) List(label labels.Selector) ([]*authorizationapi.ClusterPolicy, error) {
	list, err := r.ListClusterPolicies(apirequest.NewContext(), &metainternal.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}
	var items []*authorizationapi.ClusterPolicy
	for i := range list.Items {
		items = append(items, &list.Items[i])
	}
	return items, nil
}

func (r *ClusterPolicyRegistry) Get(name string) (*authorizationapi.ClusterPolicy, error) {
	return r.GetClusterPolicy(apirequest.NewContext(), name, &metav1.GetOptions{})
}

// ListClusterPolicies obtains list of ListClusterPolicy that match a selector.
func (r *ClusterPolicyRegistry) ListClusterPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	list := make([]authorizationapi.ClusterPolicy, 0)

	if namespace == metav1.NamespaceAll {
		for _, curr := range r.clusterPolicies {
			for _, policy := range curr {
				list = append(list, policy)
			}
		}

	} else {
		if namespacedClusterPolicies, ok := r.clusterPolicies[namespace]; ok {
			for _, curr := range namespacedClusterPolicies {
				list = append(list, curr)
			}
		}
	}

	return &authorizationapi.ClusterPolicyList{
			Items: list,
		},
		nil
}

// GetClusterPolicy retrieves a specific policy.
func (r *ClusterPolicyRegistry) GetClusterPolicy(ctx apirequest.Context, id string, options *metav1.GetOptions) (*authorizationapi.ClusterPolicy, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return nil, errors.New("invalid request.  Namespace parameter disallowed.")
	}

	if namespacedClusterPolicies, ok := r.clusterPolicies[namespace]; ok {
		if policy, ok := namespacedClusterPolicies[id]; ok {
			return &policy, nil
		}
	}

	return nil, kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicy"), id)
}

// CreateClusterPolicy creates a new policy.
func (r *ClusterPolicyRegistry) CreateClusterPolicy(ctx apirequest.Context, policy *authorizationapi.ClusterPolicy) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}
	if existing, _ := r.GetClusterPolicy(ctx, policy.Name, &metav1.GetOptions{}); existing != nil {
		return kapierrors.NewAlreadyExists(authorizationapi.Resource("ClusterPolicy"), policy.Name)
	}

	addClusterPolicy(r.clusterPolicies, *policy)

	return nil
}

// UpdateClusterPolicy updates a policy.
func (r *ClusterPolicyRegistry) UpdateClusterPolicy(ctx apirequest.Context, policy *authorizationapi.ClusterPolicy) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}
	if existing, _ := r.GetClusterPolicy(ctx, policy.Name, &metav1.GetOptions{}); existing == nil {
		return kapierrors.NewNotFound(authorizationapi.Resource("clusterpolicy"), policy.Name)
	}

	addClusterPolicy(r.clusterPolicies, *policy)

	return nil
}

// DeleteClusterPolicy deletes a policy.
func (r *ClusterPolicyRegistry) DeleteClusterPolicy(ctx apirequest.Context, id string) error {
	if r.Err != nil {
		return r.Err
	}

	namespace := apirequest.NamespaceValue(ctx)
	if len(namespace) != 0 {
		return errors.New("invalid request.  Namespace parameter disallowed.")
	}

	namespacedClusterPolicies, ok := r.clusterPolicies[namespace]
	if ok {
		delete(namespacedClusterPolicies, id)
	}

	return nil
}

func (r *ClusterPolicyRegistry) WatchClusterPolicies(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return nil, errors.New("unsupported action for test registry")
}

func addClusterPolicy(policies map[string]map[string]authorizationapi.ClusterPolicy, policy authorizationapi.ClusterPolicy) {
	resourceVersion += 1
	policy.ResourceVersion = fmt.Sprintf("%d", resourceVersion)

	namespacedClusterPolicies, ok := policies[policy.Namespace]
	if !ok {
		namespacedClusterPolicies = make(map[string]authorizationapi.ClusterPolicy)
		policies[policy.Namespace] = namespacedClusterPolicies
	}

	namespacedClusterPolicies[policy.Name] = policy
}
