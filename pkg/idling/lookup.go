package idling

import (
	"fmt"

	"github.com/golang/glog"
	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

const (
	triggerServicesIndex = "triggerServices"
)

func triggerServicesIndexFunc(obj interface{}) ([]string, error) {
	idler, wasIdler := obj.(*idling.Idler)
	if !wasIdler {
		return nil, fmt.Errorf("trigger services indexer received object %v that wasn't an Idler", obj)
	}

	res := make([]string, len(idler.Spec.TriggerServiceNames))
	for i, svcName := range idler.Spec.TriggerServiceNames {
		res[i] = idler.Namespace + "/" + svcName
	}

	return res, nil
}

// IdlerServiceLookup knows how to find if there's an idler
// that's triggered by some service.
type IdlerServiceLookup interface {
	// IdlerByService returns the idler that a given service belongs to.
	IdlerByService(service types.NamespacedName) (idler *idling.Idler, hasIdler bool, err error)
}

type informerBasedLookup struct {
	// indexer indexes idlers by their corresponding trigger services
	indexer cache.Indexer
}

// NewIdlerServiceLookup creates a new IdlerServiceLookup by
// registering an index with the given informer.  Informers may
// complain if this is used multiple times with the same informer
// (due to duplicate indicies).
func NewIdlerServiceLookup(informer cache.SharedIndexInformer) (IdlerServiceLookup, error) {
	err := informer.AddIndexers(cache.Indexers{
		triggerServicesIndex: triggerServicesIndexFunc,
	})
	if err != nil {
		return nil, err
	}

	return &informerBasedLookup{
		indexer: informer.GetIndexer(),
	}, nil
}

func (l *informerBasedLookup) IdlerByService(service types.NamespacedName) (*idling.Idler, bool, error) {
	idlers, err := l.indexer.ByIndex(triggerServicesIndex, service.Namespace+"/"+service.Name)
	if err != nil {
		return nil, false, fmt.Errorf("unable to determine idler for service: %v", err)
	}
	if len(idlers) == 0 {
		return nil, false, nil
	}

	idler := idlers[0].(*idling.Idler)
	if len(idlers) > 1 {
		glog.V(6).Infof("multiple (%v) idlers for service %s, using %s", len(idlers), service.String(), idler.Name)
	}

	return idler, true, nil
}
