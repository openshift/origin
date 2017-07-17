package clientcmd

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// ShortcutExpander is a RESTMapper that can be used for OpenShift resources.   It expands the resource first, then invokes the wrapped
type ShortcutExpander struct {
	RESTMapper meta.RESTMapper
}

var _ meta.RESTMapper = &ShortcutExpander{}

func NewShortcutExpander(discoveryClient discovery.DiscoveryInterface, delegate meta.RESTMapper) ShortcutExpander {
	defaultMapper := ShortcutExpander{RESTMapper: delegate}

	// this assumes that legacy kube versions and legacy origin versions are the same, probably fair
	apiResources, err := discoveryClient.ServerResources()
	if err != nil {
		return defaultMapper
	}

	availableResources := []schema.GroupVersionResource{}
	for _, resourceList := range apiResources {
		currVersion, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return defaultMapper
		}

		for _, resource := range resourceList.APIResources {
			availableResources = append(availableResources, currVersion.WithResource(resource.Name))
		}
	}

	return ShortcutExpander{RESTMapper: delegate}
}

func (e ShortcutExpander) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return e.RESTMapper.KindFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return e.RESTMapper.KindsFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) ResourcesFor(resource schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return e.RESTMapper.ResourcesFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) ResourceFor(resource schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return e.RESTMapper.ResourceFor(expandResourceShortcut(resource))
}

func (e ShortcutExpander) ResourceSingularizer(resource string) (string, error) {
	return e.RESTMapper.ResourceSingularizer(expandResourceShortcut(schema.GroupVersionResource{Resource: resource}).Resource)
}

func (e ShortcutExpander) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return e.RESTMapper.RESTMapping(gk, versions...)
}

func (e ShortcutExpander) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return e.RESTMapper.RESTMappings(gk, versions...)
}

// shortForms is the list of short names to their expanded names
var shortForms = map[string]string{
	"dc":           "deploymentconfigs",
	"bc":           "buildconfigs",
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
func expandResourceShortcut(resource schema.GroupVersionResource) schema.GroupVersionResource {
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
