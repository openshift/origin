package validation

import (
	"fmt"
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

type RuntimeObjectValidator interface {
	Validate(obj runtime.Object) field.ErrorList
	ValidateUpdate(obj, old runtime.Object) field.ErrorList
}

var Validator = &RuntimeObjectsValidator{map[reflect.Type]RuntimeObjectValidatorInfo{}}

type RuntimeObjectsValidator struct {
	typeToValidator map[reflect.Type]RuntimeObjectValidatorInfo
}

type RuntimeObjectValidatorInfo struct {
	Validator     RuntimeObjectValidator
	IsNamespaced  bool
	HasObjectMeta bool
	UpdateAllowed bool
}

func (v *RuntimeObjectsValidator) GetInfo(obj runtime.Object) (RuntimeObjectValidatorInfo, bool) {
	ret, ok := v.typeToValidator[reflect.TypeOf(obj)]
	return ret, ok
}

func (v *RuntimeObjectsValidator) MustRegister(obj runtime.Object, validateFunction interface{}, validateUpdateFunction interface{}) {
	if err := v.Register(obj, validateFunction, validateUpdateFunction); err != nil {
		panic(err)
	}
}

func (v *RuntimeObjectsValidator) Register(obj runtime.Object, validateFunction interface{}, validateUpdateFunction interface{}) error {
	objType := reflect.TypeOf(obj)
	if oldValidator, exists := v.typeToValidator[objType]; exists {
		panic(fmt.Sprintf("%v is already registered with %v", objType, oldValidator))
	}

	validator, err := NewValidationWrapper(validateFunction, validateUpdateFunction)
	if err != nil {
		return err
	}

	isNamespaced, err := GetRequiresNamespace(obj)
	if err != nil {
		return err
	}

	updateAllowed := validateUpdateFunction != nil

	v.typeToValidator[objType] = RuntimeObjectValidatorInfo{validator, isNamespaced, HasObjectMeta(obj), updateAllowed}

	return nil
}

func (v *RuntimeObjectsValidator) Validate(obj runtime.Object) field.ErrorList {
	if obj == nil {
		return field.ErrorList{}
	}

	allErrs := field.ErrorList{}

	specificValidationInfo, err := v.getSpecificValidationInfo(obj)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(nil, err))
		return allErrs
	}

	allErrs = append(allErrs, specificValidationInfo.Validator.Validate(obj)...)
	return allErrs
}

func (v *RuntimeObjectsValidator) ValidateUpdate(obj, old runtime.Object) field.ErrorList {
	if obj == nil && old == nil {
		return field.ErrorList{}
	}
	if newType, oldType := reflect.TypeOf(obj), reflect.TypeOf(old); newType != oldType {
		return field.ErrorList{field.Invalid(field.NewPath("kind"), newType.Kind(), validation.NewInvalidTypeError(oldType.Kind(), newType.Kind(), "runtime.Object").Error())}
	}

	allErrs := field.ErrorList{}

	specificValidationInfo, err := v.getSpecificValidationInfo(obj)
	if err != nil {
		if fieldErr, ok := err.(*field.Error); ok {
			allErrs = append(allErrs, fieldErr)
		} else {
			allErrs = append(allErrs, field.InternalError(nil, err))
		}
		return allErrs
	}

	allErrs = append(allErrs, specificValidationInfo.Validator.ValidateUpdate(obj, old)...)

	// no errors so far, make sure that the new object is actually valid against the original validator
	if len(allErrs) == 0 {
		allErrs = append(allErrs, specificValidationInfo.Validator.Validate(obj)...)
	}

	return allErrs
}

func (v *RuntimeObjectsValidator) getSpecificValidationInfo(obj runtime.Object) (RuntimeObjectValidatorInfo, error) {
	objType := reflect.TypeOf(obj)
	specificValidationInfo, exists := v.typeToValidator[objType]

	if !exists {
		return RuntimeObjectValidatorInfo{}, fmt.Errorf("no validator registered for %v", objType)
	}

	return specificValidationInfo, nil
}

func GetRequiresNamespace(obj runtime.Object) (bool, error) {
	groupVersionKinds, _, err := kapi.Scheme.ObjectKinds(obj)
	if err != nil {
		return false, err
	}

	restMapping, err := kapi.RESTMapper.RESTMapping(groupVersionKinds[0].GroupKind())
	if err != nil {
		return false, err
	}

	return restMapping.Scope.Name() == meta.RESTScopeNameNamespace, nil
}

func HasObjectMeta(obj runtime.Object) bool {
	objValue := reflect.ValueOf(obj).Elem()
	field := objValue.FieldByName("ObjectMeta")

	if !field.IsValid() {
		return false
	}

	return true
}
