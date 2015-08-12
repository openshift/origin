package ipfailover

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/ipfailover"
	"github.com/openshift/origin/pkg/ipfailover/keepalived"
)

const (
	ipFailover_long = `Configure or view IP Failover configuration

This command helps to setup an IP failover configuration for the
cluster. An administrator can configure IP failover on an entire
cluster or on a subset of nodes (as defined via a labeled selector).

If an IP failover configuration does not exist with the given name,
the --create flag can be passed to create a deployment configuration that
will provide IP failover capability. If you are running in production, it is
recommended that the labeled selector for the nodes matches at least 2 nodes
to ensure you have failover protection, and that you provide a --replicas=<n>
value that matches the number of nodes for the given labeled selector.`

	ipFailover_example = `  // Check the default IP failover configuration ("ipfailover"):
  $ %[1]s %[2]s

  // See what the IP failover configuration would look like if it is created:
  $ %[1]s %[2]s -o json

  // Create an IP failover configuration if it does not already exist:
  $ %[1]s %[2]s ipf --virtual-ips="10.1.1.1-4" --create

  // Create an IP failover configuration on a selection of nodes labeled
  // "router=us-west-ha" (on 4 nodes with 7 virtual IPs monitoring a service
  // listening on port 80, such as the router process).
  $ %[1]s %[2]s ipfailover --selector="router=us-west-ha" --virtual-ips="1.2.3.4,10.1.1.100-104,5.6.7.8" --watch-port=80 --replicas=4 --create

  // Use a different IP failover config image and see the configuration:
  $ %[1]s %[2]s ipf-alt --selector="hagroup=us-west-ha" --virtual-ips="1.2.3.4" -o yaml --images=myrepo/myipfailover:mytag`
)

func NewCmdIPFailoverConfig(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	options := &ipfailover.IPFailoverConfigCmdOptions{
		ImageTemplate:    variable.NewDefaultImageTemplate(),
		Selector:         ipfailover.DefaultSelector,
		ServicePort:      ipfailover.DefaultServicePort,
		WatchPort:        ipfailover.DefaultWatchPort,
		NetworkInterface: ipfailover.DefaultInterface,
		Replicas:         1,
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Install an IP failover group to a set of nodes",
		Long:    ipFailover_long,
		Example: fmt.Sprintf(ipFailover_example, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := processCommand(f, options, cmd, args, out)
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringVar(&options.Type, "type", ipfailover.DefaultType, "The type of IP failover configurator to use.")
	cmd.Flags().StringVar(&options.ImageTemplate.Format, "images", options.ImageTemplate.Format, "The image to base this IP failover configurator on - ${component} will be replaced based on --type.")
	cmd.Flags().BoolVar(&options.ImageTemplate.Latest, "latest-images", options.ImageTemplate.Latest, "If true, attempt to use the latest images instead of the current release")
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter nodes on.")
	cmd.Flags().StringVar(&options.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the router should use to contact the master.")
	cmd.Flags().StringVar(&options.ServiceAccount, "service-account", options.ServiceAccount, "Name of the service account to use to run the ipfailover pod.")

	cmd.Flags().BoolVar(&options.Create, "create", options.Create, "Create the configuration if it does not exist.")

	cmd.Flags().StringVar(&options.VirtualIPs, "virtual-ips", "", "A set of virtual IP ranges and/or addresses that the routers bind and serve on and provide IP failover capability for.")
	cmd.Flags().StringVarP(&options.NetworkInterface, "interface", "i", "", "Network interface bound by VRRP to use for the set of virtual IP ranges/addresses specified.")

	cmd.Flags().IntVarP(&options.WatchPort, "watch-port", "w", ipfailover.DefaultWatchPort, "Port to monitor or watch for resource availability.")
	cmd.Flags().IntVarP(&options.Replicas, "replicas", "r", options.Replicas, "The replication factor of this IP failover configuration; commonly 2 when high availability is desired. Please ensure this matches the number of nodes that satisfy the selector (or default selector) specified.")

	// autocompletion hints
	cmd.MarkFlagFilename("credentials", "kubeconfig")

	cmdutil.AddPrinterFlags(cmd)
	return cmd
}

//  Get configuration name - argv[1].
func getConfigurationName(args []string) (string, error) {
	name := ipfailover.DefaultName

	switch len(args) {
	case 0:
		// Do nothing - use default name.
	case 1:
		name = args[0]
	default:
		return "", fmt.Errorf("Please pass zero or one arguments to provide a name for this configuration.")
	}

	return name, nil
}

//  Get the configurator based on the ipfailover type.
func getConfigurator(name string, f *clientcmd.Factory, options *ipfailover.IPFailoverConfigCmdOptions, out io.Writer) (*ipfailover.Configurator, error) {
	//  Currently, the only supported plugin is keepalived (default).
	plugin, err := keepalived.NewIPFailoverConfiguratorPlugin(name, f, options)

	switch options.Type {
	case ipfailover.DefaultType:
		//  Default.
	// case <new-type>:  plugin, err = makeNewTypePlugin()
	default:
		return nil, fmt.Errorf("No plugins available to handle type %q", options.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("IPFailoverConfigurator %q plugin error: %v", options.Type, err)
	}

	return ipfailover.NewConfigurator(name, plugin, out), nil
}

//  Preview the configuration if required - returns true|false and errors.
func previewConfiguration(c *ipfailover.Configurator, cmd *cobra.Command, out io.Writer) (bool, error) {
	p, output, err := cmdutil.PrinterForCommand(cmd)
	if err != nil {
		return true, fmt.Errorf("Error configuring printer: %v", err)
	}

	// Check if we are outputting info.
	if !output {
		return false, nil
	}

	configList, err := c.Generate()
	if err != nil {
		return true, fmt.Errorf("Error generating config: %v", err)
	}

	if err := p.PrintObj(configList, out); err != nil {
		return true, fmt.Errorf("Unable to print object: %v", err)
	}

	return true, nil
}

//  Process the ipfailover command.
func processCommand(f *clientcmd.Factory, options *ipfailover.IPFailoverConfigCmdOptions, cmd *cobra.Command, args []string, out io.Writer) error {
	name, err := getConfigurationName(args)
	if err != nil {
		return err
	}

	c, err := getConfigurator(name, f, options, out)
	if err != nil {
		return err
	}

	//  First up, validate all the command line options.
	if err := ipfailover.ValidateCmdOptions(options, c); err != nil {
		return err
	}

	//  Check if we are just previewing the config.
	previewFlag, err := previewConfiguration(c, cmd, out)
	if previewFlag {
		return err
	}

	if options.Create {
		return c.Create()
	}

	return nil
}
