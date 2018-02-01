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

package binding

import (
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/spf13/cobra"
)

type unbindCmd struct {
	*command.Context
	ns           string
	instanceName string
	bindingName  string
}

// NewUnbindCmd builds a "svcat unbind" command
func NewUnbindCmd(cxt *command.Context) *cobra.Command {
	unbindCmd := unbindCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:   "unbind INSTANCE_NAME",
		Short: "Unbinds an instance. When an instance name is specified, all of its bindings are removed, otherwise use --name to remove a specific binding",
		Example: `
  svcat unbind wordpress-mysql-instance
  svcat unbind --name wordpress-mysql-binding
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return unbindCmd.run(args)
		},
	}

	cmd.Flags().StringVarP(
		&unbindCmd.ns,
		"namespace",
		"n",
		"default",
		"The namespace of the instance or binding",
	)
	cmd.Flags().StringVar(
		&unbindCmd.bindingName,
		"name",
		"",
		"The name of the binding to remove",
	)
	return cmd
}

func (c *unbindCmd) run(args []string) error {
	if len(args) == 0 {
		if c.bindingName == "" {
			return fmt.Errorf("an instance or binding name is required")
		}

		return c.App.DeleteBinding(c.ns, c.bindingName)
	}

	c.instanceName = args[0]
	return c.App.Unbind(c.ns, c.instanceName)
}
