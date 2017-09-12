package server

import (
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"

	admissionv1alpha1 "k8s.io/api/admission/v1alpha1"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"

	"github.com/openshift/openshift-project-reservation/pkg/apiserver"
)

const defaultEtcdPathPrefix = "/registry/projectreservation.openshift.io"

type ProjectReservationServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	StdOut io.Writer
	StdErr io.Writer
}

func NewProjectReservationServerOptions(out, errOut io.Writer) *ProjectReservationServerOptions {
	o := &ProjectReservationServerOptions{
		// TODO we will nil out the etcd storage options.  This requires a later level of k8s.io/apiserver
		RecommendedOptions: genericoptions.NewRecommendedOptions(defaultEtcdPathPrefix, apiserver.Scheme, apiserver.Codecs.LegacyCodec(admissionv1alpha1.SchemeGroupVersion)),

		StdOut: out,
		StdErr: errOut,
	}

	return o
}

// NewCommandStartMaster provides a CLI handler for 'start master' command
func NewCommandStartProjectReservationServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := NewProjectReservationServerOptions(out, errOut)

	cmd := &cobra.Command{
		Short: "Launch a project reservation API server",
		Long:  "Launch a project reservation API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunProjectReservationServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)

	return cmd
}

func (o ProjectReservationServerOptions) Validate(args []string) error {
	return nil
}

func (o *ProjectReservationServerOptions) Complete() error {
	return nil
}

func (o ProjectReservationServerOptions) Config() (*apiserver.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(apiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	config := &apiserver.Config{
		GenericConfig: serverConfig,
	}
	return config, nil
}

func (o ProjectReservationServerOptions) RunProjectReservationServer(stopCh <-chan struct{}) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}
	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}
