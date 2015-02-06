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

type Authorizer interface {
	Authorize(a AuthorizationAttributes) (allowed bool, reason string, err error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (AuthorizationAttributes, error)
}

type AuthorizationAttributes interface {
	GetUserInfo() authenticationapi.UserInfo
	GetVerb() string
	GetResource() string
	GetNamespace() string
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

type openshiftAuthorizationAttributes struct {
	user              authenticationapi.UserInfo
	verb              string
	resource          string
	namespace         string
	requestAttributes interface{}
}

type openshiftAuthorizationAttributeBuilder struct {
	requestsToUsers *authcontext.RequestContextMap
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

func (a *openshiftAuthorizer) Authorize(passedAttributes AuthorizationAttributes) (bool, string, error) {
	attributes, ok := passedAttributes.(openshiftAuthorizationAttributes)
	if !ok {
		return false, "", fmt.Errorf("attributes are not of expected type: %#v", attributes)
	}

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
	attributes, ok := passedAttributes.(openshiftAuthorizationAttributes)
	if !ok {
		return Deny, "", fmt.Errorf("attributes are not of expected type: %#v", attributes)
	}

	allRules, err := a.getEffectivePolicyRules(namespace, attributes.GetUserInfo())
	if err != nil {
		return Deny, "", err
	}

	for _, rule := range allRules {
		matches, err := attributes.ruleMatches(rule)
		if err != nil {
			return Allow, "", err
		}
		if matches {
			return Allow, fmt.Sprintf("allowed by rule in %v: %#v", namespace, rule), nil
		}
	}

	return Unknown, "", nil
}

func (a openshiftAuthorizationAttributes) ruleMatches(rule authorizationapi.PolicyRule) (bool, error) {
	resourceNames := resolveResources(rule)

	if a.verbMatches(util.NewStringSet(rule.Verbs...)) {
		if a.resourceMatches(resourceNames) {
			return true, nil
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

		if resourceNames, exists := authorizationapi.GroupsToResources[currResource]; exists {
			toVisit = append(toVisit, resourceNames...)
		}
	}

	return ret
}

func (a openshiftAuthorizationAttributes) verbMatches(verbs util.StringSet) bool {
	verbMatches := false
	verbMatches = verbMatches || verbs.Has(strings.ToLower(a.GetVerb()))
	verbMatches = verbMatches || verbs.Has(authorizationapi.VerbAll)

	return verbMatches
}

func (a openshiftAuthorizationAttributes) resourceMatches(resourceNames util.StringSet) bool {
	kindMatches := false
	kindMatches = kindMatches || resourceNames.Has(strings.ToLower(a.GetResource()))
	kindMatches = kindMatches || resourceNames.Has(authorizationapi.ResourceAll)

	return kindMatches
}

func (a openshiftAuthorizationAttributes) GetUserInfo() authenticationapi.UserInfo {
	return a.user
}

func (a openshiftAuthorizationAttributes) GetVerb() string {
	return a.verb
}

func (a openshiftAuthorizationAttributes) GetResource() string {
	return a.resource
}

func (a openshiftAuthorizationAttributes) GetNamespace() string {
	return a.namespace
}

func (a openshiftAuthorizationAttributes) GetRequestAttributes() interface{} {
	return a.requestAttributes
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

	return openshiftAuthorizationAttributes{
		user:              userInfo,
		verb:              verb,
		resource:          kind,
		namespace:         namespace,
		requestAttributes: nil,
	}, nil
}

// TODO waiting on kube rebase
// this section is copied from kube.  Need to modify kube to make this pluggable
var specialVerbs = map[string]bool{
	"proxy":    true,
	"redirect": true,
	"watch":    true,
}

var ErrNoStandardParts = errors.New("the provided URL does not match the standard API form")

// VerbAndKindAndNamespace returns verb, kind, namespace, remaining parts, error
func VerbAndKindAndNamespace(req *http.Request) (string, string, string, []string, error) {
	parts := splitPath(req.URL.Path)
	if len(parts) == 0 {
		return "", "", "", nil, ErrNoStandardParts
	}

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
			return "", "", "", nil, ErrNoStandardParts
		}
	}

	// TODO tweak upstream to eliminate this copy  kubernetes/pkg/apiserver/handlers.go
	// handle input of form /api/{version}/* by adjusting special paths
	if parts[0] == "api" {
		if len(parts) > 2 {
			parts = parts[2:]
		} else {
			return "", "", "", parts, ErrNoStandardParts
		}
	}

	// handle input of form /{specialVerb}/*
	if _, ok := specialVerbs[parts[0]]; ok {
		verb = parts[0]
		if len(parts) > 1 {
			parts = parts[1:]
		} else {
			return "", "", "", parts, ErrNoStandardParts
		}
	}

	// URL forms: /ns/{namespace}/{kind}/*, where parts are adjusted to be relative to kind
	if parts[0] == "ns" {
		if len(parts) < 3 {
			return "", "", "", parts, fmt.Errorf("ResourceTypeAndNamespace expects a path of form /ns/{namespace}/*")
		}
		return verb, parts[1], parts[2], parts[2:], ErrNoStandardParts
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
			"ComponentRole": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "ComponentRole",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     []string{authorizationapi.VerbAll},
						Resources: []string{authorizationapi.ResourceAll},
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
