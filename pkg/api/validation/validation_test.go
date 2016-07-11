package validation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/runtime"
	ktypes "k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// Ensures that `nil` can be passed to validation functions validating top-level objects
func TestNilPath(t *testing.T) {
	var nilPath *field.Path = nil
	if s := nilPath.String(); s != "" {
		t.Errorf("Unexpected nil path: %q", s)
	}

	child := nilPath.Child("child")
	if s := child.String(); s != "child" {
		t.Errorf("Unexpected child path: %q", s)
	}

	key := nilPath.Key("key")
	if s := key.String(); s != "[key]" {
		t.Errorf("Unexpected key path: %q", s)
	}

	index := nilPath.Index(1)
	if s := index.String(); s != "[1]" {
		t.Errorf("Unexpected index path: %q", s)
	}
}

func TestNameFunc(t *testing.T) {
	const nameRulesMessage = `must match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)* (e.g. 'example.com')`

	for apiType, validationInfo := range Validator.typeToValidator {
		if !validationInfo.HasObjectMeta {
			continue
		}

		apiValue := reflect.New(apiType.Elem())
		apiObjectMeta := apiValue.Elem().FieldByName("ObjectMeta")

		// check for illegal names
		for _, illegalName := range api.NameMayNotBe {
			apiObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: illegalName}))

			errList := validationInfo.Validator.Validate(apiValue.Interface().(runtime.Object))
			reasons := api.MinimalNameRequirements(illegalName, false)
			requiredMessage := strings.Join(reasons, ", ")

			if len(errList) == 0 {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add api.MinimalNameRequirements to your name validator..", illegalName, apiType.Elem(), errList)
				continue
			}

			foundExpectedError := false
			for _, err := range errList {
				validationError := err
				if validationError.Type != field.ErrorTypeInvalid || validationError.Field != "metadata.name" {
					continue
				}
				if validationError.Detail == requiredMessage {
					foundExpectedError = true
					break
				}
				// this message is from a stock name validation method in kube that covers our requirements in MinimalNameRequirements
				if validationError.Detail == nameRulesMessage {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add api.MinimalNameRequirements to your name validator.", illegalName, apiType.Elem(), errList)
			}
		}

		// check for illegal contents
		for _, illegalContent := range api.NameMayNotContain {
			illegalName := "a" + illegalContent + "b"

			apiObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: illegalName}))

			errList := validationInfo.Validator.Validate(apiValue.Interface().(runtime.Object))
			reasons := api.MinimalNameRequirements(illegalName, false)
			requiredMessage := strings.Join(reasons, ", ")

			if len(errList) == 0 {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add api.MinimalNameRequirements to your name validator.", illegalName, apiType.Elem(), errList)
				continue
			}

			foundExpectedError := false
			for _, err := range errList {
				validationError := err
				if validationError.Type != field.ErrorTypeInvalid || validationError.Field != "metadata.name" {
					continue
				}

				if validationError.Detail == requiredMessage {
					foundExpectedError = true
					break
				}
				// this message is from a stock name validation method in kube that covers our requirements in MinimalNameRequirements
				if validationError.Detail == nameRulesMessage {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add api.MinimalNameRequirements to your name validator.", illegalName, apiType.Elem(), errList)
			}
		}
	}
}

func TestObjectMeta(t *testing.T) {
	for apiType, validationInfo := range Validator.typeToValidator {
		if !validationInfo.HasObjectMeta {
			continue
		}

		apiValue := reflect.New(apiType.Elem())
		apiObjectMeta := apiValue.Elem().FieldByName("ObjectMeta")

		if validationInfo.IsNamespaced {
			apiObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: getValidName(apiType)}))
		} else {
			apiObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: getValidName(apiType), Namespace: kapi.NamespaceDefault}))
		}

		errList := validationInfo.Validator.Validate(apiValue.Interface().(runtime.Object))
		requiredErrors := validation.ValidateObjectMeta(apiObjectMeta.Addr().Interface().(*kapi.ObjectMeta), validationInfo.IsNamespaced, api.MinimalNameRequirements, field.NewPath("metadata"))

		if len(errList) == 0 {
			t.Errorf("expected errors %v in %v not found amongst %v.  You probably need to call kube/validation.ValidateObjectMeta in your validator.", requiredErrors, apiType.Elem(), errList)
			continue
		}

		for _, requiredError := range requiredErrors {
			foundExpectedError := false

			for _, err := range errList {
				validationError := err
				if fmt.Sprintf("%v", validationError) == fmt.Sprintf("%v", requiredError) {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf("expected error %v in %v not found amongst %v.  You probably need to call kube/validation.ValidateObjectMeta in your validator.", requiredError, apiType.Elem(), errList)
			}
		}
	}
}

func getValidName(apiType reflect.Type) string {
	apiValue := reflect.New(apiType.Elem())
	obj := apiValue.Interface().(runtime.Object)

	switch obj.(type) {
	case *authorizationapi.ClusterPolicyBinding, *authorizationapi.PolicyBinding:
		return ":default"
	case *authorizationapi.ClusterPolicy, *authorizationapi.Policy:
		return "default"
	default:
		return "any-string"
	}

}

// TestObjectMetaUpdate checks for:
// 1. missing ResourceVersion
// 2. mismatched Name
// 3. mismatched Namespace
// 4. mismatched UID
func TestObjectMetaUpdate(t *testing.T) {
	for apiType, validationInfo := range Validator.typeToValidator {
		if !validationInfo.HasObjectMeta {
			continue
		}
		if !validationInfo.UpdateAllowed {
			continue
		}

		oldAPIValue := reflect.New(apiType.Elem())
		oldAPIObjectMeta := oldAPIValue.Elem().FieldByName("ObjectMeta")
		oldAPIObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: "first-name", Namespace: "first-namespace", UID: ktypes.UID("first-uid")}))
		oldObj := oldAPIValue.Interface().(runtime.Object)
		oldObjMeta := oldAPIObjectMeta.Addr().Interface().(*kapi.ObjectMeta)

		newAPIValue := reflect.New(apiType.Elem())
		newAPIObjectMeta := newAPIValue.Elem().FieldByName("ObjectMeta")
		newAPIObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: "second-name", Namespace: "second-namespace", UID: ktypes.UID("second-uid")}))
		newObj := newAPIValue.Interface().(runtime.Object)
		newObjMeta := newAPIObjectMeta.Addr().Interface().(*kapi.ObjectMeta)

		errList := validationInfo.Validator.ValidateUpdate(newObj, oldObj)
		requiredErrors := validation.ValidateObjectMetaUpdate(newObjMeta, oldObjMeta, field.NewPath("metadata"))

		if len(errList) == 0 {
			t.Errorf("expected errors %v in %v not found amongst %v.  You probably need to call kube/validation.ValidateObjectMetaUpdate in your validator.", requiredErrors, apiType.Elem(), errList)
			continue
		}

		for _, requiredError := range requiredErrors {
			foundExpectedError := false

			for _, err := range errList {
				validationError := err
				if fmt.Sprintf("%v", validationError) == fmt.Sprintf("%v", requiredError) {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf("expected error %v in %v not found amongst %v.  You probably need to call kube/validation.ValidateObjectMetaUpdate in your validator.", requiredError, apiType.Elem(), errList)
			}
		}
	}
}

func TestPodSpecNodeSelectorUpdateDisallowed(t *testing.T) {
	oldPod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			ResourceVersion: "1",
		},
		Spec: kapi.PodSpec{
			NodeSelector: map[string]string{
				"foo": "bar",
			},
		},
	}

	if errs := validation.ValidatePodUpdate(oldPod, oldPod); len(errs) != 0 {
		t.Fatal("expected no errors")
	}

	newPod := *oldPod
	// use a new map so it doesn't change oldPod's map too
	newPod.Spec.NodeSelector = map[string]string{"foo": "other"}

	errs := validation.ValidatePodUpdate(&newPod, oldPod)
	if len(errs) == 0 {
		t.Fatal("expected at least 1 error")
	}
}
