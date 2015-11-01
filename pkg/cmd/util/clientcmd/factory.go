package clientcmd

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploygen "github.com/openshift/origin/pkg/deploy/generator"
	deployreaper "github.com/openshift/origin/pkg/deploy/reaper"
	deployscaler "github.com/openshift/origin/pkg/deploy/scaler"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
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
	mapper := ShortcutExpander{RESTMapper: kubectl.ShortcutExpander{RESTMapper: latest.RESTMapper}}

	clients := &clientCache{
		clients: make(map[string]*client.Client),
		configs: make(map[string]*kclient.Config),
		loader:  clientConfig,
	}

	generators := map[string]kubectl.Generator{
		"route/v1":          routegen.RouteGenerator{},
		"run/v1":            deploygen.BasicDeploymentConfigController{},
		"run-controller/v1": kubectl.BasicReplicationController{},
	}

	w := &Factory{
		Factory:               cmdutil.NewFactory(clientConfig),
		OpenShiftClientConfig: clientConfig,
		clients:               clients,
	}

	w.Object = func() (meta.RESTMapper, runtime.ObjectTyper) {
		// Output using whatever version was negotiated in the client cache. The
		// version we decode with may not be the same as what the server requires.
		if cfg, err := clients.ClientConfigForVersion(""); err == nil {
			return kubectl.OutputVersionMapper{RESTMapper: mapper, OutputVersion: cfg.Version}, api.Scheme
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
	w.LogsForObject = func(object, options runtime.Object) (*kclient.Request, error) {
		oc, _, err := w.Clients()
		if err != nil {
			return nil, err
		}

		switch t := object.(type) {
		case *deployapi.DeploymentConfig:
			dopts, ok := options.(*deployapi.DeploymentLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a DeploymentLogOptions")
			}
			return oc.DeploymentLogs(t.Namespace).Get(t.Name, *dopts), nil
		case *buildapi.Build:
			bopts, ok := options.(*buildapi.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a BuildLogOptions")
			}
			if bopts.Version != nil {
				// should --version work with builds at all?
				return nil, errors.New("cannot specify a version and a build")
			}
			return oc.BuildLogs(t.Namespace).Get(t.Name, *bopts), nil
		case *buildapi.BuildConfig:
			bopts, ok := options.(*buildapi.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a BuildLogOptions")
			}
			buildsForBCSelector := labels.SelectorFromSet(map[string]string{buildapi.DeprecatedBuildConfigLabel: t.Name})
			builds, err := oc.Builds(t.Namespace).List(buildsForBCSelector, fields.Everything())
			if err != nil {
				return nil, err
			}
			if len(builds.Items) == 0 {
				return nil, fmt.Errorf("no builds found for %s", t.Name)
			}
			if bopts.Version != nil {
				// If a version has been specified, try to get the logs from that build.
				desired := buildutil.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
				return oc.BuildLogs(t.Namespace).Get(desired, *bopts), nil
			}
			sort.Sort(sort.Reverse(buildapi.BuildSliceByCreationTimestamp(builds.Items)))
			return oc.BuildLogs(t.Namespace).Get(builds.Items[0].Name, *bopts), nil
		default:
			return kLogsForObjectFunc(object, options)
		}
	}
	w.Printer = func(mapping *meta.RESTMapping, noHeaders, withNamespace, wide bool, showAll bool, columnLabels []string) (kubectl.ResourcePrinter, error) {
		return describe.NewHumanReadablePrinter(noHeaders, withNamespace, wide, showAll, columnLabels), nil
	}
	kCanBeExposed := w.Factory.CanBeExposed
	w.CanBeExposed = func(kind string) error {
		if kind == "DeploymentConfig" {
			return nil
		}
		return kCanBeExposed(kind)
	}
	kAttachablePodForObjectFunc := w.Factory.AttachablePodForObject
	w.AttachablePodForObject = func(object runtime.Object) (*api.Pod, error) {
		oc, kc, err := w.Clients()
		if err != nil {
			return nil, err
		}
		switch t := object.(type) {
		case *deployapi.DeploymentConfig:
			var err error
			var pods *api.PodList
			for pods == nil || len(pods.Items) == 0 {
				if t.LatestVersion == 0 {
					time.Sleep(2 * time.Second)
				}
				if t, err = oc.DeploymentConfigs(t.Namespace).Get(t.Name); err != nil {
					return nil, err
				}
				latestDeploymentName := deployutil.LatestDeploymentNameForConfig(t)
				deployment, err := kc.ReplicationControllers(t.Namespace).Get(latestDeploymentName)
				if err != nil {
					if kerrors.IsNotFound(err) {
						continue
					}
					return nil, err
				}
				pods, err = kc.Pods(deployment.Namespace).List(labels.SelectorFromSet(deployment.Spec.Selector), fields.Everything())
				if err != nil {
					return nil, err
				}
				if len(pods.Items) == 0 {
					time.Sleep(2 * time.Second)
				}
			}
			var oldestPod *api.Pod
			for _, pod := range pods.Items {
				if oldestPod == nil || pod.CreationTimestamp.Before(oldestPod.CreationTimestamp) {
					oldestPod = &pod
				}
			}
			return oldestPod, nil
		default:
			return kAttachablePodForObjectFunc(object)
		}
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

// ResourceIsValid takes a string (kind) and checks if it's a valid resource.
// It expands the resource first, then invokes the wrapped mapper.
func (e ShortcutExpander) ResourceIsValid(resource string) bool {
	return e.RESTMapper.ResourceIsValid(expandResourceShortcut(resource))
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

// clientCache caches previously loaded clients for reuse. This is largely
// copied from upstream (because of typing) but reuses the negotiation logic.
// TODO: Consolidate this entire concept with upstream's ClientCache.
type clientCache struct {
	loader        kclientcmd.ClientConfig
	clients       map[string]*client.Client
	configs       map[string]*kclient.Config
	defaultConfig *kclient.Config
	// negotiatingClient is used only for negotiating versions with the server.
	negotiatingClient *kclient.Client
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
	if config, ok := c.configs[version]; ok {
		return config, nil
	}
	if c.negotiatingClient == nil {
		// TODO: We want to reuse the upstream negotiation logic, which is coupled
		// to a concrete kube Client. The negotiation will ultimately try and
		// build an unversioned URL using the config prefix to ask for supported
		// server versions. If we use the default kube client config, the prefix
		// will be /api, while we need to use the OpenShift prefix to ask for the
		// OpenShift server versions. For now, set OpenShift defaults on the
		// config to ensure the right prefix gets used. The client cache and
		// negotiation logic should be refactored upstream to support downstream
		// reuse so that we don't need to do any of this cache or negotiation
		// duplication.
		negotiatingConfig := *c.defaultConfig
		client.SetOpenShiftDefaults(&negotiatingConfig)
		negotiatingClient, err := kclient.New(&negotiatingConfig)
		if err != nil {
			return nil, err
		}
		c.negotiatingClient = negotiatingClient
	}
	config := *c.defaultConfig
	negotiatedVersion, err := kclient.NegotiateVersion(c.negotiatingClient, &config, version, latest.Versions)
	if err != nil {
		return nil, err
	}
	config.Version = negotiatedVersion
	client.SetOpenShiftDefaults(&config)
	c.configs[version] = &config

	return &config, nil
}

// ClientForVersion initializes or reuses a client for the specified version, or returns an
// error if that is not possible
func (c *clientCache) ClientForVersion(version string) (*client.Client, error) {
	if client, ok := c.clients[version]; ok {
		return client, nil
	}
	config, err := c.ClientConfigForVersion(version)
	if err != nil {
		return nil, err
	}
	client, err := client.New(config)
	if err != nil {
		return nil, err
	}

	c.clients[config.Version] = client
	return client, nil
}
