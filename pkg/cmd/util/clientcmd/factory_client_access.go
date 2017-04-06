package clientcmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	fedclientset "k8s.io/kubernetes/federation/client/clientset_generated/federation_internalclientset"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kclientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/cmd/set"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/homedir"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploycmd "github.com/openshift/origin/pkg/deploy/cmd"
	imageutil "github.com/openshift/origin/pkg/image/util"
	routegen "github.com/openshift/origin/pkg/route/generator"
)

type ring0Factory struct {
	clientConfig            kclientcmd.ClientConfig
	imageResolutionOptions  FlagBinder
	kubeClientAccessFactory kcmdutil.ClientAccessFactory
}

type ClientAccessFactory interface {
	kcmdutil.ClientAccessFactory

	Clients() (*client.Client, kclientset.Interface, error)
	OpenShiftClientConfig() kclientcmd.ClientConfig
	ImageResolutionOptions() FlagBinder
}

func NewClientAccessFactory(optionalClientConfig kclientcmd.ClientConfig) ClientAccessFactory {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)

	clientConfig := optionalClientConfig
	if optionalClientConfig == nil {
		// TODO: there should be two client configs, one for OpenShift, and one for Kubernetes
		clientConfig = DefaultClientConfig(flags)
		clientConfig = defaultingClientConfig{clientConfig}
	}

	return &ring0Factory{
		clientConfig:            clientConfig,
		imageResolutionOptions:  &imageResolutionOptions{},
		kubeClientAccessFactory: kcmdutil.NewClientAccessFactoryFromDiscovery(flags, clientConfig, &discoveryFactory{clientConfig: clientConfig}),
	}
}

type discoveryFactory struct {
	clientConfig kclientcmd.ClientConfig
}

func (f *discoveryFactory) DiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	// Output using whatever version was negotiated in the client cache. The
	// version we decode with may not be the same as what the server requires.
	cfg, err := f.clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	// at this point we've negotiated and can get the client
	oclient, err := client.New(cfg)
	if err != nil {
		return nil, err
	}

	// TODO: k8s dir is different, I guess we should align
	// cacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery"), cfg.Host)
	cacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube"), cfg.Host)
	return kcmdutil.NewCachedDiscoveryClient(client.NewDiscoveryClient(oclient.RESTClient), cacheDir, time.Duration(10*time.Minute)), nil
}

func DefaultClientConfig(flags *pflag.FlagSet) kclientcmd.ClientConfig {
	loadingRules := config.NewOpenShiftClientConfigLoadingRules()
	flags.StringVar(&loadingRules.ExplicitPath, config.OpenShiftConfigFlagName, "", "Path to the config file to use for CLI requests.")
	cobra.MarkFlagFilename(flags, config.OpenShiftConfigFlagName)

	// set our explicit defaults
	defaultOverrides := &kclientcmd.ConfigOverrides{ClusterDefaults: kclientcmdapi.Cluster{Server: os.Getenv("KUBERNETES_MASTER")}}
	loadingRules.DefaultClientConfig = kclientcmd.NewDefaultClientConfig(kclientcmdapi.Config{}, defaultOverrides)

	overrides := &kclientcmd.ConfigOverrides{ClusterDefaults: defaultOverrides.ClusterDefaults}
	overrideFlags := kclientcmd.RecommendedConfigOverrideFlags("")
	overrideFlags.ContextOverrideFlags.Namespace.ShortName = "n"
	overrideFlags.AuthOverrideFlags.Username.LongName = ""
	overrideFlags.AuthOverrideFlags.Password.LongName = ""
	kclientcmd.BindOverrideFlags(overrides, flags, overrideFlags)
	cobra.MarkFlagFilename(flags, overrideFlags.AuthOverrideFlags.ClientCertificate.LongName)
	cobra.MarkFlagFilename(flags, overrideFlags.AuthOverrideFlags.ClientKey.LongName)
	cobra.MarkFlagFilename(flags, overrideFlags.ClusterOverrideFlags.CertificateAuthority.LongName)

	clientConfig := kclientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	return clientConfig
}

func (f *ring0Factory) Clients() (*client.Client, kclientset.Interface, error) {
	kubeClientSet, err := f.ClientSet()
	if err != nil {
		return nil, nil, err
	}

	cfg, err := f.clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	openShiftClient, err := client.New(cfg)
	if err != nil {
		return nil, nil, err
	}

	return openShiftClient, kubeClientSet, nil
}

func (f *ring0Factory) OpenShiftClientConfig() kclientcmd.ClientConfig {
	return f.clientConfig
}

func (f *ring0Factory) DiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return f.kubeClientAccessFactory.DiscoveryClient()
}

func (f *ring0Factory) ClientSet() (*kclientset.Clientset, error) {
	return f.kubeClientAccessFactory.ClientSet()
}

func (f *ring0Factory) ClientSetForVersion(requiredVersion *unversioned.GroupVersion) (*kclientset.Clientset, error) {
	return f.kubeClientAccessFactory.ClientSetForVersion(requiredVersion)
}

func (f *ring0Factory) ClientConfig() (*restclient.Config, error) {
	return f.kubeClientAccessFactory.ClientConfig()
}

func (f *ring0Factory) ClientConfigForVersion(requiredVersion *unversioned.GroupVersion) (*restclient.Config, error) {
	return f.kubeClientAccessFactory.ClientConfigForVersion(nil)
}

func (f *ring0Factory) RESTClient() (*restclient.RESTClient, error) {
	return f.kubeClientAccessFactory.RESTClient()
}

func (f *ring0Factory) FederationClientSetForVersion(version *unversioned.GroupVersion) (fedclientset.Interface, error) {
	return f.kubeClientAccessFactory.FederationClientSetForVersion(version)
}

func (f *ring0Factory) FederationClientForVersion(version *unversioned.GroupVersion) (*restclient.RESTClient, error) {
	return f.kubeClientAccessFactory.FederationClientForVersion(version)
}

func (f *ring0Factory) Decoder(toInternal bool) runtime.Decoder {
	return f.kubeClientAccessFactory.Decoder(toInternal)
}

func (f *ring0Factory) JSONEncoder() runtime.Encoder {
	return f.kubeClientAccessFactory.JSONEncoder()
}

func (f *ring0Factory) UpdatePodSpecForObject(obj runtime.Object, fn func(*kapi.PodSpec) error) (bool, error) {
	switch t := obj.(type) {
	case *deployapi.DeploymentConfig:
		template := t.Spec.Template
		if template == nil {
			t.Spec.Template = template
			template = &kapi.PodTemplateSpec{}
		}
		return true, fn(&template.Spec)
	default:
		return f.kubeClientAccessFactory.UpdatePodSpecForObject(obj, fn)
	}
}

func (f *ring0Factory) MapBasedSelectorForObject(object runtime.Object) (string, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		return kubectl.MakeLabels(t.Spec.Selector), nil
	default:
		return f.kubeClientAccessFactory.MapBasedSelectorForObject(object)
	}
}

func (f *ring0Factory) PortsForObject(object runtime.Object) ([]string, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		return getPorts(t.Spec.Template.Spec), nil
	default:
		return f.kubeClientAccessFactory.PortsForObject(object)
	}
}

func (f *ring0Factory) ProtocolsForObject(object runtime.Object) (map[string]string, error) {
	switch t := object.(type) {
	case *deployapi.DeploymentConfig:
		return getProtocols(t.Spec.Template.Spec), nil
	default:
		return f.kubeClientAccessFactory.ProtocolsForObject(object)
	}
}

func (f *ring0Factory) LabelsForObject(object runtime.Object) (map[string]string, error) {
	return f.kubeClientAccessFactory.LabelsForObject(object)
}

func (f *ring0Factory) FlagSet() *pflag.FlagSet {
	return f.kubeClientAccessFactory.FlagSet()
}

func (f *ring0Factory) Command() string {
	return f.kubeClientAccessFactory.Command()
}

func (f *ring0Factory) BindFlags(flags *pflag.FlagSet) {
	f.kubeClientAccessFactory.BindFlags(flags)
}

func (f *ring0Factory) BindExternalFlags(flags *pflag.FlagSet) {
	f.kubeClientAccessFactory.BindExternalFlags(flags)
}

func (f *ring0Factory) DefaultResourceFilterOptions(cmd *cobra.Command, withNamespace bool) *kubectl.PrintOptions {
	return f.kubeClientAccessFactory.DefaultResourceFilterOptions(cmd, withNamespace)
}

func (f *ring0Factory) DefaultResourceFilterFunc() kubectl.Filters {
	return f.kubeClientAccessFactory.DefaultResourceFilterFunc()
}

func (f *ring0Factory) SuggestedPodTemplateResources() []unversioned.GroupResource {
	return f.kubeClientAccessFactory.SuggestedPodTemplateResources()
}

// Saves current resource name (or alias if any) in PrintOptions. Once saved, it will not be overwritten by the
// kubernetes resource alias look-up, as it will notice a non-empty value in `options.Kind`
func (f *ring0Factory) Printer(mapping *meta.RESTMapping, options kubectl.PrintOptions) (kubectl.ResourcePrinter, error) {
	if mapping != nil {
		options.Kind = mapping.Resource
		if alias, ok := resourceShortFormFor(mapping.Resource); ok {
			options.Kind = alias
		}
	}
	return describe.NewHumanReadablePrinter(options), nil
}

func (f *ring0Factory) Pauser(info *resource.Info) (bool, error) {
	switch t := info.Object.(type) {
	case *deployapi.DeploymentConfig:
		patches := set.CalculatePatches([]*resource.Info{info}, f.JSONEncoder(), func(*resource.Info) (bool, error) {
			if t.Spec.Paused {
				return false, nil
			}
			t.Spec.Paused = true
			return true, nil
		})
		if len(patches) == 0 {
			return true, nil
		}
		_, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, kapi.StrategicMergePatchType, patches[0].Patch)
		// TODO: Pause the deployer containers.
		return false, err
	default:
		return f.kubeClientAccessFactory.Pauser(info)
	}
}

// ImageResolutionOptions provides the "--source" flag to commands that deal with images
// and need to provide extra capabilities for working with ImageStreamTags and
// ImageStreamImages.
type imageResolutionOptions struct {
	bound  bool
	Source string
}

func (o *imageResolutionOptions) Bound() bool {
	return o.bound
}

func (o *imageResolutionOptions) Bind(f *pflag.FlagSet) {
	if o.Bound() {
		return
	}
	f.StringVarP(&o.Source, "source", "", "istag", "The image source type; valid types are valid values are 'imagestreamtag', 'istag', 'imagestreamimage', 'isimage', and 'docker'")
	o.bound = true
}

func (f *ring0Factory) ImageResolutionOptions() FlagBinder {
	return f.imageResolutionOptions
}

func (f *ring0Factory) ResolveImage(image string) (string, error) {
	options := f.imageResolutionOptions.(*imageResolutionOptions)
	if imageutil.IsDocker(options.Source) {
		return f.kubeClientAccessFactory.ResolveImage(image)
	}
	oc, _, err := f.Clients()
	if err != nil {
		return "", err
	}
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return "", err
	}
	return imageutil.ResolveImagePullSpec(oc, oc, options.Source, image, namespace)
}

func (f *ring0Factory) Resumer(info *resource.Info) (bool, error) {
	switch t := info.Object.(type) {
	case *deployapi.DeploymentConfig:
		patches := set.CalculatePatches([]*resource.Info{info}, f.JSONEncoder(), func(*resource.Info) (bool, error) {
			if !t.Spec.Paused {
				return false, nil
			}
			t.Spec.Paused = false
			return true, nil
		})
		if len(patches) == 0 {
			return true, nil
		}
		_, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, kapi.StrategicMergePatchType, patches[0].Patch)
		// TODO: Resume the deployer containers.
		return false, err
	default:
		return f.kubeClientAccessFactory.Resumer(info)
	}
}

func (f *ring0Factory) DefaultNamespace() (string, bool, error) {
	return f.kubeClientAccessFactory.DefaultNamespace()
}

func DefaultGenerators(cmdName string) map[string]kubectl.Generator {
	generators := map[string]map[string]kubectl.Generator{}
	generators["run"] = map[string]kubectl.Generator{
		"deploymentconfig/v1": deploycmd.BasicDeploymentConfigController{},
		"run-controller/v1":   kubectl.BasicReplicationController{}, // legacy alias for run/v1
	}
	generators["expose"] = map[string]kubectl.Generator{
		"route/v1": routegen.RouteGenerator{},
	}

	return generators[cmdName]
}

func (f *ring0Factory) Generators(cmdName string) map[string]kubectl.Generator {
	originGenerators := DefaultGenerators(cmdName)
	kubeGenerators := f.kubeClientAccessFactory.Generators(cmdName)

	ret := map[string]kubectl.Generator{}
	for k, v := range kubeGenerators {
		ret[k] = v
	}
	for k, v := range originGenerators {
		ret[k] = v
	}
	return ret
}

func (f *ring0Factory) CanBeExposed(kind unversioned.GroupKind) error {
	if deployapi.IsKindOrLegacy("DeploymentConfig", kind) {
		return nil
	}
	return f.kubeClientAccessFactory.CanBeExposed(kind)
}

func (f *ring0Factory) CanBeAutoscaled(kind unversioned.GroupKind) error {
	if deployapi.IsKindOrLegacy("DeploymentConfig", kind) {
		return nil
	}
	return f.kubeClientAccessFactory.CanBeAutoscaled(kind)
}

func (f *ring0Factory) EditorEnvs() []string {
	return []string{"OC_EDITOR", "EDITOR"}
}

func (f *ring0Factory) PrintObjectSpecificMessage(obj runtime.Object, out io.Writer) {}

// defaultingClientConfig detects whether the provided config is the default, and if
// so returns an error that indicates the user should set up their config.
type defaultingClientConfig struct {
	nested kclientcmd.ClientConfig
}

// RawConfig calls the nested method
func (c defaultingClientConfig) RawConfig() (kclientcmdapi.Config, error) {
	return c.nested.RawConfig()
}

// Namespace calls the nested method, and if an empty config error is returned
// it checks for the same default as kubectl - the value of POD_NAMESPACE or
// "default".
func (c defaultingClientConfig) Namespace() (string, bool, error) {
	namespace, ok, err := c.nested.Namespace()
	if err == nil {
		return namespace, ok, nil
	}
	if !kclientcmd.IsEmptyConfig(err) {
		return "", false, err
	}

	// This way assumes you've set the POD_NAMESPACE environment variable using the downward API.
	// This check has to be done first for backwards compatibility with the way InClusterConfig was originally set up
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns, true, nil
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns, true, nil
		}
	}

	return kapi.NamespaceDefault, false, nil
}

// ConfigAccess implements ClientConfig
func (c defaultingClientConfig) ConfigAccess() kclientcmd.ConfigAccess {
	return c.nested.ConfigAccess()
}

type errConfigurationMissing struct {
	err error
}

func (e errConfigurationMissing) Error() string {
	return fmt.Sprintf("%v", e.err)
}

func IsConfigurationMissing(err error) bool {
	switch err.(type) {
	case errConfigurationMissing:
		return true
	}
	return kclientcmd.IsContextNotFound(err)
}

// ClientConfig returns a complete client config
func (c defaultingClientConfig) ClientConfig() (*restclient.Config, error) {
	cfg, err := c.nested.ClientConfig()
	if err == nil {
		return cfg, nil
	}

	if !kclientcmd.IsEmptyConfig(err) {
		return nil, err
	}

	// TODO: need to expose inClusterConfig upstream and use that
	if icc, err := restclient.InClusterConfig(); err == nil {
		glog.V(4).Infof("Using in-cluster configuration")
		return icc, nil
	}

	return nil, errConfigurationMissing{fmt.Errorf(`Missing or incomplete configuration info.  Please login or point to an existing, complete config file:

  1. Via the command-line flag --config
  2. Via the KUBECONFIG environment variable
  3. In your home directory as ~/.kube/config

To view or setup config directly use the 'config' command.`)}
}

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")

	return filepath.Join(parentDir, safeHost)
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive, so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/\.)]`)

func getPorts(spec kapi.PodSpec) []string {
	result := []string{}
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result = append(result, strconv.Itoa(int(port.ContainerPort)))
		}
	}
	return result
}

func getProtocols(spec kapi.PodSpec) map[string]string {
	result := make(map[string]string)
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			result[strconv.Itoa(int(port.ContainerPort))] = string(port.Protocol)
		}
	}
	return result
}
