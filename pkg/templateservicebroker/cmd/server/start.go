package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/template/servicebroker/apis/config"
	configinstall "github.com/openshift/origin/pkg/template/servicebroker/apis/config/install"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/server"
)

type TemplateServiceBrokerServerOptions struct {
	// we don't have any storage, so we shouldn't use the recommended options
	SecureServing  *genericoptions.SecureServingOptions
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Audit          *genericoptions.AuditOptions
	Features       *genericoptions.FeatureOptions

	StdOut io.Writer
	StdErr io.Writer

	TSBConfig *config.TemplateServiceBrokerConfig
}

func NewTemplateServiceBrokerServerOptions(out, errOut io.Writer) *TemplateServiceBrokerServerOptions {
	o := &TemplateServiceBrokerServerOptions{
		SecureServing:  genericoptions.NewSecureServingOptions(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Audit:          genericoptions.NewAuditOptions(),
		Features:       genericoptions.NewFeatureOptions(),

		StdOut: out,
		StdErr: errOut,
	}

	return o
}

func NewCommandStartTemplateServiceBrokerServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := NewTemplateServiceBrokerServerOptions(out, errOut)

	cmd := &cobra.Command{
		Use:   "template-service-broker",
		Short: "Launch a template service broker server",
		Long:  "Launch a template service broker server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunTemplateServiceBrokerServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.SecureServing.AddFlags(flags)
	o.Authentication.AddFlags(flags)
	o.Authorization.AddFlags(flags)
	o.Audit.AddFlags(flags)
	o.Features.AddFlags(flags)
	flags.String("config", "", "filename containing the TemplateServiceBrokerConfig")

	return cmd
}

func (o TemplateServiceBrokerServerOptions) Validate(args []string) error {
	if o.TSBConfig == nil {
		return fmt.Errorf("missing config: specify --config")
	}
	if len(o.TSBConfig.TemplateNamespaces) == 0 {
		return fmt.Errorf("templateNamespaces are required")
	}

	return nil
}

func (o *TemplateServiceBrokerServerOptions) Complete(cmd *cobra.Command) error {
	configFile := util.GetFlagString(cmd, "config")
	if len(configFile) > 0 {
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}
		configObj, err := runtime.Decode(configCodecs.UniversalDecoder(), content)
		if err != nil {
			return err
		}
		config, ok := configObj.(*config.TemplateServiceBrokerConfig)
		if !ok {
			return fmt.Errorf("unexpected type: %T", configObj)
		}
		o.TSBConfig = config
	}

	return nil
}

func (o TemplateServiceBrokerServerOptions) Config() (*server.TemplateServiceBrokerConfig, error) {
	// TODO have a "real" external address
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(server.Codecs)
	if err := o.SecureServing.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	if err := o.Authentication.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	if err := o.Authorization.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	if err := o.Audit.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	if err := o.Features.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	serverConfig.EnableMetrics = true

	config := &server.TemplateServiceBrokerConfig{
		GenericConfig: &genericapiserver.RecommendedConfig{Config: *serverConfig},

		ExtraConfig: server.ExtraConfig{TemplateNamespaces: o.TSBConfig.TemplateNamespaces},
		// TODO add the code to set up the client and informers that you need here
	}
	return config, nil
}

func (o TemplateServiceBrokerServerOptions) RunTemplateServiceBrokerServer(stopCh <-chan struct{}) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New(genericapiserver.EmptyDelegate)
	if err != nil {
		return err
	}
	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}

// these are used to set up for reading the config
var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	configScheme         = runtime.NewScheme()
	configCodecs         = serializer.NewCodecFactory(configScheme)
)

func init() {
	configinstall.Install(groupFactoryRegistry, registry, configScheme)
}
