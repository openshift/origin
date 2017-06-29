// Package image implements evaluators of usage for imagestreams and images. They are supposed
// to be passed to resource quota controller and origin resource quota admission plugin.
package image

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinternalversion "github.com/openshift/origin/pkg/image/generated/informers/internalversion/image/internalversion"
)

// NewImageQuotaRegistry returns a registry for quota evaluation of OpenShift resources related to images in
// internal registry. It evaluates only image streams and related virtual resources that can cause a creation
// of new image stream objects.
func NewImageQuotaRegistry(isInformer imageinternalversion.ImageStreamInformer, osClient osclient.Interface) quota.Registry {
	imageStream := NewImageStreamEvaluator(isInformer.Lister())
	imageStreamTag := NewImageStreamTagEvaluator(isInformer.Lister(), osClient)
	imageStreamImport := NewImageStreamImportEvaluator(isInformer.Lister())
	return &generic.GenericRegistry{
		InternalEvaluators: map[schema.GroupKind]quota.Evaluator{
			imageStream.GroupKind():       imageStream,
			imageStreamTag.GroupKind():    imageStreamTag,
			imageStreamImport.GroupKind(): imageStreamImport,
		},
	}
}

// NewImageQuotaRegistryForAdmission returns a registry for quota evaluation of OpenShift resources related to images in
// internal registry. It evaluates only image streams and related virtual resources that can cause a creation
// of new image stream objects.
// This is different that is used for reconciliation because admission has to check all forms of a resource (legacy and groupified), but
// reconciliation only has to check one.
func NewImageQuotaRegistryForAdmission(isInformer imageinternalversion.ImageStreamInformer, osClient osclient.Interface) quota.Registry {
	imageStream := NewImageStreamEvaluator(isInformer.Lister())
	imageStreamTag := NewImageStreamTagEvaluator(isInformer.Lister(), osClient)
	imageStreamImport := NewImageStreamImportEvaluator(isInformer.Lister())
	return &generic.GenericRegistry{
		// TODO remove the LegacyKind entries below when the legacy api group is no longer supported
		InternalEvaluators: map[schema.GroupKind]quota.Evaluator{
			imageapi.LegacyKind("ImageStream"):       imageStream,
			imageStream.GroupKind():                  imageStream,
			imageapi.LegacyKind("ImageStreamTag"):    imageStreamTag,
			imageStreamTag.GroupKind():               imageStreamTag,
			imageapi.LegacyKind("ImageStreamImport"): imageStreamImport,
			imageStreamImport.GroupKind():            imageStreamImport,
		},
	}
}
