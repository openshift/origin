package authorizer

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authcontext "github.com/openshift/origin/pkg/auth/context"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type openshiftAuthorizationAttributeBuilder struct {
	requestsToUsers *authcontext.RequestContextMap
	infoResolver    *APIRequestInfoResolver
}

func NewAuthorizationAttributeBuilder(requestsToUsers *authcontext.RequestContextMap, infoResolver *APIRequestInfoResolver) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{requestsToUsers, infoResolver}
}

// GetAttributes implements AuthorizationAttributeBuilder
func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (AuthorizationAttributes, error) {
	requestInfo, err := a.infoResolver.GetAPIRequestInfo(req)
	if err != nil {
		return nil, err
	}

	userInterface, ok := a.requestsToUsers.Get(req)
	if !ok {
		return nil, errors.New("could not get user")
	}
	userInfo, ok := userInterface.(user.Info)
	if !ok {
		return nil, errors.New("wrong type returned for user")
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

type DefaultAuthorizationAttributes struct {
	User              user.Info
	Verb              string
	Resource          string
	ResourceName      string
	Namespace         string
	RequestAttributes interface{}
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

// TODO waiting on kube rebase to kill this

// APIRequestInfo holds information parsed from the http.Request
type APIRequestInfo struct {
	// Verb is the kube verb associated with the request, not the http verb.  This includes things like list and watch.
	Verb       string
	APIVersion string
	Namespace  string
	// Resource is the name of the resource being requested.  This is not the kind.  For example: pods
	Resource string
	// Kind is the type of object being manipulated.  For example: Pod
	Kind string
	// Name is empty for some verbs, but if the request directly indicates a name (not in body content) then this field is filled in.
	Name string
	// Parts are the path parts for the request relative to /{resource}/{name}
	Parts []string
}

type APIRequestInfoResolver struct {
	ApiPrefixes util.StringSet
	RestMapper  meta.RESTMapper
}

var specialVerbs = map[string]bool{
	"proxy":    true,
	"redirect": true,
	"watch":    true,
}

// GetAPIRequestInfo returns the information from the http request.  If error is not nil, APIRequestInfo holds the information as best it is known before the failure
// Valid Inputs:
// Storage paths
// /ns/{namespace}/{resource}
// /ns/{namespace}/{resource}/{resourceName}
// /{resource}
// /{resource}/{resourceName}
// /{resource}/{resourceName}?namespace={namespace}
// /{resource}?namespace={namespace}
//
// Special verbs:
// /proxy/{resource}/{resourceName}
// /proxy/ns/{namespace}/{resource}/{resourceName}
// /redirect/ns/{namespace}/{resource}/{resourceName}
// /redirect/{resource}/{resourceName}
// /watch/{resource}
// /watch/ns/{namespace}/{resource}
//
// Fully qualified paths for above:
// /api/{version}/*
// /api/{version}/*
func (r *APIRequestInfoResolver) GetAPIRequestInfo(req *http.Request) (APIRequestInfo, error) {
	requestInfo := APIRequestInfo{}

	currentParts := splitPath(req.URL.Path)
	if len(currentParts) < 1 {
		return requestInfo, fmt.Errorf("Unable to determine kind and namespace from an empty URL path")
	}

	for _, currPrefix := range r.ApiPrefixes.List() {
		// handle input of form /api/{version}/* by adjusting special paths
		if currentParts[0] == currPrefix {
			if len(currentParts) > 1 {
				requestInfo.APIVersion = currentParts[1]
			}

			if len(currentParts) > 2 {
				currentParts = currentParts[2:]
			} else {
				return requestInfo, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
			}
		}
	}

	// handle input of form /{specialVerb}/*
	if _, ok := specialVerbs[currentParts[0]]; ok {
		requestInfo.Verb = currentParts[0]

		if len(currentParts) > 1 {
			currentParts = currentParts[1:]
		} else {
			return requestInfo, fmt.Errorf("Unable to determine kind and namespace from url, %v", req.URL)
		}
	} else {
		switch req.Method {
		case "POST":
			requestInfo.Verb = "create"
		case "GET":
			requestInfo.Verb = "get"
		case "PUT":
			requestInfo.Verb = "update"
		case "DELETE":
			requestInfo.Verb = "delete"
		}

	}

	// URL forms: /ns/{namespace}/{resource}/*, where parts are adjusted to be relative to kind
	if currentParts[0] == "ns" {
		if len(currentParts) < 3 {
			return requestInfo, fmt.Errorf("ResourceTypeAndNamespace expects a path of form /ns/{namespace}/*")
		}
		requestInfo.Resource = currentParts[2]
		requestInfo.Namespace = currentParts[1]
		currentParts = currentParts[2:]

	} else {
		// URL forms: /{resource}/*
		// URL forms: POST /{resource} is a legacy API convention to create in "default" namespace
		// URL forms: /{resource}/{resourceName} use the "default" namespace if omitted from query param
		// URL forms: /{resource} assume cross-namespace operation if omitted from query param
		requestInfo.Resource = currentParts[0]
		requestInfo.Namespace = req.URL.Query().Get("namespace")
		if len(requestInfo.Namespace) == 0 {
			if len(currentParts) > 1 || req.Method == "POST" {
				requestInfo.Namespace = kapi.NamespaceDefault
			} else {
				requestInfo.Namespace = kapi.NamespaceAll
			}
		}
	}

	// parsing successful, so we now know the proper value for .Parts
	requestInfo.Parts = currentParts

	// if there's another part remaining after the kind, then that's the resource name
	if len(requestInfo.Parts) >= 2 {
		requestInfo.Name = requestInfo.Parts[1]
	}

	// if there's no name on the request and we thought it was a get before, then the actual verb is a list
	if len(requestInfo.Name) == 0 && requestInfo.Verb == "get" {
		requestInfo.Verb = "list"
	}

	// if we have a resource, we have a good shot at being able to determine kind
	if len(requestInfo.Resource) > 0 {
		_, requestInfo.Kind, _ = r.RestMapper.VersionAndKindForResource(requestInfo.Resource)
	}

	return requestInfo, nil
}

// splitPath returns the segments for a URL path.
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}
