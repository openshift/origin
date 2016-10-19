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

// MockFactory implements FactoryInterface to test.
type MockFactory struct {
	OnUpdateObjectEnvironment         func(obj runtime.Object, fn func(*[]api.EnvVar) error) (bool, error)
	OnExtractFileContents             func(obj runtime.Object) (map[string][]byte, bool, error)
	OnApproximatePodTemplateForObject func(object runtime.Object) (*api.PodTemplateSpec, error)
	OnPodForResource                  func(resource string, timeout time.Duration) (string, error)
	OnClients                         func() (client.Interface, client.KClientInterface, error)
	OnOriginSwaggerSchema             func(client resource.RESTClient, version unversioned.GroupVersion) (*swagger.ApiDeclaration, error)
	OnOSClientConfig                  func() kclientcmd.ClientConfig

	// Kubernetes Factory functions.
	OnObject                       func(thirdPartyDiscovery bool) (meta.RESTMapper, runtime.ObjectTyper)
	OnUnstructuredObject           func() (meta.RESTMapper, runtime.ObjectTyper, error)
	OnDecoder                      func(toInternal bool) runtime.Decoder
	OnJSONEncoder                  func() runtime.Encoder
	OnClient                       func() (*kclient.Client, error)
	OnClientConfig                 func() (*restclient.Config, error)
	OnClientForMapping             func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	OnUnstructuredClientForMapping func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	OnDescriber                    func(mapping *meta.RESTMapping) (kubectl.Describer, error)
	OnPrinter                      func(mapping *meta.RESTMapping, options kubectl.PrintOptions) (kubectl.ResourcePrinter, error)
	OnScaler                       func(mapping *meta.RESTMapping) (kubectl.Scaler, error)
	OnReaper                       func(mapping *meta.RESTMapping) (kubectl.Reaper, error)
	OnHistoryViewer                func(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error)
	OnRollbacker                   func(mapping *meta.RESTMapping) (kubectl.Rollbacker, error)
	OnStatusViewer                 func(mapping *meta.RESTMapping) (kubectl.StatusViewer, error)
	OnMapBasedSelectorForObject    func(object runtime.Object) (string, error)
	OnPortsForObject               func(object runtime.Object) ([]string, error)
	OnProtocolsForObject           func(object runtime.Object) (map[string]string, error)
	OnLabelsForObject              func(object runtime.Object) (map[string]string, error)
	OnLogsForObject                func(object, options runtime.Object) (*restclient.Request, error)
	OnPauseObject                  func(object runtime.Object) (bool, error)
	OnResumeObject                 func(object runtime.Object) (bool, error)
	OnValidator                    func(validate bool, cacheDir string) (validation.Schema, error)
	OnSwaggerSchema                func(gvk unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error)
	OnDefaultNamespace             func() (string, bool, error)
	OnGenerators                   func(cmdName string) map[string]kubectl.Generator
	OnCanBeExposed                 func(kind unversioned.GroupKind) error
	OnCanBeAutoscaled              func(kind unversioned.GroupKind) error
	OnAttachablePodForObject       func(object runtime.Object) (*api.Pod, error)
	OnUpdatePodSpecForObject       func(obj runtime.Object, fn func(*api.PodSpec) error) (bool, error)
	OnEditorEnvs                   func() []string
	OnPrintObjectSpecificMessage   func(obj runtime.Object, out io.Writer)
}

func (m *MockFactory) UpdateObjectEnvironment(obj runtime.Object, fn func(*[]api.EnvVar) error) (bool, error) {
	return m.OnUpdateObjectEnvironment(obj, fn)
}

func (m *MockFactory) ExtractFileContents(obj runtime.Object) (map[string][]byte, bool, error) {
	return m.OnExtractFileContents(obj)
}

func (m *MockFactory) ApproximatePodTemplateForObject(object runtime.Object) (*api.PodTemplateSpec, error) {
	return m.OnApproximatePodTemplateForObject(object)
}

func (m *MockFactory) PodForResource(resource string, timeout time.Duration) (string, error) {
	return m.OnPodForResource(resource, timeout)
}

// Clients returns an OpenShift and Kubernetes client.
func (m *MockFactory) Clients() (client.Interface, client.KClientInterface, error) {
	return m.OnClients()
}

// OriginSwaggerSchema returns a swagger API doc for an Origin schema under the /oapi prefix.
func (m *MockFactory) OriginSwaggerSchema(client resource.RESTClient, version unversioned.GroupVersion) (*swagger.ApiDeclaration, error) {
	return m.OnOriginSwaggerSchema(client, version)
}

// OSClientConfig returns an Openshift CLientConfig.
func (m *MockFactory) OSClientConfig() kclientcmd.ClientConfig {
	return m.OnOSClientConfig()
}

// Methods below are wrappers for kubernetes Factory functions.

func (m *MockFactory) Object(thirdPartyDiscovery bool) (meta.RESTMapper, runtime.ObjectTyper) {
	return m.OnObject(thirdPartyDiscovery)
}

func (m *MockFactory) UnstructuredObject() (meta.RESTMapper, runtime.ObjectTyper, error) {
	return m.OnUnstructuredObject()
}

func (m *MockFactory) Decoder(toInternal bool) runtime.Decoder {
	return m.OnDecoder(toInternal)
}

func (m *MockFactory) JSONEncoder() runtime.Encoder {
	return m.OnJSONEncoder()
}

func (m *MockFactory) Client() (*kclient.Client, error) {
	return m.OnClient()
}

func (m *MockFactory) ClientConfig() (*restclient.Config, error) {
	return m.OnClientConfig()
}

func (m *MockFactory) ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	return m.OnClientForMapping(mapping)
}

func (m *MockFactory) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	return m.OnUnstructuredClientForMapping(mapping)
}

func (m *MockFactory) Describer(mapping *meta.RESTMapping) (kubectl.Describer, error) {
	return m.OnDescriber(mapping)
}

func (m *MockFactory) Printer(mapping *meta.RESTMapping, options kubectl.PrintOptions) (kubectl.ResourcePrinter, error) {
	return m.OnPrinter(mapping, options)
}

func (m *MockFactory) Scaler(mapping *meta.RESTMapping) (kubectl.Scaler, error) {
	return m.OnScaler(mapping)
}

func (m *MockFactory) Reaper(mapping *meta.RESTMapping) (kubectl.Reaper, error) {
	return m.OnReaper(mapping)
}

func (m *MockFactory) HistoryViewer(mapping *meta.RESTMapping) (kubectl.HistoryViewer, error) {
	return m.OnHistoryViewer(mapping)
}

func (m *MockFactory) Rollbacker(mapping *meta.RESTMapping) (kubectl.Rollbacker, error) {
	return m.OnRollbacker(mapping)
}

func (m *MockFactory) StatusViewer(mapping *meta.RESTMapping) (kubectl.StatusViewer, error) {
	return m.OnStatusViewer(mapping)
}

func (m *MockFactory) MapBasedSelectorForObject(object runtime.Object) (string, error) {
	return m.OnMapBasedSelectorForObject(object)
}

func (m *MockFactory) PortsForObject(object runtime.Object) ([]string, error) {
	return m.OnPortsForObject(object)
}

func (m *MockFactory) ProtocolsForObject(object runtime.Object) (map[string]string, error) {
	return m.OnProtocolsForObject(object)
}

func (m *MockFactory) LabelsForObject(object runtime.Object) (map[string]string, error) {
	return m.OnLabelsForObject(object)
}

func (m *MockFactory) LogsForObject(object, options runtime.Object) (*restclient.Request, error) {
	return m.OnLogsForObject(object, options)
}

func (m *MockFactory) PauseObject(object runtime.Object) (bool, error) {
	return m.OnPauseObject(object)
}

func (m *MockFactory) ResumeObject(object runtime.Object) (bool, error) {
	return m.OnResumeObject(object)
}

func (m *MockFactory) Validator(validate bool, cacheDir string) (validation.Schema, error) {
	return m.OnValidator(validate, cacheDir)
}

func (m *MockFactory) SwaggerSchema(gvk unversioned.GroupVersionKind) (*swagger.ApiDeclaration, error) {
	return m.OnSwaggerSchema(gvk)
}

func (m *MockFactory) DefaultNamespace() (string, bool, error) {
	return m.OnDefaultNamespace()
}

func (m *MockFactory) Generators(cmdName string) map[string]kubectl.Generator {
	return m.OnGenerators(cmdName)
}

func (m *MockFactory) CanBeExposed(kind unversioned.GroupKind) error {
	return m.OnCanBeExposed(kind)
}

func (m *MockFactory) CanBeAutoscaled(kind unversioned.GroupKind) error {
	return m.OnCanBeAutoscaled(kind)
}

func (m *MockFactory) AttachablePodForObject(object runtime.Object) (*api.Pod, error) {
	return m.OnAttachablePodForObject(object)
}

func (m *MockFactory) UpdatePodSpecForObject(obj runtime.Object, fn func(*api.PodSpec) error) (bool, error) {
	return m.OnUpdatePodSpecForObject(obj, fn)
}

func (m *MockFactory) EditorEnvs() []string {
	return m.OnEditorEnvs()
}

func (m *MockFactory) PrintObjectSpecificMessage(obj runtime.Object, out io.Writer) {
	m.OnPrintObjectSpecificMessage(obj, out)
}
