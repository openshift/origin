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

// Config is the raw configuration information for a server to fill in. After the config
// information is entered, calling code should call Complete to prepare to start the server
type Config interface {
	Complete() CompletedConfig
}

// CompletedConfig is the result of a Config being Complete()-ed. Calling code should call Start()
// to start a server from its completed config
type CompletedConfig interface {
	NewServer(stopCh <-chan struct{}) (*ServiceCatalogAPIServer, error)
}
