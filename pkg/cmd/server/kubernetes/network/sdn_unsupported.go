// +build !linux

package network

import (
	"fmt"

	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/network"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
)

func NewSDNInterfaces(options configapi.NodeConfig, networkClient networkclient.Interface, kubeClient kclientset.Interface, internalKubeInformers kinternalinformers.SharedInformerFactory, proxyconfig *componentconfig.KubeProxyConfiguration) (network.NodeInterface, network.ProxyInterface, error) {
	return nil, nil, fmt.Errorf("SDN not supported on this platform")
}
