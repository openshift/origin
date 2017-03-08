package docker

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/cmd/templates"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// CmdJoinRecommendedName is the recommended command name
const CmdJoinRecommendedName = "join"

var (
	cmdJoinLong = templates.LongDesc(`
		Add a new node to an existing OpenShift cluster

		Uses an existing connection to a Docker daemon to start an OpenShift node. You must provide
		a secret to connect to the master. Before running command, ensure that you can execute docker
		commands successfully (ie. 'docker ps').

		Optionally, the command can create a new Docker machine for your OpenShift node using the VirtualBox
		driver when the --create-machine argument is specified. The machine will be named 'node'
		by default. To name the machine differently, use the --docker-machine=NAME argument. If the
		--docker-machine=NAME argument is specified, but --create-machine is not, the command will attempt
		to find an existing docker machine with that name and start it if it's not running.`)

	cmdJoinExample = templates.Examples(`
		# Start a new OpenShift node on a new docker machine named 'node'
		%[1]s --create-machine

		# Start OpenShift node and preserve data and config between restarts
		%[1]s --host-data-dir=/mydata --use-existing-config

		# Use a different set of images
		%[1]s --image="registry.example.com/origin" --version="v1.1"
`)
)

// NewCmdJoin creates a command that joins an existing OpenShift cluster.
func NewCmdJoin(name, fullName string, f *osclientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	config := &ClientJoinConfig{
		CommonStartConfig: CommonStartConfig{
			Out:      out,
			UsePorts: []int{openshift.DefaultDNSPort, 10250},
			DNSPort:  openshift.DefaultDNSPort,
		},
		In: in,
	}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Join an existing OpenShift cluster",
		Long:    cmdJoinLong,
		Example: fmt.Sprintf(cmdJoinExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(config.Complete(f, c))
			kcmdutil.CheckErr(config.Validate(out))
			if err := config.Start(out); err != nil {
				os.Exit(1)
			}
		},
	}
	config.Bind(cmd.Flags())
	return cmd
}

// ClientJoinConfig is the configuration for the client join command
type ClientJoinConfig struct {
	CommonStartConfig

	In     io.Reader
	Secret string
}

func (config *ClientJoinConfig) Bind(flags *pflag.FlagSet) {
	config.CommonStartConfig.Bind(flags)
	flags.StringVar(&config.Secret, "secret", "", "Pass the secret to use to connect to the cluster")
}

// Complete initializes fields based on command parameters and execution environment
func (c *ClientJoinConfig) Complete(f *osclientcmd.Factory, cmd *cobra.Command) error {
	if len(c.Secret) == 0 && c.In != nil {
		fmt.Fprintf(c.Out, "Please paste the contents of your secret here and hit ENTER:\n")
		data, err := ioutil.ReadAll(c.In)
		if err != nil {
			return err
		}
		c.Secret = string(data)
	}

	if err := c.CommonStartConfig.Complete(f, cmd); err != nil {
		return err
	}

	// Create an OpenShift configuration and start a container that uses it.
	c.addTask("Joining OpenShift cluster", c.StartOpenShiftNode)

	return nil
}

// StartOpenShiftNode starts the OpenShift container as a node
func (c *ClientJoinConfig) StartOpenShiftNode(out io.Writer) error {
	opt := &openshift.StartOptions{
		ServerIP:           c.ServerIP,
		UseSharedVolume:    !c.UseNsenterMount,
		Images:             c.imageFormat(),
		HostVolumesDir:     c.HostVolumesDir,
		HostConfigDir:      c.HostConfigDir,
		HostDataDir:        c.HostDataDir,
		UseExistingConfig:  c.UseExistingConfig,
		Environment:        c.Environment,
		LogLevel:           c.ServerLogLevel,
		DNSPort:            c.DNSPort,
		KubeconfigContents: c.Secret,
	}
	return c.OpenShiftHelper().StartNode(opt, out)
}
