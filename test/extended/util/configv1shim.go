package util

import (
	"context"
	"fmt"
	"sync"

	apiconfigv1 "github.com/openshift/api/config/v1"
	applyconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	fakeconfigv1client "github.com/openshift/client-go/config/clientset/versioned/fake"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1alpha1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1alpha1"
	configv1alpha2 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1alpha2"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var (
	configGroup        = "config.openshift.io"
	configVersion      = "v1"
	configGroupVersion = "config.openshift.io/v1"
)

func configAPIGroup() *metav1.APIGroup {
	return &metav1.APIGroup{
		Name: configGroup,
		Versions: []metav1.GroupVersionForDiscovery{
			{
				GroupVersion: configGroupVersion,
				Version:      configVersion,
			},
		},
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: configGroupVersion,
			Version:      configVersion,
		},
		// ServerAddressByClientCIDRs is empty
	}
}

func configGroupVersionForDiscovery() metav1.GroupVersionForDiscovery {
	return metav1.GroupVersionForDiscovery{
		GroupVersion: configGroupVersion,
		Version:      configVersion,
	}
}

func mergeResources(aList, bList []metav1.APIResource) []metav1.APIResource {
	knownKinds := make(map[string]struct{})
	for _, item := range aList {
		knownKinds[item.Kind] = struct{}{}
	}
	resources := append([]metav1.APIResource{}, aList...)
	for _, item := range bList {
		if _, exists := knownKinds[item.Kind]; exists {
			continue
		}
		resources = append(resources, item)
	}
	return resources
}

func newAPIResourceList(groupVersion string, aList, bList []metav1.APIResource) *metav1.APIResourceList {
	return &metav1.APIResourceList{
		GroupVersion: groupVersion,
		APIResources: mergeResources(aList, bList),
	}
}

// ConfigClientShim makes sure whenever there's a static
// manifest present for a config v1 kind, fake client is used
// instead of the real one.
type ConfigClientShim struct {
	configClient configv1client.Interface
	v1Kinds      map[string]bool
	fakeClient   *fakeconfigv1client.Clientset
}

func (c *ConfigClientShim) Discovery() discovery.DiscoveryInterface {
	return &ConfigV1DiscoveryClientShim{
		configClient:     c.configClient,
		fakeClient:       c.fakeClient,
		hasConfigV1Kinds: len(c.v1Kinds) > 0,
	}
}

type ConfigV1DiscoveryClientShim struct {
	configClient     configv1client.Interface
	fakeClient       *fakeconfigv1client.Clientset
	hasConfigV1Kinds bool
}

func (c *ConfigV1DiscoveryClientShim) ServerGroups() (*metav1.APIGroupList, error) {
	groups, err := c.configClient.Discovery().ServerGroups()
	if err != nil {
		return groups, err
	}

	if !c.hasConfigV1Kinds {
		return groups, nil
	}

	hasConfigGroup := false
	for i, group := range groups.Groups {
		if group.Name == configGroup {
			hasConfigGroup = true
			hasV1Version := false
			for _, version := range group.Versions {
				if version.Version == configVersion {
					hasV1Version = true
					break
				}
			}
			if !hasV1Version {
				groups.Groups[i].Versions = append(groups.Groups[i].Versions, configGroupVersionForDiscovery())
			}
			break
		}
	}
	if !hasConfigGroup {
		groups.Groups = append(groups.Groups, *configAPIGroup())
	}
	return groups, nil
}

func (c *ConfigV1DiscoveryClientShim) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	if !c.hasConfigV1Kinds || groupVersion != configGroupVersion {
		return c.configClient.Discovery().ServerResourcesForGroupVersion(groupVersion)
	}
	fakeList, err := c.fakeClient.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return nil, err
	}
	realList, err := c.configClient.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err == nil {
		return newAPIResourceList(groupVersion, fakeList.APIResources, realList.APIResources), nil
	}
	if errors.IsNotFound(err) {
		return newAPIResourceList(groupVersion, []metav1.APIResource{}, fakeList.APIResources), nil
	}
	return realList, err
}

func (c *ConfigV1DiscoveryClientShim) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	groups, resources, err := c.configClient.Discovery().ServerGroupsAndResources()
	if err != nil {
		return groups, resources, err
	}

	if c.hasConfigV1Kinds && groups != nil {
		hasConfigGroup := false
		for i, group := range groups {
			if group.Name == configGroup {
				hasConfigGroup = true
				hasV1Version := false
				for _, version := range group.Versions {
					if version.Version == configVersion {
						hasV1Version = true
						break
					}
				}
				if !hasV1Version {
					groups[i].Versions = append(groups[i].Versions, configGroupVersionForDiscovery())
				}
				break
			}
		}
		if !hasConfigGroup {
			groups = append(groups, configAPIGroup())
		}
	}

	if c.hasConfigV1Kinds && resources != nil {
		fakeList, err := c.fakeClient.Discovery().ServerResourcesForGroupVersion(configGroupVersion)
		if err != nil {
			return nil, nil, err
		}
		hasConfigGroup := false
		for i, resource := range resources {
			if resource.GroupVersion == configGroupVersion {
				hasConfigGroup = true
				resources[i].APIResources = mergeResources(fakeList.APIResources, resources[i].APIResources)
			}
		}
		if !hasConfigGroup {
			resources = append(resources, newAPIResourceList(configGroupVersion, []metav1.APIResource{}, fakeList.APIResources))
		}
	}

	return groups, resources, nil
}

func (c *ConfigV1DiscoveryClientShim) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	resources, err := c.configClient.Discovery().ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	if !c.hasConfigV1Kinds {
		return resources, nil
	}

	fakeList, err := c.fakeClient.Discovery().ServerResourcesForGroupVersion(configGroupVersion)
	if err != nil {
		return nil, err
	}

	hasConfigGroup := false
	for i, resource := range resources {
		if resource.GroupVersion == configGroupVersion {
			hasConfigGroup = true
			resources[i].APIResources = mergeResources(fakeList.APIResources, resources[i].APIResources)
		}
	}

	if !hasConfigGroup {
		resources = append(resources, newAPIResourceList(configGroupVersion, []metav1.APIResource{}, fakeList.APIResources))
	}

	return resources, nil
}

func (c *ConfigV1DiscoveryClientShim) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return c.configClient.Discovery().ServerPreferredNamespacedResources()
}

func (c *ConfigV1DiscoveryClientShim) ServerVersion() (*version.Info, error) {
	return c.configClient.Discovery().ServerVersion()
}

func (c *ConfigV1DiscoveryClientShim) OpenAPISchema() (*openapi_v2.Document, error) {
	// TODO(jchaloup): once needed implement this method as well
	panic(fmt.Errorf("APIServer not implemented"))
}

func (c *ConfigV1DiscoveryClientShim) OpenAPIV3() openapi.Client {
	// TODO(jchaloup): once needed implement this method as well
	panic(fmt.Errorf("APIServer not implemented"))
}

func (c *ConfigV1DiscoveryClientShim) WithLegacy() discovery.DiscoveryInterface {
	// TODO(jchaloup): once needed implement this method as well
	panic(fmt.Errorf("APIServer not implemented"))
}

func (c *ConfigV1DiscoveryClientShim) RESTClient() restclient.Interface {
	// TODO(jchaloup): once needed implement this method as well
	panic(fmt.Errorf("APIServer not implemented"))
}

var _ discovery.DiscoveryInterface = &ConfigV1DiscoveryClientShim{}

func (c *ConfigClientShim) ConfigV1() configv1.ConfigV1Interface {
	return &ConfigV1ClientShim{
		configv1:           c.configClient.ConfigV1(),
		v1Kinds:            c.v1Kinds,
		fakeConfigV1Client: c.fakeClient.ConfigV1(),
	}
}

func (c *ConfigClientShim) ConfigV1alpha1() configv1alpha1.ConfigV1alpha1Interface {
	return c.configClient.ConfigV1alpha1()
}

func (c *ConfigClientShim) ConfigV1alpha2() configv1alpha2.ConfigV1alpha2Interface {
	return c.configClient.ConfigV1alpha2()
}

var _ configv1client.Interface = &ConfigClientShim{}

// ConfigClientShim makes sure whenever there's a static
// manifest present for a config v1 kind, fake client is used
// instead of the real one.
type ConfigV1ClientShim struct {
	configv1           configv1.ConfigV1Interface
	v1Kinds            map[string]bool
	fakeConfigV1Client configv1.ConfigV1Interface
}

func (c *ConfigV1ClientShim) InsightsDataGathers() configv1.InsightsDataGatherInterface {
	if c.v1Kinds["APIServer"] {
		panic(fmt.Errorf("APIServer not implemented"))
	}
	return c.configv1.InsightsDataGathers()
}

func (c *ConfigV1ClientShim) APIServers() configv1.APIServerInterface {
	if c.v1Kinds["APIServer"] {
		panic(fmt.Errorf("APIServer not implemented"))
	}
	return c.configv1.APIServers()
}

func (c *ConfigV1ClientShim) Authentications() configv1.AuthenticationInterface {
	if c.v1Kinds["Authentication"] {
		panic(fmt.Errorf("Authentication not implemented"))
	}
	return c.configv1.Authentications()
}

func (c *ConfigV1ClientShim) Builds() configv1.BuildInterface {
	if c.v1Kinds["Build"] {
		panic(fmt.Errorf("Build not implemented"))
	}
	return c.configv1.Builds()
}

func (c *ConfigV1ClientShim) ClusterImagePolicies() configv1.ClusterImagePolicyInterface {
	if c.v1Kinds["ClusterImagePolicy"] {
		panic(fmt.Errorf("ClusterImagePolicies not implemented"))
	}
	return c.configv1.ClusterImagePolicies()
}

func (c *ConfigV1ClientShim) ClusterOperators() configv1.ClusterOperatorInterface {
	if c.v1Kinds["ClusterOperator"] {
		panic(fmt.Errorf("ClusterOperator not implemented"))
	}
	return c.configv1.ClusterOperators()
}

func (c *ConfigV1ClientShim) ClusterVersions() configv1.ClusterVersionInterface {
	if c.v1Kinds["ClusterVersion"] {
		panic(fmt.Errorf("ClusterVersion not implemented"))
	}
	return c.configv1.ClusterVersions()
}

func (c *ConfigV1ClientShim) Consoles() configv1.ConsoleInterface {
	if c.v1Kinds["Console"] {
		panic(fmt.Errorf("Console not implemented"))
	}
	return c.configv1.Consoles()
}

func (c *ConfigV1ClientShim) DNSes() configv1.DNSInterface {
	if c.v1Kinds["DNS"] {
		panic(fmt.Errorf("DNS not implemented"))
	}
	return c.configv1.DNSes()
}

func (c *ConfigV1ClientShim) FeatureGates() configv1.FeatureGateInterface {
	if c.v1Kinds["FeatureGate"] {
		panic(fmt.Errorf("FeatureGate not implemented"))
	}
	return c.configv1.FeatureGates()
}

func (c *ConfigV1ClientShim) Images() configv1.ImageInterface {
	if c.v1Kinds["Image"] {
		panic(fmt.Errorf("Image not implemented"))
	}
	return c.configv1.Images()
}

func (c *ConfigV1ClientShim) ImageContentPolicies() configv1.ImageContentPolicyInterface {
	if c.v1Kinds["ImageContentPolicy"] {
		panic(fmt.Errorf("ImageContentPolicie not implemented"))
	}
	return c.configv1.ImageContentPolicies()
}

func (c *ConfigV1ClientShim) ImageDigestMirrorSets() configv1.ImageDigestMirrorSetInterface {
	if c.v1Kinds["ImageDigestMirrorSet"] {
		panic(fmt.Errorf("ImageDigestMirrorSet not implemented"))
	}
	return c.configv1.ImageDigestMirrorSets()
}

func (c *ConfigV1ClientShim) ImagePolicies(namespace string) configv1.ImagePolicyInterface {
	if c.v1Kinds["ImagePolicy"] {
		panic(fmt.Errorf("ImagePolicy not implemented"))
	}
	return c.configv1.ImagePolicies(namespace)
}

func (c *ConfigV1ClientShim) ImageTagMirrorSets() configv1.ImageTagMirrorSetInterface {
	if c.v1Kinds["ImageTagMirrorSet"] {
		panic(fmt.Errorf("ImageTagMirrorSet not implemented"))
	}
	return c.configv1.ImageTagMirrorSets()
}

func (c *ConfigV1ClientShim) Infrastructures() configv1.InfrastructureInterface {
	return &ConfigV1InfrastructuresClientShim{
		fakeConfigV1InfrastructuresClient: c.fakeConfigV1Client.Infrastructures(),
		configV1InfrastructuresClient:     c.configv1.Infrastructures(),
	}
}

func (c *ConfigV1ClientShim) Ingresses() configv1.IngressInterface {
	if c.v1Kinds["Ingress"] {
		panic(fmt.Errorf("Ingresse not implemented"))
	}
	return c.configv1.Ingresses()
}

func (c *ConfigV1ClientShim) Networks() configv1.NetworkInterface {
	return &ConfigV1NetworksClientShim{
		fakeConfigV1NetworksClient: c.fakeConfigV1Client.Networks(),
		configV1NetworksClient:     c.configv1.Networks(),
	}
}

func (c *ConfigV1ClientShim) Nodes() configv1.NodeInterface {
	if c.v1Kinds["Node"] {
		panic(fmt.Errorf("Node not implemented"))
	}
	return c.configv1.Nodes()
}

func (c *ConfigV1ClientShim) OAuths() configv1.OAuthInterface {
	if c.v1Kinds["OAuth"] {
		panic(fmt.Errorf("OAuth not implemented"))
	}
	return c.configv1.OAuths()
}

func (c *ConfigV1ClientShim) OperatorHubs() configv1.OperatorHubInterface {
	if c.v1Kinds["OperatorHub"] {
		panic(fmt.Errorf("OperatorHub not implemented"))
	}
	return c.configv1.OperatorHubs()
}

func (c *ConfigV1ClientShim) Projects() configv1.ProjectInterface {
	if c.v1Kinds["Project"] {
		panic(fmt.Errorf("Project not implemented"))
	}
	return c.configv1.Projects()
}

func (c *ConfigV1ClientShim) Proxies() configv1.ProxyInterface {
	if c.v1Kinds["Proxy"] {
		panic(fmt.Errorf("Proxie not implemented"))
	}
	return c.configv1.Proxies()
}

func (c *ConfigV1ClientShim) Schedulers() configv1.SchedulerInterface {
	if c.v1Kinds["Scheduler"] {
		panic(fmt.Errorf("Scheduler not implemented"))
	}
	return c.configv1.Schedulers()
}

func (c *ConfigV1ClientShim) RESTClient() rest.Interface {
	return c.configv1.RESTClient()
}

var _ configv1.ConfigV1Interface = &ConfigV1ClientShim{}

type ConfigV1InfrastructuresClientShim struct {
	fakeConfigV1InfrastructuresClient configv1.InfrastructureInterface
	configV1InfrastructuresClient     configv1.InfrastructureInterface
}

var _ configv1.InfrastructureInterface = &ConfigV1InfrastructuresClientShim{}

func (c *ConfigV1InfrastructuresClientShim) Create(ctx context.Context, infrastructure *apiconfigv1.Infrastructure, opts metav1.CreateOptions) (*apiconfigv1.Infrastructure, error) {
	_, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "create"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.Create(ctx, infrastructure, opts)
	}
	return nil, err
}

func (c *ConfigV1InfrastructuresClientShim) Update(ctx context.Context, infrastructure *apiconfigv1.Infrastructure, opts metav1.UpdateOptions) (*apiconfigv1.Infrastructure, error) {
	_, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "update"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.Update(ctx, infrastructure, opts)
	}
	return nil, err
}

func (c *ConfigV1InfrastructuresClientShim) UpdateStatus(ctx context.Context, infrastructure *apiconfigv1.Infrastructure, opts metav1.UpdateOptions) (*apiconfigv1.Infrastructure, error) {
	_, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "updatestatus"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.UpdateStatus(ctx, infrastructure, opts)
	}
	return nil, err
}

func (c *ConfigV1InfrastructuresClientShim) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return &OperationNotPermitted{Action: "delete"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.Delete(ctx, name, opts)
	}
	return err
}

func (c *ConfigV1InfrastructuresClientShim) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	list, err := c.fakeConfigV1InfrastructuresClient.List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("unable to list objects during DeleteCollection request: %v", err)
	}
	// if either of the static manifests is expected to be deleted, the whole request is invalid
	if len(list.Items) > 0 {
		return &OperationNotPermitted{Action: "deletecollection"}
	}
	return c.configV1InfrastructuresClient.DeleteCollection(ctx, opts, listOpts)
}

func (c *ConfigV1InfrastructuresClientShim) Get(ctx context.Context, name string, opts metav1.GetOptions) (*apiconfigv1.Infrastructure, error) {
	obj, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return obj, nil
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.Get(ctx, name, opts)
	}
	return nil, err
}

func (c *ConfigV1InfrastructuresClientShim) List(ctx context.Context, opts metav1.ListOptions) (*apiconfigv1.InfrastructureList, error) {
	items := []apiconfigv1.Infrastructure{}
	knownKeys := make(map[string]struct{})

	if len(opts.FieldSelector) > 0 {
		sel, err := fields.ParseSelector(opts.FieldSelector)
		if err != nil {
			return nil, fmt.Errorf("unable to parse field selector")
		}
		// list all objects
		staticObjList, err := c.fakeConfigV1InfrastructuresClient.List(ctx, metav1.ListOptions{LabelSelector: opts.LabelSelector})
		if err != nil {
			return nil, err
		}
		for _, item := range staticObjList.Items {
			// existing item is still a known item even though it does not match
			// the field selector. It still needs to be excluded from the real objects.
			knownKeys[item.Name] = struct{}{}

			// Based on https://github.com/openshift/origin/pull/27714 and
			// https://github.com/kubernetes/kubernetes/blob/f14cc7fdfcfcedafc7910f043ec6eb74930cfee7/staging/src/k8s.io/apiextensions-apiserver/pkg/apiserver/conversion/converter.go#L128-L138
			// only metadata.Name and metadata.Namespaces are supported for CRDs
			// Infrastructure is cluster scoped -> no metadata.namespace
			if !sel.Matches(fields.Set(map[string]string{"metadata.name": item.Name})) {
				continue
			}
			items = append(items, item)
		}
	} else {
		staticObjList, err := c.fakeConfigV1InfrastructuresClient.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, item := range staticObjList.Items {
			items = append(items, item)
			knownKeys[item.Name] = struct{}{}
		}
	}

	objList, err := c.configV1InfrastructuresClient.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, item := range objList.Items {
		// skip objects with corresponding static manifests
		if _, exists := knownKeys[item.Name]; exists {
			continue
		}
		items = append(items, item)
		knownKeys[item.Name] = struct{}{}
	}

	return &apiconfigv1.InfrastructureList{
		TypeMeta: objList.TypeMeta,
		ListMeta: objList.ListMeta,
		Items:    items,
	}, nil
}

func (c *ConfigV1InfrastructuresClientShim) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	// static manifests do not produce any watch event besides create
	// If the object exists, no need to generate the ADDED watch event

	staticObjList, err := c.fakeConfigV1InfrastructuresClient.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	resourceWatcher, err := c.configV1InfrastructuresClient.Watch(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(staticObjList.Items) == 0 {
		return resourceWatcher, nil
	}

	// Only reduces the set of objects generating the ADD event
	// The Follow will still get the full list of ignored objects
	watchSel := fields.Everything()
	if len(opts.FieldSelector) > 0 {
		sel, err := fields.ParseSelector(opts.FieldSelector)
		if err != nil {
			return nil, fmt.Errorf("unable to parse field selector")
		}
		watchSel = sel
	}

	objs := []runtime.Object{}
	watcher := NewFakeWatcher()
	// Produce ADDED watch event types for the static manifests
	for _, item := range staticObjList.Items {
		// make shallow copy
		obj := item
		// Based on https://github.com/openshift/origin/pull/27714 and
		// https://github.com/kubernetes/kubernetes/blob/f14cc7fdfcfcedafc7910f043ec6eb74930cfee7/staging/src/k8s.io/apiextensions-apiserver/pkg/apiserver/conversion/converter.go#L128-L138
		// only metadata.Name and metadata.Namespaces are supported for CRDs
		// Infrastructure is cluster scoped -> no metadata.namespace
		if watchSel.Matches(fields.Set(map[string]string{"metadata.name": item.Name})) {
			watcher.Action(watch.Added, &obj)
		}
		objs = append(objs, &obj)
	}

	go func() {
		err := watcher.Follow(resourceWatcher, objs...)
		if err != nil {
			klog.Errorf("Config shim fake watcher returned prematurely: %v", err)
		}
		watcher.Stop()
	}()

	return watcher, nil
}

func (c *ConfigV1InfrastructuresClientShim) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*apiconfigv1.Infrastructure, error) {
	_, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "patch"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.Patch(ctx, name, pt, data, opts, subresources...)
	}
	return nil, err
}

func (c *ConfigV1InfrastructuresClientShim) Apply(ctx context.Context, infrastructure *applyconfigv1.InfrastructureApplyConfiguration, opts metav1.ApplyOptions) (*apiconfigv1.Infrastructure, error) {
	// Unable to determine existence of a static manifest
	if infrastructure == nil || infrastructure.Name == nil {
		return c.configV1InfrastructuresClient.Apply(ctx, infrastructure, opts)
	}
	_, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, *infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "apply"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.Apply(ctx, infrastructure, opts)
	}
	return nil, err
}

func (c *ConfigV1InfrastructuresClientShim) ApplyStatus(ctx context.Context, infrastructure *applyconfigv1.InfrastructureApplyConfiguration, opts metav1.ApplyOptions) (*apiconfigv1.Infrastructure, error) {
	// Unable to determine existence of a static manifest
	if infrastructure == nil || infrastructure.Name == nil {
		return c.configV1InfrastructuresClient.ApplyStatus(ctx, infrastructure, opts)
	}
	_, err := c.fakeConfigV1InfrastructuresClient.Get(ctx, *infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "applystatus"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1InfrastructuresClient.ApplyStatus(ctx, infrastructure, opts)
	}
	return nil, err
}

type ConfigV1NetworksClientShim struct {
	fakeConfigV1NetworksClient configv1.NetworkInterface
	configV1NetworksClient     configv1.NetworkInterface
}

var _ configv1.NetworkInterface = &ConfigV1NetworksClientShim{}

func (c *ConfigV1NetworksClientShim) Create(ctx context.Context, network *apiconfigv1.Network, opts metav1.CreateOptions) (*apiconfigv1.Network, error) {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, network.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "create"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Create(ctx, network, opts)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) Update(ctx context.Context, network *apiconfigv1.Network, opts metav1.UpdateOptions) (*apiconfigv1.Network, error) {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, network.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "update"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Update(ctx, network, opts)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) UpdateStatus(ctx context.Context, network *apiconfigv1.Network, opts metav1.UpdateOptions) (*apiconfigv1.Network, error) {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, network.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "updatestatus"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.UpdateStatus(ctx, network, opts)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return &OperationNotPermitted{Action: "delete"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Delete(ctx, name, opts)
	}
	return err
}

func (c *ConfigV1NetworksClientShim) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	list, err := c.fakeConfigV1NetworksClient.List(ctx, listOpts)
	if err != nil {
		return fmt.Errorf("unable to list objects during DeleteCollection request: %v", err)
	}
	// if either of the static manifests is expected to be deleted, the whole request is invalid
	if len(list.Items) > 0 {
		return &OperationNotPermitted{Action: "deletecollection"}
	}
	return c.configV1NetworksClient.DeleteCollection(ctx, opts, listOpts)
}

func (c *ConfigV1NetworksClientShim) Get(ctx context.Context, name string, opts metav1.GetOptions) (*apiconfigv1.Network, error) {
	obj, err := c.fakeConfigV1NetworksClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return obj, nil
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Get(ctx, name, opts)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) List(ctx context.Context, opts metav1.ListOptions) (*apiconfigv1.NetworkList, error) {
	items := []apiconfigv1.Network{}
	knownKeys := make(map[string]struct{})

	if len(opts.FieldSelector) > 0 {
		sel, err := fields.ParseSelector(opts.FieldSelector)
		if err != nil {
			return nil, fmt.Errorf("unable to parse field selector")
		}
		// list all objects
		staticObjList, err := c.fakeConfigV1NetworksClient.List(ctx, metav1.ListOptions{LabelSelector: opts.LabelSelector})
		if err != nil {
			return nil, err
		}
		for _, item := range staticObjList.Items {
			// existing item is still a known item even though it does not match
			// the field selector. It still needs to be excluded from the real objects.
			knownKeys[item.Name] = struct{}{}

			// Based on https://github.com/openshift/origin/pull/27714 and
			// https://github.com/kubernetes/kubernetes/blob/f14cc7fdfcfcedafc7910f043ec6eb74930cfee7/staging/src/k8s.io/apiextensions-apiserver/pkg/apiserver/conversion/converter.go#L128-L138
			// only metadata.Name and metadata.Namespaces are supported for CRDs
			// Network is cluster scoped -> no metadata.namespace
			if !sel.Matches(fields.Set(map[string]string{"metadata.name": item.Name})) {
				continue
			}
			items = append(items, item)
		}
	} else {
		staticObjList, err := c.fakeConfigV1NetworksClient.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, item := range staticObjList.Items {
			items = append(items, item)
			knownKeys[item.Name] = struct{}{}
		}
	}

	objList, err := c.configV1NetworksClient.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, item := range objList.Items {
		// skip objects with corresponding static manifests
		if _, exists := knownKeys[item.Name]; exists {
			continue
		}
		items = append(items, item)
		knownKeys[item.Name] = struct{}{}
	}

	return &apiconfigv1.NetworkList{
		TypeMeta: objList.TypeMeta,
		ListMeta: objList.ListMeta,
		Items:    items,
	}, nil
}

func (c *ConfigV1NetworksClientShim) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	// static manifests do not produce any watch event besides create
	// If the object exists, no need to generate the ADDED watch event

	staticObjList, err := c.fakeConfigV1NetworksClient.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	resourceWatcher, err := c.configV1NetworksClient.Watch(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(staticObjList.Items) == 0 {
		return resourceWatcher, nil
	}

	// Only reduces the set of objects generating the ADD event
	// The Follow will still get the full list of ignored objects
	watchSel := fields.Everything()
	if len(opts.FieldSelector) > 0 {
		sel, err := fields.ParseSelector(opts.FieldSelector)
		if err != nil {
			return nil, fmt.Errorf("unable to parse field selector")
		}
		watchSel = sel
	}

	objs := []runtime.Object{}
	watcher := NewFakeWatcher()
	// Produce ADDED watch event types for the static manifests
	for _, item := range staticObjList.Items {
		// make shallow copy
		obj := item
		// Based on https://github.com/openshift/origin/pull/27714 and
		// https://github.com/kubernetes/kubernetes/blob/f14cc7fdfcfcedafc7910f043ec6eb74930cfee7/staging/src/k8s.io/apiextensions-apiserver/pkg/apiserver/conversion/converter.go#L128-L138
		// only metadata.Name and metadata.Namespaces are supported for CRDs
		// Infrastructure is cluster scoped -> no metadata.namespace
		if watchSel.Matches(fields.Set(map[string]string{"metadata.name": item.Name})) {
			watcher.Action(watch.Added, &obj)
		}
		objs = append(objs, &obj)
	}

	go func() {
		err := watcher.Follow(resourceWatcher, objs...)
		if err != nil {
			klog.Errorf("Config shim fake watcher returned prematurely: %v", err)
		}
		watcher.Stop()
	}()

	return watcher, nil
}

func (c *ConfigV1NetworksClientShim) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*apiconfigv1.Network, error) {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "patch"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Patch(ctx, name, pt, data, opts, subresources...)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) Apply(ctx context.Context, network *applyconfigv1.NetworkApplyConfiguration, opts metav1.ApplyOptions) (*apiconfigv1.Network, error) {
	// Unable to determine existence of a static manifest
	if network == nil || network.Name == nil {
		return c.configV1NetworksClient.Apply(ctx, network, opts)
	}
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, *network.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "apply"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Apply(ctx, network, opts)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) ApplyStatus(ctx context.Context, network *applyconfigv1.NetworkApplyConfiguration, opts metav1.ApplyOptions) (*apiconfigv1.Network, error) {
	// Unable to determine existence of a static manifest
	if network == nil || network.Name == nil {
		return c.configV1NetworksClient.ApplyStatus(ctx, network, opts)
	}
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, *network.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "applystatus"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.ApplyStatus(ctx, network, opts)
	}
	return nil, err
}

const (
	DefaultChanSize = 1000
)

// RaceFreeFakeWatcher lets you test anything that consumes a watch.Interface; threadsafe.
type FakeWatcher struct {
	result  chan watch.Event
	Stopped bool
	sync.Mutex
}

var _ watch.Interface = &FakeWatcher{}

func NewFakeWatcher() *FakeWatcher {
	return &FakeWatcher{
		result: make(chan watch.Event, DefaultChanSize),
	}
}

// Stop implements Interface.Stop().
func (f *FakeWatcher) Stop() {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		klog.V(4).Infof("Stopping fake watcher.")
		close(f.result)
		f.Stopped = true
	}
}

func (f *FakeWatcher) IsStopped() bool {
	f.Lock()
	defer f.Unlock()
	return f.Stopped
}

// Reset prepares the watcher to be reused.
func (f *FakeWatcher) Reset() {
	f.Lock()
	defer f.Unlock()
	f.Stopped = false
	f.result = make(chan watch.Event, DefaultChanSize)
}

func (f *FakeWatcher) ResultChan() <-chan watch.Event {
	f.Lock()
	defer f.Unlock()
	return f.result
}

type itemKey struct {
	namespace, name string
}

func (f *FakeWatcher) Follow(inputChan watch.Interface, ignoreList ...runtime.Object) error {
	ignore := make(map[itemKey]struct{})

	for _, item := range ignoreList {
		accessor, err := apimeta.Accessor(item)
		if err != nil {
			return fmt.Errorf("unable to construct meta accessor: %v", err)
		}
		ignore[itemKey{namespace: accessor.GetNamespace(), name: accessor.GetName()}] = struct{}{}
	}

	for {
		if !f.Stopped {
			select {
			case item := <-inputChan.ResultChan():
				accessor, err := apimeta.Accessor(item.Object)
				if err != nil {
					return fmt.Errorf("unable to construct meta accessor: %v", err)
				}
				if _, exists := ignore[itemKey{namespace: accessor.GetNamespace(), name: accessor.GetName()}]; exists {
					continue
				}
				klog.V(4).Infof("FakeWatcher.Follow: item.Type: %v, item.Object: %v\n", item.Type, item.Object)
				f.Action(item.Type, item.Object)
			}
		}
	}
}

// Action sends an event of the requested type, for table-based testing.
func (f *FakeWatcher) Action(action watch.EventType, obj runtime.Object) {
	f.Lock()
	defer f.Unlock()
	if !f.Stopped {
		select {
		case f.result <- watch.Event{Type: action, Object: obj}:
			return
		default:
			panic(fmt.Errorf("channel full"))
		}
	}
}

type OperationNotPermitted struct {
	Action string
}

func (e OperationNotPermitted) Error() string {
	return fmt.Sprintf("operation %q not permitted", e.Action)
}

func configV1InfrastructureAPIResources() []metav1.APIResource {
	return []metav1.APIResource{{
		Name:         "infrastructures",
		SingularName: "infrastructure",
		Namespaced:   false,
		Kind:         "Infrastructure",
		Verbs: []string{
			"get", "list", "watch", // only read-only requests permitted
		},
	},
		{
			Name:         "infrastructures/status",
			SingularName: "",
			Namespaced:   false,
			Kind:         "Infrastructure",
			Verbs: []string{
				"get", // only read-only requests permitted
			},
		},
	}
}

func configV1NetworkAPIResources() []metav1.APIResource {
	return []metav1.APIResource{{
		Name:         "networks",
		SingularName: "network",
		Namespaced:   false,
		Kind:         "Network",
		Verbs: []string{
			"get", "list", "watch", // only read-only requests permitted
		},
	},
	}
}

func NewConfigClientShim(
	configClient configv1client.Interface,
	objects []runtime.Object,
) *ConfigClientShim {
	fakeClient := fakeconfigv1client.NewSimpleClientset(objects...)

	v1Kinds := make(map[string]bool)
	// make sure every mutating operation is not permitted
	for _, object := range objects {
		objectKind := object.GetObjectKind().GroupVersionKind()
		// currently supportig only config.openshift.io/v1 apiversion
		if objectKind.Group != "config.openshift.io" {
			continue
		}
		if objectKind.Version != "v1" {
			continue
		}
		v1Kinds[objectKind.Kind] = true
	}

	apiResourceList := &metav1.APIResourceList{
		GroupVersion: configGroupVersion,
		APIResources: []metav1.APIResource{},
	}

	for kind := range v1Kinds {
		switch kind {
		case "Infrastructure":
			apiResourceList.APIResources = append(apiResourceList.APIResources, configV1InfrastructureAPIResources()...)
		case "Network":
			apiResourceList.APIResources = append(apiResourceList.APIResources, configV1NetworkAPIResources()...)
		}
	}

	fakeClient.Fake.Resources = []*metav1.APIResourceList{apiResourceList}

	return &ConfigClientShim{
		configClient: configClient,
		v1Kinds:      v1Kinds,
		fakeClient:   fakeClient,
	}
}
