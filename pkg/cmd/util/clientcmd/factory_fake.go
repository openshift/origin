package clientcmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/validation"
	kclient "k8s.io/kubernetes/pkg/client"
	clientcmdapi "k8s.io/kubernetes/pkg/client/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
)

type fakeClientConfig struct {
	Raw      clientcmdapi.Config
	Client   *kclient.Config
	NS       string
	Explicit bool
	Err      error
}

// RawConfig returns the merged result of all overrides
func (c *fakeClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return c.Raw, c.Err
}

// ClientConfig returns a complete client config
func (c *fakeClientConfig) ClientConfig() (*kclient.Config, error) {
	return c.Client, c.Err
}

// Namespace returns the namespace resulting from the merged result of all overrides
func (c *fakeClientConfig) Namespace() (string, bool, error) {
	return c.NS, c.Explicit, c.Err
}

type testPrinter struct {
	Objects []runtime.Object
	Err     error
}

func (t *testPrinter) PrintObj(obj runtime.Object, out io.Writer) error {
	t.Objects = append(t.Objects, obj)
	fmt.Fprintf(out, "%#v", obj)
	return t.Err
}

type testFactory struct {
	Mapper       meta.RESTMapper
	Typer        runtime.ObjectTyper
	Client       kubectl.RESTClient
	Describer    kubectl.Describer
	Printer      kubectl.ResourcePrinter
	Validator    validation.Schema
	Namespace    string
	ClientConfig *kclient.Config
	Err          error
}

// newFakeKubeFactory returns a new fake Kubernetes factory
// TODO: Use this from upstream
func newFakeKubeFactory() (*cmdutil.Factory, *testFactory, runtime.Codec) {
	t := &testFactory{
		Client:       &kclient.FakeRESTClient{},
		Printer:      &testPrinter{},
		Validator:    validation.NullSchema{},
		ClientConfig: &kclient.Config{},
	}
	generators := map[string]kubectl.Generator{
		"run/v1":     kubectl.BasicReplicationController{},
		"service/v1": kubectl.ServiceGeneratorV1{},
		"service/v2": kubectl.ServiceGeneratorV2{},
	}
	return &cmdutil.Factory{
		Object: func() (meta.RESTMapper, runtime.ObjectTyper) {
			return latest.RESTMapper, api.Scheme
		},
		Client: func() (*kclient.Client, error) {
			// Swap out the HTTP client out of the client with the fake's version.
			fakeClient := t.Client.(*kclient.FakeRESTClient)
			c := kclient.NewOrDie(t.ClientConfig)
			c.Client = fakeClient.Client
			return c, t.Err
		},
		RESTClient: func(*meta.RESTMapping) (resource.RESTClient, error) {
			return t.Client, t.Err
		},
		Describer: func(*meta.RESTMapping) (kubectl.Describer, error) {
			return t.Describer, t.Err
		},
		Printer: func(mapping *meta.RESTMapping, noHeaders, withNamespace bool, wide bool, columnLabels []string) (kubectl.ResourcePrinter, error) {
			return t.Printer, t.Err
		},
		Validator: func() (validation.Schema, error) {
			return t.Validator, t.Err
		},
		DefaultNamespace: func() (string, bool, error) {
			return t.Namespace, false, t.Err
		},
		ClientConfig: func() (*kclient.Config, error) {
			return t.ClientConfig, t.Err
		},
		Generator: func(name string) (kubectl.Generator, bool) {
			generator, ok := generators[name]
			return generator, ok
		},
	}, t, testapi.Codec()
}

// NewFakeFactory returns a new fake OpenShift factory
func NewFakeFactory(namespace string, fake *kclient.FakeRESTClient) (*Factory, *testFactory, runtime.Codec) {
	kf, t, codec := newFakeKubeFactory()

	t.Namespace = namespace

	// Create an OpenShift client and inject it into the client cache
	osClient := client.NewOrDie(t.ClientConfig)
	fake.Codec = codec
	osClient.Client = fake
	clients := &clientCache{
		clients:       map[string]*client.Client{"v1": osClient},
		defaultConfig: &kclient.Config{},
	}

	// Override here any other function we need

	return &Factory{
		Factory:               kf,
		OpenShiftClientConfig: &fakeClientConfig{},
		clients:               clients,
	}, t, codec
}

// ObjBody wraps an object into a readcloser
func ObjBody(codec runtime.Codec, obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}
