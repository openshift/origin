// +build !linux

package network

import (
	"fmt"

	kclientset "k8s.io/client-go/kubernetes"
	kinternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/proxy/apis/kubeproxyconfig"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	networkinformers "github.com/openshift/origin/pkg/network/generated/informers/internalversion"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
)

func NewSDNInterfaces(options configapi.NodeConfig, networkClient networkclient.Interface,
	kubeClientset kclientset.Interface, kubeClient kinternalclientset.Interface,
	internalKubeInformers kinternalinformers.SharedInformerFactory,
	internalNetworkInformers networkinformers.SharedInformerFactory,
	proxyconfig *kubeproxyconfig.KubeProxyConfiguration) (NodeInterface, ProxyInterface, error) {

	return nil, nil, fmt.Errorf("SDN not supported on this platform")
}
