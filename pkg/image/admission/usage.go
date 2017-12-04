package admission

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// InternalImageReferenceHandler is a function passed to the computer when processing images that allows a
// caller to perform actions on image references. The handler is called on a unique image reference just once.
// Argument inSpec says whether the image reference is present in an image stream spec. The inStatus says the
// same for an image stream status.
//
// The reference can either be:
//
//  1. a docker image reference (e.g. 172.30.12.34:5000/test/is2:tag)
//  2. an image stream tag (e.g. project/isname:latest)
//  3. an image ID (e.g. sha256:2643199e5ed5047eeed22da854748ed88b3a63ba0497601ba75852f7b92d4640)
//
// The first two a can be obtained only from IS spec. Processing of IS status can generate only the 3rd
// option.
//
// The docker image reference will always be normalized such that registry url is always specified while a
// default docker namespace and tag are stripped.
type InternalImageReferenceHandler func(imageReference string, inSpec, inStatus bool) error

// GetImageStreamUsage counts number of unique internally managed images occupying given image stream. It
// returns a number of unique image references found in the image stream spec not contained in
// processedSpecRefs and a number of unique image hashes contained in iS status not contained in
// processedStatusRefs. Given sets will be updated with new references found.
func GetImageStreamUsage(is *imageapi.ImageStream) kapi.ResourceList {
	specRefs := resource.NewQuantity(0, resource.DecimalSI)
	statusRefs := resource.NewQuantity(0, resource.DecimalSI)

	ProcessImageStreamImages(is, false, func(ref string, inSpec, inStatus bool) error {
		if inSpec {
			specRefs.Set(specRefs.Value() + 1)
		}
		if inStatus {
			statusRefs.Set(statusRefs.Value() + 1)
		}
		return nil
	})

	return kapi.ResourceList{
		imageapi.ResourceImageStreamTags:   *specRefs,
		imageapi.ResourceImageStreamImages: *statusRefs,
	}
}

// ProcessImageStreamImages is a utility method that calls a given handler on every image reference found in
// the given image stream. If specOnly is true, only image references found in is spec will be processed. The
// handler will be called just once for each unique image reference.
func ProcessImageStreamImages(is *imageapi.ImageStream, specOnly bool, handler InternalImageReferenceHandler) error {
	type sources struct{ inSpec, inStatus bool }
	var statusReferences sets.String
	imageReferences := make(map[string]*sources)

	specReferences := gatherImagesFromImageStreamSpec(is)
	for ref := range specReferences {
		imageReferences[ref] = &sources{inSpec: true}
	}

	if !specOnly {
		statusReferences = gatherImagesFromImageStreamStatus(is)
		for ref := range statusReferences {
			if s, exists := imageReferences[ref]; exists {
				s.inStatus = true
			} else {
				imageReferences[ref] = &sources{inStatus: true}
			}
		}
	}

	for ref, s := range imageReferences {
		if err := handler(ref, s.inSpec, s.inStatus); err != nil {
			return err
		}
	}
	return nil
}

// gatherImagesFromImageStreamStatus is a utility method that collects all image references found in a status
// of a given image stream.
func gatherImagesFromImageStreamStatus(is *imageapi.ImageStream) sets.String {
	res := sets.NewString()

	for _, history := range is.Status.Tags {
		for i := range history.Items {
			ref := history.Items[i].Image
			if len(ref) == 0 {
				continue
			}

			res.Insert(ref)
		}
	}

	return res
}

// gatherImagesFromImageStreamSpec is a utility method that collects all image references found in a spec of a
// given image stream
func gatherImagesFromImageStreamSpec(is *imageapi.ImageStream) sets.String {
	res := sets.NewString()

	for _, tagRef := range is.Spec.Tags {
		if tagRef.From == nil {
			continue
		}

		ref, err := GetImageReferenceForObjectReference(is.Namespace, tagRef.From)
		if err != nil {
			glog.V(4).Infof("could not process object reference: %v", err)
			continue
		}

		res.Insert(ref)
	}

	return res
}

// GetImageReferenceForObjectReference returns corresponding image reference for the given object
// reference representing either an image stream image or image stream tag or docker image.
func GetImageReferenceForObjectReference(namespace string, objRef *kapi.ObjectReference) (string, error) {
	switch objRef.Kind {
	case "ImageStreamImage", "DockerImage":
		res, err := imageapi.ParseDockerImageReference(objRef.Name)
		if err != nil {
			return "", err
		}

		if objRef.Kind == "ImageStreamImage" {
			if res.Namespace == "" {
				res.Namespace = objRef.Namespace
			}
			if res.Namespace == "" {
				res.Namespace = namespace
			}
			if len(res.ID) == 0 {
				return "", fmt.Errorf("missing id in ImageStreamImage reference %q", objRef.Name)
			}

		} else {
			// objRef.Kind == "DockerImage"
			res = res.DockerClientDefaults()
		}

		// docker image reference
		return res.DaemonMinimal().Exact(), nil

	case "ImageStreamTag":
		isName, tag, err := imageapi.ParseImageStreamTagName(objRef.Name)
		if err != nil {
			return "", err
		}

		ns := namespace
		if len(objRef.Namespace) > 0 {
			ns = objRef.Namespace
		}

		// <namespace>/<isname>:<tag>
		return cache.MetaNamespaceKeyFunc(&metav1.ObjectMeta{
			Namespace: ns,
			Name:      imageapi.JoinImageStreamTag(isName, tag),
		})
	}

	return "", fmt.Errorf("unsupported object reference kind %s", objRef.Kind)
}
