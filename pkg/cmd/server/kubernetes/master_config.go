package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/authorizer"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
)

// MasterConfig defines the required values to start a Kubernetes master
type MasterConfig struct {
	MasterIP   net.IP
	MasterPort int
	NodeHosts  []string
	PortalNet  *net.IPNet

	RequestContextMapper kapi.RequestContextMapper

	EtcdHelper tools.EtcdHelper
	KubeClient *kclient.Client

	Authorizer       authorizer.Authorizer
	AdmissionControl admission.Interface
}

func BuildKubernetesMasterConfig(options configapi.MasterConfig, requestContextMapper kapi.RequestContextMapper, kubeClient *kclient.Client) (*MasterConfig, error) {
	if options.KubernetesMasterConfig == nil {
		return nil, errors.New("insufficient information to build KubernetesMasterConfig")
	}

	// Connect and setup etcd interfaces
	etcdClient, err := etcd.GetAndTestEtcdClient(options.EtcdClientInfo.URL)
	if err != nil {
		return nil, err
	}
	ketcdHelper, err := master.NewEtcdHelper(etcdClient, klatest.Version)
	if err != nil {
		return nil, fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
	}

	portalNet := net.IPNet(flagtypes.DefaultIPNet(options.KubernetesMasterConfig.ServicesSubnet))

	// in-order list of plug-ins that should intercept admission decisions
	// TODO: add NamespaceExists
	admissionControlPluginNames := []string{"LimitRanger", "ResourceQuota"}
	admissionController := admission.NewFromPlugins(kubeClient, admissionControlPluginNames, "")

	host, portString, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}

	kmaster := &MasterConfig{
		MasterIP:             net.ParseIP(host),
		MasterPort:           port,
		NodeHosts:            options.KubernetesMasterConfig.StaticNodeNames,
		PortalNet:            &portalNet,
		RequestContextMapper: requestContextMapper,
		EtcdHelper:           ketcdHelper,
		KubeClient:           kubeClient,
		Authorizer:           apiserver.NewAlwaysAllowAuthorizer(),
		AdmissionControl:     admissionController,
	}

	return kmaster, nil
}
