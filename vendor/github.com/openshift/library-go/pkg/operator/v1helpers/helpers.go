package v1helpers

import (
	"fmt"
	"strings"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
)

func SetOperandVersion(versions *[]configv1.OperandVersion, operandVersion configv1.OperandVersion) {
	if versions == nil {
		versions = &[]configv1.OperandVersion{}
	}
	existingVersion := FindOperandVersion(*versions, operandVersion.Name)
	if existingVersion == nil {
		*versions = append(*versions, operandVersion)
		return
	}
	existingVersion.Version = operandVersion.Version
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

func EnsureOperatorConfigExists(client dynamic.Interface, operatorConfigBytes []byte, gvr schema.GroupVersionResource) {
	configJson, err := yaml.YAMLToJSON(operatorConfigBytes)
	if err != nil {
		panic(err)
	}
	operatorConfigObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, configJson)
	if err != nil {
		panic(err)
	}

	requiredOperatorConfig, ok := operatorConfigObj.(*unstructured.Unstructured)
	if !ok {
		panic(fmt.Sprintf("unexpected object in %t", operatorConfigObj))
	}

	_, err = client.Resource(gvr).Get(requiredOperatorConfig.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		if _, err := client.Resource(gvr).Create(requiredOperatorConfig, metav1.CreateOptions{}); err != nil {
			panic(err)
		}
		return
	}
	if err != nil {
		panic(err)
	}
}

// UpdateOperatorSpecFunc is a func that mutates an operator spec.
type UpdateOperatorSpecFunc func(spec *operatorv1.OperatorSpec) error

// UpdateSpec applies the update funcs to the oldStatus and tries to update via the client.
func UpdateSpec(client OperatorClient, updateFuncs ...UpdateOperatorSpecFunc) (*operatorv1.OperatorSpec, bool, error) {
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

		operatorSpec, _, err = client.UpdateOperatorSpec(resourceVersion, newSpec)
		updated = err == nil
		return err
	})

	return operatorSpec, updated, err
}

// UpdateSpecConfigFn returns a func to update the config.
func UpdateObservedConfigFn(config map[string]interface{}) UpdateOperatorSpecFunc {
	return func(oldSpec *operatorv1.OperatorSpec) error {
		oldSpec.ObservedConfig = runtime.RawExtension{Object: &unstructured.Unstructured{Object: config}}
		return nil
	}
}

// UpdateStatusFunc is a func that mutates an operator status.
type UpdateStatusFunc func(status *operatorv1.OperatorStatus) error

// UpdateStatus applies the update funcs to the oldStatus and tries to update via the client.
func UpdateStatus(client OperatorClient, updateFuncs ...UpdateStatusFunc) (*operatorv1.OperatorStatus, bool, error) {
	updated := false
	var updatedOperatorStatus *operatorv1.OperatorStatus
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, oldStatus, resourceVersion, err := client.GetOperatorState()
		if err != nil {
			return err
		}

		newStatus := oldStatus.DeepCopy()
		for _, update := range updateFuncs {
			if err := update(newStatus); err != nil {
				return err
			}
		}

		if equality.Semantic.DeepEqual(oldStatus, newStatus) {
			return nil
		}

		updatedOperatorStatus, err = client.UpdateOperatorStatus(resourceVersion, newStatus)
		updated = err == nil
		return err
	})

	return updatedOperatorStatus, updated, err
}

// UpdateConditionFunc returns a func to update a condition.
func UpdateConditionFn(cond operatorv1.OperatorCondition) UpdateStatusFunc {
	return func(oldStatus *operatorv1.OperatorStatus) error {
		SetOperatorCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

// UpdateStatusFunc is a func that mutates an operator status.
type UpdateStaticPodStatusFunc func(status *operatorv1.StaticPodOperatorStatus) error

// UpdateStaticPodStatus applies the update funcs to the oldStatus abd tries to update via the client.
func UpdateStaticPodStatus(client StaticPodOperatorClient, updateFuncs ...UpdateStaticPodStatusFunc) (*operatorv1.StaticPodOperatorStatus, bool, error) {
	updated := false
	var updatedOperatorStatus *operatorv1.StaticPodOperatorStatus
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, oldStatus, resourceVersion, err := client.GetStaticPodOperatorState()
		if err != nil {
			return err
		}

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

		updatedOperatorStatus, err = client.UpdateStaticPodOperatorStatus(resourceVersion, newStatus)
		updated = err == nil
		return err
	})

	return updatedOperatorStatus, updated, err
}

// UpdateStaticPodConditionFn returns a func to update a condition.
func UpdateStaticPodConditionFn(cond operatorv1.OperatorCondition) UpdateStaticPodStatusFunc {
	return func(oldStatus *operatorv1.StaticPodOperatorStatus) error {
		SetOperatorCondition(&oldStatus.Conditions, cond)
		return nil
	}
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
