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
	"k8s.io/utils/ptr"

	operatorv1 "github.com/openshift/api/operator/v1"
	v1 "github.com/openshift/api/operator/v1"
	applyoperatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	"github.com/openshift/library-go/pkg/apiserver/jsonpatch"
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
	triggerSpecErr func(rv string, spec *operatorv1.StaticPodOperatorSpec) error) *fakeStaticPodOperatorClient {
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

	patchedOperatorStatus *jsonpatch.PatchSet
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

func (c *fakeStaticPodOperatorClient) GetLiveStaticPodOperatorState() (*operatorv1.StaticPodOperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error) {
	return c.GetStaticPodOperatorState()
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

func (c *fakeStaticPodOperatorClient) ApplyOperatorSpec(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.OperatorSpecApplyConfiguration) (err error) {
	return nil
}

func (c *fakeStaticPodOperatorClient) ApplyOperatorStatus(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.OperatorStatusApplyConfiguration) (err error) {
	if c.triggerStatusUpdateError != nil {
		operatorStatus := &operatorv1.StaticPodOperatorStatus{OperatorStatus: *mergeOperatorStatusApplyConfiguration(&c.fakeStaticPodOperatorStatus.OperatorStatus, applyConfiguration)}
		if err := c.triggerStatusUpdateError("", operatorStatus); err != nil {
			return err
		}
	}
	c.fakeStaticPodOperatorStatus = &operatorv1.StaticPodOperatorStatus{
		OperatorStatus: *mergeOperatorStatusApplyConfiguration(&c.fakeStaticPodOperatorStatus.OperatorStatus, applyConfiguration),
	}
	return nil
}

func (c *fakeStaticPodOperatorClient) ApplyStaticPodOperatorSpec(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.StaticPodOperatorSpecApplyConfiguration) (err error) {
	return nil
}

func (c *fakeStaticPodOperatorClient) ApplyStaticPodOperatorStatus(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.StaticPodOperatorStatusApplyConfiguration) (err error) {
	if c.triggerStatusUpdateError != nil {
		operatorStatus := mergeStaticPodOperatorStatusApplyConfiguration(&c.fakeStaticPodOperatorStatus.OperatorStatus, applyConfiguration)
		if err := c.triggerStatusUpdateError("", operatorStatus); err != nil {
			return err
		}
	}
	c.fakeStaticPodOperatorStatus = mergeStaticPodOperatorStatusApplyConfiguration(&c.fakeStaticPodOperatorStatus.OperatorStatus, applyConfiguration)
	return nil
}

func (c *fakeStaticPodOperatorClient) PatchOperatorStatus(ctx context.Context, jsonPatch *jsonpatch.PatchSet) (err error) {
	return nil
}

func (c *fakeStaticPodOperatorClient) PatchStaticOperatorStatus(ctx context.Context, jsonPatch *jsonpatch.PatchSet) (err error) {
	if c.triggerStatusUpdateError != nil {
		return c.triggerStatusUpdateError("", nil)
	}
	c.patchedOperatorStatus = jsonPatch
	return nil
}

func (c *fakeStaticPodOperatorClient) GetPatchedOperatorStatus() *jsonpatch.PatchSet {
	return c.patchedOperatorStatus
}

func (c *fakeStaticPodOperatorClient) GetOperatorState() (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	return &c.fakeStaticPodOperatorSpec.OperatorSpec, &c.fakeStaticPodOperatorStatus.OperatorStatus, c.resourceVersion, nil
}
func (c *fakeStaticPodOperatorClient) GetOperatorStateWithQuorum(ctx context.Context) (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	return c.GetOperatorState()
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
func NewFakeOperatorClient(spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, triggerErr func(rv string, status *operatorv1.OperatorStatus) error) *fakeOperatorClient {
	return NewFakeOperatorClientWithObjectMeta(nil, spec, status, triggerErr)
}

func NewFakeOperatorClientWithObjectMeta(meta *metav1.ObjectMeta, spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, triggerErr func(rv string, status *operatorv1.OperatorStatus) error) *fakeOperatorClient {
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

	patchedOperatorStatus *jsonpatch.PatchSet
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

func (c *fakeOperatorClient) GetOperatorStateWithQuorum(ctx context.Context) (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	return c.GetOperatorState()
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

func (c *fakeOperatorClient) ApplyOperatorSpec(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.OperatorSpecApplyConfiguration) (err error) {
	return nil
}

func (c *fakeOperatorClient) ApplyOperatorStatus(ctx context.Context, fieldManager string, applyConfiguration *applyoperatorv1.OperatorStatusApplyConfiguration) (err error) {
	c.fakeOperatorStatus = mergeOperatorStatusApplyConfiguration(c.fakeOperatorStatus, applyConfiguration)
	return nil
}

func (c *fakeOperatorClient) PatchOperatorStatus(ctx context.Context, jsonPatch *jsonpatch.PatchSet) (err error) {
	if c.triggerStatusUpdateError != nil {
		return c.triggerStatusUpdateError("", nil)
	}
	c.patchedOperatorStatus = jsonPatch
	return nil
}

func (c *fakeOperatorClient) GetPatchedOperatorStatus() *jsonpatch.PatchSet {
	return c.patchedOperatorStatus
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

func mergeOperatorStatusApplyConfiguration(currentOperatorStatus *v1.OperatorStatus, applyConfiguration *applyoperatorv1.OperatorStatusApplyConfiguration) *v1.OperatorStatus {
	status := &v1.OperatorStatus{
		ObservedGeneration:      ptr.Deref(applyConfiguration.ObservedGeneration, currentOperatorStatus.ObservedGeneration),
		Version:                 ptr.Deref(applyConfiguration.Version, currentOperatorStatus.Version),
		ReadyReplicas:           ptr.Deref(applyConfiguration.ReadyReplicas, currentOperatorStatus.ReadyReplicas),
		LatestAvailableRevision: ptr.Deref(applyConfiguration.LatestAvailableRevision, currentOperatorStatus.LatestAvailableRevision),
	}

	for _, condition := range applyConfiguration.Conditions {
		newCondition := operatorv1.OperatorCondition{
			Type:    ptr.Deref(condition.Type, ""),
			Status:  ptr.Deref(condition.Status, ""),
			Reason:  ptr.Deref(condition.Reason, ""),
			Message: ptr.Deref(condition.Message, ""),
		}
		status.Conditions = append(status.Conditions, newCondition)
	}
	var existingConditions []v1.OperatorCondition
	for _, condition := range currentOperatorStatus.Conditions {
		var foundCondition bool
		for _, statusCondition := range status.Conditions {
			if condition.Type == statusCondition.Type {
				foundCondition = true
				break
			}
		}
		if !foundCondition {
			existingConditions = append(existingConditions, condition)
		}
	}
	status.Conditions = append(status.Conditions, existingConditions...)

	for _, generation := range applyConfiguration.Generations {
		newGeneration := operatorv1.GenerationStatus{
			Group:          ptr.Deref(generation.Group, ""),
			Resource:       ptr.Deref(generation.Resource, ""),
			Namespace:      ptr.Deref(generation.Namespace, ""),
			Name:           ptr.Deref(generation.Name, ""),
			LastGeneration: ptr.Deref(generation.LastGeneration, 0),
			Hash:           ptr.Deref(generation.Hash, ""),
		}
		status.Generations = append(status.Generations, newGeneration)
	}
	var existingGenerations []v1.GenerationStatus
	for _, generation := range currentOperatorStatus.Generations {
		var foundGeneration bool
		for _, statusGeneration := range status.Generations {
			if generation.Namespace == statusGeneration.Namespace && generation.Name == statusGeneration.Name {
				foundGeneration = true
				break
			}
		}
		if !foundGeneration {
			existingGenerations = append(existingGenerations, generation)
		}
	}
	status.Generations = append(status.Generations, existingGenerations...)

	return status
}

func mergeStaticPodOperatorStatusApplyConfiguration(currentOperatorStatus *v1.OperatorStatus, applyConfiguration *applyoperatorv1.StaticPodOperatorStatusApplyConfiguration) *v1.StaticPodOperatorStatus {
	status := &v1.StaticPodOperatorStatus{
		OperatorStatus: *mergeOperatorStatusApplyConfiguration(currentOperatorStatus, &applyConfiguration.OperatorStatusApplyConfiguration),
	}

	for _, nodeStatus := range applyConfiguration.NodeStatuses {
		newNodeStatus := operatorv1.NodeStatus{
			NodeName:                 ptr.Deref(nodeStatus.NodeName, ""),
			CurrentRevision:          ptr.Deref(nodeStatus.CurrentRevision, 0),
			TargetRevision:           ptr.Deref(nodeStatus.TargetRevision, 0),
			LastFailedRevision:       ptr.Deref(nodeStatus.LastFailedRevision, 0),
			LastFailedTime:           nil,
			LastFailedReason:         ptr.Deref(nodeStatus.LastFailedReason, ""),
			LastFailedCount:          ptr.Deref(nodeStatus.LastFailedCount, 0),
			LastFallbackCount:        ptr.Deref(nodeStatus.LastFallbackCount, 0),
			LastFailedRevisionErrors: nil,
		}
		if nodeStatus.LastFailedTime != nil {
			newNodeStatus.LastFailedTime = nodeStatus.LastFailedTime
		}
		for _, curr := range nodeStatus.LastFailedRevisionErrors {
			newNodeStatus.LastFailedRevisionErrors = append(newNodeStatus.LastFailedRevisionErrors, curr)
		}
		status.NodeStatuses = append(status.NodeStatuses, newNodeStatus)
	}

	return status
}
