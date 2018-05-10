package validation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/validation/path"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/validation"
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
	const nameRulesMessage = `a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`

	for apiType, validationInfo := range Validator.typeToValidator {
		if !validationInfo.HasObjectMeta {
			continue
		}

		apiValue := reflect.New(apiType.Elem())
		apiObjectMeta := apiValue.Elem().FieldByName("ObjectMeta")

		// check for illegal names
		for _, illegalName := range []string{".", ".."} {
			apiObjectMeta.Set(reflect.ValueOf(metav1.ObjectMeta{Name: illegalName}))

			errList := validationInfo.Validator.Validate(apiValue.Interface().(runtime.Object))
			reasons := path.ValidatePathSegmentName(illegalName, false)
			requiredMessage := strings.Join(reasons, ", ")

			if len(errList) == 0 {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add path.ValidatePathSegmentName to your name validator..", illegalName, apiType.Elem(), errList)
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
				// this message is from a stock name validation method in kube that covers our requirements in ValidatePathSegmentName
				if validationError.Detail == nameRulesMessage {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add path.ValidatePathSegmentName to your name validator.", illegalName, apiType.Elem(), errList)
			}
		}

		// check for illegal contents
		for _, illegalContent := range []string{"/", "%"} {
			illegalName := "a" + illegalContent + "b"

			apiObjectMeta.Set(reflect.ValueOf(metav1.ObjectMeta{Name: illegalName}))

			errList := validationInfo.Validator.Validate(apiValue.Interface().(runtime.Object))
			reasons := path.ValidatePathSegmentName(illegalName, false)
			requiredMessage := strings.Join(reasons, ", ")

			if len(errList) == 0 {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add path.ValidatePathSegmentName to your name validator.", illegalName, apiType.Elem(), errList)
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
				// this message is from a stock name validation method in kube that covers our requirements in ValidatePathSegmentName
				if validationError.Detail == nameRulesMessage {
					foundExpectedError = true
					break
				}
			}

			if !foundExpectedError {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add path.ValidatePathSegmentName to your name validator.", illegalName, apiType.Elem(), errList)
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
			apiObjectMeta.Set(reflect.ValueOf(metav1.ObjectMeta{Name: getValidName(apiType)}))
		} else {
			apiObjectMeta.Set(reflect.ValueOf(metav1.ObjectMeta{Name: getValidName(apiType), Namespace: metav1.NamespaceDefault}))
		}

		errList := validationInfo.Validator.Validate(apiValue.Interface().(runtime.Object))
		requiredErrors := validation.ValidateObjectMeta(apiObjectMeta.Addr().Interface().(*metav1.ObjectMeta), validationInfo.IsNamespaced, path.ValidatePathSegmentName, field.NewPath("metadata"))

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
		oldAPIObjectMeta.Set(reflect.ValueOf(metav1.ObjectMeta{Name: "first-name", Namespace: "first-namespace", UID: ktypes.UID("first-uid")}))
		oldObj := oldAPIValue.Interface().(runtime.Object)
		oldObjMeta := oldAPIObjectMeta.Addr().Interface().(*metav1.ObjectMeta)

		newAPIValue := reflect.New(apiType.Elem())
		newAPIObjectMeta := newAPIValue.Elem().FieldByName("ObjectMeta")
		newAPIObjectMeta.Set(reflect.ValueOf(metav1.ObjectMeta{Name: "second-name", Namespace: "second-namespace", UID: ktypes.UID("second-uid")}))
		newObj := newAPIValue.Interface().(runtime.Object)
		newObjMeta := newAPIObjectMeta.Addr().Interface().(*metav1.ObjectMeta)

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
		ObjectMeta: metav1.ObjectMeta{
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
