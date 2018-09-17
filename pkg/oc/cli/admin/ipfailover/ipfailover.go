package ipfailover

import (
	"fmt"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	securityv1typedclient "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	"github.com/openshift/origin/pkg/oc/cli/admin/ipfailover/ipfailover"
	"github.com/openshift/origin/pkg/oc/cli/admin/ipfailover/keepalived"
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

type IPFailoverOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer printers.ResourcePrinter

	ConfigOptions *ipfailover.IPFailoverConfigOptions

	SecurityClient securityv1typedclient.SecurityV1Interface
	DynamicClient  dynamic.Interface

	DryRun     bool
	Namespace  string
	RESTMapper meta.RESTMapper

	genericclioptions.IOStreams
}

func NewIPFailoverOptions(streams genericclioptions.IOStreams) *IPFailoverOptions {
	return &IPFailoverOptions{
		ConfigOptions: ipfailover.NewIPFailoverConfigOptions(),
		PrintFlags:    genericclioptions.NewPrintFlags("created").WithTypeSetter(scheme.Scheme),
		IOStreams:     streams,
	}
}

func NewCmdIPFailoverConfig(f kcmdutil.Factory, parentName, name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewIPFailoverOptions(streams)

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Install an IP failover group to a set of nodes",
		Long:    ipFailover_long,
		Example: fmt.Sprintf(ipFailover_example, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.ConfigOptions.Type, "type", o.ConfigOptions.Type, "The type of IP failover configurator to use.")
	cmd.Flags().StringVar(&o.ConfigOptions.ImageTemplate.Format, "images", o.ConfigOptions.ImageTemplate.Format, "The image to base this IP failover configurator on - ${component} will be replaced based on --type.")
	cmd.Flags().BoolVar(&o.ConfigOptions.ImageTemplate.Latest, "latest-images", o.ConfigOptions.ImageTemplate.Latest, "If true, attempt to use the latest images instead of the current release")
	cmd.Flags().StringVarP(&o.ConfigOptions.Selector, "selector", "l", o.ConfigOptions.Selector, "Selector (label query) to filter nodes on.")
	cmd.Flags().StringVar(&o.ConfigOptions.ServiceAccount, "service-account", o.ConfigOptions.ServiceAccount, "Name of the service account to use to run the ipfailover pod.")

	cmd.Flags().BoolVar(&o.ConfigOptions.Create, "create", o.ConfigOptions.Create, "If true, create the configuration if it does not exist.")

	cmd.Flags().StringVar(&o.ConfigOptions.VirtualIPs, "virtual-ips", o.ConfigOptions.VirtualIPs, "A set of virtual IP ranges and/or addresses that the routers bind and serve on and provide IP failover capability for.")
	cmd.Flags().UintVar(&o.ConfigOptions.VIPGroups, "virtual-ip-groups", o.ConfigOptions.VIPGroups, "Number of groups to create for VRRP, if not set a group will be created for each virtual ip given on --virtual-ips.")
	cmd.Flags().StringVar(&o.ConfigOptions.NotifyScript, "notify-script", o.ConfigOptions.NotifyScript, "Run this script when state changes.")
	cmd.Flags().StringVar(&o.ConfigOptions.CheckScript, "check-script", o.ConfigOptions.CheckScript, "Run this script at the check-interval to verify service is OK")
	cmd.Flags().IntVar(&o.ConfigOptions.CheckInterval, "check-interval", o.ConfigOptions.CheckInterval, "Run the check-script at this interval (seconds)")
	cmd.Flags().StringVar(&o.ConfigOptions.Preemption, "preemption-strategy", o.ConfigOptions.Preemption, "Normlly VRRP will preempt a lower priority machine when a higher priority one comes online. 'nopreempt' allows the lower priority machine to maintain its MASTER status. The default 'preempt_delay 300' causes MASTER to switch after 5 min.")
	cmd.Flags().StringVar(&o.ConfigOptions.IptablesChain, "iptables-chain", o.ConfigOptions.IptablesChain, "Add a rule to this iptables chain to accept 224.0.0.28 multicast packets if no rule exists. When iptables-chain is empty do not change iptables.")
	cmd.Flags().StringVarP(&o.ConfigOptions.NetworkInterface, "interface", "i", o.ConfigOptions.NetworkInterface, "Network interface bound by VRRP to use for the set of virtual IP ranges/addresses specified.")

	cmd.Flags().IntVarP(&o.ConfigOptions.WatchPort, "watch-port", "w", o.ConfigOptions.WatchPort, "Port to monitor or watch for resource availability.")
	cmd.Flags().IntVar(&o.ConfigOptions.VRRPIDOffset, "vrrp-id-offset", o.ConfigOptions.VRRPIDOffset, "Offset to use for setting ids of VRRP instances (default offset is 0). This allows multiple ipfailover instances to run within the same cluster.")
	cmd.Flags().Int32VarP(&o.ConfigOptions.Replicas, "replicas", "r", o.ConfigOptions.Replicas, "The replication factor of this IP failover configuration; commonly 2 when high availability is desired. Please ensure this matches the number of nodes that satisfy the selector (or default selector) specified.")

	kcmdutil.AddDryRunFlag(cmd)
	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *IPFailoverOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	name, err := configurationName(args)
	if err != nil {
		return err
	}

	if o.ConfigOptions.VRRPIDOffset < 0 || o.ConfigOptions.VRRPIDOffset > 254 {
		return fmt.Errorf("The vrrp-id-offset must be in the range 0..254")
	}

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")

	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}

	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	// The ipfailover pods for a given configuration must run on different nodes.
	// We are using the ServicePort as a mechanism to prevent multiple pods for
	// same configuration starting on the same node. Since pods for different
	// configurations can run on the same node a different ServicePort is used
	// for each configuration.
	// In the future, this may be changed to pod anti-affinity.
	o.ConfigOptions.ServicePort = o.ConfigOptions.ServicePort + o.ConfigOptions.VRRPIDOffset

	o.RESTMapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.DynamicClient, err = dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.ConfigOptions.ConfiguratorPlugin, err = keepalived.NewIPFailoverConfiguratorPlugin(name, f, o.ConfigOptions)
	if err != nil {
		return fmt.Errorf("IPFailoverConfigurator %q plugin error: %v", o.ConfigOptions.Type, err)
	}

	o.SecurityClient, err = securityv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return nil
}

func (o *IPFailoverOptions) Validate() error {
	if err := validateVirtualIPs(o.ConfigOptions.VirtualIPs); err != nil {
		return err
	}

	if o.ConfigOptions.Type != ipfailover.DefaultType {
		return fmt.Errorf("no plugins available to handle type %q", o.ConfigOptions.Type)
	}

	if err := validateServiceAccount(o.SecurityClient, o.ConfigOptions.ServiceAccount); err != nil {
		return fmt.Errorf("ipfailover could not be created, invalid service account name: %v", err)
	}

	return nil
}

//  Get configuration name - argv[1].
func configurationName(args []string) (string, error) {
	name := ipfailover.DefaultName

	switch len(args) {
	case 0:
		// Do nothing - use default name.
	case 1:
		name = args[0]
	default:
		return "", fmt.Errorf("zero arguments or an argument containing a configuration name is required")
	}

	return name, nil
}

// Run runs the ipfailover command.
func (o *IPFailoverOptions) Run() error {
	items, err := o.ConfigOptions.ConfiguratorPlugin.Generate()
	if err != nil {
		return err
	}

	configList := []runtime.Object{
		&corev1.ServiceAccount{
			// this is ok because we know exactly how we want to be serialized
			TypeMeta:   metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String(), Kind: "ServiceAccount"},
			ObjectMeta: metav1.ObjectMeta{Name: o.ConfigOptions.ServiceAccount},
		},
	}

	items = append(configList, items...)

	// TODO: stop treating --output formats as --dry-run
	dryRun := o.DryRun || (o.PrintFlags.OutputFormat != nil && len(*o.PrintFlags.OutputFormat) > 0 && *o.PrintFlags.OutputFormat != "name")
	created, errs := o.createResources(items, dryRun)

	// print what we have created first, then return a potential set of errors
	if err := o.Printer.PrintObj(created, o.Out); err != nil {
		errs = append(errs, err)
	}

	return kerrors.NewAggregate(errs)
}

func (o *IPFailoverOptions) createResources(items []runtime.Object, dryRun bool) (*unstructured.UnstructuredList, []error) {
	errors := []error{}
	created := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"kind":       "List",
			"apiVersion": "v1",
			"metadata":   map[string]interface{}{},
		},
	}

	for _, item := range items {
		var err error
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(item)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		unstructuredObj := &unstructured.Unstructured{Object: obj}

		namespace := o.Namespace
		if len(unstructuredObj.GetNamespace()) > 0 {
			namespace = unstructuredObj.GetNamespace()
		}

		mapping, err := o.RESTMapper.RESTMapping(unstructuredObj.GroupVersionKind().GroupKind(), unstructuredObj.GroupVersionKind().Version)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		if mapping.Scope.Name() == meta.RESTScopeNameRoot {
			namespace = ""
		}

		if dryRun {
			created.Items = append(created.Items, *unstructuredObj)
			continue
		}

		if _, err := o.DynamicClient.Resource(mapping.Resource).Namespace(namespace).Create(unstructuredObj); err != nil {
			errors = append(errors, err)
			continue
		}

		created.Items = append(created.Items, *unstructuredObj)
	}

	return created, errors
}
