package deployment

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/workqueue"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	"github.com/openshift/origin/pkg/util"
)

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling deployment: " + string(e) }

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
	rn kclient.ReplicationControllersNamespacer
	// pn is used for creating, updating, and deleting deployer pods.
	pn kclient.PodsNamespacer

	// queue contains replication controllers that need to be synced.
	queue workqueue.RateLimitingInterface

	// rcStore is a store of replication controllers.
	rcStore cache.StoreToReplicationControllerLister
	// podStore is a store of pods.
	podStore cache.StoreToPodLister
	// rcStoreSynced makes sure the rc store is synced before reconcling any deployment.
	rcStoreSynced func() bool
	// podStoreSynced makes sure the pod store is synced before reconcling any deployment.
	podStoreSynced func() bool

	// deployerImage specifies which Docker image can support the default strategies.
	deployerImage string
	// serviceAccount to create deployment pods with.
	serviceAccount string
	// environment is a set of environment variables which should be injected into all
	// deployer pod containers.
	environment []kapi.EnvVar
	// codec is used for deserializing deploymentconfigs from replication controller
	// annotations.
	codec runtime.Codec
	// recorder is used to record events.
	recorder record.EventRecorder
}

// Handle processes deployment and either creates a deployer pod or responds
// to a terminal deployment status. Since this controller started using caches,
// the provided rc MUST be deep-copied beforehand (see work() in factory.go).
func (c *DeploymentController) Handle(deployment *kapi.ReplicationController) error {
	// Copy all the annotations from the deployment.
	updatedAnnotations := make(map[string]string)
	for key, value := range deployment.Annotations {
		updatedAnnotations[key] = value
	}

	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus

	deployerPodName := deployutil.DeployerPodNameForDeployment(deployment.Name)
	deployer, deployerErr := c.podStore.Pods(deployment.Namespace).Get(deployerPodName)
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

		// If the pod already exists, it's possible that a previous CreatePod
		// succeeded but the deployment state update failed and now we're re-
		// entering. Ensure that the pod is the one we created by verifying the
		// annotation on it, and throw a retryable error.
		if deployerErr != nil && !kerrors.IsNotFound(deployerErr) {
			return fmt.Errorf("couldn't fetch existing deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), deployerErr)
		}
		if deployerErr == nil && deployer != nil {
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
				c.emitDeploymentEvent(deployment, kapi.EventTypeWarning, "FailedCreate", fmt.Sprintf("Error creating deployer pod since another pod with the same name (%q) exists", deployer.Name))
				glog.V(2).Infof("Couldn't create deployer pod for %s since an unrelated pod with the same name (%q) exists", deployutil.LabelForDeployment(deployment), deployer.Name)
			} else {
				// Update to pending or to the appropriate status relative to the existing validated deployer pod.
				updatedAnnotations[deployapi.DeploymentPodAnnotation] = deployer.Name
				nextStatus = nextStatusComp(nextStatus, deployapi.DeploymentStatusPending)
				glog.V(4).Infof("Detected existing deployer pod %s for deployment %s", deployer.Name, deployutil.LabelForDeployment(deployment))
			}
			// Don't try and re-create the deployer pod.
			break
		}

		if _, ok := deployment.Annotations[deployapi.DeploymentIgnorePodAnnotation]; ok {
			return nil
		}

		// Generate a deployer pod spec.
		deployerPod, err := c.makeDeployerPod(deployment)
		if err != nil {
			return fatalError(fmt.Sprintf("couldn't make deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err))
		}
		// Create the deployer pod.
		deploymentPod, err := c.pn.Pods(deployment.Namespace).Create(deployerPod)
		// Retry on error.
		if err != nil {
			return actionableError(fmt.Sprintf("couldn't create deployer pod for %s: %v", deployutil.LabelForDeployment(deployment), err))
		}
		updatedAnnotations[deployapi.DeploymentPodAnnotation] = deploymentPod.Name
		nextStatus = deployapi.DeploymentStatusPending
		glog.V(4).Infof("Created deployer pod %s for deployment %s", deploymentPod.Name, deployutil.LabelForDeployment(deployment))

	case deployapi.DeploymentStatusPending, deployapi.DeploymentStatusRunning:
		switch {
		case kerrors.IsNotFound(deployerErr):
			nextStatus = deployapi.DeploymentStatusFailed
			// If the deployment is cancelled here then we deleted the deployer in a previous
			// resync of the deployment.
			if !deployutil.IsDeploymentCancelled(deployment) {
				updatedAnnotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedDeployerPodNoLongerExists
				c.emitDeploymentEvent(deployment, kapi.EventTypeWarning, "Failed", fmt.Sprintf("Deployer pod %q has gone missing", deployerPodName))
				deployerErr = fmt.Errorf("Failing deployment %q because its deployer pod %q disappeared", deployutil.LabelForDeployment(deployment), deployerPodName)
				utilruntime.HandleError(deployerErr)
			}

		case deployerErr != nil:
			// We'll try again later on resync. Continue to process cancellations.
			deployerErr = fmt.Errorf("Error getting deployer pod %q for deployment %q: %v", deployerPodName, deployutil.LabelForDeployment(deployment), deployerErr)
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
			}
		}

	case deployapi.DeploymentStatusFailed:
		// Try to cleanup once more a cancelled deployment in case hook pods
		// were created just after we issued the first cleanup request.
		if deployutil.IsDeploymentCancelled(deployment) {
			if err := c.cleanupDeployerPods(deployment); err != nil {
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
		deployment, err := deployutil.DeploymentDeepCopy(deployment)
		if err != nil {
			return err
		}

		updatedAnnotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		deployment.Annotations = updatedAnnotations

		// if we are going to transition to failed or complete and scale is non-zero, we'll check one more
		// time to see if we are a test deployment to guarantee that we maintain the test invariant.
		if deployment.Spec.Replicas != 0 && deployutil.IsTerminatedDeployment(deployment) {
			if config, err := deployutil.DecodeDeploymentConfig(deployment, c.codec); err == nil && config.Spec.Test {
				deployment.Spec.Replicas = 0
			}
		}

		if _, err := c.rn.ReplicationControllers(deployment.Namespace).Update(deployment); err != nil {
			return fmt.Errorf("couldn't update deployment %s to status %s: %v", deployutil.LabelForDeployment(deployment), nextStatus, err)
		}
		glog.V(4).Infof("Updated deployment %s status from %s to %s (scale: %d)", deployutil.LabelForDeployment(deployment), currentStatus, nextStatus, deployment.Spec.Replicas)

		if deployutil.IsDeploymentCancelled(deployment) && deployutil.IsFailedDeployment(deployment) {
			c.emitDeploymentEvent(deployment, kapi.EventTypeNormal, "DeploymentCancelled", fmt.Sprintf("Deployment %q cancelled", deployutil.LabelForDeployment(deployment)))
		}
	}
	return nil
}

func (c *DeploymentController) nextStatus(pod *kapi.Pod, deployment *kapi.ReplicationController, updatedAnnotations map[string]string) deployapi.DeploymentStatus {
	switch pod.Status.Phase {
	case kapi.PodPending:
		return deployapi.DeploymentStatusPending

	case kapi.PodRunning:
		return deployapi.DeploymentStatusRunning

	case kapi.PodSucceeded:
		// If the deployment was cancelled just prior to the deployer pod succeeding
		// then we need to remove the cancel annotations from the complete deployment
		// and emit an event letting users know their cancellation failed.
		if deployutil.IsDeploymentCancelled(deployment) {
			delete(updatedAnnotations, deployapi.DeploymentCancelledAnnotation)
			delete(updatedAnnotations, deployapi.DeploymentStatusReasonAnnotation)
			c.emitDeploymentEvent(deployment, kapi.EventTypeWarning, "FailedCancellation", "Succeeded before cancel recorded")
		}
		// Sync the internal replica annotation with the target so that we can
		// distinguish deployer updates from other scaling events.
		updatedAnnotations[deployapi.DeploymentReplicasAnnotation] = updatedAnnotations[deployapi.DesiredReplicasAnnotation]
		delete(updatedAnnotations, deployapi.DesiredReplicasAnnotation)
		return deployapi.DeploymentStatusComplete

	case kapi.PodFailed:
		return deployapi.DeploymentStatusFailed
	}
	return deployapi.DeploymentStatusNew
}

func nextStatusComp(fromDeployer, fromPath deployapi.DeploymentStatus) deployapi.DeploymentStatus {
	if deployutil.CanTransitionPhase(fromPath, fromDeployer) {
		return fromDeployer
	}
	return fromPath
}

// makeDeployerPod creates a pod which implements deployment behavior. The pod is correlated to
// the deployment with an annotation.
func (c *DeploymentController) makeDeployerPod(deployment *kapi.ReplicationController) (*kapi.Pod, error) {
	deploymentConfig, err := deployutil.DecodeDeploymentConfig(deployment, c.codec)
	if err != nil {
		return nil, err
	}

	container := c.makeDeployerContainer(&deploymentConfig.Spec.Strategy)

	// Add deployment environment variables to the container.
	envVars := []kapi.EnvVar{}
	for _, env := range container.Env {
		envVars = append(envVars, env)
	}
	envVars = append(envVars, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAME", Value: deployment.Name})
	envVars = append(envVars, kapi.EnvVar{Name: "OPENSHIFT_DEPLOYMENT_NAMESPACE", Value: deployment.Namespace})

	// Assigning to a variable since its address is required
	maxDeploymentDurationSeconds := deployapi.MaxDeploymentDurationSeconds

	gracePeriod := int64(10)

	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: deployutil.DeployerPodNameForDeployment(deployment.Name),
			Annotations: map[string]string{
				deployapi.DeploymentAnnotation: deployment.Name,
			},
			Labels: map[string]string{
				deployapi.DeployerPodForDeploymentLabel: deployment.Name,
			},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:      "deployment",
					Command:   container.Command,
					Args:      container.Args,
					Image:     container.Image,
					Env:       envVars,
					Resources: deploymentConfig.Spec.Strategy.Resources,
				},
			},
			ActiveDeadlineSeconds: &maxDeploymentDurationSeconds,
			DNSPolicy:             deployment.Spec.Template.Spec.DNSPolicy,
			ImagePullSecrets:      deployment.Spec.Template.Spec.ImagePullSecrets,
			// Setting the node selector on the deployer pod so that it is created
			// on the same set of nodes as the pods.
			NodeSelector:                  deployment.Spec.Template.Spec.NodeSelector,
			RestartPolicy:                 kapi.RestartPolicyNever,
			ServiceAccountName:            c.serviceAccount,
			TerminationGracePeriodSeconds: &gracePeriod,
		},
	}

	// MergeInfo will not overwrite values unless the flag OverwriteExistingDstKey is set.
	util.MergeInto(pod.Labels, deploymentConfig.Spec.Strategy.Labels, 0)
	util.MergeInto(pod.Annotations, deploymentConfig.Spec.Strategy.Annotations, 0)

	pod.Spec.Containers[0].ImagePullPolicy = kapi.PullIfNotPresent

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
func (c *DeploymentController) makeDeployerContainer(strategy *deployapi.DeploymentStrategy) *kapi.Container {
	image := c.deployerImage
	var environment []kapi.EnvVar
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
		for _, env := range strategy.CustomParams.Environment {
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

	return &kapi.Container{
		Image:   image,
		Command: command,
		Env:     environment,
	}
}

func (c *DeploymentController) cleanupDeployerPods(deployment *kapi.ReplicationController) error {
	selector := deployutil.DeployerPodSelector(deployment.Name)
	deployerList, err := c.podStore.Pods(deployment.Namespace).List(selector)
	if err != nil {
		return fmt.Errorf("couldn't fetch deployer pods for %q: %v", deployutil.LabelForDeployment(deployment), err)
	}

	cleanedAll := true
	for _, deployerPod := range deployerList {
		if err := c.pn.Pods(deployerPod.Namespace).Delete(deployerPod.Name, &kapi.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			// if the pod deletion failed, then log the error and continue
			// we will try to delete any remaining deployer pods and return an error later
			utilruntime.HandleError(fmt.Errorf("couldn't delete completed deployer pod %q for deployment %q: %v", deployerPod.Name, deployutil.LabelForDeployment(deployment), err))
			cleanedAll = false
		}
	}

	if !cleanedAll {
		return actionableError(fmt.Sprintf("couldn't clean up all deployer pods for %s", deployment.Name))
	}
	return nil
}

func (c *DeploymentController) emitDeploymentEvent(deployment *kapi.ReplicationController, eventType, title, message string) {
	if config, _ := deployutil.DecodeDeploymentConfig(deployment, c.codec); config != nil {
		c.recorder.Eventf(config, eventType, title, fmt.Sprintf("%s: %s", deployment.Name, message))
	} else {
		c.recorder.Eventf(deployment, eventType, title, message)
	}
}

func (c *DeploymentController) handleErr(err error, key interface{}, deployment *kapi.ReplicationController) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if _, isFatal := err.(fatalError); isFatal {
		utilruntime.HandleError(err)
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < 2 {
		c.queue.AddRateLimited(key)
		return
	}

	if _, isActionableErr := err.(actionableError); isActionableErr {
		c.emitDeploymentEvent(deployment, kapi.EventTypeWarning, "FailedRetry", fmt.Sprintf("About to stop retrying %s: %v", deployment.Name, err))
	}
	c.queue.Forget(key)
}
