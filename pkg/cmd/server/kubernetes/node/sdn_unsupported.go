// +build !linux

package node

import (
	"fmt"

	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/sdn"
)

func NewSDNInterfaces(options configapi.NodeConfig, originClient *osclient.Client, kubeClient kclientset.Interface, internalKubeInformers kinternalinformers.SharedInformerFactory, proxyconfig *componentconfig.KubeProxyConfiguration) (sdn.NodeInterface, sdn.ProxyInterface, error) {
	return nil, nil, fmt.Errorf("SDN not supported on this platform")
}
