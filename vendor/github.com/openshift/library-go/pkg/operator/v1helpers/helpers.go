package v1helpers

import (
	"fmt"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	operatorsv1 "github.com/openshift/api/operator/v1"
)

func SetOperatorCondition(conditions *[]operatorsv1.OperatorCondition, newCondition operatorsv1.OperatorCondition) {
	if conditions == nil {
		conditions = &[]operatorsv1.OperatorCondition{}
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

func RemoveOperatorCondition(conditions *[]operatorsv1.OperatorCondition, conditionType string) {
	if conditions == nil {
		conditions = &[]operatorsv1.OperatorCondition{}
	}
	newConditions := []operatorsv1.OperatorCondition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	*conditions = newConditions
}

func FindOperatorCondition(conditions []operatorsv1.OperatorCondition, conditionType string) *operatorsv1.OperatorCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsOperatorConditionTrue(conditions []operatorsv1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1.ConditionTrue)
}

func IsOperatorConditionFalse(conditions []operatorsv1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1.ConditionFalse)
}

func IsOperatorConditionPresentAndEqual(conditions []operatorsv1.OperatorCondition, conditionType string, status operatorsv1.ConditionStatus) bool {
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
