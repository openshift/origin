package authorizer

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapiserver "github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type Authorizer interface {
	Authorize(ctx kapi.Context, a AuthorizationAttributes) (allowed bool, reason string, err error)
	GetAllowedSubjects(ctx kapi.Context, attributes AuthorizationAttributes) ([]string, []string, error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (AuthorizationAttributes, error)
}

type AuthorizationAttributes interface {
	GetVerb() string
	// GetResource returns the resource type.  If IsNonResourceURL() is true, then GetResource() is "".
	GetResource() string
	GetResourceName() string
	// GetRequestAttributes is of type interface{} because different verbs and different Authorizer/AuthorizationAttributeBuilder pairs may have different contract requirements.
	GetRequestAttributes() interface{}
	// IsNonResourceURL returns true if this is not an action performed against the resource API
	IsNonResourceURL() bool
	// GetURL returns the URL split on '/'s
	GetURL() string
}

type openshiftAuthorizer struct {
	masterAuthorizationNamespace string
	ruleResolver                 rulevalidation.AuthorizationRuleResolver
}

func NewAuthorizer(masterAuthorizationNamespace string, ruleResolver rulevalidation.AuthorizationRuleResolver) Authorizer {
	return &openshiftAuthorizer{masterAuthorizationNamespace, ruleResolver}
}

type DefaultAuthorizationAttributes struct {
	Verb              string
	Resource          string
	ResourceName      string
	RequestAttributes interface{}
	NonResourceURL    bool
	URL               string
}

type openshiftAuthorizationAttributeBuilder struct {
	contextMapper kapi.RequestContextMapper
	infoResolver  *kapiserver.APIRequestInfoResolver
}

func NewAuthorizationAttributeBuilder(contextMapper kapi.RequestContextMapper, infoResolver *kapiserver.APIRequestInfoResolver) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{contextMapper, infoResolver}
}

func doesApplyToUser(ruleUsers, ruleGroups util.StringSet, user user.Info) bool {
	if ruleUsers.Has(user.GetName()) {
		return true
	}

	for _, currGroup := range user.GetGroups() {
		if ruleGroups.Has(currGroup) {
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
func (a *openshiftAuthorizer) getAllowedSubjectsFromNamespaceBindings(ctx kapi.Context, passedAttributes AuthorizationAttributes) (util.StringSet, util.StringSet, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	roleBindings, err := a.ruleResolver.GetRoleBindings(ctx)
	if err != nil {
		return nil, nil, err
	}

	users := util.StringSet{}
	groups := util.StringSet{}
	for _, roleBinding := range roleBindings {
		role, err := a.ruleResolver.GetRole(roleBinding)
		if err != nil {
			return nil, nil, err
		}

		for _, rule := range role.Rules {
			matches, err := attributes.RuleMatches(rule)
			if err != nil {
				return nil, nil, err
			}

			if matches {
				users.Insert(roleBinding.Users.List()...)
				groups.Insert(roleBinding.Groups.List()...)
			}
		}
	}

	return users, groups, nil
}

func (a *openshiftAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes AuthorizationAttributes) ([]string, []string, error) {
	masterContext := kapi.WithNamespace(ctx, a.masterAuthorizationNamespace)
	globalUsers, globalGroups, err := a.getAllowedSubjectsFromNamespaceBindings(masterContext, attributes)
	if err != nil {
		return nil, nil, err
	}
	localUsers, localGroups, err := a.getAllowedSubjectsFromNamespaceBindings(ctx, attributes)
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

func (a *openshiftAuthorizer) Authorize(ctx kapi.Context, passedAttributes AuthorizationAttributes) (bool, string, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	// keep track of errors in case we are unable to authorize the action.
	// It is entirely possible to get an error and be able to continue determine authorization status in spite of it.
	// This is most common when a bound role is missing, but enough roles are still present and bound to authorize the request.
	errs := []error{}

	masterContext := kapi.WithNamespace(ctx, a.masterAuthorizationNamespace)
	globalAllowed, globalReason, err := a.authorizeWithNamespaceRules(masterContext, attributes)
	if globalAllowed {
		return true, globalReason, nil
	}
	if err != nil {
		errs = append(errs, err)
	}

	namespace, _ := kapi.NamespaceFrom(ctx)
	if len(namespace) != 0 {
		namespaceAllowed, namespaceReason, err := a.authorizeWithNamespaceRules(ctx, attributes)
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
func (a *openshiftAuthorizer) authorizeWithNamespaceRules(ctx kapi.Context, passedAttributes AuthorizationAttributes) (bool, string, error) {
	attributes := coerceToDefaultAuthorizationAttributes(passedAttributes)

	allRules, ruleRetrievalError := a.ruleResolver.GetEffectivePolicyRules(ctx)

	for _, rule := range allRules {
		matches, err := attributes.RuleMatches(rule)
		if err != nil {
			return false, "", err
		}
		if matches {
			return true, fmt.Sprintf("allowed by rule in %v: %#v", kapi.NamespaceValue(ctx), rule), nil
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
			Verb:              passedAttributes.GetVerb(),
			RequestAttributes: passedAttributes.GetRequestAttributes(),
			Resource:          passedAttributes.GetResource(),
			ResourceName:      passedAttributes.GetResourceName(),
			NonResourceURL:    passedAttributes.IsNonResourceURL(),
			URL:               passedAttributes.GetURL(),
		}
	}

	return attributes
}

func (a DefaultAuthorizationAttributes) RuleMatches(rule authorizationapi.PolicyRule) (bool, error) {
	if a.IsNonResourceURL() {
		if a.nonResourceMatches(rule) {
			if a.verbMatches(rule.Verbs) {
				return true, nil
			}
		}

		return false, nil
	}

	if a.verbMatches(rule.Verbs) {
		allowedResourceTypes := authorizationapi.ExpandResources(rule.Resources)

		if a.resourceMatches(allowedResourceTypes) {
			if a.nameMatches(rule.ResourceNames) {
				return true, nil
			}
		}
	}

	return false, nil
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

func (a DefaultAuthorizationAttributes) GetVerb() string {
	return a.Verb
}

// nonResourceMatches take the remainer of a URL and attempts to match it against a series of explicitly allowed steps that can end in a wildcard
func (a DefaultAuthorizationAttributes) nonResourceMatches(rule authorizationapi.PolicyRule) bool {
	for allowedNonResourcePath := range rule.NonResourceURLs {
		// if the allowed resource path ends in a wildcard, check to see if the URL starts with it
		if strings.HasSuffix(allowedNonResourcePath, "*") {
			if strings.HasPrefix(a.GetURL(), allowedNonResourcePath[0:len(allowedNonResourcePath)-1]) {
				return true
			}
		}

		// if we have an exact match, return true
		if a.GetURL() == allowedNonResourcePath {
			return true
		}
	}

	return false
}

// splitPath returns the segments for a URL path.
func splitPath(thePath string) []string {
	thePath = strings.Trim(path.Clean(thePath), "/")
	if thePath == "" {
		return []string{}
	}
	return strings.Split(thePath, "/")
}

func (a DefaultAuthorizationAttributes) GetResource() string {
	return a.Resource
}

func (a DefaultAuthorizationAttributes) GetResourceName() string {
	return a.ResourceName
}

func (a DefaultAuthorizationAttributes) GetRequestAttributes() interface{} {
	return a.RequestAttributes
}

func (a DefaultAuthorizationAttributes) IsNonResourceURL() bool {
	return a.NonResourceURL
}

func (a DefaultAuthorizationAttributes) GetURL() string {
	return a.URL
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (AuthorizationAttributes, error) {
	// any url that starts with an API prefix and is more than one step long is considered to be a resource URL.
	// That means that /api is non-resource, /api/v1beta1 is resource, /healthz is non-resource, and /swagger/anything is non-resource
	urlSegments := splitPath(req.URL.Path)
	isResourceURL := (len(urlSegments) > 1) && a.infoResolver.APIPrefixes.Has(urlSegments[0])

	if !isResourceURL {
		return DefaultAuthorizationAttributes{
			Verb:           strings.ToLower(req.Method),
			NonResourceURL: true,
			URL:            req.URL.Path,
		}, nil
	}

	requestInfo, err := a.infoResolver.GetAPIRequestInfo(req)
	if err != nil {
		return nil, err
	}

	// TODO reconsider special casing this.  Having the special case hereallow us to fully share the kube
	// APIRequestInfoResolver without any modification or customization.
	if (requestInfo.Resource == "projects") && (len(requestInfo.Name) > 0) {
		requestInfo.Namespace = requestInfo.Name
	}

	return DefaultAuthorizationAttributes{
		Verb:              requestInfo.Verb,
		Resource:          requestInfo.Resource,
		ResourceName:      requestInfo.Name,
		RequestAttributes: nil,
		NonResourceURL:    false,
		URL:               req.URL.Path,
	}, nil
}

func GetBootstrapPolicy(masterNamespace string) *authorizationapi.Policy {
	return &authorizationapi.Policy{
		ObjectMeta: kapi.ObjectMeta{
			Name:              authorizationapi.PolicyName,
			Namespace:         masterNamespace,
			CreationTimestamp: util.Now(),
			UID:               util.NewUUID(),
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
						Verbs:     util.NewStringSet(authorizationapi.VerbAll),
						Resources: util.NewStringSet(authorizationapi.ResourceAll),
					},
					{
						Verbs:           util.NewStringSet(authorizationapi.VerbAll),
						NonResourceURLs: util.NewStringSet(authorizationapi.NonResourceAll),
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
						Verbs:     util.NewStringSet("get", "list", "watch", "create", "update", "delete"),
						Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.PermissionGrantingGroupName, authorizationapi.KubeExposedGroupName),
					},
					{
						Verbs:     util.NewStringSet("get", "list", "watch"),
						Resources: util.NewStringSet(authorizationapi.PolicyOwnerGroupName, authorizationapi.KubeAllGroupName),
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
						Verbs:     util.NewStringSet("get", "list", "watch", "create", "update", "delete"),
						Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeExposedGroupName),
					},
					{
						Verbs:     util.NewStringSet("get", "list", "watch"),
						Resources: util.NewStringSet(authorizationapi.KubeAllGroupName),
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
						Verbs:     util.NewStringSet("get", "list", "watch"),
						Resources: util.NewStringSet(authorizationapi.OpenshiftExposedGroupName, authorizationapi.KubeAllGroupName),
					},
				},
			},
			"basic-user": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "view-self",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{Verbs: util.NewStringSet("get"), Resources: util.NewStringSet("users"), ResourceNames: util.NewStringSet("~")},
					{Verbs: util.NewStringSet("list"), Resources: util.NewStringSet("projects")},
				},
			},
			"cluster-status": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-status",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:           util.NewStringSet("get"),
						NonResourceURLs: util.NewStringSet("/healthz", "/version", "/api", "/osapi"),
					},
				},
			},
			"system:deployer": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "system:deployer",
					Namespace: masterNamespace,
				},
				Rules: []authorizationapi.PolicyRule{
					{
						Verbs:     util.NewStringSet(authorizationapi.VerbAll),
						Resources: util.NewStringSet(authorizationapi.ResourceAll),
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
						Verbs:     util.NewStringSet(authorizationapi.VerbAll),
						Resources: util.NewStringSet(authorizationapi.ResourceAll),
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
						Verbs:     util.NewStringSet("delete"),
						Resources: util.NewStringSet("oauthaccesstoken", "oauthauthorizetoken"),
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
			UID:               util.NewUUID(),
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
				Users: util.NewStringSet("system:openshift-client", "system:kube-client"),
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
				Users: util.NewStringSet("system:openshift-deployer"),
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
				Users: util.NewStringSet("system:admin"),
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
				Groups: util.NewStringSet("system:authenticated"),
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
				Groups: util.NewStringSet("system:authenticated", "system:unauthenticated"),
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
				Groups: util.NewStringSet("system:authenticated", "system:unauthenticated"),
			},
			"cluster-status-binding": {
				ObjectMeta: kapi.ObjectMeta{
					Name:      "cluster-status-binding",
					Namespace: masterNamespace,
				},
				RoleRef: kapi.ObjectReference{
					Name:      "cluster-status",
					Namespace: masterNamespace,
				},
				Groups: util.NewStringSet("system:authenticated", "system:unauthenticated"),
			},
		},
	}
}
