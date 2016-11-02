package router

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	ktypes "k8s.io/kubernetes/pkg/types"

	ocmd "github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
	templateplugin "github.com/openshift/origin/pkg/router/template"
	"github.com/openshift/origin/pkg/util/proc"
)

// defaultReloadInterval is how often to do reloads in seconds.
const defaultReloadInterval = 5

var routerLong = templates.LongDesc(`
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

type TemplateRouterOptions struct {
	Config *clientcmd.Config

	TemplateRouter
	RouterStats
	RouterSelection
}

type TemplateRouter struct {
	RouterName             string
	WorkingDir             string
	TemplateFile           string
	ReloadScript           string
	ReloadInterval         time.Duration
	DefaultCertificate     string
	DefaultCertificatePath string
	DefaultCertificateDir  string
	ExtendedValidation     bool
	RouterService          *ktypes.NamespacedName
}

// reloadInterval returns how often to run the router reloads. The interval
// value is based on an environment variable or the default.
func reloadInterval() time.Duration {
	interval := util.Env("RELOAD_INTERVAL", fmt.Sprintf("%vs", defaultReloadInterval))
	value, err := time.ParseDuration(interval)
	if err != nil {
		glog.Warningf("Invalid RELOAD_INTERVAL %q, using default value %v ...", interval, defaultReloadInterval)
		value = time.Duration(defaultReloadInterval * time.Second)
	}
	return value
}

func (o *TemplateRouter) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.RouterName, "name", util.Env("ROUTER_SERVICE_NAME", "public"), "The name the router will identify itself with in the route status")
	flag.StringVar(&o.WorkingDir, "working-dir", "/var/lib/haproxy/router", "The working directory for the router plugin")
	flag.StringVar(&o.DefaultCertificate, "default-certificate", util.Env("DEFAULT_CERTIFICATE", ""), "The contents of a default certificate to use for routes that don't expose a TLS server cert; in PEM format")
	flag.StringVar(&o.DefaultCertificatePath, "default-certificate-path", util.Env("DEFAULT_CERTIFICATE_PATH", ""), "A path to default certificate to use for routes that don't expose a TLS server cert; in PEM format")
	flag.StringVar(&o.DefaultCertificateDir, "default-certificate-dir", util.Env("DEFAULT_CERTIFICATE_DIR", ""), "A path to a directory that contains a file named tls.crt. If tls.crt is not a PEM file which also contains a private key, it is first combined with a file named tls.key in the same directory. The PEM-format contents are then used as the default certificate. Only used if default-certificate and default-certificate-path are not specified.")
	flag.StringVar(&o.TemplateFile, "template", util.Env("TEMPLATE_FILE", ""), "The path to the template file to use")
	flag.StringVar(&o.ReloadScript, "reload", util.Env("RELOAD_SCRIPT", ""), "The path to the reload script to use")
	flag.DurationVar(&o.ReloadInterval, "interval", reloadInterval(), "Controls how often router reloads are invoked. Mutiple router reload requests are coalesced for the duration of this interval since the last reload time.")
	flag.BoolVar(&o.ExtendedValidation, "extended-validation", util.Env("EXTENDED_VALIDATION", "true") == "true", "If set, then an additional extended validation step is performed on all routes admitted in by this router. Defaults to true and enables the extended validation checks.")
}

type RouterStats struct {
	StatsPortString string
	StatsPassword   string
	StatsUsername   string

	StatsPort int
}

func (o *RouterStats) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.StatsPortString, "stats-port", util.Env("STATS_PORT", ""), "If the underlying router implementation can provide statistics this is a hint to expose it on this port.")
	flag.StringVar(&o.StatsPassword, "stats-password", util.Env("STATS_PASSWORD", ""), "If the underlying router implementation can provide statistics this is the requested password for auth.")
	flag.StringVar(&o.StatsUsername, "stats-user", util.Env("STATS_USERNAME", ""), "If the underlying router implementation can provide statistics this is the requested username for auth.")
}

// NewCommndTemplateRouter provides CLI handler for the template router backend
func NewCommandTemplateRouter(name string) *cobra.Command {
	options := &TemplateRouterOptions{
		Config: clientcmd.NewConfig(),
	}
	options.Config.FromFile = true

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start a router",
		Long:  routerLong,
		Run: func(c *cobra.Command, args []string) {
			options.RouterSelection.Namespace = cmdutil.GetFlagString(c, "namespace")
			cmdutil.CheckErr(options.Complete())
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.Run())
		},
	}

	cmd.AddCommand(ocmd.NewCmdVersion(name, nil, os.Stdout, ocmd.VersionOptions{}))

	flag := cmd.Flags()
	options.Config.Bind(flag)
	options.TemplateRouter.Bind(flag)
	options.RouterStats.Bind(flag)
	options.RouterSelection.Bind(flag)

	return cmd
}

func (o *TemplateRouterOptions) Complete() error {
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

	if len(o.StatsPortString) > 0 {
		statsPort, err := strconv.Atoi(o.StatsPortString)
		if err != nil {
			return fmt.Errorf("stat port is not valid: %v", err)
		}
		o.StatsPort = statsPort
	}

	if nsecs := int(o.ReloadInterval.Seconds()); nsecs < 1 {
		return fmt.Errorf("invalid reload interval: %v - must be a positive duration", nsecs)
	}

	return o.RouterSelection.Complete()
}

func (o *TemplateRouterOptions) Validate() error {
	if len(o.RouterName) == 0 {
		return errors.New("router must have a name to identify itself in route status")
	}
	if len(o.TemplateFile) == 0 {
		return errors.New("template file must be specified")
	}

	if len(o.ReloadScript) == 0 {
		return errors.New("reload script must be specified")
	}
	return nil
}

// Run launches a template router using the provided options. It never exits.
func (o *TemplateRouterOptions) Run() error {
	pluginCfg := templateplugin.TemplatePluginConfig{
		WorkingDir:             o.WorkingDir,
		TemplatePath:           o.TemplateFile,
		ReloadScriptPath:       o.ReloadScript,
		ReloadInterval:         o.ReloadInterval,
		DefaultCertificate:     o.DefaultCertificate,
		DefaultCertificatePath: o.DefaultCertificatePath,
		DefaultCertificateDir:  o.DefaultCertificateDir,
		StatsPort:              o.StatsPort,
		StatsUsername:          o.StatsUsername,
		StatsPassword:          o.StatsPassword,
		PeerService:            o.RouterService,
		IncludeUDP:             o.RouterSelection.IncludeUDP,
		AllowWildcardRoutes:    o.RouterSelection.AllowWildcardRoutes,
	}

	oc, kc, err := o.Config.Clients()
	if err != nil {
		return err
	}

	svcFetcher := templateplugin.NewListWatchServiceLookup(kc, 10*time.Minute)
	templatePlugin, err := templateplugin.NewTemplatePlugin(pluginCfg, svcFetcher)
	if err != nil {
		return err
	}

	statusPlugin := controller.NewStatusAdmitter(templatePlugin, oc, o.RouterName)
	var nextPlugin router.Plugin = statusPlugin
	if o.ExtendedValidation {
		nextPlugin = controller.NewExtendedValidator(nextPlugin, controller.RejectionRecorder(statusPlugin))
	}
	uniqueHostPlugin := controller.NewUniqueHost(nextPlugin, o.RouteSelectionFunc(), controller.RejectionRecorder(statusPlugin))
	plugin := controller.NewHostAdmitter(uniqueHostPlugin, o.RouteAdmissionFunc(), o.RestrictSubdomainOwnership, controller.RejectionRecorder(statusPlugin))

	factory := o.RouterSelection.NewFactory(oc, kc)
	controller := factory.Create(plugin, false)
	controller.Run()

	proc.StartReaper()

	select {}
}
