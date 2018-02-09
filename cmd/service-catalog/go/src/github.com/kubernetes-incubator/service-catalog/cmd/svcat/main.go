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

package main

import (
	"fmt"
	"os"

	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/binding"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/broker"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/class"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/command"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/instance"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/plan"
	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/plugin"
	"github.com/kubernetes-incubator/service-catalog/pkg/svcat"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// These are build-time values, set during an official release
var (
	commit  string
	version string
)

func main() {
	cmd := buildRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCommand() *cobra.Command {
	// root command context
	cxt := &command.Context{
		Viper: viper.New(),
	}

	// root command flags
	var opts struct {
		Version     bool
		KubeConfig  string
		KubeContext string
	}

	cmd := &cobra.Command{
		Use:          "svcat",
		Short:        "The Kubernetes Service Catalog Command-Line Interface (CLI)",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Enable tests to swap the output
			cxt.Output = cmd.OutOrStdout()

			// Initialize flags from environment variables
			bindViperToCobra(cxt.Viper, cmd)

			app, err := svcat.NewApp(opts.KubeConfig, opts.KubeContext)
			cxt.App = app

			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Version {
				printVersion(cxt)
				return nil
			}

			fmt.Fprint(cxt.Output, cmd.UsageString())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&opts.Version, "version", "v", false, "Show the application version")

	if plugin.IsPlugin() {
		plugin.BindEnvironmentVariables(cxt.Viper)
	} else {
		cmd.PersistentFlags().StringVar(&opts.KubeContext, "kube-context", "", "name of the kube context to use")
		cmd.PersistentFlags().StringVar(&opts.KubeConfig, "kubeconfig", "", "path to kubeconfig file. Overrides $KUBECONFIG")
	}

	cmd.AddCommand(newGetCmd(cxt))
	cmd.AddCommand(newDescribeCmd(cxt))
	cmd.AddCommand(instance.NewProvisionCmd(cxt))
	cmd.AddCommand(instance.NewDeprovisionCmd(cxt))
	cmd.AddCommand(binding.NewBindCmd(cxt))
	cmd.AddCommand(binding.NewUnbindCmd(cxt))
	cmd.AddCommand(newSyncCmd(cxt))
	cmd.AddCommand(newInstallCmd(cxt))

	return cmd
}

func printVersion(cxt *command.Context) {
	if commit == "" { // commit is empty for Homebrew builds
		fmt.Fprintf(cxt.Output, "svcat %s\n", version)
	} else {
		fmt.Fprintf(cxt.Output, "svcat %s (%s)\n", version, commit)
	}
}

func newSyncCmd(cxt *command.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Syncs service catalog for a service broker",
		Aliases: []string{"relist"},
	}
	cmd.AddCommand(broker.NewSyncCmd(cxt))

	return cmd
}

func newGetCmd(cxt *command.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "List a resource, optionally filtered by name",
	}
	cmd.AddCommand(binding.NewGetCmd(cxt))
	cmd.AddCommand(broker.NewGetCmd(cxt))
	cmd.AddCommand(class.NewGetCmd(cxt))
	cmd.AddCommand(instance.NewGetCmd(cxt))
	cmd.AddCommand(plan.NewGetCmd(cxt))

	return cmd
}

func newDescribeCmd(cxt *command.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Show details of a specific resource",
	}
	cmd.AddCommand(binding.NewDescribeCmd(cxt))
	cmd.AddCommand(broker.NewDescribeCmd(cxt))
	cmd.AddCommand(class.NewDescribeCmd(cxt))
	cmd.AddCommand(instance.NewDescribeCmd(cxt))
	cmd.AddCommand(plan.NewDescribeCmd(cxt))

	return cmd
}

func newInstallCmd(cxt *command.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use: "install",
	}
	cmd.AddCommand(plugin.NewInstallCmd(cxt))

	return cmd
}

// Bind the viper configuration back to the cobra command flags.
// Allows us to interact with the cobra flags normally, and while still
// using viper's automatic environment variable binding.
func bindViperToCobra(vip *viper.Viper, cmd *cobra.Command) {
	vip.BindPFlags(cmd.Flags())
	vip.AutomaticEnv()
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Changed && vip.IsSet(f.Name) {
			cmd.Flags().Set(f.Name, vip.GetString(f.Name))
		}
	})
}
