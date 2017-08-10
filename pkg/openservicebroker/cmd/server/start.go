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

type TemplateServiceBrokerServerOptions struct {
	// we don't have any storage, so we shouldn't use the recommended options
	SecureServing  *genericoptions.SecureServingOptions
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Audit          *genericoptions.AuditOptions
	Features       *genericoptions.FeatureOptions

	StdOut io.Writer
	StdErr io.Writer

	TemplateNamespaces []string
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
	o.SecureServing.AddFlags(flags)
	o.Authentication.AddFlags(flags)
	o.Authorization.AddFlags(flags)
	o.Audit.AddFlags(flags)
	o.Features.AddFlags(flags)
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
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(server.Codecs)
	if err := o.SecureServing.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	// TODO restore this after https://github.com/openshift/openshift-ansible/issues/5056 is fixed
	//if err := o.Authentication.ApplyTo(serverConfig); err != nil {
	//	return nil, err
	//}
	// the TSB server *can* limp along without terminating client certs or front proxy authn. Do that for now
	// this wiring is a bit tricky.
	cfg, err := o.Authentication.ToAuthenticationConfig()
	if err != nil {
		return nil, err
	}
	authenticator, _, err := cfg.New()
	if err != nil {
		return nil, err
	}
	serverConfig.Authenticator = authenticator

	if err := o.Authorization.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	if err := o.Audit.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	if err := o.Features.ApplyTo(serverConfig); err != nil {
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
