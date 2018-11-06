package installer

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
)

const installerControllerWorkQueueKey = "key"

type InstallerController struct {
	targetNamespace string
	// configMaps is the list of configmaps that are directly copied.A different actor/controller modifies these.
	// the first element should be the configmap that contains the static pod manifest
	configMaps []string
	// secrets is a list of secrets that are directly copied for the current values.  A different actor/controller modifies these.
	secrets []string
	// command is the string to use for the installer pod command
	command []string

	operatorConfigClient common.OperatorClient

	kubeClient kubernetes.Interface

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	// installerPodImageFn returns the image name for the installer pod
	installerPodImageFn func() string
}

func NewInstallerController(
	targetNamespace string,
	configMaps []string,
	secrets []string,
	command []string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorConfigClient common.OperatorClient,
	kubeClient kubernetes.Interface,
) *InstallerController {
	c := &InstallerController{
		targetNamespace: targetNamespace,
		configMaps:      configMaps,
		secrets:         secrets,
		command:         command,

		operatorConfigClient: operatorConfigClient,
		kubeClient:           kubeClient,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "InstallerController"),

		installerPodImageFn: getInstallerPodImageFromEnv,
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().Pods().Informer().AddEventHandler(c.eventHandler())

	return c
}

// createInstallerController_v311_00_to_latest takes care of creating content for the static pods to deploy.
// returns whether or not requeue and if an error happened when updating status.  Normally it updates status itself.
func (c *InstallerController) createInstallerController(operatorSpec *operatorv1alpha1.OperatorSpec, originalOperatorStatus *operatorv1alpha1.StaticPodOperatorStatus, resourceVersion string) (bool, error) {
	operatorStatus := originalOperatorStatus.DeepCopy()

	for i := range operatorStatus.NodeStatuses {
		var currNodeState *operatorv1alpha1.NodeStatus
		var prevNodeState *operatorv1alpha1.NodeStatus
		currNodeState = &operatorStatus.NodeStatuses[i]
		if i > 0 {
			prevNodeState = &operatorStatus.NodeStatuses[i-1]
		}

		// if we are in a transition, check to see if our installer pod completed
		if currNodeState.TargetDeploymentGeneration > currNodeState.CurrentDeploymentGeneration {
			// TODO check to see if our installer pod completed.  Success or failure there indicates whether we should be failed.
			newCurrNodeState, err := c.newNodeStateForInstallInProgress(currNodeState)
			if err != nil {
				return true, err
			}

			// if we make a change to this status, we want to write it out to the API before we commence work on the next node.
			// it's an extra write/read, but it makes the state debuggable from outside this process
			if !equality.Semantic.DeepEqual(newCurrNodeState, currNodeState) {
				glog.Infof("%q moving to %v", currNodeState.NodeName, spew.Sdump(*newCurrNodeState))
				operatorStatus.NodeStatuses[i] = *newCurrNodeState
				if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
					_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
					return false, updateError
				}

			} else {
				glog.V(2).Infof("%q is in transition to %d, but has not made progress", currNodeState.NodeName, currNodeState.TargetDeploymentGeneration)
			}
			break
		}

		deploymentIDToStart := c.getDeploymentIDToStart(currNodeState, prevNodeState, operatorStatus)
		if deploymentIDToStart == 0 {
			glog.V(4).Infof("%q does not need update", currNodeState.NodeName)
			continue
		}
		glog.Infof("%q needs to deploy to %d", currNodeState.NodeName, deploymentIDToStart)

		// we need to start a deployment create a pod that will lay down the static pod resources
		newCurrNodeState, err := c.createInstallerPod(currNodeState, operatorSpec, deploymentIDToStart)
		if err != nil {
			return true, err
		}
		// if we make a change to this status, we want to write it out to the API before we commence work on the next node.
		// it's an extra write/read, but it makes the state debuggable from outside this process
		if !equality.Semantic.DeepEqual(newCurrNodeState, currNodeState) {
			glog.Infof("%q moving to %v", currNodeState.NodeName, spew.Sdump(*newCurrNodeState))
			operatorStatus.NodeStatuses[i] = *newCurrNodeState
			if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
				_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
				return false, updateError
			}
		}
		break
	}

	v1alpha1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1alpha1.OperatorCondition{
		Type:   "InstallerControllerFailing",
		Status: operatorv1alpha1.ConditionFalse,
	})
	if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
		_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
		if updateError != nil {
			return true, updateError
		}
	}

	return false, nil
}

// newNodeStateForInstallInProgress returns the new NodeState or error
func (c *InstallerController) newNodeStateForInstallInProgress(currNodeState *operatorv1alpha1.NodeStatus) (*operatorv1alpha1.NodeStatus, error) {
	ret := currNodeState.DeepCopy()
	installerPod, err := c.kubeClient.CoreV1().Pods(c.targetNamespace).Get(getInstallerPodName(currNodeState.TargetDeploymentGeneration, currNodeState.NodeName), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ret.LastFailedDeploymentGeneration = currNodeState.TargetDeploymentGeneration
		ret.TargetDeploymentGeneration = currNodeState.CurrentDeploymentGeneration
		ret.LastFailedDeploymentErrors = []string{err.Error()}
		return ret, nil
	}
	if err != nil {
		return nil, err
	}
	switch installerPod.Status.Phase {
	case corev1.PodSucceeded:
		ret.CurrentDeploymentGeneration = currNodeState.TargetDeploymentGeneration
		ret.TargetDeploymentGeneration = 0
		ret.LastFailedDeploymentGeneration = 0
		ret.LastFailedDeploymentErrors = nil
	case corev1.PodFailed:
		ret.LastFailedDeploymentGeneration = currNodeState.TargetDeploymentGeneration
		ret.TargetDeploymentGeneration = 0

		errors := []string{}
		for _, containerStatus := range installerPod.Status.ContainerStatuses {
			if containerStatus.State.Terminated != nil && len(containerStatus.State.Terminated.Message) > 0 {
				errors = append(errors, fmt.Sprintf("%s: %s", containerStatus.Name, containerStatus.State.Terminated.Message))
			}
		}
		if len(errors) == 0 {
			errors = append(errors, "no detailed termination message, see `oc get -n %q pods/%q -oyaml`", installerPod.Namespace, installerPod.Name)
		}
		ret.LastFailedDeploymentErrors = errors
	}

	return ret, nil
}

// getDeploymentIDToStart returns the deploymentID we need to start or zero if none
func (c *InstallerController) getDeploymentIDToStart(currNodeState, prevNodeState *operatorv1alpha1.NodeStatus, operatorStatus *operatorv1alpha1.StaticPodOperatorStatus) int32 {
	if prevNodeState == nil {
		currentAtLatest := currNodeState.CurrentDeploymentGeneration == operatorStatus.LatestAvailableDeploymentGeneration
		failedAtLatest := currNodeState.LastFailedDeploymentGeneration == operatorStatus.LatestAvailableDeploymentGeneration
		if !currentAtLatest && !failedAtLatest {
			return operatorStatus.LatestAvailableDeploymentGeneration
		}
		return 0
	}

	prevFinished := prevNodeState.TargetDeploymentGeneration == 0
	prevInTransition := prevNodeState.CurrentDeploymentGeneration != prevNodeState.TargetDeploymentGeneration
	if prevInTransition && !prevFinished {
		return 0
	}

	prevAhead := prevNodeState.CurrentDeploymentGeneration > currNodeState.CurrentDeploymentGeneration
	failedAtPrev := currNodeState.LastFailedDeploymentGeneration == prevNodeState.CurrentDeploymentGeneration
	if prevAhead && !failedAtPrev {
		return prevNodeState.CurrentDeploymentGeneration
	}

	return 0
}

func getInstallerPodName(deploymentID int32, nodeName string) string {
	return fmt.Sprintf("installer-%d-%s", deploymentID, nodeName)
}

// createInstallerPod creates the installer pod with the secrets required to
func (c *InstallerController) createInstallerPod(currNodeState *operatorv1alpha1.NodeStatus, operatorSpec *operatorv1alpha1.OperatorSpec, deploymentID int32) (*operatorv1alpha1.NodeStatus, error) {
	required := resourceread.ReadPodV1OrDie([]byte(installerPod))
	switch corev1.PullPolicy(operatorSpec.ImagePullPolicy) {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
		required.Spec.Containers[0].ImagePullPolicy = corev1.PullPolicy(operatorSpec.ImagePullPolicy)
	case "":
	default:
		return nil, fmt.Errorf("invalid imagePullPolicy specified: %v", operatorSpec.ImagePullPolicy)
	}
	required.Name = getInstallerPodName(deploymentID, currNodeState.NodeName)
	required.Namespace = c.targetNamespace
	required.Spec.NodeName = currNodeState.NodeName
	required.Spec.Containers[0].Image = c.installerPodImageFn()
	required.Spec.Containers[0].Command = c.command
	required.Spec.Containers[0].Args = append(required.Spec.Containers[0].Args,
		fmt.Sprintf("-v=%d", operatorSpec.Logging.Level),
		fmt.Sprintf("--deployment-id=%d", deploymentID),
		fmt.Sprintf("--namespace=%s", c.targetNamespace),
		fmt.Sprintf("--pod=%s", c.configMaps[0]),
		fmt.Sprintf("--resource-dir=%s", "/etc/kubernetes/static-pod-resources"),
		fmt.Sprintf("--pod-manifest-dir=%s", "/etc/kubernetes/manifests"),
	)
	for _, name := range c.configMaps {
		required.Spec.Containers[0].Args = append(required.Spec.Containers[0].Args, fmt.Sprintf("--configmaps=%s", name))
	}
	for _, name := range c.secrets {
		required.Spec.Containers[0].Args = append(required.Spec.Containers[0].Args, fmt.Sprintf("--secrets=%s", name))
	}

	if _, err := c.kubeClient.CoreV1().Pods(c.targetNamespace).Create(required); err != nil {
		glog.Errorf("failed to create pod on node %q for %s: %v", currNodeState.NodeName, resourceread.WritePodV1OrDie(required), err)
		return nil, err
	}

	ret := currNodeState.DeepCopy()
	ret.TargetDeploymentGeneration = deploymentID
	ret.LastFailedDeploymentErrors = nil

	return ret, nil
}

func getInstallerPodImageFromEnv() string {
	return os.Getenv("OPERATOR_IMAGE")
}

func (c InstallerController) sync() error {
	operatorSpec, originalOperatorStatus, resourceVersion, err := c.operatorConfigClient.Get()
	if err != nil {
		return err
	}
	operatorStatus := originalOperatorStatus.DeepCopy()

	switch operatorSpec.ManagementState {
	case operatorv1alpha1.Unmanaged:
		return nil
	case operatorv1alpha1.Removed:
		// TODO probably just fail.  Static pod managers can't be removed.
		return nil
	}
	requeue, syncErr := c.createInstallerController(operatorSpec, operatorStatus, resourceVersion)
	if requeue && syncErr == nil {
		return fmt.Errorf("synthetic requeue request")
	}
	err = syncErr

	if err != nil {
		v1alpha1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1alpha1.OperatorCondition{
			Type:    operatorv1alpha1.OperatorStatusTypeFailing,
			Status:  operatorv1alpha1.ConditionTrue,
			Reason:  "StatusUpdateError",
			Message: err.Error(),
		})
		if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
			if _, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus); updateError != nil {
				glog.Error(updateError)
			}
		}
		return err
	}

	return nil
}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *InstallerController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting InstallerController")
	defer glog.Infof("Shutting down InstallerController")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *InstallerController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *InstallerController) processNextWorkItem() bool {
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

// eventHandler queues the operator to check spec and status
func (c *InstallerController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(installerControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(installerControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(installerControllerWorkQueueKey) },
	}
}

const installerPod = `apiVersion: v1
kind: Pod
metadata:
  namespace: <namespace>
  name: installer-<deployment-id>-<nodeName>
  labels:
    app: installer
spec:
  serviceAccountName: installer-sa
  containers:
  - name: installer
    image: ${IMAGE}
    imagePullPolicy: Always
    securityContext:
      privileged: true
      runAsUser: 0
    terminationMessagePolicy: FallbackToLogsOnError
    volumeMounts:
    - mountPath: /etc/kubernetes/
      name: kubelet-dir
  restartPolicy: Never
  securityContext:
    runAsUser: 0
  volumes:
  - hostPath:
      path: /etc/kubernetes/
    name: kubelet-dir
`
