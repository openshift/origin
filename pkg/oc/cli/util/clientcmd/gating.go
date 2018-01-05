package clientcmd

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	authorization "github.com/openshift/api/authorization/v1"
)

// LegacyPolicyResourceGate returns err if the server does not support the set of legacy policy objects (< 3.7)
func LegacyPolicyResourceGate(client discovery.ServerResourcesInterface) error {
	// The server must support the 4 legacy policy objects in either of the GV schemes.
	_, all, err := DiscoverGroupVersionResources(client,
		schema.GroupVersionResource{
			Group:    authorization.LegacySchemeGroupVersion.Group,
			Version:  authorization.LegacySchemeGroupVersion.Version,
			Resource: "clusterpolicies",
		},
		schema.GroupVersionResource{
			Group:    authorization.LegacySchemeGroupVersion.Group,
			Version:  authorization.LegacySchemeGroupVersion.Version,
			Resource: "clusterpolicybindings",
		},
		schema.GroupVersionResource{
			Group:    authorization.LegacySchemeGroupVersion.Group,
			Version:  authorization.LegacySchemeGroupVersion.Version,
			Resource: "policies",
		},
		schema.GroupVersionResource{
			Group:    authorization.LegacySchemeGroupVersion.Group,
			Version:  authorization.LegacySchemeGroupVersion.Version,
			Resource: "policybindings",
		},
	)

	if err != nil {
		return err
	}
	if all {
		return nil
	}
	_, all, err = DiscoverGroupVersionResources(client,
		schema.GroupVersionResource{
			Group:    authorization.SchemeGroupVersion.Group,
			Version:  authorization.SchemeGroupVersion.Version,
			Resource: "clusterpolicies",
		},
		schema.GroupVersionResource{
			Group:    authorization.SchemeGroupVersion.Group,
			Version:  authorization.SchemeGroupVersion.Version,
			Resource: "clusterpolicybindings",
		},
		schema.GroupVersionResource{
			Group:    authorization.SchemeGroupVersion.Group,
			Version:  authorization.SchemeGroupVersion.Version,
			Resource: "policies",
		},
		schema.GroupVersionResource{
			Group:    authorization.SchemeGroupVersion.Group,
			Version:  authorization.SchemeGroupVersion.Version,
			Resource: "policybindings",
		},
	)

	if err != nil {
		return err
	}
	if all {
		return nil
	}

	return fmt.Errorf("the server does not support legacy policy resources")
}

// DiscoverGroupVersionResources performs a server resource discovery for each filterGVR, returning a slice of
// GVRs for the matching Resources, and a bool for "all" indicating that each item in filterGVR was found.
func DiscoverGroupVersionResources(client discovery.ServerResourcesInterface, filterGVR ...schema.GroupVersionResource) ([]schema.GroupVersionResource, bool, error) {
	if len(filterGVR) == 0 {
		return nil, false, fmt.Errorf("at least one GroupVersionResource must be provided")
	}

	var groupVersionCache = make(map[string]*v1.APIResourceList)
	all := true
	ret := []schema.GroupVersionResource{}
	for i := range filterGVR {
		var serverResources *v1.APIResourceList
		var err error

		// Discover the list of resources for this GVR, with a cache of resources per GV to avoid extra round trips
		gv := filterGVR[i].GroupVersion()
		if cachedList, ok := groupVersionCache[gv.String()]; ok {
			serverResources = cachedList
		} else {
			serverResources, err = client.ServerResourcesForGroupVersion(gv.String())
			if err != nil && errors.IsNotFound(err) {
				// Cache an empty resource list when not found, we don't want another discovery
				groupVersionCache[gv.String()] = &v1.APIResourceList{}
				all = false
				continue
			}

			if err != nil {
				return nil, false, err
			}

			groupVersionCache[gv.String()] = serverResources
		}

		seen := false
		// Add matching resources to the return slice
		for _, resource := range serverResources.APIResources {
			if filterGVR[i].Resource == resource.Name {
				seen = true
				ret = append(ret, filterGVR[i])
				break
			}
		}
		all = all && seen
	}

	return ret, all, nil
}
