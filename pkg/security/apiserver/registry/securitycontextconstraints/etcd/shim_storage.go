package etcd

import (
	"k8s.io/apiserver/pkg/registry/rest"
)

// AddSCC returns a func to adds SCC to storage
func AddSCC(sccStorage *REST) func(restStorage map[string]rest.Storage) map[string]rest.Storage {
	return func(restStorage map[string]rest.Storage) map[string]rest.Storage {
		restStorage["securityContextConstraints"] = sccStorage
		return restStorage
	}
}
