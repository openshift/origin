package staticpodstate

import (
	"context"
	"fmt"
	"strings"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

var (
	staticPodStateControllerWorkQueueKey = "key"
)

// StaticPodStateController is a controller that watches static pods and will produce a failing status if the
//// static pods start crashing for some reason.
type StaticPodStateController struct {
	targetNamespace   string
	staticPodName     string
	operandName       string
	operatorNamespace string

	operatorClient  v1helpers.StaticPodOperatorClient
	configMapGetter corev1client.ConfigMapsGetter
	podsGetter      corev1client.PodsGetter
	versionRecorder status.VersionGetter
}

// NewStaticPodStateController creates a controller that watches static pods and will produce a failing status if the
// static pods start crashing for some reason.
func NewStaticPodStateController(
	targetNamespace, staticPodName, operatorNamespace, operandName string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorClient v1helpers.StaticPodOperatorClient,
	configMapGetter corev1client.ConfigMapsGetter,
	podsGetter corev1client.PodsGetter,
	versionRecorder status.VersionGetter,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &StaticPodStateController{
		targetNamespace:   targetNamespace,
		staticPodName:     staticPodName,
		operandName:       operandName,
		operatorNamespace: operatorNamespace,
		operatorClient:    operatorClient,
		configMapGetter:   configMapGetter,
		podsGetter:        podsGetter,
		versionRecorder:   versionRecorder,
	}
	return factory.New().WithInformers(operatorClient.Informer(), kubeInformersForTargetNamespace.Core().V1().Pods().Informer()).WithSync(c.sync).ToController("StaticPodStateController", eventRecorder)
}

func (c *StaticPodStateController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	operatorSpec, originalOperatorStatus, _, err := c.operatorClient.GetStaticPodOperatorState()
	if err != nil {
		return err
	}

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	errs := []error{}
	failingErrorCount := 0
	images := sets.NewString()
	for _, node := range originalOperatorStatus.NodeStatuses {
		pod, err := c.podsGetter.Pods(c.targetNamespace).Get(ctx, mirrorPodNameForNode(c.staticPodName, node.NodeName), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, err)
			failingErrorCount++
			continue
		}
		images.Insert(pod.Spec.Containers[0].Image)

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				// When container is not ready, we can't determine whether the operator is failing or not and every container will become not
				// ready when created, so do not blip the failing state for it.
				// We will still reflect the container not ready state in error conditions, but we don't set the operator as failed.
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is not ready", node.NodeName, pod.Name, containerStatus.Name))
			}
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason != "PodInitializing" {
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is waiting: %q - %q", node.NodeName, pod.Name, containerStatus.Name, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message))
				failingErrorCount++
			}
			if containerStatus.State.Terminated != nil {
				// Containers can be terminated gracefully to trigger certificate reload, do not report these as failures.
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is terminated: %q - %q", node.NodeName, pod.Name, containerStatus.Name, containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message))
				// Only in case when the termination was caused by error.
				if containerStatus.State.Terminated.ExitCode != 0 {
					failingErrorCount++
				}

			}
		}
	}

	if len(images) == 0 {
		syncCtx.Recorder().Warningf("MissingVersion", "no image found for operand pod")
	} else if len(images) > 1 {
		syncCtx.Recorder().Eventf("MultipleVersions", "multiple versions found, probably in transition: %v", strings.Join(images.List(), ","))
	} else {
		c.versionRecorder.SetVersion(
			c.operandName,
			status.VersionForOperandFromEnv(),
		)
		c.versionRecorder.SetVersion(
			"operator",
			status.VersionForOperatorFromEnv(),
		)
	}

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   condition.StaticPodsDegradedConditionType,
		Status: operatorv1.ConditionFalse,
	}
	// Failing errors
	if failingErrorCount > 0 {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = v1helpers.NewMultiLineAggregate(errs).Error()
	}
	// Not failing errors
	if failingErrorCount == 0 && len(errs) > 0 {
		cond.Reason = "Error"
		cond.Message = v1helpers.NewMultiLineAggregate(errs).Error()
	}
	if _, _, updateError := v1helpers.UpdateStaticPodStatus(c.operatorClient, v1helpers.UpdateStaticPodConditionFn(cond), v1helpers.UpdateStaticPodConditionFn(cond)); updateError != nil {
		return updateError
	}

	return err
}

func mirrorPodNameForNode(staticPodName, nodeName string) string {
	return staticPodName + "-" + nodeName
}
