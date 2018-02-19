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
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/spf13/cobra"
)

type syncCmd struct {
	*command.Context
	name string
}

// NewSyncCmd builds a "svcat sync broker" command
func NewSyncCmd(cxt *command.Context) *cobra.Command {
	syncCmd := &syncCmd{Context: cxt}
	rootCmd := &cobra.Command{
		Use:     "broker [name]",
		Short:   "Syncs service catalog for a service broker",
		PreRunE: command.PreRunE(syncCmd),
		RunE:    command.RunE(syncCmd),
	}
	return rootCmd
}

func (c *syncCmd) Validate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}
	c.name = args[0]
	return nil
}

func (c *syncCmd) Run() error {
	return c.sync()
}

func (c *syncCmd) sync() error {
	const retries = 3
	err := c.App.Sync(c.name, retries)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.Output, "Successfully fetched catalog entries from the %s broker\n", c.name)
	return nil
}
