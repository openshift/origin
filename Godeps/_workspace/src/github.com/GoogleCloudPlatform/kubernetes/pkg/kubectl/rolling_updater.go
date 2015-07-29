/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubectl

import (
	goerrors "errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/wait"
)

const (
	sourceIdAnnotation        = kubectlAnnotationPrefix + "update-source-id"
	desiredReplicasAnnotation = kubectlAnnotationPrefix + "desired-replicas"
	nextControllerAnnotation  = kubectlAnnotationPrefix + "next-controller-id"
)

// RollingUpdaterConfig is the configuration for a rolling deployment process.
type RollingUpdaterConfig struct {
	// Out is a writer for progress output.
	Out io.Writer
	// OldRC is an existing controller to be replaced.
	OldRc *api.ReplicationController
	// NewRc is a controller that will take ownership of updated pods (will be
	// created if needed).
	NewRc *api.ReplicationController
	// UpdatePeriod is the time to wait between individual pod updates.
	UpdatePeriod time.Duration
	// Interval is the time to wait between polling controller status after
	// update.
	Interval time.Duration
	// Timeout is the time to wait for controller updates before giving up.
	Timeout time.Duration
	// CleanupPolicy defines the cleanup action to take after the deployment is
	// complete.
	CleanupPolicy RollingUpdaterCleanupPolicy
	// UpdatePercent is an optional percentage of replicas to scale up each
	// interval and is used to compute the minimum pods to keep ready during the
	// update.
	UpdatePercent *int
}

// RollingUpdaterCleanupPolicy is a cleanup action to take after the
// deployment is complete.
type RollingUpdaterCleanupPolicy string

const (
	// DeleteRollingUpdateCleanupPolicy means delete the old controller.
	DeleteRollingUpdateCleanupPolicy RollingUpdaterCleanupPolicy = "Delete"
	// PreserveRollingUpdateCleanupPolicy means keep the old controller.
	PreserveRollingUpdateCleanupPolicy RollingUpdaterCleanupPolicy = "Preserve"
	// RenameRollingUpdateCleanupPolicy means delete the old controller, and rename
	// the new controller to the name of the old controller.
	RenameRollingUpdateCleanupPolicy RollingUpdaterCleanupPolicy = "Rename"
)

// RollingUpdater provides methods for updating replicated pods in a predictable,
// fault-tolerant way.
type RollingUpdater struct {
	// Client interface for creating and updating controllers
	c client.Interface
	// Namespace for resources
	ns string
	// scaleAndWait scales a controller and returns its updated state.
	scaleAndWait func(rc *api.ReplicationController, retry *RetryParams, wait *RetryParams) (*api.ReplicationController, error)
	//getOrCreateTargetController gets and validates an existing controller or
	//makes a new one.
	getOrCreateTargetController func(controller *api.ReplicationController, sourceId string) (*api.ReplicationController, bool, error)
	// waitForReadyPods should block until there are >0 total pods ready amongst
	// the old and new controllers, and should return the amount of old, new,
	// and total ready.
	waitForReadyPods func(interval, timeout time.Duration, oldRc, newRc *api.ReplicationController) (int, int, int, error)
	// cleanup performs post deployment cleanup tasks for newRc and oldRc.
	cleanup func(oldRc, newRc *api.ReplicationController, config *RollingUpdaterConfig) error
}

// NewRollingUpdater creates a RollingUpdater from a client.
func NewRollingUpdater(namespace string, client client.Interface) *RollingUpdater {
	updater := &RollingUpdater{
		c:  client,
		ns: namespace,
	}
	// Inject real implementations.
	updater.scaleAndWait = updater.scaleAndWaitWithScaler
	updater.getOrCreateTargetController = updater.getOrCreateTargetControllerWithClient
	updater.waitForReadyPods = updater.pollForReadyPods

	return updater
}

// Update all pods for a ReplicationController (oldRc) by creating a new
// controller (newRc) with 0 replicas, and synchronously scaling oldRc and
// newRc until oldRc has 0 replicas and newRc has the original # of desired
// replicas. Cleanup occurs based on a RollingUpdaterCleanupPolicy.
//
// The scaling increment each interval is either 1 or based on a percent of
// the desired replicas. The default scaling direction is up/down. If
// percentage is negative, the direction is down/up for in-place updates.
//
// When a percentage is used, the updater will compute a minimum pod readiness
// requirement and ensure that a minimum number of pods will be ready for the
// duration of the update. Each interval, the updater will scale down whatever
// it can without violating the minimum, and will scale up as much as it can
// up to a maximum increment. This means amount scaled up or down each
// interval will vary based on the timeliness of readiness and the updater
// will always try to make progress, even slowly.
//
// If an update from newRc to oldRc is already in progress, we attempt to
// drive it to completion. If an error occurs at any step of the update, the
// error will be returned.
//
// TODO: make this handle performing a rollback of a partially completed
// rollout.
func (r *RollingUpdater) Update(config *RollingUpdaterConfig) error {
	out := config.Out
	oldRc := config.OldRc

	// Find an existing controller (for continuing an interrupted update) or
	// create a new one if necessary.
	sourceId := fmt.Sprintf("%s:%s", oldRc.Name, oldRc.UID)
	newRc, existed, err := r.getOrCreateTargetController(config.NewRc, sourceId)
	if err != nil {
		return err
	}
	if existed {
		fmt.Fprintf(out, "Continuing update with existing controller %s.\n", newRc.Name)
	} else {
		fmt.Fprintf(out, "Created %s\n", newRc.Name)
	}
	// Extract the desired replica count from the controller.
	desired, err := strconv.Atoi(newRc.Annotations[desiredReplicasAnnotation])
	if err != nil {
		return fmt.Errorf("Unable to parse annotation for %s: %s=%s",
			newRc.Name, desiredReplicasAnnotation, newRc.Annotations[desiredReplicasAnnotation])
	}

	maxIncrement := 1
	// This is the minimum number of replicas (total of old/new) which must
	// remain ready for the duration of the update.
	minReady := oldRc.Spec.Replicas - maxIncrement
	// The default is scale up/down. If scaleUpFirst is false, use down/up (in-
	// place).
	scaleUpFirst := true
	if config.UpdatePercent != nil {
		// Compute the scale increment based on a percentage of the new desired
		// count.
		maxIncrement = int(math.Ceil(float64(desired) * (math.Abs(float64(*config.UpdatePercent)) / 100)))
		// Compute the minimum ready requirement as a percentage of the old
		// replica count.
		percentOld := int(math.Ceil(float64(oldRc.Spec.Replicas) * (math.Abs(float64(*config.UpdatePercent)) / 100)))
		minReady = oldRc.Spec.Replicas - percentOld
		// A negative percentage indicates our scale direction should be
		// down/up instead of the default up/down.
		if *config.UpdatePercent < 0 {
			scaleUpFirst = false
		}
	}
	// Impose a floor of 1 for the minimum ready unless this is a 100% down/up
	// scaling operation. A down/up can't have this floor because it would be
	// impossible to maintain a minimum ready of 1 with a maximum overall pod
	// count of 2 when scaling down first.
	if config.UpdatePercent == nil || (*config.UpdatePercent > 0) {
		minReady = int(math.Max(float64(1), float64(minReady)))
	}
	// Helpful output about what we're about to do.
	direction := "up"
	if !scaleUpFirst {
		direction = "down"
	}
	fmt.Fprintf(out, "Scaling up %s from %d to %d, scaling down %s from %d to 0 (scale %s first by %d each interval, maintain at least %d ready)\n",
		newRc.Name, newRc.Spec.Replicas, desired, oldRc.Name, oldRc.Spec.Replicas, direction, maxIncrement, minReady)

	// Scale newRc and oldRc until newRc has the desired number of replicas and
	// oldRc has 0 replicas.
	increment := maxIncrement
	for newRc.Spec.Replicas != desired || oldRc.Spec.Replicas != 0 {
		if scaleUpFirst {
			// Scale up/down. Initially this means scale up by the max increment.
			scaledRc, err := r.scaleUp(newRc, oldRc, desired, increment, config)
			if err != nil {
				return err
			}
			newRc = scaledRc
			scaleUpFirst = false
		} // Otherwise, scale down/up.

		// Scale down as much as possible while maintaining the minimum ready
		// amount. The scale-down informs us as to how much we can safely scale up
		// next interval.
		scaledRc, newIncrement, err := r.scaleDown(newRc, oldRc, desired, maxIncrement, minReady, config)
		if err != nil {
			return err
		}
		oldRc = scaledRc
		increment = newIncrement

		// Wait between down/up.
		time.Sleep(config.UpdatePeriod)

		// Scale up as much as possible.
		scaledRc, err = r.scaleUp(newRc, oldRc, desired, increment, config)
		if err != nil {
			return err
		}
		newRc = scaledRc
	}

	// Housekeeping.
	return r.cleanup(oldRc, newRc, config)
}

// scaleUp scales up newRc to desired by increment. It will safely no-op as
// necessary when it detects redundancy or other relevant conditions.
func (r *RollingUpdater) scaleUp(newRc, oldRc *api.ReplicationController, desired, increment int, config *RollingUpdaterConfig) (*api.ReplicationController, error) {
	// If we're already at the desired, do nothing.
	if newRc.Spec.Replicas == desired {
		fmt.Printf("Scaling %s up; already at desired, no-op\n", newRc.Name)
		return newRc, nil
	}
	// If the current safe increment is 0, do nothing.
	if increment == 0 {
		fmt.Printf("Scaling %s up; increment is 0, no-op\n", newRc.Name)
		return newRc, nil
	}
	// If the old is already scaled down, go ahead and scale all the way up.
	if oldRc.Spec.Replicas == 0 {
		increment = desired - newRc.Spec.Replicas
	}
	newRc.Spec.Replicas += increment
	// Account for fenceposts.
	if newRc.Spec.Replicas > desired {
		newRc.Spec.Replicas = desired
	}
	// TODO: remove debugging output
	fmt.Printf("Scaling %s up (current=%d, desired=%d, increment=%d)\n", newRc.Name, newRc.Spec.Replicas, desired, increment)
	// Perform the scale-up.
	fmt.Fprintf(config.Out, "Scaling %s up to %d\n", newRc.Name, newRc.Spec.Replicas)
	retryWait := &RetryParams{config.Interval, config.Timeout}
	scaledRc, err := r.scaleAndWait(newRc, retryWait, retryWait)
	if err != nil {
		return nil, err
	}
	return scaledRc, nil
}

// scaleDown scales down oldRc to 0 by increment. It will safely no-op as
// necessary when it detects redundancy or other relevant conditions.
func (r *RollingUpdater) scaleDown(newRc, oldRc *api.ReplicationController, desired, maxIncrement, minReady int, config *RollingUpdaterConfig) (*api.ReplicationController, int, error) {
	// Already scaled down; do nothing.
	if oldRc.Spec.Replicas == 0 {
		fmt.Printf("Scaling %s down; already scaled down, no-op (current=%d)\n", oldRc.Name, oldRc.Spec.Replicas)
		return oldRc, maxIncrement, nil
	}
	oldReady, newReady, ready, err := r.waitForReadyPods(config.Interval, config.Timeout, oldRc, newRc)
	if err != nil {
		return nil, 0, err
	}
	// The increment will be whatever we can while staying above the minimum up
	// to the max.
	increment := int(math.Min(float64(maxIncrement), float64(ready-minReady)))
	// The increment normally shouldn't drop below 0 because the ready count
	// always starts below the old replica count, but the old replica count can
	// decrement due to externalities like pods death in the replica set. This
	// will be considered a transient condition; do nothing and try again later
	// with new readiness values.
	if increment < 0 {
		fmt.Printf("Scaling %s down; readiness deficit, no-op (current=%d, maxIncrement=%d, ready=%d, minReady=%d, oldReady=%d, newReady=%d, increment=%d)\n", oldRc.Name, oldRc.Spec.Replicas, maxIncrement, ready, minReady, oldReady, newReady, increment)
		return oldRc, 0, nil
	}
	// If the most we can scale is 0, it means we can't scale down without
	// violating the minimum. Do nothing and try again later when there are more
	// new ready (or more old ready if we're in a recovery from a negative
	// increment.)
	if increment == 0 {
		fmt.Printf("Scaling %s down; can't increment without minimum violation, no-op (current=%d, maxIncrement=%d, ready=%d, minReady=%d, oldReady=%d, newReady=%d, increment=%d)\n", oldRc.Name, oldRc.Spec.Replicas, maxIncrement, ready, minReady, oldReady, newReady, increment)
		return oldRc, increment, nil
	}
	// Reduce the replica count.
	oldReplicas := oldRc.Spec.Replicas
	oldRc.Spec.Replicas -= increment
	// Account for fenceposts.
	if oldRc.Spec.Replicas < 0 {
		oldRc.Spec.Replicas = 0
	}
	// If the new is already fully scaled and ready up to the desired size, go
	// ahead and scale old all the way down.
	if newRc.Spec.Replicas == desired && newReady == desired {
		oldRc.Spec.Replicas = 0
	}
	fmt.Printf("Scaling %s down fom %d to %d (maxIncrement=%d, ready=%d, minReady=%d, oldReady=%d, newReady=%d, increment=%d)\n", oldRc.Name, oldReplicas, oldRc.Spec.Replicas, maxIncrement, ready, minReady, oldReady, newReady, increment)
	// Perform the scale-down.
	fmt.Fprintf(config.Out, "Scaling %s down to %d\n", oldRc.Name, oldRc.Spec.Replicas)
	retryWait := &RetryParams{config.Interval, config.Timeout}
	scaledRc, err := r.scaleAndWait(oldRc, retryWait, retryWait)
	if err != nil {
		return nil, 0, err
	}
	return scaledRc, increment, nil
}

// scalerScaleAndWait scales a controller using a Scaler and a real client.
func (r *RollingUpdater) scaleAndWaitWithScaler(rc *api.ReplicationController, retry *RetryParams, wait *RetryParams) (*api.ReplicationController, error) {
	scalerClient := NewScalerClient(r.c)
	scaler, err := ScalerFor("ReplicationController", scalerClient)
	if err != nil {
		return nil, fmt.Errorf("Couldn't make scaler: %s", err)
	}
	if err := scaler.Scale(rc.Namespace, rc.Name, uint(rc.Spec.Replicas), &ScalePrecondition{-1, ""}, retry, wait); err != nil {
		return nil, err
	}
	return r.c.ReplicationControllers(rc.Namespace).Get(rc.Name)
}

// pollForReadyPods polls oldRc and newRc each interval and returns the old,
// new, and total ready counts for their pods. If a pod is observed as being
// ready, it's considered ready even if it later becomes unready.
func (r *RollingUpdater) pollForReadyPods(interval, timeout time.Duration, oldRc, newRc *api.ReplicationController) (int, int, int, error) {
	controllers := []*api.ReplicationController{oldRc, newRc}
	ready := map[string]int{
		oldRc.Name: 0,
		newRc.Name: 0,
	}
	err := wait.Poll(interval, timeout, func() (done bool, err error) {
		anyReady := false
		for _, controller := range controllers {
			selector := labels.Set(controller.Spec.Selector).AsSelector()
			pods, err := r.c.Pods(controller.Namespace).List(selector, fields.Everything())
			if err != nil {
				return false, err
			}
			for _, pod := range pods.Items {
				if api.IsPodReady(&pod) {
					ready[controller.Name]++
					anyReady = true
				}
			}
		}
		if anyReady {
			return true, nil
		}
		return false, nil
	})
	oldReady := ready[oldRc.Name]
	newReady := ready[newRc.Name]
	return oldReady, newReady, oldReady + newReady, err
}

// getOrCreateTargetControllerWithClient looks for an existing controller with
// sourceId. If found, the existing controller is returned with true
// indicating that the controller already exists. If the controller isn't
// found, a new one is created and returned along with false indicating the
// controller was created.
//
// Existing controllers are validated to ensure their sourceIdAnnotation
// matches sourceId; if there's a mismatch, an error is returned.
func (r *RollingUpdater) getOrCreateTargetControllerWithClient(controller *api.ReplicationController, sourceId string) (*api.ReplicationController, bool, error) {
	existing, err := r.c.ReplicationControllers(controller.Namespace).Get(controller.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			// There was an error trying to find the controller; don't assume we
			// should create it.
			return nil, false, err
		}
		if controller.Spec.Replicas <= 0 {
			return nil, false, fmt.Errorf("Invalid controller spec for %s; required: > 0 replicas, actual: %s\n", controller.Name, controller.Spec)
		}
		// The controller wasn't found, so create it.
		if controller.ObjectMeta.Annotations == nil {
			controller.ObjectMeta.Annotations = map[string]string{}
		}
		controller.ObjectMeta.Annotations[desiredReplicasAnnotation] = fmt.Sprintf("%d", controller.Spec.Replicas)
		controller.ObjectMeta.Annotations[sourceIdAnnotation] = sourceId
		controller.Spec.Replicas = 0
		newRc, err := r.c.ReplicationControllers(r.ns).Create(controller)
		return newRc, false, err
	}
	// Validate and use the existing controller.
	annotations := existing.ObjectMeta.Annotations
	source := annotations[sourceIdAnnotation]
	_, ok := annotations[desiredReplicasAnnotation]
	if source != sourceId || !ok {
		return nil, false, fmt.Errorf("Missing/unexpected annotations for controller %s, expected %s : %s", controller.Name, sourceId, annotations)
	}
	return existing, true, nil
}

// cleanupWithClients performs cleanup tasks after the deployment. Deployment
// process related annotations are removed from oldRc and newRc. The
// CleanupPolicy on config is executed.
func (r *RollingUpdater) cleanupWithClients(oldRc, newRc *api.ReplicationController, config *RollingUpdaterConfig) error {
	// Clean up annotations
	var err error
	newRc, err = r.c.ReplicationControllers(r.ns).Get(newRc.Name)
	if err != nil {
		return err
	}
	delete(newRc.ObjectMeta.Annotations, sourceIdAnnotation)
	delete(newRc.ObjectMeta.Annotations, desiredReplicasAnnotation)

	newRc, err = r.c.ReplicationControllers(r.ns).Update(newRc)
	if err != nil {
		return err
	}
	scalerClient := NewScalerClient(r.c)
	if err = wait.Poll(config.Interval, config.Timeout, scalerClient.ControllerHasDesiredReplicas(newRc)); err != nil {
		return err
	}
	newRc, err = r.c.ReplicationControllers(r.ns).Get(newRc.Name)
	if err != nil {
		return err
	}

	switch config.CleanupPolicy {
	case DeleteRollingUpdateCleanupPolicy:
		// delete old rc
		fmt.Fprintf(config.Out, "Update succeeded. Deleting %s\n", oldRc.Name)
		return r.c.ReplicationControllers(r.ns).Delete(oldRc.Name)
	case RenameRollingUpdateCleanupPolicy:
		// delete old rc
		fmt.Fprintf(config.Out, "Update succeeded. Deleting old controller: %s\n", oldRc.Name)
		if err := r.c.ReplicationControllers(r.ns).Delete(oldRc.Name); err != nil {
			return err
		}
		fmt.Fprintf(config.Out, "Renaming %s to %s\n", newRc.Name, oldRc.Name)
		return Rename(r.c, newRc, oldRc.Name)
	case PreserveRollingUpdateCleanupPolicy:
		return nil
	default:
		return nil
	}
}

func Rename(c client.ReplicationControllersNamespacer, rc *api.ReplicationController, newName string) error {
	oldName := rc.Name
	rc.Name = newName
	rc.ResourceVersion = ""

	_, err := c.ReplicationControllers(rc.Namespace).Create(rc)
	if err != nil {
		return err
	}
	err = c.ReplicationControllers(rc.Namespace).Delete(oldName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func LoadExistingNextReplicationController(c client.ReplicationControllersNamespacer, namespace, newName string) (*api.ReplicationController, error) {
	if len(newName) == 0 {
		return nil, nil
	}
	newRc, err := c.ReplicationControllers(namespace).Get(newName)
	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	return newRc, err
}

func CreateNewControllerFromCurrentController(c *client.Client, namespace, oldName, newName, image, deploymentKey string) (*api.ReplicationController, error) {
	// load the old RC into the "new" RC
	newRc, err := c.ReplicationControllers(namespace).Get(oldName)
	if err != nil {
		return nil, err
	}

	if len(newRc.Spec.Template.Spec.Containers) > 1 {
		// TODO: support multi-container image update.
		return nil, goerrors.New("Image update is not supported for multi-container pods")
	}
	if len(newRc.Spec.Template.Spec.Containers) == 0 {
		return nil, goerrors.New(fmt.Sprintf("Pod has no containers! (%v)", newRc))
	}
	newRc.Spec.Template.Spec.Containers[0].Image = image

	newHash, err := api.HashObject(newRc, c.Codec)
	if err != nil {
		return nil, err
	}

	if len(newName) == 0 {
		newName = fmt.Sprintf("%s-%s", newRc.Name, newHash)
	}
	newRc.Name = newName

	newRc.Spec.Selector[deploymentKey] = newHash
	newRc.Spec.Template.Labels[deploymentKey] = newHash
	// Clear resource version after hashing so that identical updates get different hashes.
	newRc.ResourceVersion = ""
	return newRc, nil
}

func AbortRollingUpdate(c *RollingUpdaterConfig) {
	// Swap the controllers
	tmp := c.OldRc
	c.OldRc = c.NewRc
	c.NewRc = tmp

	if c.NewRc.Annotations == nil {
		c.NewRc.Annotations = map[string]string{}
	}
	c.NewRc.Annotations[sourceIdAnnotation] = fmt.Sprintf("%s:%s", c.OldRc.Name, c.OldRc.UID)
	desiredSize, found := c.OldRc.Annotations[desiredReplicasAnnotation]
	if found {
		fmt.Printf("Found desired replicas.")
		c.NewRc.Annotations[desiredReplicasAnnotation] = desiredSize
	}
	c.CleanupPolicy = DeleteRollingUpdateCleanupPolicy
}

func GetNextControllerAnnotation(rc *api.ReplicationController) (string, bool) {
	res, found := rc.Annotations[nextControllerAnnotation]
	return res, found
}

func SetNextControllerAnnotation(rc *api.ReplicationController, name string) {
	if rc.Annotations == nil {
		rc.Annotations = map[string]string{}
	}
	rc.Annotations[nextControllerAnnotation] = name
}

func UpdateExistingReplicationController(c client.Interface, oldRc *api.ReplicationController, namespace, newName, deploymentKey, deploymentValue string, out io.Writer) (*api.ReplicationController, error) {
	SetNextControllerAnnotation(oldRc, newName)
	if _, found := oldRc.Spec.Selector[deploymentKey]; !found {
		return AddDeploymentKeyToReplicationController(oldRc, c, deploymentKey, deploymentValue, namespace, out)
	} else {
		// If we didn't need to update the controller for the deployment key, we still need to write
		// the "next" controller.
		return c.ReplicationControllers(namespace).Update(oldRc)
	}
}

const MaxRetries = 3

func AddDeploymentKeyToReplicationController(oldRc *api.ReplicationController, client client.Interface, deploymentKey, deploymentValue, namespace string, out io.Writer) (*api.ReplicationController, error) {
	var err error
	// First, update the template label.  This ensures that any newly created pods will have the new label
	if oldRc, err = updateWithRetries(client.ReplicationControllers(namespace), oldRc, func(rc *api.ReplicationController) {
		if rc.Spec.Template.Labels == nil {
			rc.Spec.Template.Labels = map[string]string{}
		}
		rc.Spec.Template.Labels[deploymentKey] = deploymentValue
	}); err != nil {
		return nil, err
	}

	// Update all pods managed by the rc to have the new hash label, so they are correctly adopted
	// TODO: extract the code from the label command and re-use it here.
	podList, err := client.Pods(namespace).List(labels.SelectorFromSet(oldRc.Spec.Selector), fields.Everything())
	if err != nil {
		return nil, err
	}
	for ix := range podList.Items {
		pod := &podList.Items[ix]
		if pod.Labels == nil {
			pod.Labels = map[string]string{
				deploymentKey: deploymentValue,
			}
		} else {
			pod.Labels[deploymentKey] = deploymentValue
		}
		err = nil
		delay := 3
		for i := 0; i < MaxRetries; i++ {
			_, err = client.Pods(namespace).Update(pod)
			if err != nil {
				fmt.Fprintf(out, "Error updating pod (%v), retrying after %d seconds", err, delay)
				time.Sleep(time.Second * time.Duration(delay))
				delay *= delay
			} else {
				break
			}
		}
		if err != nil {
			return nil, err
		}
	}

	if oldRc.Spec.Selector == nil {
		oldRc.Spec.Selector = map[string]string{}
	}
	// Copy the old selector, so that we can scrub out any orphaned pods
	selectorCopy := map[string]string{}
	for k, v := range oldRc.Spec.Selector {
		selectorCopy[k] = v
	}
	oldRc.Spec.Selector[deploymentKey] = deploymentValue

	// Update the selector of the rc so it manages all the pods we updated above
	if oldRc, err = updateWithRetries(client.ReplicationControllers(namespace), oldRc, func(rc *api.ReplicationController) {
		rc.Spec.Selector[deploymentKey] = deploymentValue
	}); err != nil {
		return nil, err
	}

	// Clean up any orphaned pods that don't have the new label, this can happen if the rc manager
	// doesn't see the update to its pod template and creates a new pod with the old labels after
	// we've finished re-adopting existing pods to the rc.
	podList, err = client.Pods(namespace).List(labels.SelectorFromSet(selectorCopy), fields.Everything())
	for ix := range podList.Items {
		pod := &podList.Items[ix]
		if value, found := pod.Labels[deploymentKey]; !found || value != deploymentValue {
			if err := client.Pods(namespace).Delete(pod.Name, nil); err != nil {
				return nil, err
			}
		}
	}

	return oldRc, nil
}

type updateFunc func(controller *api.ReplicationController)

// updateWithRetries updates applies the given rc as an update.
func updateWithRetries(rcClient client.ReplicationControllerInterface, rc *api.ReplicationController, applyUpdate updateFunc) (*api.ReplicationController, error) {
	// Each update could take ~100ms, so give it 0.5 second
	var err error
	oldRc := rc
	err = wait.Poll(10*time.Millisecond, 500*time.Millisecond, func() (bool, error) {
		// Apply the update, then attempt to push it to the apiserver.
		applyUpdate(rc)
		if rc, err = rcClient.Update(rc); err == nil {
			// rc contains the latest controller post update
			return true, nil
		}
		// Update the controller with the latest resource version, if the update failed we
		// can't trust rc so use oldRc.Name.
		if rc, err = rcClient.Get(oldRc.Name); err != nil {
			// The Get failed: Value in rc cannot be trusted.
			rc = oldRc
		}
		// The Get passed: rc contains the latest controller, expect a poll for the update.
		return false, nil
	})
	// If the error is non-nil the returned controller cannot be trusted, if it is nil, the returned
	// controller contains the applied update.
	return rc, err
}

func FindSourceController(r client.ReplicationControllersNamespacer, namespace, name string) (*api.ReplicationController, error) {
	list, err := r.ReplicationControllers(namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for ix := range list.Items {
		rc := &list.Items[ix]
		if rc.Annotations != nil && strings.HasPrefix(rc.Annotations[sourceIdAnnotation], name) {
			return rc, nil
		}
	}
	return nil, fmt.Errorf("couldn't find a replication controller with source id == %s/%s", namespace, name)
}
