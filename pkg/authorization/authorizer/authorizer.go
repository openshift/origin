package authorizer

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	authcontext "github.com/openshift/origin/pkg/auth/context"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

type openshiftAuthorizer struct {
	masterAuthorizationNamespace string
	policyRegistry               policyregistry.Registry
	policyBindingRegistry        policybindingregistry.Registry
}
type DefaultAuthorizationAttributes struct {
	User              authenticationapi.UserInfo
	Verb              string
	ResourceKind      string
	Namespace         string
	RequestAttributes interface{}
}
type openshiftAuthorizationAttributeBuilder struct {
	requestsToUsers *authcontext.RequestContextMap
}

type Authorizer interface {
	Authorize(a AuthorizationAttributes) (allowed bool, reason string, err error)
	GetAllowedSubjects(attributes AuthorizationAttributes) ([]string, []string, error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (AuthorizationAttributes, error)
}

type AuthorizationAttributes interface {
	GetUserInfo() authenticationapi.UserInfo
	GetVerb() string
	GetNamespace() string
	GetResourceKind() string
	// GetRequestAttributes is of type interface{} because different verbs and different Authorizer/AuthorizationAttributeBuilder pairs may have different contract requirements
	GetRequestAttributes() interface{}
}

func NewAuthorizer(masterAuthorizationNamespace string, policyRuleBindingRegistry policyregistry.Registry, policyBindingRegistry policybindingregistry.Registry) Authorizer {
	return &openshiftAuthorizer{masterAuthorizationNamespace, policyRuleBindingRegistry, policyBindingRegistry}
}
func NewAuthorizationAttributeBuilder(requestsToUsers *authcontext.RequestContextMap) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{requestsToUsers}
}

func doesApplyToUser(ruleUsers, ruleGroups []string, user authenticationapi.UserInfo) bool {
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

// getPolicy provides a point for easy caching
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

// getRole
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

func (a *openshiftAuthorizer) getEffectivePolicyRules(namespace string, user authenticationapi.UserInfo) ([]authorizationapi.PolicyRule, error) {
	roleBindings, err := a.getRoleBindings(namespace)
	if err != nil {
		return nil, err
	}

	effectiveRules := make([]authorizationapi.PolicyRule, 0, len(roleBindings))
	for _, roleBinding := range roleBindings {
		role, err := a.getRole(roleBinding)
		if err != nil {
			return nil, err
		}

		for _, curr := range role.Rules {
			if doesApplyToUser(roleBinding.UserNames, roleBinding.GroupNames, user) {
				effectiveRules = append(effectiveRules, curr)
			}
		}
	}

	return effectiveRules, nil
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
			if rule.Deny {
				continue
			}

			matches, err := attributes.RuleMatches(rule)
			if err != nil {
				return nil, nil, err
			}

			if matches {
				users.Insert(roleBinding.UserNames...)
				groups.Insert(roleBinding.GroupNames...)
			}
		}

		for _, rule := range role.Rules {
			if !rule.Deny {
				continue
			}

			matches, err := attributes.RuleMatches(rule)
			if err != nil {
				return nil, nil, err
			}

			if matches {
				users.Delete(roleBinding.UserNames...)
				groups.Delete(roleBinding.GroupNames...)
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

	globalAuthorizationResult, globalReason, err := a.authorizeWithNamespaceRules(a.masterAuthorizationNamespace, attributes)
	if err != nil {
		return false, "", err
	}
	switch globalAuthorizationResult {
	case Allow:
		return true, globalReason, nil
	case Deny:
		return false, globalReason, nil
	}

	if len(attributes.GetNamespace()) != 0 {
		namespaceAuthorizationResult, namespaceReason, err := a.authorizeWithNamespaceRules(attributes.GetNamespace(), attributes)
		if err != nil {
			return false, "", err
		}
		switch namespaceAuthorizationResult {
		case Allow:
			return true, namespaceReason, nil
		case Deny:
			return false, namespaceReason, nil
		}
	}

	return false, "denied by default", nil
}

type authorizationResult string

const (
	Allow   = authorizationResult("allow")
	Deny    = authorizationResult("deny")
	Unknown = authorizationResult("unknown")
)

func (a *openshiftAuthorizer) authorizeWithNamespaceRules(namespace string, passedAttributes AuthorizationAttributes) (authorizationResult, string, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	allRules, err := a.getEffectivePolicyRules(namespace, attributes.GetUserInfo())
	if err != nil {
		return Deny, "", err
	}

	// check for denies
	for _, rule := range allRules {
		if !rule.Deny {
			continue
		}

		matches, err := attributes.RuleMatches(rule)
		if err != nil {
			return Deny, "", err
		}
		if matches {
			return Deny, fmt.Sprintf("denied by rule in %v: %#v", namespace, rule), nil
		}
	}

	// check for allows
	for _, rule := range allRules {
		if rule.Deny {
			continue
		}

		matches, err := attributes.RuleMatches(rule)
		if err != nil {
			return Allow, "", err
		}
		if matches {
			return Allow, fmt.Sprintf("allowed by rule in %v: %#v", namespace, rule), nil
		}
	}

	return Unknown, "", nil
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
			ResourceKind:      passedAttributes.GetResourceKind(),
			User:              passedAttributes.GetUserInfo(),
		}
	}

	return attributes
}

func (a DefaultAuthorizationAttributes) RuleMatches(rule authorizationapi.PolicyRule) (bool, error) {
	if a.verbMatches(rule) {
		if a.kindMatches(rule) {
			return true, nil
		}
	}

	return false, nil
}

func (a DefaultAuthorizationAttributes) verbMatches(rule authorizationapi.PolicyRule) bool {
	verbMatches := false
	verbMatches = verbMatches || contains(rule.Verbs, a.GetVerb())
	verbMatches = verbMatches || contains(rule.Verbs, "*")

	//check for negations that would force this match to false
	verbMatches = verbMatches && !contains(rule.Verbs, "-"+a.GetVerb())
	verbMatches = verbMatches && !contains(rule.Verbs, "-*")

	return verbMatches
}

func (a DefaultAuthorizationAttributes) kindMatches(rule authorizationapi.PolicyRule) bool {
	kindMatches := false
	kindMatches = kindMatches || contains(rule.ResourceKinds, a.GetResourceKind())
	kindMatches = kindMatches || contains(rule.ResourceKinds, "*")

	//check for negations that would force this match to false
	kindMatches = kindMatches && !contains(rule.ResourceKinds, "-"+a.GetResourceKind())
	kindMatches = kindMatches && !contains(rule.ResourceKinds, "-*")

	return kindMatches
}

func (a DefaultAuthorizationAttributes) GetUserInfo() authenticationapi.UserInfo {
	return a.User
}
func (a DefaultAuthorizationAttributes) GetVerb() string {
	return a.Verb
}
func (a DefaultAuthorizationAttributes) GetResourceKind() string {
	return a.ResourceKind
}
func (a DefaultAuthorizationAttributes) GetNamespace() string {
	return a.Namespace
}
func (a DefaultAuthorizationAttributes) GetRequestAttributes() interface{} {
	return a.RequestAttributes
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (AuthorizationAttributes, error) {
	verb, kind, namespace, _, err := VerbAndKindAndNamespace(req)
	if err != nil {
		return nil, err
	}

	userInterface, ok := a.requestsToUsers.Get(req)
	if !ok {
		return nil, errors.New("could not get user")
	}
	userInfo, ok := userInterface.(authenticationapi.UserInfo)
	if !ok {
		return nil, errors.New("wrong type returned for user")
	}

	return DefaultAuthorizationAttributes{
		User:              userInfo,
		Verb:              verb,
		ResourceKind:      kind,
		Namespace:         namespace,
		RequestAttributes: nil,
	}, nil
}

// TODO waiting on kube rebase
// this section is copied from kube.  Need to modify kube to make this pluggable
var specialVerbs = map[string]bool{
	"proxy":    true,
	"redirect": true,
	"watch":    true,
}

// VerbAndKindAndNamespace returns verb, kind, namespace, remaining parts, error
func VerbAndKindAndNamespace(req *http.Request) (string, string, string, []string, error) {
	parts := splitPath(req.URL.Path)

	verb := ""
	switch req.Method {
	case "POST":
		verb = "create"
	case "GET":
		verb = "get"
	case "PUT":
		verb = "update"
	case "DELETE":
		verb = "delete"
	}

	if parts[0] == "osapi" {
		if len(parts) > 2 {
			parts = parts[2:]
		} else {
			return "", "", "", nil, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
		}
	}

	// TODO tweak upstream to eliminate this copy  kubernetes/pkg/apiserver/handlers.go
	// handle input of form /api/{version}/* by adjusting special paths
	if parts[0] == "api" {
		if len(parts) > 2 {
			parts = parts[2:]
		} else {
			return "", "", "", parts, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
		}
	}

	// handle input of form /{specialVerb}/*
	if _, ok := specialVerbs[parts[0]]; ok {
		verb = parts[0]
		if len(parts) > 1 {
			parts = parts[1:]
		} else {
			return "", "", "", parts, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
		}
	}

	// URL forms: /ns/{namespace}/{kind}/*, where parts are adjusted to be relative to kind
	if parts[0] == "ns" {
		if len(parts) < 3 {
			return "", "", "", parts, fmt.Errorf("ResourceTypeAndNamespace expects a path of form /ns/{namespace}/*")
		}
		return verb, parts[1], parts[2], parts[2:], fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
	}

	// URL forms: /{kind}/*
	// URL forms: POST /{kind} is a legacy API convention to create in "default" namespace
	// URL forms: /{kind}/{resourceName} use the "default" namespace if omitted from query param
	// URL forms: /{kind} assume cross-namespace operation if omitted from query param
	kind := parts[0]
	namespace := req.URL.Query().Get("namespace")
	if len(namespace) == 0 {
		if len(parts) > 1 || req.Method == "POST" {
			namespace = kapi.NamespaceDefault
		} else {
			namespace = kapi.NamespaceAll
		}
	}
	return verb, kind, namespace, parts, nil
}

// splitPath returns the segments for a URL path.
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

// TODO enumerate all resourceKinds and verbs instead of using *
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
						Verbs:         []string{"*"},
						ResourceKinds: []string{"*"},
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
						Verbs:         []string{"*", "-create", "-update", "-delete"},
						ResourceKinds: []string{"*"},
					},
					{
						Verbs:         []string{"create", "update", "delete"},
						ResourceKinds: []string{"*", "-policies", "-policyBindings"},
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
						Verbs:         []string{"*", "-create", "-update", "-delete"},
						ResourceKinds: []string{"*", "-roles", "-roleBindings", "-policyBindings", "-policies"},
					},
					{
						Verbs:         []string{"create", "update", "delete"},
						ResourceKinds: []string{"*", "-roles", "-roleBindings", "-policyBindings", "-policies"},
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
						Verbs:         []string{"watch", "list", "get"},
						ResourceKinds: []string{"*", "-roles", "-roleBindings", "-policyBindings", "-policies"},
					},
				},
			},
			"ComponentRole": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "ComponentRole",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:         []string{"*"},
						ResourceKinds: []string{"*"},
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
			"Components": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "Components",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "ComponentRole",
					Namespace: masterNamespace,
				},
				// TODO until we get components added to their proper groups, enumerate them here
				UserNames: []string{"openshift-client", "kube-client"},
			},
			"Cluster-Admins": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "Cluster-Admins",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-admin",
					Namespace: masterNamespace,
				},
				// TODO until we decide to enforce policy, simply allow every one access
				GroupNames: []string{"system:authenticated", "system:unauthenticated"},
			},
		},
	}
}
