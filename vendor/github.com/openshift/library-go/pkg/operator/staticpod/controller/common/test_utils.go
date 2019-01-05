package common

import (
	"fmt"
	"strconv"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	corelisterv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// NewFakeSharedIndexInformer returns a fake shared index informer, suitable to use in static pod controller unit tests.
func NewFakeSharedIndexInformer() cache.SharedIndexInformer {
	return &fakeSharedIndexInformer{}
}

type fakeSharedIndexInformer struct{}

func (fakeSharedIndexInformer) AddEventHandler(handler cache.ResourceEventHandler) {
}

func (fakeSharedIndexInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) {
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
	panic("implement me")
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

// NewFakeStaticPodOperatorClient returns a fake operator client suitable to use in static pod controller unit tests.
func NewFakeStaticPodOperatorClient(spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus,
	staticPodStatus *operatorv1.StaticPodOperatorStatus, triggerErr func(rv string, status *operatorv1.StaticPodOperatorStatus) error) OperatorClient {
	return &fakeStaticPodOperatorClient{
		fakeOperatorSpec:            spec,
		fakeOperatorStatus:          status,
		fakeStaticPodOperatorStatus: staticPodStatus,
		resourceVersion:             "0",
		triggerStatusUpdateError:    triggerErr,
	}
}

type fakeStaticPodOperatorClient struct {
	fakeOperatorSpec            *operatorv1.OperatorSpec
	fakeOperatorStatus          *operatorv1.OperatorStatus
	fakeStaticPodOperatorStatus *operatorv1.StaticPodOperatorStatus
	resourceVersion             string
	triggerStatusUpdateError    func(rv string, status *operatorv1.StaticPodOperatorStatus) error
}

func (c *fakeStaticPodOperatorClient) Informer() cache.SharedIndexInformer {
	return &fakeSharedIndexInformer{}
}

func (c *fakeStaticPodOperatorClient) Get() (*operatorv1.OperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error) {
	return c.fakeOperatorSpec, c.fakeStaticPodOperatorStatus, c.resourceVersion, nil
}

func (c *fakeStaticPodOperatorClient) UpdateStatus(resourceVersion string, status *operatorv1.StaticPodOperatorStatus) (*operatorv1.StaticPodOperatorStatus, error) {
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

func (c *fakeStaticPodOperatorClient) CurrentStatus() (operatorv1.OperatorStatus, error) {
	return *c.fakeOperatorStatus, nil
}

// NewFakeNodeLister returns a fake node lister suitable to use in node controller unit test
func NewFakeNodeLister(client kubernetes.Interface) corelisterv1.NodeLister {
	return &fakeNodeLister{client: client}
}

type fakeNodeLister struct {
	client kubernetes.Interface
}

func (n *fakeNodeLister) List(selector labels.Selector) ([]*v1.Node, error) {
	nodes, err := n.client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	ret := []*v1.Node{}
	for i := range nodes.Items {
		ret = append(ret, &nodes.Items[i])
	}
	return ret, nil
}

func (n *fakeNodeLister) Get(name string) (*v1.Node, error) {
	panic("implement me")
}

func (n *fakeNodeLister) ListWithPredicate(predicate corelisterv1.NodeConditionPredicate) ([]*v1.Node, error) {
	panic("implement me")
}
