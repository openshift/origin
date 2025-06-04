package v1helpers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

// SetOperandVersion sets the new version and returns the previous value.
func SetOperandVersion(versions *[]configv1.OperandVersion, operandVersion configv1.OperandVersion) string {
	if versions == nil {
		versions = &[]configv1.OperandVersion{}
	}
	existingVersion := FindOperandVersion(*versions, operandVersion.Name)
	if existingVersion == nil {
		*versions = append(*versions, operandVersion)
		return ""
	}

	previous := existingVersion.Version
	existingVersion.Version = operandVersion.Version
	return previous
}

func FindOperandVersion(versions []configv1.OperandVersion, name string) *configv1.OperandVersion {
	if versions == nil {
		return nil
	}
	for i := range versions {
		if versions[i].Name == name {
			return &versions[i]
		}
	}
	return nil
}

func SetOperatorCondition(conditions *[]operatorv1.OperatorCondition, newCondition operatorv1.OperatorCondition) {
	if conditions == nil {
		conditions = &[]operatorv1.OperatorCondition{}
	}
	existingCondition := FindOperatorCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		*conditions = append(*conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

func RemoveOperatorCondition(conditions *[]operatorv1.OperatorCondition, conditionType string) {
	if conditions == nil {
		conditions = &[]operatorv1.OperatorCondition{}
	}
	newConditions := []operatorv1.OperatorCondition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	*conditions = newConditions
}

func FindOperatorCondition(conditions []operatorv1.OperatorCondition, conditionType string) *operatorv1.OperatorCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsOperatorConditionTrue(conditions []operatorv1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorv1.ConditionTrue)
}

func IsOperatorConditionFalse(conditions []operatorv1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorv1.ConditionFalse)
}

func IsOperatorConditionPresentAndEqual(conditions []operatorv1.OperatorCondition, conditionType string, status operatorv1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

// UpdateOperatorSpecFunc is a func that mutates an operator spec.
type UpdateOperatorSpecFunc func(spec *operatorv1.OperatorSpec) error

// UpdateSpec applies the update funcs to the oldStatus and tries to update via the client.
func UpdateSpec(ctx context.Context, client OperatorClient, updateFuncs ...UpdateOperatorSpecFunc) (*operatorv1.OperatorSpec, bool, error) {
	updated := false
	var operatorSpec *operatorv1.OperatorSpec
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		oldSpec, _, resourceVersion, err := client.GetOperatorState()
		if err != nil {
			return err
		}

		newSpec := oldSpec.DeepCopy()
		for _, update := range updateFuncs {
			if err := update(newSpec); err != nil {
				return err
			}
		}

		if equality.Semantic.DeepEqual(oldSpec, newSpec) {
			return nil
		}

		operatorSpec, _, err = client.UpdateOperatorSpec(ctx, resourceVersion, newSpec)
		updated = err == nil
		return err
	})

	return operatorSpec, updated, err
}

// UpdateObservedConfigFn returns a func to update the config.
func UpdateObservedConfigFn(config map[string]interface{}) UpdateOperatorSpecFunc {
	return func(oldSpec *operatorv1.OperatorSpec) error {
		oldSpec.ObservedConfig = runtime.RawExtension{Object: &unstructured.Unstructured{Object: config}}
		return nil
	}
}

// UpdateStatusFunc is a func that mutates an operator status.
type UpdateStatusFunc func(status *operatorv1.OperatorStatus) error

// UpdateStatus applies the update funcs to the oldStatus and tries to update via the client.
func UpdateStatus(ctx context.Context, client OperatorClient, updateFuncs ...UpdateStatusFunc) (*operatorv1.OperatorStatus, bool, error) {
	updated := false
	var updatedOperatorStatus *operatorv1.OperatorStatus
	numberOfAttempts := 0
	previousResourceVersion := ""
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		defer func() {
			numberOfAttempts++
		}()
		var oldStatus *operatorv1.OperatorStatus
		var resourceVersion string
		var err error

		// prefer lister if we haven't already failed.
		_, oldStatus, resourceVersion, err = client.GetOperatorState()
		if err != nil {
			return err
		}
		if resourceVersion == previousResourceVersion {
			listerResourceVersion := resourceVersion
			// this indicates that we've had a conflict and the lister has not caught up, so do a live GET
			_, oldStatus, resourceVersion, err = client.GetOperatorStateWithQuorum(ctx)
			if err != nil {
				return err
			}
			klog.V(2).Infof("lister was stale at resourceVersion=%v, live get showed resourceVersion=%v", listerResourceVersion, resourceVersion)
		}
		previousResourceVersion = resourceVersion

		newStatus := oldStatus.DeepCopy()
		for _, update := range updateFuncs {
			if err := update(newStatus); err != nil {
				return err
			}
		}

		if equality.Semantic.DeepEqual(oldStatus, newStatus) {
			// We return the newStatus which is a deep copy of oldStatus but with all update funcs applied.
			updatedOperatorStatus = newStatus
			return nil
		}
		if klog.V(4).Enabled() {
			klog.Infof("Operator status changed: %v", operatorStatusJSONPatchNoError(oldStatus, newStatus))
		}

		updatedOperatorStatus, err = client.UpdateOperatorStatus(ctx, resourceVersion, newStatus)
		updated = err == nil
		return err
	})

	return updatedOperatorStatus, updated, err
}

func operatorStatusJSONPatchNoError(original, modified *operatorv1.OperatorStatus) string {
	if original == nil {
		return "original object is nil"
	}
	if modified == nil {
		return "modified object is nil"
	}

	return cmp.Diff(original, modified)
}

// UpdateConditionFn returns a func to update a condition.
func UpdateConditionFn(cond operatorv1.OperatorCondition) UpdateStatusFunc {
	return func(oldStatus *operatorv1.OperatorStatus) error {
		SetOperatorCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

// UpdateStaticPodStatusFunc is a func that mutates an operator status.
type UpdateStaticPodStatusFunc func(status *operatorv1.StaticPodOperatorStatus) error

// UpdateStaticPodStatus applies the update funcs to the oldStatus abd tries to update via the client.
func UpdateStaticPodStatus(ctx context.Context, client StaticPodOperatorClient, updateFuncs ...UpdateStaticPodStatusFunc) (*operatorv1.StaticPodOperatorStatus, bool, error) {
	updated := false
	var updatedOperatorStatus *operatorv1.StaticPodOperatorStatus
	numberOfAttempts := 0
	previousResourceVersion := ""
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		defer func() {
			numberOfAttempts++
		}()
		var oldStatus *operatorv1.StaticPodOperatorStatus
		var resourceVersion string
		var err error

		// prefer lister if we haven't already failed.
		_, oldStatus, resourceVersion, err = client.GetStaticPodOperatorState()
		if err != nil {
			return err
		}
		if resourceVersion == previousResourceVersion {
			listerResourceVersion := resourceVersion
			// this indicates that we've had a conflict and the lister has not caught up, so do a live GET
			_, oldStatus, resourceVersion, err = client.GetStaticPodOperatorStateWithQuorum(ctx)
			if err != nil {
				return err
			}
			klog.V(2).Infof("lister was stale at resourceVersion=%v, live get showed resourceVersion=%v", listerResourceVersion, resourceVersion)
		}
		previousResourceVersion = resourceVersion

		newStatus := oldStatus.DeepCopy()
		for _, update := range updateFuncs {
			if err := update(newStatus); err != nil {
				return err
			}
		}

		if equality.Semantic.DeepEqual(oldStatus, newStatus) {
			// We return the newStatus which is a deep copy of oldStatus but with all update funcs applied.
			updatedOperatorStatus = newStatus
			return nil
		}
		if klog.V(4).Enabled() {
			klog.Infof("Operator status changed: %v", staticPodOperatorStatusJSONPatchNoError(oldStatus, newStatus))
		}

		updatedOperatorStatus, err = client.UpdateStaticPodOperatorStatus(ctx, resourceVersion, newStatus)
		updated = err == nil
		return err
	})

	return updatedOperatorStatus, updated, err
}

func staticPodOperatorStatusJSONPatchNoError(original, modified *operatorv1.StaticPodOperatorStatus) string {
	if original == nil {
		return "original object is nil"
	}
	if modified == nil {
		return "modified object is nil"
	}
	return cmp.Diff(original, modified)
}

// UpdateStaticPodConditionFn returns a func to update a condition.
func UpdateStaticPodConditionFn(cond operatorv1.OperatorCondition) UpdateStaticPodStatusFunc {
	return func(oldStatus *operatorv1.StaticPodOperatorStatus) error {
		SetOperatorCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

// EnsureFinalizer adds a new finalizer to the operator CR, if it does not exists. No-op otherwise.
// The finalizer name is computed from the controller name and operator name ($OPERATOR_NAME or os.Args[0])
// It re-tries on conflicts.
func EnsureFinalizer(ctx context.Context, client OperatorClientWithFinalizers, controllerName string) error {
	finalizer := getFinalizerName(controllerName)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return client.EnsureFinalizer(ctx, finalizer)
	})
	return err
}

// RemoveFinalizer removes a finalizer from the operator CR, if it is there. No-op otherwise.
// The finalizer name is computed from the controller name and operator name ($OPERATOR_NAME or os.Args[0])
// It re-tries on conflicts.
func RemoveFinalizer(ctx context.Context, client OperatorClientWithFinalizers, controllerName string) error {
	finalizer := getFinalizerName(controllerName)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return client.RemoveFinalizer(ctx, finalizer)
	})
	return err
}

// getFinalizerName computes a nice finalizer name from controllerName and the operator name ($OPERATOR_NAME or os.Args[0]).
func getFinalizerName(controllerName string) string {
	return fmt.Sprintf("%s.operator.openshift.io/%s", getOperatorName(), controllerName)
}

func getOperatorName() string {
	if name := os.Getenv("OPERATOR_NAME"); name != "" {
		return name
	}
	return os.Args[0]
}

type aggregate []error

var _ utilerrors.Aggregate = aggregate{}

// NewMultiLineAggregate returns an aggregate error with multi-line output
func NewMultiLineAggregate(errList []error) error {
	var errs []error
	for _, e := range errList {
		if e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return aggregate(errs)
}

// Error is part of the error interface.
func (agg aggregate) Error() string {
	msgs := make([]string, len(agg))
	for i := range agg {
		msgs[i] = agg[i].Error()
	}
	return strings.Join(msgs, "\n")
}

// Errors is part of the Aggregate interface.
func (agg aggregate) Errors() []error {
	return []error(agg)
}

// Is is part of the Aggregate interface
func (agg aggregate) Is(target error) bool {
	return agg.visit(func(err error) bool {
		return errors.Is(err, target)
	})
}

func (agg aggregate) visit(f func(err error) bool) bool {
	for _, err := range agg {
		switch err := err.(type) {
		case aggregate:
			if match := err.visit(f); match {
				return match
			}
		case utilerrors.Aggregate:
			for _, nestedErr := range err.Errors() {
				if match := f(nestedErr); match {
					return match
				}
			}
		default:
			if match := f(err); match {
				return match
			}
		}
	}

	return false
}

// MapToEnvVars converts a string-string map to a slice of corev1.EnvVar-s
func MapToEnvVars(mapEnvVars map[string]string) []corev1.EnvVar {
	if mapEnvVars == nil {
		return nil
	}

	envVars := make([]corev1.EnvVar, len(mapEnvVars))
	i := 0
	for k, v := range mapEnvVars {
		envVars[i] = corev1.EnvVar{Name: k, Value: v}
		i++
	}

	// need to sort the slice so that kube-controller-manager-pod configmap does not change all the time
	sort.Slice(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })
	return envVars
}

// InjectObservedProxyIntoContainers injects proxy environment variables in containers specified in containerNames.
func InjectObservedProxyIntoContainers(podSpec *corev1.PodSpec, containerNames []string, observedConfig []byte, fields ...string) error {
	var config map[string]interface{}
	if err := yaml.Unmarshal(observedConfig, &config); err != nil {
		return fmt.Errorf("failed to unmarshal the observedConfig: %w", err)
	}

	proxyConfig, found, err := unstructured.NestedStringMap(config, fields...)
	if err != nil {
		return fmt.Errorf("couldn't get the proxy config from observedConfig: %w", err)
	}

	proxyEnvVars := MapToEnvVars(proxyConfig)
	if !found || len(proxyEnvVars) < 1 {
		// There's no observed proxy config, we should tolerate that
		return nil
	}

	for _, containerName := range containerNames {
		for i := range podSpec.InitContainers {
			if podSpec.InitContainers[i].Name == containerName {
				podSpec.InitContainers[i].Env = append(podSpec.InitContainers[i].Env, proxyEnvVars...)
			}
		}
		for i := range podSpec.Containers {
			if podSpec.Containers[i].Name == containerName {
				podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, proxyEnvVars...)
			}
		}
	}

	return nil
}

func InjectTrustedCAIntoContainers(podSpec *corev1.PodSpec, configMapName string, containerNames []string) error {
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "non-standard-root-system-trust-ca-bundle",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
				Items: []corev1.KeyToPath{
					{Key: "ca-bundle.crt", Path: "tls-ca-bundle.pem"},
				},
			},
		},
	})

	for _, containerName := range containerNames {
		for i := range podSpec.InitContainers {
			if podSpec.InitContainers[i].Name == containerName {
				podSpec.InitContainers[i].VolumeMounts = append(podSpec.InitContainers[i].VolumeMounts, corev1.VolumeMount{
					Name:      "non-standard-root-system-trust-ca-bundle",
					MountPath: "/etc/pki/ca-trust/extracted/pem",
					ReadOnly:  true,
				})
			}
		}
		for i := range podSpec.Containers {
			if podSpec.Containers[i].Name == containerName {
				podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, corev1.VolumeMount{
					Name:      "non-standard-root-system-trust-ca-bundle",
					MountPath: "/etc/pki/ca-trust/extracted/pem",
					ReadOnly:  true,
				})
			}
		}
	}

	return nil
}

func SetCondition(conditions *[]metav1.Condition, newCondition metav1.Condition) {
	if conditions == nil {
		conditions = &[]metav1.Condition{}
	}
	existingCondition := FindCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		*conditions = append(*conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

func RemoveCondition(conditions *[]metav1.Condition, conditionType string) {
	if conditions == nil {
		conditions = &[]metav1.Condition{}
	}
	newConditions := []metav1.Condition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	*conditions = newConditions
}

func FindCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	return IsConditionPresentAndEqual(conditions, conditionType, metav1.ConditionTrue)
}

func IsConditionFalse(conditions []metav1.Condition, conditionType string) bool {
	return IsConditionPresentAndEqual(conditions, conditionType, metav1.ConditionFalse)
}

func IsConditionPresentAndEqual(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}
