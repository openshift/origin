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

package class

import (
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/output"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/spf13/cobra"
)

type getCmd struct {
	*command.Context
	lookupByUUID bool
	uuid         string
	name         string
}

// NewGetCmd builds a "svcat get classes" command
func NewGetCmd(cxt *command.Context) *cobra.Command {
	getCmd := &getCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:     "classes [name]",
		Aliases: []string{"class", "cl"},
		Short:   "List classes, optionally filtered by name",
		Example: `
  svcat get classes
  svcat get class mysqldb
  svcat get class --uuid 997b8372-8dac-40ac-ae65-758b4a5075a5
`,
		PreRunE: command.PreRunE(getCmd),
		RunE:    command.RunE(getCmd),
	}
	cmd.Flags().BoolVarP(
		&getCmd.lookupByUUID,
		"uuid",
		"u",
		false,
		"Whether or not to get the class by UUID (the default is by name)",
	)
	return cmd
}

func (c *getCmd) Validate(args []string) error {
	if len(args) > 0 {
		if c.lookupByUUID {
			c.uuid = args[0]
		} else {
			c.name = args[0]
		}
	}

	return nil
}

func (c *getCmd) Run() error {
	if c.uuid == "" && c.name == "" {
		return c.getAll()
	}

	return c.get()
}

func (c *getCmd) getAll() error {
	classes, err := c.App.RetrieveClasses()
	if err != nil {
		return err
	}

	output.WriteClassList(c.Output, classes...)
	return nil
}

func (c *getCmd) get() error {
	var class *v1beta1.ClusterServiceClass
	var err error

	if c.lookupByUUID {
		class, err = c.App.RetrieveClassByID(c.uuid)
	} else if c.name != "" {
		class, err = c.App.RetrieveClassByName(c.name)
	}
	if err != nil {
		return err
	}

	output.WriteClassList(c.Output, *class)
	return nil
}
