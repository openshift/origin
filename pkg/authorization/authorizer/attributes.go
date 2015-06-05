package authorizer

import (
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type DefaultAuthorizationAttributes struct {
	Verb              string
	APIVersion        string
	Resource          string
	ResourceName      string
	RequestAttributes interface{}
	NonResourceURL    bool
	URL               string
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
				// this rule matches the request, so we should check the additional restrictions to be sure that it's allowed
				if !reflect.ValueOf(rule.AttributeRestrictions).IsNil() {
					switch rule.AttributeRestrictions.(type) {
					case (*authorizationapi.IsPersonalSubjectAccessReview):
						return IsPersonalAccessReview(a)
					default:
						return false, fmt.Errorf("unable to interpret: %#v", rule.AttributeRestrictions)
					}
				}

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

func (a DefaultAuthorizationAttributes) GetAPIVersion() string {
	return a.APIVersion
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
