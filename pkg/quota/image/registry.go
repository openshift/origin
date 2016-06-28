// Package image implements evaluators of usage for imagestreams and images. They are supposed
// to be passed to resource quota controller and origin resource quota admission plugin.
package image

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"

	osclient "github.com/openshift/origin/pkg/client"
)

// NewImageQuotaRegistry returns a registry for quota evaluation of OpenShift resources related to images in
// internal registry. It evaluates only image streams and related virtual resources that can cause a creation
// of new image stream objects.
func NewImageQuotaRegistry(osClient osclient.Interface) quota.Registry {
	imageStream := NewImageStreamEvaluator(osClient)
	imageStreamTag := NewImageStreamTagEvaluator(osClient, osClient)
	imageStreamImport := NewImageStreamImportEvaluator(osClient)
	return &generic.GenericRegistry{
		InternalEvaluators: map[unversioned.GroupKind]quota.Evaluator{
			imageStream.GroupKind():       imageStream,
			imageStreamTag.GroupKind():    imageStreamTag,
			imageStreamImport.GroupKind(): imageStreamImport,
		},
	}
}
