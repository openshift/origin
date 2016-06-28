package admission

import (
	"fmt"
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	kerrutil "k8s.io/kubernetes/pkg/util/errors"
	watch "k8s.io/kubernetes/pkg/watch"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

type LimitVerifier interface {
	VerifyLimits(namespace string, is *imageapi.ImageStream) error
}

func NewLimitVerifier(client kclient.LimitRangesNamespacer) LimitVerifier {
	lw := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return client.LimitRanges(kapi.NamespaceAll).List(options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return client.LimitRanges(kapi.NamespaceAll).Watch(options)
		},
	}
	indexer, reflector := cache.NewNamespaceKeyedIndexerAndReflector(lw, &kapi.LimitRange{}, 0)
	reflector.Run()
	return &limitVerifier{
		client:  client,
		indexer: indexer,
	}
}

type limitVerifier struct {
	client  kclient.LimitRangesNamespacer
	indexer cache.Indexer
}

func (v *limitVerifier) VerifyLimits(namespace string, is *imageapi.ImageStream) error {
	items, err := v.indexer.Index("namespace", &kapi.LimitRange{ObjectMeta: kapi.ObjectMeta{Namespace: namespace}})
	if err != nil {
		return fmt.Errorf("error resolving limit ranges: %v", err)
	}
	if len(items) == 0 {
		return nil
	}
	limits := getMaxLimits(items)
	if len(limits) == 0 {
		return nil
	}

	usage := GetImageStreamUsage(is)

	if err := verifyImageStreamUsage(usage, limits); err != nil {
		return kapierrors.NewForbidden(imageapi.Resource("ImageStream"), is.Name, err)
	}
	return nil
}

func getMaxLimits(limits []interface{}) kapi.ResourceList {
	res := kapi.ResourceList{}

	for _, limitObject := range limits {
		lr := limitObject.(*kapi.LimitRange)
		for _, item := range lr.Spec.Limits {
			if item.Type != imageapi.LimitTypeImageStream {
				continue
			}
			for _, resource := range []kapi.ResourceName{imageapi.ResourceImageStreamImages, imageapi.ResourceImageStreamTags} {
				if max, ok := item.Max[resource]; ok {
					if oldMax, exists := res[resource]; !exists || oldMax.Cmp(max) > 0 {
						res[resource] = max
					}
				}
			}
		}
	}

	return res
}

func verifyImageStreamUsage(isUsage kapi.ResourceList, limits kapi.ResourceList) error {
	errs := []error{}

	for resource, limit := range limits {
		if usage, ok := isUsage[resource]; ok && usage.Cmp(limit) > 0 {
			errs = append(errs, newLimitExceededError(imageapi.LimitTypeImageStream, resource, &usage, &limit))
		}
	}

	return kerrutil.NewAggregate(errs)
}
