package openshift_apiserver

import (
	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/pkg/version"
	"k8s.io/kubernetes/pkg/capabilities"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func RunOpenShiftAPIServer(masterConfig *configapi.MasterConfig) error {
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

	validationResults := validation.ValidateMasterConfig(masterConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("%v", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("MasterConfig"), "master-config.yaml", validationResults.Errors)
	}

	openshiftAPIServerRuntimeConfig, err := openshiftapiserver.NewOpenshiftAPIConfig(masterConfig)
	if err != nil {
		return err
	}
	openshiftAPIServer, err := openshiftAPIServerRuntimeConfig.Complete().New(genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}
	// this sets up the openapi endpoints
	preparedOpenshiftAPIServer := openshiftAPIServer.GenericAPIServer.PrepareRun()

	glog.Infof("Starting master on %s (%s)", masterConfig.ServingInfo.BindAddress, version.Get().String())
	glog.Infof("Public master address is %s", masterConfig.MasterPublicURL)
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = masterConfig.ImageConfig.Format
	imageTemplate.Latest = masterConfig.ImageConfig.Latest
	glog.Infof("Using images from %q", imageTemplate.ExpandOrDie("<component>"))

	if err := preparedOpenshiftAPIServer.Run(utilwait.NeverStop); err != nil {
		return err
	}

	return nil
}
