package kubernetes

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/cmd/kube-apiserver/app"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

// MasterConfig defines the required values to start a Kubernetes master
type MasterConfig struct {
	Options configapi.KubernetesMasterConfig

	Master     *master.Config
	KubeClient *kclient.Client
}

func BuildKubernetesMasterConfig(options configapi.MasterConfig, requestContextMapper kapi.RequestContextMapper, kubeClient *kclient.Client) (*MasterConfig, error) {
	if options.KubernetesMasterConfig == nil {
		return nil, errors.New("insufficient information to build KubernetesMasterConfig")
	}

	// Connect and setup etcd interfaces
	etcdClient, err := etcd.GetAndTestEtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	ketcdHelper, err := master.NewEtcdHelper(etcdClient, options.EtcdStorageConfig.KubernetesStorageVersion, options.EtcdStorageConfig.KubernetesStoragePrefix)
	if err != nil {
		return nil, fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
	}

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)
	kubeletClient, err := kclient.NewKubeletClient(kubeletClientConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to configure Kubelet client: %v", err)
	}

	// in-order list of plug-ins that should intercept admission decisions
	// TODO: Push node environment support to upstream in future

	_, portString, err := net.SplitHostPort(options.ServingInfo.BindAddress)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}

	portRange, err := util.ParsePortRange(options.KubernetesMasterConfig.ServicesNodePortRange)
	if err != nil {
		return nil, err
	}

	server := app.NewAPIServer()
	server.EventTTL = 2 * time.Hour
	server.PortalNet = util.IPNet(flagtypes.DefaultIPNet(options.KubernetesMasterConfig.ServicesSubnet))
	server.ServiceNodePorts = *portRange
	server.AdmissionControl = strings.Join([]string{
		"NamespaceExists", "NamespaceLifecycle", "OriginPodNodeEnvironment", "LimitRanger", "ServiceAccount", "ResourceQuota",
	}, ",")

	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.APIServerArguments, server.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	admissionController := admission.NewFromPlugins(kubeClient, strings.Split(server.AdmissionControl, ","), server.AdmissionControlConfigFile)

	m := &master.Config{
		PublicAddress: net.ParseIP(options.KubernetesMasterConfig.MasterIP),
		ReadWritePort: port,
		ReadOnlyPort:  port,

		EtcdHelper: ketcdHelper,

		EventTTL: server.EventTTL,
		//MinRequestTimeout: server.MinRequestTimeout,

		PortalNet:        (*net.IPNet)(&server.PortalNet),
		ServiceNodePorts: server.ServiceNodePorts,

		RequestContextMapper: requestContextMapper,

		KubeletClient: kubeletClient,
		APIPrefix:     KubeAPIPrefix,

		EnableCoreControllers: true,

		MasterCount: options.KubernetesMasterConfig.MasterCount,

		Authorizer:       apiserver.NewAlwaysAllowAuthorizer(),
		AdmissionControl: admissionController,

		DisableV1Beta1: true,
		DisableV1Beta2: true,
		DisableV1Beta3: !configapi.HasKubernetesAPILevel(*options.KubernetesMasterConfig, "v1beta3"),
		EnableV1:       configapi.HasKubernetesAPILevel(*options.KubernetesMasterConfig, "v1"),
	}

	kmaster := &MasterConfig{
		Options:    *options.KubernetesMasterConfig,
		Master:     m,
		KubeClient: kubeClient,
	}

	return kmaster, nil
}
