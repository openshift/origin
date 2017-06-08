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

// The apiserver is the api server and master for the service catalog.
// It is responsible for serving the service catalog management API.

package main

import (
	"os"

	"github.com/golang/glog"
	// set up logging the k8s way
	"k8s.io/apiserver/pkg/util/logs"

	// The API groups for our API must be installed before we can use the
	// client to work with them.  This needs to be done once per process; this
	// is the point at which we handle this for the API server process.
	// Please do not remove.
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"

	_ "github.com/kubernetes-incubator/service-catalog/cmd/apiserver/app"
	"github.com/kubernetes-incubator/service-catalog/cmd/apiserver/app/server"
)

func main() {
	logs.InitLogs()
	// make sure we print all the logs while shutting down.
	defer logs.FlushLogs()

	cmd, err := server.NewCommandServer(os.Stdout)
	if err != nil {
		glog.Errorf("Error creating server: %v", err)
		logs.FlushLogs()
		os.Exit(1)
	}

	if err := cmd.Execute(); err != nil {
		glog.Errorf("server exited unexpectedly (%s)", err)
		logs.FlushLogs()
		os.Exit(1)
	}
}
