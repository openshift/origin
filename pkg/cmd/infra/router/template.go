package router

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	ocmd "github.com/openshift/origin/pkg/oc/cli/cmd"
	projectinternalclientset "github.com/openshift/origin/pkg/project/generated/internalclientset"
	routeinternalclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
	"github.com/openshift/origin/pkg/router/metrics"
	"github.com/openshift/origin/pkg/router/metrics/haproxy"
	templateplugin "github.com/openshift/origin/pkg/router/template"
	"github.com/openshift/origin/pkg/util/proc"
	"github.com/openshift/origin/pkg/version"
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
	RouterName               string
	RouterCanonicalHostname  string
	WorkingDir               string
	TemplateFile             string
	ReloadScript             string
	ReloadInterval           time.Duration
	DefaultCertificate       string
	DefaultCertificatePath   string
	DefaultCertificateDir    string
	DefaultDestinationCAPath string
	ExtendedValidation       bool
	RouterService            *ktypes.NamespacedName
	BindPortsAfterSync       bool
	MaxConnections           string
	Ciphers                  string
	StrictSNI                bool
	MetricsType              string
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
	flag.StringVar(&o.RouterCanonicalHostname, "router-canonical-hostname", util.Env("ROUTER_CANONICAL_HOSTNAME", ""), "CanonicalHostname is the external host name for the router that can be used as a CNAME for the host requested for this route. This value is optional and may not be set in all cases.")
	flag.StringVar(&o.WorkingDir, "working-dir", "/var/lib/haproxy/router", "The working directory for the router plugin")
	flag.StringVar(&o.DefaultCertificate, "default-certificate", util.Env("DEFAULT_CERTIFICATE", ""), "The contents of a default certificate to use for routes that don't expose a TLS server cert; in PEM format")
	flag.StringVar(&o.DefaultCertificatePath, "default-certificate-path", util.Env("DEFAULT_CERTIFICATE_PATH", ""), "A path to default certificate to use for routes that don't expose a TLS server cert; in PEM format")
	flag.StringVar(&o.DefaultCertificateDir, "default-certificate-dir", util.Env("DEFAULT_CERTIFICATE_DIR", ""), "A path to a directory that contains a file named tls.crt. If tls.crt is not a PEM file which also contains a private key, it is first combined with a file named tls.key in the same directory. The PEM-format contents are then used as the default certificate. Only used if default-certificate and default-certificate-path are not specified.")
	flag.StringVar(&o.DefaultDestinationCAPath, "default-destination-ca-path", util.Env("DEFAULT_DESTINATION_CA_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"), "A path to a PEM file containing the default CA bundle to use with re-encrypt routes. This CA should sign for certificates in the Kubernetes DNS space (service.namespace.svc).")
	flag.StringVar(&o.TemplateFile, "template", util.Env("TEMPLATE_FILE", ""), "The path to the template file to use")
	flag.StringVar(&o.ReloadScript, "reload", util.Env("RELOAD_SCRIPT", ""), "The path to the reload script to use")
	flag.DurationVar(&o.ReloadInterval, "interval", reloadInterval(), "Controls how often router reloads are invoked. Mutiple router reload requests are coalesced for the duration of this interval since the last reload time.")
	flag.BoolVar(&o.ExtendedValidation, "extended-validation", util.Env("EXTENDED_VALIDATION", "true") == "true", "If set, then an additional extended validation step is performed on all routes admitted in by this router. Defaults to true and enables the extended validation checks.")
	flag.BoolVar(&o.BindPortsAfterSync, "bind-ports-after-sync", util.Env("ROUTER_BIND_PORTS_AFTER_SYNC", "") == "true", "Bind ports only after route state has been synchronized")
	flag.StringVar(&o.MaxConnections, "max-connections", util.Env("ROUTER_MAX_CONNECTIONS", ""), "Specifies the maximum number of concurrent connections.")
	flag.StringVar(&o.Ciphers, "ciphers", util.Env("ROUTER_CIPHERS", ""), "Specifies the cipher suites to use. You can choose a predefined cipher set ('modern', 'intermediate', or 'old') or specify exact cipher suites by passing a : separated list.")
	flag.BoolVar(&o.StrictSNI, "strict-sni", util.Env("ROUTER_STRICT_SNI", "") == "true", "Use strict-sni bind processing (do not use default cert).")
	flag.StringVar(&o.MetricsType, "metrics-type", util.Env("ROUTER_METRICS_TYPE", ""), "Specifies the type of metrics to gather. Supports 'haproxy'.")
}

type RouterStats struct {
	StatsPortString string
	StatsPassword   string
	StatsUsername   string

	StatsPort int
}

func (o *RouterStats) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.StatsPortString, "stats-port", util.Env("STATS_PORT", ""), "If the underlying router implementation can provide statistics this is a hint to expose it on this port. Ignored if listen-addr is specified.")
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
			// if the user did not specify a destination ca path, and the file does not exist, disable the default in order
			// to preserve backwards compatibility with older clusters
			if !c.Flags().Lookup("default-destination-ca-path").Changed && util.Env("DEFAULT_DESTINATION_CA_PATH", "") == "" {
				if _, err := os.Stat(options.TemplateRouter.DefaultDestinationCAPath); err != nil {
					options.TemplateRouter.DefaultDestinationCAPath = ""
				}
			}
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
	routerCanonicalHostname := util.Env("ROUTER_CANONICAL_HOSTNAME", "")
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
	if len(o.ListenAddr) > 0 {
		_, port, err := net.SplitHostPort(o.ListenAddr)
		if err != nil {
			return fmt.Errorf("listen-addr is not valid: %v", err)
		}
		// stats port on listen-addr overrides stats port argument
		statsPort, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("listen-addr port is not valid: %v", err)
		}
		o.StatsPort = statsPort
	} else {
		if o.StatsPort != 0 {
			o.ListenAddr = fmt.Sprintf("0.0.0.0:%d", o.StatsPort)
		}
	}

	if nsecs := int(o.ReloadInterval.Seconds()); nsecs < 1 {
		return fmt.Errorf("invalid reload interval: %v - must be a positive duration", nsecs)
	}

	if len(routerCanonicalHostname) > 0 {
		if errs := validation.IsDNS1123Subdomain(routerCanonicalHostname); len(errs) != 0 {
			return fmt.Errorf("invalid canonical hostname: %s", routerCanonicalHostname)
		}
		if errs := validation.IsValidIP(routerCanonicalHostname); len(errs) == 0 {
			return fmt.Errorf("canonical hostname must not be an IP address: %s", routerCanonicalHostname)
		}
	}

	return o.RouterSelection.Complete()
}

// supportedMetricsTypes is the set of supported metrics arguments
var supportedMetricsTypes = sets.NewString("haproxy")

func (o *TemplateRouterOptions) Validate() error {
	if len(o.MetricsType) > 0 && !supportedMetricsTypes.Has(o.MetricsType) {
		return fmt.Errorf("supported metrics types are: %s", strings.Join(supportedMetricsTypes.List(), ", "))
	}
	if len(o.RouterName) == 0 {
		return errors.New("router must have a name to identify itself in route status")
	}
	if len(o.TemplateFile) == 0 {
		return errors.New("template file must be specified")
	}
	if len(o.TemplateRouter.DefaultDestinationCAPath) != 0 {
		if _, err := os.Stat(o.TemplateRouter.DefaultDestinationCAPath); err != nil {
			return fmt.Errorf("unable to load default destination CA certificate: %v", err)
		}
	}
	if len(o.ReloadScript) == 0 {
		return errors.New("reload script must be specified")
	}
	return nil
}

// Run launches a template router using the provided options. It never exits.
func (o *TemplateRouterOptions) Run() error {
	glog.Infof("Starting template router (%s)", version.Get())

	statsPort := o.StatsPort
	switch {
	case o.MetricsType == "haproxy":
		if len(o.StatsUsername) == 0 || len(o.StatsPassword) == 0 {
			glog.Warningf("Metrics were requested but no username or password has been provided - the metrics endpoint will not be accessible to prevent accidental security breaches")
		}
		// Exposed to allow tuning in production if this becomes an issue
		var timeout time.Duration
		if t := util.Env("ROUTER_METRICS_HAPROXY_TIMEOUT", ""); len(t) > 0 {
			d, err := time.ParseDuration(t)
			if err != nil {
				return fmt.Errorf("ROUTER_METRICS_HAPROXY_TIMEOUT is not a valid duration: %v", err)
			}
			timeout = d
		}
		// Exposed to allow tuning in production if this becomes an issue
		var baseScrapeInterval time.Duration
		if t := util.Env("ROUTER_METRICS_HAPROXY_BASE_SCRAPE_INTERVAL", ""); len(t) > 0 {
			d, err := time.ParseDuration(t)
			if err != nil {
				return fmt.Errorf("ROUTER_METRICS_HAPROXY_BASE_SCRAPE_INTERVAL is not a valid duration: %v", err)
			}
			baseScrapeInterval = d
		}
		// Exposed to allow tuning in production if this becomes an issue
		var serverThreshold int
		if t := util.Env("ROUTER_METRICS_HAPROXY_SERVER_THRESHOLD", ""); len(t) > 0 {
			i, err := strconv.Atoi(t)
			if err != nil {
				return fmt.Errorf("ROUTER_METRICS_HAPROXY_SERVER_THRESHOLD is not a valid integer: %v", err)
			}
			serverThreshold = i
		}
		// Exposed to allow tuning in production if this becomes an issue
		var exported []int
		if t := util.Env("ROUTER_METRICS_HAPROXY_EXPORTED", ""); len(t) > 0 {
			for _, s := range strings.Split(t, ",") {
				i, err := strconv.Atoi(s)
				if err != nil {
					return errors.New("ROUTER_METRICS_HAPROXY_EXPORTED must be a comma delimited list of column numbers to extract from the HAProxy configuration")
				}
				exported = append(exported, i)
			}
		}

		_, err := haproxy.NewPrometheusCollector(haproxy.PrometheusOptions{
			// Only template router customizers who alter the image should need this
			ScrapeURI: util.Env("ROUTER_METRICS_HAPROXY_SCRAPE_URI", ""),
			// Only template router customizers who alter the image should need this
			PidFile:            util.Env("ROUTER_METRICS_HAPROXY_PID_FILE", ""),
			Timeout:            timeout,
			ServerThreshold:    serverThreshold,
			BaseScrapeInterval: baseScrapeInterval,
			ExportedMetrics:    exported,
		})
		if err != nil {
			return err
		}

		// Metrics will handle healthz on the stats port, and instruct the template router to disable stats completely.
		// The underlying router must provide a custom health check if customized which will be called into.
		statsPort = -1
		httpURL := util.Env("ROUTER_METRICS_READY_HTTP_URL", fmt.Sprintf("http://%s:%s/_______internal_router_healthz", "localhost", util.Env("ROUTER_SERVICE_HTTP_PORT", "80")))
		u, err := url.Parse(httpURL)
		if err != nil {
			return fmt.Errorf("ROUTER_METRICS_READY_HTTP_URL must be a valid URL or empty: %v", err)
		}
		check := metrics.HTTPBackendAvailable(u)
		if useProxy := util.Env("ROUTER_USE_PROXY_PROTOCOL", ""); useProxy == "true" || useProxy == "TRUE" {
			check = metrics.ProxyProtocolHTTPBackendAvailable(u)
		}
		metrics.Listen(o.ListenAddr, o.StatsUsername, o.StatsPassword, check)
	}

	pluginCfg := templateplugin.TemplatePluginConfig{
		WorkingDir:               o.WorkingDir,
		TemplatePath:             o.TemplateFile,
		ReloadScriptPath:         o.ReloadScript,
		ReloadInterval:           o.ReloadInterval,
		DefaultCertificate:       o.DefaultCertificate,
		DefaultCertificatePath:   o.DefaultCertificatePath,
		DefaultCertificateDir:    o.DefaultCertificateDir,
		DefaultDestinationCAPath: o.DefaultDestinationCAPath,
		StatsPort:                statsPort,
		StatsUsername:            o.StatsUsername,
		StatsPassword:            o.StatsPassword,
		PeerService:              o.RouterService,
		BindPortsAfterSync:       o.BindPortsAfterSync,
		IncludeUDP:               o.RouterSelection.IncludeUDP,
		AllowWildcardRoutes:      o.RouterSelection.AllowWildcardRoutes,
		MaxConnections:           o.MaxConnections,
		Ciphers:                  o.Ciphers,
		StrictSNI:                o.StrictSNI,
	}

	_, kc, err := o.Config.Clients()
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

	svcFetcher := templateplugin.NewListWatchServiceLookup(kc.Core(), 10*time.Minute)
	templatePlugin, err := templateplugin.NewTemplatePlugin(pluginCfg, svcFetcher)
	if err != nil {
		return err
	}

	statusPlugin := controller.NewStatusAdmitter(templatePlugin, routeclient, o.RouterName, o.RouterCanonicalHostname)
	var nextPlugin router.Plugin = statusPlugin
	if o.ExtendedValidation {
		nextPlugin = controller.NewExtendedValidator(nextPlugin, controller.RejectionRecorder(statusPlugin))
	}
	uniqueHostPlugin := controller.NewUniqueHost(nextPlugin, o.RouteSelectionFunc(), o.RouterSelection.DisableNamespaceOwnershipCheck, controller.RejectionRecorder(statusPlugin))
	plugin := controller.NewHostAdmitter(uniqueHostPlugin, o.RouteAdmissionFunc(), o.AllowWildcardRoutes, o.RouterSelection.DisableNamespaceOwnershipCheck, controller.RejectionRecorder(statusPlugin))

	factory := o.RouterSelection.NewFactory(routeclient, projectclient.Projects(), kc)
	controller := factory.Create(plugin, false, o.EnableIngress)
	controller.Run()

	proc.StartReaper()

	select {}
}
