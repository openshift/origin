package openshift_apiserver

import (
	"github.com/golang/glog"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/pkg/version"
	"k8s.io/kubernetes/pkg/capabilities"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	"github.com/openshift/origin/pkg/cmd/util"

	// for metrics
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus"
)

// default this to true so that the integration tests don't instantly break
var featureKeepRemovedNetworkingAPI = true

func RunOpenShiftAPIServer(serverConfig *openshiftcontrolplanev1.OpenShiftAPIServerConfig, stopCh <-chan struct{}) error {
	util.InitLogrus()
	// Allow privileged containers
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
		PrivilegedSources: capabilities.PrivilegedSources{
			HostNetworkSources: []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostPIDSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostIPCSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
		},
	})

	openshiftAPIServerRuntimeConfig, err := openshiftapiserver.NewOpenshiftAPIConfig(serverConfig)
	if err != nil {
		return err
	}
	openshiftAPIServer, err := openshiftAPIServerRuntimeConfig.Complete().New(genericapiserver.NewEmptyDelegate(), featureKeepRemovedNetworkingAPI)
	if err != nil {
		return err
	}
	// this sets up the openapi endpoints
	preparedOpenshiftAPIServer := openshiftAPIServer.GenericAPIServer.PrepareRun()

	glog.Infof("Starting master on %s (%s)", serverConfig.ServingInfo.BindAddress, version.Get().String())

	return preparedOpenshiftAPIServer.Run(stopCh)
}
