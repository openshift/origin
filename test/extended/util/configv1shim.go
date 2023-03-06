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

	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

// ConfigClientShim makes sure whenever there's a static
// manifest present for a config v1 kind, fake client is used
// instead of the real one.
type ConfigClientShim struct {
	configClient configv1client.Interface
	v1Kinds      map[string]bool
	fakeClient   *fakeconfigv1client.Clientset
}

func (c *ConfigClientShim) Discovery() discovery.DiscoveryInterface {
	return c.configClient.Discovery()
}
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

var _ configv1client.Interface = &ConfigClientShim{}

// ConfigClientShim makes sure whenever there's a static
// manifest present for a config v1 kind, fake client is used
// instead of the real one.
type ConfigV1ClientShim struct {
	configv1           configv1.ConfigV1Interface
	v1Kinds            map[string]bool
	fakeConfigV1Client configv1.ConfigV1Interface
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

	// INFO: field selectors will not work
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

func (c *ConfigV1NetworksClientShim) Create(ctx context.Context, infrastructure *apiconfigv1.Network, opts metav1.CreateOptions) (*apiconfigv1.Network, error) {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "create"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Create(ctx, infrastructure, opts)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) Update(ctx context.Context, infrastructure *apiconfigv1.Network, opts metav1.UpdateOptions) (*apiconfigv1.Network, error) {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "update"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.Update(ctx, infrastructure, opts)
	}
	return nil, err
}

func (c *ConfigV1NetworksClientShim) UpdateStatus(ctx context.Context, infrastructure *apiconfigv1.Network, opts metav1.UpdateOptions) (*apiconfigv1.Network, error) {
	_, err := c.fakeConfigV1NetworksClient.Get(ctx, infrastructure.Name, metav1.GetOptions{})
	if err == nil {
		return nil, &OperationNotPermitted{Action: "updatestatus"}
	}
	if apierrors.IsNotFound(err) {
		return c.configV1NetworksClient.UpdateStatus(ctx, infrastructure, opts)
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

	// INFO: field selectors will not work
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

	return &ConfigClientShim{
		configClient: configClient,
		v1Kinds:      v1Kinds,
		fakeClient:   fakeClient,
	}
}
