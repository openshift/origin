package limitrange

import (
	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrutil "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/openshift-apiserver/pkg/image/apis/image"
)

type LimitVerifier interface {
	VerifyLimits(namespace string, is *imageapi.ImageStream) error
}

type NamespaceLimiter interface {
	LimitsForNamespace(namespace string) (corev1.ResourceList, error)
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
		return kapierrors.NewForbidden(image.Resource("ImageStream"), is.Name, err)
	}
	return nil
}

func verifyImageStreamUsage(isUsage corev1.ResourceList, limits corev1.ResourceList) error {
	var errs []error

	for resource, limit := range limits {
		if usage, ok := isUsage[resource]; ok && usage.Cmp(limit) > 0 {
			errs = append(errs, newLimitExceededError(imagev1.LimitTypeImageStream, resource, &usage, &limit))
		}
	}

	return kerrutil.NewAggregate(errs)
}

type LimitRangesForNamespaceFunc func(namespace string) ([]*corev1.LimitRange, error)

func (fn LimitRangesForNamespaceFunc) LimitsForNamespace(namespace string) (corev1.ResourceList, error) {
	items, err := fn(namespace)
	if err != nil {
		return nil, err
	}
	var res corev1.ResourceList
	for _, limitRange := range items {
		res = getMaxLimits(limitRange, res)
	}
	return res, nil
}

// getMaxLimits updates the resource list to include the max allowed image count
// TODO: use the existing Max function for resource lists.
func getMaxLimits(limit *corev1.LimitRange, current corev1.ResourceList) corev1.ResourceList {
	res := current

	for _, item := range limit.Spec.Limits {
		if item.Type != imagev1.LimitTypeImageStream {
			continue
		}
		for _, resource := range []corev1.ResourceName{imagev1.ResourceImageStreamImages, imagev1.ResourceImageStreamTags} {
			if max, ok := item.Max[resource]; ok {
				if oldMax, exists := res[resource]; !exists || oldMax.Cmp(max) > 0 {
					if res == nil {
						res = make(corev1.ResourceList)
					}
					res[resource] = max
				}
			}
		}
	}

	return res
}
