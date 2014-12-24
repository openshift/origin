package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
)

// Factory provides common options for OpenShift commands
type Factory struct {
	*kubecmd.Factory

	OpenShiftClientBuilder clientcmd.Builder
}

// NewFactory creates an object that holds common methods across all OpenShift commands
func NewFactory(builder clientcmd.Builder) *Factory {
	w := wrapper{kubecmd.NewFactory(builder), builder}

	return &Factory{
		Factory: &kubecmd.Factory{
			ClientBuilder: builder,
			Mapper:        latest.RESTMapper,
			Typer:         kapi.Scheme,
			Client:        w.Client,
			Describer:     w.Describer,
			Printer:       w.Printer,
			Validator: func(cmd *cobra.Command) (validation.Schema, error) {
				return validation.NullSchema{}, nil
			},
		},
		// TODO: this should use a URL to the OpenShift server and api version, distinct
		// from the Kubernetes client URL and api version (the two versions are distinct)
		OpenShiftClientBuilder: builder,
	}
}

// Clients returns an OpenShift and Kubernetes client.
func (f *Factory) Clients(cmd *cobra.Command) (*client.Client, *kclient.Client, error) {
	kube, err := f.ClientBuilder.Config()
	if err != nil {
		return nil, nil, err
	}
	os, err := f.OpenShiftClientBuilder.Config()
	if err != nil {
		return nil, nil, err
	}
	oc, err := client.New(os)
	if err != nil {
		return nil, nil, err
	}
	kc, err := kclient.New(kube)
	if err != nil {
		return nil, nil, err
	}
	return oc, kc, nil
}

// wrapper provides methods that wrap an underlying kubecmd.Factory with OpenShift types
type wrapper struct {
	factory   *kubecmd.Factory
	osBuilder clientcmd.Builder
}

// Printer exposes the OpenShift types
func (f *wrapper) Printer(cmd *cobra.Command, mapping *kmeta.RESTMapping, noHeaders bool) (kubectl.ResourcePrinter, error) {
	return describe.NewHumanReadablePrinter(noHeaders), nil
}

// Client provides a REST client suitable for managing the given RESTMapping.
func (f *wrapper) Client(cmd *cobra.Command, m *kmeta.RESTMapping) (kubectl.RESTClient, error) {
	if latest.OriginKind(m.Kind, m.APIVersion) {
		cfg, err := f.osBuilder.Config()
		if err != nil {
			return nil, err
		}
		c, err := client.New(cfg)
		if err != nil {
			return nil, err
		}
		return c.RESTClient, nil
	}
	return f.factory.Client(cmd, m)
}

// Describer provides pretty-printed output about both OpenShift and Kubernetes types.
func (f *wrapper) Describer(cmd *cobra.Command, m *kmeta.RESTMapping) (kubectl.Describer, error) {
	if latest.OriginKind(m.Kind, m.APIVersion) {
		cfg, err := f.osBuilder.Config()
		if err != nil {
			return nil, fmt.Errorf("unable to describe %s: %v", m.Kind, err)
		}
		cli, err := client.New(cfg)
		if err != nil {
			return nil, fmt.Errorf("unable to describe %s: %v", m.Kind, err)
		}
		if describer, ok := describe.DescriberFor(m.Kind, cli, ""); ok {
			return describer, nil
		}
	}
	return f.factory.Describer(cmd, m)
}
