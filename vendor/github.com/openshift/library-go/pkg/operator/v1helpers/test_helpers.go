package v1helpers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// NewFakeSharedIndexInformer returns a fake shared index informer, suitable to use in static pod controller unit tests.
func NewFakeSharedIndexInformer() cache.SharedIndexInformer {
	return &fakeSharedIndexInformer{}
}

type fakeSharedIndexInformer struct{}

func (i fakeSharedIndexInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (i fakeSharedIndexInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (i fakeSharedIndexInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	panic("implement me")
}

func (i fakeSharedIndexInformer) IsStopped() bool {
	panic("implement me")
}

func (fakeSharedIndexInformer) GetStore() cache.Store {
	panic("implement me")
}

func (fakeSharedIndexInformer) GetController() cache.Controller {
	panic("implement me")
}

func (fakeSharedIndexInformer) Run(stopCh <-chan struct{}) {
	panic("implement me")
}

func (fakeSharedIndexInformer) HasSynced() bool {
	return true
}

func (fakeSharedIndexInformer) LastSyncResourceVersion() string {
	panic("implement me")
}

func (fakeSharedIndexInformer) AddIndexers(indexers cache.Indexers) error {
	panic("implement me")
}

func (fakeSharedIndexInformer) GetIndexer() cache.Indexer {
	panic("implement me")
}

func (fakeSharedIndexInformer) SetWatchErrorHandler(handler cache.WatchErrorHandler) error {
	panic("implement me")
}

func (fakeSharedIndexInformer) SetTransform(f cache.TransformFunc) error {
	panic("implement me")
}

// NewFakeStaticPodOperatorClient returns a fake operator client suitable to use in static pod controller unit tests.
func NewFakeStaticPodOperatorClient(
	staticPodSpec *operatorv1.StaticPodOperatorSpec, staticPodStatus *operatorv1.StaticPodOperatorStatus,
	triggerStatusErr func(rv string, status *operatorv1.StaticPodOperatorStatus) error,
	triggerSpecErr func(rv string, spec *operatorv1.StaticPodOperatorSpec) error) StaticPodOperatorClient {
	return &fakeStaticPodOperatorClient{
		fakeStaticPodOperatorSpec:   staticPodSpec,
		fakeStaticPodOperatorStatus: staticPodStatus,
		resourceVersion:             "0",
		triggerStatusUpdateError:    triggerStatusErr,
		triggerSpecUpdateError:      triggerSpecErr,
	}
}

type fakeStaticPodOperatorClient struct {
	fakeStaticPodOperatorSpec   *operatorv1.StaticPodOperatorSpec
	fakeStaticPodOperatorStatus *operatorv1.StaticPodOperatorStatus
	resourceVersion             string
	triggerStatusUpdateError    func(rv string, status *operatorv1.StaticPodOperatorStatus) error
	triggerSpecUpdateError      func(rv string, status *operatorv1.StaticPodOperatorSpec) error
}

func (c *fakeStaticPodOperatorClient) Informer() cache.SharedIndexInformer {
	return &fakeSharedIndexInformer{}

}
func (c *fakeStaticPodOperatorClient) GetObjectMeta() (*metav1.ObjectMeta, error) {
	panic("not supported")
}

func (c *fakeStaticPodOperatorClient) GetStaticPodOperatorState() (*operatorv1.StaticPodOperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error) {
	return c.fakeStaticPodOperatorSpec, c.fakeStaticPodOperatorStatus, c.resourceVersion, nil
}

func (c *fakeStaticPodOperatorClient) GetStaticPodOperatorStateWithQuorum(ctx context.Context) (*operatorv1.StaticPodOperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error) {
	return c.fakeStaticPodOperatorSpec, c.fakeStaticPodOperatorStatus, c.resourceVersion, nil
}

func (c *fakeStaticPodOperatorClient) UpdateStaticPodOperatorStatus(ctx context.Context, resourceVersion string, status *operatorv1.StaticPodOperatorStatus) (*operatorv1.StaticPodOperatorStatus, error) {
	if c.resourceVersion != resourceVersion {
		return nil, errors.NewConflict(schema.GroupResource{Group: operatorv1.GroupName, Resource: "TestOperatorConfig"}, "instance", fmt.Errorf("invalid resourceVersion"))
	}
	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return nil, err
	}
	c.resourceVersion = strconv.Itoa(rv + 1)
	if c.triggerStatusUpdateError != nil {
		if err := c.triggerStatusUpdateError(resourceVersion, status); err != nil {
			return nil, err
		}
	}
	c.fakeStaticPodOperatorStatus = status
	return c.fakeStaticPodOperatorStatus, nil
}

func (c *fakeStaticPodOperatorClient) UpdateStaticPodOperatorSpec(ctx context.Context, resourceVersion string, spec *operatorv1.StaticPodOperatorSpec) (*operatorv1.StaticPodOperatorSpec, string, error) {
	if c.resourceVersion != resourceVersion {
		return nil, "", errors.NewConflict(schema.GroupResource{Group: operatorv1.GroupName, Resource: "TestOperatorConfig"}, "instance", fmt.Errorf("invalid resourceVersion"))
	}
	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return nil, "", err
	}
	c.resourceVersion = strconv.Itoa(rv + 1)
	if c.triggerSpecUpdateError != nil {
		if err := c.triggerSpecUpdateError(resourceVersion, spec); err != nil {
			return nil, "", err
		}
	}
	c.fakeStaticPodOperatorSpec = spec
	return c.fakeStaticPodOperatorSpec, c.resourceVersion, nil
}

func (c *fakeStaticPodOperatorClient) GetOperatorState() (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	return &c.fakeStaticPodOperatorSpec.OperatorSpec, &c.fakeStaticPodOperatorStatus.OperatorStatus, c.resourceVersion, nil
}
func (c *fakeStaticPodOperatorClient) UpdateOperatorSpec(ctx context.Context, s string, p *operatorv1.OperatorSpec) (spec *operatorv1.OperatorSpec, resourceVersion string, err error) {
	panic("not supported")
}
func (c *fakeStaticPodOperatorClient) UpdateOperatorStatus(ctx context.Context, resourceVersion string, status *operatorv1.OperatorStatus) (*operatorv1.OperatorStatus, error) {
	if c.resourceVersion != resourceVersion {
		return nil, errors.NewConflict(schema.GroupResource{Group: operatorv1.GroupName, Resource: "TestOperatorConfig"}, "instance", fmt.Errorf("invalid resourceVersion"))
	}
	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return nil, err
	}
	c.resourceVersion = strconv.Itoa(rv + 1)
	if c.triggerStatusUpdateError != nil {
		staticPodStatus := c.fakeStaticPodOperatorStatus.DeepCopy()
		staticPodStatus.OperatorStatus = *status
		if err := c.triggerStatusUpdateError(resourceVersion, staticPodStatus); err != nil {
			return nil, err
		}
	}
	c.fakeStaticPodOperatorStatus.OperatorStatus = *status
	return &c.fakeStaticPodOperatorStatus.OperatorStatus, nil
}

// NewFakeNodeLister returns a fake node lister suitable to use in node controller unit test
func NewFakeNodeLister(client kubernetes.Interface) corev1listers.NodeLister {
	return &fakeNodeLister{client: client}
}

type fakeNodeLister struct {
	client kubernetes.Interface
}

func (n *fakeNodeLister) List(selector labels.Selector) ([]*corev1.Node, error) {
	nodes, err := n.client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	ret := []*corev1.Node{}
	for i := range nodes.Items {
		ret = append(ret, &nodes.Items[i])
	}
	return ret, nil
}

func (n *fakeNodeLister) Get(name string) (*corev1.Node, error) {
	panic("implement me")
}

// NewFakeOperatorClient returns a fake operator client suitable to use in static pod controller unit tests.
func NewFakeOperatorClient(spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, triggerErr func(rv string, status *operatorv1.OperatorStatus) error) OperatorClientWithFinalizers {
	return NewFakeOperatorClientWithObjectMeta(nil, spec, status, triggerErr)
}

func NewFakeOperatorClientWithObjectMeta(meta *metav1.ObjectMeta, spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, triggerErr func(rv string, status *operatorv1.OperatorStatus) error) OperatorClientWithFinalizers {
	return &fakeOperatorClient{
		fakeOperatorSpec:         spec,
		fakeOperatorStatus:       status,
		fakeObjectMeta:           meta,
		resourceVersion:          "0",
		triggerStatusUpdateError: triggerErr,
	}
}

type fakeOperatorClient struct {
	fakeOperatorSpec         *operatorv1.OperatorSpec
	fakeOperatorStatus       *operatorv1.OperatorStatus
	fakeObjectMeta           *metav1.ObjectMeta
	resourceVersion          string
	triggerStatusUpdateError func(rv string, status *operatorv1.OperatorStatus) error
}

func (c *fakeOperatorClient) Informer() cache.SharedIndexInformer {
	return &fakeSharedIndexInformer{}
}

func (c *fakeOperatorClient) GetObjectMeta() (*metav1.ObjectMeta, error) {
	if c.fakeObjectMeta == nil {
		return &metav1.ObjectMeta{}, nil
	}

	return c.fakeObjectMeta, nil
}

func (c *fakeOperatorClient) GetOperatorState() (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	return c.fakeOperatorSpec, c.fakeOperatorStatus, c.resourceVersion, nil
}

func (c *fakeOperatorClient) UpdateOperatorStatus(ctx context.Context, resourceVersion string, status *operatorv1.OperatorStatus) (*operatorv1.OperatorStatus, error) {
	if c.resourceVersion != resourceVersion {
		return nil, errors.NewConflict(schema.GroupResource{Group: operatorv1.GroupName, Resource: "TestOperatorConfig"}, "instance", fmt.Errorf("invalid resourceVersion"))
	}
	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return nil, err
	}
	c.resourceVersion = strconv.Itoa(rv + 1)
	if c.triggerStatusUpdateError != nil {
		if err := c.triggerStatusUpdateError(resourceVersion, status); err != nil {
			return nil, err
		}
	}
	c.fakeOperatorStatus = status
	return c.fakeOperatorStatus, nil
}

func (c *fakeOperatorClient) UpdateOperatorSpec(ctx context.Context, resourceVersion string, spec *operatorv1.OperatorSpec) (*operatorv1.OperatorSpec, string, error) {
	if c.resourceVersion != resourceVersion {
		return nil, c.resourceVersion, errors.NewConflict(schema.GroupResource{Group: operatorv1.GroupName, Resource: "TestOperatorConfig"}, "instance", fmt.Errorf("invalid resourceVersion"))
	}
	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return nil, c.resourceVersion, err
	}
	c.resourceVersion = strconv.Itoa(rv + 1)
	c.fakeOperatorSpec = spec
	return c.fakeOperatorSpec, c.resourceVersion, nil
}

func (c *fakeOperatorClient) EnsureFinalizer(ctx context.Context, finalizer string) error {
	if c.fakeObjectMeta == nil {
		c.fakeObjectMeta = &metav1.ObjectMeta{}
	}
	for _, f := range c.fakeObjectMeta.Finalizers {
		if f == finalizer {
			return nil
		}
	}
	c.fakeObjectMeta.Finalizers = append(c.fakeObjectMeta.Finalizers, finalizer)
	return nil
}

func (c *fakeOperatorClient) RemoveFinalizer(ctx context.Context, finalizer string) error {
	newFinalizers := []string{}
	for _, f := range c.fakeObjectMeta.Finalizers {
		if f == finalizer {
			continue
		}
		newFinalizers = append(newFinalizers, f)
	}
	c.fakeObjectMeta.Finalizers = newFinalizers
	return nil
}

func (c *fakeOperatorClient) SetObjectMeta(meta *metav1.ObjectMeta) {
	c.fakeObjectMeta = meta
}
