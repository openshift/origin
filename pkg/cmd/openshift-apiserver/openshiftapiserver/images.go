package openshiftapiserver

import (
	"fmt"

	"github.com/golang/glog"

	imageadmission "github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	coreinternalinformer "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/core/internalversion"
)

func ImageLimitVerifier(limitRangeInformer coreinternalinformer.LimitRangeInformer) imageadmission.LimitVerifier {
	//func ImageLimitVerifier(limitRangeInformer corev1informers.LimitRangeInformer) imageadmission.LimitVerifier {
	// this call just forces the informer to be registered
	limitRangeInformer.Informer()

	return imageadmission.NewLimitVerifier(imageadmission.LimitRangesForNamespaceFunc(func(ns string) ([]*kapi.LimitRange, error) {
		//return imageadmission.NewLimitVerifier(imageadmission.LimitRangesForNamespaceFunc(func(ns string) ([]*corev1.LimitRange, error) {
		list, err := limitRangeInformer.Lister().LimitRanges(ns).List(labels.Everything())
		if err != nil {
			return nil, err
		}
		// the verifier must return an error
		if len(list) == 0 && len(limitRangeInformer.Informer().LastSyncResourceVersion()) == 0 {
			glog.V(4).Infof("LimitVerifier still waiting for ranges to load: %#v", limitRangeInformer.Informer())
			forbiddenErr := apierrors.NewForbidden(schema.GroupResource{Resource: "limitranges"}, "", fmt.Errorf("the server is still loading limit information"))
			forbiddenErr.ErrStatus.Details.RetryAfterSeconds = 1
			return nil, forbiddenErr
		}
		return list, nil
	}))
}
