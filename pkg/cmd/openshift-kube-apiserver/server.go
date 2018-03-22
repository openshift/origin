package openshift_kube_apiserver

import (
	"fmt"

	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/pkg/version"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/capabilities"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func RunOpenShiftKubeAPIServerServer(masterConfig *configapi.MasterConfig) error {
	// Allow privileged containers
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
		PrivilegedSources: capabilities.PrivilegedSources{
			HostNetworkSources: []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostPIDSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostIPCSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
		},
	})

	if masterConfig.KubernetesMasterConfig == nil {
		return fmt.Errorf("KubernetesMasterConfig is required to start this server - use of external Kubernetes is no longer supported.")
	}

	// install aggregator types into the scheme so that "normal" RESTOptionsGetters can work for us.
	// done in Start() prior to doing any other initialization so we don't mutate the scheme after it is being used by clients in other goroutines.
	// TODO: make scheme threadsafe and do this as part of aggregator config building
	aggregatorinstall.Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)

	validationResults := validation.ValidateMasterConfig(masterConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("%v", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("MasterConfig"), "master-config.yaml", validationResults.Errors)
	}

	// informers are shared amongst all the various api components we build
	// TODO the needs of the apiserver and the controllers are drifting. We should consider two different skins here
	informers, err := origin.NewInformers(*masterConfig)
	if err != nil {
		return err
	}
	if err := informers.AddUserIndexes(); err != nil {
		return err
	}

	openshiftConfig, err := origin.BuildMasterConfig(*masterConfig, informers)
	if err != nil {
		return err
	}

	glog.Infof("Starting master on %s (%s)", masterConfig.ServingInfo.BindAddress, version.Get().String())
	glog.Infof("Public master address is %s", masterConfig.MasterPublicURL)
	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = masterConfig.ImageConfig.Format
	imageTemplate.Latest = masterConfig.ImageConfig.Latest
	glog.Infof("Using images from %q", imageTemplate.ExpandOrDie("<component>"))

	if err := openshiftConfig.RunKubeAPIServer(utilwait.NeverStop); err != nil {
		return err
	}

	return nil
}
