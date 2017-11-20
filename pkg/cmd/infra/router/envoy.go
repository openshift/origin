package router

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	ktypes "k8s.io/apimachinery/pkg/types"
	kapi "k8s.io/kubernetes/pkg/api"
	corelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	cmdversion "github.com/openshift/origin/pkg/cmd/version"
	projectinternalclientset "github.com/openshift/origin/pkg/project/generated/internalclientset"
	routeinternalclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
	"github.com/openshift/origin/pkg/router/envoy"
	"github.com/openshift/origin/pkg/util/proc"
	"github.com/openshift/origin/pkg/version"
)

var envoyLong = templates.LongDesc(`
	Start a router

	This command launches a router connected to your cluster master. The router listens for routes and endpoints
	created by users and keeps a local router configuration up to date with those changes.

	You may customize the router by providing your own --template and --reload scripts.

	The router must have a default certificate in pem format. You may provide it via --default-cert otherwise
	one is automatically created.

	You may restrict the set of routes exposed to a single project (with --namespace), projects your client has
	access to with a set of labels (--project-labels), namespaces matching a label (--namespace-labels), or all
	namespaces (no argument). You can limit the routes to those matching a --labels or --fields selector. Note
	that you must have a cluster-wide administrative role to view all namespaces.`)

type EnvoyRouterOptions struct {
	Config *clientcmd.Config

	EnvoyRouter
	RouterSelection
}

type EnvoyRouter struct {
	DefaultDestinationCAPath string
	ADSListenAddr            string
	RouterService            *ktypes.NamespacedName
}

func (o *EnvoyRouter) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.DefaultDestinationCAPath, "default-destination-ca-path", util.Env("DEFAULT_DESTINATION_CA_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"), "A path to a PEM file containing the default CA bundle to use with re-encrypt routes. This CA should sign for certificates in the Kubernetes DNS space (service.namespace.svc).")
	flag.StringVar(&o.ADSListenAddr, "envoy-ads-listen-addr", util.Env("ENVOY_ADS_LISTEN_ADDR", "127.0.0.1:8888"), "The address and port to listen on for Envoy Aggregated Discovery Service requests.")
}

// NewCommndEnvoyRouter provides CLI handler for the envoy router backend
func NewCommandEnvoyRouter(name string) *cobra.Command {
	options := &EnvoyRouterOptions{
		Config: clientcmd.NewConfig(),
	}
	options.Config.FromFile = true

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start a router",
		Long:  routerLong,
		Run: func(c *cobra.Command, args []string) {
			options.RouterSelection.Namespace = cmdutil.GetFlagString(c, "namespace")
			// if the user did not specify a destination ca path, and the file does not exist, disable the default in order
			// to preserve backwards compatibility with older clusters
			if !c.Flags().Lookup("default-destination-ca-path").Changed && util.Env("DEFAULT_DESTINATION_CA_PATH", "") == "" {
				if _, err := os.Stat(options.EnvoyRouter.DefaultDestinationCAPath); err != nil {
					options.EnvoyRouter.DefaultDestinationCAPath = ""
				}
			}
			cmdutil.CheckErr(options.Complete())
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.Run())
		},
	}

	cmd.AddCommand(cmdversion.NewCmdVersion(name, version.Get(), os.Stdout))

	flag := cmd.Flags()
	options.Config.Bind(flag)
	options.EnvoyRouter.Bind(flag)
	options.RouterSelection.Bind(flag)

	return cmd
}

func (o *EnvoyRouterOptions) Complete() error {
	routerSvcName := util.Env("ROUTER_SERVICE_NAME", "")
	routerSvcNamespace := util.Env("ROUTER_SERVICE_NAMESPACE", "")
	if len(routerSvcName) > 0 {
		if len(routerSvcNamespace) == 0 {
			return fmt.Errorf("ROUTER_SERVICE_NAMESPACE is required when ROUTER_SERVICE_NAME is specified")
		}
		o.RouterService = &ktypes.NamespacedName{
			Namespace: routerSvcNamespace,
			Name:      routerSvcName,
		}
	}

	return o.RouterSelection.Complete()
}

func (o *EnvoyRouterOptions) Validate() error {
	if len(o.RouterName) == 0 && o.UpdateStatus {
		return errors.New("router must have a name to identify itself in route status")
	}
	if len(o.EnvoyRouter.DefaultDestinationCAPath) != 0 {
		if _, err := os.Stat(o.EnvoyRouter.DefaultDestinationCAPath); err != nil {
			return fmt.Errorf("unable to load default destination CA certificate: %v", err)
		}
	}
	return nil
}

// Run launches a template router using the provided options. It never exits.
func (o *EnvoyRouterOptions) Run() error {
	glog.Infof("Starting Envoy router (%s)", version.Get())

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

	envoyPlugin := envoy.NewPlugin()

	var recorder controller.RejectionRecorder = controller.LogRejections
	var plugin router.Plugin = envoyPlugin
	if o.UpdateStatus {
		status := controller.NewStatusAdmitter(plugin, routeclient.Route(), o.RouterName, o.RouterCanonicalHostname)
		recorder = status
		plugin = status
	}
	if o.ExtendedValidation {
		plugin = controller.NewExtendedValidator(plugin, recorder)
	}
	plugin = controller.NewUniqueHost(plugin, o.RouteSelectionFunc(), o.RouterSelection.DisableNamespaceOwnershipCheck, recorder)
	plugin = controller.NewHostAdmitter(plugin, o.RouteAdmissionFunc(), o.AllowWildcardRoutes, o.RouterSelection.DisableNamespaceOwnershipCheck, recorder)

	factory := o.RouterSelection.NewFactory(routeclient, projectclient.Project().Projects(), kc)
	controller := factory.Create(plugin, false, o.EnableIngress)
	controller.Run()

	proc.StartReaper()

	envoyPlugin.SetListers(corelisters.NewEndpointsLister(factory.InformerFor(&kapi.Endpoints{}).GetIndexer()))
	listener, err := net.Listen("tcp", o.ADSListenAddr)
	if err != nil {
		return err
	}
	go func() {
		if err := envoyPlugin.Serve(listener); err != nil {
			glog.Fatalf("Envoy server halted: %v")
		}
	}()

	select {}
}
