package validation

import (
	"fmt"
	"reflect"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
)

type WrappingValidator struct {
	validate       *reflect.Value
	validateUpdate *reflect.Value
}

func (v *WrappingValidator) Validate(obj runtime.Object) fielderrors.ValidationErrorList {
	return callValidate(reflect.ValueOf(obj), *v.validate)
}

func (v *WrappingValidator) ValidateUpdate(obj, old runtime.Object) fielderrors.ValidationErrorList {
	if v.validateUpdate == nil {
		// not all types care about updates.  If there's no specific validation for updates, call to normal validate
		return v.Validate(obj)
	}

	return callValidateUpdate(reflect.ValueOf(obj), reflect.ValueOf(old), *v.validateUpdate)
}

func NewValidationWrapper(validateFunction interface{}, validateUpdateFunction interface{}) (*WrappingValidator, error) {
	validateFunctionValue := reflect.ValueOf(validateFunction)
	validateType := validateFunctionValue.Type()
	if err := verifyValidateFunctionSignature(validateType); err != nil {
		return nil, err
	}

	var validateUpdateFunctionValue *reflect.Value
	if validateUpdateFunction != nil {
		functionValue := reflect.ValueOf(validateUpdateFunction)
		validateUpdateType := functionValue.Type()
		if err := verifyValidateUpdateFunctionSignature(validateUpdateType); err != nil {
			return nil, err
		}

		validateUpdateFunctionValue = &functionValue
	}

	return &WrappingValidator{&validateFunctionValue, validateUpdateFunctionValue}, nil
}

func verifyValidateFunctionSignature(ft reflect.Type) error {
	if ft.Kind() != reflect.Func {
		return fmt.Errorf("expected func, got: %v", ft)
	}
	if ft.NumIn() != 1 {
		return fmt.Errorf("expected one 'in' param, got: %v", ft)
	}
	if ft.NumOut() != 1 {
		return fmt.Errorf("expected one 'out' param, got: %v", ft)
	}
	if ft.In(0).Kind() != reflect.Ptr {
		return fmt.Errorf("expected pointer arg for 'in' param 0, got: %v", ft)
	}
	errorType := reflect.TypeOf(&fielderrors.ValidationErrorList{}).Elem()
	if ft.Out(0) != errorType {
		return fmt.Errorf("expected fielderrors.ValidationErrorList return, got: %v", ft)
	}
	return nil
}

func verifyValidateUpdateFunctionSignature(ft reflect.Type) error {
	if ft.Kind() != reflect.Func {
		return fmt.Errorf("expected func, got: %v", ft)
	}
	if ft.NumIn() != 2 {
		return fmt.Errorf("expected two 'in' params, got: %v", ft)
	}
	if ft.NumOut() != 1 {
		return fmt.Errorf("expected one 'out' param, got: %v", ft)
	}
	if ft.In(0).Kind() != reflect.Ptr {
		return fmt.Errorf("expected pointer arg for 'in' param 0, got: %v", ft)
	}
	if ft.In(1).Kind() != reflect.Ptr {
		return fmt.Errorf("expected pointer arg for 'in' param 1, got: %v", ft)
	}
	errorType := reflect.TypeOf(&fielderrors.ValidationErrorList{}).Elem()
	if ft.Out(0) != errorType {
		return fmt.Errorf("expected fielderrors.ValidationErrorList return, got: %v", ft)
	}
	return nil
}

// callCustom calls 'custom' with sv & dv. custom must be a conversion function.
func callValidate(obj, validateMethod reflect.Value) fielderrors.ValidationErrorList {
	args := []reflect.Value{obj}
	ret := validateMethod.Call(args)[0].Interface()

	// This convolution is necessary because nil interfaces won't convert
	// to errors.
	if ret == nil {
		return nil
	}
	return ret.(fielderrors.ValidationErrorList)
}

func callValidateUpdate(obj, old, validateMethod reflect.Value) fielderrors.ValidationErrorList {
	args := []reflect.Value{obj, old}
	ret := validateMethod.Call(args)[0].Interface()

	// This convolution is necessary because nil interfaces won't convert
	// to errors.
	if ret == nil {
		return nil
	}
	return ret.(fielderrors.ValidationErrorList)
}
