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

package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/broker/server"
	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/broker/user_provided/controller"
	"github.com/kubernetes-incubator/service-catalog/pkg"
)

var options struct {
	Port int
}

func init() {
	flag.IntVar(&options.Port, "port", 8005, "use '--port' option to specify the port for broker to listen on")
	flag.Parse()
}

func main() {
	if flag.Arg(0) == "version" {
		fmt.Printf("%s/%s\n", path.Base(os.Args[0]), pkg.VERSION)
		return
	}

	server.Start(options.Port, controller.CreateController())
}
