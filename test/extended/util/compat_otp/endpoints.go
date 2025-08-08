package compat_otp

import (
	"context"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func WaitForEndpointsAvailable(oc *exutil.CLI, serviceName string) error {
	return wait.Poll(200*time.Millisecond, 3*time.Minute, func() (bool, error) {
		ep, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return false, err
		}

		return (len(ep.Subsets) > 0) && (len(ep.Subsets[0].Addresses) > 0), nil
	})
}
