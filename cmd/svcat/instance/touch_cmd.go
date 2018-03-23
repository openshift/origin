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

type touchInstanceCmd struct {
	*command.Context
	namespace string
	name      string
}

// NewTouchCommand builds a "svcat touch instance" command.
func NewTouchCommand(cxt *command.Context) *cobra.Command {
	touchInstanceCmd := &touchInstanceCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Touch an instance to make service-catalog try to process the spec again",
		Long: `Touch instance will increment the updateRequests field on the instance. 
Then, service catalog will process the instance's spec again. It might do an update, a delete, or 
nothing.`,
		Example: `svcat touch instance wordpress-mysql-instance --namespace mynamespace`,
		PreRunE: command.PreRunE(touchInstanceCmd),
		RunE:    command.RunE(touchInstanceCmd),
	}
	cmd.Flags().StringVarP(&touchInstanceCmd.namespace, "namespace", "n", "default",
		"The namespace for the instance to touch")
	return cmd
}

func (c *touchInstanceCmd) Validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("an instance name is required")
	}
	c.name = args[0]

	return nil
}

func (c *touchInstanceCmd) Run() error {
	const retries = 3
	return c.App.TouchInstance(c.namespace, c.name, retries)
}
