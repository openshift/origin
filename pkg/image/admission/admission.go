package admission

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	admission "k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/plugin/pkg/admission/limitranger"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	PluginName = "openshift.io/ImageLimitRange"
)

func newLimitExceededError(limitType kapi.LimitType, resourceName kapi.ResourceName, requested, limit *resource.Quantity) error {
	return fmt.Errorf("requested usage of %s exceeds the maximum limit per %s (%s > %s)", resourceName, limitType, requested.String(), limit.String())
}

func init() {
	kadmission.RegisterPlugin(PluginName, func(client clientset.Interface, config io.Reader) (kadmission.Interface, error) {
		plugin, err := NewImageLimitRangerPlugin(client, config)
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
var _ kadmission.WantsInformerFactory = &imageLimitRangerPlugin{}
var _ admission.Validator = &imageLimitRangerPlugin{}

// NewImageLimitRangerPlugin provides a new imageLimitRangerPlugin.
func NewImageLimitRangerPlugin(client clientset.Interface, config io.Reader) (kadmission.Interface, error) {
	plugin := &imageLimitRangerPlugin{
		Handler: kadmission.NewHandler(kadmission.Create),
	}
	limitRanger, err := limitranger.NewLimitRanger(client, plugin)
	if err != nil {
		return nil, err
	}
	plugin.limitRanger = limitRanger

	return plugin, nil
}

func (a *imageLimitRangerPlugin) SetInformerFactory(f informers.SharedInformerFactory) {
	w, ok := a.limitRanger.(kadmission.WantsInformerFactory)
	if !ok {
		return
	}
	w.SetInformerFactory(f)
}

func (a *imageLimitRangerPlugin) Validate() error {
	v, ok := a.limitRanger.(admission.Validator)
	if !ok {
		return fmt.Errorf("limitRanger does not implement kadmission.Validator")
	}
	return v.Validate()
}

// Admit invokes the admission logic for checking against LimitRanges.
func (a *imageLimitRangerPlugin) Admit(attr admission.Attributes) error {
	if !a.SupportsAttributes(attr) {
		return nil // not applicable
	}

	return a.limitRanger.Admit(attr)
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

// Limit is the limit range implementation that checks resource against the
// image limit ranges.
// Implements the LimitRangerActions interface
func (a *imageLimitRangerPlugin) Limit(limitRange *kapi.LimitRange, kind string, obj runtime.Object) error {
	isObj, ok := obj.(*imageapi.ImageStreamMapping)
	if !ok {
		glog.V(5).Infof("%s: received object other than ImageStreamMapping (%T)", PluginName, obj)
		return nil
	}

	image := &isObj.Image
	if err := imageapi.ImageWithMetadata(image); err != nil {
		return err
	}

	for _, limit := range limitRange.Spec.Limits {
		if err := AdmitImage(image.DockerImageMetadata.Size, limit); err != nil {
			return err
		}
	}

	return nil
}

// AdmitImage checks if the size is greater than the limit range.  Abstracted for reuse in the registry.
func AdmitImage(size int64, limit kapi.LimitRangeItem) error {
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
