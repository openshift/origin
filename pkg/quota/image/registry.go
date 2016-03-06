// Package image implements evaluators of usage for images stored in an internal registry. They are supposed
// to be passed to resource quota controller and origin resource quota admission plugin. As opposed to
// kubernetes evaluators that can be used both with the controller and an admission plugin, these cannot.
// That's because they're counting a number of unique images which aren't namespaced. In order to do that they
// always need to enumerate all image streams in the project to see whether the newly tagged images are new to
// the project or not. The resource quota controller iterates over them implicitly while the admission plugin
// invokes the evaluator just once on a single object. Thus different usage implementations.
//
// To instantiate a registry for use with the resource quota controller, use NewImageRegistry. To instantiate a
// registry for use with the origin resource quota admission plugin, use NewImageRegistryForAdmission.
package image

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"

	osclient "github.com/openshift/origin/pkg/client"
)

// NewImageRegistry returns a registry for quota evaluation of OpenShift resources related to images in
// internal registry. It evaluates only image streams. This registry is supposed to be used with resource
// quota controller. Contained evaluators aren't usable for admission because they assume the Usage method to
// be called on all images in the project.
func NewImageRegistry(osClient osclient.Interface) quota.Registry {
	imageStream := NewImageStreamEvaluator(osClient)
	return &generic.GenericRegistry{
		InternalEvaluators: map[unversioned.GroupKind]quota.Evaluator{
			imageStream.GroupKind(): imageStream,
		},
	}
}

// NewImageRegistryForAdmission returns a registry for quota evaluation of OpenShift resources related to
// images in internal registry. Returned registry is supposed to be used with origin resource quota admission
// plugin. It evaluates image streams, image stream mappings and image stream tags. It cannot be passed to
// resource quota controller because contained evaluators return just usage increments.
func NewImageRegistryForAdmission(osClient osclient.Interface) quota.Registry {
	imageStream := NewImageStreamAdmissionEvaluator(osClient)
	imageStreamMapping := NewImageStreamMappingEvaluator(osClient)
	imageStreamTag := NewImageStreamTagEvaluator(osClient)
	return &generic.GenericRegistry{
		InternalEvaluators: map[unversioned.GroupKind]quota.Evaluator{
			imageStream.GroupKind():        imageStream,
			imageStreamMapping.GroupKind(): imageStreamMapping,
			imageStreamTag.GroupKind():     imageStreamTag,
		},
	}
}
