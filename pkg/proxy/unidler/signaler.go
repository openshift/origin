package unidler

import (
	"fmt"

	idlingclient "github.com/openshift/service-idler/pkg/client/clientset/versioned/typed/idling/v1alpha2"
	"k8s.io/apimachinery/pkg/types"

	idlingutil "github.com/openshift/origin/pkg/idling"
)

type NeedPodsSignaler interface {
	// NeedPods signals that endpoint addresses are needed in order to
	// service a traffic coming to the given service and port
	NeedPods(serviceName types.NamespacedName) error
}

type idlerSignaler struct {
	idlerClient idlingclient.IdlersGetter
	lookup      idlingutil.IdlerServiceLookup
}

func (s *idlerSignaler) NeedPods(service types.NamespacedName) error {
	idler, present, err := s.lookup.IdlerByService(service)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("no idler found for service %s", service.String())
	}

	if idler.Spec.WantIdle == false {
		// we're already in the process of unidling, no need to ask again
		// and flood the API server with requests
		return nil
	}

	// patch to avoid conflicts, since we don't really care about anything but the `wantIdled` field
	_, err = s.idlerClient.Idlers(idler.Namespace).Patch(idler.Name, types.JSONPatchType, idlingutil.UnidlePatchData)
	return err
}

// NewIdlerSignaler creates a new NeedPodsSignaler that users the idling.openshift.io APIs
func NewIdlerSignaler(client idlingclient.IdlersGetter, lookup idlingutil.IdlerServiceLookup) NeedPodsSignaler {
	return &idlerSignaler{
		idlerClient: client,
		lookup:      lookup,
	}
}
