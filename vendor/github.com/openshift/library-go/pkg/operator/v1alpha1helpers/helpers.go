package v1alpha1helpers

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func SetErrors(versionAvailability *operatorsv1alpha1.VersionAvailability, errors ...error) {
	versionAvailability.Errors = []string{}
	for _, err := range errors {
		versionAvailability.Errors = append(versionAvailability.Errors, err.Error())
	}
}

func SetOperatorCondition(conditions *[]operatorsv1alpha1.OperatorCondition, newCondition operatorsv1alpha1.OperatorCondition) {
	if conditions == nil {
		conditions = &[]operatorsv1alpha1.OperatorCondition{}
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

func RemoveOperatorCondition(conditions *[]operatorsv1alpha1.OperatorCondition, conditionType string) {
	if conditions == nil {
		conditions = &[]operatorsv1alpha1.OperatorCondition{}
	}
	newConditions := []operatorsv1alpha1.OperatorCondition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	*conditions = newConditions
}

func FindOperatorCondition(conditions []operatorsv1alpha1.OperatorCondition, conditionType string) *operatorsv1alpha1.OperatorCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsOperatorConditionTrue(conditions []operatorsv1alpha1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1alpha1.ConditionTrue)
}

func IsOperatorConditionFalse(conditions []operatorsv1alpha1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1alpha1.ConditionFalse)
}

func IsOperatorConditionPresentAndEqual(conditions []operatorsv1alpha1.OperatorCondition, conditionType string, status operatorsv1alpha1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

// TODO this may not be sustainable/practical
func SetStatusFromAvailability(status *operatorsv1alpha1.OperatorStatus, specGeneration int64, versionAvailability *operatorsv1alpha1.VersionAvailability) {
	// given the VersionAvailability and the status.Version, we can compute availability
	availableCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeAvailable,
		Status: operatorsv1alpha1.ConditionUnknown,
	}
	if versionAvailability != nil && versionAvailability.ReadyReplicas > 0 {
		availableCondition.Status = operatorsv1alpha1.ConditionTrue
		availableCondition.Message = "replicas ready"
	} else {
		availableCondition.Status = operatorsv1alpha1.ConditionFalse
		availableCondition.Message = "replicas not ready or unknown"
	}
	SetOperatorCondition(&status.Conditions, availableCondition)

	failureCondition := operatorsv1alpha1.OperatorCondition{
		Type:    operatorsv1alpha1.OperatorStatusTypeFailing,
		Status:  operatorsv1alpha1.ConditionFalse,
		Message: "no errors found",
	}
	if versionAvailability != nil && len(versionAvailability.Errors) > 0 {
		failureCondition.Status = operatorsv1alpha1.ConditionTrue
		failureCondition.Message = strings.Join(versionAvailability.Errors, "\n")
	}
	if status.TargetAvailability != nil && len(status.TargetAvailability.Errors) > 0 {
		failureCondition.Status = operatorsv1alpha1.ConditionTrue
		if len(failureCondition.Message) == 0 {
			failureCondition.Message = strings.Join(status.TargetAvailability.Errors, "\n")
		} else {
			failureCondition.Message = availableCondition.Message + "\n" + strings.Join(status.TargetAvailability.Errors, "\n")
		}
	}
	SetOperatorCondition(&status.Conditions, failureCondition)
	if failureCondition.Status == operatorsv1alpha1.ConditionFalse {
		status.ObservedGeneration = specGeneration
	}

	progressingCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeProgressing,
		Status: operatorsv1alpha1.ConditionUnknown,
	}
	if availableCondition.Status == operatorsv1alpha1.ConditionTrue {
		progressingCondition.Status = operatorsv1alpha1.ConditionFalse
		progressingCondition.Message = "available and not waiting for a change"
	} else if versionAvailability != nil && versionAvailability.ReadyReplicas == 0 {
		progressingCondition.Status = operatorsv1alpha1.ConditionTrue
		progressingCondition.Message = "not replicas available"
	} else {
		progressingCondition.Status = operatorsv1alpha1.ConditionTrue
		progressingCondition.Message = "not available"
	}
	SetOperatorCondition(&status.Conditions, progressingCondition)

	status.CurrentAvailability = versionAvailability
}

type GetImageEnvFunc func() string

func GetImageEnv() string {
	return os.Getenv("IMAGE")
}

func EnsureOperatorConfigExists(client dynamic.Interface, operatorConfigBytes []byte, gvr schema.GroupVersionResource, getEnv GetImageEnvFunc) {
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

	hasImageEnvVar := false
	if imagePullSpecFromEnv := getEnv(); len(imagePullSpecFromEnv) > 0 {
		hasImageEnvVar = true
		err = unstructured.SetNestedField(requiredOperatorConfig.Object, imagePullSpecFromEnv, "spec", "imagePullSpec")
		if err != nil {
			panic(err)
		}
	}

	existing, err := client.Resource(gvr).Get(requiredOperatorConfig.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		if _, err := client.Resource(gvr).Create(requiredOperatorConfig); err != nil {
			panic(err)
		}
		return
	}
	if err != nil {
		panic(err)
	}

	if !hasImageEnvVar {
		return
	}

	// If ImagePullSpec changed, update the existing config instance
	existingSpec, _, err := unstructured.NestedString(existing.Object, "spec", "imagePullSpec")
	if err != nil {
		panic(err)
	}
	requiredSpec, _, err := unstructured.NestedString(requiredOperatorConfig.Object, "spec", "imagePullSpec")
	if err != nil {
		panic(err)
	}
	if existingSpec != requiredSpec {
		err = unstructured.SetNestedField(existing.Object, requiredSpec, "spec", "imagePullSpec")
		if err != nil {
			panic(err)
		}
		if _, err := client.Resource(gvr).Update(existing); err != nil {
			panic(err)
		}
	}
}
