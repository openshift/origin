package client

import (
	ocpconfigv1 "github.com/openshift/api/config"
	machineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ocpoperatorv1 "github.com/openshift/api/operator/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	mcopclientset "github.com/openshift/client-go/operator/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientSet struct {
	runtimeclient.Client
	corev1client.CoreV1Interface
	Config      *rest.Config
	MCInterface mcopclientset.Interface
	ImageClient imagev1client.ImageV1Interface
}

func New() (*ClientSet, error) {
	var err error

	restConfig := ctrl.GetConfigOrDie()
	clientSet := &ClientSet{}

	clientSet.CoreV1Interface = corev1client.NewForConfigOrDie(restConfig)
	clientSet.MCInterface = mcopclientset.NewForConfigOrDie(restConfig)
	clientSet.ImageClient = imagev1client.NewForConfigOrDie(restConfig)

	clientSet.Config = restConfig

	myScheme := runtime.NewScheme()

	err = corev1.AddToScheme(myScheme)
	if err != nil {
		return nil, err
	}

	err = discoveryv1.AddToScheme(myScheme)
	if err != nil {
		return nil, err
	}

	err = ocpoperatorv1.AddToScheme(myScheme)
	if err != nil {
		return nil, err
	}

	err = machineconfigurationv1.AddToScheme(myScheme)
	if err != nil {
		return nil, err
	}

	err = ocpconfigv1.Install(myScheme)
	if err != nil {
		return nil, err
	}

	clientSet.Client, err = runtimeclient.New(restConfig, runtimeclient.Options{Scheme: myScheme})
	if err != nil {
		return nil, err
	}

	return clientSet, nil
}
