package server

import (
	"fmt"
	"net"
	_ "net/http/pprof"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/cmd/server/kubernetes"
)

func (cfg Config) BuildKubernetesMasterConfig(requestContextMapper kapi.RequestContextMapper, kubeClient *kclient.Client) (*kubernetes.MasterConfig, error) {
	masterAddr, err := cfg.GetMasterAddress()
	if err != nil {
		return nil, err
	}

	// Connect and setup etcd interfaces
	etcdClient, err := cfg.getEtcdClient()
	if err != nil {
		return nil, err
	}
	ketcdHelper, err := kmaster.NewEtcdHelper(etcdClient, klatest.Version)
	if err != nil {
		return nil, fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
	}

	portalNet := net.IPNet(cfg.PortalNet)
	masterIP := net.ParseIP(getHost(*masterAddr))
	if masterIP == nil {
		addrs, err := net.LookupIP(getHost(*masterAddr))
		if err != nil {
			return nil, fmt.Errorf("Unable to find an IP for %q - specify an IP directly? %v", getHost(*masterAddr), err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("Unable to find an IP for %q - specify an IP directly?", getHost(*masterAddr))
		}
		masterIP = addrs[0]
	}

	kmaster := &kubernetes.MasterConfig{
		MasterIP:             masterIP,
		MasterPort:           cfg.MasterAddr.Port,
		NodeHosts:            cfg.NodeList,
		PortalNet:            &portalNet,
		RequestContextMapper: requestContextMapper,
		EtcdHelper:           ketcdHelper,
		KubeClient:           kubeClient,
		Authorizer:           apiserver.NewAlwaysAllowAuthorizer(),
		AdmissionControl:     admit.NewAlwaysAdmit(),
	}

	return kmaster, nil
}
