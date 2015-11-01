package policybased

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
)

// TODO sort out resourceVersions.  Perhaps a hash of the object contents?

type VirtualStorage struct {
	PolicyStorage policyregistry.Registry

	CreateStrategy rest.RESTCreateStrategy
	UpdateStrategy rest.RESTUpdateStrategy
}

// NewVirtualStorage creates a new REST for policies.
func NewVirtualStorage(policyStorage policyregistry.Registry) roleregistry.Storage {
	return &VirtualStorage{policyStorage, roleregistry.LocalStrategy, roleregistry.LocalStrategy}
}

func (m *VirtualStorage) New() runtime.Object {
	return &authorizationapi.Role{}
}
func (m *VirtualStorage) NewList() runtime.Object {
	return &authorizationapi.RoleList{}
}

func (m *VirtualStorage) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	policyList, err := m.PolicyStorage.ListPolicies(ctx, labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	roleList := &authorizationapi.RoleList{}

	for _, policy := range policyList.Items {
		for _, role := range policy.Roles {
			if label.Matches(labels.Set(role.Labels)) && field.Matches(authorizationapi.RoleToSelectableFields(role)) {
				roleList.Items = append(roleList.Items, *role)
			}
		}
	}

	return roleList, nil
}

func (m *VirtualStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound("Role", name)
	}
	if err != nil {
		return nil, err
	}

	role, exists := policy.Roles[name]
	if !exists {
		return nil, kapierrors.NewNotFound("Role", name)
	}

	return role, nil
}

// Delete(ctx api.Context, name string) (runtime.Object, error)
func (m *VirtualStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound("Role", name)
	}
	if err != nil {
		return nil, err
	}

	if _, exists := policy.Roles[name]; !exists {
		return nil, kapierrors.NewNotFound("Role", name)
	}

	delete(policy.Roles, name)
	policy.LastModified = unversioned.Now()

	if err := m.PolicyStorage.UpdatePolicy(ctx, policy); err != nil {
		return nil, err
	}
	return &unversioned.Status{Status: unversioned.StatusSuccess}, nil
}

func (m *VirtualStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(m.CreateStrategy, ctx, obj); err != nil {
		return nil, err
	}

	role := obj.(*authorizationapi.Role)

	policy, err := m.EnsurePolicy(ctx)
	if err != nil {
		return nil, err
	}
	if _, exists := policy.Roles[role.Name]; exists {
		return nil, kapierrors.NewAlreadyExists("Role", role.Name)
	}

	role.ResourceVersion = policy.ResourceVersion
	policy.Roles[role.Name] = role
	policy.LastModified = unversioned.Now()

	if err := m.PolicyStorage.UpdatePolicy(ctx, policy); err != nil {
		return nil, err
	}

	return role, nil
}

func (m *VirtualStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	role, ok := obj.(*authorizationapi.Role)
	if !ok {
		return nil, false, kapierrors.NewBadRequest(fmt.Sprintf("obj is not a role: %#v", obj))
	}

	old, err := m.Get(ctx, role.Name)
	if err != nil {
		return nil, false, err
	}

	if err := rest.BeforeUpdate(m.UpdateStrategy, ctx, obj, old); err != nil {
		return nil, false, err
	}

	policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && kapierrors.IsNotFound(err) {
		return nil, false, kapierrors.NewNotFound("Role", role.Name)
	}
	if err != nil {
		return nil, false, err
	}

	if _, exists := policy.Roles[role.Name]; !exists {
		return nil, false, kapierrors.NewNotFound("Role", role.Name)
	}

	role.ResourceVersion = policy.ResourceVersion
	policy.Roles[role.Name] = role
	policy.LastModified = unversioned.Now()

	if err := m.PolicyStorage.UpdatePolicy(ctx, policy); err != nil {
		return nil, false, err
	}
	return role, false, nil
}

// EnsurePolicy returns the policy object for the specified namespace.  If one does not exist, it is created for you.  Permission to
// create, update, or delete roles in a namespace implies the ability to create a Policy object itself.
func (m *VirtualStorage) EnsurePolicy(ctx kapi.Context) (*authorizationapi.Policy, error) {
	policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}

		// if we have no policy, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policy = NewEmptyPolicy(kapi.NamespaceValue(ctx))
		if err := m.PolicyStorage.CreatePolicy(ctx, policy); err != nil {
			return nil, err
		}

		policy, err = m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName)
		if err != nil {
			return nil, err
		}

	}

	if policy.Roles == nil {
		policy.Roles = make(map[string]*authorizationapi.Role)
	}

	return policy, nil
}

func NewEmptyPolicy(namespace string) *authorizationapi.Policy {
	policy := &authorizationapi.Policy{}
	policy.Name = authorizationapi.PolicyName
	policy.Namespace = namespace
	policy.CreationTimestamp = unversioned.Now()
	policy.LastModified = unversioned.Now()
	policy.Roles = make(map[string]*authorizationapi.Role)

	return policy
}
