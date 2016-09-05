package restoptions

import (
	"fmt"

	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/storage"
)

// ApplyOptions updates the given generic storage from the provided rest options
// TODO: remove need for etcdPrefix once Decorator interface is refactored upstream
func ApplyOptions(optsGetter Getter, store *registry.Store, etcdPrefix string, triggerFn storage.TriggerPublisherFunc) error {
	if store.QualifiedResource.Empty() {
		return fmt.Errorf("store must have a non-empty qualified resource")
	}
	if store.NewFunc == nil {
		return fmt.Errorf("store for %s must have NewFunc set", store.QualifiedResource.String())
	}
	if store.NewListFunc == nil {
		return fmt.Errorf("store for %s must have NewListFunc set", store.QualifiedResource.String())
	}
	if store.CreateStrategy == nil {
		return fmt.Errorf("store for %s must have CreateStrategy set", store.QualifiedResource.String())
	}

	opts, err := optsGetter.GetRESTOptions(store.QualifiedResource)
	if err != nil {
		return fmt.Errorf("error building RESTOptions for %s store: %v", store.QualifiedResource.String(), err)
	}

	store.DeleteCollectionWorkers = opts.DeleteCollectionWorkers
	store.Storage, _ = opts.Decorator(
		opts.StorageConfig,
		UseConfiguredCacheSize,
		store.NewFunc(),
		etcdPrefix,
		store.CreateStrategy,
		store.NewListFunc,
		triggerFn,
	)
	return nil

}
