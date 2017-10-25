/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiserver

import (
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/storage"
)

// EtcdConfig contains a generic API server Config along with config specific to
// the service catalog API server.
type etcdConfig struct {
	genericConfig *genericapiserver.RecommendedConfig
	extraConfig   *extraConfig
}

// extraConfig contains all additional configuration parameters for etcdConfig
type extraConfig struct {
	// BABYNETES: cargo culted from master.go
	deleteCollectionWorkers int
	storageFactory          storage.StorageFactory
}

// NewEtcdConfig returns a new server config to describe an etcd-backed API server
func NewEtcdConfig(
	genCfg *genericapiserver.RecommendedConfig,
	deleteCollWorkers int,
	factory storage.StorageFactory,
) Config {
	return &etcdConfig{
		genericConfig: genCfg,
		extraConfig: &extraConfig{
			deleteCollectionWorkers: deleteCollWorkers,
			storageFactory:          factory,
		},
	}
}

// Complete fills in any fields not set that are required to have valid data
// and can be derived from other fields.
func (c *etcdConfig) Complete() CompletedConfig {
	completedGenericConfig := completeGenericConfig(c.genericConfig)
	return completedEtcdConfig{
		genericConfig: completedGenericConfig,
		extraConfig:   c.extraConfig,
		// Not every API group compiled in is necessarily enabled by the operator
		// at runtime.
		//
		// Install the API resource config source, which describes versions of
		// which API groups are enabled.
		apiResourceConfigSource: DefaultAPIResourceConfigSource(),
	}
}

// CompletedEtcdConfig is an internal type to take advantage of typechecking in
// the type system.
type completedEtcdConfig struct {
	genericConfig           genericapiserver.CompletedConfig
	extraConfig             *extraConfig
	apiResourceConfigSource storage.APIResourceConfigSource
}

// NewServer creates a new server that can be run. Returns a non-nil error if the server couldn't
// be created
func (c completedEtcdConfig) NewServer() (*ServiceCatalogAPIServer, error) {
	s, err := createSkeletonServer(c.genericConfig)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infoln("Created skeleton API server")

	roFactory := etcdRESTOptionsFactory{
		deleteCollectionWorkers: c.extraConfig.deleteCollectionWorkers,
		// TODO https://github.com/kubernetes/kubernetes/issues/44507
		// we still need to enable it so finalizers are respected.
		enableGarbageCollection: true,
		storageFactory:          c.extraConfig.storageFactory,
		storageDecorator:        generic.UndecoratedStorage,
	}

	glog.V(4).Infoln("Installing API groups")
	// default namespace doesn't matter for etcd
	providers := restStorageProviders("" /* default namespace */, server.StorageTypeEtcd, nil)
	for _, provider := range providers {
		groupInfo, err := provider.NewRESTStorage(c.apiResourceConfigSource, roFactory)
		if IsErrAPIGroupDisabled(err) {
			glog.Warningf("Skipping API group %v because it is not enabled", provider.GroupName())
			continue
		} else if err != nil {
			return nil, err
		}

		glog.V(4).Infof("Installing API group %v", provider.GroupName())
		if err := s.GenericAPIServer.InstallAPIGroup(groupInfo); err != nil {
			glog.Fatalf("Error installing API group %v: %v", provider.GroupName(), err)
		}
	}

	glog.Infoln("Finished installing API groups")

	return s, nil
}
