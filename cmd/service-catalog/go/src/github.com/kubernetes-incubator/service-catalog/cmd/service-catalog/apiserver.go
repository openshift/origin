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

package main

import (
	"github.com/kubernetes-incubator/service-catalog/cmd/apiserver/app/server"
	"github.com/kubernetes-incubator/service-catalog/pkg/hyperkube"
)

// NewAPIServer creates a new hyperkube Server object that includes the
// description and flags.
func NewAPIServer() *hyperkube.Server {
	s := server.NewServiceCatalogServerOptions()

	hks := hyperkube.Server{
		PrimaryName:     "apiserver",
		AlternativeName: "service-catalog-apiserver",
		SimpleUsage:     "apiserver",
		Long:            "The main API entrypoint and interface to the storage system.  The API server is also the focal point for all authorization decisions.",
		Run: func(_ *hyperkube.Server, args []string, stopCh <-chan struct{}) error {
			return server.Run(s, stopCh)
		},
		RespectsStopCh: true,
	}
	s.AddFlags(hks.Flags())
	return &hks
}
