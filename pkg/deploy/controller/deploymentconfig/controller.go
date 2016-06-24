package deploymentconfig

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/workqueue"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
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
	rn kclient.ReplicationControllersNamespacer

	// queue contains deployment configs that need to be synced.
	queue workqueue.RateLimitingInterface

	// dcStore provides a local cache for deployment configs.
	dcStore oscache.StoreToDeploymentConfigLister
	// rcStore provides a local cache for replication controllers.
	rcStore cache.StoreToReplicationControllerLister
	// podStore provides a local cache for pods.
	podStore cache.StoreToPodLister

	// dcStoreSynced makes sure the dc store is synced before reconcling any deployment config.
	dcStoreSynced func() bool
	// rcStoreSynced makes sure the rc store is synced before reconcling any deployment config.
	rcStoreSynced func() bool
	// podStoreSynced makes sure the pod store is synced before reconcling any deployment config.
	podStoreSynced func() bool

	// codec is used to build deployments from configs.
	codec runtime.Codec
	// recorder is used to record events.
	recorder record.EventRecorder
}

// Handle implements the loop that processes deployment configs. Since this controller started
// using caches, the provided config MUST be deep-copied beforehand (see work() in factory.go).
func (c *DeploymentConfigController) Handle(config *deployapi.DeploymentConfig) error {
	// There's nothing to reconcile until the version is nonzero.
	if config.Status.LatestVersion == 0 {
		return c.updateStatus(config, []kapi.ReplicationController{})
	}

	// Find all deployments owned by the deployment config.
	selector := deployutil.ConfigSelector(config.Name)
	existingDeployments, err := c.rcStore.ReplicationControllers(config.Namespace).List(selector)
	if err != nil {
		return err
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
			if deployutil.IsTerminatedDeployment(&deployment) {
				continue
			}
			// Cancel running deployments.
			awaitingCancellations = true
			if !deployutil.IsDeploymentCancelled(&deployment) {
				copied, err := deploymentCopy(&deployment)
				if err != nil {
					return err
				}

				copied.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
				copied.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledNewerDeploymentExists

				updatedDeployment, err := c.rn.ReplicationControllers(copied.Namespace).Update(copied)
				if err != nil {
					c.recorder.Eventf(config, kapi.EventTypeWarning, "DeploymentCancellationFailed", "Failed to cancel deployment %q superceded by version %d: %s", deployment.Name, config.Status.LatestVersion, err)
				} else {
					// replace the current deployment with the updated copy so that a future update has a chance at working
					existingDeployments[i] = *updatedDeployment
					c.recorder.Eventf(config, kapi.EventTypeNormal, "DeploymentCancelled", "Cancelled deployment %q superceded by version %d", deployment.Name, config.Status.LatestVersion)
				}
			}
		}
	}
	// Wait for deployment cancellations before reconciling or creating a new
	// deployment to avoid competing with existing deployment processes.
	if awaitingCancellations {
		c.recorder.Eventf(config, kapi.EventTypeNormal, "DeploymentAwaitingCancellation", "Deployment of version %d awaiting cancellation of older running deployments", config.Status.LatestVersion)
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
		return c.reconcileDeployments(existingDeployments, config)
	}
	// If the config is paused we shouldn't create new deployments for it.
	// TODO: Make sure cleanup policy will work for paused configs.
	if config.Spec.Paused {
		return c.updateStatus(config, existingDeployments)
	}
	// No deployments are running and the latest deployment doesn't exist, so
	// create the new deployment.
	deployment, err := deployutil.MakeDeployment(config, c.codec)
	if err != nil {
		return fatalError(fmt.Sprintf("couldn't make deployment from (potentially invalid) deployment config %s: %v", deployutil.LabelForDeploymentConfig(config), err))
	}
	created, err := c.rn.ReplicationControllers(config.Namespace).Create(deployment)
	if err != nil {
		// If the deployment was already created, just move on. The cache could be
		// stale, or another process could have already handled this update.
		if errors.IsAlreadyExists(err) {
			return c.updateStatus(config, existingDeployments)
		}
		c.recorder.Eventf(config, kapi.EventTypeWarning, "DeploymentCreationFailed", "Couldn't deploy version %d: %s", config.Status.LatestVersion, err)
		return fmt.Errorf("couldn't create deployment for deployment config %s: %v", deployutil.LabelForDeploymentConfig(config), err)
	}
	c.recorder.Eventf(config, kapi.EventTypeNormal, "DeploymentCreated", "Created new deployment %q for version %d", created.Name, config.Status.LatestVersion)

	return c.updateStatus(config, existingDeployments)
}

// reconcileDeployments reconciles existing deployment replica counts which
// could have diverged outside the deployment process (e.g. due to auto or
// manual scaling, or partial deployments). The active deployment is the last
// successful deployment, not necessarily the latest in terms of the config
// version. The active deployment replica count should follow the config, and
// all other deployments should be scaled to zero.
//
// Previously, scaling behavior was that the config replica count was used
// only for initial deployments and the active deployment had to be scaled up
// directly. To continue supporting that old behavior we must detect when the
// deployment has been directly manipulated, and if so, preserve the directly
// updated value and sync the config with the deployment.
func (c *DeploymentConfigController) reconcileDeployments(existingDeployments []kapi.ReplicationController, config *deployapi.DeploymentConfig) error {
	latestIsDeployed, latestDeployment := deployutil.LatestDeploymentInfo(config, existingDeployments)
	if !latestIsDeployed {
		// We shouldn't be reconciling if the latest deployment hasn't been
		// created; this is enforced on the calling side, but double checking
		// can't hurt.
		return c.updateStatus(config, existingDeployments)
	}
	activeDeployment := deployutil.ActiveDeployment(config, existingDeployments)
	// Compute the replica count for the active deployment (even if the active
	// deployment doesn't exist). The active replica count is the value that
	// should be assigned to the config, to allow the replica propagation to
	// flow downward from the config.
	//
	// By default we'll assume the config replicas should be used to update the
	// active deployment except in special cases (like first sync or externally
	// updated deployments.)
	activeReplicas := config.Spec.Replicas
	source := "the deploymentConfig itself (no change)"

	activeDeploymentExists := activeDeployment != nil
	activeDeploymentIsLatest := activeDeploymentExists && activeDeployment.Name == latestDeployment.Name
	latestDesiredReplicas, latestHasDesiredReplicas := deployutil.DeploymentDesiredReplicas(latestDeployment)

	switch {
	case activeDeploymentExists && activeDeploymentIsLatest:
		// The active/latest deployment follows the config unless this is its first
		// sync or if an external change to the deployment replicas is detected.
		lastActiveReplicas, hasLastActiveReplicas := deployutil.DeploymentReplicas(activeDeployment)
		if !hasLastActiveReplicas || lastActiveReplicas != activeDeployment.Spec.Replicas {
			activeReplicas = activeDeployment.Spec.Replicas
			source = fmt.Sprintf("the latest/active deployment %q which was scaled directly or has not previously been synced", deployutil.LabelForDeployment(activeDeployment))
		}
	case activeDeploymentExists && !activeDeploymentIsLatest:
		// The active/non-latest deployment follows the config if it was
		// previously synced; if this is the first sync, infer what the config
		// value should be based on either the latest desired or whatever the
		// deployment is currently scaled to.
		_, hasLastActiveReplicas := deployutil.DeploymentReplicas(activeDeployment)
		if hasLastActiveReplicas {
			break
		}
		if latestHasDesiredReplicas {
			activeReplicas = latestDesiredReplicas
			source = fmt.Sprintf("the desired replicas of latest deployment %q which has not been previously synced", deployutil.LabelForDeployment(latestDeployment))
		} else if activeDeployment.Spec.Replicas > 0 {
			activeReplicas = activeDeployment.Spec.Replicas
			source = fmt.Sprintf("the active deployment %q which has not been previously synced", deployutil.LabelForDeployment(activeDeployment))
		}
	case !activeDeploymentExists && latestHasDesiredReplicas:
		// If there's no active deployment, use the latest desired, if available.
		activeReplicas = latestDesiredReplicas
		source = fmt.Sprintf("the desired replicas of latest deployment %q with no active deployment", deployutil.LabelForDeployment(latestDeployment))
	}

	// Bring the config in sync with the deployment. Once we know the config
	// accurately represents the desired replica count of the active deployment,
	// we can safely reconcile deployments.
	//
	// If the deployment config is test, never update the deployment config based
	// on deployments, since test behavior overrides user scaling.
	switch {
	case config.Spec.Replicas == activeReplicas:
	case config.Spec.Test:
		glog.V(4).Infof("Detected changed replicas for test deploymentConfig %q, ignoring that change", deployutil.LabelForDeploymentConfig(config))
	default:
		oldReplicas := config.Spec.Replicas
		config.Spec.Replicas = activeReplicas
		var err error
		config, err = c.dn.DeploymentConfigs(config.Namespace).Update(config)
		if err != nil {
			return err
		}
		glog.V(4).Infof("Synced deploymentConfig %q replicas from %d to %d based on %s", deployutil.LabelForDeploymentConfig(config), oldReplicas, activeReplicas, source)
	}

	// Reconcile deployments. The active deployment follows the config, and all
	// other deployments should be scaled to zero.
	var updatedDeployments []kapi.ReplicationController
	for i := range existingDeployments {
		deployment := existingDeployments[i]
		toAppend := deployment

		isActiveDeployment := activeDeployment != nil && deployment.Name == activeDeployment.Name

		oldReplicaCount := deployment.Spec.Replicas
		newReplicaCount := int32(0)
		if isActiveDeployment {
			newReplicaCount = activeReplicas
		}
		if config.Spec.Test {
			glog.V(4).Infof("Deployment config %q is test and deployment %q will be scaled down", deployutil.LabelForDeploymentConfig(config), deployutil.LabelForDeployment(&deployment))
			newReplicaCount = 0
		}
		lastReplicas, hasLastReplicas := deployutil.DeploymentReplicas(&deployment)
		// Only update if necessary.
		if !hasLastReplicas || newReplicaCount != oldReplicaCount || lastReplicas != newReplicaCount {
			copied, err := deploymentCopy(&deployment)
			if err != nil {
				glog.V(2).Infof("Deep copy of deployment %q failed: %v", deployment.Name, err)
				return err
			}

			copied.Spec.Replicas = newReplicaCount
			copied.Annotations[deployapi.DeploymentReplicasAnnotation] = strconv.Itoa(int(newReplicaCount))

			if _, err := c.rn.ReplicationControllers(copied.Namespace).Update(copied); err != nil {
				c.recorder.Eventf(config, kapi.EventTypeWarning, "DeploymentScaleFailed",
					"Failed to scale deployment %q from %d to %d: %v", copied.Name, oldReplicaCount, newReplicaCount, err)
				return err
			}
			// Only report scaling events if we changed the replica count.
			if oldReplicaCount != newReplicaCount {
				c.recorder.Eventf(config, kapi.EventTypeNormal, "DeploymentScaled",
					"Scaled deployment %q from %d to %d", copied.Name, oldReplicaCount, newReplicaCount)
			} else {
				glog.V(4).Infof("Updated deployment %q replica annotation to match current replica count %d", deployutil.LabelForDeployment(copied), newReplicaCount)
			}
			toAppend = *copied
		}

		updatedDeployments = append(updatedDeployments, toAppend)
	}

	return c.updateStatus(config, updatedDeployments)
}

func (c *DeploymentConfigController) updateStatus(config *deployapi.DeploymentConfig, deployments []kapi.ReplicationController) error {
	newStatus, err := c.calculateStatus(*config, deployments)
	if err != nil {
		glog.V(2).Infof("Cannot calculate the status for %q: %v", deployutil.LabelForDeploymentConfig(config), err)
		return err
	}

	// NOTE: We should update the status of the deployment config only if we need to, otherwise
	// we hotloop between updates.
	if reflect.DeepEqual(newStatus, config.Status) {
		return nil
	}

	config.Status = newStatus
	if _, err := c.dn.DeploymentConfigs(config.Namespace).UpdateStatus(config); err != nil {
		glog.V(2).Infof("Cannot update the status for %q: %v", deployutil.LabelForDeploymentConfig(config), err)
		return err
	}
	glog.V(4).Infof("Updated the status for %q (observed generation: %d)", deployutil.LabelForDeploymentConfig(config), config.Status.ObservedGeneration)
	return nil
}

func (c *DeploymentConfigController) calculateStatus(config deployapi.DeploymentConfig, deployments []kapi.ReplicationController) (deployapi.DeploymentConfigStatus, error) {
	// TODO: Implement MinReadySeconds for deploymentconfigs: https://github.com/openshift/origin/issues/7114
	minReadSeconds := int32(0)
	selector := labels.Set(config.Spec.Selector).AsSelector()
	pods, err := c.podStore.Pods(config.Namespace).List(selector)
	if err != nil {
		return config.Status, err
	}
	available := deployutil.GetAvailablePods(pods.Items, minReadSeconds)

	// UpdatedReplicas represents the replicas that use the deployment config template which means
	// we should inform about the replicas of the latest deployment and not the active.
	latestReplicas := int32(0)
	for _, deployment := range deployments {
		if deployment.Name == deployutil.LatestDeploymentNameForConfig(&config) {
			updatedDeployment := []kapi.ReplicationController{deployment}
			latestReplicas = deployutil.GetStatusReplicaCountForDeployments(updatedDeployment)
			break
		}
	}

	total := deployutil.GetReplicaCountForDeployments(deployments)

	return deployapi.DeploymentConfigStatus{
		LatestVersion:       config.Status.LatestVersion,
		Details:             config.Status.Details,
		ObservedGeneration:  config.Generation,
		Replicas:            deployutil.GetStatusReplicaCountForDeployments(deployments),
		UpdatedReplicas:     latestReplicas,
		AvailableReplicas:   available,
		UnavailableReplicas: total - available,
	}, nil
}

func (c *DeploymentConfigController) handleErr(err error, key interface{}) {
	if err == nil {
		return
	}
	if _, isFatal := err.(fatalError); isFatal {
		utilruntime.HandleError(err)
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < 10 {
		c.queue.AddRateLimited(key)
	} else {
		glog.V(2).Infof(err.Error())
		c.queue.Forget(key)
	}
}

func deploymentCopy(rc *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	objCopy, err := kapi.Scheme.DeepCopy(rc)
	if err != nil {
		return nil, err
	}
	copied, ok := objCopy.(*kapi.ReplicationController)
	if !ok {
		return nil, fmt.Errorf("expected ReplicationController, got %#v", objCopy)
	}
	return copied, nil
}
