package openshift_apiserver

import (
	"github.com/golang/glog"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/pkg/version"
	"k8s.io/kubernetes/pkg/capabilities"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/util"
)

func RunOpenShiftAPIServer(serverConfig *configapi.OpenshiftAPIServerConfig) error {
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
	openshiftAPIServer, err := openshiftAPIServerRuntimeConfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}
	// this sets up the openapi endpoints
	preparedOpenshiftAPIServer := openshiftAPIServer.GenericAPIServer.PrepareRun()

	glog.Infof("Starting master on %s (%s)", serverConfig.ServingInfo.BindAddress, version.Get().String())

	if err := preparedOpenshiftAPIServer.Run(utilwait.NeverStop); err != nil {
		return err
	}

	return nil
}
