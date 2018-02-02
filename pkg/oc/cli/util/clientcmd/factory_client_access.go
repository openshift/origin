package clientcmd

import (
	"errors"
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
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	osclientcmd "github.com/openshift/origin/pkg/client/cmd"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	appsapiv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	deploymentcmd "github.com/openshift/origin/pkg/oc/cli/deploymentconfigs"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	routegen "github.com/openshift/origin/pkg/route/generator"
)

type ring0Factory struct {
	*OpenshiftCLIClientBuilder

	clientConfig            kclientcmd.ClientConfig
	imageResolutionOptions  FlagBinder
	kubeClientAccessFactory kcmdutil.ClientAccessFactory
}

type ClientAccessFactory interface {
	kcmdutil.ClientAccessFactory
	CLIClientBuilder

	OpenShiftClientConfig() kclientcmd.ClientConfig
	ImageResolutionOptions() FlagBinder
}

func NewClientAccessFactory(optionalClientConfig kclientcmd.ClientConfig) ClientAccessFactory {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	clientConfig := optionalClientConfig
	if optionalClientConfig == nil {
		// TODO: there should be two client configs, one for OpenShift, and one for Kubernetes
		clientConfig = osclientcmd.DefaultClientConfig(flags)
		clientConfig = defaultingClientConfig{clientConfig}
	}
	factory := &ring0Factory{
		clientConfig:           clientConfig,
		imageResolutionOptions: &imageResolutionOptions{},
	}
	factory.kubeClientAccessFactory = kcmdutil.NewClientAccessFactoryFromDiscovery(
		flags,
		clientConfig,
		&discoveryFactory{clientConfig: clientConfig},
	)
	factory.OpenshiftCLIClientBuilder = &OpenshiftCLIClientBuilder{config: clientConfig}

	return factory
}

type discoveryFactory struct {
	clientConfig kclientcmd.ClientConfig
	cacheDir     string
}

func (f *discoveryFactory) BindFlags(flags *pflag.FlagSet) {
	defaultCacheDir := filepath.Join(homedir.HomeDir(), ".kube", "http-cache")
	flags.StringVar(&f.cacheDir, kcmdutil.FlagHTTPCacheDir, defaultCacheDir, "Default HTTP cache directory")
}

func (f *discoveryFactory) DiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	// Output using whatever version was negotiated in the client cache. The
	// version we decode with may not be the same as what the server requires.
	cfg, err := f.clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	// given 25 groups with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	cfg.Burst = 100
	cfg.CacheDir = f.cacheDir

	// at this point we've negotiated and can get the client
	kubeClient, err := kclientset.NewForConfig(cfg)
	if err != nil {
		return nil, err

	}

	// TODO: k8s dir is different, I guess we should align
	// cacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery"), cfg.Host)
	cacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube"), cfg.Host)
	return kcmdutil.NewCachedDiscoveryClient(newLegacyDiscoveryClient(kubeClient.Discovery().RESTClient()), cacheDir, time.Duration(10*time.Minute)), nil
}

func (f *ring0Factory) OpenShiftClientConfig() kclientcmd.ClientConfig {
	return f.clientConfig
}

func (f *ring0Factory) DiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return f.kubeClientAccessFactory.DiscoveryClient()
}

func (f *ring0Factory) KubernetesClientSet() (*kubernetes.Clientset, error) {
	return f.kubeClientAccessFactory.KubernetesClientSet()
}

func (f *ring0Factory) ClientSet() (kclientset.Interface, error) {
	return f.kubeClientAccessFactory.ClientSet()
}

func (f *ring0Factory) ClientSetForVersion(requiredVersion *schema.GroupVersion) (kclientset.Interface, error) {
	return f.kubeClientAccessFactory.ClientSetForVersion(requiredVersion)
}

func (f *ring0Factory) ClientConfig() (*restclient.Config, error) {
	return f.kubeClientAccessFactory.ClientConfig()
}

func (f *ring0Factory) BareClientConfig() (*restclient.Config, error) {
	return f.clientConfig.ClientConfig()
}

func (f *ring0Factory) ClientConfigForVersion(requiredVersion *schema.GroupVersion) (*restclient.Config, error) {
	return f.kubeClientAccessFactory.ClientConfigForVersion(nil)
}

func (f *ring0Factory) RESTClient() (*restclient.RESTClient, error) {
	return f.kubeClientAccessFactory.RESTClient()
}

func (f *ring0Factory) Decoder(toInternal bool) runtime.Decoder {
	return f.kubeClientAccessFactory.Decoder(toInternal)
}

func (f *ring0Factory) JSONEncoder() runtime.Encoder {
	return f.kubeClientAccessFactory.JSONEncoder()
}

func (f *ring0Factory) UpdatePodSpecForObject(obj runtime.Object, fn func(*corev1.PodSpec) error) (bool, error) {
	switch t := obj.(type) {
	case *appsapi.DeploymentConfig:
		template := t.Spec.Template
		if template == nil {
			t.Spec.Template = template
			template = &kapi.PodTemplateSpec{}
		}
		if err := ConvertExteralPodSpecToInternal(fn)(&template.Spec); err != nil {
			return true, err
		}
		return true, nil

	case *appsapiv1.DeploymentConfig:
		template := t.Spec.Template
		if template == nil {
			template = &corev1.PodTemplateSpec{}
			t.Spec.Template = template
		}
		return true, fn(&template.Spec)

	default:
		return f.kubeClientAccessFactory.UpdatePodSpecForObject(obj, fn)
	}
}

func ConvertInteralPodSpecToExternal(inFn func(*kapi.PodSpec) error) func(*corev1.PodSpec) error {
	return func(specToMutate *corev1.PodSpec) error {
		internalPodSpec := &kapi.PodSpec{}
		if err := legacyscheme.Scheme.Convert(specToMutate, internalPodSpec, nil); err != nil {
			return err
		}
		if err := inFn(internalPodSpec); err != nil {
			return err
		}
		externalPodSpec := &corev1.PodSpec{}
		if err := legacyscheme.Scheme.Convert(internalPodSpec, externalPodSpec, nil); err != nil {
			return err
		}
		*specToMutate = *externalPodSpec
		return nil
	}
}

func ConvertExteralPodSpecToInternal(inFn func(*corev1.PodSpec) error) func(*kapi.PodSpec) error {
	return func(specToMutate *kapi.PodSpec) error {
		externalPodSpec := &corev1.PodSpec{}
		if err := legacyscheme.Scheme.Convert(specToMutate, externalPodSpec, nil); err != nil {
			return err
		}
		if err := inFn(externalPodSpec); err != nil {
			return err
		}
		internalPodSpec := &kapi.PodSpec{}
		if err := legacyscheme.Scheme.Convert(externalPodSpec, internalPodSpec, nil); err != nil {
			return err
		}
		*specToMutate = *internalPodSpec
		return nil
	}
}

func (f *ring0Factory) MapBasedSelectorForObject(object runtime.Object) (string, error) {
	switch t := object.(type) {
	case *appsapi.DeploymentConfig:
		return kubectl.MakeLabels(t.Spec.Selector), nil
	default:
		return f.kubeClientAccessFactory.MapBasedSelectorForObject(object)
	}
}

func (f *ring0Factory) PortsForObject(object runtime.Object) ([]string, error) {
	switch t := object.(type) {
	case *appsapi.DeploymentConfig:
		return getPorts(t.Spec.Template.Spec), nil
	default:
		return f.kubeClientAccessFactory.PortsForObject(object)
	}
}

func (f *ring0Factory) ProtocolsForObject(object runtime.Object) (map[string]string, error) {
	switch t := object.(type) {
	case *appsapi.DeploymentConfig:
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

func (f *ring0Factory) Command(cmd *cobra.Command, showSecrets bool) string {
	return f.kubeClientAccessFactory.Command(cmd, showSecrets)
}

func (f *ring0Factory) BindFlags(flags *pflag.FlagSet) {
	f.kubeClientAccessFactory.BindFlags(flags)
}

func (f *ring0Factory) BindExternalFlags(flags *pflag.FlagSet) {
	f.kubeClientAccessFactory.BindExternalFlags(flags)
}

func (f *ring0Factory) DefaultResourceFilterFunc() kubectl.Filters {
	return f.kubeClientAccessFactory.DefaultResourceFilterFunc()
}

func (f *ring0Factory) SuggestedPodTemplateResources() []schema.GroupResource {
	return f.kubeClientAccessFactory.SuggestedPodTemplateResources()
}

func (f *ring0Factory) Printer(mapping *meta.RESTMapping, options kprinters.PrintOptions) (kprinters.ResourcePrinter, error) {
	return describe.NewHumanReadablePrinter(f.JSONEncoder(), f.Decoder(true), options), nil
}

func (f *ring0Factory) Pauser(info *resource.Info) ([]byte, error) {
	switch t := info.Object.(type) {
	case *appsapi.DeploymentConfig:
		if t.Spec.Paused {
			return nil, errors.New("is already paused")
		}
		t.Spec.Paused = true
		// TODO: Pause the deployer containers.
		return runtime.Encode(f.JSONEncoder(), info.Object)
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
	f.StringVarP(&o.Source, "source", "", "docker", "The image source type; valid types are 'imagestreamtag', 'istag', 'imagestreamimage', 'isimage', and 'docker'")
	o.bound = true
}

func (f *ring0Factory) ImageResolutionOptions() FlagBinder {
	return f.imageResolutionOptions
}

func (f *ring0Factory) ResolveImage(image string) (string, error) {
	options := f.imageResolutionOptions.(*imageResolutionOptions)
	if isDockerImageSource(options.Source) {
		return f.kubeClientAccessFactory.ResolveImage(image)
	}
	config, err := f.OpenShiftClientConfig().ClientConfig()
	if err != nil {
		return "", err
	}
	imageClient, err := imageclient.NewForConfig(config)
	if err != nil {
		return "", err
	}
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return "", err
	}

	return resolveImagePullSpec(imageClient.Image(), options.Source, image, namespace)
}

func (f *ring0Factory) Resumer(info *resource.Info) ([]byte, error) {
	switch t := info.Object.(type) {
	case *appsapi.DeploymentConfig:
		if !t.Spec.Paused {
			return nil, errors.New("is not paused")
		}
		t.Spec.Paused = false
		// TODO: Resume the deployer containers.
		return runtime.Encode(f.JSONEncoder(), info.Object)
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
		"deploymentconfig/v1": deploymentcmd.BasicDeploymentConfigController{},
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

func (f *ring0Factory) CanBeExposed(kind schema.GroupKind) error {
	if appsapi.IsKindOrLegacy("DeploymentConfig", kind) {
		return nil
	}
	return f.kubeClientAccessFactory.CanBeExposed(kind)
}

func (f *ring0Factory) CanBeAutoscaled(kind schema.GroupKind) error {
	if appsapi.IsKindOrLegacy("DeploymentConfig", kind) {
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

	return metav1.NamespaceDefault, false, nil
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
