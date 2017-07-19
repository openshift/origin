package restmapper

import (
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	kapi "k8s.io/kubernetes/pkg/api"
)

type discoveryRESTMapper struct {
	discoveryClient discovery.DiscoveryInterface

	delegate meta.RESTMapper

	initLock sync.Mutex
}

// NewDiscoveryRESTMapper that initializes using the discovery APIs, relying on group ordering and preferred versions
// to build its appropriate priorities.  Only versions are registered with API machinery are added now.
// TODO make this work with generic resources at some point.  For now, this handles enabled and disabled resources cleanly.
func NewDiscoveryRESTMapper(discoveryClient discovery.DiscoveryInterface) meta.RESTMapper {
	return &discoveryRESTMapper{discoveryClient: discoveryClient}
}

func (d *discoveryRESTMapper) getDelegate() (meta.RESTMapper, error) {
	d.initLock.Lock()
	defer d.initLock.Unlock()

	if d.delegate != nil {
		return d.delegate, nil
	}

	serverGroups, err := d.discoveryClient.ServerGroups()
	if err != nil {
		return nil, err
	}

	// always prefer our default group for now.  The version should be discovered from discovery, but this will hold us
	// for quite some time.
	resourcePriority := []schema.GroupVersionResource{
		{Group: kapi.GroupName, Version: meta.AnyVersion, Resource: meta.AnyResource},
	}
	kindPriority := []schema.GroupVersionKind{
		{Group: kapi.GroupName, Version: meta.AnyVersion, Kind: meta.AnyKind},
	}
	groupPriority := []string{}

	unionMapper := meta.MultiRESTMapper{}

	for _, group := range serverGroups.Groups {
		if len(group.Versions) == 0 {
			continue
		}
		groupPriority = append(groupPriority, group.Name)

		if len(group.PreferredVersion.Version) != 0 {
			preferredVersion := schema.GroupVersion{Group: group.Name, Version: group.PreferredVersion.Version}
			if kapi.Registry.IsEnabledVersion(preferredVersion) {
				resourcePriority = append(resourcePriority, preferredVersion.WithResource(meta.AnyResource))
				kindPriority = append(kindPriority, preferredVersion.WithKind(meta.AnyKind))
			}
		}

		for _, discoveryVersion := range group.Versions {
			version := schema.GroupVersion{Group: group.Name, Version: discoveryVersion.Version}
			if !kapi.Registry.IsEnabledVersion(version) {
				continue
			}
			groupMeta, err := kapi.Registry.Group(group.Name)
			if err != nil {
				return nil, err
			}
			resources, err := d.discoveryClient.ServerResourcesForGroupVersion(version.String())
			if err != nil {
				return nil, err
			}

			versionMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{version}, groupMeta.InterfacesFor)
			for _, resource := range resources.APIResources {
				// TODO properly handle resource versus kind
				gvk := version.WithKind(resource.Kind)

				scope := meta.RESTScopeNamespace
				if !resource.Namespaced {
					scope = meta.RESTScopeRoot
				}
				versionMapper.Add(gvk, scope)

				// TODO formalize this by checking to see if they support listing
				versionMapper.Add(version.WithKind(resource.Kind+"List"), scope)
			}

			// we need to add List.  Its a special case of something we need that isn't in the discovery doc
			if group.Name == kapi.GroupName {
				versionMapper.Add(version.WithKind("List"), meta.RESTScopeNamespace)
			}

			unionMapper = append(unionMapper, versionMapper)
		}
	}

	for _, group := range groupPriority {
		resourcePriority = append(resourcePriority, schema.GroupVersionResource{Group: group, Version: meta.AnyVersion, Resource: meta.AnyResource})
		kindPriority = append(kindPriority, schema.GroupVersionKind{Group: group, Version: meta.AnyVersion, Kind: meta.AnyKind})
	}

	d.delegate = meta.PriorityRESTMapper{Delegate: unionMapper, ResourcePriority: resourcePriority, KindPriority: kindPriority}
	return d.delegate, nil
}

func (d *discoveryRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	delegate, err := d.getDelegate()
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return delegate.KindFor(resource)
}

func (d *discoveryRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	delegate, err := d.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.KindsFor(resource)
}

func (d *discoveryRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	delegate, err := d.getDelegate()
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return delegate.ResourceFor(input)
}

func (d *discoveryRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	delegate, err := d.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ResourcesFor(input)
}

func (d *discoveryRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	delegate, err := d.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.RESTMapping(gk, versions...)
}

func (d *discoveryRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	delegate, err := d.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.RESTMappings(gk, versions...)
}

func (d *discoveryRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	delegate, err := d.getDelegate()
	if err != nil {
		return resource, err
	}
	return delegate.ResourceSingularizer(resource)
}
