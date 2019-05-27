package server

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"time"

	"k8s.io/apiserver/pkg/util/webhook"

	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericapiserveroptions "k8s.io/apiserver/pkg/server/options"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/template/servicebroker/apis/config"
	configinstall "github.com/openshift/origin/pkg/template/servicebroker/apis/config/install"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/server"
)

type TemplateServiceBrokerServerOptions struct {
	// we don't have any storage, so we shouldn't use the recommended options
	SecureServing  *genericapiserveroptions.SecureServingOptionsWithLoopback
	Authentication *genericapiserveroptions.DelegatingAuthenticationOptions
	Authorization  *genericapiserveroptions.DelegatingAuthorizationOptions
	Audit          *genericapiserveroptions.AuditOptions
	Features       *genericapiserveroptions.FeatureOptions

	StdOut io.Writer
	StdErr io.Writer

	TSBConfig *config.TemplateServiceBrokerConfig
}

func NewTemplateServiceBrokerServerOptions(out, errOut io.Writer) *TemplateServiceBrokerServerOptions {
	o := &TemplateServiceBrokerServerOptions{
		SecureServing:  genericapiserveroptions.NewSecureServingOptions().WithLoopback(),
		Authentication: genericapiserveroptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericapiserveroptions.NewDelegatingAuthorizationOptions(),
		Audit:          genericapiserveroptions.NewAuditOptions(),
		Features:       genericapiserveroptions.NewFeatureOptions(),

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

	kubeClientConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return nil, err
	}
	serverConfig := genericapiserver.NewRecommendedConfig(server.Codecs)
	serverConfig.ClientConfig = kubeClientConfig
	serverConfig.SharedInformerFactory = informers.NewSharedInformerFactory(kubeClient, 10*time.Hour)
	if err := o.SecureServing.ApplyTo(&serverConfig.SecureServing, &serverConfig.LoopbackClientConfig); err != nil {
		return nil, err
	}
	if err := o.Authentication.ApplyTo(&serverConfig.Authentication, serverConfig.SecureServing, serverConfig.OpenAPIConfig); err != nil {
		return nil, err
	}
	if err := o.Authorization.ApplyTo(&serverConfig.Authorization); err != nil {
		return nil, err
	}

	authInfoResolverWrapper := webhook.NewDefaultAuthenticationInfoResolverWrapper(nil, serverConfig.Config.LoopbackClientConfig)
	if err := o.Audit.ApplyTo(
		&serverConfig.Config,
		serverConfig.Config.LoopbackClientConfig,
		serverConfig.SharedInformerFactory,
		genericapiserveroptions.NewProcessInfo("template-service-broker", "openshift-template-service-broker"),
		&genericapiserveroptions.WebhookOptions{
			AuthInfoResolverWrapper: authInfoResolverWrapper,
			// the openshift-apiserver runs on cluster as a normal pod, accessed by a service, so it should always have access to the service network
			ServiceResolver: aggregatorapiserver.NewClusterIPServiceResolver(serverConfig.SharedInformerFactory.Core().V1().Services().Lister()),
		},
	); err != nil {
		return nil, err
	}

	if err := o.Features.ApplyTo(&serverConfig.Config); err != nil {
		return nil, err
	}

	serverConfig.EnableMetrics = true

	config := &server.TemplateServiceBrokerConfig{
		GenericConfig: serverConfig,

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

	server, err := config.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}
	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}

// these are used to set up for reading the config
var (
	configScheme = runtime.NewScheme()
	configCodecs = serializer.NewCodecFactory(configScheme)
)

func init() {
	configinstall.Install(configScheme)
}
