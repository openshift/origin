/*
Copyright 2018 The Kubernetes Authors.

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

package instance

import (
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/spf13/cobra"
)

type deprovisonCmd struct {
	*command.Context
	ns           string
	instanceName string
}

// NewDeprovisionCmd builds a "svcat deprovision" command
func NewDeprovisionCmd(cxt *command.Context) *cobra.Command {
	deprovisonCmd := &deprovisonCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:   "deprovision NAME",
		Short: "Deletes an instance of a service",
		Example: `
  svcat deprovision wordpress-mysql-instance
`,
		PreRunE: command.PreRunE(deprovisonCmd),
		RunE:    command.RunE(deprovisonCmd),
	}
	cmd.Flags().StringVarP(&deprovisonCmd.ns, "namespace", "n", "default",
		"The namespace of the instance")
	return cmd
}

func (c *deprovisonCmd) Validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("name is required")
	}
	c.instanceName = args[0]

	return nil
}

func (c *deprovisonCmd) Run() error {
	return c.deprovision()
}

func (c *deprovisonCmd) deprovision() error {
	return c.App.Deprovision(c.ns, c.instanceName)
}
