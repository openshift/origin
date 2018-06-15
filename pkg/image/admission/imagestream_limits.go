package admission

import (
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrutil "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type LimitVerifier interface {
	VerifyLimits(namespace string, is *imageapi.ImageStream) error
}

type NamespaceLimiter interface {
	LimitsForNamespace(namespace string) (kapi.ResourceList, error)
}

// NewLimitVerifier accepts a NamespaceLimiter
func NewLimitVerifier(limiter NamespaceLimiter) LimitVerifier {
	return &limitVerifier{
		limiter: limiter,
	}
}

type limitVerifier struct {
	limiter NamespaceLimiter
}

func (v *limitVerifier) VerifyLimits(namespace string, is *imageapi.ImageStream) error {
	limits, err := v.limiter.LimitsForNamespace(namespace)
	if err != nil || len(limits) == 0 {
		return err
	}

	usage := GetImageStreamUsage(is)
	if err := verifyImageStreamUsage(usage, limits); err != nil {
		return kapierrors.NewForbidden(imageapi.Resource("ImageStream"), is.Name, err)
	}
	return nil
}

func verifyImageStreamUsage(isUsage kapi.ResourceList, limits kapi.ResourceList) error {
	var errs []error

	for resource, limit := range limits {
		if usage, ok := isUsage[resource]; ok && usage.Cmp(limit) > 0 {
			errs = append(errs, newLimitExceededError(imageapi.LimitTypeImageStream, resource, &usage, &limit))
		}
	}

	return kerrutil.NewAggregate(errs)
}

type LimitRangesForNamespaceFunc func(namespace string) ([]*kapi.LimitRange, error)

func (fn LimitRangesForNamespaceFunc) LimitsForNamespace(namespace string) (kapi.ResourceList, error) {
	items, err := fn(namespace)
	if err != nil {
		return nil, err
	}
	var res kapi.ResourceList
	for _, limitRange := range items {
		res = getMaxLimits(limitRange, res)
	}
	return res, nil
}

// getMaxLimits updates the resource list to include the max allowed image count
// TODO: use the existing Max function for resource lists.
func getMaxLimits(limit *kapi.LimitRange, current kapi.ResourceList) kapi.ResourceList {
	res := current

	for _, item := range limit.Spec.Limits {
		if item.Type != imageapi.LimitTypeImageStream {
			continue
		}
		for _, resource := range []kapi.ResourceName{imageapi.ResourceImageStreamImages, imageapi.ResourceImageStreamTags} {
			if max, ok := item.Max[resource]; ok {
				if oldMax, exists := res[resource]; !exists || oldMax.Cmp(max) > 0 {
					if res == nil {
						res = make(kapi.ResourceList)
					}
					res[resource] = max
				}
			}
		}
	}

	return res
}
