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
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/output"
	"github.com/spf13/cobra"
)

type getCmd struct {
	*command.Context
	ns   string
	name string
}

// NewGetCmd builds a "svcat get instances" command
func NewGetCmd(cxt *command.Context) *cobra.Command {
	getCmd := &getCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:     "instances [name]",
		Aliases: []string{"instance", "inst"},
		Short:   "List instances, optionally filtered by name",
		Example: `
  svcat get instances
  svcat get instance wordpress-mysql-instance
  svcat get instance -n ci concourse-postgres-instance
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getCmd.run(args)
		},
	}
	cmd.Flags().StringVarP(
		&getCmd.ns,
		"namespace",
		"n",
		"default",
		"The namespace in which to get the ServiceInstance",
	)
	return cmd
}

func (c *getCmd) run(args []string) error {
	if len(args) == 0 {
		return c.getAll()
	}

	c.name = args[0]
	return c.get()
}

func (c *getCmd) getAll() error {
	instances, err := c.App.RetrieveInstances(c.ns)
	if err != nil {
		return err
	}

	output.WriteInstanceList(c.Output, instances.Items...)
	return nil
}

func (c *getCmd) get() error {
	instance, err := c.App.RetrieveInstance(c.ns, c.name)
	if err != nil {
		return err
	}

	output.WriteInstanceList(c.Output, *instance)
	return nil
}
