package framework

import (
	"fmt"
	"os"

	clientbuildv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	clientimagev1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	clientoperatorsv1alpha1 "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1alpha1"
	clientmachineconfigv1 "github.com/openshift/machine-config-operator/pkg/generated/clientset/versioned/typed/machineconfiguration.openshift.io/v1"
	clientapiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

type ClientSet struct {
	corev1client.CoreV1Interface
	appsv1client.AppsV1Interface
	clientconfigv1.ConfigV1Interface
	clientmachineconfigv1.MachineconfigurationV1Interface
	clientapiextensionsv1.ApiextensionsV1Interface
	clientoperatorsv1alpha1.OperatorV1alpha1Interface
	clientbuildv1.BuildV1Interface
	clientimagev1.ImageV1Interface
	kubeconfig string
}

func (cs *ClientSet) GetKubeconfig() (string, error) {
	if cs.kubeconfig != "" {
		return cs.kubeconfig, nil
	}

	return "", fmt.Errorf("no kubeconfig found; are you running a custom config or in-cluster?")
}

// NewClientSet returns a *ClientBuilder with the given kubeconfig.
func NewClientSet(kubeconfig string) *ClientSet {
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
		panic(err)
	}

	cs := NewClientSetFromConfig(config)
	cs.kubeconfig = kubeconfig
	return cs
}

// NewClientSetFromConfig returns a *ClientBuilder with the given rest config.
func NewClientSetFromConfig(config *rest.Config) *ClientSet {
	return &ClientSet{
		CoreV1Interface:                 corev1client.NewForConfigOrDie(config),
		AppsV1Interface:                 appsv1client.NewForConfigOrDie(config),
		ConfigV1Interface:               clientconfigv1.NewForConfigOrDie(config),
		MachineconfigurationV1Interface: clientmachineconfigv1.NewForConfigOrDie(config),
		ApiextensionsV1Interface:        clientapiextensionsv1.NewForConfigOrDie(config),
		OperatorV1alpha1Interface:       clientoperatorsv1alpha1.NewForConfigOrDie(config),
		BuildV1Interface:                clientbuildv1.NewForConfigOrDie(config),
		ImageV1Interface:                clientimagev1.NewForConfigOrDie(config),
	}
}
