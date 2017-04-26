/*
Copyright 2014 The Kubernetes Authors.

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

package etcd

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/cachesize"
	"k8s.io/kubernetes/pkg/registry/securitycontextconstraints"
)

// REST implements a RESTStorage for security context constraints against etcd
type REST struct {
	*registry.Store
}

// NewREST returns a RESTStorage object that will work against security context constraints objects.
func NewREST(optsGetter generic.RESTOptionsGetter) *REST {
	store := &registry.Store{
		Copier:      api.Scheme,
		NewFunc:     func() runtime.Object { return &api.SecurityContextConstraints{} },
		NewListFunc: func() runtime.Object { return &api.SecurityContextConstraintsList{} },
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.SecurityContextConstraints).Name, nil
		},
		PredicateFunc:     securitycontextconstraints.Matcher,
		QualifiedResource: api.Resource("securitycontextconstraints"),
		WatchCacheSize:    cachesize.GetWatchCacheSizeByResource("securitycontextconstraints"),

		CreateStrategy:      securitycontextconstraints.Strategy,
		UpdateStrategy:      securitycontextconstraints.Strategy,
		DeleteStrategy:      securitycontextconstraints.Strategy,
		ReturnDeletedObject: true,
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: securitycontextconstraints.GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		panic(err) // TODO: Propagate error up
	}
	return &REST{store}
}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"scc"}
}
