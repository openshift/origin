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

// A binary that can morph into all of the other kubernetes service-catalog
// binaries. You can also soft-link to it busybox style.
//
package main

import (
	"os"

	"github.com/kubernetes-incubator/service-catalog/pkg/hyperkube"
)

func main() {
	hk := hyperkube.HyperKube{
		Name: "service-catalog",
		Long: "This is an all-in-one binary that can run any of the various Kubernetes service-catalog servers.",
	}

	hk.AddServer(NewAPIServer())
	hk.AddServer(NewControllerManager())

	hk.RunToExit(os.Args)
}
