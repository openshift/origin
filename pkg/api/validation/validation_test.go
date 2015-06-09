package validation

import (
	"fmt"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func TestNameFunc(t *testing.T) {
	for apiType, validationInfo := range Validator.typeToValidator {
		if !validationInfo.hasObjectMeta {
			continue
		}

		apiValue := reflect.New(apiType.Elem())
		apiObjectMeta := apiValue.Elem().FieldByName("ObjectMeta")

		// check for illegal names
		for _, illegalName := range api.NameMayNotBe {
			apiObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: illegalName}))

			errList := validationInfo.validator.Validate(apiValue.Interface().(runtime.Object))
			_, requiredMessage := api.MinimalNameRequirements(illegalName, false)

			if len(errList) == 0 {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add api.MinimalNameRequirements to your name validator..", illegalName, apiType.Elem(), errList)
				continue
			}

			foundExpectedError := false
			for _, err := range errList {
				validationError, ok := err.(*fielderrors.ValidationError)
				if !ok || validationError.Type != fielderrors.ValidationErrorTypeInvalid || validationError.Field != "metadata.name" {
					continue
				}

				if validationError.Detail == requiredMessage {
					foundExpectedError = true
					break
				}
				// this message is from a stock name validation method in kube that covers our requirements in MinimalNameRequirements
				if validationError.Detail == validation.DNSSubdomainErrorMsg {
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

			errList := validationInfo.validator.Validate(apiValue.Interface().(runtime.Object))
			_, requiredMessage := api.MinimalNameRequirements(illegalName, false)

			if len(errList) == 0 {
				t.Errorf("expected error for %v in %v not found amongst %v.  You probably need to add api.MinimalNameRequirements to your name validator.", illegalName, apiType.Elem(), errList)
				continue
			}

			foundExpectedError := false
			for _, err := range errList {
				validationError, ok := err.(*fielderrors.ValidationError)
				if !ok || validationError.Type != fielderrors.ValidationErrorTypeInvalid || validationError.Field != "metadata.name" {
					continue
				}

				if validationError.Detail == requiredMessage {
					foundExpectedError = true
					break
				}
				if validationError.Detail == validation.DNSSubdomainErrorMsg {
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
		if !validationInfo.hasObjectMeta {
			continue
		}

		apiValue := reflect.New(apiType.Elem())
		apiObjectMeta := apiValue.Elem().FieldByName("ObjectMeta")

		if validationInfo.isNamespaced {
			apiObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: getValidName(apiType)}))
		} else {
			apiObjectMeta.Set(reflect.ValueOf(kapi.ObjectMeta{Name: getValidName(apiType), Namespace: kapi.NamespaceDefault}))
		}

		// these have special case conversions that always strip
		switch apiValue.Interface().(type) {
		case *authorizationapi.ClusterPolicyBinding, *authorizationapi.PolicyBinding, *authorizationapi.ClusterPolicy, *authorizationapi.Policy:
			continue
		}

		errList := validationInfo.validator.Validate(apiValue.Interface().(runtime.Object))
		requiredErrors := validation.ValidateObjectMeta(apiObjectMeta.Addr().Interface().(*kapi.ObjectMeta), validationInfo.isNamespaced, api.MinimalNameRequirements).Prefix("metadata")

		if len(errList) == 0 {
			t.Errorf("expected errors %v in %v not found amongst %v.  You probably need to call kube/validation.ValidateObjectMeta in your validator.", requiredErrors, apiType.Elem(), errList)
			continue
		}

		for _, requiredError := range requiredErrors {
			foundExpectedError := false

			for _, err := range errList {
				validationError, ok := err.(*fielderrors.ValidationError)
				if !ok {
					continue
				}

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
