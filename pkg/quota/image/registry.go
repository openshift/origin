package image

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"

	osclient "github.com/openshift/origin/pkg/client"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// NewImageRegistry returns a registry for quota evaluation of OpenShift resources related to images and image
// streams.
func NewImageRegistry(osClient osclient.Interface, rcFactory quotautil.InternalRegistryClientFactory) quota.Registry {
	imageStream := NewImageStreamEvaluator(osClient, rcFactory)
	imageStreamMapping := NewImageStreamMappingEvaluator(osClient, rcFactory)
	return &generic.GenericRegistry{
		InternalEvaluators: map[unversioned.GroupKind]quota.Evaluator{
			imageStream.GroupKind():        imageStream,
			imageStreamMapping.GroupKind(): imageStreamMapping,
		},
	}
}
