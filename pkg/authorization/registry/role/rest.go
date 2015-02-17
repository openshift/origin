package role

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
)

// TODO add get and list
// TODO prevent privilege escalation

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry policyregistry.Registry
}

// NewREST creates a new REST for policies.
func NewREST(registry policyregistry.Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New creates a new Role object
func (r *REST) New() runtime.Object {
	return &authorizationapi.Role{}
}

// Delete asynchronously deletes the Policy specified by its id.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	policy, err := r.EnsurePolicy(ctx)
	if err != nil {
		return nil, err
	}

	if !doesRoleExist(id, policy) {
		return nil, fmt.Errorf("role %v does not exist", id)
	}

	delete(policy.Roles, id)
	policy.LastModified = util.Now()

	return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.UpdatePolicy(ctx, policy)
}

// Create registers a given new Role inside the Policy instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	role, ok := obj.(*authorizationapi.Role)
	if !ok {
		return nil, fmt.Errorf("not a role: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &role.ObjectMeta) {
		return nil, kerrors.NewConflict("role", role.Namespace, fmt.Errorf("Role.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &role.ObjectMeta)
	if errs := validation.ValidateRole(role); len(errs) > 0 {
		return nil, kerrors.NewInvalid("role", role.Name, errs)
	}

	policy, err := r.EnsurePolicy(ctx)
	if err != nil {
		return nil, err
	}
	if doesRoleExist(role.Name, policy) {
		return nil, fmt.Errorf("role %v already exists", role.Name)
	}

	policy.Roles[role.Name] = *role
	policy.LastModified = util.Now()

	if err := r.registry.UpdatePolicy(ctx, policy); err != nil {
		return nil, err
	}
	return role, nil
}

// Update replaces a given Role inside the Policy instance with an existing instance in r.registry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	role, ok := obj.(*authorizationapi.Role)
	if !ok {
		return nil, false, fmt.Errorf("not a role: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &role.ObjectMeta) {
		return nil, false, kerrors.NewConflict("role", role.Namespace, fmt.Errorf("Role.Namespace does not match the provided context"))
	}

	if errs := validation.ValidateRole(role); len(errs) > 0 {
		return nil, false, kerrors.NewInvalid("role", role.Name, errs)
	}

	policy, err := r.EnsurePolicy(ctx)
	if err != nil {
		return nil, false, err
	}
	if !doesRoleExist(role.Name, policy) {
		return nil, false, fmt.Errorf("role %v does not exist", role.Name)
	}

	// set defaults
	role.CreationTimestamp = util.Now()

	policy.Roles[role.Name] = *role
	policy.LastModified = util.Now()

	if err := r.registry.UpdatePolicy(ctx, policy); err != nil {
		return nil, false, err
	}
	return role, false, nil
}

func doesRoleExist(name string, policy *authorizationapi.Policy) bool {
	_, exists := policy.Roles[name]

	return exists
}

// EnsurePolicy returns the policy object for the specified namespace.  If one does not exist, it is created for you.  Permission to
// create, update, or delete roles in a namespace implies the ability to create a Policy object itself.
func (r *REST) EnsurePolicy(ctx kapi.Context) (*authorizationapi.Policy, error) {
	policy, err := r.registry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, err
		}

		// if we have no policy, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policy = NewEmptyPolicy(kapi.NamespaceValue(ctx))
		if err := r.registry.CreatePolicy(ctx, policy); err != nil {
			return nil, err
		}

		policy, err = r.registry.GetPolicy(ctx, authorizationapi.PolicyName)
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
