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

package plan

import (
	"fmt"

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

// NewGetCmd builds a "svcat get plans" command
func NewGetCmd(cxt *command.Context) *cobra.Command {
	getCmd := &getCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:     "plans [name]",
		Aliases: []string{"plan", "pl"},
		Short:   "List plans, optionally filtered by name",
		Example: `
  svcat get plans
  svcat get plan standard800
  svcat get plan --uuid 08e4b43a-36bc-447e-a81f-8202b13e339c
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return getCmd.run(args)
		},
	}
	cmd.Flags().BoolVarP(
		&getCmd.lookupByUUID,
		"uuid",
		"u",
		false,
		"Whether or not to get the plan by UUID (the default is by name)",
	)
	return cmd
}

func (c *getCmd) run(args []string) error {
	if len(args) == 0 {
		return c.getAll()
	}

	if c.lookupByUUID {
		c.uuid = args[0]
	} else {
		c.name = args[0]
	}

	return c.get()
}

func (c *getCmd) getAll() error {
	plans, err := c.App.RetrievePlans()
	if err != nil {
		return fmt.Errorf("unable to list plans (%s)", err)
	}

	// Retrieve the classes as well because plans don't have the external class name
	classes, err := c.App.RetrieveClasses()
	if err != nil {
		return fmt.Errorf("unable to list classes (%s)", err)
	}

	output.WritePlanList(c.Output, plans, classes)
	return nil
}

func (c *getCmd) get() error {
	var plan *v1beta1.ClusterServicePlan
	var err error
	if c.lookupByUUID {
		plan, err = c.App.RetrievePlanByID(c.uuid)
	} else {
		plan, err = c.App.RetrievePlanByName(c.name)
	}
	if err != nil {
		return err
	}

	// Retrieve the class as well because plans don't have the external class name
	class, err := c.App.RetrieveClassByID(plan.Spec.ClusterServiceClassRef.Name)
	if err != nil {
		return err
	}

	output.WritePlanList(c.Output, []v1beta1.ClusterServicePlan{*plan}, []v1beta1.ClusterServiceClass{*class})

	return nil
}
