package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
)

// Factory provides common options for OpenShift commands
type Factory struct {
	*kubecmd.Factory
	OpenShiftClientConfig clientcmd.ClientConfig
}

// NewFactory creates an object that holds common methods across all OpenShift commands
func NewFactory(clientConfig clientcmd.ClientConfig) *Factory {
	mapper := kubectl.ShortcutExpander{latest.RESTMapper}

	w := &Factory{kubecmd.NewFactory(), clientConfig}

	w.Object = func(cmd *cobra.Command) (meta.RESTMapper, runtime.ObjectTyper) {
		version := kubecmd.GetFlagString(cmd, "api-version")
		return kubectl.OutputVersionMapper{mapper, version}, api.Scheme
	}

	w.RESTClient = func(cmd *cobra.Command, mapping *meta.RESTMapping) (resource.RESTClient, error) {
		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			cfg, err := w.OpenShiftClientConfig.ClientConfig()
			if err != nil {
				return nil, fmt.Errorf("unable to find client config %s: %v", mapping.Kind, err)
			}
			cli, err := client.New(cfg)
			if err != nil {
				return nil, fmt.Errorf("unable to create client %s: %v", mapping.Kind, err)
			}
			return cli.RESTClient, nil
		}
		return kubecmd.NewFactory().RESTClient(cmd, mapping)
	}

	w.Describer = func(cmd *cobra.Command, mapping *meta.RESTMapping) (kubectl.Describer, error) {
		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			cfg, err := w.OpenShiftClientConfig.ClientConfig()
			if err != nil {
				return nil, fmt.Errorf("unable to describe %s: %v", mapping.Kind, err)
			}
			cli, err := client.New(cfg)
			if err != nil {
				return nil, fmt.Errorf("unable to describe %s: %v", mapping.Kind, err)
			}
			describer, ok := describe.DescriberFor(mapping.Kind, cli, "")
			if !ok {
				return nil, fmt.Errorf("no description has been implemented for %q", mapping.Kind)
			}
			return describer, nil
		}
		return kubecmd.NewFactory().Describer(cmd, mapping)
	}

	w.Printer = func(cmd *cobra.Command, mapping *meta.RESTMapping, noHeaders bool) (kubectl.ResourcePrinter, error) {
		return describe.NewHumanReadablePrinter(noHeaders), nil
	}

	return w
}

// Clients returns an OpenShift and Kubernetes client.
func (f *Factory) Clients(cmd *cobra.Command) (*client.Client, *kclient.Client, error) {
	os, err := f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	oc, err := client.New(os)
	if err != nil {
		return nil, nil, err
	}
	kc, err := f.Client(cmd)
	if err != nil {
		return nil, nil, err
	}
	return oc, kc, nil
}
