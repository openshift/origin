package haconfig

import (
	"fmt"
	"io"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	hac "github.com/openshift/origin/pkg/haconfig"
	"github.com/openshift/origin/plugins/haconfig/keepalived"
)

const shortDesc = "Configure or view High Availability configuration"
const description = `
Configure or view High Availability configuration

This command helps to setup High Availability (HA) configuration for an
OpenShift environment. An administrator can configure HA on an entire
cluster or as would normally be the case on a subset of nodes (as defined
via a labelled selector).
If no arguments are passed, this command will display the HA configuration
for a resource name 'ha-config'.

If a HA configuration does not exist with the given name, the --create flag
can be passed to create a deployment configuration and service that will
provide HA and failover capability. If you are running in production, it is
recommended that the labelled selector for the nodes matches atleast 2
nodes to ensure you have failover protection and that you provide a
--replicas=<n> value that matches the number of nodes for the given
labelled selector.


Examples:
  Check the default HA configuration ("ha-config"):

  $ %[1]s %[2]s

  See what the HA configuration would look like if it is created:

  $ %[1]s %[2]s -o json

  Create a HA configuration if it does not already exist:

  $ %[1]s %[2]s hac --virtual-ips="10.1.1.1-4" --create

  Create a HA configuration on a selection of nodes labelled
  "router=us-west-ha" (on 4 nodes with 7 virtual IPs monitoring a service
  listening on port 80 (aka the OpenShift router process).

  $ %[1]s %[2]s ha-config --selector="router=us-west-ha" --virtual-ips="1.2.3.4,10.1.1.100-104,5.6.7.8" --watch-port=80 --replicas=4 --create

  Delete a previously created HA configuration:

  $ %[1]s %[2]s hac --delete

  Use a different HA config image and see the configuration:

  $ %[1]s %[2]s ha-alt --selector="jack=the-vipper" --virtual-ips="1.2.3.4" -o yaml --images=myrepo/myhaconfig:mytag

ALPHA: This command is currently being actively developed. It is intended
       to simplify the administrative tasks of setting up a highly
       available failover configuration.
`

func NewCmdHAConfig(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	options := &hac.HAConfigCmdOptions{
		ImageTemplate:    variable.NewDefaultImageTemplate(),
		Selector:         hac.DefaultSelector,
		ServicePort:      hac.DefaultServicePort,
		WatchPort:        hac.DefaultWatchPort,
		NetworkInterface: hac.DefaultInterface,
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

	cmd.Flags().StringVar(&options.Type, "type", hac.DefaultType, "The type of HA configurator to use.")
	cmd.Flags().StringVar(&options.ImageTemplate.Format, "images", options.ImageTemplate.Format, "The image to base this HA configurator on - ${component} will be replaced based on --type.")
	cmd.Flags().BoolVar(&options.ImageTemplate.Latest, "latest-images", options.ImageTemplate.Latest, "If true, attempt to use the latest images instead of the current release")
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter nodes on.")
	cmd.Flags().StringVar(&options.Credentials, "credentials", "", "Path to a .kubeconfig file that will contain the credentials the router should use to contact the master.")

	cmd.Flags().BoolVar(&options.Create, "create", options.Create, "Create the configuration if it does not exist.")
	cmd.Flags().BoolVar(&options.Delete, "delete", options.Delete, "Delete the configuration if it exists.")

	cmd.Flags().StringVar(&options.VirtualIPs, "virtual-ips", "", "A set of virtual IP ranges and/or addresses that the routers bind and serve on and provide IP failover capability for.")
	cmd.Flags().StringVarP(&options.NetworkInterface, "interface", "i", "", "Network interface bound by VRRP to use for the set of virtual IP ranges/addresses specified.")

	// unicastHelp := `Send VRRP adverts using unicast instead of over the VRRP multicast group. This is useful in environments where multicast is not supported. Use with caution as this can get slow if the list of peers is large - it is recommended running this with the label option to select a set of nodes.`
	// cmd.Flags().StringVarP(&options.UseUnicast, "unicast", "u", options.UseUnicast, unicastHelp)

	cmd.Flags().IntVarP(&options.WatchPort, "watch-port", "w", hac.DefaultWatchPort, "Port to monitor or watch for resource availability.")
	cmd.Flags().IntVarP(&options.Replicas, "replicas", "r", options.Replicas, "The replication factor of the HA configuration; commonly 2 when high availability is desired.")

	cmdutil.AddPrinterFlags(cmd)
	return cmd
}

func getConfigurationName(args []string) string {
	name := hac.DefaultName

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

func getConfigurator(name string, f *clientcmd.Factory, options *hac.HAConfigCmdOptions, out io.Writer) *hac.Configurator {
	//  Currently, the only supported plugin is keepalived (default).
	plugin, err := keepalived.NewHAConfiguratorPlugin(name, f, options)

	switch options.Type {
	case hac.DefaultType:
		//  Default.
	// case <new-type>:  plugin, err = makeNewTypePlugin()
	default:
		glog.Fatalf("No plugins available to handle type %q", options.Type)
	}

	if err != nil {
		glog.Fatalf("HAConfigurator %q plugin error: %v", options.Type, err)
	}

	return hac.NewConfigurator(name, plugin, out)
}

func previewConfiguration(c *hac.Configurator, cmd *cobra.Command, out io.Writer) bool {
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

func processCommand(f *clientcmd.Factory, options *hac.HAConfigCmdOptions, cmd *cobra.Command, args []string, out io.Writer) {
	name := getConfigurationName(args)
	c := getConfigurator(name, f, options, out)

	//  First up, validate all the command line options.
	if err := hac.ValidateCmdOptions(options, c); err != nil {
		glog.Fatal(err)
	}

	//  Check if we are just previewing the config.
	if previewConfiguration(c, cmd, out) {
		return
	}

	if options.Create {
		c.Create()
		if options.Delete {
			glog.Warning("Superfluous --delete option was ignored.")
		}
		return
	}

	if options.Delete {
		c.Delete()
		return
	}
}
