/*
Copyright 2016 The Kubernetes Authors.

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
	"github.com/golang/glog"
)

// Run runs the specified APIServer.  This should never exit.
func Run(opts *ServiceCatalogServerOptions, stopCh <-chan struct{}) error {
	storageType, err := opts.StorageType()
	if err != nil {
		glog.Fatalf("invalid storage type '%s' (%s)", storageType, err)
		return err
	}

	return RunServer(opts, stopCh)
}
