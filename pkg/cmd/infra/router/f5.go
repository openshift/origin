package router

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	clientcmd "github.com/openshift/origin/pkg/client/cmd"
	"github.com/openshift/origin/pkg/cmd/util"
	cmdversion "github.com/openshift/origin/pkg/cmd/version"
	projectinternalclientset "github.com/openshift/origin/pkg/project/generated/internalclientset"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeinternalclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
	f5plugin "github.com/openshift/origin/pkg/router/f5"
	"github.com/openshift/origin/pkg/util/writerlease"
	"github.com/openshift/origin/pkg/version"
)

var (
	f5Long = templates.LongDesc(`
		Start an F5 route synchronizer

		This command launches a process that will synchronize an F5 to the route configuration of your master.

		You may restrict the set of routes exposed to a single project (with --namespace), projects your client has
		access to with a set of labels (--project-labels), namespaces matching a label (--namespace-labels), or all
		namespaces (no argument). You can limit the routes to those matching a --labels or --fields selector. Note
		that you must have a cluster-wide administrative role to view all namespaces.`)
)

// F5RouterOptions represent the complete structure needed to start an F5 router
// sync process.
type F5RouterOptions struct {
	Config *clientcmd.Config

	F5Router
	RouterSelection
}

// F5Router is the config necessary to start an F5 router plugin.
type F5Router struct {
	// Host specifies the hostname or IP address of the F5 BIG-IP host.
	Host string

	// Username specifies the username with which the plugin should authenticate
	// with the F5 BIG-IP host.
	Username string

	// Password specifies the password with which the plugin should authenticate
	// with the F5 BIG-IP host.
	Password string

	// HttpVserver specifies the name of the vserver object in F5 BIG-IP that the
	// plugin will configure for HTTP connections.
	HttpVserver string

	// HttpsVserver specifies the name of the vserver object in F5 BIG-IP that the
	// plugin will configure for HTTPS connections.
	HttpsVserver string

	// PrivateKey specifies the filename of an SSH private key for
	// authenticating with F5.  This key is required to copy certificates
	// to the F5 BIG-IP host.
	PrivateKey string

	// Insecure specifies whether the F5 plugin should perform strict certificate
	// validation for connections to the F5 BIG-IP host.
	Insecure bool

	// PartitionPath specifies the path to the F5 partition. This is
	// normally used to create access control boundaries for users
	// and applications.
	PartitionPath string

	// VxlanGateway is the ip address assigned to the local tunnel interface
	// inside F5 box. This address is the one that the packets generated from F5
	// will carry. The pods will return the packets to this address itself.
	// It is important that the gateway be one of the ip addresses of the subnet
	// that has been generated for F5.
	VxlanGateway string

	// InternalAddress is the ip address of the vtep interface used to connect to
	// VxLAN overlay. It is the hostIP address listed in the subnet generated for F5
	InternalAddress string
}

// Bind binds F5Router arguments to flags
func (o *F5Router) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.Host, "f5-host", util.Env("ROUTER_EXTERNAL_HOST_HOSTNAME", ""), "The host of F5 BIG-IP's management interface")
	flag.StringVar(&o.Username, "f5-username", util.Env("ROUTER_EXTERNAL_HOST_USERNAME", ""), "The username for F5 BIG-IP's management utility")
	flag.StringVar(&o.Password, "f5-password", util.Env("ROUTER_EXTERNAL_HOST_PASSWORD", ""), "The password for F5 BIG-IP's management utility")
	flag.StringVar(&o.HttpVserver, "f5-http-vserver", util.Env("ROUTER_EXTERNAL_HOST_HTTP_VSERVER", "ose-vserver"), "The F5 BIG-IP virtual server for HTTP connections")
	flag.StringVar(&o.HttpsVserver, "f5-https-vserver", util.Env("ROUTER_EXTERNAL_HOST_HTTPS_VSERVER", "https-ose-vserver"), "The F5 BIG-IP virtual server for HTTPS connections")
	flag.StringVar(&o.PrivateKey, "f5-private-key", util.Env("ROUTER_EXTERNAL_HOST_PRIVKEY", ""), "The path to the F5 BIG-IP SSH private key file")
	flag.BoolVar(&o.Insecure, "f5-insecure", isTrue(util.Env("ROUTER_EXTERNAL_HOST_INSECURE", "")), "Skip strict certificate verification")
	flag.StringVar(&o.PartitionPath, "f5-partition-path", util.Env("ROUTER_EXTERNAL_HOST_PARTITION_PATH", f5plugin.F5DefaultPartitionPath), "The F5 BIG-IP partition path to use")
	flag.StringVar(&o.InternalAddress, "f5-internal-address", util.Env("ROUTER_EXTERNAL_HOST_INTERNAL_ADDRESS", ""), "The F5 BIG-IP internal interface's IP address")
	flag.StringVar(&o.VxlanGateway, "f5-vxlan-gateway-cidr", util.Env("ROUTER_EXTERNAL_HOST_VXLAN_GW_CIDR", ""), "The F5 BIG-IP gateway-ip-address/cidr-mask for setting up the VxLAN")
}

// Validate verifies the required F5 flags are present
func (o *F5Router) Validate() error {
	if o.Host == "" {
		return errors.New("F5 host must be specified")
	}

	if o.Username == "" {
		return errors.New("F5 username must be specified")
	}

	if o.Password == "" {
		return errors.New("F5 password must be specified")
	}

	if len(o.HttpVserver) == 0 && len(o.HttpsVserver) == 0 {
		return errors.New("F5 HTTP and HTTPS vservers cannot both be blank")
	}

	valid := (len(o.VxlanGateway) == 0 && len(o.InternalAddress) == 0) || (len(o.VxlanGateway) != 0 && len(o.InternalAddress) != 0)
	if !valid {
		return errors.New("For VxLAN setup, both internal-address and gateway-cidr must be specified")
	}

	return nil
}

// NewCommandF5Router provides CLI handler for the F5 router sync plugin.
func NewCommandF5Router(name string) *cobra.Command {
	options := &F5RouterOptions{
		Config: clientcmd.NewConfig(),
	}
	options.Config.FromFile = true

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start an F5 route synchronizer",
		Long:  f5Long,
		Run: func(c *cobra.Command, args []string) {
			options.RouterSelection.Namespace = cmdutil.GetFlagString(c, "namespace")
			cmdutil.CheckErr(options.Complete())
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.Run())
		},
	}

	cmd.AddCommand(cmdversion.NewCmdVersion(name, version.Get(), os.Stdout))

	flag := cmd.Flags()
	options.Config.Bind(flag)
	options.F5Router.Bind(flag)
	options.RouterSelection.Bind(flag)

	return cmd
}

func (o *F5RouterOptions) Complete() error {
	if len(o.PartitionPath) == 0 {
		o.PartitionPath = f5plugin.F5DefaultPartitionPath
		glog.Warningf("Partition path was empty, using default: %q",
			f5plugin.F5DefaultPartitionPath)
	}

	return o.RouterSelection.Complete()
}

func (o *F5RouterOptions) Validate() error {
	return o.F5Router.Validate()
}

// F5RouteAdmitterFunc returns a func that checks if a route is a
// wildcard route and currently denies it.
func (o *F5RouterOptions) F5RouteAdmitterFunc() controller.RouteAdmissionFunc {
	return func(route *routeapi.Route) error {
		if err := o.AdmissionCheck(route); err != nil {
			return err
		}

		switch route.Spec.WildcardPolicy {
		case routeapi.WildcardPolicyNone:
			return nil

		case routeapi.WildcardPolicySubdomain:
			// TODO: F5 wildcard route support.
			return fmt.Errorf("Wildcard routes are currently not supported by the F5 router")
		}

		return fmt.Errorf("unknown wildcard policy %v", route.Spec.WildcardPolicy)
	}
}

// Run launches an F5 route sync process using the provided options. It never exits.
func (o *F5RouterOptions) Run() error {
	cfg := f5plugin.F5PluginConfig{
		Host:            o.Host,
		Username:        o.Username,
		Password:        o.Password,
		HttpVserver:     o.HttpVserver,
		HttpsVserver:    o.HttpsVserver,
		PrivateKey:      o.PrivateKey,
		Insecure:        o.Insecure,
		PartitionPath:   o.PartitionPath,
		InternalAddress: o.InternalAddress,
		VxlanGateway:    o.VxlanGateway,
	}
	f5Plugin, err := f5plugin.NewF5Plugin(cfg)
	if err != nil {
		return err
	}

	kc, err := o.Config.Clients()
	if err != nil {
		return err
	}
	routeclient, err := routeinternalclientset.NewForConfig(o.Config.OpenShiftConfig())
	if err != nil {
		return err
	}
	projectclient, err := projectinternalclientset.NewForConfig(o.Config.OpenShiftConfig())
	if err != nil {
		return err
	}

	var plugin router.Plugin = f5Plugin
	var recorder controller.RejectionRecorder = controller.LogRejections
	if o.UpdateStatus {
		lease := writerlease.New(time.Minute, 3*time.Second)
		go lease.Run(wait.NeverStop)
		tracker := controller.NewSimpleContentionTracker(o.ResyncInterval / 10)
		tracker.SetConflictMessage(fmt.Sprintf("The router detected another process is writing conflicting updates to route status with name %q. Please ensure that the configuration of all routers is consistent. Route status will not be updated as long as conflicts are detected.", o.RouterName))
		go tracker.Run(wait.NeverStop)
		status := controller.NewStatusAdmitter(plugin, routeclient.Route(), o.RouterName, o.RouterCanonicalHostname, lease, tracker)
		recorder = status
		plugin = status
	}
	if o.ExtendedValidation {
		plugin = controller.NewExtendedValidator(plugin, recorder)
	}
	plugin = controller.NewUniqueHost(plugin, o.RouteSelectionFunc(), o.RouterSelection.DisableNamespaceOwnershipCheck, recorder)
	plugin = controller.NewHostAdmitter(plugin, o.F5RouteAdmitterFunc(), o.AllowWildcardRoutes, o.RouterSelection.DisableNamespaceOwnershipCheck, recorder)

	factory := o.RouterSelection.NewFactory(routeclient, projectclient.Project().Projects(), kc)
	watchNodes := (len(o.InternalAddress) != 0 && len(o.VxlanGateway) != 0)
	controller := factory.Create(plugin, watchNodes, o.EnableIngress)
	controller.Run()

	select {}
}
