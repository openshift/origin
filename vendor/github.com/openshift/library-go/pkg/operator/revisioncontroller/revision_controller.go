package revisioncontroller

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const revisionControllerWorkQueueKey = "key"

// LatestRevisionClient is an operator client for an operator status with a latest revision field.
type LatestRevisionClient interface {
	v1helpers.OperatorClient

	// GetLatestRevisionState returns the spec, status and latest revision.
	GetLatestRevisionState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, rev int32, rv string, err error)
	// UpdateLatestRevisionOperatorStatus updates the status with the given latestAvailableRevision and the by applying the given updateFuncs.
	UpdateLatestRevisionOperatorStatus(latestAvailableRevision int32, updateFuncs ...v1helpers.UpdateStatusFunc) (*operatorv1.OperatorStatus, bool, error)
}

// RevisionController is a controller that watches a set of configmaps and secrets and them against a revision snapshot
// of them. If the original resources changes, the revision counter is increased, stored in LatestAvailableRevision
// field of the operator config and new snapshots suffixed by the revision are created.
type RevisionController struct {
	targetNamespace string
	// configMaps is the list of configmaps that are directly copied.A different actor/controller modifies these.
	// the first element should be the configmap that contains the static pod manifest
	configMaps []RevisionResource
	// secrets is a list of secrets that are directly copied for the current values.  A different actor/controller modifies these.
	secrets []RevisionResource

	operatorClient  LatestRevisionClient
	configMapGetter corev1client.ConfigMapsGetter
	secretGetter    corev1client.SecretsGetter
}

type RevisionResource struct {
	Name     string
	Optional bool
}

// NewRevisionController create a new revision controller.
func NewRevisionController(
	targetNamespace string,
	configMaps []RevisionResource,
	secrets []RevisionResource,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorClient LatestRevisionClient,
	configMapGetter corev1client.ConfigMapsGetter,
	secretGetter corev1client.SecretsGetter,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &RevisionController{
		targetNamespace: targetNamespace,
		configMaps:      configMaps,
		secrets:         secrets,

		operatorClient:  operatorClient,
		configMapGetter: configMapGetter,
		secretGetter:    secretGetter,
	}

	return factory.New().WithInformers(
		operatorClient.Informer(),
		kubeInformersForTargetNamespace.Core().V1().ConfigMaps().Informer(),
		kubeInformersForTargetNamespace.Core().V1().Secrets().Informer()).WithSync(c.sync).ToController("RevisionController", eventRecorder)
}

// createRevisionIfNeeded takes care of creating content for the static pods to use.
// returns whether or not requeue and if an error happened when updating status.  Normally it updates status itself.
func (c RevisionController) createRevisionIfNeeded(recorder events.Recorder, latestAvailableRevision int32, resourceVersion string) (bool, error) {
	isLatestRevisionCurrent, reason := c.isLatestRevisionCurrent(latestAvailableRevision)

	// check to make sure that the latestRevision has the exact content we expect.  No mutation here, so we start creating the next Revision only when it is required
	if isLatestRevisionCurrent {
		return false, nil
	}

	nextRevision := latestAvailableRevision + 1
	recorder.Eventf("RevisionTriggered", "new revision %d triggered by %q", nextRevision, reason)
	if err := c.createNewRevision(recorder, nextRevision); err != nil {
		cond := operatorv1.OperatorCondition{
			Type:    "RevisionControllerDegraded",
			Status:  operatorv1.ConditionTrue,
			Reason:  "ContentCreationError",
			Message: err.Error(),
		}
		if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
			recorder.Warningf("RevisionCreateFailed", "Failed to create revision %d: %v", nextRevision, err.Error())
			return true, updateError
		}
		return true, nil
	}

	cond := operatorv1.OperatorCondition{
		Type:   "RevisionControllerDegraded",
		Status: operatorv1.ConditionFalse,
	}
	if _, updated, updateError := c.operatorClient.UpdateLatestRevisionOperatorStatus(nextRevision, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		return true, updateError
	} else if updated {
		recorder.Eventf("RevisionCreate", "Revision %d created because %s", latestAvailableRevision, reason)
	}

	return false, nil
}

func nameFor(name string, revision int32) string {
	return fmt.Sprintf("%s-%d", name, revision)
}

// isLatestRevisionCurrent returns whether the latest revision is up to date and an optional reason
func (c RevisionController) isLatestRevisionCurrent(revision int32) (bool, string) {
	configChanges := []string{}
	for _, cm := range c.configMaps {
		requiredData := map[string]string{}
		existingData := map[string]string{}

		required, err := c.configMapGetter.ConfigMaps(c.targetNamespace).Get(context.TODO(), cm.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) && !cm.Optional {
			return false, err.Error()
		}
		existing, err := c.configMapGetter.ConfigMaps(c.targetNamespace).Get(context.TODO(), nameFor(cm.Name, revision), metav1.GetOptions{})
		if apierrors.IsNotFound(err) && !cm.Optional {
			return false, err.Error()
		}
		if required != nil {
			requiredData = required.Data
		}
		if existing != nil {
			existingData = existing.Data
		}
		if !equality.Semantic.DeepEqual(existingData, requiredData) {
			if klog.V(4) {
				klog.Infof("configmap %q changes for revision %d: %s", cm.Name, revision, resourceapply.JSONPatchNoError(existing, required))
			}
			configChanges = append(configChanges, fmt.Sprintf("configmap/%s has changed", cm.Name))
		}
	}

	secretChanges := []string{}
	for _, s := range c.secrets {
		requiredData := map[string][]byte{}
		existingData := map[string][]byte{}

		required, err := c.secretGetter.Secrets(c.targetNamespace).Get(context.TODO(), s.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) && !s.Optional {
			return false, err.Error()
		}
		existing, err := c.secretGetter.Secrets(c.targetNamespace).Get(context.TODO(), nameFor(s.Name, revision), metav1.GetOptions{})
		if apierrors.IsNotFound(err) && !s.Optional {
			return false, err.Error()
		}
		if required != nil {
			requiredData = required.Data
		}
		if existing != nil {
			existingData = existing.Data
		}
		if !equality.Semantic.DeepEqual(existingData, requiredData) {
			if klog.V(4) {
				klog.Infof("Secret %q changes for revision %d: %s", s.Name, revision, resourceapply.JSONPatchSecretNoError(existing, required))
			}
			secretChanges = append(secretChanges, fmt.Sprintf("secret/%s has changed", s.Name))
		}
	}

	if len(secretChanges) > 0 || len(configChanges) > 0 {
		return false, strings.Join(append(secretChanges, configChanges...), ",")
	}

	return true, ""
}

func (c RevisionController) createNewRevision(recorder events.Recorder, revision int32) error {
	statusConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.targetNamespace,
			Name:      nameFor("revision-status", revision),
		},
		Data: map[string]string{
			"status":   "InProgress",
			"revision": fmt.Sprintf("%d", revision),
		},
	}
	statusConfigMap, _, err := resourceapply.ApplyConfigMap(c.configMapGetter, recorder, statusConfigMap)
	if err != nil {
		return err
	}
	ownerRefs := []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       statusConfigMap.Name,
		UID:        statusConfigMap.UID,
	}}

	for _, cm := range c.configMaps {
		obj, _, err := resourceapply.SyncConfigMap(c.configMapGetter, recorder, c.targetNamespace, cm.Name, c.targetNamespace, nameFor(cm.Name, revision), ownerRefs)
		if err != nil {
			return err
		}
		if obj == nil && !cm.Optional {
			return apierrors.NewNotFound(corev1.Resource("configmaps"), cm.Name)
		}
	}
	for _, s := range c.secrets {
		obj, _, err := resourceapply.SyncSecret(c.secretGetter, recorder, c.targetNamespace, s.Name, c.targetNamespace, nameFor(s.Name, revision), ownerRefs)
		if err != nil {
			return err
		}
		if obj == nil && !s.Optional {
			return apierrors.NewNotFound(corev1.Resource("secrets"), s.Name)
		}
	}

	return nil
}

// getLatestAvailableRevision returns the latest known revision to the operator
// This is either the LatestAvailableRevision in the status or by checking revision status configmaps
func (c RevisionController) getLatestAvailableRevision(operatorStatus *operatorv1.OperatorStatus) (int32, error) {
	configMaps, err := c.configMapGetter.ConfigMaps(c.targetNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	var latestRevision int32
	for _, configMap := range configMaps.Items {
		if !strings.HasPrefix(configMap.Name, "revision-status-") {
			continue
		}
		if revision, ok := configMap.Data["revision"]; ok {
			revisionNumber, err := strconv.Atoi(revision)
			if err != nil {
				return 0, err
			}
			if int32(revisionNumber) > latestRevision {
				latestRevision = int32(revisionNumber)
			}
		}
	}
	// If there are no configmaps, then this should actually be revision 0
	return latestRevision, nil
}

func (c RevisionController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	operatorSpec, originalOperatorStatus, latestAvailableRevision, resourceVersion, err := c.operatorClient.GetLatestRevisionState()
	if err != nil {
		return err
	}
	operatorStatus := originalOperatorStatus.DeepCopy()

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	// If the operator status has 0 as its latest available revision, this is either the first revision
	// or possibly the operator resource was deleted and reset back to 0, which is not what we want so check configmaps
	if latestAvailableRevision == 0 {
		// Check to see if current revision is accurate and if not, search through configmaps for latest revision
		latestRevision, err := c.getLatestAvailableRevision(operatorStatus)
		if err != nil {
			return err
		}
		if latestRevision != 0 {
			// Then make sure that revision number is what's in the operator status
			_, _, err = c.operatorClient.UpdateLatestRevisionOperatorStatus(latestRevision)
			// If we made a change return and requeue with the correct status
			return fmt.Errorf("synthetic requeue request (err: %v)", err)
		}
	}

	requeue, syncErr := c.createRevisionIfNeeded(syncCtx.Recorder(), latestAvailableRevision, resourceVersion)
	if requeue && syncErr == nil {
		return fmt.Errorf("synthetic requeue request (err: %v)", syncErr)
	}
	err = syncErr

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   condition.RevisionControllerDegradedConditionType,
		Status: operatorv1.ConditionFalse,
	}
	if err != nil {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = err.Error()
	}
	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return err
}
