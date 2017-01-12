package generic

import (
	genericrest "k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/storage/storagebackend"
	"k8s.io/kubernetes/pkg/storage/storagebackend/factory"
)

// Temporary hack until rebase of upstream PR 37770 ///////////////////////////////////////////////

// RESTOptions is set of configuration options to generic registries.
type RESTOptions struct {
	StorageConfig           *storagebackend.Config
	Decorator               StorageDecorator
	DeleteCollectionWorkers int

	ResourcePrefix string
}

// StorageDecorator is a function signature for producing
// a storage.Interface from given parameters.
type StorageDecorator func(
	config *storagebackend.Config,
	capacity int,
	objectType runtime.Object,
	resourcePrefix string,
	keyFunc func(obj runtime.Object) (string, error),
	newListFunc func() runtime.Object,
	trigger storage.TriggerPublisherFunc) (storage.Interface, factory.DestroyFunc)

// Returns given 'storageInterface' without any decoration.
func UndecoratedStorage(
	config *storagebackend.Config,
	capacity int,
	objectType runtime.Object,
	resourcePrefix string,
	keyFunc func(obj runtime.Object) (string, error),
	newListFunc func() runtime.Object,
	trigger storage.TriggerPublisherFunc) (storage.Interface, factory.DestroyFunc) {
	return genericrest.NewRawStorage(config)
}

// End temporary hack /////////////////////////////////////////////////////////////////////////////
