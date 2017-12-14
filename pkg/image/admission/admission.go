package admission

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	admission "k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/plugin/pkg/admission/limitranger"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/util"
)

const (
	PluginName = "openshift.io/ImageLimitRange"
)

func newLimitExceededError(limitType kapi.LimitType, resourceName kapi.ResourceName, requested, limit *resource.Quantity) error {
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
	limitRanger admission.Interface
}

// imageLimitRangerPlugin implements the LimitRangerActions interface.
var _ limitranger.LimitRangerActions = &imageLimitRangerPlugin{}
var _ kadmission.WantsInternalKubeInformerFactory = &imageLimitRangerPlugin{}
var _ kadmission.WantsInternalKubeClientSet = &imageLimitRangerPlugin{}

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

func (q *imageLimitRangerPlugin) SetInternalKubeClientSet(c kclientset.Interface) {
	q.limitRanger.(kadmission.WantsInternalKubeClientSet).SetInternalKubeClientSet(c)
}

func (a *imageLimitRangerPlugin) SetInternalKubeInformerFactory(f informers.SharedInformerFactory) {
	a.limitRanger.(kadmission.WantsInternalKubeInformerFactory).SetInternalKubeInformerFactory(f)
}

func (a *imageLimitRangerPlugin) ValidateInitialization() error {
	v, ok := a.limitRanger.(admission.InitializationValidator)
	if !ok {
		return fmt.Errorf("limitRanger does not implement kadmission.Validator")
	}
	return v.ValidateInitialization()
}

// Admit invokes the admission logic for checking against LimitRanges.
func (a *imageLimitRangerPlugin) Admit(attr admission.Attributes) error {
	if !a.SupportsAttributes(attr) {
		return nil // not applicable
	}

	err := a.limitRanger.(admission.MutationInterface).Admit(attr)
	if err != nil {
		return err
	}
	return a.limitRanger.(admission.ValidationInterface).Validate(attr)
}

func (a *imageLimitRangerPlugin) Validate(attr admission.Attributes) error {
	if !a.SupportsAttributes(attr) {
		return nil // not applicable
	}

	return a.limitRanger.(admission.ValidationInterface).Validate(attr)
}

// SupportsAttributes is a helper that returns true if the resource is supported by the plugin.
// Implements the LimitRangerActions interface.
func (a *imageLimitRangerPlugin) SupportsAttributes(attr admission.Attributes) bool {
	if attr.GetSubresource() != "" {
		return false
	}
	gk := attr.GetKind().GroupKind()
	return imageapi.IsKindOrLegacy("ImageStreamMapping", gk)
}

// SupportsLimit provides a check to see if the limitRange is applicable to image objects.
// Implements the LimitRangerActions interface.
func (a *imageLimitRangerPlugin) SupportsLimit(limitRange *kapi.LimitRange) bool {
	if limitRange == nil {
		return false
	}

	for _, limit := range limitRange.Spec.Limits {
		if limit.Type == imageapi.LimitTypeImage {
			return true
		}
	}
	return false
}

// MutateLimit is a pluggable function to set limits on the object.
func (a *imageLimitRangerPlugin) MutateLimit(limitRange *kapi.LimitRange, kind string, obj runtime.Object) error {
	return nil
}

// ValidateLimits is a pluggable function to enforce limits on the object.
func (a *imageLimitRangerPlugin) ValidateLimit(limitRange *kapi.LimitRange, kind string, obj runtime.Object) error {
	isObj, ok := obj.(*imageapi.ImageStreamMapping)
	if !ok {
		glog.V(5).Infof("%s: received object other than ImageStreamMapping (%T)", PluginName, obj)
		return nil
	}

	image := &isObj.Image
	if err := util.ImageWithMetadata(image); err != nil {
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
func admitImage(size int64, limit kapi.LimitRangeItem) error {
	if limit.Type != imageapi.LimitTypeImage {
		return nil
	}

	limitQuantity, ok := limit.Max[kapi.ResourceStorage]
	if !ok {
		return nil
	}

	imageQuantity := resource.NewQuantity(size, resource.BinarySI)
	if limitQuantity.Cmp(*imageQuantity) < 0 {
		// image size is larger than the permitted limit range max size, image is forbidden
		return newLimitExceededError(imageapi.LimitTypeImage, kapi.ResourceStorage, imageQuantity, &limitQuantity)
	}
	return nil
}
