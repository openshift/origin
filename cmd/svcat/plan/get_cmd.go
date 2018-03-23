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
	"strings"

	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/output"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/svcat/service-catalog"
	"github.com/spf13/cobra"
)

type getCmd struct {
	*command.Context
	lookupByUUID bool
	uuid         string
	name         string

	classFilter string
	classUUID   string
	className   string
}

// NewGetCmd builds a "svcat get plans" command
func NewGetCmd(cxt *command.Context) *cobra.Command {
	getCmd := &getCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:     "plans [name]",
		Aliases: []string{"plan", "pl"},
		Short:   "List plans, optionally filtered by name or class",
		Example: `
  svcat get plans
  svcat get plan PLAN_NAME
  svcat get plan CLASS_NAME/PLAN_NAME
  svcat get plan --uuid PLAN_UUID
  svcat get plans --class CLASS_NAME
  svcat get plan --class CLASS_NAME PLAN_NAME
  svcat get plans --uuid --class CLASS_UUID
  svcat get plan --uuid --class CLASS_UUID PLAN_UUID
`,
		PreRunE: command.PreRunE(getCmd),
		RunE:    command.RunE(getCmd),
	}
	cmd.Flags().BoolVarP(
		&getCmd.lookupByUUID,
		"uuid",
		"u",
		false,
		"Whether or not to get the plan by UUID (the default is by name)",
	)
	cmd.Flags().StringVarP(
		&getCmd.classFilter,
		"class",
		"c",
		"",
		"Filter plans based on class. When --uuid is specified, the class name is interpreted as a uuid.",
	)
	return cmd
}

func (c *getCmd) Validate(args []string) error {
	if len(args) > 0 {
		if c.lookupByUUID {
			c.uuid = args[0]
		} else if strings.Contains(args[0], "/") {
			names := strings.Split(args[0], "/")
			if len(names) != 2 {
				return fmt.Errorf("failed to parse class/plan name combination '%s'", c.name)
			}
			c.className = names[0]
			c.name = names[1]
		} else {
			c.name = args[0]
		}
	}
	if c.classFilter != "" {
		if c.lookupByUUID {
			c.classUUID = c.classFilter
		} else {
			c.className = c.classFilter
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

	var opts *servicecatalog.FilterOptions

	// Retrieve the classes as well because plans don't have the external class name
	classes, err := c.App.RetrieveClasses()
	if err != nil {
		return fmt.Errorf("unable to list classes (%s)", err)
	}

	if c.classFilter != "" {
		if !c.lookupByUUID {
			// Map the external class name to the class name.
			for _, class := range classes {
				if c.className == class.Spec.ExternalName {
					c.classUUID = class.Name
					break
				}
			}
		}
		opts = &servicecatalog.FilterOptions{
			ClassID: c.classUUID,
		}
	}

	plans, err := c.App.RetrievePlans(opts)
	if err != nil {
		return fmt.Errorf("unable to list plans (%s)", err)
	}

	output.WritePlanList(c.Output, plans, classes)
	return nil
}

func (c *getCmd) get() error {
	var plan *v1beta1.ClusterServicePlan
	var err error
	switch {
	case c.lookupByUUID:
		plan, err = c.App.RetrievePlanByID(c.uuid)

	case c.className != "":
		plan, err = c.App.RetrievePlanByClassAndPlanNames(c.className, c.name)

	default:
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
