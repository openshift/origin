package openshift_kube_apiserver

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	"k8s.io/kubernetes/pkg/capabilities"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/openshiftkubeapiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	originadmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	"k8s.io/kubernetes/pkg/kubeapiserver/options"
)

func RunOpenShiftKubeAPIServerServer(kubeAPIServerConfig *configapi.KubeAPIServerConfig) error {
	// Allow privileged containers
	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: true,
		PrivilegedSources: capabilities.PrivilegedSources{
			HostNetworkSources: []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostPIDSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
			HostIPCSources:     []string{kubelettypes.ApiserverSource, kubelettypes.FileSource},
		},
	})

	bootstrappolicy.ClusterRoles = bootstrappolicy.OpenshiftClusterRoles
	bootstrappolicy.ClusterRoleBindings = bootstrappolicy.OpenshiftClusterRoleBindings

	options.AllOrderedPlugins = originadmission.CombinedAdmissionControlPlugins
	kubeRegisterAdmission := options.RegisterAllAdmissionPlugins
	options.RegisterAllAdmissionPlugins = func(plugins *admission.Plugins) {
		kubeRegisterAdmission(plugins)
		originadmission.RegisterOpenshiftAdmissionPlugins(plugins)
	}
	kubeDefaultOffAdmission := options.DefaultOffAdmissionPlugins
	options.DefaultOffAdmissionPlugins = func() sets.String {
		kubeOff := kubeDefaultOffAdmission()
		kubeOff.Delete(originadmission.DefaultOnPlugins.List()...)
		return kubeOff
	}

	configPatchFn, serverPatchContext := openshiftkubeapiserver.NewOpenShiftKubeAPIServerConfigPatch(genericapiserver.NewEmptyDelegate(), kubeAPIServerConfig)
	app.OpenShiftKubeAPIServerConfigPatch = configPatchFn
	app.OpenShiftKubeAPIServerServerPatch = serverPatchContext.PatchServer

	cmd := app.NewAPIServerCommand(utilwait.NeverStop)
	args, err := openshiftkubeapiserver.ConfigToFlags(kubeAPIServerConfig)
	if err != nil {
		return err
	}
	if err := cmd.ParseFlags(args); err != nil {
		return err
	}
	glog.Infof("`kube-apiserver %v`", args)
	if err := cmd.RunE(cmd, nil); err != nil {
		return err
	}

	return fmt.Errorf("`kube-apiserver %v` exited", args)
}
