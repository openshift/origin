package clientcmd

import (
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// ShortcutExpander is a RESTMapper that can be used for OpenShift resources.   It expands the resource first, then invokes the wrapped
type ShortcutExpander struct {
	RESTMapper meta.RESTMapper

	All []string
}

var _ meta.RESTMapper = &ShortcutExpander{}

func NewShortcutExpander(discoveryClient discovery.DiscoveryInterface, delegate meta.RESTMapper) ShortcutExpander {
	defaultMapper := ShortcutExpander{RESTMapper: delegate}

	// this assumes that legacy kube versions and legacy origin versions are the same, probably fair
	apiResources, err := discoveryClient.ServerResources()
	if err != nil {
		return defaultMapper
	}

	availableResources := []unversioned.GroupVersionResource{}
	for groupVersionString, resourceList := range apiResources {
		currVersion, err := unversioned.ParseGroupVersion(groupVersionString)
		if err != nil {
			return defaultMapper
		}

		for _, resource := range resourceList.APIResources {
			availableResources = append(availableResources, currVersion.WithResource(resource.Name))
		}
	}

	availableAll := []string{}
	for _, requestedResource := range kcmdutil.UserResources {
		for _, availableResource := range availableResources {
			if requestedResource.Resource == availableResource.Resource {
				availableAll = append(availableAll, requestedResource.Resource)
				break
			}
		}
	}

	return ShortcutExpander{All: availableAll, RESTMapper: delegate}
}

func (e ShortcutExpander) KindFor(resource unversioned.GroupVersionResource) (unversioned.GroupVersionKind, error) {
	return e.RESTMapper.KindFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) KindsFor(resource unversioned.GroupVersionResource) ([]unversioned.GroupVersionKind, error) {
	return e.RESTMapper.KindsFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) ResourcesFor(resource unversioned.GroupVersionResource) ([]unversioned.GroupVersionResource, error) {
	return e.RESTMapper.ResourcesFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) ResourceFor(resource unversioned.GroupVersionResource) (unversioned.GroupVersionResource, error) {
	return e.RESTMapper.ResourceFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) ResourceSingularizer(resource string) (string, error) {
	return e.RESTMapper.ResourceSingularizer(expandResourceShortcut(unversioned.GroupVersionResource{Resource: resource}).Resource)
}

func (e ShortcutExpander) RESTMapping(gk unversioned.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return e.RESTMapper.RESTMapping(gk, versions...)
}

func (e ShortcutExpander) RESTMappings(gk unversioned.GroupKind) ([]*meta.RESTMapping, error) {
	return e.RESTMapper.RESTMappings(gk)
}

// AliasesForResource returns whether a resource has an alias or not
func (e ShortcutExpander) AliasesForResource(resource string) ([]string, bool) {
	allResources := []string{}
	for _, userResource := range kcmdutil.UserResources {
		allResources = append(allResources, userResource.Resource)
	}

	aliases := map[string][]string{
		"all": allResources,
	}
	if len(e.All) != 0 {
		aliases["all"] = e.All
	}

	if res, ok := aliases[resource]; ok {
		return res, true
	}
	return e.RESTMapper.AliasesForResource(expandResourceShortcut(unversioned.GroupVersionResource{Resource: resource}).Resource)
}

// shortForms is the list of short names to their expanded names
var shortForms = map[string]string{
	"dc":           "deploymentconfigs",
	"bc":           "buildconfigs",
	"hpa":          "horizontalpodautoscalers",
	"is":           "imagestreams",
	"istag":        "imagestreamtags",
	"isimage":      "imagestreamimages",
	"sa":           "serviceaccounts",
	"pv":           "persistentvolumes",
	"pvc":          "persistentvolumeclaims",
	"clusterquota": "clusterresourcequota",
}

// expandResourceShortcut will return the expanded version of resource
// (something that a pkg/api/meta.RESTMapper can understand), if it is
// indeed a shortcut. Otherwise, will return resource unmodified.
func expandResourceShortcut(resource unversioned.GroupVersionResource) unversioned.GroupVersionResource {
	if expanded, ok := shortForms[resource.Resource]; ok {
		resource.Resource = expanded
		return resource
	}
	return resource
}

// resourceShortFormFor looks up for a short form of resource names.
func resourceShortFormFor(resource string) (string, bool) {
	var alias string
	exists := false
	for k, val := range shortForms {
		if val == resource {
			alias = k
			exists = true
			break
		}
	}
	return alias, exists
}
