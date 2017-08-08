package server

import (
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"

	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"

	"github.com/openshift/origin/pkg/openservicebroker/server"
)

const defaultEtcdPathPrefix = "/registry/templateservicebroker.openshift.io"

type TemplateServiceBrokerServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	StdOut io.Writer
	StdErr io.Writer

	TemplateNamespaces []string
}

func NewTemplateServiceBrokerServerOptions(out, errOut io.Writer) *TemplateServiceBrokerServerOptions {
	o := &TemplateServiceBrokerServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(defaultEtcdPathPrefix, server.Scheme, server.Codecs.LegacyCodec()),

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
			if err := o.Complete(); err != nil {
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
	o.RecommendedOptions.AddFlags(flags)
	flags.StringSliceVar(&o.TemplateNamespaces, "template-namespace", o.TemplateNamespaces, "TemplateNamespaces indicates the namespace(s) in which the template service broker looks for templates to serve to the catalog.")

	return cmd
}

func (o TemplateServiceBrokerServerOptions) Validate(args []string) error {
	return nil
}

func (o *TemplateServiceBrokerServerOptions) Complete() error {
	return nil
}

func (o TemplateServiceBrokerServerOptions) Config() (*server.TemplateServiceBrokerConfig, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(server.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	config := &server.TemplateServiceBrokerConfig{
		GenericConfig: serverConfig,

		TemplateNamespaces: o.TemplateNamespaces,
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
