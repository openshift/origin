package cmd

import (
	"fmt"
	"github.com/openshift/origin/pkg/cmd/experimental/diagnostics/options"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/templates"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/diagnostics/run"
	"github.com/spf13/cobra"
	"io"
)

const longAllDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot OpenShift. It is
intended to be run from the same context as an OpenShift client or running
master / node in order to troubleshoot from the perspective of each.

    $ %[1]s

If run without flags or subcommands, it will check for config files for
client, master, and node, and if found, use them for troubleshooting
those components. If master/node config files are not found, the tool
assumes they are not present and does diagnostics only as a client.

You may also specify config files explicitly with flags below, in which
case you will receive an error if they are invalid or not found.

    $ %[1]s --master-config=/etc/openshift/master/master-config.yaml

Subcommands may be used to scope the troubleshooting to a particular
component and are not limited to using config files; you can and should
use the same flags that are actually set on the command line for that
component to configure the diagnostic.

    $ %[1]s node --hostname='node.example.com' --kubeconfig=...

NOTE: This is an alpha version of diagnostics and will change significantly.
NOTE: Global flags (from the 'options' subcommand) are ignored here but
can be used with subcommands.
`

func NewCommandDiagnostics(name string, fullName string, out io.Writer) *cobra.Command {
	opts := options.NewAllDiagnosticsOptions(out)
	cmd := &cobra.Command{
		Use:   name,
		Short: "This utility helps you understand and troubleshoot OpenShift v3.",
		Long:  fmt.Sprintf(longAllDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			opts.GlobalFlags = c.PersistentFlags()
			run.Diagnose(opts)
		},
	}
	cmd.SetOutput(out) // for output re: usage / help
	opts.BindFlags(cmd.Flags(), options.NewAllDiagnosticsFlagInfos())
	// Although we reuse DiagOptions across all commands, we do not want the flags buried
	// in the "global" flags, so we add them locally at each command.
	opts.DiagOptions.BindFlags(cmd.Flags(), options.NewDiagnosticsFlagInfos())

	/*
	   This command needs the client factory built in the "client" subcommand.
	   Generating the factory adds flags to the "client" cmd, and we do not want
	   to add those flags to this command (the only client option here is a config
	   file). So the factory object from client cmd is reused for this command.
	*/
	clientCmd, factory := NewClientCommand("client", name+" client", out)
	opts.ClientDiagOptions.Factory = factory

	cmd.AddCommand(clientCmd)
	cmd.AddCommand(NewMasterCommand("master", name+" master", out))
	cmd.AddCommand(NewNodeCommand("node", name+" node", out))
	cmd.AddCommand(NewOptionsCommand())

	return cmd
}

const longClientDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot OpenShift as a user. It is
intended to be run from the same context as an OpenShift client
("openshift cli" or "osc") and with the same configuration options.

    $ %s
`

func NewClientCommand(name string, fullName string, out io.Writer) (*cobra.Command, *osclientcmd.Factory) {
	opts := options.NewClientDiagnosticsOptions(out, nil)
	cmd := &cobra.Command{
		Use:   name,
		Short: "Troubleshoot using the OpenShift v3 client.",
		Long:  fmt.Sprintf(longClientDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			run.Diagnose(&options.AllDiagnosticsOptions{
				ClientDiagOptions: opts,
				DiagOptions:       opts.DiagOptions,
				GlobalFlags:       c.PersistentFlags(),
			})
		},
	}
	cmd.SetOutput(out) // for output re: usage / help
	opts.MustCheck = true
	opts.Factory = osclientcmd.New(cmd.PersistentFlags()) // side effect: add standard persistent flags for openshift client
	opts.BindFlags(cmd.Flags(), options.NewClientDiagnosticsFlagInfos())
	opts.DiagOptions.BindFlags(cmd.Flags(), options.NewDiagnosticsFlagInfos())

	cmd.AddCommand(NewOptionsCommand())
	return cmd, opts.Factory
}

const longMasterDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot a running OpenShift
master. It is intended to be run from the same context as the master
(where "openshift start" or "openshift start master" is run, possibly from
systemd or inside a container) and with the same configuration options.

    $ %s
`

func NewMasterCommand(name string, fullName string, out io.Writer) *cobra.Command {
	opts := options.NewMasterDiagnosticsOptions(out, nil)
	cmd := &cobra.Command{
		Use:   name,
		Short: "Troubleshoot an OpenShift v3 master.",
		Long:  fmt.Sprintf(longMasterDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			run.Diagnose(&options.AllDiagnosticsOptions{
				MasterDiagOptions: opts,
				DiagOptions:       opts.DiagOptions,
				GlobalFlags:       c.PersistentFlags(),
			})
		},
	}
	cmd.SetOutput(out) // for output re: usage / help
	opts.MustCheck = true
	opts.MasterStartOptions = &start.MasterOptions{MasterArgs: start.MasterArgsAndFlags(cmd.Flags())}
	opts.BindFlags(cmd.Flags(), options.NewMasterDiagnosticsFlagInfos())
	opts.DiagOptions.BindFlags(cmd.Flags(), options.NewDiagnosticsFlagInfos())

	cmd.AddCommand(NewOptionsCommand())
	return cmd
}

const longNodeDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot a running OpenShift
node. It is intended to be run from the same context as the node
(where "openshift start" or "openshift start node" is run, possibly from
systemd or inside a container) and with the same configuration options.

    $ %s
`

func NewNodeCommand(name string, fullName string, out io.Writer) *cobra.Command {
	opts := options.NewNodeDiagnosticsOptions(out, nil)
	cmd := &cobra.Command{
		Use:   name,
		Short: "Troubleshoot an OpenShift v3 node.",
		Long:  fmt.Sprintf(longNodeDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			run.Diagnose(&options.AllDiagnosticsOptions{
				NodeDiagOptions: opts,
				DiagOptions:     opts.DiagOptions,
				GlobalFlags:     c.PersistentFlags(),
			})
		},
	}
	cmd.SetOutput(out) // for output re: usage / help
	opts.MustCheck = true
	opts.NodeStartOptions = &start.NodeOptions{NodeArgs: start.NodeArgsAndFlags(cmd.Flags())}
	opts.BindFlags(cmd.Flags(), options.NewNodeDiagnosticsFlagInfos())
	opts.DiagOptions.BindFlags(cmd.Flags(), options.NewDiagnosticsFlagInfos())

	cmd.AddCommand(NewOptionsCommand())
	return cmd
}

func NewOptionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "options",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}

	templates.UseOptionsTemplates(cmd)

	return cmd
}
