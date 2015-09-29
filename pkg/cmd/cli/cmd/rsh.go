package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	rshLong = `
Open a remote shell session to a container

This command will attempt to start a shell session in the specified pod. It will default to the
first container if none is specified, and will attempt to use '/bin/bash' as the default shell.
You may pass an optional command after the pod name, which will be executed instead of a login
shell. A TTY will be automatically allocated if standard input is interactive - use -t and -T
to override.

Note, some containers may not include a shell - use '%[1]s exec' if you need to run commands
directly.`

	rshExample = `
  # Open a shell session on the first container in pod 'foo'
  $ %[1]s rsh foo

  # Run the command 'cat /etc/resolv.conf' inside pod 'foo'
  $ %[1]s rsh foo cat /etc/resolv.conf`
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
	forceTTY, disableTTY := false, false

	cmd := &cobra.Command{
		Use:     "rsh POD [command]",
		Short:   "Start a shell session in a pod",
		Long:    fmt.Sprintf(rshLong, fullName),
		Example: fmt.Sprintf(rshExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			switch {
			case forceTTY && disableTTY:
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, "you may not specify -t and -T together"))
			case forceTTY:
				options.TTY = true
			case disableTTY:
				options.TTY = false
			default:
				options.TTY = cmdutil.IsTerminal(in)
			}
			if len(args) < 1 {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, "rsh requires a single Pod to connect to"))
			}
			options.PodName = args[0]
			args = args[1:]
			if len(args) > 0 {
				options.Command = args
			} else {
				options.Command = []string{executable}
			}

			kcmdutil.CheckErr(RunRsh(options, f, cmd, args))
		},
	}
	cmd.Flags().BoolVarP(&forceTTY, "tty", "t", false, "Force a pseudo-terminal to be allocated")
	cmd.Flags().BoolVarP(&disableTTY, "no-tty", "T", false, "Disable pseudo-terminal allocation")
	cmd.Flags().StringVar(&executable, "shell", executable, "Path to shell command")
	cmd.Flags().StringVarP(&options.ContainerName, "container", "c", "", "Container name; defaults to first container")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// RunRsh starts a remote shell session on the server
func RunRsh(options *kubecmd.ExecOptions, f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
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
