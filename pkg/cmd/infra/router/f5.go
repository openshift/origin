package router

import (
	"errors"

	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/util"
	f5plugin "github.com/openshift/origin/plugins/router/f5"
)

// f5RouterConfig is the config necessary to start an F5 router plugin.
type f5RouterConfig struct {
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
}

// bindFlagsForTemplateRouterConfig binds flags for template router
// configuration.
func bindFlagsForF5RouterConfig(flag *pflag.FlagSet, cfg *routerConfig) {
	flag.StringVar(&cfg.F5RouterConfig.Host, "f5-host", util.Env("ROUTER_EXTERNAL_HOST_HOSTNAME", ""), "The host of F5 BIG-IP's management interface")
	flag.StringVar(&cfg.F5RouterConfig.Username, "f5-username", util.Env("ROUTER_EXTERNAL_HOST_USERNAME", ""), "The username for F5 BIG-IP's management utility")
	flag.StringVar(&cfg.F5RouterConfig.Password, "f5-password", util.Env("ROUTER_EXTERNAL_HOST_PASSWORD", ""), "The password for F5 BIG-IP's management utility")
	flag.StringVar(&cfg.F5RouterConfig.HttpVserver, "f5-http-vserver", util.Env("ROUTER_EXTERNAL_HOST_HTTP_VSERVER", "ose-vserver"), "The F5 BIG-IP virtual server for HTTP connections")
	flag.StringVar(&cfg.F5RouterConfig.HttpsVserver, "f5-https-vserver", util.Env("ROUTER_EXTERNAL_HOST_HTTPS_VSERVER", "https-ose-vserver"), "The F5 BIG-IP virtual server for HTTPS connections")
	flag.StringVar(&cfg.F5RouterConfig.PrivateKey, "f5-private-key", util.Env("ROUTER_EXTERNAL_HOST_PRIVKEY", ""), "The path to the F5 BIG-IP SSH private key file")
	flag.BoolVar(&cfg.F5RouterConfig.Insecure, "f5-insecure", util.Env("ROUTER_EXTERNAL_HOST_INSECURE", "") == "true", "Skip strict certificate verification")
}

// makeF5Plugin makes an F5 router plugin.
func makeF5Plugin(cfg *routerConfig) (*f5plugin.F5Plugin, error) {
	if cfg.F5RouterConfig.Host == "" {
		return nil, errors.New("F5 host must be specified")
	}

	if cfg.F5RouterConfig.Username == "" {
		return nil, errors.New("F5 username must be specified")
	}

	if cfg.F5RouterConfig.Password == "" {
		return nil, errors.New("F5 password must be specified")
	}

	if cfg.F5RouterConfig.HttpVserver == "" &&
		cfg.F5RouterConfig.HttpsVserver == "" {
		return nil, errors.New("F5 HTTP and HTTPS vservers cannot both be blank")
	}

	f5PluginCfg := f5plugin.F5PluginConfig{
		Host:         cfg.F5RouterConfig.Host,
		Username:     cfg.F5RouterConfig.Username,
		Password:     cfg.F5RouterConfig.Password,
		HttpVserver:  cfg.F5RouterConfig.HttpVserver,
		HttpsVserver: cfg.F5RouterConfig.HttpsVserver,
		PrivateKey:   cfg.F5RouterConfig.PrivateKey,
		Insecure:     cfg.F5RouterConfig.Insecure,
	}
	return f5plugin.NewF5Plugin(f5PluginCfg)
}
