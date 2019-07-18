package limitrange

import (
	"fmt"
	"io"

	"github.com/openshift/openshift-apiserver/pkg/image/apiserver/internalimageutil"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/plugin/pkg/admission/limitranger"

	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/openshift-apiserver/pkg/api/legacy"
	imageapi "github.com/openshift/openshift-apiserver/pkg/image/apis/image"
)

const (
	PluginName = "image.openshift.io/ImageLimitRange"
)

func newLimitExceededError(limitType corev1.LimitType, resourceName corev1.ResourceName, requested, limit *resource.Quantity) error {
	return fmt.Errorf("requested usage of %s exceeds the maximum limit per %s (%s > %s)", resourceName, limitType, requested.String(), limit.String())
}

func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName,
		func(config io.Reader) (admission.Interface, error) {
			plugin, err := NewImageLimitRangerPlugin(config)
			if err != nil {
				return nil, err
			}
			return plugin, nil
		})
}

// imageLimitRangerPlugin is the admission plugin.
type imageLimitRangerPlugin struct {
	*admission.Handler
	limitRanger *limitranger.LimitRanger
}

// imageLimitRangerPlugin implements the LimitRangerActions interface.
var _ limitranger.LimitRangerActions = &imageLimitRangerPlugin{}
var _ initializer.WantsExternalKubeInformerFactory = &imageLimitRangerPlugin{}
var _ initializer.WantsExternalKubeClientSet = &imageLimitRangerPlugin{}
var _ admission.ValidationInterface = &imageLimitRangerPlugin{}
var _ admission.MutationInterface = &imageLimitRangerPlugin{}

// NewImageLimitRangerPlugin provides a new imageLimitRangerPlugin.
func NewImageLimitRangerPlugin(config io.Reader) (admission.Interface, error) {
	plugin := &imageLimitRangerPlugin{
		Handler: admission.NewHandler(admission.Create),
	}
	limitRanger, err := limitranger.NewLimitRanger(plugin)
	if err != nil {
		return nil, err
	}
	plugin.limitRanger = limitRanger
	return plugin, nil
}

func (a *imageLimitRangerPlugin) SetExternalKubeClientSet(c kubernetes.Interface) {
	a.limitRanger.SetExternalKubeClientSet(c)
}

func (a *imageLimitRangerPlugin) SetExternalKubeInformerFactory(f informers.SharedInformerFactory) {
	a.limitRanger.SetExternalKubeInformerFactory(f)
}

func (a *imageLimitRangerPlugin) ValidateInitialization() error {
	return a.limitRanger.ValidateInitialization()
}

// Admit invokes the admission logic for checking against LimitRanges.
func (a *imageLimitRangerPlugin) Admit(attr admission.Attributes, o admission.ObjectInterfaces) error {
	if !a.SupportsAttributes(attr) {
		return nil // not applicable
	}

	err := a.limitRanger.Admit(attr, o)
	if err != nil {
		return err
	}
	return a.limitRanger.Validate(attr, o)
}

func (a *imageLimitRangerPlugin) Validate(attr admission.Attributes, o admission.ObjectInterfaces) error {
	if !a.SupportsAttributes(attr) {
		return nil // not applicable
	}

	return a.limitRanger.Validate(attr, o)
}

// SupportsAttributes is a helper that returns true if the resource is supported by the plugin.
// Implements the LimitRangerActions interface.
func (a *imageLimitRangerPlugin) SupportsAttributes(attr admission.Attributes) bool {
	if attr.GetSubresource() != "" {
		return false
	}
	gk := attr.GetKind().GroupKind()
	return image.Kind("ImageStreamMapping") == gk || legacy.Kind("ImageStreamMapping") == gk
}

// SupportsLimit provides a check to see if the limitRange is applicable to image objects.
// Implements the LimitRangerActions interface.
func (a *imageLimitRangerPlugin) SupportsLimit(limitRange *corev1.LimitRange) bool {
	if limitRange == nil {
		return false
	}

	for _, limit := range limitRange.Spec.Limits {
		if limit.Type == imagev1.LimitTypeImage {
			return true
		}
	}
	return false
}

// MutateLimit is a pluggable function to set limits on the object.
func (a *imageLimitRangerPlugin) MutateLimit(limitRange *corev1.LimitRange, kind string, obj runtime.Object) error {
	return nil
}

// ValidateLimits is a pluggable function to enforce limits on the object.
func (a *imageLimitRangerPlugin) ValidateLimit(limitRange *corev1.LimitRange, kind string, obj runtime.Object) error {
	isObj, ok := obj.(*imageapi.ImageStreamMapping)
	if !ok {
		klog.V(5).Infof("%s: received object other than ImageStreamMapping (%T)", PluginName, obj)
		return nil
	}

	image := &isObj.Image
	if err := internalimageutil.InternalImageWithMetadata(image); err != nil {
		return err
	}

	for _, limit := range limitRange.Spec.Limits {
		if err := admitImage(image.DockerImageMetadata.Size, limit); err != nil {
			return err
		}
	}

	return nil
}

// admitImage checks if the size is greater than the limit range.  Abstracted for reuse in the registry.
func admitImage(size int64, limit corev1.LimitRangeItem) error {
	if limit.Type != imagev1.LimitTypeImage {
		return nil
	}

	limitQuantity, ok := limit.Max[corev1.ResourceStorage]
	if !ok {
		return nil
	}

	imageQuantity := resource.NewQuantity(size, resource.BinarySI)
	if limitQuantity.Cmp(*imageQuantity) < 0 {
		// image size is larger than the permitted limit range max size, image is forbidden
		return newLimitExceededError(imagev1.LimitTypeImage, corev1.ResourceStorage, imageQuantity, &limitQuantity)
	}
	return nil
}
