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
	RshRecommendedName = "rsh"

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
  $ %[1]s foo

  # Run the command 'cat /etc/resolv.conf' inside pod 'foo'
  $ %[1]s foo cat /etc/resolv.conf`
)

// RshOptions declare the arguments accepted by the Rsh command
type RshOptions struct {
	ForceTTY   bool
	DisableTTY bool
	Executable string
	*kubecmd.ExecOptions
}

// NewCmdRsh returns a command that attempts to open a shell session to the server.
func NewCmdRsh(name string, parent string, f *clientcmd.Factory, in io.Reader, out, err io.Writer) *cobra.Command {
	options := &RshOptions{
		ForceTTY:   false,
		DisableTTY: false,
		ExecOptions: &kubecmd.ExecOptions{
			In:  in,
			Out: out,
			Err: err,

			TTY:   true,
			Stdin: true,

			Executor: &kubecmd.DefaultRemoteExecutor{},
		},
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s POD [command]", name),
		Short:   "Start a shell session in a pod",
		Long:    fmt.Sprintf(rshLong, parent),
		Example: fmt.Sprintf(rshExample, parent+" "+name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	cmd.Flags().BoolVarP(&options.ForceTTY, "tty", "t", false, "Force a pseudo-terminal to be allocated")
	cmd.Flags().BoolVarP(&options.DisableTTY, "no-tty", "T", false, "Disable pseudo-terminal allocation")
	cmd.Flags().StringVar(&options.Executable, "shell", "/bin/bash", "Path to shell command")
	cmd.Flags().StringVarP(&options.ContainerName, "container", "c", "", "Container name; defaults to first container")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

// Complete applies the command environment to RshOptions
func (o *RshOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	switch {
	case o.ForceTTY && o.DisableTTY:
		return kcmdutil.UsageError(cmd, "you may not specify -t and -T together")
	case o.ForceTTY:
		o.TTY = true
	case o.DisableTTY:
		o.TTY = false
	default:
		o.TTY = cmdutil.IsTerminal(o.In)
	}

	if len(args) < 1 {
		return kcmdutil.UsageError(cmd, "rsh requires a single Pod to connect to")
	}
	o.PodName = args[0]
	args = args[1:]
	if len(args) > 0 {
		o.Command = args
	} else {
		o.Command = []string{o.Executable}
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.Config = config

	client, err := f.Client()
	if err != nil {
		return err
	}
	o.Client = client

	return nil
}

// Validate ensures that RshOptions are valid
func (o *RshOptions) Validate() error {
	return o.ExecOptions.Validate()
}

// Run starts a remote shell session on the server
func (o *RshOptions) Run() error {
	return o.ExecOptions.Run()
}
