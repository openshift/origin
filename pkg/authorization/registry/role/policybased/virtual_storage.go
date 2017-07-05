package policybased

import (
	"errors"
	"fmt"
	"sort"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	"k8s.io/kubernetes/pkg/client/retry"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

// TODO sort out resourceVersions.  Perhaps a hash of the object contents?

type VirtualStorage struct {
	PolicyStorage policyregistry.Registry

	RuleResolver       rulevalidation.AuthorizationRuleResolver
	CachedRuleResolver rulevalidation.AuthorizationRuleResolver

	CreateStrategy rest.RESTCreateStrategy
	UpdateStrategy rest.RESTUpdateStrategy
	Resource       schema.GroupResource
}

// NewVirtualStorage creates a new REST for policies.
func NewVirtualStorage(policyRegistry policyregistry.Registry, liveRuleResolver, cachedRuleResolver rulevalidation.AuthorizationRuleResolver) roleregistry.Storage {
	return &VirtualStorage{
		PolicyStorage: policyRegistry,

		RuleResolver:       liveRuleResolver,
		CachedRuleResolver: cachedRuleResolver,

		CreateStrategy: roleregistry.LocalStrategy,
		UpdateStrategy: roleregistry.LocalStrategy,
		Resource:       authorizationapi.Resource("role"),
	}
}

func (m *VirtualStorage) New() runtime.Object {
	return &authorizationapi.Role{}
}
func (m *VirtualStorage) NewList() runtime.Object {
	return &authorizationapi.RoleList{}
}

func (m *VirtualStorage) List(ctx apirequest.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	policyList, err := m.PolicyStorage.ListPolicies(ctx, &metainternal.ListOptions{})
	if err != nil {
		return nil, err
	}

	matcher := roleregistry.Matcher(oapi.InternalListOptionsToSelectors(options))

	roleList := &authorizationapi.RoleList{}
	for _, policy := range policyList.Items {
		for _, role := range policy.Roles {
			if matches, err := matcher.Matches(role); err == nil && matches {
				roleList.Items = append(roleList.Items, *role)
			}
		}
	}

	sort.Sort(byName(roleList.Items))
	return roleList, nil
}

func (m *VirtualStorage) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName, options)
	if kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound(m.Resource, name)
	}
	if err != nil {
		return nil, err
	}

	role, exists := policy.Roles[name]
	if !exists {
		return nil, kapierrors.NewNotFound(m.Resource, name)
	}

	return role, nil
}

func (m *VirtualStorage) Delete(ctx apirequest.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName, &metav1.GetOptions{})
		if kapierrors.IsNotFound(err) {
			return kapierrors.NewNotFound(m.Resource, name)
		}
		if err != nil {
			return err
		}

		if _, exists := policy.Roles[name]; !exists {
			return kapierrors.NewNotFound(m.Resource, name)
		}

		delete(policy.Roles, name)
		policy.LastModified = metav1.Now()

		return m.PolicyStorage.UpdatePolicy(ctx, policy)
	}); err != nil {
		return nil, false, err
	}

	return &metav1.Status{Status: metav1.StatusSuccess}, true, nil
}

func (m *VirtualStorage) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	return m.createRole(ctx, obj, rulevalidation.EscalationAllowed(ctx))
}

func (m *VirtualStorage) CreateRoleWithEscalation(ctx apirequest.Context, obj *authorizationapi.Role) (*authorizationapi.Role, error) {
	return m.createRole(ctx, obj, true)
}

func (m *VirtualStorage) createRole(ctx apirequest.Context, obj runtime.Object, allowEscalation bool) (*authorizationapi.Role, error) {
	// Copy object before passing to BeforeCreate, since it mutates
	objCopy, err := kapi.Scheme.DeepCopy(obj)
	if err != nil {
		return nil, err
	}
	obj = objCopy.(runtime.Object)

	if err := rest.BeforeCreate(m.CreateStrategy, ctx, obj); err != nil {
		return nil, err
	}

	role := obj.(*authorizationapi.Role)
	if !allowEscalation {
		if err := rulevalidation.ConfirmNoEscalation(ctx, m.Resource, role.Name, m.RuleResolver, m.CachedRuleResolver, authorizationinterfaces.NewLocalRoleAdapter(role)); err != nil {
			return nil, err
		}
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		policy, err := m.EnsurePolicy(ctx)
		if err != nil {
			return err
		}
		if _, exists := policy.Roles[role.Name]; exists {
			return kapierrors.NewAlreadyExists(m.Resource, role.Name)
		}

		role.ResourceVersion = policy.ResourceVersion
		policy.Roles[role.Name] = role
		policy.LastModified = metav1.Now()

		return m.PolicyStorage.UpdatePolicy(ctx, policy)
	}); err != nil {
		return nil, err
	}

	return role, nil
}

func (m *VirtualStorage) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return m.updateRole(ctx, name, objInfo, rulevalidation.EscalationAllowed(ctx))
}
func (m *VirtualStorage) UpdateRoleWithEscalation(ctx apirequest.Context, obj *authorizationapi.Role) (*authorizationapi.Role, bool, error) {
	return m.updateRole(ctx, obj.Name, rest.DefaultUpdatedObjectInfo(obj, kapi.Scheme), true)
}

func (m *VirtualStorage) updateRole(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, allowEscalation bool) (*authorizationapi.Role, bool, error) {
	var updatedRole *authorizationapi.Role
	var roleConflicted = false

	// Retry if the policy update hits a conflict
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName, &metav1.GetOptions{})
		if kapierrors.IsNotFound(err) {
			return kapierrors.NewNotFound(m.Resource, name)
		}
		if err != nil {
			return err
		}

		oldRole, exists := policy.Roles[name]
		if !exists {
			return kapierrors.NewNotFound(m.Resource, name)
		}

		obj, err := objInfo.UpdatedObject(ctx, oldRole)
		if err != nil {
			return err
		}

		role, ok := obj.(*authorizationapi.Role)
		if !ok {
			return kapierrors.NewBadRequest(fmt.Sprintf("obj is not a role: %#v", obj))
		}

		if len(role.ResourceVersion) == 0 && m.UpdateStrategy.AllowUnconditionalUpdate() {
			role.ResourceVersion = oldRole.ResourceVersion
		}

		if err := rest.BeforeUpdate(m.UpdateStrategy, ctx, obj, oldRole); err != nil {
			return err
		}

		if !allowEscalation {
			if err := rulevalidation.ConfirmNoEscalation(ctx, m.Resource, role.Name, m.RuleResolver, m.CachedRuleResolver, authorizationinterfaces.NewLocalRoleAdapter(role)); err != nil {
				return err
			}
		}

		// conflict detection
		if role.ResourceVersion != oldRole.ResourceVersion {
			// mark as a conflict err, but return an untyped error to escape the retry
			roleConflicted = true
			return errors.New(registry.OptimisticLockErrorMsg)
		}
		// non-mutating change
		if kapihelper.Semantic.DeepEqual(oldRole, role) {
			updatedRole = role
			return nil
		}

		role.ResourceVersion = policy.ResourceVersion
		policy.Roles[role.Name] = role
		policy.LastModified = metav1.Now()

		if err := m.PolicyStorage.UpdatePolicy(ctx, policy); err != nil {
			return err
		}
		updatedRole = role
		return nil
	}); err != nil {
		if roleConflicted {
			// construct the typed conflict error
			return nil, false, kapierrors.NewConflict(authorizationapi.Resource("name"), name, err)
		}
		return nil, false, err
	}

	return updatedRole, false, nil
}

// EnsurePolicy returns the policy object for the specified namespace.  If one does not exist, it is created for you.  Permission to
// create, update, or delete roles in a namespace implies the ability to create a Policy object itself.
func (m *VirtualStorage) EnsurePolicy(ctx apirequest.Context) (*authorizationapi.Policy, error) {
	policy, err := m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName, &metav1.GetOptions{})
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}

		// if we have no policy, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policy = NewEmptyPolicy(apirequest.NamespaceValue(ctx))
		if err := m.PolicyStorage.CreatePolicy(ctx, policy); err != nil {
			// Tolerate the policy having been created in the meantime
			if !kapierrors.IsAlreadyExists(err) {
				return nil, err
			}
		}

		policy, err = m.PolicyStorage.GetPolicy(ctx, authorizationapi.PolicyName, &metav1.GetOptions{})
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
	policy.CreationTimestamp = metav1.Now()
	policy.LastModified = policy.CreationTimestamp
	policy.Roles = make(map[string]*authorizationapi.Role)

	return policy
}

type byName []authorizationapi.Role

func (r byName) Len() int           { return len(r) }
func (r byName) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byName) Less(i, j int) bool { return r[i].Name < r[j].Name }
