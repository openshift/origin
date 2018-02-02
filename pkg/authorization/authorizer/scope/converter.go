package scope

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	kauthorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	authorizerrbac "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"

	oauthapi "github.com/openshift/api/oauth/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

// ScopesToRules takes the scopes and return the rules back.  We ALWAYS add the discovery rules and it is possible to get some rules and and
// an error since errors aren't fatal to evaluation
func ScopesToRules(scopes []string, namespace string, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]rbac.PolicyRule, error) {
	rules := append([]rbac.PolicyRule{}, authorizationapi.DiscoveryRule)

	errors := []error{}
	for _, scope := range scopes {
		found := false

		for _, evaluator := range ScopeEvaluators {
			if evaluator.Handles(scope) {
				found = true
				currRules, err := evaluator.ResolveRules(scope, namespace, clusterRoleGetter)
				if err != nil {
					errors = append(errors, err)
					continue
				}

				rules = append(rules, currRules...)
			}
		}

		if !found {
			errors = append(errors, fmt.Errorf("no scope evaluator found for %q", scope))
		}
	}

	return rules, kutilerrors.NewAggregate(errors)
}

// ScopesToVisibleNamespaces returns a list of namespaces that the provided scopes have "get" access to.
// This exists only to support efficiently list/watch of projects (ACLed namespaces)
func ScopesToVisibleNamespaces(scopes []string, clusterRoleGetter rbaclisters.ClusterRoleLister) (sets.String, error) {
	if len(scopes) == 0 {
		return sets.NewString("*"), nil
	}

	visibleNamespaces := sets.String{}

	errors := []error{}
	for _, scope := range scopes {
		found := false

		for _, evaluator := range ScopeEvaluators {
			if evaluator.Handles(scope) {
				found = true
				allowedNamespaces, err := evaluator.ResolveGettableNamespaces(scope, clusterRoleGetter)
				if err != nil {
					errors = append(errors, err)
					continue
				}

				visibleNamespaces.Insert(allowedNamespaces...)
				break
			}
		}

		if !found {
			errors = append(errors, fmt.Errorf("no scope evaluator found for %q", scope))
		}
	}

	return visibleNamespaces, kutilerrors.NewAggregate(errors)
}

const (
	UserIndicator        = "user:"
	ClusterRoleIndicator = "role:"
)

// ScopeEvaluator takes a scope and returns the rules that express it
type ScopeEvaluator interface {
	// Handles returns true if this evaluator can evaluate this scope
	Handles(scope string) bool
	// Validate returns an error if the scope is malformed
	Validate(scope string) error
	// Describe returns a description, warning (typically used to warn about escalation dangers), or an error if the scope is malformed
	Describe(scope string) (description string, warning string, err error)
	// ResolveRules returns the policy rules that this scope allows
	ResolveRules(scope, namespace string, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]rbac.PolicyRule, error)
	ResolveGettableNamespaces(scope string, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]string, error)
}

// ScopeEvaluators map prefixes to a function that handles that prefix
var ScopeEvaluators = []ScopeEvaluator{
	userEvaluator{},
	clusterRoleEvaluator{},
}

// scopes are in the format
// <indicator><indicator choice>
// we have the following formats:
// user:<scope name>
// role:<clusterrole name>:<namespace to allow the cluster role, * means all>
// TODO
// cluster:<comma-delimited verbs>:<comma-delimited resources>
// namespace:<namespace name>:<comma-delimited verbs>:<comma-delimited resources>

const (
	UserInfo        = UserIndicator + "info"
	UserAccessCheck = UserIndicator + "check-access"

	// UserListScopedProjects gives explicit permission to see the projects that this token can see.
	UserListScopedProjects = UserIndicator + "list-scoped-projects"

	// UserListAllProjects gives explicit permission to see the projects a user can see.  This is often used to prime secondary ACL systems
	// unrelated to openshift and to display projects for selection in a secondary UI.
	UserListAllProjects = UserIndicator + "list-projects"

	// UserFull includes all permissions of the user
	UserFull = UserIndicator + "full"
)

var defaultSupportedScopesMap = map[string]string{
	UserInfo:               "Read-only access to your user information (including username, identities, and group membership)",
	UserAccessCheck:        `Read-only access to view your privileges (for example, "can I create builds?")`,
	UserListScopedProjects: `Read-only access to list your projects viewable with this token and view their metadata (display name, description, etc.)`,
	UserListAllProjects:    `Read-only access to list your projects and view their metadata (display name, description, etc.)`,
	UserFull:               `Full read/write access with all of your permissions`,
}

func DefaultSupportedScopes() []string {
	return sets.StringKeySet(defaultSupportedScopesMap).List()
}

func DefaultSupportedScopesMap() map[string]string {
	return defaultSupportedScopesMap
}

// user:<scope name>
type userEvaluator struct{}

func (userEvaluator) Handles(scope string) bool {
	return strings.HasPrefix(scope, UserIndicator)
}

func (userEvaluator) Validate(scope string) error {
	switch scope {
	case UserFull, UserInfo, UserAccessCheck, UserListScopedProjects, UserListAllProjects:
		return nil
	}

	return fmt.Errorf("unrecognized scope: %v", scope)
}

func (userEvaluator) Describe(scope string) (string, string, error) {
	switch scope {
	case UserInfo, UserAccessCheck, UserListScopedProjects, UserListAllProjects:
		return defaultSupportedScopesMap[scope], "", nil
	case UserFull:
		return defaultSupportedScopesMap[scope], `Includes any access you have to escalating resources like secrets`, nil
	default:
		return "", "", fmt.Errorf("unrecognized scope: %v", scope)
	}
}

func (userEvaluator) ResolveRules(scope, namespace string, _ rbaclisters.ClusterRoleLister) ([]rbac.PolicyRule, error) {
	switch scope {
	case UserInfo:
		return []rbac.PolicyRule{
			rbac.NewRule("get").
				Groups(userapi.GroupName, userapi.LegacyGroupName).
				Resources("users").
				Names("~").
				RuleOrDie(),
		}, nil
	case UserAccessCheck:
		return []rbac.PolicyRule{
			rbac.NewRule("create").
				Groups(kauthorizationapi.GroupName).
				Resources("selfsubjectaccessreviews").
				RuleOrDie(),
			rbac.NewRule("create").
				Groups(authorizationapi.GroupName, authorizationapi.LegacyGroupName).
				Resources("selfsubjectrulesreviews").
				RuleOrDie(),
		}, nil
	case UserListScopedProjects:
		return []rbac.PolicyRule{
			rbac.NewRule("list", "watch").
				Groups(projectapi.GroupName, projectapi.LegacyGroupName).
				Resources("projects").
				RuleOrDie(),
		}, nil
	case UserListAllProjects:
		return []rbac.PolicyRule{
			rbac.NewRule("list", "watch").
				Groups(projectapi.GroupName, projectapi.LegacyGroupName).
				Resources("projects").
				RuleOrDie(),
			rbac.NewRule("get").
				Groups(kapi.GroupName).
				Resources("namespaces").
				RuleOrDie(),
		}, nil
	case UserFull:
		return []rbac.PolicyRule{
			rbac.NewRule(rbac.VerbAll).
				Groups(rbac.APIGroupAll).
				Resources(rbac.ResourceAll).
				RuleOrDie(),
			rbac.NewRule(rbac.VerbAll).
				URLs(rbac.NonResourceAll).
				RuleOrDie(),
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized scope: %v", scope)
	}
}

func (userEvaluator) ResolveGettableNamespaces(scope string, _ rbaclisters.ClusterRoleLister) ([]string, error) {
	switch scope {
	case UserFull, UserListAllProjects:
		return []string{"*"}, nil
	default:
		return []string{}, nil
	}
}

// escalatingScopeResources are resources that are considered escalating for scope evaluation
var escalatingScopeResources = []schema.GroupResource{
	{Group: kapi.GroupName, Resource: "secrets"},

	{Group: imageapi.GroupName, Resource: "imagestreams/secrets"},
	{Group: imageapi.LegacyGroupName, Resource: "imagestreams/secrets"},

	{Group: oauthapi.GroupName, Resource: "oauthauthorizetokens"},
	{Group: oauthapi.LegacyGroupName, Resource: "oauthauthorizetokens"},

	{Group: oauthapi.GroupName, Resource: "oauthaccesstokens"},
	{Group: oauthapi.LegacyGroupName, Resource: "oauthaccesstokens"},

	{Group: authorizationapi.GroupName, Resource: "roles"},
	{Group: authorizationapi.LegacyGroupName, Resource: "roles"},

	{Group: authorizationapi.GroupName, Resource: "rolebindings"},
	{Group: authorizationapi.LegacyGroupName, Resource: "rolebindings"},

	{Group: authorizationapi.GroupName, Resource: "clusterroles"},
	{Group: authorizationapi.LegacyGroupName, Resource: "clusterroles"},

	{Group: authorizationapi.GroupName, Resource: "clusterrolebindings"},
	{Group: authorizationapi.LegacyGroupName, Resource: "clusterrolebindings"},
}

// role:<clusterrole name>:<namespace to allow the cluster role, * means all>
type clusterRoleEvaluator struct{}

var clusterRoleEvaluatorInstance = clusterRoleEvaluator{}

func (clusterRoleEvaluator) Handles(scope string) bool {
	return strings.HasPrefix(scope, ClusterRoleIndicator)
}

func (e clusterRoleEvaluator) Validate(scope string) error {
	_, _, _, err := e.parseScope(scope)
	return err
}

// parseScope parses the requested scope, determining the requested role name, namespace, and if
// access to escalating objects is required.  It will return an error if it doesn't parse cleanly
func (e clusterRoleEvaluator) parseScope(scope string) (string /*role name*/, string /*namespace*/, bool /*escalating*/, error) {
	if !e.Handles(scope) {
		return "", "", false, fmt.Errorf("bad format for scope %v", scope)
	}
	return ParseClusterRoleScope(scope)
}
func ParseClusterRoleScope(scope string) (string /*role name*/, string /*namespace*/, bool /*escalating*/, error) {
	if !strings.HasPrefix(scope, ClusterRoleIndicator) {
		return "", "", false, fmt.Errorf("bad format for scope %v", scope)
	}
	escalating := false
	if strings.HasSuffix(scope, ":!") {
		escalating = true
		// clip that last segment before parsing the rest
		scope = scope[:strings.LastIndex(scope, ":")]
	}

	tokens := strings.SplitN(scope, ":", 2)
	if len(tokens) != 2 {
		return "", "", false, fmt.Errorf("bad format for scope %v", scope)
	}

	// namespaces can't have colons, but roles can.  pick last.
	lastColonIndex := strings.LastIndex(tokens[1], ":")
	if lastColonIndex <= 0 || lastColonIndex == (len(tokens[1])-1) {
		return "", "", false, fmt.Errorf("bad format for scope %v", scope)
	}

	return tokens[1][0:lastColonIndex], tokens[1][lastColonIndex+1:], escalating, nil
}

func (e clusterRoleEvaluator) Describe(scope string) (string, string, error) {
	roleName, scopeNamespace, escalating, err := e.parseScope(scope)
	if err != nil {
		return "", "", err
	}

	// Anything you can do [in project "foo" | server-wide] that is also allowed by the "admin" role[, except access escalating resources like secrets]

	scopePhrase := ""
	if scopeNamespace == authorizationapi.ScopesAllNamespaces {
		scopePhrase = "server-wide"
	} else {
		scopePhrase = fmt.Sprintf("in project %q", scopeNamespace)
	}

	warning := ""
	escalatingPhrase := ""
	if escalating {
		warning = fmt.Sprintf("Includes access to escalating resources like secrets")
	} else {
		escalatingPhrase = ", except access escalating resources like secrets"
	}

	description := fmt.Sprintf("Anything you can do %s that is also allowed by the %q role%s", scopePhrase, roleName, escalatingPhrase)

	return description, warning, nil
}

func (e clusterRoleEvaluator) ResolveRules(scope, namespace string, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]rbac.PolicyRule, error) {
	_, scopeNamespace, _, err := e.parseScope(scope)
	if err != nil {
		return nil, err
	}

	// if the scope limit on the clusterrole doesn't match, then don't add any rules, but its not an error
	if !(scopeNamespace == authorizationapi.ScopesAllNamespaces || scopeNamespace == namespace) {
		return []rbac.PolicyRule{}, nil
	}

	return e.resolveRules(scope, clusterRoleGetter)
}

func has(set []string, value string) bool {
	for _, element := range set {
		if value == element {
			return true
		}
	}
	return false
}

// resolveRules doesn't enforce namespace checks
func (e clusterRoleEvaluator) resolveRules(scope string, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]rbac.PolicyRule, error) {
	roleName, _, escalating, err := e.parseScope(scope)
	if err != nil {
		return nil, err
	}

	role, err := clusterRoleGetter.Get(roleName)
	if err != nil {
		return nil, err
	}

	rules := []rbac.PolicyRule{}
	for _, rule := range role.Rules {
		if escalating {
			rules = append(rules, rule)
			continue
		}

		// rules with unbounded access shouldn't be allowed in scopes.
		if has(rule.Verbs, rbac.VerbAll) ||
			has(rule.Resources, rbac.ResourceAll) ||
			has(rule.APIGroups, rbac.APIGroupAll) {
			continue
		}
		// rules that allow escalating resource access should be cleaned.
		safeRule := removeEscalatingResources(rule)
		rules = append(rules, safeRule)
	}

	return rules, nil
}

func (e clusterRoleEvaluator) ResolveGettableNamespaces(scope string, clusterRoleGetter rbaclisters.ClusterRoleLister) ([]string, error) {
	_, scopeNamespace, _, err := e.parseScope(scope)
	if err != nil {
		return nil, err
	}
	rules, err := e.resolveRules(scope, clusterRoleGetter)
	if err != nil {
		return nil, err
	}

	attributes := kauthorizer.AttributesRecord{
		APIGroup:        kapi.GroupName,
		Verb:            "get",
		Resource:        "namespaces",
		ResourceRequest: true,
	}

	if authorizerrbac.RulesAllow(attributes, rules...) {
		return []string{scopeNamespace}, nil
	}

	return []string{}, nil
}

func remove(array []string, item string) []string {
	newar := array[:0]
	for _, element := range array {
		if element != item {
			newar = append(newar, element)
		}
	}
	return newar
}

// removeEscalatingResources inspects a PolicyRule and removes any references to escalating resources.
// It has coarse logic for now.  It is possible to rewrite one rule into many for the finest grain control
// but removing the entire matching resource regardless of verb or secondary group is cheaper, easier, and errs on the side removing
// too much, not too little
func removeEscalatingResources(in rbac.PolicyRule) rbac.PolicyRule {
	var ruleCopy *rbac.PolicyRule

	for _, resource := range escalatingScopeResources {
		if !(has(in.APIGroups, resource.Group) && has(in.Resources, resource.Resource)) {
			continue
		}

		if ruleCopy == nil {
			// we're using a cache of cache of an object that uses pointers to data.  I'm pretty sure we need to do a copy to avoid
			// muddying the cache
			ruleCopy = in.DeepCopy()
		}

		ruleCopy.Resources = remove(ruleCopy.Resources, resource.Resource)
	}

	if ruleCopy != nil {
		return *ruleCopy
	}

	return in
}

func ValidateScopeRestrictions(client *oauthapi.OAuthClient, scopes ...string) error {
	if len(client.ScopeRestrictions) == 0 {
		return nil
	}
	if len(scopes) == 0 {
		return fmt.Errorf("%v may not request unscoped tokens", client.Name)
	}

	errs := []error{}
	for _, scope := range scopes {
		if err := validateScopeRestrictions(client, scope); err != nil {
			errs = append(errs, err)
		}
	}

	return kutilerrors.NewAggregate(errs)
}

func validateScopeRestrictions(client *oauthapi.OAuthClient, scope string) error {
	errs := []error{}

	if len(client.ScopeRestrictions) == 0 {
		return nil
	}

	for _, restriction := range client.ScopeRestrictions {
		if len(restriction.ExactValues) > 0 {
			if err := validateLiteralScopeRestrictions(scope, restriction.ExactValues); err != nil {
				errs = append(errs, err)
				continue
			}
			return nil
		}

		if restriction.ClusterRole != nil {
			if !clusterRoleEvaluatorInstance.Handles(scope) {
				continue
			}
			if err := validateClusterRoleScopeRestrictions(scope, *restriction.ClusterRole); err != nil {
				errs = append(errs, err)
				continue
			}
			return nil
		}
	}

	// if we got here, then nothing matched.   If we already have errors, do nothing, otherwise add one to make it report failed.
	if len(errs) == 0 {
		errs = append(errs, fmt.Errorf("%v did not match any scope restriction", scope))
	}

	return kutilerrors.NewAggregate(errs)
}

func validateLiteralScopeRestrictions(scope string, literals []string) error {
	for _, literal := range literals {
		if literal == scope {
			return nil
		}
	}

	return fmt.Errorf("%v not found in %v", scope, literals)
}

func validateClusterRoleScopeRestrictions(scope string, restriction oauthapi.ClusterRoleScopeRestriction) error {
	role, namespace, escalating, err := clusterRoleEvaluatorInstance.parseScope(scope)
	if err != nil {
		return err
	}

	foundName := false
	for _, restrictedRoleName := range restriction.RoleNames {
		if restrictedRoleName == "*" || restrictedRoleName == role {
			foundName = true
			break
		}
	}
	if !foundName {
		return fmt.Errorf("%v does not use an approved name", scope)
	}

	foundNamespace := false
	for _, restrictedNamespace := range restriction.Namespaces {
		if restrictedNamespace == "*" || restrictedNamespace == namespace {
			foundNamespace = true
			break
		}
	}
	if !foundNamespace {
		return fmt.Errorf("%v does not use an approved namespace", scope)
	}

	if escalating && !restriction.AllowEscalation {
		return fmt.Errorf("%v is not allowed to escalate", scope)
	}

	return nil
}
