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
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/output"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/parameters"
	"github.com/spf13/cobra"
)

type bindCmd struct {
	*command.Context
	ns           string
	instanceName string
	bindingName  string
	secretName   string
	rawParams    []string
	params       map[string]string
	rawSecrets   []string
	secrets      map[string]string
}

// NewBindCmd builds a "svcat bind" command
func NewBindCmd(cxt *command.Context) *cobra.Command {
	bindCmd := &bindCmd{Context: cxt}
	cmd := &cobra.Command{
		Use:   "bind INSTANCE_NAME",
		Short: "Binds an instance's metadata to a secret, which can then be used by an application to connect to the instance",
		Example: `
  svcat bind wordpress
  svcat bind wordpress-mysql-instance --name wordpress-mysql-binding --secret-name wordpress-mysql-secret
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return bindCmd.run(args)
		},
	}
	cmd.Flags().StringVarP(
		&bindCmd.ns,
		"namespace",
		"n",
		"default",
		"The instance namespace",
	)
	cmd.Flags().StringVarP(
		&bindCmd.bindingName,
		"name",
		"",
		"",
		"The name of the binding. Defaults to the name of the instance.",
	)
	cmd.Flags().StringVarP(
		&bindCmd.secretName,
		"secret-name",
		"",
		"",
		"The name of the secret. Defaults to the name of the instance.",
	)
	cmd.Flags().StringArrayVarP(&bindCmd.rawParams, "param", "p", nil,
		"Additional parameter to use when binding the instance, format: NAME=VALUE")
	cmd.Flags().StringArrayVarP(&bindCmd.rawSecrets, "secret", "s", nil,
		"Additional parameter, whose value is stored in a secret, to use when binding the instance, format: SECRET[KEY]")

	return cmd
}

func (c *bindCmd) run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("instance is required")
	}
	c.instanceName = args[0]

	var err error
	c.params, err = parameters.ParseVariableAssignments(c.rawParams)
	if err != nil {
		return fmt.Errorf("invalid --param value (%s)", err)
	}

	c.secrets, err = parameters.ParseKeyMaps(c.rawSecrets)
	if err != nil {
		return fmt.Errorf("invalid --secret value (%s)", err)
	}

	return c.bind()
}

func (c *bindCmd) bind() error {
	binding, err := c.App.Bind(c.ns, c.bindingName, c.instanceName, c.secretName, c.params, c.secrets)
	if err != nil {
		return err
	}

	output.WriteBindingDetails(c.Output, binding)
	return nil
}
