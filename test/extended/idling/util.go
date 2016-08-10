package idling

import (
	"time"

	"github.com/openshift/origin/pkg/util/errors"
	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/wait"
)

func waitForEndpointsAvailable(oc *exutil.CLI, serviceName string) error {
	return wait.Poll(200*time.Millisecond, 2*time.Minute, func() (bool, error) {
		ep, err := oc.KubeREST().Endpoints(oc.Namespace()).Get(serviceName)
		// Tolerate NotFound b/c it could take a moment for the endpoints to be created
		if errors.TolerateNotFoundError(err) != nil {
			return false, err
		}

		return (len(ep.Subsets) > 0) && (len(ep.Subsets[0].Addresses) > 0), nil
	})
}

func waitForNoPodsAvailable(oc *exutil.CLI) error {
	return wait.Poll(200*time.Millisecond, 2*time.Minute, func() (bool, error) {
		//ep, err := oc.KubeREST().Endpoints(oc.Namespace()).Get(serviceName)
		pods, err := oc.KubeREST().Pods(oc.Namespace()).List(kapi.ListOptions{})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == 0, nil
	})
}
