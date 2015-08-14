package clientcmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	kclient "k8s.io/kubernetes/pkg/client"
	kclientcmd "k8s.io/kubernetes/pkg/client/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployreaper "github.com/openshift/origin/pkg/deploy/reaper"
	deployscaler "github.com/openshift/origin/pkg/deploy/scaler"
	routegen "github.com/openshift/origin/pkg/route/generator"
)

// New creates a default Factory for commands that should share identical server
// connection behavior. Most commands should use this method to get a factory.
func New(flags *pflag.FlagSet) *Factory {
	// Override global default to "" so we force the client to ask for user input
	// TODO refactor this upstream:
	// DefaultCluster should not be a global
	// A call to ClientConfig() should always return the best clientCfg possible
	// even if an error was returned, and let the caller decide what to do
	kclientcmd.DefaultCluster.Server = ""

	// TODO: there should be two client configs, one for OpenShift, and one for Kubernetes
	clientConfig := DefaultClientConfig(flags)
	f := NewFactory(clientConfig)
	f.BindFlags(flags)

	return f
}

// Factory provides common options for OpenShift commands
type Factory struct {
	*cmdutil.Factory
	OpenShiftClientConfig kclientcmd.ClientConfig
	clients               *clientCache
}

// NewFactory creates an object that holds common methods across all OpenShift commands
func NewFactory(clientConfig kclientcmd.ClientConfig) *Factory {
	mapper := ShortcutExpander{kubectl.ShortcutExpander{latest.RESTMapper}}

	clients := &clientCache{
		clients: make(map[string]*client.Client),
		loader:  clientConfig,
	}

	generators := map[string]kubectl.Generator{
		"route/v1": routegen.RouteGenerator{},
	}

	w := &Factory{
		Factory:               cmdutil.NewFactory(clientConfig),
		OpenShiftClientConfig: clientConfig,
		clients:               clients,
	}

	w.Object = func() (meta.RESTMapper, runtime.ObjectTyper) {
		if cfg, err := clientConfig.ClientConfig(); err == nil {
			return kubectl.OutputVersionMapper{mapper, cfg.Version}, api.Scheme
		}
		return mapper, api.Scheme
	}

	kRESTClient := w.Factory.RESTClient
	w.RESTClient = func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			client, err := clients.ClientForVersion(mapping.APIVersion)
			if err != nil {
				return nil, err
			}
			return client.RESTClient, nil
		}
		return kRESTClient(mapping)
	}

	// Save original Describer function
	kDescriberFunc := w.Factory.Describer
	w.Describer = func(mapping *meta.RESTMapping) (kubectl.Describer, error) {
		if latest.OriginKind(mapping.Kind, mapping.APIVersion) {
			oClient, kClient, err := w.Clients()
			if err != nil {
				return nil, fmt.Errorf("unable to create client %s: %v", mapping.Kind, err)
			}

			cfg, err := clients.ClientConfigForVersion(mapping.APIVersion)
			if err != nil {
				return nil, fmt.Errorf("unable to load a client %s: %v", mapping.Kind, err)
			}

			describer, ok := describe.DescriberFor(mapping.Kind, oClient, kClient, cfg.Host)
			if !ok {
				return nil, fmt.Errorf("no description has been implemented for %q", mapping.Kind)
			}
			return describer, nil
		}
		return kDescriberFunc(mapping)
	}
	kScalerFunc := w.Factory.Scaler
	w.Scaler = func(mapping *meta.RESTMapping) (kubectl.Scaler, error) {
		oc, kc, err := w.Clients()
		if err != nil {
			return nil, err
		}

		if mapping.Kind == "DeploymentConfig" {
			return deployscaler.NewDeploymentConfigScaler(oc, kc), nil
		}
		return kScalerFunc(mapping)
	}
	kReaperFunc := w.Factory.Reaper
	w.Reaper = func(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
		oc, kc, err := w.Clients()
		if err != nil {
			return nil, err
		}

		if mapping.Kind == "DeploymentConfig" {
			return deployreaper.NewDeploymentConfigReaper(oc, kc), nil
		}
		return kReaperFunc(mapping)
	}
	// Override the Kubernetes Generator and add support for routes
	kGeneratorFunc := w.Factory.Generator
	w.Generator = func(name string) (kubectl.Generator, bool) {
		if generator, ok := generators[name]; ok {
			return generator, true
		}
		return kGeneratorFunc(name)
	}
	kPodSelectorForObjectFunc := w.Factory.PodSelectorForObject
	w.PodSelectorForObject = func(object runtime.Object) (string, error) {
		switch t := object.(type) {
		case *deployapi.DeploymentConfig:
			return kubectl.MakeLabels(t.Template.ControllerTemplate.Selector), nil
		default:
			return kPodSelectorForObjectFunc(object)
		}
	}
	kPortsForObjectFunc := w.Factory.PortsForObject
	w.PortsForObject = func(object runtime.Object) ([]string, error) {
		switch t := object.(type) {
		case *deployapi.DeploymentConfig:
			return getPorts(t.Template.ControllerTemplate.Template.Spec), nil
		default:
			return kPortsForObjectFunc(object)
		}
	}
	kLogsForObjectFunc := w.Factory.LogsForObject
	w.LogsForObject = func(object runtime.Object, opts *api.PodLogOptions) (io.ReadCloser, error) {
		client, err := clients.ClientForVersion("")
		if err != nil {
			return nil, err
		}

		switch t := object.(type) {
		case *buildapi.Build:
			opt := buildapi.BuildLogOptions{Follow: opts.Follow}
			return client.BuildLogs(t.Namespace).Get(t.Name, opt).Stream()
		default:
			return kLogsForObjectFunc(object, opts)
		}
	}
	w.Printer = func(mapping *meta.RESTMapping, noHeaders, withNamespace, wide bool, columnLabels []string) (kubectl.ResourcePrinter, error) {
		return describe.NewHumanReadablePrinter(noHeaders, withNamespace, wide, columnLabels), nil
	}

	return w
}

func getPorts(spec api.PodSpec) []string {
	result := []string{}
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result = append(result, strconv.Itoa(port.ContainerPort))
		}
	}
	return result
}

// UpdatePodSpecForObject update the pod specification for the provided object
// TODO: move to upstream
func (f *Factory) UpdatePodSpecForObject(obj runtime.Object, fn func(*api.PodSpec) error) (bool, error) {
	// TODO: replace with a swagger schema based approach (identify pod template via schema introspection)
	switch t := obj.(type) {
	case *api.Pod:
		return true, fn(&t.Spec)
	case *api.PodTemplate:
		return true, fn(&t.Template.Spec)
	case *api.ReplicationController:
		if t.Spec.Template == nil {
			t.Spec.Template = &api.PodTemplateSpec{}
		}
		return true, fn(&t.Spec.Template.Spec)
	case *deployapi.DeploymentConfig:
		template := t.Template.ControllerTemplate
		if template.Template == nil {
			template.Template = &api.PodTemplateSpec{}
		}
		return true, fn(&template.Template.Spec)
	default:
		return false, fmt.Errorf("the object is not a pod or does not have a pod template")
	}
}

// Clients returns an OpenShift and Kubernetes client.
func (f *Factory) Clients() (*client.Client, *kclient.Client, error) {
	kClient, err := f.Client()
	if err != nil {
		return nil, nil, err
	}
	osClient, err := f.clients.ClientForVersion("")
	if err != nil {
		return nil, nil, err
	}
	return osClient, kClient, nil
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

// AliasesForResource returns whether a resource has an alias or not
func (e ShortcutExpander) AliasesForResource(resource string) ([]string, bool) {
	aliases := map[string][]string{
		"all": latest.UserResources,
	}

	if res, ok := aliases[resource]; ok {
		return res, true
	}
	return nil, false
}

// expandResourceShortcut will return the expanded version of resource
// (something that a pkg/api/meta.RESTMapper can understand), if it is
// indeed a shortcut. Otherwise, will return resource unmodified.
func expandResourceShortcut(resource string) string {
	shortForms := map[string]string{
		"dc":      "deploymentConfigs",
		"bc":      "buildConfigs",
		"is":      "imageStreams",
		"istag":   "imageStreamTags",
		"isimage": "imageStreamImages",
		"sa":      "serviceAccounts",
		"pv":      "persistentVolumes",
		"pvc":     "persistentVolumeClaims",
	}
	if expanded, ok := shortForms[resource]; ok {
		return expanded
	}
	return resource
}

// clientCache caches previously loaded clients for reuse, and ensures MatchServerVersion
// is invoked only once
type clientCache struct {
	loader        kclientcmd.ClientConfig
	clients       map[string]*client.Client
	defaultConfig *kclient.Config
}

// ClientConfigForVersion returns the correct config for a server
func (c *clientCache) ClientConfigForVersion(version string) (*kclient.Config, error) {
	if c.defaultConfig == nil {
		config, err := c.loader.ClientConfig()
		if err != nil {
			return nil, err
		}
		c.defaultConfig = config
	}
	// TODO: have a better config copy method
	config := *c.defaultConfig
	if len(version) != 0 {
		config.Version = version
	}
	client.SetOpenShiftDefaults(&config)

	return &config, nil
}

// ClientForVersion initializes or reuses a client for the specified version, or returns an
// error if that is not possible
func (c *clientCache) ClientForVersion(version string) (*client.Client, error) {
	config, err := c.ClientConfigForVersion(version)
	if err != nil {
		return nil, err
	}

	if client, ok := c.clients[config.Version]; ok {
		return client, nil
	}

	client, err := client.New(config)
	if err != nil {
		return nil, err
	}

	c.clients[config.Version] = client
	return client, nil
}
