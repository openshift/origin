package authorizer

import (
	"path"
	"strings"

	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type DefaultAuthorizationAttributes struct {
	Verb           string
	APIVersion     string
	APIGroup       string
	Resource       string
	Subresource    string
	ResourceName   string
	NonResourceURL bool
	URL            string
}

// ToDefaultAuthorizationAttributes coerces Action to DefaultAuthorizationAttributes.  Namespace is not included
// because the authorizer takes that information on the context
func ToDefaultAuthorizationAttributes(in authorizationapi.Action) Action {
	tokens := strings.SplitN(in.Resource, "/", 2)
	resource := ""
	subresource := ""
	switch {
	case len(tokens) == 2:
		subresource = tokens[1]
		fallthrough
	case len(tokens) == 1:
		resource = tokens[0]
	}

	return DefaultAuthorizationAttributes{
		Verb:           in.Verb,
		APIGroup:       in.Group,
		APIVersion:     in.Version,
		Resource:       resource,
		Subresource:    subresource,
		ResourceName:   in.ResourceName,
		URL:            in.Path,
		NonResourceURL: in.IsNonResourceURL,
	}
}

func RuleMatches(a Action, rule authorizationapi.PolicyRule) (bool, error) {
	if a.IsNonResourceURL() {
		if nonResourceMatches(a, rule) {
			if verbMatches(a, rule.Verbs) {
				return true, nil
			}
		}

		return false, nil
	}

	// attribute restriction rules are no longer respected.  We don't return an error, because it would bubble
	// up and there nothing that a normal user can or should have to do in this situation.
	if rule.AttributeRestrictions != nil {
		return false, nil
	}

	if verbMatches(a, rule.Verbs) {
		if apiGroupMatches(a, rule.APIGroups) {

			allowedResourceTypes := authorizationapi.NormalizeResources(rule.Resources)
			if resourceMatches(a, allowedResourceTypes) {
				if nameMatches(a, rule.ResourceNames) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func apiGroupMatches(a Action, allowedGroups []string) bool {
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

func verbMatches(a Action, verbs sets.String) bool {
	return verbs.Has(authorizationapi.VerbAll) || verbs.Has(strings.ToLower(a.GetVerb()))
}

func resourceMatches(a Action, allowedResourceTypes sets.String) bool {
	if allowedResourceTypes.Has(authorizationapi.ResourceAll) {
		return true
	}

	if len(a.GetSubresource()) == 0 {
		return allowedResourceTypes.Has(strings.ToLower(a.GetResource()))
	}

	return allowedResourceTypes.Has(strings.ToLower(a.GetResource() + "/" + a.GetSubresource()))
}

// nameMatches checks to see if the resourceName of the action is in a the specified whitelist.  An empty whitelist indicates that any name is allowed.
// An empty string in the whitelist should only match the action's resourceName if the resourceName itself is empty string.  This behavior allows for the
// combination of a whitelist for gets in the same rule as a list that won't have a resourceName.  I don't recommend writing such a rule, but we do
// handle it like you'd expect: white list is respected for gets while not preventing the list you explicitly asked for.
func nameMatches(a Action, allowedResourceNames sets.String) bool {
	if len(allowedResourceNames) == 0 {
		return true
	}

	return allowedResourceNames.Has(a.GetResourceName())
}

// nonResourceMatches take the remainer of a URL and attempts to match it against a series of explicitly allowed steps that can end in a wildcard
func nonResourceMatches(a Action, rule authorizationapi.PolicyRule) bool {
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

// DefaultAuthorizationAttributes satisfies the Action interface
var _ Action = DefaultAuthorizationAttributes{}

func (a DefaultAuthorizationAttributes) GetVerb() string {
	return a.Verb
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

func (a DefaultAuthorizationAttributes) GetSubresource() string {
	return a.Subresource
}

func (a DefaultAuthorizationAttributes) GetResourceName() string {
	return a.ResourceName
}

func (a DefaultAuthorizationAttributes) IsNonResourceURL() bool {
	return a.NonResourceURL
}

func (a DefaultAuthorizationAttributes) GetURL() string {
	return a.URL
}
