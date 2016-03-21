package deployerpod

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeployerPodController keeps a deployment's status in sync with the deployer pod
// handling the deployment.
//
// Use the DeployerPodControllerFactory to create this controller.
type DeployerPodController struct {
	store   cache.Store
	client  osclient.Interface
	kClient kclient.Interface

	// decodeConfig knows how to decode the deploymentConfig from a deployment's annotations.
	decodeConfig func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error)

	// recorder is used to record events.
	recorder record.EventRecorder
}

// transientError is an error which will be retried indefinitely.
type transientError string

func (e transientError) Error() string { return "transient error handling deployer pod: " + string(e) }

// Handle syncs pod's status with any associated deployment.
func (c *DeployerPodController) Handle(pod *kapi.Pod) error {
	// Find the deployment associated with the deployer pod.
	deploymentName := deployutil.DeploymentNameFor(pod)
	if len(deploymentName) == 0 {
		return nil
	}
	// Reject updates to anything but the main deployer pod
	// TODO: Find a way to filter this on the watch side.
	if pod.Name != deployutil.DeployerPodNameForDeployment(deploymentName) {
		return nil
	}

	deployment := &kapi.ReplicationController{ObjectMeta: kapi.ObjectMeta{Namespace: pod.Namespace, Name: deploymentName}}
	cached, exists, err := c.store.Get(deployment)
	if err == nil && exists {
		// Try to use the cache first. Trust hits and return them.
		deployment = cached.(*kapi.ReplicationController)
	} else {
		// Double-check with the master for cache misses/errors, since those
		// are rare and API calls are expensive but more reliable.
		deployment, err = c.kClient.ReplicationControllers(pod.Namespace).Get(deploymentName)
	}

	// If the deployment for this pod has disappeared, we should clean up this
	// and any other deployer pods, then bail out.
	if err != nil {
		// Some retrieval error occurred. Retry.
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("couldn't get deployment %s/%s which owns deployer pod %s/%s", pod.Namespace, deploymentName, pod.Name, pod.Namespace)
		}
		// Find all the deployer pods for the deployment (including this one).
		opts := kapi.ListOptions{LabelSelector: deployutil.DeployerPodSelector(deploymentName)}
		deployers, err := c.kClient.Pods(pod.Namespace).List(opts)
		if err != nil {
			// Retry.
			return fmt.Errorf("couldn't get deployer pods for %s: %v", deployutil.LabelForDeployment(deployment), err)
		}
		// Delete all deployers.
		for _, deployer := range deployers.Items {
			err := c.kClient.Pods(deployer.Namespace).Delete(deployer.Name, kapi.NewDeleteOptions(0))
			if err != nil {
				if !kerrors.IsNotFound(err) {
					// TODO: Should this fire an event?
					glog.V(2).Infof("Couldn't delete orphaned deployer pod %s/%s: %v", deployer.Namespace, deployer.Name, err)
				}
			} else {
				// TODO: Should this fire an event?
				glog.V(2).Infof("Deleted orphaned deployer pod %s/%s", deployer.Namespace, deployer.Name)
			}
		}
		return nil
	}

	currentStatus := deployutil.DeploymentStatusFor(deployment)
	nextStatus := currentStatus

	switch pod.Status.Phase {
	case kapi.PodRunning:
		if !deployutil.IsTerminatedDeployment(deployment) {
			nextStatus = deployapi.DeploymentStatusRunning
		}
	case kapi.PodSucceeded:
		nextStatus = deployapi.DeploymentStatusComplete

		config, decodeErr := c.decodeConfig(deployment)
		// If the deployment was cancelled just prior to the deployer pod succeeding
		// then we need to remove the cancel annotations from the complete deployment
		// and emit an event letting users know their cancellation failed.
		if deployutil.IsDeploymentCancelled(deployment) {
			delete(deployment.Annotations, deployapi.DeploymentCancelledAnnotation)
			delete(deployment.Annotations, deployapi.DeploymentStatusReasonAnnotation)

			if decodeErr == nil {
				c.recorder.Eventf(config, kapi.EventTypeWarning, "FailedCancellation", "Deployment %q succeeded before cancel recorded", deployutil.LabelForDeployment(deployment))
			} else {
				c.recorder.Event(deployment, kapi.EventTypeWarning, "FailedCancellation", "Succeeded before cancel recorded")
			}
		}
		// Sync the internal replica annotation with the target so that we can
		// distinguish deployer updates from other scaling events.
		deployment.Annotations[deployapi.DeploymentReplicasAnnotation] = deployment.Annotations[deployapi.DesiredReplicasAnnotation]
		if nextStatus == deployapi.DeploymentStatusComplete {
			delete(deployment.Annotations, deployapi.DesiredReplicasAnnotation)
		}

		// reset the size of any test container, since we are the ones updating the RC
		if decodeErr == nil && config.Spec.Test {
			deployment.Spec.Replicas = 0
		}
	case kapi.PodFailed:
		nextStatus = deployapi.DeploymentStatusFailed

		// reset the size of any test container, since we are the ones updating the RC
		if config, err := c.decodeConfig(deployment); err == nil && config.Spec.Test {
			deployment.Spec.Replicas = 0
		}
	}

	if currentStatus != nextStatus {
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
		if _, err := c.kClient.ReplicationControllers(deployment.Namespace).Update(deployment); err != nil {
			if kerrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("couldn't update Deployment %s to status %s: %v", deployutil.LabelForDeployment(deployment), nextStatus, err)
		}
		glog.V(4).Infof("Updated deployment %s status from %s to %s (scale: %d)", deployutil.LabelForDeployment(deployment), currentStatus, nextStatus, deployment.Spec.Replicas)

		// If the deployment was canceled, trigger a reconcilation of its deployment config
		// so that the latest complete deployment can immediately rollback in place of the
		// canceled deployment.
		if nextStatus == deployapi.DeploymentStatusFailed && deployutil.IsDeploymentCancelled(deployment) {
			// If we are unable to get the deployment config, then the deploymentconfig controller will
			// perform its duties once the resync interval forces the deploymentconfig to be reconciled.
			name := deployutil.DeploymentConfigNameFor(deployment)
			kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
				config, err := c.client.DeploymentConfigs(deployment.Namespace).Get(name)
				if err != nil {
					return err
				}
				if config.Annotations == nil {
					config.Annotations = make(map[string]string)
				}
				config.Annotations[deployapi.DeploymentCancelledAnnotation] = strconv.Itoa(config.Status.LatestVersion)
				_, err = c.client.DeploymentConfigs(config.Namespace).Update(config)
				return err
			})
		}
	}

	return nil
}
