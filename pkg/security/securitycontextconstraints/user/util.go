package user

import (
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	api "k8s.io/kubernetes/pkg/apis/core"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

func AnnotationToIntPtr(sUID string) (*int64, error) {
	uid, err := strconv.ParseInt(sUID, 10, 64)
	if err != nil {
		return nil, err
	}
	return &uid, nil
}

func GetAllocatedID(kClient clientset.Interface, pod *api.Pod, annotation string) (*int64, error) {
	if len(pod.Spec.ServiceAccountName) > 0 {
		sa, err := kClient.Core().ServiceAccounts(pod.Namespace).Get(pod.Spec.ServiceAccountName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		sUID, ok := sa.Annotations[annotation]
		if !ok {
			return nil, fmt.Errorf("Unable to find annotation %s on service account %s", annotation, pod.Spec.ServiceAccountName)
		}
		return AnnotationToIntPtr(sUID)
	} else {
		ns, err := kClient.Core().Namespaces().Get(pod.Namespace, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		sUID, ok := ns.Annotations[annotation]
		if !ok {
			return nil, fmt.Errorf("Unable to find annotation %s on namespace %s", annotation, pod.Namespace)
		}
		return AnnotationToIntPtr(sUID)
	}
}
