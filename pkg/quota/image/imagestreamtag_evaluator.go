package image

import (
	"fmt"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/generic"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const imageStreamTagEvaluatorName = "Evaluator.ImageStreamTag"

// NewImageStreamTagEvaluator computes resource usage of ImageStreamsTags. Its sole purpose is to handle
// UPDATE admission operations on imageStreamTags resource.
func NewImageStreamTagEvaluator(store *oscache.StoreToImageStreamLister, istNamespacer osclient.ImageStreamTagsNamespacer) kquota.Evaluator {
	computeResources := []kapi.ResourceName{
		imageapi.ResourceImageStreams,
	}

	matchesScopeFunc := func(kapi.ResourceQuotaScope, runtime.Object) bool { return true }
	getFuncByNamespace := func(namespace, id string) (runtime.Object, error) {
		isName, tag, err := imageapi.ParseImageStreamTagName(id)
		if err != nil {
			return nil, err
		}

		obj, err := istNamespacer.ImageStreamTags(namespace).Get(isName, tag)
		if err != nil {
			if !kerrors.IsNotFound(err) {
				return nil, err
			}
			obj = &imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: namespace,
					Name:      id,
				},
			}
		}
		return obj, nil
	}

	return &generic.GenericEvaluator{
		Name:              imageStreamTagEvaluatorName,
		InternalGroupKind: imageapi.LegacyKind("ImageStreamTag"),
		InternalOperationResources: map[admission.Operation][]kapi.ResourceName{
			admission.Update: computeResources,
			admission.Create: computeResources,
		},
		MatchedResourceNames: computeResources,
		MatchesScopeFunc:     matchesScopeFunc,
		UsageFunc:            makeImageStreamTagAdmissionUsageFunc(store),
		GetFuncByNamespace:   getFuncByNamespace,
		ListFuncByNamespace: func(namespace string, options kapi.ListOptions) ([]runtime.Object, error) {
			return []runtime.Object{}, nil
		},
		ConstraintsFunc: imageStreamTagConstraintsFunc,
	}
}

// imageStreamTagConstraintsFunc checks that given object is an image stream tag
func imageStreamTagConstraintsFunc(required []kapi.ResourceName, object runtime.Object) error {
	if _, ok := object.(*imageapi.ImageStreamTag); !ok {
		return fmt.Errorf("unexpected input object %v", object)
	}
	return nil
}

// makeImageStreamTagAdmissionUsageFunc returns a function that computes a resource usage for given image
// stream tag during admission.
func makeImageStreamTagAdmissionUsageFunc(store *oscache.StoreToImageStreamLister) generic.UsageFunc {
	return func(object runtime.Object) kapi.ResourceList {
		ist, ok := object.(*imageapi.ImageStreamTag)
		if !ok {
			return kapi.ResourceList{}
		}

		res := map[kapi.ResourceName]resource.Quantity{
			imageapi.ResourceImageStreams: *resource.NewQuantity(0, resource.BinarySI),
		}

		isName, _, err := imageapi.ParseImageStreamTagName(ist.Name)
		if err != nil {
			utilruntime.HandleError(err)
			return kapi.ResourceList{}
		}

		is, err := store.ImageStreams(ist.Namespace).Get(isName)
		if err != nil && !kerrors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("failed to get image stream %s/%s: %v", ist.Namespace, isName, err))
		}
		if is == nil || kerrors.IsNotFound(err) {
			res[imageapi.ResourceImageStreams] = *resource.NewQuantity(1, resource.BinarySI)
		}

		return res
	}
}
