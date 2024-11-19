package clients

import (
	"os"

	buildclientset "github.com/openshift/client-go/build/clientset/versioned"
	configclientset "github.com/openshift/client-go/config/clientset/versioned"
	imageclientset "github.com/openshift/client-go/image/clientset/versioned"
	mapiclientset "github.com/openshift/client-go/machine/clientset/versioned"
	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	operatorclientset "github.com/openshift/client-go/operator/clientset/versioned"
	apiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// Builder can create a variety of kubernetes client interface
// with its embedded rest.Config.
type Builder struct {
	config *rest.Config
}

// MachineConfigClientOrDie returns the kubernetes client interface for machine config.
func (cb *Builder) MachineConfigClientOrDie(name string) mcfgclientset.Interface {
	return mcfgclientset.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

// MachineConfigClient is used in onceFrom mode where we can or cannot have a cluster ready
// based on the configuration provided
func (cb *Builder) MachineConfigClient(name string) (mcfgclientset.Interface, error) {
	return mcfgclientset.NewForConfig(rest.AddUserAgent(cb.config, name))
}

// KubeClientOrDie returns the kubernetes client interface for general kubernetes objects.
func (cb *Builder) KubeClientOrDie(name string) kubernetes.Interface {
	return kubernetes.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

// KubeClient is used in onceFrom mode where we can or cannot have a cluster ready
// based on the configuration provided
func (cb *Builder) KubeClient(name string) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(rest.AddUserAgent(cb.config, name))
}

// ConfigClientOrDie returns the config client interface for openshift
func (cb *Builder) ConfigClientOrDie(name string) configclientset.Interface {
	return configclientset.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

// OperatorClientOrDie returns the kubernetes client interface for operator-specific configuration.
func (cb *Builder) OperatorClientOrDie(name string) operatorclientset.Interface {
	return operatorclientset.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

// APIExtClientOrDie returns the kubernetes client interface for extended kubernetes objects.
func (cb *Builder) APIExtClientOrDie(name string) apiext.Interface {
	return apiext.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

func (cb *Builder) BuildClientOrDie(name string) buildclientset.Interface {
	return buildclientset.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

func (cb *Builder) ImageClientOrDie(name string) imageclientset.Interface {
	return imageclientset.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

// MachineClientOrDie returns the machine api client interface for machine api objects.
func (cb *Builder) MachineClientOrDie(name string) mapiclientset.Interface {
	return mapiclientset.NewForConfigOrDie(rest.AddUserAgent(cb.config, name))
}

// GetBuilderConfig returns a copy of the builders *rest.Config
func (cb *Builder) GetBuilderConfig() *rest.Config {
	return rest.CopyConfig(cb.config)
}

// NewBuilder returns a *ClientBuilder with the given kubeconfig.
func NewBuilder(kubeconfig string) (*Builder, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	if kubeconfig != "" {
		klog.V(4).Infof("Loading kube client config from path %q", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		klog.V(4).Infof("Using in-cluster kube client config")
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}

	return &Builder{
		config: config,
	}, nil
}

// BuilderFromConfig creates a *Builder with the given rest config.
func BuilderFromConfig(config *rest.Config) *Builder {
	return &Builder{
		config: config,
	}
}
