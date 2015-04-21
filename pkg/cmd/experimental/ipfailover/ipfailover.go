package ipfailover

import (
	"fmt"
	"io"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	ipf "github.com/openshift/origin/pkg/ipfailover"
	"github.com/openshift/origin/pkg/ipfailover/keepalived"
)

const shortDesc = "Configure or view IP Failover configuration"
const description = `
Configure or view IP Failover configuration

This command helps to setup IP Failover configuration for an OpenShift
environment. An administrator can configure IP failover on an entire
cluster or as would normally be the case on a subset of nodes (as defined
via a labelled selector).

If an IP failover configuration does not exist with the given name,
the --create flag can be passed to create a deployment configuration and
service that will provide IP failover capability. If you are running in
production, it is recommended that the labelled selector for the nodes
matches atleast 2 nodes to ensure you have failover protection and that
you provide a --replicas=<n> value that matches the number of nodes for
the given labelled selector.


Examples:
  Check the default IP failover configuration ("ipfailover"):

  $ %[1]s %[2]s

  See what the IP failover configuration would look like if it is created:

  $ %[1]s %[2]s -o json

  Create an IP failover configuration if it does not already exist:

  $ %[1]s %[2]s ipf --virtual-ips="10.1.1.1-4" --create

  Create an IP failover configuration on a selection of nodes labelled
  "router=us-west-ha" (on 4 nodes with 7 virtual IPs monitoring a service
  listening on port 80 (aka the OpenShift router process).

  $ %[1]s %[2]s ipfailover --selector="router=us-west-ha" --virtual-ips="1.2.3.4,10.1.1.100-104,5.6.7.8" --watch-port=80 --replicas=4 --create

  Use a different IP failover config image and see the configuration:

  $ %[1]s %[2]s ipf-alt --selector="jack=the-vipper" --virtual-ips="1.2.3.4" -o yaml --images=myrepo/myipfailover:mytag

ALPHA: This command is currently being actively developed. It is intended
       to simplify the administrative tasks of setting up a highly
       available failover configuration.
`

func NewCmdIPFailoverConfig(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	options := &ipf.IPFailoverConfigCmdOptions{
		ImageTemplate:    variable.NewDefaultImageTemplate(),
		Selector:         ipf.DefaultSelector,
		ServicePort:      ipf.DefaultServicePort,
		WatchPort:        ipf.DefaultWatchPort,
		NetworkInterface: ipf.DefaultInterface,
		Replicas:         1,
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [<name>]", name),
		Short: shortDesc,
		Long:  fmt.Sprintf(description, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			processCommand(f, options, cmd, args, out)
		},
	}

	cmd.Flags().StringVar(&options.Type, "type", ipf.DefaultType, "The type of IP failover configurator to use.")
	cmd.Flags().StringVar(&options.ImageTemplate.Format, "images", options.ImageTemplate.Format, "The image to base this IP failover configurator on - ${component} will be replaced based on --type.")
	cmd.Flags().BoolVar(&options.ImageTemplate.Latest, "latest-images", options.ImageTemplate.Latest, "If true, attempt to use the latest images instead of the current release")
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter nodes on.")
	cmd.Flags().StringVar(&options.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the router should use to contact the master.")

	cmd.Flags().BoolVar(&options.Create, "create", options.Create, "Create the configuration if it does not exist.")

	cmd.Flags().StringVar(&options.VirtualIPs, "virtual-ips", "", "A set of virtual IP ranges and/or addresses that the routers bind and serve on and provide IP failover capability for.")
	cmd.Flags().StringVarP(&options.NetworkInterface, "interface", "i", "", "Network interface bound by VRRP to use for the set of virtual IP ranges/addresses specified.")

	// unicastHelp := `Send VRRP adverts using unicast instead of over the VRRP multicast group. This is useful in environments where multicast is not supported. Use with caution as this can get slow if the list of peers is large - it is recommended running this with the label option to select a set of nodes.`
	// cmd.Flags().StringVarP(&options.UseUnicast, "unicast", "u", options.UseUnicast, unicastHelp)

	cmd.Flags().IntVarP(&options.WatchPort, "watch-port", "w", ipf.DefaultWatchPort, "Port to monitor or watch for resource availability.")
	cmd.Flags().IntVarP(&options.Replicas, "replicas", "r", options.Replicas, "The replication factor of this IP failover configuration; commonly 2 when high availability is desired.")

	cmdutil.AddPrinterFlags(cmd)
	return cmd
}

func getConfigurationName(args []string) string {
	name := ipf.DefaultName

	switch len(args) {
	case 0:
		// Do nothing - use default name.
	case 1:
		name = args[0]
	default:
		glog.Fatalf("Please pass zero or one arguments to provide a name for this configuration.")
	}

	return name
}

func getConfigurator(name string, f *clientcmd.Factory, options *ipf.IPFailoverConfigCmdOptions, out io.Writer) *ipf.Configurator {
	//  Currently, the only supported plugin is keepalived (default).
	plugin, err := keepalived.NewIPFailoverConfiguratorPlugin(name, f, options)

	switch options.Type {
	case ipf.DefaultType:
		//  Default.
	// case <new-type>:  plugin, err = makeNewTypePlugin()
	default:
		glog.Fatalf("No plugins available to handle type %q", options.Type)
	}

	if err != nil {
		glog.Fatalf("IPFailoverConfigurator %q plugin error: %v", options.Type, err)
	}

	return ipf.NewConfigurator(name, plugin, out)
}

func previewConfiguration(c *ipf.Configurator, cmd *cobra.Command, out io.Writer) bool {
	p, output, err := cmdutil.PrinterForCommand(cmd)
	if err != nil {
		glog.Fatalf("Error configuring printer: %v", err)
	}

	// Check if we are outputting info.
	if !output {
		return false
	}

	if err := p.PrintObj(c.Generate(), out); err != nil {
		glog.Fatalf("Unable to print object: %v", err)
	}

	return true
}

func processCommand(f *clientcmd.Factory, options *ipf.IPFailoverConfigCmdOptions, cmd *cobra.Command, args []string, out io.Writer) {
	name := getConfigurationName(args)
	c := getConfigurator(name, f, options, out)

	//  First up, validate all the command line options.
	if err := ipf.ValidateCmdOptions(options, c); err != nil {
		glog.Fatal(err)
	}

	//  Check if we are just previewing the config.
	if previewConfiguration(c, cmd, out) {
		return
	}

	if options.Create {
		c.Create()
		return
	}
}
