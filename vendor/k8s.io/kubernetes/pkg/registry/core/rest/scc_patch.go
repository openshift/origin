package rest

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

// LegacyStorageMutatorFunc allows someone to mutate the storage.  We use this to add SCCs.
type LegacyStorageMutatorFunc func(restStorage map[string]rest.Storage) map[string]rest.Storage

var LegacyStorageMutatorFn LegacyStorageMutatorFunc = nil

func patchStorage(restStorage map[string]rest.Storage) map[string]rest.Storage {
	if LegacyStorageMutatorFn == nil {
		return restStorage
	}

	return LegacyStorageMutatorFn(restStorage)
}
