package synthetictests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
)

func testAlerts(events monitorapi.Intervals, restConfig *rest.Config, duration time.Duration, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {
	

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return &junitapi.JUnitTestCase{
			Name: "Alert setup, kube client",
			FailureOutput: &junitapi.FailureOutput{
				Output: err.Error(),
			},
			SystemOut: err.Error(),
		}
	}

	ret := []*junitapi.JUnitTestCase{}

	_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), "openshift-monitoring", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return ret
	}

	alertTests := allowedalerts.AllAlertTests(context.TODO(), restConfig, duration)
	for i := range alertTests {
		alertTest := alertTests[i]

		junit, err := alertTest.InvariantCheck(context.TODO(), restConfig, events, *recordedResource)
		if err != nil {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: alertTest.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: err.Error(),
				},
				SystemOut: err.Error(),
			})
		}
		ret = append(ret, junit...)
	}

	return ret
}
