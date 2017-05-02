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
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"

	capp "github.com/kubernetes-incubator/cluster-capacity/cmd/cluster-capacity/app"
	gapp "github.com/kubernetes-incubator/cluster-capacity/cmd/genpod/app"
)

func main() {
	switch path.Base(os.Args[0]) {
	case "cluster-capacity":
		cmdExecute(capp.NewClusterCapacityCommand())
	case "genpod":
		cmdExecute(gapp.NewGenPodCommand())
	default:
		cmdExecute(NewHyperCCCommand())
	}
}

func cmdExecute(cmd *cobra.Command) {
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
