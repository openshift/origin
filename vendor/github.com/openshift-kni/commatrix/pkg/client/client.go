package client

import (
	"fmt"

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

	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}
	clientSet := &ClientSet{}

	coreV1, err := corev1client.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create corev1 client: %w", err)
	}
	clientSet.CoreV1Interface = coreV1

	mcop, err := mcopclientset.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine config operator client: %w", err)
	}
	clientSet.MCInterface = mcop

	imgClient, err := imagev1client.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create image client: %w", err)
	}
	clientSet.ImageClient = imgClient

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
