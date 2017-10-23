package deployment

import (
	"fmt"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/v1"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	deployutil "github.com/openshift/origin/pkg/apps/util"
	"github.com/openshift/origin/pkg/util"
)

// maxRetryCount is the maximum number of times the controller will retry errors.
// The first requeue is after 5ms and subsequent requeues grow exponentially.
// This effectively can extend up to 5*2^14ms which caps to 82s:
//
// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
//
//
// The most common errors are:
//
// * failure to delete the deployer pods
// * failure to update the replication controller
// * pod may be missing from the cache once the deployment transitions to Pending.
//
// In most cases, we shouldn't need to retry up to maxRetryCount...
const maxRetryCount = 15

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling rollout: " + string(e) }

// actionableError is an error on which users can act.
type actionableError string

func (e actionableError) Error() string { return string(e) }

// DeploymentController starts a deployment by creating a deployer pod which
// implements a deployment strategy. The status of the deployment will follow
// the status of the deployer pod. The deployer pod is correlated to the
// deployment with annotations.
//
// When the deployment enters a terminal status:
//
//   1. If the deployment finished normally, the deployer pod is deleted.
//   2. If the deployment failed, the deployer pod is not deleted.
type DeploymentController struct {
	// rn is used for updating replication controllers.
	rn kcoreclient.ReplicationControllersGetter
	// pn is used for creating, updating, and deleting deployer pods.
	pn kcoreclient.PodsGetter

	// queue contains replication controllers that need to be synced.
	queue workqueue.RateLimitingInterface

	// rcLister can list/get replication controllers from a shared informer's cache
	rcLister kcorelisters.ReplicationControllerLister
	// rcListerSynced makes sure the rc store is synced before reconcling any deployment.
	rcListerSynced cache.InformerSynced
	// podLister can list/get pods from a shared informer's cache
	podLister kcorelisters.PodLister
	// podListerSynced makes sure the pod store is synced before reconcling any deployment.
	podListerSynced cache.InformerSynced

	// deployerImage specifies which Docker image can support the default strategies.
	deployerImage string
	// serviceAccount to create deployment pods with.
	serviceAccount string
	// environment is a set of environment variables which should be injected into all
	// deployer pod containers.
	environment []v1.EnvVar
	// codec is used for deserializing deploymentconfigs from replication controller
	// annotations.
	codec runtime.Codec
	// recorder is used to record events.
	recorder record.EventRecorder
}

// handle processes a deployment and either creates a deployer pod or responds
// to a terminal deployment status. Since this controller started using caches,
// the provided rc MUST be deep-copied beforehand (see work() in factory.go).
func (c *DeploymentController) handle(deployment *v1.ReplicationController, willBeDropped bool) error {
	// Copy all the annotations from the deployment.
	updatedAnnotations := make(map[string]string)
	for key, value := range deployment.Annotations {
		updatedAnnotations[key] = value
	}

	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus

	deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)
	deployer, deployerErr := c.podLister.Pods(deployment.Namespace).Get(deployerPodName)
	if deployerErr == nil {
		nextStatus = c.nextStatus(deployer, deployment, updatedAnnotations)
	}

	switch currentStatus {
	case deployapi.DeploymentStatusNew:
		// If the deployment has been cancelled, don't create a deployer pod.
		// Instead try to delete any deployer pods found and transition the
		// deployment to Pending so that the deployment config controller
		// continues to see the deployment as in-flight. Eventually the deletion
		// of the deployer pod should cause a requeue of this deployment and
		// then it can be transitioned to Failed by this controller.
		if deployutil.IsDeploymentCancelled(deployment) {
			nextStatus = deployapi.DeploymentStatusPending
			if err := c.cleanupDeployerPods(deployment); err != nil {
				return err
			}
			break
		}
		// In case the deployment is stuck in "new" state because we fail to create
		// deployer pod (quota, etc..) we should respect the timeoutSeconds in the
		// config strategy and transition the rollout to failed instead of waiting for
		// the deployment pod forever.
		config, err := deployutil.DecodeDeploymentConfig(deployment, c.codec)
		if err != nil {
			return err
		}
		if deployutil.RolloutExceededTimeoutSeconds(config, deployment) {
			nextStatus = deployapi.DeploymentStatusFailed
			updatedAnnotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedUnableToCreateDeployerPod
			c.emitDeploymentEvent(deployment, v1.EventTypeWarning, "RolloutTimeout", fmt.Sprintf("Rollout for %q failed to create deployer pod (timeoutSeconds: %ds)", deployutil.LabelForDeploymentV1(deployment), deployutil.GetTimeoutSecondsForStrategy(config)))
			glog.V(4).Infof("Failing deployment %s/%s as we reached timeout while waiting for the deployer pod to be created", deployment.Namespace, deployment.Name)
			break
		}

		switch {
		case kerrors.IsNotFound(deployerErr):
			if _, ok := deployment.Annotations[deployapi.DeploymentIgnorePodAnnotation]; ok {
				return nil
			}

			// Generate a deployer pod spec.
			deployerPod, err := c.makeDeployerPod(deployment)
			if err != nil {
				return fatalError(fmt.Sprintf("couldn't make deployer pod for %q: %v", deployutil.LabelForDeploymentV1(deployment), err))
			}
			// Create the deployer pod.
			deploymentPod, err := c.pn.Pods(deployment.Namespace).Create(deployerPod)
			// Retry on error.
			if err != nil {
				// if we cannot create a deployment pod (i.e lack of quota), match normal replica set experience and
				// emit an event.
				c.emitDeploymentEvent(deployment, v1.EventTypeWarning, "FailedCreate", fmt.Sprintf("Error creating deployer pod: %v", err))
				return actionableError(fmt.Sprintf("couldn't create deployer pod for %q: %v", deployutil.LabelForDeploymentV1(deployment), err))
			}
			updatedAnnotations[deployapi.DeploymentPodAnnotation] = deploymentPod.Name
			updatedAnnotations[deployapi.DeployerPodCreatedAtAnnotation] = deploymentPod.CreationTimestamp.String()
			if deploymentPod.Status.StartTime != nil {
				updatedAnnotations[deployapi.DeployerPodStartedAtAnnotation] = deploymentPod.Status.StartTime.String()
			}
			nextStatus = deployapi.DeploymentStatusPending
			glog.V(4).Infof("Created deployer pod %q for %q", deploymentPod.Name, deployutil.LabelForDeploymentV1(deployment))

		// Most likely dead code since we never get an error different from 404 back from the cache.
		case deployerErr != nil:
			// If the pod already exists, it's possible that a previous CreatePod
			// succeeded but the deployment state update failed and now we're re-
			// entering. Ensure that the pod is the one we created by verifying the
			// annotation on it, and throw a retryable error.
			return fmt.Errorf("couldn't fetch existing deployer pod for %s: %v", deployutil.LabelForDeploymentV1(deployment), deployerErr)

		default: /* deployerErr == nil */
			// Do a stronger check to validate that the existing deployer pod is
			// actually for this deployment, and if not, fail this deployment.
			//
			// TODO: Investigate checking the container image of the running pod and
			// comparing with the intended deployer pod image. If we do so, we'll need
			// to ensure that changes to 'unrelated' pods don't result in updates to
			// the deployment. So, the image check will have to be done in other areas
			// of the code as well.
			if deployutil.DeploymentNameFor(deployer) != deployment.Name {
				nextStatus = deployapi.DeploymentStatusFailed
				updatedAnnotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedUnrelatedDeploymentExists
				c.emitDeploymentEvent(deployment, v1.EventTypeWarning, "FailedCreate", fmt.Sprintf("Error creating deployer pod since another pod with the same name (%q) exists", deployer.Name))
			} else {
				// Update to pending or to the appropriate status relative to the existing validated deployer pod.
				updatedAnnotations[deployapi.DeploymentPodAnnotation] = deployer.Name
				updatedAnnotations[deployapi.DeployerPodCreatedAtAnnotation] = deployer.CreationTimestamp.String()
				if deployer.Status.StartTime != nil {
					updatedAnnotations[deployapi.DeployerPodStartedAtAnnotation] = deployer.Status.StartTime.String()
				}
				nextStatus = nextStatusComp(nextStatus, deployapi.DeploymentStatusPending)
			}
		}

	case deployapi.DeploymentStatusPending, deployapi.DeploymentStatusRunning:
		switch {
		case kerrors.IsNotFound(deployerErr):
			nextStatus = deployapi.DeploymentStatusFailed
			// If the deployment is cancelled here then we deleted the deployer in a previous
			// resync of the deployment.
			if !deployutil.IsDeploymentCancelled(deployment) {
				// Retry more before setting the deployment to Failed if it's Pending - the pod might not have
				// appeared in the cache yet.
				if !willBeDropped && currentStatus == deployapi.DeploymentStatusPending {
					return deployerErr
				}
				updatedAnnotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedDeployerPodNoLongerExists
				c.emitDeploymentEvent(deployment, v1.EventTypeWarning, "Failed", fmt.Sprintf("Deployer pod %q has gone missing", deployerPodName))
				deployerErr = fmt.Errorf("Failing rollout for %q because its deployer pod %q disappeared", deployutil.LabelForDeploymentV1(deployment), deployerPodName)
				utilruntime.HandleError(deployerErr)
			}

		// Most likely dead code since we never get an error different from 404 back from the cache.
		case deployerErr != nil:
			// We'll try again later on resync. Continue to process cancellations.
			deployerErr = fmt.Errorf("Error getting deployer pod %q for %q: %v", deployerPodName, deployutil.LabelForDeploymentV1(deployment), deployerErr)
			utilruntime.HandleError(deployerErr)

		default: /* err == nil */
			// If the deployment has been cancelled, delete any deployer pods
			// found. Eventually the deletion of the deployer pod should cause
			// a requeue of this deployment and then it can be transitioned to
			// Failed.
			if deployutil.IsDeploymentCancelled(deployment) {
				if err := c.cleanupDeployerPods(deployment); err != nil {
					return err
				}
			} else {
				// Set an ownerRef for the deployment lifecycle pods so they are cleaned up when the
				// replication controller is deleted.
				if err := c.setDeployerPodsOwnerRef(deployment); err != nil {
					return err
				}
			}
		}

	case deployapi.DeploymentStatusFailed:
		// Try to cleanup once more a cancelled deployment in case hook pods
		// were created just after we issued the first cleanup request.
		if deployutil.IsDeploymentCancelled(deployment) {
			if err := c.cleanupDeployerPods(deployment); err != nil {
				return err
			}
		} else {
			// Set an ownerRef for the deployment lifecycle pods so they are cleaned up when the
			// replication controller is deleted.
			if err := c.setDeployerPodsOwnerRef(deployment); err != nil {
				return err
			}
		}

	case deployapi.DeploymentStatusComplete:
		if err := c.cleanupDeployerPods(deployment); err != nil {
			return err
		}
	}

	// Update only if we need to transition to a new phase.
	if deployutil.CanTransitionPhase(currentStatus, nextStatus) {
		deployment, err := deployutil.DeploymentDeepCopyV1(deployment)
		if err != nil {
			return err
		}

		updatedAnnotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		deployment.Annotations = updatedAnnotations

		// If we are going to transition to failed or complete and scale is non-zero, we'll check one more
		// time to see if we are a test deployment to guarantee that we maintain the test invariant.
		if *deployment.Spec.Replicas != 0 && deployutil.IsTerminatedDeployment(deployment) {
			if config, err := deployutil.DecodeDeploymentConfig(deployment, c.codec); err == nil && config.Spec.Test {
				zero := int32(0)
				deployment.Spec.Replicas = &zero
			}
		}

		if _, err := c.rn.ReplicationControllers(deployment.Namespace).Update(deployment); err != nil {
			return fmt.Errorf("couldn't update rollout status for %q to %s: %v", deployutil.LabelForDeploymentV1(deployment), nextStatus, err)
		}
		glog.V(4).Infof("Updated rollout status for %q from %s to %s (scale: %d)", deployutil.LabelForDeploymentV1(deployment), currentStatus, nextStatus, deployment.Spec.Replicas)

		if deployutil.IsDeploymentCancelled(deployment) && deployutil.IsFailedDeployment(deployment) {
			c.emitDeploymentEvent(deployment, v1.EventTypeNormal, "RolloutCancelled", fmt.Sprintf("Rollout for %q cancelled", deployutil.LabelForDeploymentV1(deployment)))
		}
	}
	return nil
}

func (c *DeploymentController) nextStatus(pod *v1.Pod, deployment *v1.ReplicationController, updatedAnnotations map[string]string) deployapi.DeploymentStatus {
	switch pod.Status.Phase {
	case v1.PodPending:
		return deployapi.DeploymentStatusPending

	case v1.PodRunning:
		return deployapi.DeploymentStatusRunning

	case v1.PodSucceeded:
		// If the deployment was cancelled just prior to the deployer pod succeeding
		// then we need to remove the cancel annotations from the complete deployment
		// and emit an event letting users know their cancellation failed.
		if deployutil.IsDeploymentCancelled(deployment) {
			delete(updatedAnnotations, deployapi.DeploymentCancelledAnnotation)
			delete(updatedAnnotations, deployapi.DeploymentStatusReasonAnnotation)
			c.emitDeploymentEvent(deployment, v1.EventTypeWarning, "FailedCancellation", "Succeeded before cancel recorded")
		}
		// Sync the internal replica annotation with the target so that we can
		// distinguish deployer updates from other scaling events.
		completedTimestamp := getPodTerminatedTimestamp(pod)
		if completedTimestamp != nil {
			updatedAnnotations[deployapi.DeployerPodCompletedAtAnnotation] = completedTimestamp.String()
		}
		updatedAnnotations[deployapi.DeploymentReplicasAnnotation] = updatedAnnotations[deployapi.DesiredReplicasAnnotation]
		delete(updatedAnnotations, deployapi.DesiredReplicasAnnotation)
		return deployapi.DeploymentStatusComplete

	case v1.PodFailed:
		completedTimestamp := getPodTerminatedTimestamp(pod)
		if completedTimestamp != nil {
			updatedAnnotations[deployapi.DeployerPodCompletedAtAnnotation] = completedTimestamp.String()
		}
		return deployapi.DeploymentStatusFailed
	}
	return deployapi.DeploymentStatusNew
}

// getPodTerminatedTimestamp gets the first terminated container in a pod and
// return its termination timestamp.
func getPodTerminatedTimestamp(pod *v1.Pod) *metav1.Time {
	for _, c := range pod.Status.ContainerStatuses {
		if t := c.State.Terminated; t != nil {
			return &t.FinishedAt
		}
	}
	return nil
}

func nextStatusComp(fromDeployer, fromPath deployapi.DeploymentStatus) deployapi.DeploymentStatus {
	if deployutil.CanTransitionPhase(fromPath, fromDeployer) {
		return fromDeployer
	}
	return fromPath
}

// makeDeployerPod creates a pod which implements deployment behavior. The pod is correlated to
// the deployment with an annotation.
func (c *DeploymentController) makeDeployerPod(deployment *v1.ReplicationController) (*v1.Pod, error) {
	deploymentConfig, err := deployutil.DecodeDeploymentConfig(deployment, c.codec)
	if err != nil {
		return nil, err
	}

	container := c.makeDeployerContainer(&deploymentConfig.Spec.Strategy)

	// Add deployment environment variables to the container.
	envVars := []v1.EnvVar{}
	for _, env := range container.Env {
		envVars = append(envVars, env)
	}
	envVars = append(envVars, v1.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: deployment.Name})
	envVars = append(envVars, v1.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: deployment.Namespace})

	// Assigning to a variable since its address is required
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds
	if deploymentConfig.Spec.Strategy.ActiveDeadlineSeconds != nil {
		maxDeploymentDurationSeconds = *(deploymentConfig.Spec.Strategy.ActiveDeadlineSeconds)
	}

	gracePeriod := int64(10)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: deployutil.DeployerPodNameForDeployment(deployment.Name),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
			Labels: map[string]string{
				deployapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
			// Set the owner reference to current deployment, so in case the deployment fails
			// and the deployer pod is preserved when a revisionHistory limit is reached and the
			// deployment is removed, we also remove the deployer pod with it.
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ReplicationController",
				Name:       deployment.Name,
				UID:        deployment.UID,
			}},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:      "deployment",
					Command:   container.Command,
					Args:      container.Args,
					Image:     container.Image,
					Env:       envVars,
					Resources: deployutil.CopyApiResourcesToV1Resources(&deploymentConfig.Spec.Strategy.Resources),
				},
			},
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			DNSPolicy:             deployment.Spec.Template.Spec.DNSPolicy,
			ImagePullSecrets:      deployment.Spec.Template.Spec.ImagePullSecrets,
			// Setting the node selector on the deployer pod so that it is created
			// on the same set of nodes as the pods.
			NodeSelector:                  deployment.Spec.Template.Spec.NodeSelector,
			RestartPolicy:                 v1.RestartPolicyNever,
			ServiceAccountName:            c.serviceAccount,
			TerminationGracePeriodSeconds: &gracePeriod,
		},
	}

	// MergeInfo will not overwrite values unless the flag OverwriteExistingDstKey is set.
	util.MergeInto(pod.Labels, deploymentConfig.Spec.Strategy.Labels, 0)
	util.MergeInto(pod.Annotations, deploymentConfig.Spec.Strategy.Annotations, 0)

	pod.Spec.Containers[0].ImagePullPolicy = v1.PullIfNotPresent

	return pod, nil
}

// makeDeployerContainer creates containers in the following way:
//
//   1. For the Recreate and Rolling strategies, strategy, use the factory's
//      DeployerImage as the container image, and the factory's Environment
//      as the container environment.
//   2. For all Custom strategies, or if the CustomParams field is set, use
//      the strategy's image for the container image, and use the combination
//      of the factory's Environment and the strategy's environment as the
//      container environment.
//
func (c *DeploymentController) makeDeployerContainer(strategy *deployapi.DeploymentStrategy) *v1.Container {
	image := c.deployerImage
	var environment []v1.EnvVar
	var command []string

	set := sets.NewString()
	// Use user-defined values from the strategy input.
	if p := strategy.CustomParams; p != nil {
		if len(p.Image) > 0 {
			image = p.Image
		}
		if len(p.Command) > 0 {
			command = p.Command
		}
		for _, env := range deployutil.CopyApiEnvVarToV1EnvVar(strategy.CustomParams.Environment) {
			set.Insert(env.Name)
			environment = append(environment, env)
		}
	}

	// Set default environment values
	for _, env := range c.environment {
		if set.Has(env.Name) {
			continue
		}
		environment = append(environment, env)
	}

	return &v1.Container{
		Image:   image,
		Command: command,
		Env:     environment,
	}
}

func (c *DeploymentController) getDeployerPods(deployment *v1.ReplicationController) ([]*v1.Pod, error) {
	return c.podLister.Pods(deployment.Namespace).List(deployutil.DeployerPodSelector(deployment.Name))
}

func (c *DeploymentController) setDeployerPodsOwnerRef(deployment *v1.ReplicationController) error {
	deployerPodsList, err := c.getDeployerPods(deployment)
	if err != nil {
		return fmt.Errorf("couldn't fetch deployer pods for %q: %v", deployutil.LabelForDeploymentV1(deployment), err)
	}

	encoder := kapi.Codecs.LegacyCodec(kapi.Registry.EnabledVersions()...)
	glog.V(4).Infof("deployment %s/%s owning %d pods", deployment.Namespace, deployment.Name, len(deployerPodsList))

	var errors []error
	for _, pod := range deployerPodsList {
		if len(pod.OwnerReferences) > 0 {
			continue
		}
		glog.V(4).Infof("setting ownerRef for pod %s/%s to deployment %s/%s", pod.Namespace, pod.Name, deployment.Namespace, deployment.Name)
		objCopy, err := kapi.Scheme.DeepCopy(pod)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		newPod, ok := objCopy.(*v1.Pod)
		if !ok {
			errors = append(errors, fmt.Errorf("object %#+v is not a pod", objCopy))
			continue
		}
		newPod.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: "v1",
			Name:       deployment.Name,
			Kind:       kapi.Kind("ReplicationController").Kind,
			UID:        deployment.UID,
		}})
		newPodBytes, err := runtime.Encode(encoder, newPod)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		oldPodBytes, err := runtime.Encode(encoder, pod)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldPodBytes, newPodBytes, &v1.Pod{})
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if _, err := c.pn.Pods(pod.Namespace).Patch(pod.Name, types.StrategicMergePatchType, patchBytes); err != nil {
			errors = append(errors, err)
		}
	}
	return kutilerrors.NewAggregate(errors)
}

func (c *DeploymentController) cleanupDeployerPods(deployment *v1.ReplicationController) error {
	deployerList, err := c.getDeployerPods(deployment)
	if err != nil {
		return fmt.Errorf("couldn't fetch deployer pods for %q: %v", deployutil.LabelForDeploymentV1(deployment), err)
	}

	cleanedAll := true
	for _, deployerPod := range deployerList {
		if err := c.pn.Pods(deployerPod.Namespace).Delete(deployerPod.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			// if the pod deletion failed, then log the error and continue
			// we will try to delete any remaining deployer pods and return an error later
			utilruntime.HandleError(fmt.Errorf("couldn't delete completed deployer pod %q for %q: %v", deployerPod.Name, deployutil.LabelForDeploymentV1(deployment), err))
			cleanedAll = false
		}
	}

	if !cleanedAll {
		return actionableError(fmt.Sprintf("couldn't clean up all deployer pods for %q", deployutil.LabelForDeploymentV1(deployment)))
	}
	return nil
}

func (c *DeploymentController) emitDeploymentEvent(deployment *v1.ReplicationController, eventType, title, message string) {
	if config, _ := deployutil.DecodeDeploymentConfig(deployment, c.codec); config != nil {
		c.recorder.Eventf(config, eventType, title, message)
	} else {
		c.recorder.Eventf(deployment, eventType, title, message)
	}
}

func (c *DeploymentController) handleErr(err error, key interface{}, deployment *v1.ReplicationController) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if _, isFatal := err.(fatalError); isFatal {
		utilruntime.HandleError(err)
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetryCount {
		c.queue.AddRateLimited(key)
		return
	}

	msg := fmt.Sprintf("Stop retrying: %v", err)
	if _, isActionableErr := err.(actionableError); isActionableErr {
		c.emitDeploymentEvent(deployment, v1.EventTypeWarning, "FailedRetry", msg)
	}
	glog.V(2).Infof(msg)
	c.queue.Forget(key)
}
