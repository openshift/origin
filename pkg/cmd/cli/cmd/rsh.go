package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	rshLong = `
Open a remote shell session to a container

This command will attempt to start a shell session in the specified pod. It will default to the
first container if none is specified, and will attempt to use '/bin/bash' as the default shell.
Note, some containers may not include a shell - use '%[1]s exec' if you need to run commands
directly.`

	rshExample = `
  // Open a shell session on the first container in pod 'foo'
  $ %[1]s rsh foo`
)

// NewCmdRsh attempts to open a shell session to the server.
func NewCmdRsh(fullName string, f *clientcmd.Factory, in io.Reader, out, err io.Writer) *cobra.Command {
	options := &kubecmd.ExecOptions{
		In:  in,
		Out: out,
		Err: err,

		TTY:   true,
		Stdin: true,

		Executor: &kubecmd.DefaultRemoteExecutor{},
	}
	executable := "/bin/bash"

	cmd := &cobra.Command{
		Use:     "rsh POD",
		Short:   "Start a shell session in a pod",
		Long:    fmt.Sprintf(rshLong, fullName),
		Example: fmt.Sprintf(rshExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			options.Args = []string{executable}
			cmdutil.CheckErr(RunRsh(options, f, cmd, args))
		},
	}
	cmd.Flags().StringVar(&executable, "shell", executable, "Path to shell command")
	cmd.Flags().StringVarP(&options.ContainerName, "container", "c", "", "Container name; defaults to first container")
	return cmd
}

// RunRsh starts a remote shell session on the server
func RunRsh(options *kubecmd.ExecOptions, f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return cmdutil.UsageError(cmd, "rsh requires a single POD to connect to")
	}
	options.PodName = args[0]

	_, client, err := f.Clients()
	if err != nil {
		return err
	}
	options.Client = client

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return nil
	}
	options.Namespace = namespace

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}
	options.Config = config

	if err := options.Validate(); err != nil {
		return err
	}
	return options.Run()
}
