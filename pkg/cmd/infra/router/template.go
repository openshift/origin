package router

import (
	"errors"
	"strconv"

	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/util"
	templateplugin "github.com/openshift/origin/plugins/router/template"

	ktypes "k8s.io/kubernetes/pkg/types"
)

// templateRouterConfig is the config necessary to start a template router plugin.
type templateRouterConfig struct {
	// TemplateFile is the path to the file containing the templates for any files
	// the template plugin should generate.
	TemplateFile string

	// ReloadScript is the path to the command that should be executed to reload
	// the configuration when the plugin updates it.
	ReloadScript string

	// RouterService can be used to specify a namespace/name that identifies
	// peer routers.
	RouterService ktypes.NamespacedName

	// StatsPort specifies a port at which the router can provide statistics.
	StatsPort string

	// StatsPassword specifies a password required to authenticate connections to
	// the statistics port.
	StatsPassword string

	// StatsUsername specifies a username required to authenticate connections to
	// the statistics port.
	StatsUsername string
}

// bindFlagsForTemplateRouterConfig binds flags for template router
// configuration.
func bindFlagsForTemplateRouterConfig(flag *pflag.FlagSet,
	cfg *routerConfig) {
	flag.StringVar(&cfg.TemplateRouterConfig.TemplateFile, "template", util.Env("TEMPLATE_FILE", ""), "The path to the template file to use")
	flag.StringVar(&cfg.TemplateRouterConfig.ReloadScript, "reload", util.Env("RELOAD_SCRIPT", ""), "The path to the reload script to use")
	flag.StringVar(&cfg.TemplateRouterConfig.StatsPort, "stats-port", util.Env("STATS_PORT", ""), "If the underlying router implementation can provide statistics this is a hint to expose it on this port.")
	flag.StringVar(&cfg.TemplateRouterConfig.StatsPassword, "stats-password", util.Env("STATS_PASSWORD", ""), "If the underlying router implementation can provide statistics this is the requested password for auth.")
	flag.StringVar(&cfg.TemplateRouterConfig.StatsUsername, "stats-user", util.Env("STATS_USERNAME", ""), "If the underlying router implementation can provide statistics this is the requested username for auth.")
}

// makeTemplatePlugin creates a template router plugin.
func makeTemplatePlugin(cfg *routerConfig) (*templateplugin.TemplatePlugin, error) {
	if cfg.TemplateRouterConfig.TemplateFile == "" {
		return nil, errors.New("Template file must be specified")
	}

	if cfg.TemplateRouterConfig.ReloadScript == "" {
		return nil, errors.New("Reload script must be specified")
	}

	statsPort := 0
	var err error = nil
	if cfg.TemplateRouterConfig.StatsPort != "" {
		statsPort, err = strconv.Atoi(cfg.TemplateRouterConfig.StatsPort)
		if err != nil {
			return nil, errors.New("Invalid stats port")
		}
	}

	routerSvcNamespace := util.Env("ROUTER_SERVICE_NAMESPACE", "")
	routerSvcName := util.Env("ROUTER_SERVICE_NAME", "")
	cfg.TemplateRouterConfig.RouterService = ktypes.NamespacedName{
		Namespace: routerSvcNamespace,
		Name:      routerSvcName,
	}

	templatePluginCfg := templateplugin.TemplatePluginConfig{
		TemplatePath:       cfg.TemplateRouterConfig.TemplateFile,
		ReloadScriptPath:   cfg.TemplateRouterConfig.ReloadScript,
		DefaultCertificate: cfg.DefaultCertificate,
		StatsPort:          statsPort,
		StatsUsername:      cfg.TemplateRouterConfig.StatsUsername,
		StatsPassword:      cfg.TemplateRouterConfig.StatsPassword,
		PeerService:        cfg.TemplateRouterConfig.RouterService,
	}
	return templateplugin.NewTemplatePlugin(templatePluginCfg)
}
