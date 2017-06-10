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
	"fmt"

	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/storage"
)

// ErrAPIGroupDisabled is an error indicating that an API group should be disabled
type ErrAPIGroupDisabled struct {
	Name string
}

func (e ErrAPIGroupDisabled) Error() string {
	return fmt.Sprintf("API group %s is disabled", e.Name)
}

// IsErrAPIGroupDisabled returns true if e is an error indicating that an API group is disabled
func IsErrAPIGroupDisabled(e error) bool {
	_, ok := e.(ErrAPIGroupDisabled)
	return ok
}

// RESTStorageProvider is a local interface describing a REST storage factory.
// It can report the name of the API group and create a new storage interface
// for it.
type RESTStorageProvider interface {
	// GroupName returns the API group name that the storage manages
	GroupName() string
	// NewRESTStorage returns a new group info representing the storage this provider provides. If
	// the API group should be disabled, a non-nil error will be returned, and
	// IsErrAPIGroupDisabled(e) will return true for it. Otherwise, if the storage couldn't be
	// created, a different non-nil error will be returned
	// second parameter indicates whether the API group should be enabled. A non-nil error will
	// be returned if a new group info couldn't be created
	NewRESTStorage(
		apiResourceConfigSource storage.APIResourceConfigSource,
		restOptionsGetter generic.RESTOptionsGetter,
	) (*genericapiserver.APIGroupInfo, error)
}
