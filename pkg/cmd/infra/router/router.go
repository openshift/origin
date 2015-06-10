package router

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/router"
	controllerfactory "github.com/openshift/origin/pkg/router/controller/factory"
	"github.com/openshift/origin/pkg/util/proc"
	"github.com/openshift/origin/pkg/version"
)

const (
	routerLong = `
Start a router

This command launches a router connected to your cluster master. The router listens for routes and endpoints
created by users and keeps a local router configuration up to date with those changes.`
)

// routerConfig represents a container for all other router config types.
// Any global config belongs on this level.
type routerConfig struct {
	// Config holds global flags that are passed to all client commands that need
	// to connect to master.
	Config *clientcmd.Config

	// DefaultCertificate holds the certificate that will be used if no more
	// specific certificate is found.  This is typically a wildcard certificate.
	DefaultCertificate string

	// TemplateRouterConfig holds config items for template router plugins.
	TemplateRouterConfig *templateRouterConfig

	// F5RouterConfig holds the configuration information for the F5
	// router plugin.
	F5RouterConfig *f5RouterConfig
}

const (
	// routerTypeF5 is the type value for an f5 router plugin.
	routerTypeF5 = "f5"
	// routerTypeTemplate is the type value for a template router plugin.
	routerTypeTemplate = "template"
)

// NewCommandRouter provides a CLI handler for the router.
func NewCommandRouter(name string) *cobra.Command {
	cfg := &routerConfig{
		Config:               clientcmd.NewConfig(),
		TemplateRouterConfig: &templateRouterConfig{},
		F5RouterConfig:       &f5RouterConfig{},
	}

	cfgType := util.Env("ROUTER_TYPE", "")

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Start a router",
		Long:  routerLong,
		Run: func(c *cobra.Command, args []string) {
			if !isValidType(cfgType) {
				glog.Fatalf("Invalid type specified: %s.  Valid values are %s, %s",
					cfgType, routerTypeF5, routerTypeTemplate)
			}

			if cfgType == routerTypeF5 {
				cfg.TemplateRouterConfig = nil
			} else if cfgType == routerTypeTemplate {
				cfg.F5RouterConfig = nil
			}

			defaultCert := util.Env("DEFAULT_CERTIFICATE", "")
			if len(defaultCert) > 0 {
				cfg.DefaultCertificate = defaultCert
			}

			var plugin router.Plugin
			var err error

			if cfg.F5RouterConfig != nil {
				plugin, err = makeF5Plugin(cfg)
			} else if cfg.TemplateRouterConfig != nil {
				plugin, err = makeTemplatePlugin(cfg)
			} else {
				err = fmt.Errorf("Plugin type not recognized")
			}
			if err != nil {
				glog.Fatal(err)
			}

			if err := start(cfg.Config, plugin); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name))

	flag := cmd.Flags()

	// Binds for generic config
	cfg.Config.Bind(flag)

	flag.StringVar(&cfgType, "type", util.Env("ROUTER_TYPE", ""), "The type of router to create.  Valid values: template, f5")

	bindFlagsForTemplateRouterConfig(flag, cfg)
	bindFlagsForF5RouterConfig(flag, cfg)

	return cmd
}

// isValidType checks that the router type specified is a known type.
func isValidType(t string) bool {
	return t == routerTypeF5 || t == routerTypeTemplate
}

// start launches the router.
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
}
