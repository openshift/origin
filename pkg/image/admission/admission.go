package admission

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/watch"
	limitranger "k8s.io/kubernetes/plugin/pkg/admission/limitranger"

	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagequota "github.com/openshift/origin/pkg/quota/image"
)

const (
	PluginName = "ImageLimitRange"
)

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
	*kadmission.Handler
	osClient    client.Interface
	limitRanger kadmission.Interface

	imageStreamReflector *cache.Reflector
	imageStreamStore     cache.Store
	stopChan             chan struct{}
}

// imageLimitRangerPlugin implements the LimitRangerActions interface.
var _ limitranger.LimitRangerActions = &imageLimitRangerPlugin{}

var _ = oadmission.WantsOpenshiftClient(&imageLimitRangerPlugin{})
var _ = oadmission.Validator(&imageLimitRangerPlugin{})

// NewImageLimitRangerPlugin provides a new imageLimitRangerPlugin.  Also see SetOpenshiftClient.
func NewImageLimitRangerPlugin(client clientset.Interface, config io.Reader) (*imageLimitRangerPlugin, error) {
	plugin := &imageLimitRangerPlugin{
		Handler:          kadmission.NewHandler(kadmission.Create, kadmission.Update),
		imageStreamStore: cache.NewStore(cache.MetaNamespaceKeyFunc),
	}
	limitRanger, err := limitranger.NewLimitRanger(client, plugin)
	if err != nil {
		return nil, err
	}
	plugin.limitRanger = limitRanger

	// NOTE: the reflector is set up in SetOpenshiftClient to ensure we have access to the client
	return plugin, nil
}

// Run starts the reflectors used by the plugin.
func (a *imageLimitRangerPlugin) Run() {
	if a.stopChan == nil {
		a.stopChan = make(chan struct{})
		a.imageStreamReflector.RunUntil(a.stopChan)
	}
}

// Stop stops the reflectors used by the plugin.
func (a *imageLimitRangerPlugin) Stop() {
	if a.stopChan != nil {
		close(a.stopChan)
		a.stopChan = nil
	}
}

// Admit invokes the admission logic for checking against LimitRanges.
func (a *imageLimitRangerPlugin) Admit(attr kadmission.Attributes) error {
	if !a.SupportsAttributes(attr) {
		return nil // not applicable
	}

	// If the namespace is not set on the object ensure it gets set.
	if om, err := kapi.ObjectMetaFor(attr.GetObject()); err == nil {
		if om.Namespace == "" {
			glog.V(5).Infof("%s: namespace was not set for object, defaulting to %s", PluginName, attr.GetNamespace())
			om.Namespace = attr.GetNamespace()
		}
	}

	return a.limitRanger.Admit(attr)
}

// SupportsAttributes is a helper that returns true if the resource is supported by the plugin.
// Implements the LimitRangerActions interface.
func (a *imageLimitRangerPlugin) SupportsAttributes(attr kadmission.Attributes) bool {
	if attr.GetSubresource() != "" {
		return false
	}

	resource := attr.GetResource()
	return resource == imageapi.Resource("imagestreammappings") ||
		resource == imageapi.Resource("imagestreamtags") ||
		resource == imageapi.Resource("imagestreams")
}

// SupportsLimit provides a check to see if the limitRange is applicable to image objects.
// Implements the LimitRangerActions interface.
func (a *imageLimitRangerPlugin) SupportsLimit(limitRange *kapi.LimitRange) bool {
	if limitRange != nil {
		for _, limit := range limitRange.Spec.Limits {
			if limit.Type == imageapi.LimitTypeImageSize {
				return true
			}
		}
	}
	return false
}

// Limit is the limit range implementation that checks resource against the
// image limit ranges.
// Implements the LimitRangerActions interface
func (a *imageLimitRangerPlugin) Limit(limitRange *kapi.LimitRange, kind string, obj runtime.Object) error {
	var images map[string]imageapi.Image

	switch isObj := obj.(type) {
	case *imageapi.ImageStreamMapping:
		images = map[string]imageapi.Image{isObj.Image.Name: isObj.Image}
	case *imageapi.ImageStreamTag:
		if isObj.Tag == nil || isObj.Tag.From == nil {
			glog.V(4).Infof("%s: ignoring ist with no tag reference to follow: %v", PluginName, isObj)
			return nil
		}

		computer := imagequota.NewGenericImageStreamUsageComputer(a.osClient, true, false)
		ref, err := computer.GetImageReferenceForObjectReference(isObj.Namespace, isObj.Tag.From)
		if err != nil {
			return err
		}
		image, err := computer.GetImage(ref.ID)
		if err != nil {
			return err
		}
		if image == nil {
			return fmt.Errorf("unable to find image with ID %s", ref.ID)
		}
		images = map[string]imageapi.Image{image.Name: *image}
	case *imageapi.ImageStream:
		previousIS, exists, err := a.imageStreamStore.Get(isObj)
		if err != nil {
			// don't return, process all images in the IS
			glog.V(4).Infof("%s: error retrieving previous image stream from cache: %v", PluginName, err)
		}

		if !exists {
			images, err = a.getImageStreamImages(isObj, nil)
		} else {
			images, err = a.getImageStreamImages(isObj, previousIS.(*imageapi.ImageStream))
		}
		if err != nil {
			return err
		}
	default:
		glog.V(5).Infof("%s: recieved object that was not an ImageStreamMapping, ImageStreamTag, or ImageStream.  Ignoring", PluginName)
		return nil
	}

	if len(images) == 0 {
		glog.V(4).Infof("%s: unable to validate image limit ranges for %#v, no images were found", PluginName, obj)
		return nil
	}

	errs := []error{}

	for _, img := range images {
		if value, exists := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !exists || value != "true" {
			glog.V(5).Infof("%s: image is not managed by openshift, ignoring", PluginName)
			continue
		}

		// ensure metadata is loaded from the manifest, this may not be automatically filled in if
		// an older registry is in use
		if err := imageapi.ImageWithMetadata(&img); err != nil {
			errs = append(errs, err)
			continue
		}

		for _, limit := range limitRange.Spec.Limits {
			if err := AdmitImage(img.DockerImageMetadata.Size, limit); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) != 0 {
		return utilerrors.NewAggregate(errs)
	}

	return nil
}

// getImageStreamImages gives back a slice of images that need checked against the limit range
// based on the image stream.  It filters out preexisting images that do not need to be checked
// by using the previousIS.
func (a *imageLimitRangerPlugin) getImageStreamImages(is, previousIS *imageapi.ImageStream) (map[string]imageapi.Image, error) {
	computer := imagequota.NewGenericImageStreamUsageComputer(a.osClient, true, false)

	previousImages := map[string]imageapi.Image{}
	if previousIS != nil {
		// get images referenced in the previous image stream so we can filter them out of
		// the new images stream so we are only guarding against new images
		handler := newImageRecordingHandler(previousImages)
		err := computer.ProcessImageStreamImages(previousIS, handler)
		if err != nil {
			return nil, err
		}
	}

	images := map[string]imageapi.Image{}
	handler := newImageRecordingHandler(images)
	err := computer.ProcessImageStreamImages(is, handler)
	if err != nil {
		return nil, err
	}

	// filter out anything in previous
	imagesToProcess := map[string]imageapi.Image{}
	for _, img := range images {
		if _, ok := previousImages[img.Name]; !ok {
			imagesToProcess[img.Name] = img
		}
	}

	glog.V(4).Infof("%s: prevImages: %v, newImages: %v, toProcess: %v", PluginName, len(previousImages), len(images), len(imagesToProcess))

	return imagesToProcess, nil
}

// SetOpenshiftClient provides init functionality for anything that requires the OpenShift client.
func (a *imageLimitRangerPlugin) SetOpenshiftClient(c client.Interface) {
	a.osClient = c

	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return a.osClient.ImageStreams(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return a.osClient.ImageStreams(kapi.NamespaceAll).Watch(options)
			},
		},
		&imageapi.ImageStream{},
		a.imageStreamStore,
		0,
	)
	a.imageStreamReflector = reflector

	a.Run()
}

// Validate ensures that the OpenShift client is set on the plugin.
func (a *imageLimitRangerPlugin) Validate() error {
	if a.osClient == nil {
		return fmt.Errorf("%s needs an Openshift client", PluginName)
	}
	return nil
}

// AdmitImage checks if the size is greater than the limit range.  Abstracted for reuse in the registry.
func AdmitImage(size int64, limit kapi.LimitRangeItem) error {
	imageQuantity := resource.NewQuantity(size, resource.BinarySI)

	if limit.Type == imageapi.LimitTypeImageSize {
		if limitQuantity, ok := limit.Max[kapi.ResourceStorage]; ok {
			if limitQuantity.Cmp(*imageQuantity) < 0 {
				// image size is larger than the permitted limit range max size, image is forbidden
				return fmt.Errorf("%s exceeds the maximum %s usage per %s (%s)", imageQuantity.String(), kapi.ResourceStorage, imageapi.LimitTypeImageSize, limitQuantity.String())
			}
		}
	}
	return nil
}

// newImageRecordingHandler provides a handler that will record each image seen in images.  Images
// will be keyed by image.Name.
func newImageRecordingHandler(images map[string]imageapi.Image) imagequota.ImageStreamComputerFunc {
	return func(tag string, dockerImageReference string, image *imageapi.Image) error {
		images[image.Name] = *image
		return nil
	}
}
