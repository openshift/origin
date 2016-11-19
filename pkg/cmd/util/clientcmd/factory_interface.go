package clientcmd

import (
	"io"
	"time"

	"github.com/emicklei/go-restful/swagger"
	"github.com/openshift/origin/pkg/client"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"k8s.io/kubernetes/pkg/runtime"
)

// KubernetesFactory represents the kubernetes Factory interface.
// TODO: When implemented in kubernetes this can be removed.
type KubernetesFactory interface {
	Object(thirdPartyDiscovery bool) (meta.RESTMapper, runtime.ObjectTyper)
	UnstructuredObject() (meta.RESTMapper, runtime.ObjectTyper, error)
	Decoder(toInternal bool) runtime.Decoder
	JSONEncoder() runtime.Encoder
	Client() (*kclient.Client, error)
	ClientConfig() (*restclient.Config, error)
	ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error)
	UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error)
	Describer(mapping *meta.RESTMapping) (kubectl.Describer, error)
	Printer(mapping *meta.RESTMapping, options kubectl.PrintOptions) (kubectl.ResourcePrinter, error)
	Scaler(mapping *meta.RESTMapping) (kubectl.Scaler, error)
	Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error)
	HistoryViewer(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error)
	Rollbacker(mapping *meta.RESTMapping) (kubectl.Rollbacker, error)
	StatusViewer(mapping *meta.RESTMapping) (kubectl.StatusViewer, error)
	MapBasedSelectorForObject(object runtime.Object) (string, error)
	PortsForObject(object runtime.Object) ([]string, error)
	ProtocolsForObject(object runtime.Object) (map[string]string, error)
	LabelsForObject(object runtime.Object) (map[string]string, error)
	LogsForObject(object, options runtime.Object) (*restclient.Request, error)
	PauseObject(object runtime.Object) (bool, error)
	ResumeObject(object runtime.Object) (bool, error)
	Validator(validate bool, cacheDir string) (validation.Schema, error)
	SwaggerSchema(gvk unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error)
	DefaultNamespace() (string, bool, error)
	Generators(cmdName string) map[string]kubectl.Generator
	CanBeExposed(kind unversioned.GroupKind) error
	CanBeAutoscaled(kind unversioned.GroupKind) error
	AttachablePodForObject(object runtime.Object) (*api.Pod, error)
	UpdatePodSpecForObject(obj runtime.Object, fn func(*api.PodSpec) error) (bool, error)
	EditorEnvs() []string
	PrintObjectSpecificMessage(obj runtime.Object, out io.Writer)
}

// FactoryInterface represents an interface for a Factory.
type FactoryInterface interface {
	KubernetesFactory

	UpdateObjectEnvironment(obj runtime.Object, fn func(*[]api.EnvVar) error) (bool, error)
	ExtractFileContents(obj runtime.Object) (map[string][]byte, bool, error)
	ApproximatePodTemplateForObject(object runtime.Object) (*api.PodTemplateSpec, error)
	PodForResource(resource string, timeout time.Duration) (string, error)
	Clients() (client.Interface, client.KClientInterface, error)
	OriginSwaggerSchema(client resource.RESTClient, version unversioned.GroupVersion) (*swagger.ApiDeclaration, error)
	OSClientConfig() kclientcmd.ClientConfig
}

// Missing methods needed to make Factory implement FactoryInterface.

// OSClientConfig returns an Openshift ClientConfig.
func (f *Factory) OSClientConfig() kclientcmd.ClientConfig {
	return f.OpenShiftClientConfig
}

// Methods below are wrappers for kubernetes Factory functions.
func (f *Factory) Object(thirdPartyDiscovery bool) (meta.RESTMapper, runtime.ObjectTyper) {
	return f.Factory.Object(thirdPartyDiscovery)
}

func (f *Factory) UnstructuredObject() (meta.RESTMapper, runtime.ObjectTyper, error) {
	return f.Factory.UnstructuredObject()
}

func (f *Factory) Decoder(toInternal bool) runtime.Decoder {
	return f.Factory.Decoder(toInternal)
}

func (f *Factory) JSONEncoder() runtime.Encoder {
	return f.Factory.JSONEncoder()
}

func (f *Factory) Client() (*kclient.Client, error) {
	return f.Factory.Client()
}

func (f *Factory) ClientConfig() (*restclient.Config, error) {
	return f.Factory.ClientConfig()
}

func (f *Factory) ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	return f.Factory.ClientForMapping(mapping)
}

func (f *Factory) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	return f.Factory.UnstructuredClientForMapping(mapping)
}

func (f *Factory) Describer(mapping *meta.RESTMapping) (kubectl.Describer, error) {
	return f.Factory.Describer(mapping)
}

func (f *Factory) Printer(mapping *meta.RESTMapping, options kubectl.PrintOptions) (kubectl.ResourcePrinter, error) {
	return f.Factory.Printer(mapping, options)
}

func (f *Factory) Scaler(mapping *meta.RESTMapping) (kubectl.Scaler, error) {
	return f.Factory.Scaler(mapping)
}

func (f *Factory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	return f.Factory.Reaper(mapping)
}

func (f *Factory) HistoryViewer(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
	return f.Factory.HistoryViewer(mapping)
}

func (f *Factory) Rollbacker(mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
	return f.Factory.Rollbacker(mapping)
}

func (f *Factory) StatusViewer(mapping *meta.RESTMapping) (kubectl.StatusViewer, error) {
	return f.Factory.StatusViewer(mapping)
}

func (f *Factory) MapBasedSelectorForObject(object runtime.Object) (string, error) {
	return f.Factory.MapBasedSelectorForObject(object)
}

func (f *Factory) PortsForObject(object runtime.Object) ([]string, error) {
	return f.Factory.PortsForObject(object)
}

func (f *Factory) ProtocolsForObject(object runtime.Object) (map[string]string, error) {
	return f.Factory.ProtocolsForObject(object)
}

func (f *Factory) LabelsForObject(object runtime.Object) (map[string]string, error) {
	return f.Factory.LabelsForObject(object)
}

func (f *Factory) LogsForObject(object, options runtime.Object) (*restclient.Request, error) {
	return f.Factory.LogsForObject(object, options)
}

func (f *Factory) PauseObject(object runtime.Object) (bool, error) {
	return f.Factory.PauseObject(object)
}

func (f *Factory) ResumeObject(object runtime.Object) (bool, error) {
	return f.Factory.ResumeObject(object)
}

func (f *Factory) Validator(validate bool, cacheDir string) (validation.Schema, error) {
	return f.Factory.Validator(validate, cacheDir)
}

func (f *Factory) SwaggerSchema(gvk unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error) {
	return f.Factory.SwaggerSchema(gvk)
}

func (f *Factory) DefaultNamespace() (string, bool, error) {
	return f.Factory.DefaultNamespace()
}

func (f *Factory) Generators(cmdName string) map[string]kubectl.Generator {
	return f.Factory.Generators(cmdName)
}

func (f *Factory) CanBeExposed(kind unversioned.GroupKind) error {
	return f.Factory.CanBeExposed(kind)
}

func (f *Factory) CanBeAutoscaled(kind unversioned.GroupKind) error {
	return f.Factory.CanBeAutoscaled(kind)
}

func (f *Factory) AttachablePodForObject(object runtime.Object) (*api.Pod, error) {
	return f.Factory.AttachablePodForObject(object)
}
func (f *Factory) UpdatePodSpecForObject(obj runtime.Object, fn func(*api.PodSpec) error) (bool, error) {
	return f.Factory.UpdatePodSpecForObject(obj, fn)
}

func (f *Factory) EditorEnvs() []string {
	return f.Factory.EditorEnvs()
}

func (f *Factory) PrintObjectSpecificMessage(obj runtime.Object, out io.Writer) {
	f.Factory.PrintObjectSpecificMessage(obj, out)
}
