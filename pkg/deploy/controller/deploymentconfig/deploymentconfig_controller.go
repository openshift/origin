package deploymentconfig

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/v1"
	"k8s.io/kubernetes/pkg/client/retry"
	kcontroller "k8s.io/kubernetes/pkg/controller"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

const (
	// maxRetryCount is the number of times a deployment config will be retried before it is dropped out
	// of the queue.
	maxRetryCount = 15
)

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling deployment config: %s", string(e))
}

// DeploymentConfigController is responsible for creating a new deployment
// when:
//
//    1. The config version is > 0 and,
//    2. No deployment for the version exists.
//
// The controller reconciles deployments with the replica count specified on
// the config. The active deployment (that is, the latest successful
// deployment) will always be scaled to the config replica count. All other
// deployments will be scaled to zero.
//
// If a new version is observed for which no deployment exists, any running
// deployments will be cancelled. The controller will not attempt to scale
// running deployments.
type DeploymentConfigController struct {
	// dn provides access to deploymentconfigs.
	dn osclient.DeploymentConfigsNamespacer
	// rn provides access to replication controllers.
	rn kcoreclient.ReplicationControllersGetter

	// queue contains deployment configs that need to be synced.
	queue workqueue.RateLimitingInterface

	// dcStore provides a local cache for deployment configs.
	dcStore oscache.StoreToDeploymentConfigLister
	// dcStoreSynced makes sure the dc store is synced before reconcling any deployment config.
	dcStoreSynced func() bool
	// rcLister can list/get replication controllers from a shared informer's cache
	rcLister kcorelisters.ReplicationControllerLister
	// rcListerSynced makes sure the rc shared informer is synced before reconcling any deployment config.
	rcListerSynced func() bool
	// rcControl is used for adopting/releasing replication controllers.
	rcControl RCControlInterface

	// codec is used to build deployments from configs.
	codec runtime.Codec
	// recorder is used to record events.
	recorder record.EventRecorder
}

// Handle implements the loop that processes deployment configs. Since this controller started
// using caches, the provided config MUST be deep-copied beforehand (see work() in factory.go).
func (c *DeploymentConfigController) Handle(config *deployapi.DeploymentConfig) error {
	glog.V(5).Infof("Reconciling %s/%s", config.Namespace, config.Name)
	// There's nothing to reconcile until the version is nonzero.
	if config.Status.LatestVersion == 0 {
		return c.updateStatus(config, []*v1.ReplicationController{})
	}

	// List all ReplicationControllers to find also those we own but that no longer match our selector.
	// They will be orphaned by ClaimReplicationControllers().
	rcList, err := c.rcLister.ReplicationControllers(config.Namespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error while deploymentConfigController listing replication controllers: %v", err)
	}
	selector := deployutil.ConfigSelector(config.Name)
	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing ReplicationControllers (see Kubernetes #42639).
	canAdoptFunc := kcontroller.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := c.dn.DeploymentConfigs(config.Namespace).Get(config.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != config.UID {
			return nil, fmt.Errorf("original DeploymentConfig %v/%v is gone: got uid %v, wanted %v", config.Namespace, config.Name, fresh.UID, config.UID)
		}
		return fresh, nil
	})
	cm := NewRCControllerRefManager(c.rcControl, config, selector, deployutil.DeploymentConfigControllerRefKind, canAdoptFunc)
	existingDeployments, err := cm.ClaimReplicationControllers(rcList)
	if err != nil {
		return fmt.Errorf("error while deploymentConfigController claiming replication controllers: %v", err)
	}

	// In case the deployment config has been marked for deletion, merely update its status with
	// the latest available information. Some deletions make take some time to complete so there
	// is value in doing this.
	if config.DeletionTimestamp != nil {
		return c.updateStatus(config, existingDeployments)
	}

	latestIsDeployed, latestDeployment := deployutil.LatestDeploymentInfo(config, existingDeployments)
	// If the latest deployment doesn't exist yet, cancel any running
	// deployments to allow them to be superceded by the new config version.
	awaitingCancellations := false
	if !latestIsDeployed {
		for i := range existingDeployments {
			deployment := existingDeployments[i]
			// Skip deployments with an outcome.
			if deployutil.IsTerminatedDeployment(deployment) {
				continue
			}
			// Cancel running deployments.
			awaitingCancellations = true
			if deployutil.IsDeploymentCancelled(deployment) {
				continue
			}

			// Retry faster on conflicts
			var updatedDeployment *v1.ReplicationController
			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				rc, err := c.rcLister.ReplicationControllers(deployment.Namespace).Get(deployment.Name)
				if kapierrors.IsNotFound(err) {
					return nil
				}
				if err != nil {
					return err
				}
				// We need to make sure we own that RC or adopt it if possible
				isOurs, err := cm.ClaimReplicationController(rc)
				if err != nil {
					return fmt.Errorf("error while deploymentConfigController claiming the replication controller %s/%s: %v", rc.Namespace, rc.Name, err)
				}
				if !isOurs {
					return nil
				}

				copied, err := deployutil.DeploymentDeepCopyV1(rc)
				if err != nil {
					return err
				}
				copied.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
				copied.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledNewerDeploymentExists
				updatedDeployment, err = c.rn.ReplicationControllers(copied.Namespace).Update(copied)
				return err
			}); err != nil {
				c.recorder.Eventf(config, v1.EventTypeWarning, "DeploymentCancellationFailed", "Failed to cancel deployment %q superceded by version %d: %s", deployment.Name, config.Status.LatestVersion, err)
			} else if updatedDeployment != nil {
				// replace the current deployment with the updated copy so that a future update has a chance at working
				existingDeployments[i] = updatedDeployment
				c.recorder.Eventf(config, v1.EventTypeNormal, "DeploymentCancelled", "Cancelled deployment %q superceded by version %d", deployment.Name, config.Status.LatestVersion)
			}
		}
	}
	// Wait for deployment cancellations before reconciling or creating a new
	// deployment to avoid competing with existing deployment processes.
	if awaitingCancellations {
		c.recorder.Eventf(config, v1.EventTypeNormal, "DeploymentAwaitingCancellation", "Deployment of version %d awaiting cancellation of older running deployments", config.Status.LatestVersion)
		return fmt.Errorf("found previous inflight deployment for %s - requeuing", deployutil.LabelForDeploymentConfig(config))
	}
	// If the latest deployment already exists, reconcile existing deployments
	// and return early.
	if latestIsDeployed {
		// If the latest deployment is still running, try again later. We don't
		// want to compete with the deployer.
		if !deployutil.IsTerminatedDeployment(latestDeployment) {
			return c.updateStatus(config, existingDeployments)
		}

		return c.reconcileDeployments(existingDeployments, config, cm)
	}
	// If the config is paused we shouldn't create new deployments for it.
	if config.Spec.Paused {
		// in order for revision history limit cleanup to work for paused
		// deployments, we need to trigger it here
		if err := c.cleanupOldDeployments(existingDeployments, config); err != nil {
			c.recorder.Eventf(config, v1.EventTypeWarning, "DeploymentCleanupFailed", "Couldn't clean up deployments: %v", err)
		}

		return c.updateStatus(config, existingDeployments)
	}
	// No deployments are running and the latest deployment doesn't exist, so
	// create the new deployment.
	deployment, err := deployutil.MakeDeploymentV1(config, c.codec)
	if err != nil {
		return fatalError(fmt.Sprintf("couldn't make deployment from (potentially invalid) deployment config %s: %v", deployutil.LabelForDeploymentConfig(config), err))
	}
	created, err := c.rn.ReplicationControllers(config.Namespace).Create(deployment)
	if err != nil {
		// We need to find out if our controller owns that deployment and report error if not
		if kapierrors.IsAlreadyExists(err) {
			rc, err := c.rcLister.ReplicationControllers(deployment.Namespace).Get(deployment.Name)
			if err != nil {
				return fmt.Errorf("error while deploymentConfigController getting the replication controller %s/%s: %v", rc.Namespace, rc.Name, err)
			}
			// We need to make sure we own that RC or adopt it if possible
			isOurs, err := cm.ClaimReplicationController(rc)
			if err != nil {
				return fmt.Errorf("error while deploymentConfigController claiming the replication controller: %v", err)
			}
			if isOurs {
				// If the deployment was already created, just move on. The cache could be
				// stale, or another process could have already handled this update.
				return c.updateStatus(config, existingDeployments)
			} else {
				err = fmt.Errorf("replication controller %s already exists and deployment config is not allowed to claim it.", deployment.Name)
				c.recorder.Eventf(config, v1.EventTypeWarning, "DeploymentCreationFailed", "Couldn't deploy version %d: %v", config.Status.LatestVersion, err)
				return c.updateStatus(config, existingDeployments)
			}
		}
		c.recorder.Eventf(config, v1.EventTypeWarning, "DeploymentCreationFailed", "Couldn't deploy version %d: %s", config.Status.LatestVersion, err)
		// We don't care about this error since we need to report the create failure.
		cond := deployutil.NewDeploymentCondition(deployapi.DeploymentProgressing, kapi.ConditionFalse, deployapi.FailedRcCreateReason, err.Error())
		_ = c.updateStatus(config, existingDeployments, *cond)
		return fmt.Errorf("couldn't create deployment for deployment config %s: %v", deployutil.LabelForDeploymentConfig(config), err)
	}
	msg := fmt.Sprintf("Created new replication controller %q for version %d", created.Name, config.Status.LatestVersion)
	c.recorder.Eventf(config, v1.EventTypeNormal, "DeploymentCreated", msg)

	// As we've just created a new deployment, we need to make sure to clean
	// up old deployments if we have reached our deployment history quota
	existingDeployments = append(existingDeployments, created)
	if err := c.cleanupOldDeployments(existingDeployments, config); err != nil {
		c.recorder.Eventf(config, v1.EventTypeWarning, "DeploymentCleanupFailed", "Couldn't clean up deployments: %v", err)
	}

	cond := deployutil.NewDeploymentCondition(deployapi.DeploymentProgressing, kapi.ConditionTrue, deployapi.NewReplicationControllerReason, msg)
	return c.updateStatus(config, existingDeployments, *cond)
}

// reconcileDeployments reconciles existing deployment replica counts which
// could have diverged outside the deployment process (e.g. due to auto or
// manual scaling, or partial deployments). The active deployment is the last
// successful deployment, not necessarily the latest in terms of the config
// version. The active deployment replica count should follow the config, and
// all other deployments should be scaled to zero.
func (c *DeploymentConfigController) reconcileDeployments(existingDeployments []*v1.ReplicationController, config *deployapi.DeploymentConfig, cm *RCControllerRefManager) error {
	activeDeployment := deployutil.ActiveDeploymentV1(existingDeployments)

	// Reconcile deployments. The active deployment follows the config, and all
	// other deployments should be scaled to zero.
	var updatedDeployments []*v1.ReplicationController
	for i := range existingDeployments {
		deployment := existingDeployments[i]
		toAppend := deployment

		isActiveDeployment := activeDeployment != nil && deployment.Name == activeDeployment.Name

		oldReplicaCount := deployment.Spec.Replicas
		if oldReplicaCount == nil {
			zero := int32(0)
			oldReplicaCount = &zero
		}
		newReplicaCount := int32(0)
		if isActiveDeployment {
			newReplicaCount = config.Spec.Replicas
		}
		if config.Spec.Test {
			glog.V(4).Infof("Deployment config %q is test and deployment %q will be scaled down", deployutil.LabelForDeploymentConfig(config), deployutil.LabelForDeploymentV1(deployment))
			newReplicaCount = 0
		}

		// Only update if necessary.
		var copied *v1.ReplicationController
		if newReplicaCount != *oldReplicaCount {
			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				// refresh the replication controller version
				rc, err := c.rcLister.ReplicationControllers(deployment.Namespace).Get(deployment.Name)
				if err != nil {
					return err
				}
				// We need to make sure we own that RC or adopt it if possible
				isOurs, err := cm.ClaimReplicationController(rc)
				if err != nil {
					return fmt.Errorf("error while deploymentConfigController claiming the replication controller %s/%s: %v", rc.Namespace, rc.Name, err)
				}
				if !isOurs {
					return fmt.Errorf("deployment config %s/%s (%v) no longer owns replication controller %s/%s (%v)",
						config.Namespace, config.Name, config.UID,
						deployment.Namespace, deployment.Name, deployment.UID,
					)
				}

				copied, err = deployutil.DeploymentDeepCopyV1(rc)
				if err != nil {
					return err
				}
				copied.Spec.Replicas = &newReplicaCount
				copied, err = c.rn.ReplicationControllers(copied.Namespace).Update(copied)
				return err
			}); err != nil {
				c.recorder.Eventf(config, v1.EventTypeWarning, "ReplicationControllerScaleFailed",
					"Failed to scale replication controler %q from %d to %d: %v", deployment.Name, oldReplicaCount, newReplicaCount, err)
				return err
			}

			c.recorder.Eventf(config, v1.EventTypeNormal, "ReplicationControllerScaled", "Scaled replication controller %q from %d to %d", copied.Name, oldReplicaCount, newReplicaCount)
			toAppend = copied
		}

		updatedDeployments = append(updatedDeployments, toAppend)
	}

	// As the deployment configuration has changed, we need to make sure to clean
	// up old deployments if we have now reached our deployment history quota
	if err := c.cleanupOldDeployments(updatedDeployments, config); err != nil {
		c.recorder.Eventf(config, v1.EventTypeWarning, "ReplicationControllerCleanupFailed", "Couldn't clean up replication controllers: %v", err)
	}

	return c.updateStatus(config, updatedDeployments)
}

// Update the status of the provided deployment config. Additional conditions will override any other condition in the
// deployment config status.
func (c *DeploymentConfigController) updateStatus(config *deployapi.DeploymentConfig, deployments []*v1.ReplicationController, additional ...deployapi.DeploymentCondition) error {
	newStatus := calculateStatus(config, deployments, additional...)

	// NOTE: We should update the status of the deployment config only if we need to, otherwise
	// we hotloop between updates.
	if reflect.DeepEqual(newStatus, config.Status) {
		return nil
	}

	copied, err := deployutil.DeploymentConfigDeepCopy(config)
	if err != nil {
		return err
	}

	copied.Status = newStatus
	// TODO: Retry update conficts
	if _, err := c.dn.DeploymentConfigs(copied.Namespace).UpdateStatus(copied); err != nil {
		return err
	}
	glog.V(4).Infof(fmt.Sprintf("Updated status for DeploymentConfig: %s, ", deployutil.LabelForDeploymentConfig(config)) +
		fmt.Sprintf("replicas %d->%d (need %d), ", config.Status.Replicas, newStatus.Replicas, config.Spec.Replicas) +
		fmt.Sprintf("readyReplicas %d->%d, ", config.Status.ReadyReplicas, newStatus.ReadyReplicas) +
		fmt.Sprintf("availableReplicas %d->%d, ", config.Status.AvailableReplicas, newStatus.AvailableReplicas) +
		fmt.Sprintf("unavailableReplicas %d->%d, ", config.Status.UnavailableReplicas, newStatus.UnavailableReplicas) +
		fmt.Sprintf("sequence No: %v->%v", config.Status.ObservedGeneration, newStatus.ObservedGeneration))
	return nil
}

func calculateStatus(config *deployapi.DeploymentConfig, rcs []*v1.ReplicationController, additional ...deployapi.DeploymentCondition) deployapi.DeploymentConfigStatus {
	// UpdatedReplicas represents the replicas that use the current deployment config template which means
	// we should inform about the replicas of the latest deployment and not the active.
	latestReplicas := int32(0)
	latestExists, latestRC := deployutil.LatestDeploymentInfo(config, rcs)
	if !latestExists {
		latestRC = nil
	} else {
		latestReplicas = deployutil.GetStatusReplicaCountForDeployments([]*v1.ReplicationController{latestRC})
	}

	available := deployutil.GetAvailableReplicaCountForReplicationControllers(rcs)
	total := deployutil.GetReplicaCountForDeployments(rcs)
	unavailableReplicas := total - available
	if unavailableReplicas < 0 {
		unavailableReplicas = 0
	}

	status := deployapi.DeploymentConfigStatus{
		LatestVersion:       config.Status.LatestVersion,
		Details:             config.Status.Details,
		ObservedGeneration:  config.Generation,
		Replicas:            deployutil.GetStatusReplicaCountForDeployments(rcs),
		UpdatedReplicas:     latestReplicas,
		AvailableReplicas:   available,
		ReadyReplicas:       deployutil.GetReadyReplicaCountForReplicationControllers(rcs),
		UnavailableReplicas: unavailableReplicas,
		Conditions:          config.Status.Conditions,
	}

	updateConditions(config, &status, latestRC)
	for _, cond := range additional {
		deployutil.SetDeploymentCondition(&status, cond)
	}

	return status
}

func updateConditions(config *deployapi.DeploymentConfig, newStatus *deployapi.DeploymentConfigStatus, latestRC *v1.ReplicationController) {
	// Availability condition.
	if newStatus.AvailableReplicas >= config.Spec.Replicas-deployutil.MaxUnavailable(config) && newStatus.AvailableReplicas > 0 {
		minAvailability := deployutil.NewDeploymentCondition(deployapi.DeploymentAvailable, kapi.ConditionTrue, "", "Deployment config has minimum availability.")
		deployutil.SetDeploymentCondition(newStatus, *minAvailability)
	} else {
		noMinAvailability := deployutil.NewDeploymentCondition(deployapi.DeploymentAvailable, kapi.ConditionFalse, "", "Deployment config does not have minimum availability.")
		deployutil.SetDeploymentCondition(newStatus, *noMinAvailability)
	}

	// Condition about progress.
	if latestRC != nil {
		switch deployutil.DeploymentStatusFor(latestRC) {
		case deployapi.DeploymentStatusPending:
			msg := fmt.Sprintf("replication controller %q is waiting for pod %q to run", latestRC.Name, deployutil.DeployerPodNameForDeployment(latestRC.Name))
			condition := deployutil.NewDeploymentCondition(deployapi.DeploymentProgressing, kapi.ConditionUnknown, "", msg)
			deployutil.SetDeploymentCondition(newStatus, *condition)
		case deployapi.DeploymentStatusRunning:
			if deployutil.IsProgressing(config, newStatus) {
				deployutil.RemoveDeploymentCondition(newStatus, deployapi.DeploymentProgressing)
				msg := fmt.Sprintf("replication controller %q is progressing", latestRC.Name)
				condition := deployutil.NewDeploymentCondition(deployapi.DeploymentProgressing, kapi.ConditionTrue, deployapi.ReplicationControllerUpdatedReason, msg)
				// TODO: Right now, we use lastTransitionTime for storing the last time we had any progress instead
				// of the last time the condition transitioned to a new status. We should probably change that.
				deployutil.SetDeploymentCondition(newStatus, *condition)
			}
		case deployapi.DeploymentStatusFailed:
			var condition *deployapi.DeploymentCondition
			if deployutil.IsDeploymentCancelled(latestRC) {
				msg := fmt.Sprintf("rollout of replication controller %q was cancelled", latestRC.Name)
				condition = deployutil.NewDeploymentCondition(deployapi.DeploymentProgressing, kapi.ConditionFalse, deployapi.CancelledRolloutReason, msg)
			} else {
				msg := fmt.Sprintf("replication controller %q has failed progressing", latestRC.Name)
				condition = deployutil.NewDeploymentCondition(deployapi.DeploymentProgressing, kapi.ConditionFalse, deployapi.TimedOutReason, msg)
			}
			deployutil.SetDeploymentCondition(newStatus, *condition)
		case deployapi.DeploymentStatusComplete:
			msg := fmt.Sprintf("replication controller %q successfully rolled out", latestRC.Name)
			condition := deployutil.NewDeploymentCondition(deployapi.DeploymentProgressing, kapi.ConditionTrue, deployapi.NewRcAvailableReason, msg)
			deployutil.SetDeploymentCondition(newStatus, *condition)
		}
	}
}

func (c *DeploymentConfigController) handleErr(err error, key interface{}) {
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
		glog.V(2).Infof("Error syncing deployment config %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	glog.V(2).Infof("Dropping deployment config %q out of the queue: %v", key, err)
	c.queue.Forget(key)
}

// cleanupOldDeployments deletes old replication controller deployments if their quota has been reached
func (c *DeploymentConfigController) cleanupOldDeployments(existingDeployments []*v1.ReplicationController, deploymentConfig *deployapi.DeploymentConfig) error {
	if deploymentConfig.Spec.RevisionHistoryLimit == nil {
		// there is no past deplyoment quota set
		return nil
	}

	prunableDeployments := deployutil.DeploymentsForCleanup(deploymentConfig, existingDeployments)
	if len(prunableDeployments) <= int(*deploymentConfig.Spec.RevisionHistoryLimit) {
		// the past deployment quota has not been exceeded
		return nil
	}

	deletionErrors := []error{}
	for i := 0; i < (len(prunableDeployments) - int(*deploymentConfig.Spec.RevisionHistoryLimit)); i++ {
		deployment := prunableDeployments[i]
		if *deployment.Spec.Replicas != 0 {
			// we do not want to clobber active older deployments, but we *do* want them to count
			// against the quota so that they will be pruned when they're scaled down
			continue
		}

		policy := metav1.DeletePropagationBackground
		err := c.rn.ReplicationControllers(deployment.Namespace).Delete(deployment.Name, &metav1.DeleteOptions{
			PropagationPolicy: &policy,
		})
		if err != nil && !kapierrors.IsNotFound(err) {
			deletionErrors = append(deletionErrors, err)
		}
	}

	return kutilerrors.NewAggregate(deletionErrors)
}
