package authorizer

import (
	"fmt"
	"path"
	"strings"

	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type DefaultAuthorizationAttributes struct {
	Verb              string
	APIVersion        string
	APIGroup          string
	Resource          string
	ResourceName      string
	RequestAttributes interface{}
	NonResourceURL    bool
	URL               string
}

// ToDefaultAuthorizationAttributes coerces AuthorizationAttributes to DefaultAuthorizationAttributes.  Namespace is not included
// because the authorizer takes that information on the context
func ToDefaultAuthorizationAttributes(in authorizationapi.AuthorizationAttributes) DefaultAuthorizationAttributes {
	return DefaultAuthorizationAttributes{
		Verb:         in.Verb,
		Resource:     in.Resource,
		ResourceName: in.ResourceName,
	}
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
		if a.apiGroupMatches(rule.APIGroups) {

			allowedResourceTypes := authorizationapi.ExpandResources(rule.Resources)
			if a.resourceMatches(allowedResourceTypes) {
				if a.nameMatches(rule.ResourceNames) {
					// this rule matches the request, so we should check the additional restrictions to be sure that it's allowed
					if rule.AttributeRestrictions.Object != nil {
						switch rule.AttributeRestrictions.Object.(type) {
						case (*authorizationapi.IsPersonalSubjectAccessReview):
							return IsPersonalAccessReview(a)
						default:
							return false, fmt.Errorf("unable to interpret: %#v", rule.AttributeRestrictions.Object)
						}
					}

					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (a DefaultAuthorizationAttributes) apiGroupMatches(allowedGroups []string) bool {
	// if no APIGroups are specified, then the default APIGroup of "" is assumed.
	if len(allowedGroups) == 0 && len(a.GetAPIGroup()) == 0 {
		return true
	}

	// allowedGroups is expected to be small, so I don't feel bad about this.
	for _, allowedGroup := range allowedGroups {
		if allowedGroup == authorizationapi.APIGroupAll {
			return true
		}

		if strings.ToLower(allowedGroup) == strings.ToLower(a.GetAPIGroup()) {
			return true
		}
	}

	return false
}

func (a DefaultAuthorizationAttributes) verbMatches(verbs sets.String) bool {
	return verbs.Has(authorizationapi.VerbAll) || verbs.Has(strings.ToLower(a.GetVerb()))
}

func (a DefaultAuthorizationAttributes) resourceMatches(allowedResourceTypes sets.String) bool {
	return allowedResourceTypes.Has(authorizationapi.ResourceAll) || allowedResourceTypes.Has(strings.ToLower(a.GetResource()))
}

// nameMatches checks to see if the resourceName of the action is in a the specified whitelist.  An empty whitelist indicates that any name is allowed.
// An empty string in the whitelist should only match the action's resourceName if the resourceName itself is empty string.  This behavior allows for the
// combination of a whitelist for gets in the same rule as a list that won't have a resourceName.  I don't recommend writing such a rule, but we do
// handle it like you'd expect: white list is respected for gets while not preventing the list you explicitly asked for.
func (a DefaultAuthorizationAttributes) nameMatches(allowedResourceNames sets.String) bool {
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

func (a DefaultAuthorizationAttributes) GetAPIGroup() string {
	return a.APIGroup
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
