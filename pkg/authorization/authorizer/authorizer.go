package authorizer

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapiserver "github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

type Authorizer interface {
	Authorize(a AuthorizationAttributes) (allowed bool, reason string, err error)
	GetAllowedSubjects(attributes AuthorizationAttributes) ([]string, []string, error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (AuthorizationAttributes, error)
}

type AuthorizationAttributes interface {
	GetUserInfo() user.Info
	GetVerb() string
	GetResource() string
	GetNamespace() string
	GetResourceName() string
	// GetRequestAttributes is of type interface{} because different verbs and different Authorizer/AuthorizationAttributeBuilder pairs may have different contract requirements
	GetRequestAttributes() interface{}
}

type openshiftAuthorizer struct {
	masterAuthorizationNamespace string
	policyRegistry               policyregistry.Registry
	policyBindingRegistry        policybindingregistry.Registry
}

func NewAuthorizer(masterAuthorizationNamespace string, policyRuleBindingRegistry policyregistry.Registry, policyBindingRegistry policybindingregistry.Registry) Authorizer {
	return &openshiftAuthorizer{masterAuthorizationNamespace, policyRuleBindingRegistry, policyBindingRegistry}
}

type DefaultAuthorizationAttributes struct {
	User              user.Info
	Verb              string
	Resource          string
	ResourceName      string
	Namespace         string
	RequestAttributes interface{}
}

type openshiftAuthorizationAttributeBuilder struct {
	contextMapper kapi.RequestContextMapper
	infoResolver  *kapiserver.APIRequestInfoResolver
}

func NewAuthorizationAttributeBuilder(contextMapper kapi.RequestContextMapper, infoResolver *kapiserver.APIRequestInfoResolver) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{contextMapper, infoResolver}
}

func doesApplyToUser(ruleUsers, ruleGroups []string, user user.Info) bool {
	if contains(ruleUsers, user.GetName()) {
		return true
	}

	for _, currGroup := range user.GetGroups() {
		if contains(ruleGroups, currGroup) {
			return true
		}
	}

	return false
}
func contains(list []string, token string) bool {
	for _, curr := range list {
		if curr == token {
			return true
		}
	}
	return false
}

// getPolicy provides a point for easy caching
func (a *openshiftAuthorizer) getPolicy(namespace string) (*authorizationapi.Policy, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	policy, err := a.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, err
	}

	return policy, nil
}

// getPolicyBindings provides a point for easy caching
func (a *openshiftAuthorizer) getPolicyBindings(namespace string) ([]authorizationapi.PolicyBinding, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	policyBindingList, err := a.policyBindingRegistry.ListPolicyBindings(ctx, klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	return policyBindingList.Items, nil
}

// getRoleBindings provides a point for easy caching
func (a *openshiftAuthorizer) getRoleBindings(namespace string) ([]authorizationapi.RoleBinding, error) {
	policyBindings, err := a.getPolicyBindings(namespace)
	if err != nil {
		return nil, err
	}

	ret := make([]authorizationapi.RoleBinding, 0, len(policyBindings))
	for _, policyBinding := range policyBindings {
		for _, value := range policyBinding.RoleBindings {
			ret = append(ret, value)
		}
	}

	return ret, nil
}

func (a *openshiftAuthorizer) getRole(roleBinding authorizationapi.RoleBinding) (*authorizationapi.Role, error) {
	roleNamespace := roleBinding.RoleRef.Namespace
	roleName := roleBinding.RoleRef.Name

	rolePolicy, err := a.getPolicy(roleNamespace)
	if err != nil {
		return nil, err
	}

	role, exists := rolePolicy.Roles[roleName]
	if !exists {
		return nil, fmt.Errorf("role %#v not found", roleBinding.RoleRef)
	}

	return &role, nil
}

// getEffectivePolicyRules returns the list of rules that apply to a given user in a given namespace and error.  If an error is returned, the slice of
// PolicyRules may not be complete, but it contains all retrievable rules.  This is done because policy rules are purely additive and policy determinations
// can be made on the basis of those rules that are found.
func (a *openshiftAuthorizer) getEffectivePolicyRules(namespace string, user user.Info) ([]authorizationapi.PolicyRule, error) {
	roleBindings, err := a.getRoleBindings(namespace)
	if err != nil {
		return nil, err
	}

	errs := []error{}
	effectiveRules := make([]authorizationapi.PolicyRule, 0, len(roleBindings))
	for _, roleBinding := range roleBindings {
		role, err := a.getRole(roleBinding)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		for _, curr := range role.Rules {
			if doesApplyToUser(roleBinding.UserNames, roleBinding.GroupNames, user) {
				effectiveRules = append(effectiveRules, curr)
			}
		}
	}

	return effectiveRules, kerrors.NewAggregate(errs)
}
func (a *openshiftAuthorizer) getAllowedSubjectsFromNamespaceBindings(namespace string, passedAttributes AuthorizationAttributes) (util.StringSet, util.StringSet, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	roleBindings, err := a.getRoleBindings(namespace)
	if err != nil {
		return nil, nil, err
	}

	users := util.StringSet{}
	groups := util.StringSet{}
	for _, roleBinding := range roleBindings {
		role, err := a.getRole(roleBinding)
		if err != nil {
			return nil, nil, err
		}

		for _, rule := range role.Rules {
			matches, err := attributes.RuleMatches(rule)
			if err != nil {
				return nil, nil, err
			}

			if matches {
				users.Insert(roleBinding.UserNames...)
				groups.Insert(roleBinding.GroupNames...)
			}
		}
	}

	return users, groups, nil
}

func (a *openshiftAuthorizer) GetAllowedSubjects(attributes AuthorizationAttributes) ([]string, []string, error) {
	globalUsers, globalGroups, err := a.getAllowedSubjectsFromNamespaceBindings(a.masterAuthorizationNamespace, attributes)
	if err != nil {
		return nil, nil, err
	}
	localUsers, localGroups, err := a.getAllowedSubjectsFromNamespaceBindings(attributes.GetNamespace(), attributes)
	if err != nil {
		return nil, nil, err
	}

	users := util.StringSet{}
	users.Insert(globalUsers.List()...)
	users.Insert(localUsers.List()...)

	groups := util.StringSet{}
	groups.Insert(globalGroups.List()...)
	groups.Insert(localGroups.List()...)

	return users.List(), groups.List(), nil
}

func (a *openshiftAuthorizer) Authorize(passedAttributes AuthorizationAttributes) (bool, string, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	// keep track of errors in case we are unable to authorize the action.
	// It is entirely possible to get an error and be able to continue determine authorization status in spite of it.
	// This is most common when a bound role is missing, but enough roles are still present and bound to authorize the request.
	errs := []error{}

	globalAllowed, globalReason, err := a.authorizeWithNamespaceRules(a.masterAuthorizationNamespace, attributes)
	if globalAllowed {
		return true, globalReason, nil
	}
	if err != nil {
		errs = append(errs, err)
	}

	if len(attributes.GetNamespace()) != 0 {
		namespaceAllowed, namespaceReason, err := a.authorizeWithNamespaceRules(attributes.GetNamespace(), attributes)
		if namespaceAllowed {
			return true, namespaceReason, nil
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return false, "", kerrors.NewAggregate(errs)
	}

	return false, "denied by default", nil
}

// authorizeWithNamespaceRules returns isAllowed, reason, and error.  If an error is returned, isAllowed and reason are still valid.  This seems strange
// but errors are not always fatal to the authorization process.  It is entirely possible to get an error and be able to continue determine authorization
// status in spite of it.  This is most common when a bound role is missing, but enough roles are still present and bound to authorize the request.
func (a *openshiftAuthorizer) authorizeWithNamespaceRules(namespace string, passedAttributes AuthorizationAttributes) (bool, string, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	allRules, ruleRetrievalError := a.getEffectivePolicyRules(namespace, attributes.GetUserInfo())

	for _, rule := range allRules {
		matches, err := attributes.RuleMatches(rule)
		if err != nil {
			return false, "", err
		}
		if matches {
			return true, fmt.Sprintf("allowed by rule in %v: %#v", namespace, rule), nil
		}
	}

	return false, "", ruleRetrievalError
}

// TODO this may or may not be the behavior we want for managing rules.  As a for instance, a verb might be specified
// that our attributes builder will never satisfy.  For now, I think gets us close.  Maybe a warning message of some kind?
func coerceToDefaultAuthorizationAttributes(passedAttributes AuthorizationAttributes) *DefaultAuthorizationAttributes {
	attributes, ok := passedAttributes.(*DefaultAuthorizationAttributes)
	if !ok {
		attributes = &DefaultAuthorizationAttributes{
			Namespace:         passedAttributes.GetNamespace(),
			Verb:              passedAttributes.GetVerb(),
			RequestAttributes: passedAttributes.GetRequestAttributes(),
			Resource:          passedAttributes.GetResource(),
			ResourceName:      passedAttributes.GetResourceName(),
			User:              passedAttributes.GetUserInfo(),
		}
	}

	return attributes
}

func (a DefaultAuthorizationAttributes) RuleMatches(rule authorizationapi.PolicyRule) (bool, error) {
	allowedResourceTypes := resolveResources(rule)

	if a.verbMatches(util.NewStringSet(rule.Verbs...)) {
		if a.resourceMatches(allowedResourceTypes) {
			if a.nameMatches(rule.ResourceNames) {
				return true, nil
			}
		}
	}

	return false, nil
}

func resolveResources(rule authorizationapi.PolicyRule) util.StringSet {
	ret := util.StringSet{}
	toVisit := rule.Resources
	visited := util.StringSet{}

	for i := 0; i < len(toVisit); i++ {
		currResource := toVisit[i]
		if visited.Has(currResource) {
			continue
		}
		visited.Insert(currResource)

		if strings.Index(currResource, authorizationapi.ResourceGroupPrefix+":") != 0 {
			ret.Insert(strings.ToLower(currResource))
			continue
		}

		if resourceTypes, exists := authorizationapi.GroupsToResources[currResource]; exists {
			toVisit = append(toVisit, resourceTypes...)
		}
	}

	return ret
}

func (a DefaultAuthorizationAttributes) verbMatches(verbs util.StringSet) bool {
	return verbs.Has(authorizationapi.VerbAll) || verbs.Has(strings.ToLower(a.GetVerb()))
}

func (a DefaultAuthorizationAttributes) resourceMatches(allowedResourceTypes util.StringSet) bool {
	return allowedResourceTypes.Has(authorizationapi.ResourceAll) || allowedResourceTypes.Has(strings.ToLower(a.GetResource()))
}

// nameMatches checks to see if the resourceName of the action is in a the specified whitelist.  An empty whitelist indicates that any name is allowed.
// An empty string in the whitelist should only match the action's resourceName if the resourceName itself is empty string.  This behavior allows for the
// combination of a whitelist for gets in the same rule as a list that won't have a resourceName.  I don't recommend writing such a rule, but we do
// handle it like you'd expect: white list is respected for gets while not preventing the list you explicitly asked for.
func (a DefaultAuthorizationAttributes) nameMatches(allowedResourceNames util.StringSet) bool {
	if len(allowedResourceNames) == 0 {
		return true
	}

	return allowedResourceNames.Has(a.GetResourceName())
}

func (a DefaultAuthorizationAttributes) GetUserInfo() user.Info {
	return a.User
}
func (a DefaultAuthorizationAttributes) GetVerb() string {
	return a.Verb
}
func (a DefaultAuthorizationAttributes) GetResource() string {
	return a.Resource
}

func (a DefaultAuthorizationAttributes) GetResourceName() string {
	return a.ResourceName
}

func (a DefaultAuthorizationAttributes) GetNamespace() string {
	return a.Namespace
}
func (a DefaultAuthorizationAttributes) GetRequestAttributes() interface{} {
	return a.RequestAttributes
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (AuthorizationAttributes, error) {
	requestInfo, err := a.infoResolver.GetAPIRequestInfo(req)
	if err != nil {
		return nil, err
	}

	if (requestInfo.Resource == "projects") && (len(requestInfo.Name) > 0) {
		requestInfo.Namespace = requestInfo.Name
	}

	ctx, ok := a.contextMapper.Get(req)
	if !ok {
		return nil, errors.New("could not get request context")
	}
	userInfo, ok := kapi.UserFrom(ctx)
	if !ok {
		return nil, errors.New("could not get user")
	}

	return DefaultAuthorizationAttributes{
		User:              userInfo,
		Verb:              requestInfo.Verb,
		Resource:          requestInfo.Resource,
		ResourceName:      requestInfo.Name,
		Namespace:         requestInfo.Namespace,
		RequestAttributes: nil,
	}, nil
}

// TODO enumerate all resources and verbs instead of using *
func GetBootstrapPolicy(masterNamespace string) *authorizationapi.Policy {
	return &authorizationapi.Policy{
		ObjectMeta: kapi.ObjectMeta{
			Name:              authorizationapi.PolicyName,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
		},
		LastModified: util.Now(),
		Roles: map[string]authorizationapi.Role{
			"cluster-admin": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{authorizationapi.VerbAll},
						Resources: []string{authorizationapi.ResourceAll},
					},
				},
			},
			"admin": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "admin",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
						Resources: []string{authorizationapi.OpenshiftExposedGroupName, authorizationapi.PermissionGrantingGroupName, authorizationapi.KubeExposedGroupName},
					},
					{
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName},
					},
				},
			},
			"edit": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "edit",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
						Resources: []string{authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeExposedGroupName},
					},
					{
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{authorizationapi.KubeAllGroupName},
					},
				},
			},
			"view": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "view",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"get", "list", "watch"},
						Resources: []string{authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName},
					},
				},
			},
			"basic-user": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "view-self",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{Verbs: []string{"get"}, Resources: []string{"users"}, ResourceNames: util.NewStringSet("~")},
					{Verbs: []string{"list"}, Resources: []string{"projects"}},
				},
			},
			"system:deployer": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:deployer",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{authorizationapi.VerbAll},
						Resources: []string{authorizationapi.ResourceAll},
					},
				},
			},
			"system:component": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:component",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{authorizationapi.VerbAll},
						Resources: []string{authorizationapi.ResourceAll},
					},
				},
			},
			"system:delete-tokens": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:delete-tokens",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{"delete"},
						Resources: []string{"oauthaccesstoken", "oauthauthorizetoken"},
					},
				},
			},
		},
	}
}

func GetBootstrapPolicyBinding(masterNamespace string) *authorizationapi.PolicyBinding {
	return &authorizationapi.PolicyBinding{
		ObjectMeta: kapi.ObjectMeta{
			Name:              masterNamespace,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
		},
		LastModified: util.Now(),
		PolicyRef:    kapi.ObjectReference{Namespace: masterNamespace},
		RoleBindings: map[string]authorizationapi.RoleBinding{
			"system:component-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:component-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "system:component",
					Namespace: masterNamespace,
				},
				UserNames: []string{"system:openshift-client", "system:kube-client"},
			},
			"system:deployer-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:deployer-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "system:deployer",
					Namespace: masterNamespace,
				},
				UserNames: []string{"system:openshift-deployer"},
			},
			"cluster-admin-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-admin-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				UserNames: []string{"system:admin"},
			},
			"basic-user-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "basic-user-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "basic-user",
					Namespace: masterNamespace,
				},
				GroupNames: []string{"system:authenticated"},
			},
			"insecure-cluster-admin-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "insecure-cluster-admin-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				// TODO until we decide to enforce policy, simply allow every one access
				GroupNames: []string{"system:authenticated", "system:unauthenticated"},
			},
			"system:delete-tokens-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:delete-tokens-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "system:delete-tokens",
					Namespace: masterNamespace,
				},
				GroupNames: []string{"system:authenticated", "system:unauthenticated"},
			},
		},
	}
}
