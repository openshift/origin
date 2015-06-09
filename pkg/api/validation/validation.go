package validation

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/api/latest"
)

type RuntimeObjectValidator interface {
	Validate(obj runtime.Object) fielderrors.ValidationErrorList
	ValidateUpdate(obj, old runtime.Object) fielderrors.ValidationErrorList
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

func (v *RuntimeObjectsValidator) Validate(obj runtime.Object) fielderrors.ValidationErrorList {
	if obj == nil {
		return fielderrors.ValidationErrorList{}
	}

	allErrs := fielderrors.ValidationErrorList{}

	specificValidationInfo, err := v.getSpecificValidationInfo(obj)
	if err != nil {
		allErrs = append(allErrs, err)
		return allErrs
	}

	allErrs = append(allErrs, specificValidationInfo.Validator.Validate(obj)...)
	return allErrs
}

func (v *RuntimeObjectsValidator) ValidateUpdate(obj, old runtime.Object) fielderrors.ValidationErrorList {
	if obj == nil && old == nil {
		return fielderrors.ValidationErrorList{}
	}
	if newType, oldType := reflect.TypeOf(obj), reflect.TypeOf(old); newType != oldType {
		return fielderrors.ValidationErrorList{validation.NewInvalidTypeError(oldType.Kind(), newType.Kind(), "runtime.Object")}
	}

	allErrs := fielderrors.ValidationErrorList{}

	specificValidationInfo, err := v.getSpecificValidationInfo(obj)
	if err != nil {
		allErrs = append(allErrs, err)
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
	version, kind, err := kapi.Scheme.ObjectVersionAndKind(obj)
	if err != nil {
		return false, err
	}

	restMapping, err := latest.RESTMapper.RESTMapping(kind, version)
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
