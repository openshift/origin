package network

import (
	"fmt"
	"io"
	"net"

	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/customresourcevalidation"
)

const PluginName = "config.openshift.io/ValidateNetwork"

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return customresourcevalidation.NewValidator(
			map[schema.GroupResource]bool{
				configv1.Resource("networks"): true,
			},
			map[schema.GroupVersionKind]customresourcevalidation.ObjectValidator{
				configv1.GroupVersion.WithKind("Network"): networkV1{},
			})
	})
}

func toNetworkV1(uncastObj runtime.Object) (*configv1.Network, field.ErrorList) {
	if uncastObj == nil {
		return nil, nil
	}

	allErrs := field.ErrorList{}

	obj, ok := uncastObj.(*configv1.Network)
	if !ok {
		return nil, append(allErrs,
			field.NotSupported(field.NewPath("kind"), fmt.Sprintf("%T", uncastObj), []string{"Network"}),
			field.NotSupported(field.NewPath("apiVersion"), fmt.Sprintf("%T", uncastObj), []string{"config.openshift.io/v1"}))
	}

	return obj, nil
}

type networkV1 struct {
}

// This only validates the syntax; the operator will worry about the semantics
func validateNetworkSpec(spec configv1.NetworkSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.ClusterNetwork) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("clusterNetwork"), ""))
	}
	for i, cnet := range spec.ClusterNetwork {
		_, cidr, err := net.ParseCIDR(cnet.CIDR)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("clusterNetwork").Index(i).Child("cidr"), cnet.CIDR, err.Error()))
		}
		_, bits := cidr.Mask.Size()
		if cnet.HostPrefix > uint32(bits) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("clusterNetwork").Index(i).Child("hostPrefix"), cnet.HostPrefix, "too large for address type"))
		}
	}

	if len(spec.ServiceNetwork) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("serviceNetwork"), ""))
	} else if len(spec.ServiceNetwork) > 1 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("serviceNetwork"), spec.ServiceNetwork, "multiple serviceNetwork values are not yet supported"))
	}
	for i, snet := range spec.ServiceNetwork {
		_, _, err := net.ParseCIDR(snet)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("serviceNetwork").Index(i), snet, err.Error()))
		}
	}

	if spec.NetworkType == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("networkType"), ""))
	}

	return allErrs
}

func (networkV1) ValidateCreate(uncastObj runtime.Object) field.ErrorList {
	obj, errs := toNetworkV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}

	errs = append(errs, validation.ValidateObjectMeta(&obj.ObjectMeta, false, customresourcevalidation.RequireNameCluster, field.NewPath("metadata"))...)
	errs = append(errs, validateNetworkSpec(obj.Spec)...)

	return errs
}

func (networkV1) ValidateUpdate(uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, errs := toNetworkV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}
	oldObj, errs := toNetworkV1(uncastOldObj)
	if len(errs) > 0 {
		return errs
	}

	errs = append(errs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)
	errs = append(errs, validateNetworkSpec(obj.Spec)...)

	return errs
}

func (networkV1) ValidateStatusUpdate(uncastObj runtime.Object, uncastOldObj runtime.Object) field.ErrorList {
	obj, errs := toNetworkV1(uncastObj)
	if len(errs) > 0 {
		return errs
	}
	oldObj, errs := toNetworkV1(uncastOldObj)
	if len(errs) > 0 {
		return errs
	}

	// TODO validate the obj.  remember that status validation should *never* fail on spec validation errors.
	errs = append(errs, validation.ValidateObjectMetaUpdate(&obj.ObjectMeta, &oldObj.ObjectMeta, field.NewPath("metadata"))...)

	return errs
}
