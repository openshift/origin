package idling

import (
	"time"

	"github.com/openshift/origin/pkg/util/errors"
	exutil "github.com/openshift/origin/test/extended/util"
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
