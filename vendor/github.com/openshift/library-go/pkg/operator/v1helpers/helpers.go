package v1helpers

import (
	"fmt"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"

	operatorv1 "github.com/openshift/api/operator/v1"
)

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
		existingCondition.LastTransitionTime = newCondition.LastTransitionTime
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
		if _, err := client.Resource(gvr).Create(requiredOperatorConfig); err != nil {
			panic(err)
		}
		return
	}
	if err != nil {
		panic(err)
	}
}

// UpdateStatusFunc is a func that mutates an operator status.
type UpdateStatusFunc func(status *operatorv1.OperatorStatus) error

// UpdateStatus applies the update funcs to the oldStatus and tries to update via the client.
func UpdateStatus(client OperatorClient, updateFuncs ...UpdateStatusFunc) (bool, error) {
	updated := false
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

		_, _, err = client.UpdateOperatorStatus(resourceVersion, newStatus)
		updated = err == nil
		return err
	})

	return updated, err
}

// UpdateConditionFunc returns a func to update a condition.
func UpdateConditionFn(cond operatorv1.OperatorCondition) UpdateStatusFunc {
	return func(oldStatus *operatorv1.OperatorStatus) error {
		SetOperatorCondition(&oldStatus.Conditions, cond)
		return nil
	}
}
