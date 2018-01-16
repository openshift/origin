package clientcmd

import (
	"net/url"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"
)

// legacyDiscoveryClient  implements the functions that discovery server-supported API groups,
// versions and resources.
type legacyDiscoveryClient struct {
	*discovery.DiscoveryClient
}

// ServerResourcesForGroupVersion returns the supported resources for a group and version.
// This can return an error *and* a partial result
func (d *legacyDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (resources *metav1.APIResourceList, err error) {
	parentList, err := d.DiscoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return parentList, err
	}

	if groupVersion != "v1" {
		return parentList, nil
	}

	// we request v1, we must combine the parent list with the list from /oapi

	url := url.URL{}
	url.Path = "/oapi/" + groupVersion
	originResources := &metav1.APIResourceList{}
	err = d.RESTClient().Get().AbsPath(url.String()).Do().Into(originResources)
	if err != nil {
		// ignore 403 or 404 error to be compatible with an v1.0 server.
		if groupVersion == "v1" && (errors.IsNotFound(err) || errors.IsForbidden(err)) {
			return parentList, nil
		}
		return parentList, err
	}

	parentList.APIResources = append(parentList.APIResources, originResources.APIResources...)
	return parentList, nil
}

// ServerResources returns the supported resources for all groups and versions.
// This can return an error *and* a partial result
func (d *legacyDiscoveryClient) ServerResources() ([]*metav1.APIResourceList, error) {
	apiGroups, err := d.ServerGroups()
	if err != nil {
		return nil, err
	}

	result := []*metav1.APIResourceList{}
	failedGroups := make(map[schema.GroupVersion]error)

	for _, apiGroup := range apiGroups.Groups {
		for _, version := range apiGroup.Versions {
			gv := schema.GroupVersion{Group: apiGroup.Name, Version: version.Version}
			resources, err := d.ServerResourcesForGroupVersion(version.GroupVersion)
			if err != nil {
				failedGroups[gv] = err
				continue
			}

			result = append(result, resources)
		}
	}

	if len(failedGroups) == 0 {
		return result, nil
	}

	return result, &discovery.ErrGroupDiscoveryFailed{Groups: failedGroups}
}

// newLegacyDiscoveryClient creates a new DiscoveryClient for the given RESTClient.
func newLegacyDiscoveryClient(c restclient.Interface) *legacyDiscoveryClient {
	return &legacyDiscoveryClient{discovery.NewDiscoveryClient(c)}
}
