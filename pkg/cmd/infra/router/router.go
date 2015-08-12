package router

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/router"
	controllerfactory "github.com/openshift/origin/pkg/router/controller/factory"
	"github.com/openshift/origin/pkg/util/proc"
	"github.com/openshift/origin/pkg/version"
	templateplugin "github.com/openshift/origin/plugins/router/template"

	ktypes "k8s.io/kubernetes/pkg/types"
)

const (
	routerLong = `
Start a router

This command launches a router connected to your cluster master. The router listens for routes and endpoints
created by users and keeps a local router configuration up to date with those changes.`
)

type templateRouterConfig struct {
	Config             *clientcmd.Config
	TemplateFile       string
	ReloadScript       string
	DefaultCertificate string
	RouterService      ktypes.NamespacedName
	StatsPort          string
	StatsPassword      string
	StatsUsername      string
}

// NewCommndTemplateRouter provides CLI handler for the template router backend
func NewCommandTemplateRouter(name string) *cobra.Command {
	cfg := &templateRouterConfig{
		Config: clientcmd.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start a router",
		Long:  routerLong,
		Run: func(c *cobra.Command, args []string) {
			defaultCert := util.Env("DEFAULT_CERTIFICATE", "")
			if len(defaultCert) > 0 {
				cfg.DefaultCertificate = defaultCert
			}

			routerSvcNamespace := util.Env("ROUTER_SERVICE_NAMESPACE", "")
			routerSvcName := util.Env("ROUTER_SERVICE_NAME", "")
			cfg.RouterService = ktypes.NamespacedName{
				Namespace: routerSvcNamespace,
				Name:      routerSvcName,
			}

			plugin, err := makeTemplatePlugin(cfg)
			if err != nil {
				glog.Fatal(err)
			}

			if err = start(cfg.Config, plugin); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name))

	flag := cmd.Flags()
	cfg.Config.Bind(flag)
	flag.StringVar(&cfg.TemplateFile, "template", util.Env("TEMPLATE_FILE", ""), "The path to the template file to use")
	flag.StringVar(&cfg.ReloadScript, "reload", util.Env("RELOAD_SCRIPT", ""), "The path to the reload script to use")
	flag.StringVar(&cfg.StatsPort, "stats-port", util.Env("STATS_PORT", ""), "If the underlying router implementation can provide statistics this is a hint to expose it on this port.")
	flag.StringVar(&cfg.StatsPassword, "stats-password", util.Env("STATS_PASSWORD", ""), "If the underlying router implementation can provide statistics this is the requested password for auth.")
	flag.StringVar(&cfg.StatsUsername, "stats-user", util.Env("STATS_USERNAME", ""), "If the underlying router implementation can provide statistics this is the requested username for auth.")

	return cmd
}

func makeTemplatePlugin(cfg *templateRouterConfig) (*templateplugin.TemplatePlugin, error) {
	if cfg.TemplateFile == "" {
		return nil, errors.New("Template file must be specified")
	}

	if cfg.ReloadScript == "" {
		return nil, errors.New("Reload script must be specified")
	}

	statsPort := 0
	var err error = nil
	if cfg.StatsPort != "" {
		statsPort, err = strconv.Atoi(cfg.StatsPort)
		if err != nil {
			return nil, errors.New("Invalid stats port")
		}
	}

	templatePluginCfg := templateplugin.TemplatePluginConfig{
		TemplatePath:       cfg.TemplateFile,
		ReloadScriptPath:   cfg.ReloadScript,
		DefaultCertificate: cfg.DefaultCertificate,
		StatsPort:          statsPort,
		StatsUsername:      cfg.StatsUsername,
		StatsPassword:      cfg.StatsPassword,
		PeerService:        cfg.RouterService,
	}
	return templateplugin.NewTemplatePlugin(templatePluginCfg)
}

// start launches the load balancer.
func start(cfg *clientcmd.Config, plugin router.Plugin) error {
	osClient, kubeClient, err := cfg.Clients()
	if err != nil {
		return err
	}

	proc.StartReaper()

	factory := controllerfactory.RouterControllerFactory{KClient: kubeClient, OSClient: osClient}
	controller := factory.Create(plugin)
	controller.Run()

	select {}

	return nil
}
