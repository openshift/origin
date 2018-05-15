package router

import (
	"crypto/tls"
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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/server/healthz"
	authenticationclient "k8s.io/client-go/kubernetes/typed/authentication/v1beta1"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1beta1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/cmd/util"
	cmdversion "github.com/openshift/origin/pkg/cmd/version"
	projectinternalclientset "github.com/openshift/origin/pkg/project/generated/internalclientset"
	routeinternalclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	routelisters "github.com/openshift/origin/pkg/route/generated/listers/route/internalversion"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
	"github.com/openshift/origin/pkg/router/metrics"
	"github.com/openshift/origin/pkg/router/metrics/haproxy"
	templateplugin "github.com/openshift/origin/pkg/router/template"
	"github.com/openshift/origin/pkg/util/proc"
	"github.com/openshift/origin/pkg/util/writerlease"
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
	Config *Config

	TemplateRouter
	RouterStats
	RouterSelection
}

type TemplateRouter struct {
	WorkingDir               string
	TemplateFile             string
	ReloadScript             string
	ReloadInterval           time.Duration
	DefaultCertificate       string
	DefaultCertificatePath   string
	DefaultCertificateDir    string
	DefaultDestinationCAPath string
	RouterService            *ktypes.NamespacedName
	BindPortsAfterSync       bool
	MaxConnections           string
	Ciphers                  string
	StrictSNI                bool
	MetricsType              string
}

// isTrue here has the same logic as the function within package pkg/router/template
func isTrue(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
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
	flag.StringVar(&o.WorkingDir, "working-dir", "/var/lib/haproxy/router", "The working directory for the router plugin")
	flag.StringVar(&o.DefaultCertificate, "default-certificate", util.Env("DEFAULT_CERTIFICATE", ""), "The contents of a default certificate to use for routes that don't expose a TLS server cert; in PEM format")
	flag.StringVar(&o.DefaultCertificatePath, "default-certificate-path", util.Env("DEFAULT_CERTIFICATE_PATH", ""), "A path to default certificate to use for routes that don't expose a TLS server cert; in PEM format")
	flag.StringVar(&o.DefaultCertificateDir, "default-certificate-dir", util.Env("DEFAULT_CERTIFICATE_DIR", ""), "A path to a directory that contains a file named tls.crt. If tls.crt is not a PEM file which also contains a private key, it is first combined with a file named tls.key in the same directory. The PEM-format contents are then used as the default certificate. Only used if default-certificate and default-certificate-path are not specified.")
	flag.StringVar(&o.DefaultDestinationCAPath, "default-destination-ca-path", util.Env("DEFAULT_DESTINATION_CA_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"), "A path to a PEM file containing the default CA bundle to use with re-encrypt routes. This CA should sign for certificates in the Kubernetes DNS space (service.namespace.svc).")
	flag.StringVar(&o.TemplateFile, "template", util.Env("TEMPLATE_FILE", ""), "The path to the template file to use")
	flag.StringVar(&o.ReloadScript, "reload", util.Env("RELOAD_SCRIPT", ""), "The path to the reload script to use")
	flag.DurationVar(&o.ReloadInterval, "interval", reloadInterval(), "Controls how often router reloads are invoked. Mutiple router reload requests are coalesced for the duration of this interval since the last reload time.")
	flag.BoolVar(&o.BindPortsAfterSync, "bind-ports-after-sync", util.Env("ROUTER_BIND_PORTS_AFTER_SYNC", "") == "true", "Bind ports only after route state has been synchronized")
	flag.StringVar(&o.MaxConnections, "max-connections", util.Env("ROUTER_MAX_CONNECTIONS", ""), "Specifies the maximum number of concurrent connections.")
	flag.StringVar(&o.Ciphers, "ciphers", util.Env("ROUTER_CIPHERS", ""), "Specifies the cipher suites to use. You can choose a predefined cipher set ('modern', 'intermediate', or 'old') or specify exact cipher suites by passing a : separated list.")
	flag.BoolVar(&o.StrictSNI, "strict-sni", isTrue(util.Env("ROUTER_STRICT_SNI", "")), "Use strict-sni bind processing (do not use default cert).")
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
		Config: NewConfig(),
	}
	options.Config.FromFile = true

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, ConfigSyntax),
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

	cmd.AddCommand(cmdversion.NewCmdVersion(name, version.Get(), os.Stdout))

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

	return o.RouterSelection.Complete()
}

// supportedMetricsTypes is the set of supported metrics arguments
var supportedMetricsTypes = sets.NewString("haproxy")

func (o *TemplateRouterOptions) Validate() error {
	if len(o.MetricsType) > 0 && !supportedMetricsTypes.Has(o.MetricsType) {
		return fmt.Errorf("supported metrics types are: %s", strings.Join(supportedMetricsTypes.List(), ", "))
	}
	if len(o.RouterName) == 0 && o.UpdateStatus {
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

	var reloadCallbacks []func()

	statsPort := o.StatsPort
	switch {
	case o.MetricsType == "haproxy" && statsPort != 0:
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

		collector, err := haproxy.NewPrometheusCollector(haproxy.PrometheusOptions{
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
		if isTrue(util.Env("ROUTER_USE_PROXY_PROTOCOL", "")) {
			check = metrics.ProxyProtocolHTTPBackendAvailable(u)
		}

		kubeconfig := o.Config.KubeConfig()
		client, err := authorizationclient.NewForConfig(kubeconfig)
		if err != nil {
			return err
		}
		authz, err := authorizerfactory.DelegatingAuthorizerConfig{
			SubjectAccessReviewClient: client.SubjectAccessReviews(),
			AllowCacheTTL:             2 * time.Minute,
			DenyCacheTTL:              5 * time.Second,
		}.New()
		if err != nil {
			return err
		}
		tokenClient, err := authenticationclient.NewForConfig(kubeconfig)
		if err != nil {
			return err
		}
		authn, _, err := authenticatorfactory.DelegatingAuthenticatorConfig{
			Anonymous:               true,
			TokenAccessReviewClient: tokenClient.TokenReviews(),
			CacheTTL:                10 * time.Second,
			ClientCAFile:            util.Env("ROUTER_METRICS_AUTHENTICATOR_CA_FILE", ""),
		}.New()
		if err != nil {
			return err
		}
		l := metrics.Listener{
			Addr:          o.ListenAddr,
			Username:      o.StatsUsername,
			Password:      o.StatsPassword,
			Authenticator: authn,
			Authorizer:    authz,
			Record: authorizer.AttributesRecord{
				ResourceRequest: true,
				APIGroup:        "route.openshift.io",
				Resource:        "routers",
				Name:            o.RouterName,
			},
			Checks: []healthz.HealthzChecker{check},
		}
		if certFile := util.Env("ROUTER_METRICS_TLS_CERT_FILE", ""); len(certFile) > 0 {
			certificate, err := tls.LoadX509KeyPair(certFile, util.Env("ROUTER_METRICS_TLS_KEY_FILE", ""))
			if err != nil {
				return err
			}
			l.TLSConfig = crypto.SecureTLSConfig(&tls.Config{
				Certificates: []tls.Certificate{certificate},
				ClientAuth:   tls.RequestClientCert,
			})
		}
		l.Listen()

		// on reload, invoke the collector to preserve whatever metrics we can
		reloadCallbacks = append(reloadCallbacks, collector.CollectNow)
	}

	pluginCfg := templateplugin.TemplatePluginConfig{
		WorkingDir:               o.WorkingDir,
		TemplatePath:             o.TemplateFile,
		ReloadScriptPath:         o.ReloadScript,
		ReloadInterval:           o.ReloadInterval,
		ReloadCallbacks:          reloadCallbacks,
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

	svcFetcher := templateplugin.NewListWatchServiceLookup(kc.Core(), o.ResyncInterval, o.Namespace)
	templatePlugin, err := templateplugin.NewTemplatePlugin(pluginCfg, svcFetcher)
	if err != nil {
		return err
	}

	factory := o.RouterSelection.NewFactory(routeclient, projectclient.Project().Projects(), kc)
	factory.RouteModifierFn = o.RouteUpdate

	var plugin router.Plugin = templatePlugin
	var recorder controller.RejectionRecorder = controller.LogRejections
	if o.UpdateStatus {
		lease := writerlease.New(time.Minute, 3*time.Second)
		go lease.Run(wait.NeverStop)
		tracker := controller.NewSimpleContentionTracker(o.ResyncInterval / 10)
		tracker.SetConflictMessage(fmt.Sprintf("The router detected another process is writing conflicting updates to route status with name %q. Please ensure that the configuration of all routers is consistent. Route status will not be updated as long as conflicts are detected.", o.RouterName))
		go tracker.Run(wait.NeverStop)
		routeLister := routelisters.NewRouteLister(factory.CreateRoutesSharedInformer().GetIndexer())
		status := controller.NewStatusAdmitter(plugin, routeclient.Route(), routeLister, o.RouterName, o.RouterCanonicalHostname, lease, tracker)
		recorder = status
		plugin = status
	}
	if o.ExtendedValidation {
		plugin = controller.NewExtendedValidator(plugin, recorder)
	}
	plugin = controller.NewUniqueHost(plugin, o.RouterSelection.DisableNamespaceOwnershipCheck, recorder)
	plugin = controller.NewHostAdmitter(plugin, o.RouteAdmissionFunc(), o.AllowWildcardRoutes, o.RouterSelection.DisableNamespaceOwnershipCheck, recorder)

	controller := factory.Create(plugin, false)
	controller.Run()

	proc.StartReaper()

	select {}
}
