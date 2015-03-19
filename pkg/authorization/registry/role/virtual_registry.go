package role

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
)

// TODO sort out resourceVersions.  Perhaps a hash of the object contents?

type VirtualRegistry struct {
	policyRegistry policyregistry.Registry
}

// NewVirtualRegistry creates a new REST for policies.
func NewVirtualRegistry(policyRegistry policyregistry.Registry) Registry {
	return &VirtualRegistry{policyRegistry}
}

// TODO either add selector for fields ot eliminate the option
func (m *VirtualRegistry) ListRoles(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.RoleList, error) {
	policyList, err := m.policyRegistry.ListPolicies(ctx, labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	roleList := &authorizationapi.RoleList{}

	for _, policy := range policyList.Items {
		for _, role := range policy.Roles {
			if label.Matches(labels.Set(role.Labels)) {
				roleList.Items = append(roleList.Items, role)
			}
		}
	}

	return roleList, nil
}

func (m *VirtualRegistry) GetRole(ctx kapi.Context, name string) (*authorizationapi.Role, error) {
	policy, err := m.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
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

	return &role, nil
}

func (m *VirtualRegistry) DeleteRole(ctx kapi.Context, name string) error {
	policy, err := m.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && kapierrors.IsNotFound(err) {
		return kapierrors.NewNotFound("Role", name)
	}
	if err != nil {
		return err
	}

	if _, exists := policy.Roles[name]; !exists {
		return kapierrors.NewNotFound("Role", name)
	}

	delete(policy.Roles, name)
	policy.LastModified = util.Now()

	if err := m.policyRegistry.UpdatePolicy(ctx, policy); err != nil {
		return err
	}
	return nil
}

func (m *VirtualRegistry) CreateRole(ctx kapi.Context, role *authorizationapi.Role) error {
	policy, err := m.EnsurePolicy(ctx)
	if err != nil {
		return err
	}
	if _, exists := policy.Roles[role.Name]; exists {
		return kapierrors.NewAlreadyExists("Role", role.Name)
	}

	policy.Roles[role.Name] = *role
	policy.LastModified = util.Now()

	if err := m.policyRegistry.UpdatePolicy(ctx, policy); err != nil {
		return err
	}

	return nil
}

func (m *VirtualRegistry) UpdateRole(ctx kapi.Context, role *authorizationapi.Role) error {
	policy, err := m.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && kapierrors.IsNotFound(err) {
		return kapierrors.NewNotFound("Role", role.Name)
	}
	if err != nil {
		return err
	}

	if _, exists := policy.Roles[role.Name]; !exists {
		return kapierrors.NewNotFound("Role", role.Name)
	}

	policy.Roles[role.Name] = *role
	policy.LastModified = util.Now()

	if err := m.policyRegistry.UpdatePolicy(ctx, policy); err != nil {
		return err
	}
	return nil
}

// EnsurePolicy returns the policy object for the specified namespace.  If one does not exist, it is created for you.  Permission to
// create, update, or delete roles in a namespace implies the ability to create a Policy object itself.
func (m *VirtualRegistry) EnsurePolicy(ctx kapi.Context) (*authorizationapi.Policy, error) {
	policy, err := m.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}

		// if we have no policy, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policy = NewEmptyPolicy(kapi.NamespaceValue(ctx))
		if err := m.policyRegistry.CreatePolicy(ctx, policy); err != nil {
			return nil, err
		}

		policy, err = m.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
		if err != nil {
			return nil, err
		}

	}

	if policy.Roles == nil {
		policy.Roles = make(map[string]authorizationapi.Role)
	}

	return policy, nil
}

func NewEmptyPolicy(namespace string) *authorizationapi.Policy {
	policy := &authorizationapi.Policy{}
	policy.Name = authorizationapi.PolicyName
	policy.Namespace = namespace
	policy.CreationTimestamp = util.Now()
	policy.LastModified = util.Now()
	policy.Roles = make(map[string]authorizationapi.Role)

	return policy
}
