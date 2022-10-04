package ingress

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/openshift/cluster-ingress-operator/pkg/manifests"
	"github.com/openshift/cluster-ingress-operator/pkg/operator/controller"
	"github.com/openshift/cluster-ingress-operator/pkg/util/retryableerror"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	iov1 "github.com/openshift/api/operatoringress/v1"

	oputil "github.com/openshift/cluster-ingress-operator/pkg/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilclock "k8s.io/utils/clock"
)

// clock is to enable unit testing
var clock utilclock.Clock = utilclock.RealClock{}

// expectedCondition contains a condition that is expected to be checked when
// determining Available or Degraded status of the ingress controller
type expectedCondition struct {
	condition string
	status    operatorv1.ConditionStatus
	// ifConditionsTrue is a list of prerequisite conditions that should be true
	// or else the condition is not checked.
	ifConditionsTrue []string
	gracePeriod      time.Duration
}

// syncIngressControllerStatus computes the current status of ic and
// updates status upon any changes since last sync.
func (r *reconciler) syncIngressControllerStatus(ic *operatorv1.IngressController, deployment *appsv1.Deployment, deploymentRef metav1.OwnerReference, pods []corev1.Pod, service *corev1.Service, operandEvents []corev1.Event, wildcardRecord *iov1.DNSRecord, dnsConfig *configv1.DNS, platformStatus *configv1.PlatformStatus) (error, bool) {
	updatedIc := false
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return fmt.Errorf("deployment has invalid spec.selector: %v", err), updatedIc
	}

	secret := &corev1.Secret{}
	secretName := controller.RouterEffectiveDefaultCertificateSecretName(ic, deployment.Namespace)
	if err := r.client.Get(context.TODO(), secretName, secret); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get the default certificate secret %s for ingresscontroller %s/%s: %w", secretName, ic.Namespace, ic.Name, err), updatedIc
	}

	var errs []error

	updated := ic.DeepCopy()
	updated.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	updated.Status.Selector = selector.String()
	updated.Status.TLSProfile = computeIngressTLSProfile(ic.Status.TLSProfile, deployment)

	if updated.Status.EndpointPublishingStrategy != nil && updated.Status.EndpointPublishingStrategy.LoadBalancer != nil {
		updated.Status.EndpointPublishingStrategy.LoadBalancer.AllowedSourceRanges = computeAllowedSourceRanges(service)
	}

	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeDeploymentPodsScheduledCondition(deployment, pods))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeDeploymentAvailableCondition(deployment))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeDeploymentReplicasMinAvailableCondition(deployment))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeDeploymentReplicasAllAvailableCondition(deployment))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeDeploymentRollingOutCondition(deployment))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeLoadBalancerStatus(ic, service, operandEvents)...)
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeLoadBalancerProgressingStatus(ic, service, platformStatus))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeDNSStatus(ic, wildcardRecord, platformStatus, dnsConfig)...)
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeIngressAvailableCondition(updated.Status.Conditions))
	degradedCondition, err := computeIngressDegradedCondition(updated.Status.Conditions, updated.Name)
	errs = append(errs, err)
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeIngressProgressingCondition(updated.Status.Conditions))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, degradedCondition)
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeIngressUpgradeableCondition(ic, deploymentRef, service, platformStatus, secret))
	updated.Status.Conditions = MergeConditions(updated.Status.Conditions, computeIngressEvaluationConditionsDetectedCondition(ic, service))

	updated.Status.Conditions = PruneConditions(updated.Status.Conditions)

	if !IngressStatusesEqual(updated.Status, ic.Status) {
		if err := r.client.Status().Update(context.TODO(), updated); err != nil {
			errs = append(errs, fmt.Errorf("failed to update ingresscontroller status: %v", err))
		} else {
			updatedIc = true
			SetIngressControllerConditionsMetric(updated)
		}
	}

	return retryableerror.NewMaybeRetryableAggregate(errs), updatedIc
}

// syncIngressControllerSelectorStatus syncs the routeSelector and namespaceSelector
// from the spec to the status for tracking selector state.
func (r *reconciler) syncIngressControllerSelectorStatus(ic *operatorv1.IngressController) error {
	// Sync selectors from Spec to Status. This allows us to determine if either of these were updated.
	updated := ic.DeepCopy()
	updated.Status.RouteSelector = ic.Spec.RouteSelector
	updated.Status.NamespaceSelector = ic.Spec.NamespaceSelector

	if err := r.client.Status().Update(context.TODO(), updated); err != nil {
		return fmt.Errorf("failed to update ingresscontroller status: %w", err)
	}

	return nil
}

// MergeConditions adds or updates matching conditions, and updates
// the transition time if details of a condition have changed. Returns
// the updated condition array.
func MergeConditions(conditions []operatorv1.OperatorCondition, updates ...operatorv1.OperatorCondition) []operatorv1.OperatorCondition {
	now := metav1.NewTime(clock.Now())
	var additions []operatorv1.OperatorCondition
	for i, update := range updates {
		add := true
		for j, cond := range conditions {
			if cond.Type == update.Type {
				add = false
				if conditionChanged(cond, update) {
					conditions[j].Status = update.Status
					conditions[j].Reason = update.Reason
					conditions[j].Message = update.Message
					conditions[j].LastTransitionTime = now
					break
				}
			}
		}
		if add {
			updates[i].LastTransitionTime = now
			additions = append(additions, updates[i])
		}
	}
	conditions = append(conditions, additions...)
	return conditions
}

// PruneConditions removes any conditions that are not currently supported.
// Returns the updated condition array.
func PruneConditions(conditions []operatorv1.OperatorCondition) []operatorv1.OperatorCondition {
	for i, condition := range conditions {
		// TODO: Remove this fix-up logic in 4.8
		if condition.Type == "DeploymentDegraded" {
			// DeploymentDegraded was removed in 4.6.0
			conditions = append(conditions[:i], conditions[i+1:]...)
		}
	}
	return conditions
}

// computeIngressTLSProfile computes the ingresscontroller's current TLS
// profile.  If the deployment is ready, then the TLS profile is inferred from
// deployment's pod template spec.  Otherwise the previous TLS profile is used.
func computeIngressTLSProfile(oldProfile *configv1.TLSProfileSpec, deployment *appsv1.Deployment) *configv1.TLSProfileSpec {
	if deployment.Status.Replicas != deployment.Status.UpdatedReplicas {
		return oldProfile
	}

	newProfile := inferTLSProfileSpecFromDeployment(deployment)

	return newProfile
}

// computeAllowedSourceRanges computes the effective AllowedSourceRanges value
// by looking at the LoadBalancerSourceRanges field and service.beta.kubernetes.io/load-balancer-source-ranges
// annotation of the LoadBalancer-typed Service. The field takes precedence over the annotation.
func computeAllowedSourceRanges(service *corev1.Service) []operatorv1.CIDR {
	if service == nil {
		return nil
	}
	cidrs := []operatorv1.CIDR{}
	if len(service.Spec.LoadBalancerSourceRanges) > 0 {
		for _, r := range service.Spec.LoadBalancerSourceRanges {
			cidrs = append(cidrs, operatorv1.CIDR(r))
		}
		return cidrs
	}

	if a, ok := service.Annotations[corev1.AnnotationLoadBalancerSourceRangesKey]; ok {
		a = strings.TrimSpace(a)
		if len(a) > 0 {
			sourceRanges := strings.Split(a, ",")
			for _, r := range sourceRanges {
				cidrs = append(cidrs, operatorv1.CIDR(r))
			}
			return cidrs
		}
	}

	return nil
}

// computeDeploymentPodsScheduledCondition computes the ingress controller's
// current PodsScheduled status condition state by inspecting the PodScheduled
// conditions of the pods associated with the deployment.
func computeDeploymentPodsScheduledCondition(deployment *appsv1.Deployment, pods []corev1.Pod) operatorv1.OperatorCondition {
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil || selector.Empty() {
		return operatorv1.OperatorCondition{
			Type:    IngressControllerPodsScheduledConditionType,
			Status:  operatorv1.ConditionUnknown,
			Reason:  "InvalidLabelSelector",
			Message: "Deployment has an invalid label selector.",
		}
	}
	unscheduled := make(map[*corev1.Pod]corev1.PodCondition)
	for i, pod := range pods {
		if !selector.Matches(labels.Set(pod.Labels)) {
			continue
		}
		for j, cond := range pod.Status.Conditions {
			if cond.Type != corev1.PodScheduled {
				continue
			}
			if cond.Status == corev1.ConditionTrue {
				continue
			}
			unscheduled[&pods[i]] = pod.Status.Conditions[j]
		}
	}
	if len(unscheduled) != 0 {
		var haveUnschedulable bool
		message := "Some pods are not scheduled:"
		// Sort keys so that the result is deterministic.
		keys := make([]*corev1.Pod, 0, len(unscheduled))
		for pod := range unscheduled {
			keys = append(keys, pod)
		}
		sort.Slice(keys, func(i, j int) bool {
			return oputil.ObjectLess(&keys[i].ObjectMeta, &keys[j].ObjectMeta)
		})
		for _, pod := range keys {
			cond := unscheduled[pod]
			if cond.Reason == corev1.PodReasonUnschedulable {
				haveUnschedulable = true
				message = fmt.Sprintf("%s Pod %q cannot be scheduled: %s", message, pod.Name, cond.Message)
			} else {
				message = fmt.Sprintf("%s Pod %q is not yet scheduled: %s: %s", message, pod.Name, cond.Reason, cond.Message)
			}
		}
		if haveUnschedulable {
			message = message + " Make sure you have sufficient worker nodes."
		}
		return operatorv1.OperatorCondition{
			Type:    IngressControllerPodsScheduledConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "PodsNotScheduled",
			Message: message,
		}
	}
	return operatorv1.OperatorCondition{
		Type:   IngressControllerPodsScheduledConditionType,
		Status: operatorv1.ConditionTrue,
	}
}

// computeIngressAvailableCondition computes the ingress controller's current Available status state
// by inspecting the following:
// 1) the Available condition of Deployment,
// 2) the DNSReady condition of the IngressController, and
// 3) the LoadBalancerReady condition of the IngressController.
// The ingresscontroller is judged Available only if all 3 conditions are true
func computeIngressAvailableCondition(conditions []operatorv1.OperatorCondition) operatorv1.OperatorCondition {
	expected := []expectedCondition{
		{
			condition: IngressControllerDeploymentAvailableConditionType,
			status:    operatorv1.ConditionTrue,
		},
		{
			condition: operatorv1.DNSReadyIngressConditionType,
			status:    operatorv1.ConditionTrue,
			ifConditionsTrue: []string{
				operatorv1.LoadBalancerManagedIngressConditionType,
				operatorv1.LoadBalancerReadyIngressConditionType,
				operatorv1.DNSManagedIngressConditionType,
			},
		},
		{
			condition:        operatorv1.LoadBalancerReadyIngressConditionType,
			status:           operatorv1.ConditionTrue,
			ifConditionsTrue: []string{operatorv1.LoadBalancerManagedIngressConditionType},
		},
	}

	// Cover the rare case of no conditions
	if len(conditions) == 0 {
		return operatorv1.OperatorCondition{Type: operatorv1.OperatorStatusTypeAvailable, Status: operatorv1.ConditionFalse}
	}
	_, unavailableConditions, _ := checkConditions(expected, conditions)
	if len(unavailableConditions) != 0 {
		degraded := formatConditions(unavailableConditions)
		return operatorv1.OperatorCondition{
			Type:    operatorv1.IngressControllerAvailableConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "IngressControllerUnavailable",
			Message: "One or more status conditions indicate unavailable: " + degraded,
		}
	}
	return operatorv1.OperatorCondition{
		Type:   operatorv1.IngressControllerAvailableConditionType,
		Status: operatorv1.ConditionTrue,
	}
}

// checkConditions compares expected operator conditions to existing operator
// conditions and returns a list of graceConditions, degradedconditions, and a
// requeueing wait time.
func checkConditions(expectedConds []expectedCondition, conditions []operatorv1.OperatorCondition) ([]*operatorv1.OperatorCondition, []*operatorv1.OperatorCondition, time.Duration) {
	var graceConditions, degradedConditions []*operatorv1.OperatorCondition
	var requeueAfter time.Duration
	conditionsMap := make(map[string]*operatorv1.OperatorCondition)

	for i := range conditions {
		conditionsMap[conditions[i].Type] = &conditions[i]
	}
	now := clock.Now()
	for _, expected := range expectedConds {
		condition, haveCondition := conditionsMap[expected.condition]
		if !haveCondition {
			continue
		}
		if condition.Status == expected.status {
			continue
		}
		failedPredicates := false
		for _, ifCond := range expected.ifConditionsTrue {
			predicate, havePredicate := conditionsMap[ifCond]
			if !havePredicate || predicate.Status != operatorv1.ConditionTrue {
				failedPredicates = true
				break
			}
		}
		if failedPredicates {
			continue
		}
		if expected.gracePeriod != 0 {
			t1 := now.Add(-expected.gracePeriod)
			t2 := condition.LastTransitionTime
			if t2.After(t1) {
				d := t2.Sub(t1)
				if len(graceConditions) == 0 || d < requeueAfter {
					// Recompute status conditions again
					// after the grace period has elapsed.
					requeueAfter = d
				}
				graceConditions = append(graceConditions, condition)
				continue
			}
		}
		degradedConditions = append(degradedConditions, condition)
	}
	return graceConditions, degradedConditions, requeueAfter
}

// computeDeploymentAvailableCondition computes the ingresscontroller's
// "DeploymentAvailable" status condition by examining the status conditions of
// the deployment.  The "DeploymentAvailable" condition is true if the
// deployment's "Available" condition is true.
//
// Note: Due to a defect in the deployment controller, the deployment reports
// Available=True before minimum availability requirements are met (see
// <https://bugzilla.redhat.com/show_bug.cgi?id=1830271#c5>).
func computeDeploymentAvailableCondition(deployment *appsv1.Deployment) operatorv1.OperatorCondition {
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			switch cond.Status {
			case corev1.ConditionFalse:
				return operatorv1.OperatorCondition{
					Type:    IngressControllerDeploymentAvailableConditionType,
					Status:  operatorv1.ConditionFalse,
					Reason:  "DeploymentUnavailable",
					Message: fmt.Sprintf("The deployment has Available status condition set to False (reason: %s) with message: %s", cond.Reason, cond.Message),
				}
			case corev1.ConditionTrue:
				return operatorv1.OperatorCondition{
					Type:    IngressControllerDeploymentAvailableConditionType,
					Status:  operatorv1.ConditionTrue,
					Reason:  "DeploymentAvailable",
					Message: "The deployment has Available status condition set to True",
				}
			}
			break
		}
	}
	return operatorv1.OperatorCondition{
		Type:    IngressControllerDeploymentAvailableConditionType,
		Status:  operatorv1.ConditionUnknown,
		Reason:  "DeploymentAvailabilityUnknown",
		Message: "The deployment has no Available status condition set",
	}
}

// computeDeploymentReplicasMinAvailableCondition computes the
// ingresscontroller's "DeploymentReplicasMinAvailable" status condition by
// examining the number of available replicas reported in the deployment's
// status and the maximum unavailable as configured in the deployment's rolling
// update parameters.  The "DeploymentReplicasMinAvailable" condition is true if
// the number of available replicas is equal to or greater than the number of
// desired replicas less the number minimum available.
func computeDeploymentReplicasMinAvailableCondition(deployment *appsv1.Deployment) operatorv1.OperatorCondition {
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	pointerTo := func(val intstr.IntOrString) *intstr.IntOrString { return &val }
	maxUnavailableIntStr := pointerTo(intstr.FromString("25%"))
	maxSurgeIntStr := pointerTo(intstr.FromString("25%"))
	if deployment.Spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType && deployment.Spec.Strategy.RollingUpdate != nil {
		if deployment.Spec.Strategy.RollingUpdate.MaxUnavailable != nil {
			maxUnavailableIntStr = deployment.Spec.Strategy.RollingUpdate.MaxUnavailable
		}
		if deployment.Spec.Strategy.RollingUpdate.MaxSurge != nil {
			maxSurgeIntStr = deployment.Spec.Strategy.RollingUpdate.MaxSurge
		}
	}
	maxSurge, err := intstr.GetValueFromIntOrPercent(maxSurgeIntStr, int(replicas), true)
	if err != nil {
		return operatorv1.OperatorCondition{
			Type:    IngressControllerDeploymentReplicasMinAvailableConditionType,
			Status:  operatorv1.ConditionUnknown,
			Reason:  "InvalidMaxSurgeValue",
			Message: fmt.Sprintf("invalid value for max surge: %v", err),
		}
	}
	maxUnavailable, err := intstr.GetValueFromIntOrPercent(maxUnavailableIntStr, int(replicas), false)
	if err != nil {
		return operatorv1.OperatorCondition{
			Type:    IngressControllerDeploymentReplicasMinAvailableConditionType,
			Status:  operatorv1.ConditionUnknown,
			Reason:  "InvalidMaxUnavailableValue",
			Message: fmt.Sprintf("invalid value for max unavailable: %v", err),
		}
	}
	if maxSurge == 0 && maxUnavailable == 0 {
		maxUnavailable = 1
	}
	if int(deployment.Status.AvailableReplicas) < int(replicas)-maxUnavailable {
		return operatorv1.OperatorCondition{
			Type:    IngressControllerDeploymentReplicasMinAvailableConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "DeploymentMinimumReplicasNotMet",
			Message: fmt.Sprintf("%d/%d of replicas are available, max unavailable is %d", deployment.Status.AvailableReplicas, replicas, maxUnavailable),
		}
	}

	return operatorv1.OperatorCondition{
		Type:    IngressControllerDeploymentReplicasMinAvailableConditionType,
		Status:  operatorv1.ConditionTrue,
		Reason:  "DeploymentMinimumReplicasMet",
		Message: "Minimum replicas requirement is met",
	}
}

// computeDeploymentReplicasAllAvailableCondition computes the
// ingresscontroller's "DeploymentReplicasAllAvailable" status condition by
// examining the number of available replicas reported in the deployment's
// status.  The "DeploymentReplicasAllAvailable" condition is true if the number
// of available replicas is equal to or greater than the number of desired
// replicas.
func computeDeploymentReplicasAllAvailableCondition(deployment *appsv1.Deployment) operatorv1.OperatorCondition {
	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	if deployment.Status.AvailableReplicas < replicas {
		return operatorv1.OperatorCondition{
			Type:    IngressControllerDeploymentReplicasAllAvailableConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "DeploymentReplicasNotAvailable",
			Message: fmt.Sprintf("%d/%d of replicas are available", deployment.Status.AvailableReplicas, replicas),
		}
	}

	return operatorv1.OperatorCondition{
		Type:    IngressControllerDeploymentReplicasAllAvailableConditionType,
		Status:  operatorv1.ConditionTrue,
		Reason:  "DeploymentReplicasAvailable",
		Message: "All replicas are available",
	}
}

// computeDeploymentRollingOutCondition computes the ingress controller's
// "DeploymentRollingOut" status condition by examining the number of updated
// replicas reported in the deployment's status. The "DeploymentRollingOut"
// condition is true if the number of updated replicas is not equal to the number
// of expected or available replicas.
// See Reference: https://github.com/kubernetes/kubectl/blob/master/pkg/polymorphichelpers/rollout_status.go
func computeDeploymentRollingOutCondition(deployment *appsv1.Deployment) operatorv1.OperatorCondition {
	// If have replicas is less than want replicas, then we are waiting for replicas to be updated.
	if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
		return operatorv1.OperatorCondition{
			Type:   IngressControllerDeploymentRollingOutConditionType,
			Status: operatorv1.ConditionTrue,
			Reason: "DeploymentRollingOut",
			Message: fmt.Sprintf(
				"Waiting for router deployment rollout to finish: %d out of %d new replica(s) have been updated...\n",
				deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas),
		}
	}
	// If have replicas greater than updated replicas, then we are waiting for old replicas to terminate.
	if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
		return operatorv1.OperatorCondition{
			Type:   IngressControllerDeploymentRollingOutConditionType,
			Status: operatorv1.ConditionTrue,
			Reason: "DeploymentRollingOut",
			Message: fmt.Sprintf(
				"Waiting for router deployment rollout to finish: %d old replica(s) are pending termination...\n",
				deployment.Status.Replicas-deployment.Status.UpdatedReplicas),
		}
	}
	// If available replicas less than updated replicas, then we are waiting for updated replicas to become available.
	if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
		return operatorv1.OperatorCondition{
			Type:   IngressControllerDeploymentRollingOutConditionType,
			Status: operatorv1.ConditionTrue,
			Reason: "DeploymentRollingOut",
			Message: fmt.Sprintf(
				"Waiting for router deployment rollout to finish: %d of %d updated replica(s) are available...\n",
				deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas),
		}
	}

	return operatorv1.OperatorCondition{
		Type:    IngressControllerDeploymentRollingOutConditionType,
		Status:  operatorv1.ConditionFalse,
		Reason:  "DeploymentNotRollingOut",
		Message: "Deployment is not actively rolling out",
	}
}

// computeIngressDegradedCondition computes the ingresscontroller's "Degraded"
// status condition, which aggregates other status conditions that can indicate
// a degraded state.  In addition, computeIngressDegradedCondition returns a
// duration value that indicates, if it is non-zero, that the operator should
// reconcile the ingresscontroller again after that period to update its status
// conditions.
func computeIngressDegradedCondition(conditions []operatorv1.OperatorCondition, icName string) (operatorv1.OperatorCondition, error) {
	expectedConditions := []expectedCondition{
		{
			condition: IngressControllerAdmittedConditionType,
			status:    operatorv1.ConditionTrue,
		},
		{
			condition:   IngressControllerPodsScheduledConditionType,
			status:      operatorv1.ConditionTrue,
			gracePeriod: time.Minute * 10,
		},
		{
			condition:   IngressControllerDeploymentAvailableConditionType,
			status:      operatorv1.ConditionTrue,
			gracePeriod: time.Second * 30,
		},
		{
			condition:   IngressControllerDeploymentReplicasMinAvailableConditionType,
			status:      operatorv1.ConditionTrue,
			gracePeriod: time.Second * 60,
		},
		{
			condition:   IngressControllerDeploymentReplicasAllAvailableConditionType,
			status:      operatorv1.ConditionTrue,
			gracePeriod: time.Minute * 60,
		},
		{
			condition:        operatorv1.LoadBalancerReadyIngressConditionType,
			status:           operatorv1.ConditionTrue,
			ifConditionsTrue: []string{operatorv1.LoadBalancerManagedIngressConditionType},
			gracePeriod:      time.Second * 90,
		},
		{
			condition: operatorv1.DNSReadyIngressConditionType,
			status:    operatorv1.ConditionTrue,
			ifConditionsTrue: []string{
				operatorv1.LoadBalancerManagedIngressConditionType,
				operatorv1.LoadBalancerReadyIngressConditionType,
				operatorv1.DNSManagedIngressConditionType,
			},
			gracePeriod: time.Second * 30,
		},
	}

	// Only check the default ingress controller for the canary
	// success status condition.
	if icName == manifests.DefaultIngressControllerName {
		canaryCond := struct {
			condition        string
			status           operatorv1.ConditionStatus
			ifConditionsTrue []string
			gracePeriod      time.Duration
		}{
			condition:   IngressControllerCanaryCheckSuccessConditionType,
			status:      operatorv1.ConditionTrue,
			gracePeriod: time.Second * 60,
		}

		expectedConditions = append(expectedConditions, canaryCond)
	}

	// Cover the rare case of no conditions
	if len(conditions) == 0 {
		return operatorv1.OperatorCondition{Type: operatorv1.OperatorStatusTypeDegraded, Status: operatorv1.ConditionFalse}, nil
	}
	graceConditions, degradedConditions, requeueAfter := checkConditions(expectedConditions, conditions)
	if len(degradedConditions) != 0 {
		// Keep checking conditions every minute while degraded.
		retryAfter := time.Minute

		degraded := formatConditions(degradedConditions)
		condition := operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeDegraded,
			Status:  operatorv1.ConditionTrue,
			Reason:  "DegradedConditions",
			Message: "One or more other status conditions indicate a degraded state: " + degraded,
		}

		return condition, retryableerror.New(errors.New("IngressController is degraded: "+degraded), retryAfter)
	}
	condition := operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeDegraded,
		Status: operatorv1.ConditionFalse,
	}
	var err error
	if len(graceConditions) != 0 {
		var grace string
		for _, cond := range graceConditions {
			grace = grace + fmt.Sprintf(", %s=%s", cond.Type, cond.Status)
		}
		grace = grace[2:]

		err = retryableerror.New(errors.New("IngressController may become degraded soon: "+grace), requeueAfter)
	}
	return condition, err
}

// computeIngressUpgradeableCondition computes the IngressController's "Upgradeable" status condition.
func computeIngressUpgradeableCondition(ic *operatorv1.IngressController, deploymentRef metav1.OwnerReference, service *corev1.Service, platform *configv1.PlatformStatus, secret *corev1.Secret) operatorv1.OperatorCondition {
	var errs []error

	errs = append(errs, checkDefaultCertificate(secret, "*."+ic.Status.Domain))

	if service != nil {
		errs = append(errs, loadBalancerServiceIsUpgradeable(ic, deploymentRef, service, platform))
	}

	if err := kerrors.NewAggregate(errs); err != nil {
		return operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeUpgradeable,
			Status:  operatorv1.ConditionFalse,
			Reason:  "OperandsNotUpgradeable",
			Message: fmt.Sprintf("One or more managed resources are not upgradeable: %s", err),
		}
	}

	return operatorv1.OperatorCondition{
		Type:    operatorv1.OperatorStatusTypeUpgradeable,
		Status:  operatorv1.ConditionTrue,
		Reason:  "Upgradeable",
		Message: "IngressController is upgradeable.",
	}
}

// computeIngressEvaluationConditionsDetectedCondition computes the IngressController's "EvaluationConditionsDetected" status condition.
func computeIngressEvaluationConditionsDetectedCondition(ic *operatorv1.IngressController, service *corev1.Service) operatorv1.OperatorCondition {
	var errs []error

	if service != nil {
		errs = append(errs, loadBalancerServiceEvaluationConditionsDetected(ic, service))
	}

	if err := kerrors.NewAggregate(errs); err != nil {
		return operatorv1.OperatorCondition{
			Type:    IngressControllerEvaluationConditionsDetectedConditionType,
			Status:  operatorv1.ConditionTrue,
			Reason:  "OperandsEvaluationConditionsDetected",
			Message: fmt.Sprintf("One or more managed resources have evaluation conditions: %s", err),
		}
	}

	return operatorv1.OperatorCondition{
		Type:    IngressControllerEvaluationConditionsDetectedConditionType,
		Status:  operatorv1.ConditionFalse,
		Reason:  "NoEvaluationCondition",
		Message: "No evaluation condition is detected.",
	}
}

// checkDefaultCertificate returns an error value indicating whether the default
// certificate is safe for upgrades.  In particular, if the current default
// certificate specifies a Subject Alternative Name (SAN) for the ingress
// domain, then it is safe to upgrade, and the return value is nil.  Otherwise,
// if the certificate has a legacy Common Name (CN) and no SAN, then the return
// value is an error indicating that the certificate must be replaced by one
// with a SAN before upgrading is allowed.  This check is necessary because
// OpenShift 4.10 and newer are built using Go 1.17, which rejects certificates
// without SANs.  Note that this function only checks the validity of the
// certificate insofar as it affects upgrades.
func checkDefaultCertificate(secret *corev1.Secret, domain string) error {
	var certData []byte
	if v, ok := secret.Data["tls.crt"]; !ok {
		return nil
	} else {
		certData = v
	}

	for len(certData) > 0 {
		block, data := pem.Decode(certData)
		if block == nil {
			break
		}
		certData = data
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		foundSAN := false
		for i := range cert.DNSNames {
			if cert.DNSNames[i] == domain {
				foundSAN = true
			}
		}
		if cert.Subject.CommonName == domain && !foundSAN {
			return fmt.Errorf("certificate in secret %s/%s has legacy Common Name (CN) but has no Subject Alternative Name (SAN) for domain: %s", secret.Namespace, secret.Name, domain)
		}
	}

	return nil
}

func formatConditions(conditions []*operatorv1.OperatorCondition) string {
	var formatted string
	if len(conditions) == 0 {
		return ""
	}
	for _, cond := range conditions {
		formatted = formatted + fmt.Sprintf(", %s=%s (%s: %s)", cond.Type, cond.Status, cond.Reason, cond.Message)
	}
	formatted = formatted[2:]
	return formatted
}

// computeIngressProgressingCondition computes the IngressController's current Progressing status state
// by inspecting the following:
// 1) the Deployment Replicas Updated condition of the IngressController
// 2) the LoadBalancer Progressing condition of the IngressController
// The IngressController is judged NOT Progressing only if all 2 conditions are true; otherwise
// it is considered to be Progressing.
func computeIngressProgressingCondition(conditions []operatorv1.OperatorCondition) operatorv1.OperatorCondition {
	expected := []expectedCondition{
		{
			condition:        IngressControllerLoadBalancerProgressingConditionType,
			status:           operatorv1.ConditionFalse,
			ifConditionsTrue: []string{operatorv1.LoadBalancerManagedIngressConditionType},
		},
		{
			condition: IngressControllerDeploymentRollingOutConditionType,
			status:    operatorv1.ConditionFalse,
		},
	}

	// Check for the rare case of no conditions
	if len(conditions) != 0 {
		_, progressingConditions, _ := checkConditions(expected, conditions)
		if len(progressingConditions) != 0 {
			progressing := formatConditions(progressingConditions)
			return operatorv1.OperatorCondition{
				Type:    operatorv1.OperatorStatusTypeProgressing,
				Status:  operatorv1.ConditionTrue,
				Reason:  "IngressControllerProgressing",
				Message: "One or more status conditions indicate progressing: " + progressing,
			}
		}
	}
	return operatorv1.OperatorCondition{
		Type:   operatorv1.OperatorStatusTypeProgressing,
		Status: operatorv1.ConditionFalse,
	}
}

// IngressStatusesEqual compares two IngressControllerStatus values.  Returns true
// if the provided values should be considered equal for the purpose of determining
// whether an update is necessary, false otherwise.
func IngressStatusesEqual(a, b operatorv1.IngressControllerStatus) bool {
	if a.ObservedGeneration != b.ObservedGeneration {
		return false
	}
	if !conditionsEqual(a.Conditions, b.Conditions) || a.AvailableReplicas != b.AvailableReplicas ||
		a.Selector != b.Selector {
		return false
	}
	if !reflect.DeepEqual(a.TLSProfile, b.TLSProfile) {
		return false
	}
	if a.EndpointPublishingStrategy != nil && a.EndpointPublishingStrategy.LoadBalancer != nil &&
		b.EndpointPublishingStrategy != nil && b.EndpointPublishingStrategy.LoadBalancer != nil {
		if !reflect.DeepEqual(a.EndpointPublishingStrategy.LoadBalancer.AllowedSourceRanges, b.EndpointPublishingStrategy.LoadBalancer.AllowedSourceRanges) {
			return false
		}
	}

	return true
}

func conditionsEqual(a, b []operatorv1.OperatorCondition) bool {
	conditionCmpOpts := []cmp.Option{
		cmpopts.EquateEmpty(),
		cmpopts.SortSlices(func(a, b operatorv1.OperatorCondition) bool { return a.Type < b.Type }),
	}

	return cmp.Equal(a, b, conditionCmpOpts...)
}

func conditionChanged(a, b operatorv1.OperatorCondition) bool {
	return a.Status != b.Status || a.Reason != b.Reason || a.Message != b.Message
}

// computeLoadBalancerStatus returns the set of current
// LoadBalancer-prefixed conditions for the given ingress controller, which are
// used later to determine the ingress controller's Degraded or Available status.
func computeLoadBalancerStatus(ic *operatorv1.IngressController, service *corev1.Service, operandEvents []corev1.Event) []operatorv1.OperatorCondition {
	// Compute the LoadBalancerManagedIngressConditionType condition
	if ic.Status.EndpointPublishingStrategy == nil ||
		ic.Status.EndpointPublishingStrategy.Type != operatorv1.LoadBalancerServiceStrategyType {
		return []operatorv1.OperatorCondition{
			{
				Type:    operatorv1.LoadBalancerManagedIngressConditionType,
				Status:  operatorv1.ConditionFalse,
				Reason:  "EndpointPublishingStrategyExcludesManagedLoadBalancer",
				Message: "The configured endpoint publishing strategy does not include a managed load balancer",
			},
		}
	}

	conditions := []operatorv1.OperatorCondition{}

	conditions = append(conditions, operatorv1.OperatorCondition{
		Type:    operatorv1.LoadBalancerManagedIngressConditionType,
		Status:  operatorv1.ConditionTrue,
		Reason:  "WantedByEndpointPublishingStrategy",
		Message: "The endpoint publishing strategy supports a managed load balancer",
	})

	// Compute the LoadBalancerReadyIngressConditionType condition
	switch {
	case service == nil:
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.LoadBalancerReadyIngressConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "ServiceNotFound",
			Message: "The LoadBalancer service resource is missing",
		})
	case isProvisioned(service):
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.LoadBalancerReadyIngressConditionType,
			Status:  operatorv1.ConditionTrue,
			Reason:  "LoadBalancerProvisioned",
			Message: "The LoadBalancer service is provisioned",
		})
	case isPending(service):
		reason := "LoadBalancerPending"
		message := "The LoadBalancer service is pending"

		// Try and find a more specific reason for for the pending status.
		createFailedReason := "SyncLoadBalancerFailed"
		failedLoadBalancerEvents := getEventsByReason(operandEvents, "service-controller", createFailedReason)
		for _, event := range failedLoadBalancerEvents {
			involved := event.InvolvedObject
			if involved.Kind == "Service" && involved.Namespace == service.Namespace && involved.Name == service.Name && involved.UID == service.UID {
				reason = "SyncLoadBalancerFailed"
				message = fmt.Sprintf("The %s component is reporting SyncLoadBalancerFailed events like: %s\n%s",
					event.Source.Component, event.Message, "The kube-controller-manager logs may contain more details.")
				break
			}
		}
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.LoadBalancerReadyIngressConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  reason,
			Message: message,
		})
	}
	return conditions
}

// computeLoadBalancerProgressingStatus returns the LoadBalancerProgressing
// conditions for the given ingress controller. These conditions subsequently determine
// the ingress controller's Progressing status.
func computeLoadBalancerProgressingStatus(ic *operatorv1.IngressController, service *corev1.Service, platform *configv1.PlatformStatus) operatorv1.OperatorCondition {
	// Compute the IngressControllerLoadBalancerProgressingConditionType condition for the LoadBalancer
	if ic.Status.EndpointPublishingStrategy.Type == operatorv1.LoadBalancerServiceStrategyType {
		switch {
		case ic.Status.EndpointPublishingStrategy.LoadBalancer == nil:
			return operatorv1.OperatorCondition{
				Type:    IngressControllerLoadBalancerProgressingConditionType,
				Status:  operatorv1.ConditionUnknown,
				Reason:  "StatusIncomplete",
				Message: "status.endpointPublishingStrategy.loadBalancer is not set.",
			}
		case service == nil:
			return operatorv1.OperatorCondition{
				Type:    IngressControllerLoadBalancerProgressingConditionType,
				Status:  operatorv1.ConditionTrue,
				Reason:  "NoService",
				Message: "LoadBalancer Service not created.",
			}
		default:
			if err := loadBalancerServiceIsProgressing(ic, service, platform); err != nil {
				return operatorv1.OperatorCondition{
					Type:    IngressControllerLoadBalancerProgressingConditionType,
					Status:  operatorv1.ConditionTrue,
					Reason:  "OperandsProgressing",
					Message: fmt.Sprintf("One or more managed resources are progressing: %s", err),
				}
			}
		}
	}
	return operatorv1.OperatorCondition{
		Type:    IngressControllerLoadBalancerProgressingConditionType,
		Status:  operatorv1.ConditionFalse,
		Reason:  "LoadBalancerNotProgressing",
		Message: "LoadBalancer is not progressing",
	}
}

func isProvisioned(service *corev1.Service) bool {
	ingresses := service.Status.LoadBalancer.Ingress
	return len(ingresses) > 0 && (len(ingresses[0].Hostname) > 0 || len(ingresses[0].IP) > 0)
}

func isPending(service *corev1.Service) bool {
	return !isProvisioned(service)
}

func getEventsByReason(events []corev1.Event, component, reason string) []corev1.Event {
	var filtered []corev1.Event
	for i := range events {
		event := events[i]
		if event.Source.Component == component && event.Reason == reason {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func computeDNSStatus(ic *operatorv1.IngressController, wildcardRecord *iov1.DNSRecord, status *configv1.PlatformStatus, dnsConfig *configv1.DNS) []operatorv1.OperatorCondition {
	if dnsConfig.Spec.PublicZone == nil && dnsConfig.Spec.PrivateZone == nil {
		return []operatorv1.OperatorCondition{
			{
				Type:    operatorv1.DNSManagedIngressConditionType,
				Status:  operatorv1.ConditionFalse,
				Reason:  "NoDNSZones",
				Message: "No DNS zones are defined in the cluster dns config.",
			},
		}
	}

	if ic.Status.EndpointPublishingStrategy.Type != operatorv1.LoadBalancerServiceStrategyType {
		return []operatorv1.OperatorCondition{
			{
				Type:    operatorv1.DNSManagedIngressConditionType,
				Status:  operatorv1.ConditionFalse,
				Reason:  "UnsupportedEndpointPublishingStrategy",
				Message: "The endpoint publishing strategy doesn't support DNS management.",
			},
		}
	}
	var conditions []operatorv1.OperatorCondition
	if ic.Status.EndpointPublishingStrategy.LoadBalancer.DNSManagementPolicy == operatorv1.UnmanagedLoadBalancerDNS {
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.DNSManagedIngressConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "UnmanagedLoadBalancerDNS",
			Message: "The DNS management policy is set to Unmanaged.",
		})
	} else {
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.DNSManagedIngressConditionType,
			Status:  operatorv1.ConditionTrue,
			Reason:  "Normal",
			Message: "DNS management is supported and zones are specified in the cluster DNS config.",
		})
	}

	switch {
	case wildcardRecord == nil:
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.DNSReadyIngressConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "RecordNotFound",
			Message: "The wildcard record resource was not found.",
		})
	case wildcardRecord.Spec.DNSManagementPolicy == iov1.UnmanagedDNS:
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.DNSReadyIngressConditionType,
			Status:  operatorv1.ConditionUnknown,
			Reason:  "UnmanagedDNS",
			Message: "The DNS management policy is set to Unmanaged.",
		})
	case len(wildcardRecord.Status.Zones) == 0:
		conditions = append(conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.DNSReadyIngressConditionType,
			Status:  operatorv1.ConditionFalse,
			Reason:  "NoZones",
			Message: "The record isn't present in any zones.",
		})
	case len(wildcardRecord.Status.Zones) > 0:
		var failedZones []configv1.DNSZone
		var unknownZones []configv1.DNSZone
		for _, zone := range wildcardRecord.Status.Zones {
			for _, cond := range zone.Conditions {
				if cond.Type != iov1.DNSRecordPublishedConditionType {
					continue
				}
				if !checkZoneInConfig(dnsConfig, zone.DNSZone) {
					continue
				}
				switch cond.Status {
				case string(operatorv1.ConditionFalse):
					// check to see if the zone is in the dnsConfig.Spec
					// fix:BZ1942657 - relates to status changes when updating DNS PrivateZone config
					failedZones = append(failedZones, zone.DNSZone)
				case string(operatorv1.ConditionUnknown):
					unknownZones = append(unknownZones, zone.DNSZone)
				}
			}
		}
		if len(failedZones) != 0 {
			// TODO: Add failed condition reasons
			conditions = append(conditions, operatorv1.OperatorCondition{
				Type:    operatorv1.DNSReadyIngressConditionType,
				Status:  operatorv1.ConditionFalse,
				Reason:  "FailedZones",
				Message: fmt.Sprintf("The record failed to provision in some zones: %v", failedZones),
			})
		} else if len(unknownZones) != 0 {
			// This condition is an edge case where DNSManaged=True but
			// there was an internal error during publishing record.
			conditions = append(conditions, operatorv1.OperatorCondition{
				Type:    operatorv1.DNSReadyIngressConditionType,
				Status:  operatorv1.ConditionFalse,
				Reason:  "UnknownZones",
				Message: fmt.Sprintf("Provisioning of the record is in an unknown state in some zones: %v", unknownZones),
			})
		} else {
			conditions = append(conditions, operatorv1.OperatorCondition{
				Type:    operatorv1.DNSReadyIngressConditionType,
				Status:  operatorv1.ConditionTrue,
				Reason:  "NoFailedZones",
				Message: "The record is provisioned in all reported zones.",
			})
		}
	}

	return conditions
}

// checkZoneInConfig - private utility to check for a zone in the current config
func checkZoneInConfig(dnsConfig *configv1.DNS, zone configv1.DNSZone) bool {
	// check PrivateZone settings only
	// check for private zone ID
	if dnsConfig.Spec.PrivateZone != nil && dnsConfig.Spec.PrivateZone.ID != "" && zone.ID != "" {
		if dnsConfig.Spec.PrivateZone.ID == zone.ID {
			return true
		}
	}

	// check for private zone Tags
	if dnsConfig.Spec.PrivateZone != nil && dnsConfig.Spec.PrivateZone.Tags["Name"] != "" && zone.Tags["Name"] != "" {
		if dnsConfig.Spec.PrivateZone.Tags["Name"] == zone.Tags["Name"] {
			return true
		}
	}

	return false
}
