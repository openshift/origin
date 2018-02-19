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

package broker

import (
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/output"
	"github.com/spf13/cobra"
)

type getCmd struct {
	*command.Context
	name string
}

// NewGetCmd builds a "svcat get brokers" command
func NewGetCmd(cxt *command.Context) *cobra.Command {
	getCmd := &getCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:     "brokers [name]",
		Aliases: []string{"broker", "brk"},
		Short:   "List brokers, optionally filtered by name",
		Example: `
  svcat get brokers
  svcat get broker asb
`,
		PreRunE: command.PreRunE(getCmd),
		RunE:    command.RunE(getCmd),
	}

	return cmd
}

func (c *getCmd) Validate(args []string) error {
	if len(args) > 0 {
		c.name = args[0]
	}

	return nil
}

func (c *getCmd) Run() error {
	if c.name == "" {
		return c.getAll()
	}

	return c.get()
}

func (c *getCmd) getAll() error {
	brokers, err := c.App.RetrieveBrokers()
	if err != nil {
		return err
	}

	output.WriteBrokerList(c.Output, brokers...)
	return nil
}

func (c *getCmd) get() error {
	broker, err := c.App.RetrieveBroker(c.name)
	if err != nil {
		return err
	}

	output.WriteBrokerList(c.Output, *broker)
	return nil
}
