package clientcmd

import (
	scaleclient "k8s.io/client-go/scale"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/plugins"
)

type ring2Factory struct {
	clientAccessFactory  kcmdutil.ClientAccessFactory
	objectMappingFactory kcmdutil.ObjectMappingFactory
	kubeBuilderFactory   kcmdutil.BuilderFactory
}

func NewBuilderFactory(clientAccessFactory kcmdutil.ClientAccessFactory, objectMappingFactory kcmdutil.ObjectMappingFactory) kcmdutil.BuilderFactory {
	return &ring2Factory{
		clientAccessFactory:  clientAccessFactory,
		objectMappingFactory: objectMappingFactory,
		kubeBuilderFactory:   kcmdutil.NewBuilderFactory(clientAccessFactory, objectMappingFactory),
	}
}

// NewBuilder returns a new resource builder for structured api objects.
func (f *ring2Factory) NewBuilder() *resource.Builder {
	return f.kubeBuilderFactory.NewBuilder()
}

// PluginLoader loads plugins from a path set by the KUBECTL_PLUGINS_PATH env var.
// If this env var is not set, it defaults to
//   "~/.kube/plugins", plus
//  "./kubectl/plugins" directory under the "data dir" directory specified by the XDG
// system directory structure spec for the given platform.
func (f *ring2Factory) PluginLoader() plugins.PluginLoader {
	return f.kubeBuilderFactory.PluginLoader()
}

func (f *ring2Factory) PluginRunner() plugins.PluginRunner {
	return f.kubeBuilderFactory.PluginRunner()
}

func (f *ring2Factory) ScaleClient() (scaleclient.ScalesGetter, error) {
	return f.kubeBuilderFactory.ScaleClient()
}

func (f *ring2Factory) Scaler() (kubectl.Scaler, error) {
	return f.kubeBuilderFactory.Scaler()
}
