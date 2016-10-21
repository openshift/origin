package ipfailover

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	"github.com/openshift/origin/pkg/ipfailover"
	"github.com/openshift/origin/pkg/ipfailover/keepalived"
)

var (
	ipFailover_long = templates.LongDesc(`
		Configure or view IP Failover configuration

		This command helps to setup an IP failover configuration for the
		cluster. An administrator can configure IP failover on an entire
		cluster or on a subset of nodes (as defined via a labeled selector).

		If an IP failover configuration does not exist with the given name,
		the --create flag can be passed to create a deployment configuration that
		will provide IP failover capability. If you are running in production, it is
		recommended that the labeled selector for the nodes matches at least 2 nodes
		to ensure you have failover protection, and that you provide a --replicas=<n>
		value that matches the number of nodes for the given labeled selector.`)

	ipFailover_example = templates.Examples(`
		# Check the default IP failover configuration ("ipfailover"):
	  %[1]s %[2]s

	  # See what the IP failover configuration would look like if it is created:
	  %[1]s %[2]s -o json

	  # Create an IP failover configuration if it does not already exist:
	  %[1]s %[2]s ipf --virtual-ips="10.1.1.1-4" --create

	  # Create an IP failover configuration on a selection of nodes labeled
	  # "router=us-west-ha" (on 4 nodes with 7 virtual IPs monitoring a service
	  # listening on port 80, such as the router process).
	  %[1]s %[2]s ipfailover --selector="router=us-west-ha" --virtual-ips="1.2.3.4,10.1.1.100-104,5.6.7.8" --watch-port=80 --replicas=4 --create

	  # Use a different IP failover config image and see the configuration:
	  %[1]s %[2]s ipf-alt --selector="hagroup=us-west-ha" --virtual-ips="1.2.3.4" -o yaml --images=myrepo/myipfailover:mytag`)
)

func NewCmdIPFailoverConfig(f *clientcmd.Factory, parentName, name string, out, errout io.Writer) *cobra.Command {
	options := &ipfailover.IPFailoverConfigCmdOptions{
		Action: configcmd.BulkAction{
			Out:    out,
			ErrOut: errout,
		},
		ImageTemplate:    variable.NewDefaultImageTemplate(),
		ServiceAccount:   "ipfailover",
		Selector:         ipfailover.DefaultSelector,
		ServicePort:      ipfailover.DefaultServicePort,
		WatchPort:        ipfailover.DefaultWatchPort,
		NetworkInterface: ipfailover.DefaultInterface,
		VRRPIDOffset:     0,
		Replicas:         1,
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Install an IP failover group to a set of nodes",
		Long:    ipFailover_long,
		Example: fmt.Sprintf(ipFailover_example, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := Run(f, options, cmd, args)
			if err == cmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
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
	cmd.Flags().StringVar(&options.IptablesChain, "iptables-chain", ipfailover.DefaultIptablesChain, "Add a rule to this iptables chain to accept 224.0.0.28 multicast packets if no rule exists. When iptables-chain is empty do not change iptables.")
	cmd.Flags().StringVarP(&options.NetworkInterface, "interface", "i", "", "Network interface bound by VRRP to use for the set of virtual IP ranges/addresses specified.")

	cmd.Flags().IntVarP(&options.WatchPort, "watch-port", "w", ipfailover.DefaultWatchPort, "Port to monitor or watch for resource availability.")
	cmd.Flags().IntVar(&options.VRRPIDOffset, "vrrp-id-offset", options.VRRPIDOffset, "Offset to use for setting ids of VRRP instances (default offset is 0). This allows multiple ipfailover instances to run within the same cluster.")
	cmd.Flags().Int32VarP(&options.Replicas, "replicas", "r", options.Replicas, "The replication factor of this IP failover configuration; commonly 2 when high availability is desired. Please ensure this matches the number of nodes that satisfy the selector (or default selector) specified.")

	// autocompletion hints
	cmd.MarkFlagFilename("credentials", "kubeconfig")
	cmd.Flags().MarkDeprecated("credentials", "use --service-account to specify the service account the ipfailover pod will use to make API calls")

	options.Action.BindForOutput(cmd.Flags())
	cmd.Flags().String("output-version", "", "The preferred API versions of the output objects")

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
func getPlugin(name string, f *clientcmd.Factory, options *ipfailover.IPFailoverConfigCmdOptions) (ipfailover.IPFailoverConfiguratorPlugin, error) {
	if options.Type == ipfailover.DefaultType {
		plugin, err := keepalived.NewIPFailoverConfiguratorPlugin(name, f, options)
		if err != nil {
			return nil, fmt.Errorf("IPFailoverConfigurator %q plugin error: %v", options.Type, err)
		}

		return plugin, nil
	}

	return nil, fmt.Errorf("No plugins available to handle type %q", options.Type)
}

// Run runs the ipfailover command.
func Run(f *clientcmd.Factory, options *ipfailover.IPFailoverConfigCmdOptions, cmd *cobra.Command, args []string) error {
	name, err := getConfigurationName(args)
	if err != nil {
		return err
	}

	if len(options.ServiceAccount) == 0 {
		return fmt.Errorf("you must specify a service account for the ipfailover pod with --service-account, it cannot be blank")
	}

	options.Action.Bulk.Mapper = clientcmd.ResourceMapper(f)
	options.Action.Bulk.Op = configcmd.Create

	if err := ipfailover.ValidateCmdOptions(options); err != nil {
		return err
	}

	p, err := getPlugin(name, f, options)
	if err != nil {
		return err
	}

	list, err := p.Generate()
	if err != nil {
		return err
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	_, kClient, err := f.Clients()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}
	if err := validateServiceAccount(kClient, namespace, options.ServiceAccount); err != nil {
		return fmt.Errorf("ipfailover could not be created; %v", err)
	}

	configList := []runtime.Object{
		&kapi.ServiceAccount{ObjectMeta: kapi.ObjectMeta{Name: options.ServiceAccount}},
	}

	list.Items = append(configList, list.Items...)

	if options.Action.ShouldPrint() {
		mapper, _ := f.Object(false)
		return cmdutil.VersionedPrintObject(f.PrintObject, cmd, mapper, options.Action.Out)(list)
	}

	if errs := options.Action.WithMessage(fmt.Sprintf("Creating IP failover %s", name), "created").Run(list, namespace); len(errs) > 0 {
		return cmdutil.ErrExit
	}
	return nil
}

func validateServiceAccount(client *kclient.Client, ns string, serviceAccount string) error {

	sccList, err := client.SecurityContextConstraints().List(kapi.ListOptions{})
	if err != nil {
		if !errors.IsUnauthorized(err) {
			return fmt.Errorf("could not retrieve list of security constraints to verify service account %q: %v", serviceAccount, err)
		}
	}

	for _, scc := range sccList.Items {
		if scc.AllowPrivilegedContainer {
			for _, user := range scc.Users {
				if strings.Contains(user, serviceAccount) {
					return nil
				}
			}
		}
	}
	errMsg := "service account %q does not have sufficient privileges, grant access with oadm policy add-scc-to-user %s -z %s"
	return fmt.Errorf(errMsg, serviceAccount, bootstrappolicy.SecurityContextConstraintPrivileged, serviceAccount)
}
