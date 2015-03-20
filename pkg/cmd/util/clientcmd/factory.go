package clientcmd

import (
	"fmt"
	"net/http"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util"

	"github.com/spf13/pflag"
)

// NewFactory creates a default Factory for commands that should share identical server
// connection behavior. Most commands should use this method to get a factory.
func New(flags *pflag.FlagSet) *Factory {
	// Override global default to "" so we force the client to ask for user input
	// TODO refactor this usptream:
	// DefaultCluster should not be a global
	// A call to ClientConfig() should always return the best clientCfg possible
	// even if an error was returned, and let the caller decide what to do
	kclientcmd.DefaultCluster.Server = ""

	// TODO: there should be two client configs, one for OpenShift, and one for Kubernetes
	clientConfig := util.DefaultClientConfig(flags)
	f := NewFactory(clientConfig)
	f.BindFlags(flags)

	return f
}

// Factory provides common options for OpenShift commands
type Factory struct {
	*kubecmd.Factory
	OpenShiftClientConfig kclientcmd.ClientConfig
}

// NewFactory creates an object that holds common methods across all OpenShift commands
func NewFactory(clientConfig kclientcmd.ClientConfig) *Factory {
	mapper := ShortcutExpander{kubectl.ShortcutExpander{latest.RESTMapper}}

	w := &Factory{kubecmd.NewFactory(clientConfig), clientConfig}

	w.Object = func() (meta.RESTMapper, runtime.ObjectTyper) {
		return mapper, api.Scheme
	}

	w.RESTClient = func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
		oClient, kClient, err := w.Clients()
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.Kind, err)
		}

		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			return oClient.RESTClient, nil
		} else {
			return kClient.RESTClient, nil
		}
	}

	w.Describer = func(mapping *meta.RESTMapping) (kubectl.Describer, error) {
		oClient, kClient, err := w.Clients()
		if err != nil {
			return nil, fmt.Errorf("unable to create client %s: %v", mapping.Kind, err)
		}

		cfg, err := w.OpenShiftClientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("unable to describe %s: %v", mapping.Kind, err)
		}

		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			describer, ok := describe.DescriberFor(mapping.Kind, oClient, kClient, cfg.Host)
			if !ok {
				return nil, fmt.Errorf("no description has been implemented for %q", mapping.Kind)
			}
			return describer, nil
		}
		return w.Factory.Describer(mapping)
	}

	w.Printer = func(mapping *meta.RESTMapping, noHeaders bool) (kubectl.ResourcePrinter, error) {
		return describe.NewHumanReadablePrinter(noHeaders), nil
	}

	w.DefaultNamespace = func() (string, error) {
		return w.OpenShiftClientConfig.Namespace()
	}

	return w
}

// Clients returns an OpenShift and Kubernetes client.
func (f *Factory) Clients() (*client.Client, *kclient.Client, error) {
	cfg, err := f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	transport, err := kclient.TransportFor(cfg)
	if err != nil {
		return nil, nil, err
	}
	httpClient := &http.Client{
		Transport: transport,
	}

	oClient, err := client.New(cfg)
	if err != nil {
		return nil, nil, err
	}
	kClient, err := kclient.New(cfg)
	if err != nil {
		return nil, nil, err
	}

	oClient.Client = &statusHandlerClient{httpClient}
	kClient.Client = &statusHandlerClient{httpClient}

	return oClient, kClient, nil
}

// ShortcutExpander is a RESTMapper that can be used for OpenShift resources.
type ShortcutExpander struct {
	meta.RESTMapper
}

// VersionAndKindForResource implements meta.RESTMapper. It expands the resource first, then invokes the wrapped
// mapper.
func (e ShortcutExpander) VersionAndKindForResource(resource string) (defaultVersion, kind string, err error) {
	resource = expandResourceShortcut(resource)
	return e.RESTMapper.VersionAndKindForResource(resource)
}

// expandResourceShortcut will return the expanded version of resource
// (something that a pkg/api/meta.RESTMapper can understand), if it is
// indeed a shortcut. Otherwise, will return resource unmodified.
func expandResourceShortcut(resource string) string {
	shortForms := map[string]string{
		"dc": "deploymentConfigs",
		"bc": "buildConfigs",
	}
	if expanded, ok := shortForms[resource]; ok {
		return expanded
	}
	return resource
}
