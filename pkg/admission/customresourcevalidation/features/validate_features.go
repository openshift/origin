package features

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation"
)

const PluginName = "config.openshift.io/ValidateFeatures"

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return customresourcevalidation.NewValidator(
			map[schema.GroupResource]bool{
				configv1.Resource("features"): true,
			},
			map[schema.GroupVersionKind]customresourcevalidation.ObjectValidator{
				configv1.GroupVersion.WithKind("Feature"): featuresV1{},
			})
	})
}

func toFeatureV1(uncastObj runtime.Object) (*configv1.Features, field.ErrorList) {
	if uncastObj == nil {
		return nil, nil
	}

	allErrs := field.ErrorList{}

	obj, ok := uncastObj.(*configv1.Features)
	if !ok {
		return nil, append(allErrs,
			field.NotSupported(field.NewPath("kind"), fmt.Sprintf("%T", uncastObj), []string{"Features"}),
			field.NotSupported(field.NewPath("apiVersion"), fmt.Sprintf("%T", uncastObj), []string{"config.openshift.io/v1"}))
	}

	return obj, nil
}

type featuresV1 struct {
}

var knownFeatureSets = sets.NewString("", string(configv1.TechPreviewNoUpgrade))

func validateFeatureSpecCreate(spec configv1.FeaturesSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	// on create, we only allow values that we are aware of
	if !knownFeatureSets.Has(string(spec.FeatureSet)) {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec.featureSet"), spec.FeatureSet, knownFeatureSets.List()))
	}

	return allErrs
}

func validateFeatureSpecUpdate(spec, oldSpec configv1.FeaturesSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	// on update, we don't fail validation on a field we don't recognize as long as it is not changing
	if !knownFeatureSets.Has(string(spec.FeatureSet)) && oldSpec.FeatureSet != spec.FeatureSet {
		allErrs = append(allErrs, field.NotSupported(field.NewPath("spec.featureSet"), spec.FeatureSet, knownFeatureSets.List()))
	}

	// we do not allow anyone to take back TechPreview
	if oldSpec.FeatureSet == configv1.TechPreviewNoUpgrade && spec.FeatureSet != configv1.TechPreviewNoUpgrade {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec.featureSet"), "once enabled, tech preview features may not be disabled"))
	}

	return allErrs
}

func (featuresV1) ValidateCreate(uncastObj runtime.Object) field.ErrorList {
	obj, allErrs := toFeatureV1(uncastObj)
	if len(allErrs) > 0 {
		return allErrs
	}

	allErrs = append(allErrs, validation.ValidateObjectMeta(&obj.ObjectMeta, false, customresourcevalidation.RequireNameCluster, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateFeatureSpecCreate(obj.Spec)...)

	return allErrs
}

func (featuresV1) ValidateUpdate(uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, allErrs := toFeatureV1(uncastObj)
	if len(allErrs) > 0 {
		return allErrs
	}
	oldObj, allErrs := toFeatureV1(uncastOldObj)
	if len(allErrs) > 0 {
		return allErrs
	}

	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateFeatureSpecUpdate(obj.Spec, oldObj.Spec)...)

	return allErrs
}

func (featuresV1) ValidateStatusUpdate(uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, errs := toFeatureV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}
	oldObj, errs := toFeatureV1(uncastOldObj)
	if len(errs) > 0 {
		return errs
	}

	// TODO validate the obj.  remember that status validation should *never* fail on spec validation errors.
	errs = append(errs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)

	return errs
}
