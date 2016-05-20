package scope

import (
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

// ScopesToRules takes the scopes and return the rules back.  We ALWAYS add the discovery rules and it is possible to get some rules and and
// an error since errors aren't fatal to evaluation
func ScopesToRules(scopes []string, namespace string, clusterPolicyGetter rulevalidation.ClusterPolicyGetter) ([]authorizationapi.PolicyRule, error) {
	rules := append([]authorizationapi.PolicyRule{}, authorizationapi.DiscoveryRule)

	errors := []error{}
	for _, scope := range scopes {
		found := false

		for _, evaluator := range ScopeEvaluators {
			if evaluator.Handles(scope) {
				found = true
				currRules, err := evaluator.ResolveRules(scope, namespace, clusterPolicyGetter)
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

const (
	UserIndicator          = "user:"
	ClusterRoleIndicator   = "role:"
	ClusterWideIndicator   = "clusterwide:"
	NamespaceWideIndicator = "namespace:"
)

// ScopeEvaluator takes a scope and returns the rules that express it
type ScopeEvaluator interface {
	Handles(scope string) bool
	Describe(scope string) string
	Validate(scope string) error
	ResolveRules(scope, namespace string, clusterPolicyGetter rulevalidation.ClusterPolicyGetter) ([]authorizationapi.PolicyRule, error)
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
	UserInfo        = "info"
	UserAccessCheck = "check-access"

	// UserListProject gives explicit permission to see the projects a user can see.  This is often used to prime secondary ACL systems
	// unrelated to openshift and to display projects for selection in a secondary UI.
	UserListProject = "list-projects"
)

// user:<scope name>
type userEvaluator struct{}

func (userEvaluator) Handles(scope string) bool {
	return strings.HasPrefix(scope, UserIndicator)
}

func (userEvaluator) Validate(scope string) error {
	switch scope {
	case UserIndicator + UserInfo,
		UserIndicator + UserAccessCheck,
		UserIndicator + UserListProject:
		return nil
	}

	return fmt.Errorf("unrecognized scope: %v", scope)
}

func (userEvaluator) Describe(scope string) string {
	switch scope {
	case UserIndicator + UserInfo:
		return "Information about you, including: username, identity names, and group membership."
	case UserIndicator + UserAccessCheck:
		return `Information about user privileges, e.g. "Can I create builds?"`
	case UserIndicator + UserListProject:
		return `See projects you're aware of and the metadata (display name, description, etc) about those projects.`
	default:
		return fmt.Sprintf("unrecognized scope: %v", scope)
	}
}

func (userEvaluator) ResolveRules(scope, namespace string, clusterPolicyGetter rulevalidation.ClusterPolicyGetter) ([]authorizationapi.PolicyRule, error) {
	switch scope {
	case UserIndicator + UserInfo:
		return []authorizationapi.PolicyRule{
			{Verbs: sets.NewString("get"), APIGroups: []string{userapi.GroupName}, Resources: sets.NewString("users"), ResourceNames: sets.NewString("~")},
		}, nil
	case UserIndicator + UserAccessCheck:
		return []authorizationapi.PolicyRule{
			{Verbs: sets.NewString("create"), Resources: sets.NewString("subjectaccessreviews", "localsubjectaccessreviews"), AttributeRestrictions: &authorizationapi.IsPersonalSubjectAccessReview{}},
		}, nil
	case UserIndicator + UserListProject:
		return []authorizationapi.PolicyRule{
			{Verbs: sets.NewString("list"), Resources: sets.NewString("projects")},
		}, nil
	default:
		return nil, fmt.Errorf("unrecognized scope: %v", scope)
	}
}

// escalatingScopeResources are resources that are considered escalating for scope evaluation
var escalatingScopeResources = []unversioned.GroupResource{
	{Group: kapi.GroupName, Resource: "secrets"},
	/*imageapi.GroupName*/ {Group: "", Resource: "imagestreams/secrets"},
	/*oauthapi.GroupName*/ {Group: "", Resource: "oauthauthorizetokens"}, {Group: "", Resource: "oauthaccesstokens"},
	/*authorizationapi.GroupName*/ {Group: "", Resource: "roles"}, {Group: "", Resource: "rolebindings"},
	/*authorizationapi.GroupName*/ {Group: "", Resource: "clusterroles"}, {Group: "", Resource: "clusterrolebindings"},
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

func (e clusterRoleEvaluator) Describe(scope string) string {
	roleName, scopeNamespace, escalating, err := e.parseScope(scope)
	if err != nil {
		return err.Error()
	}
	escalatingPhrase := "including any escalating resources like secrets"
	if !escalating {
		escalatingPhrase = "excluding any escalating resources like secrets"
	}

	if scopeNamespace == authorizationapi.ScopesAllNamespaces {
		return roleName + " access in all projects, " + escalatingPhrase
	}

	return roleName + " access in the " + scopeNamespace + " project, " + escalatingPhrase
}

func (e clusterRoleEvaluator) ResolveRules(scope, namespace string, clusterPolicyGetter rulevalidation.ClusterPolicyGetter) ([]authorizationapi.PolicyRule, error) {
	roleName, scopeNamespace, escalating, err := e.parseScope(scope)
	if err != nil {
		return nil, err
	}

	// if the scope limit on the clusterrole doesn't match, then don't add any rules, but its not an error
	if !(scopeNamespace == authorizationapi.ScopesAllNamespaces || scopeNamespace == namespace) {
		return []authorizationapi.PolicyRule{}, nil
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	policy, err := clusterPolicyGetter.GetClusterPolicy(ctx, "default")
	if err != nil {
		return nil, err
	}
	role, exists := policy.Roles[roleName]
	if !exists {
		return nil, kapierrors.NewNotFound(authorizationapi.Resource("clusterrole"), roleName)
	}

	rules := []authorizationapi.PolicyRule{}
	for _, rule := range role.Rules {
		if escalating {
			rules = append(rules, rule)
			continue
		}

		// rules with unbounded access shouldn't be allowed in scopes.
		if rule.Verbs.Has(authorizationapi.VerbAll) || rule.Resources.Has(authorizationapi.ResourceAll) || getAPIGroupSet(rule).Has(authorizationapi.APIGroupAll) {
			continue
		}
		// rules that allow escalating resource access should be cleaned.
		safeRule := removeEscalatingResources(rule)
		rules = append(rules, safeRule)
	}

	return rules, nil
}

// removeEscalatingResources inspects a PolicyRule and removes any references to escalating resources.
// It has coarse logic for now.  It is possible to rewrite one rule into many for the finest grain control
// but removing the entire matching resource regardless of verb or secondary group is cheaper, easier, and errs on the side removing
// too much, not too little
func removeEscalatingResources(in authorizationapi.PolicyRule) authorizationapi.PolicyRule {
	var ruleCopy *authorizationapi.PolicyRule

	apiGroups := getAPIGroupSet(in)
	for _, resource := range escalatingScopeResources {
		if !(apiGroups.Has(resource.Group) && in.Resources.Has(resource.Resource)) {
			continue
		}

		if ruleCopy == nil {
			// we're using a cache of cache of an object that uses pointers to data.  I'm pretty sure we need to do a copy to avoid
			// muddying the cache
			ruleCopy = &authorizationapi.PolicyRule{}
			authorizationapi.DeepCopy_api_PolicyRule(in, ruleCopy, nil)
		}

		ruleCopy.Resources.Delete(resource.Resource)
	}

	if ruleCopy != nil {
		return *ruleCopy
	}

	return in
}

func getAPIGroupSet(rule authorizationapi.PolicyRule) sets.String {
	apiGroups := sets.NewString(rule.APIGroups...)
	if len(apiGroups) == 0 {
		// this was done for backwards compatibility in the authorizer
		apiGroups.Insert("")
	}

	return apiGroups
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
			if err := ValidateLiteralScopeRestrictions(scope, restriction.ExactValues); err != nil {
				errs = append(errs, err)
				continue
			}
			return nil
		}

		if restriction.ClusterRole != nil {
			if !clusterRoleEvaluatorInstance.Handles(scope) {
				continue
			}
			if err := ValidateClusterRoleScopeRestrictions(scope, *restriction.ClusterRole); err != nil {
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

func ValidateLiteralScopeRestrictions(scope string, literals []string) error {
	for _, literal := range literals {
		if literal == scope {
			return nil
		}
	}

	return fmt.Errorf("%v not found in %v", scope, literals)
}

func ValidateClusterRoleScopeRestrictions(scope string, restriction oauthapi.ClusterRoleScopeRestriction) error {
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
