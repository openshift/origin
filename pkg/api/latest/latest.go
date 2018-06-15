package latest

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

// HACK TO ELIMINATE CYCLES UNTIL WE KILL THIS PACKAGE

// Version is the string that represents the current external default version.
var Version = schema.GroupVersion{Group: "", Version: "v1"}

// OldestVersion is the string that represents the oldest server version supported,
// for client code that wants to hardcode the lowest common denominator.
var OldestVersion = schema.GroupVersion{Group: "", Version: "v1"}

// Versions is the list of versions that are recognized in code. The order provided
// may be assumed to be most preferred to least preferred, and clients may
// choose to prefer the earlier items in the list over the latter items when presented
// with a set of versions to choose.
var Versions = []schema.GroupVersion{
	{Group: "authorization.openshift.io", Version: "v1"},
	{Group: "build.openshift.io", Version: "v1"},
	{Group: "apps.openshift.io", Version: "v1"},
	{Group: "template.openshift.io", Version: "v1"},
	{Group: "image.openshift.io", Version: "v1"},
	{Group: "project.openshift.io", Version: "v1"},
	{Group: "user.openshift.io", Version: "v1"},
	{Group: "oauth.openshift.io", Version: "v1"},
	{Group: "network.openshift.io", Version: "v1"},
	{Group: "route.openshift.io", Version: "v1"},
	{Group: "quota.openshift.io", Version: "v1"},
	{Group: "security.openshift.io", Version: "v1"},
	{Group: "", Version: "v1"},
}

// originTypes are the hardcoded types defined by the OpenShift API.
var originTypes map[schema.GroupVersionKind]bool

// originTypesLock allows lazying initialization of originTypes to allow initializers to run before
// loading the map.  It means that initializers have to know ahead of time where their type is from,
// but that is not onerous
var originTypesLock sync.Once

// OriginKind returns true if OpenShift owns the GroupVersionKind.
func OriginKind(gvk schema.GroupVersionKind) bool {
	return getOrCreateOriginKinds()[gvk]
}

// OriginLegacyKind returns true for OriginKinds which are not in their own api group.
func OriginLegacyKind(gvk schema.GroupVersionKind) bool {
	return OriginKind(gvk) && gvk.Group == ""
}

// IsOriginAPIGroup returns true if the provided group name belongs to Origin API.
func IsOriginAPIGroup(groupName string) bool {
	for _, v := range Versions {
		if v.Group == groupName {
			return true
		}
	}
	return false
}

func getOrCreateOriginKinds() map[schema.GroupVersionKind]bool {
	if originTypes == nil {
		originTypesLock.Do(func() {
			newOriginTypes := map[schema.GroupVersionKind]bool{}

			// enumerate all supported versions, get the kinds, and register with the mapper how to address our resources
			for _, version := range Versions {
				for kind, t := range legacyscheme.Scheme.KnownTypes(version) {
					// these don't require special handling at the RESTMapping level since they are either "normal" when groupified
					// or under /api (not /oapi)
					if kind == "SecurityContextConstraints" {
						continue
					}
					isExternal := strings.Contains(t.PkgPath(), "github.com/openshift/api")
					isVendored := strings.Contains(t.PkgPath(), "github.com/openshift/origin/vendor/")
					if isVendored && !isExternal {
						continue
					}

					gvk := version.WithKind(kind)
					newOriginTypes[gvk] = true
				}
			}
			originTypes = newOriginTypes
		})

		return originTypes
	}

	return originTypes
}
