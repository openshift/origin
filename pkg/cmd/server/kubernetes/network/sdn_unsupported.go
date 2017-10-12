// +build !linux

package network

import (
	"fmt"

	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kinternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/network"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
)

func NewSDNInterfaces(options configapi.NodeConfig, networkClient networkclient.Interface, kubeClientset kclientset.Interface, kubeClient kinternalclientset.Interface, internalKubeInformers kinternalinformers.SharedInformerFactory, proxyconfig *componentconfig.KubeProxyConfiguration) (network.NodeInterface, network.ProxyInterface, error) {
	return nil, nil, fmt.Errorf("SDN not supported on this platform")
}
