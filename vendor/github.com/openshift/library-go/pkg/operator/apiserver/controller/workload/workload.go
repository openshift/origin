package workload

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	operatorv1 "github.com/openshift/api/operator/v1"
	openshiftconfigclientv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	clusteroperatorv1helpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	workQueueKey = "key"
)

// Delegate captures a set of methods that hold a custom logic
type Delegate interface {
	// Sync a method that will be used for delegation. It should bring the desired workload into operation.
	Sync() (*appsv1.Deployment, bool, []error)

	// PreconditionFulfilled a method that indicates whether all prerequisites are met and we can Sync.
	PreconditionFulfilled() (bool, error)
}

// Controller is a generic workload controller that deals with Deployment resource.
// Callers must provide a sync function for delegation. It should bring the desired workload into operation.
// The returned state along with errors will be converted into conditions and persisted in the status field.
type Controller struct {
	name string
	// conditionsPrefix an optional prefix that will be used as operator's condition type field for example APIServerDeploymentDegraded where APIServer indicates the prefix
	conditionsPrefix     string
	operatorNamespace    string
	targetNamespace      string
	targetOperandVersion string
	// operandNamePrefix is used to set the version for an operand via versionRecorder.SetVersion method
	operandNamePrefix string

	operatorClient               v1helpers.OperatorClient
	kubeClient                   kubernetes.Interface
	openshiftClusterConfigClient openshiftconfigclientv1.ClusterOperatorInterface

	delegate           Delegate
	queue              workqueue.RateLimitingInterface
	eventRecorder      events.Recorder
	versionRecorder    status.VersionGetter
	preRunCachesSynced []cache.InformerSynced
}

// NewController creates a brand new Controller instance.
//
// the "name" param will be used to set conditions in the status field. It will be suffixed with "WorkloadController",
// so it can end up in the condition in the form of "OAuthAPIWorkloadControllerDeploymentAvailable"
//
// the "operatorNamespace" is used to set "version-mapping" in the correct namespace
//
// the "targetNamespace" represent the namespace for the managed resource (DaemonSet)
func NewController(name, operatorNamespace, targetNamespace, targetOperandVersion, operandNamePrefix, conditionsPrefix string,
	operatorClient v1helpers.OperatorClient,
	kubeClient kubernetes.Interface,
	delegate Delegate,
	openshiftClusterConfigClient openshiftconfigclientv1.ClusterOperatorInterface,
	eventRecorder events.Recorder,
	versionRecorder status.VersionGetter) *Controller {
	controllerRef := &Controller{
		operatorNamespace:            operatorNamespace,
		name:                         fmt.Sprintf("%sWorkloadController", name),
		targetNamespace:              targetNamespace,
		targetOperandVersion:         targetOperandVersion,
		operandNamePrefix:            operandNamePrefix,
		conditionsPrefix:             conditionsPrefix,
		operatorClient:               operatorClient,
		kubeClient:                   kubeClient,
		delegate:                     delegate,
		openshiftClusterConfigClient: openshiftClusterConfigClient,
		eventRecorder:                eventRecorder.WithComponentSuffix("workload-controller"),
		versionRecorder:              versionRecorder,
		queue:                        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
	}

	return controllerRef
}

func (c *Controller) sync() error {
	operatorSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	if run, err := c.shouldSync(operatorSpec); !run {
		return err
	}

	if fulfilled, err := c.preconditionFulfilled(operatorSpec); !fulfilled {
		return err
	}

	if fulfilled, err := c.delegate.PreconditionFulfilled(); !fulfilled {
		return err
	}

	workload, operatorConfigAtHighestGeneration, errs := c.delegate.Sync()

	return c.updateOperatorStatus(workload, operatorConfigAtHighestGeneration, errs)
}

// Run starts workload controller and blocks until stopCh is closed.
// Note that setting workers doesn't have any effect, the controller is single-threaded.
func (c *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %s", c.name)
	defer klog.Infof("Shutting down %s", c.name)
	if !cache.WaitForCacheSync(ctx.Done(), c.preRunCachesSynced...) {
		utilruntime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, ctx.Done())

	<-ctx.Done()
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// AddInformer queues the given informer to check spec, status and managed resources
func (c *Controller) AddInformer(informer cache.SharedIndexInformer) *Controller {
	informer.AddEventHandler(c.eventHandler())
	c.preRunCachesSynced = append(c.preRunCachesSynced, informer.HasSynced)
	return c
}

// AddNamespaceInformer queues the given ns informer for the targetNamespace
func (c *Controller) AddNamespaceInformer(informer cache.SharedIndexInformer) *Controller {
	interestingNamespaces := sets.NewString(c.targetNamespace)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				c.queue.Add(workQueueKey)
			}
			if interestingNamespaces.Has(ns.Name) {
				c.queue.Add(workQueueKey)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			ns, ok := old.(*corev1.Namespace)
			if !ok {
				c.queue.Add(workQueueKey)
			}
			if interestingNamespaces.Has(ns.Name) {
				c.queue.Add(workQueueKey)
			}
		},
		DeleteFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
					return
				}
				ns, ok = tombstone.Obj.(*corev1.Namespace)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Namespace %#v", obj))
					return
				}
			}
			if interestingNamespaces.Has(ns.Name) {
				c.queue.Add(workQueueKey)
			}
		},
	})
	c.preRunCachesSynced = append(c.preRunCachesSynced, informer.HasSynced)

	return c
}

func (c *Controller) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}

// shouldSync checks ManagementState to determine if we can run this operator, probably set by a cluster administrator.
func (c *Controller) shouldSync(operatorSpec *operatorv1.OperatorSpec) (bool, error) {
	switch operatorSpec.ManagementState {
	case operatorv1.Managed:
		return true, nil
	case operatorv1.Unmanaged:
		return false, nil
	case operatorv1.Removed:
		if err := c.kubeClient.CoreV1().Namespaces().Delete(context.TODO(), c.targetNamespace, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	default:
		c.eventRecorder.Warningf("ManagementStateUnknown", "Unrecognized operator management state %q", operatorSpec.ManagementState)
		return false, nil
	}
}

// preconditionFulfilled checks if kube-apiserver is present and available
func (c *Controller) preconditionFulfilled(operatorSpec *operatorv1.OperatorSpec) (bool, error) {
	kubeAPIServerClusterOperator, err := c.openshiftClusterConfigClient.Get(context.TODO(), "kube-apiserver", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		message := "clusteroperator/kube-apiserver not found"
		c.eventRecorder.Warning("PrereqNotReady", message)
		return false, fmt.Errorf(message)
	}
	if err != nil {
		return false, err
	}
	if !clusteroperatorv1helpers.IsStatusConditionTrue(kubeAPIServerClusterOperator.Status.Conditions, "Available") {
		message := fmt.Sprintf("clusteroperator/%s is not Available", kubeAPIServerClusterOperator.Name)
		c.eventRecorder.Warning("PrereqNotReady", message)
		return false, fmt.Errorf(message)
	}

	return true, nil
}

// updateOperatorStatus updates the status based on the actual workload and errors that might have occurred during synchronization.
func (c *Controller) updateOperatorStatus(workload *appsv1.Deployment, operatorConfigAtHighestGeneration bool, errs []error) error {
	if errs == nil {
		errs = []error{}
	}

	deploymentAvailableCondition := operatorv1.OperatorCondition{
		Type:   fmt.Sprintf("%sDeployment%s", c.conditionsPrefix, operatorv1.OperatorStatusTypeAvailable),
		Status: operatorv1.ConditionTrue,
	}

	workloadDegradedCondition := operatorv1.OperatorCondition{
		Type:   fmt.Sprintf("%sWorkloadDegraded", c.conditionsPrefix),
		Status: operatorv1.ConditionFalse,
	}

	deploymentDegradedCondition := operatorv1.OperatorCondition{
		Type:   fmt.Sprintf("%sDeploymentDegraded", c.conditionsPrefix),
		Status: operatorv1.ConditionFalse,
	}

	deploymentProgressingCondition := operatorv1.OperatorCondition{
		Type:   fmt.Sprintf("%sDeployment%s", c.conditionsPrefix, operatorv1.OperatorStatusTypeProgressing),
		Status: operatorv1.ConditionFalse,
	}

	if len(errs) > 0 {
		message := ""
		for _, err := range errs {
			message = message + err.Error() + "\n"
		}
		workloadDegradedCondition.Status = operatorv1.ConditionTrue
		workloadDegradedCondition.Reason = "SyncError"
		workloadDegradedCondition.Message = message
	} else {
		workloadDegradedCondition.Status = operatorv1.ConditionFalse
	}

	if workload == nil {
		message := fmt.Sprintf("deployment/%s: could not be retrieved", c.targetNamespace)
		deploymentAvailableCondition.Status = operatorv1.ConditionFalse
		deploymentAvailableCondition.Reason = "NoDeployment"
		deploymentAvailableCondition.Message = message

		deploymentProgressingCondition.Status = operatorv1.ConditionTrue
		deploymentProgressingCondition.Reason = "NoDeployment"
		deploymentProgressingCondition.Message = message

		deploymentDegradedCondition.Status = operatorv1.ConditionTrue
		deploymentDegradedCondition.Reason = "NoDeployment"
		deploymentDegradedCondition.Message = message

		if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient,
			v1helpers.UpdateConditionFn(deploymentAvailableCondition),
			v1helpers.UpdateConditionFn(deploymentDegradedCondition),
			v1helpers.UpdateConditionFn(deploymentProgressingCondition),
			v1helpers.UpdateConditionFn(workloadDegradedCondition)); updateError != nil {
			return updateError
		}
		return errors.NewAggregate(errs)
	}

	if workload.Status.AvailableReplicas == 0 {
		deploymentAvailableCondition.Status = operatorv1.ConditionFalse
		deploymentAvailableCondition.Reason = "NoPod"
		deploymentAvailableCondition.Message = fmt.Sprintf("no %s.%s pods available on any node.", workload.Name, c.targetNamespace)
	} else {
		deploymentAvailableCondition.Status = operatorv1.ConditionTrue
		deploymentAvailableCondition.Reason = "AsExpected"
	}

	// If the workload is up to date, then we are no longer progressing
	workloadAtHighestGeneration := workload.ObjectMeta.Generation == workload.Status.ObservedGeneration
	if !workloadAtHighestGeneration {
		deploymentProgressingCondition.Status = operatorv1.ConditionTrue
		deploymentProgressingCondition.Reason = "NewGeneration"
		deploymentProgressingCondition.Message = fmt.Sprintf("deployment/%s.%s: observed generation is %d, desired generation is %d.", workload.Name, c.targetNamespace, workload.Status.ObservedGeneration, workload.ObjectMeta.Generation)
	} else {
		deploymentProgressingCondition.Status = operatorv1.ConditionFalse
		deploymentProgressingCondition.Reason = "AsExpected"
	}

	desiredReplicas := int32(1)
	if workload.Spec.Replicas != nil {
		desiredReplicas = *(workload.Spec.Replicas)
	}

	// During a rollout the default maxSurge (25%) will allow the available
	// replicas to temporarily exceed the desired replica count. If this were
	// to occur, the operator should not report degraded.
	workloadHasAllPodsAvailable := workload.Status.AvailableReplicas >= desiredReplicas
	if !workloadHasAllPodsAvailable {
		numNonAvailablePods := desiredReplicas - workload.Status.AvailableReplicas
		deploymentDegradedCondition.Status = operatorv1.ConditionTrue
		deploymentDegradedCondition.Reason = "UnavailablePod"
		deploymentDegradedCondition.Message = fmt.Sprintf("%v of %v requested instances are unavailable for %s.%s", numNonAvailablePods, desiredReplicas, workload.Name, c.targetNamespace)
	} else {
		deploymentDegradedCondition.Status = operatorv1.ConditionFalse
		deploymentDegradedCondition.Reason = "AsExpected"
	}

	// if the deployment is all available and at the expected generation, then update the version to the latest
	// when we update, the image pull spec should immediately be different, which should immediately cause a deployment rollout
	// which should immediately result in a deployment generation diff, which should cause this block to be skipped until it is ready.
	workloadHasAllPodsUpdated := workload.Status.UpdatedReplicas == desiredReplicas
	if workloadAtHighestGeneration && workloadHasAllPodsAvailable && workloadHasAllPodsUpdated && operatorConfigAtHighestGeneration {
		c.versionRecorder.SetVersion(fmt.Sprintf("%s-%s", c.operandNamePrefix, workload.Name), c.targetOperandVersion)
	}

	updateGenerationFn := func(newStatus *operatorv1.OperatorStatus) error {
		resourcemerge.SetDeploymentGeneration(&newStatus.Generations, workload)
		return nil
	}

	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient,
		v1helpers.UpdateConditionFn(deploymentAvailableCondition),
		v1helpers.UpdateConditionFn(deploymentDegradedCondition),
		v1helpers.UpdateConditionFn(deploymentProgressingCondition),
		v1helpers.UpdateConditionFn(workloadDegradedCondition),
		updateGenerationFn); updateError != nil {
		return updateError
	}

	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

// EnsureAtMostOnePodPerNode updates the deployment spec to prevent more than
// one pod of a given replicaset from landing on a node. It accomplishes this
// by adding a uuid as a label on the template and updates the pod
// anti-affinity term to include that label. Since the deployment is only
// written (via ApplyDeployment) when the metadata differs or the generations
// don't match, the uuid should only be updated in the API when a new
// replicaset is created.
func EnsureAtMostOnePodPerNode(spec *appsv1.DeploymentSpec) error {
	uuidKey := "anti-affinity-uuid"
	uuidValue := uuid.New().String()

	// Label the pod template with the template hash
	spec.Template.Labels[uuidKey] = uuidValue

	// Ensure that match labels are defined
	if spec.Selector == nil {
		return fmt.Errorf("deployment is missing spec.selector")
	}
	if len(spec.Selector.MatchLabels) == 0 {
		return fmt.Errorf("deployment is missing spec.selector.matchLabels")
	}

	// Ensure anti-affinity selects on the uuid
	antiAffinityMatchLabels := map[string]string{
		uuidKey: uuidValue,
	}
	// Ensure anti-affinity selects on the same labels as the deployment
	for key, value := range spec.Selector.MatchLabels {
		antiAffinityMatchLabels[key] = value
	}

	// Add an anti-affinity rule to the pod template that precludes more than
	// one pod for a uuid from being scheduled to a node.
	spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					TopologyKey: "kubernetes.io/hostname",
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: antiAffinityMatchLabels,
					},
				},
			},
		},
	}

	return nil
}

// CountNodesFuncWrapper returns a function that returns the number of nodes that match the given
// selector. This supports determining the number of master nodes to
// allow setting the deployment replica count to match.
func CountNodesFuncWrapper(nodeLister corev1listers.NodeLister) func(nodeSelector map[string]string) (*int32, error) {
	return func(nodeSelector map[string]string) (*int32, error) {
		nodes, err := nodeLister.List(labels.SelectorFromSet(nodeSelector))
		if err != nil {
			return nil, err
		}
		replicas := int32(len(nodes))
		return &replicas, nil
	}
}
