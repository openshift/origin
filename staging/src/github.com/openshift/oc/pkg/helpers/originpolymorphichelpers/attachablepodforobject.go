package originpolymorphichelpers

import (
	"sort"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsv1 "github.com/openshift/api/apps/v1"
)

func NewAttachablePodForObjectFn(delegate polymorphichelpers.AttachablePodForObjectFunc) polymorphichelpers.AttachablePodForObjectFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, object runtime.Object, timeout time.Duration) (*v1.Pod, error) {
		switch t := object.(type) {
		case *appsv1.DeploymentConfig:
			config, err := restClientGetter.ToRESTConfig()
			if err != nil {
				return nil, err
			}
			coreClient, err := kubernetes.NewForConfig(config)
			if err != nil {
				return nil, err
			}

			selector := labels.SelectorFromSet(t.Spec.Selector)
			f := func(pods []*v1.Pod) sort.Interface {
				return sort.Reverse(controller.ActivePods(pods))
			}
			pod, _, err := polymorphichelpers.GetFirstPod(coreClient.CoreV1(), t.Namespace, selector.String(), 1*time.Minute, f)
			return pod, err

		default:
			return delegate(restClientGetter, object, timeout)
		}
	}
}
