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
)

// user:<scope name>
type userEvaluator struct{}

func (userEvaluator) Handles(scope string) bool {
	return strings.HasPrefix(scope, UserIndicator)
}

func (userEvaluator) Validate(scope string) error {
	switch scope {
	case UserIndicator + UserInfo,
		UserIndicator + UserAccessCheck:
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

func (clusterRoleEvaluator) Handles(scope string) bool {
	return strings.HasPrefix(scope, ClusterRoleIndicator)
}

func (e clusterRoleEvaluator) Validate(scope string) error {
	_, _, err := e.getRoleNamespace(scope)
	return err
}

func (e clusterRoleEvaluator) getRoleNamespace(scope string) (string, string, error) {
	if !e.Handles(scope) {
		return "", "", fmt.Errorf("bad format for scope %v", scope)
	}
	tokens := strings.SplitN(scope, ":", 2)
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("bad format for scope %v", scope)
	}

	// namespaces can't have colons, but roles can.  pick last.
	lastColonIndex := strings.LastIndex(tokens[1], ":")
	if lastColonIndex <= 0 || lastColonIndex == (len(tokens[1])-1) {
		return "", "", fmt.Errorf("bad format for scope %v", scope)
	}

	return tokens[1][0:lastColonIndex], tokens[1][lastColonIndex+1:], nil
}

func (e clusterRoleEvaluator) Describe(scope string) string {
	roleName, scopeNamespace, err := e.getRoleNamespace(scope)
	if err != nil {
		return err.Error()
	}

	if scopeNamespace == authorizationapi.ScopesAllNamespaces {
		return roleName + " access in all projects"
	}

	return roleName + " access in the " + scopeNamespace + " project"
}

func (e clusterRoleEvaluator) ResolveRules(scope, namespace string, clusterPolicyGetter rulevalidation.ClusterPolicyGetter) ([]authorizationapi.PolicyRule, error) {
	roleName, scopeNamespace, err := e.getRoleNamespace(scope)
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

// removeEscalatingResources has coarse logic for now.  It is possible to rewrite one rule into many for the finest grain control
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
