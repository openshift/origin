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

package server

import (
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"github.com/kubernetes-incubator/service-catalog/pkg/storage/etcd"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/storagebackend/factory"
)

type errUnsupportedStorageType struct {
	t StorageType
}

func (e errUnsupportedStorageType) Error() string {
	return fmt.Sprintf("unsupported storage type %s", e.t)
}

// StorageType represents the type of storage a storage interface should use
type StorageType string

// StorageTypeFromString converts s to a valid StorageType. Returns StorageType("") and a non-nil
// error if s names an invalid or unsupported storage type
func StorageTypeFromString(s string) (StorageType, error) {
	switch s {
	case StorageTypeEtcd.String():
		return StorageTypeEtcd, nil
	default:
		return StorageType(""), errUnsupportedStorageType{t: StorageType(s)}
	}
}

func (s StorageType) String() string {
	return string(s)
}

const (
	// StorageTypeEtcd indicates a storage interface should use etcd
	StorageTypeEtcd StorageType = "etcd"
)

// Options is the extension of a generic.RESTOptions struct, complete with service-catalog
// specific things
type Options struct {
	EtcdOptions etcd.Options
	storageType StorageType
}

// NewOptions returns a new Options with the given parameters
func NewOptions(
	etcdOpts etcd.Options,
	sType StorageType,
) *Options {
	return &Options{
		EtcdOptions: etcdOpts,
		storageType: sType,
	}
}

// StorageType returns the storage type the rest server should use, or an error if an unsupported
// storage type is indicated
func (o Options) StorageType() (StorageType, error) {
	switch o.storageType {
	case StorageTypeEtcd:
		return o.storageType, nil
	default:
		return StorageType(""), errUnsupportedStorageType{t: o.storageType}
	}
}

// ResourcePrefix gets the resource prefix of all etcd keys
func (o Options) ResourcePrefix() string {
	return o.EtcdOptions.RESTOptions.ResourcePrefix
}

// KeyRootFunc returns the appropriate key root function for the storage type in o.
// This function produces a path that etcd or TPR storage understands, to the root of the resource
// by combining the namespace in the context with the given prefix
func (o Options) KeyRootFunc() func(genericapirequest.Context) string {
	prefix := o.ResourcePrefix()
	sType, err := o.StorageType()
	if err != nil {
		return nil
	}
	if sType == StorageTypeEtcd {
		return func(ctx genericapirequest.Context) string {
			return registry.NamespaceKeyRootFunc(ctx, prefix)
		}
	}
	// This should never happen, catch for potential bugs
	panic("Unexpected storage type: " + sType)
}

// KeyFunc returns the appropriate key function for the storage type in o.
// This function should produce a path that etcd or TPR storage understands, to the resource
// by combining the namespace in the context with the given prefix
func (o Options) KeyFunc(namespaced bool) func(genericapirequest.Context, string) (string, error) {
	prefix := o.ResourcePrefix()
	sType, err := o.StorageType()
	if err != nil {
		return nil
	}
	if sType == StorageTypeEtcd {
		return func(ctx genericapirequest.Context, name string) (string, error) {
			if namespaced {
				return registry.NamespaceKeyFunc(ctx, prefix, name)
			}
			return registry.NoNamespaceKeyFunc(ctx, prefix, name)
		}
	}
	panic("Unexpected storage type: " + o.storageType)
}

// GetStorage returns the storage from the given parameters
func (o Options) GetStorage(
	capacity int,
	objectType runtime.Object,
	resourcePrefix string,
	scopeStrategy rest.NamespaceScopedStrategy,
	newListFunc func() runtime.Object,
	getAttrsFunc storage.AttrFunc,
	trigger storage.TriggerPublisherFunc,
) (storage.Interface, factory.DestroyFunc) {
	if o.storageType == StorageTypeEtcd {
		etcdRESTOpts := o.EtcdOptions.RESTOptions
		return etcdRESTOpts.Decorator(
			api.Scheme,
			etcdRESTOpts.StorageConfig,
			&capacity,
			objectType,
			resourcePrefix,
			nil, /* keyFunc for decorator -- looks to be unused everywhere */
			newListFunc,
			getAttrsFunc,
			trigger,
		)
	}
	panic("Unexpected storage type: " + o.storageType)
}
