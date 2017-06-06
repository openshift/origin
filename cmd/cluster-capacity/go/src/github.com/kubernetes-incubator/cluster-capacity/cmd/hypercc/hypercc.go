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
	"github.com/spf13/cobra"

	capp "github.com/kubernetes-incubator/cluster-capacity/cmd/cluster-capacity/app"
	gapp "github.com/kubernetes-incubator/cluster-capacity/cmd/genpod/app"
)

// HyperCC represents a single binary that can run any cluster capacity commands.
func NewHyperCCCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hypercc <command> [flags]",
		Short: "hypercc represents a single binary that can run any cluster capacity commands.",
		Long:  "hypercc represents a single binary that can run any cluster capacity commands.",
	}
	cmd.AddCommand(capp.NewClusterCapacityCommand())
	cmd.AddCommand(gapp.NewGenPodCommand())
	return cmd
}
