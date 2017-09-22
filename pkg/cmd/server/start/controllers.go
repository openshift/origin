package start

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	cmappoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/cm"
	origincontrollers "github.com/openshift/origin/pkg/cmd/server/origin/controller"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func newControllerContext(
	openshiftControllerOptions origincontrollers.OpenshiftControllerOptions,
	privilegedLoopbackConfig *rest.Config,
	kubeExternal kclientsetexternal.Interface,
	informers *informers,
	stopCh <-chan struct{},
) origincontrollers.ControllerContext {

	// divide up the QPS since it re-used separately for every client
	// TODO, eventually make this configurable individually in some way.
	if privilegedLoopbackConfig.QPS > 0 {
		privilegedLoopbackConfig.QPS = privilegedLoopbackConfig.QPS/10 + 1
	}
	if privilegedLoopbackConfig.Burst > 0 {
		privilegedLoopbackConfig.Burst = privilegedLoopbackConfig.Burst/10 + 1
	}

	openshiftControllerContext := origincontrollers.ControllerContext{
		OpenshiftControllerOptions: openshiftControllerOptions,

		ClientBuilder: origincontrollers.OpenshiftControllerClientBuilder{
			ControllerClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         rest.AnonymousClientConfig(privilegedLoopbackConfig),
				CoreClient:           kubeExternal.Core(),
				AuthenticationClient: kubeExternal.Authentication(),
				Namespace:            bootstrappolicy.DefaultOpenShiftInfraNamespace,
			},
		},
		InternalKubeInformers:  informers.internalKubeInformers,
		ExternalKubeInformers:  informers.externalKubeInformers,
		AppInformers:           informers.appInformers,
		AuthorizationInformers: informers.authorizationInformers,
		BuildInformers:         informers.buildInformers,
		ImageInformers:         informers.imageInformers,
		QuotaInformers:         informers.quotaInformers,
		SecurityInformers:      informers.securityInformers,
		TemplateInformers:      informers.templateInformers,
		Stop:                   stopCh,
	}

	return openshiftControllerContext
}

// getOpenshiftControllerOptions parses the CLI args used by the kube-controllers (which control these options today), so that
// we can defer making the controller options structs until we have a better idea what they should look like.
// This does mean we pull in an upstream command that hopefully won't change much.
func getOpenshiftControllerOptions(args map[string][]string) (origincontrollers.OpenshiftControllerOptions, error) {
	cmserver := cmappoptions.NewCMServer()
	if err := cmdflags.Resolve(args, cm.OriginControllerManagerAddFlags(cmserver)); len(err) > 0 {
		return origincontrollers.OpenshiftControllerOptions{}, kutilerrors.NewAggregate(err)
	}

	return origincontrollers.OpenshiftControllerOptions{
		HPAControllerOptions: origincontrollers.HPAControllerOptions{
			SyncPeriod:               cmserver.KubeControllerManagerConfiguration.HorizontalPodAutoscalerSyncPeriod,
			UpscaleForbiddenWindow:   cmserver.KubeControllerManagerConfiguration.HorizontalPodAutoscalerUpscaleForbiddenWindow,
			DownscaleForbiddenWindow: cmserver.KubeControllerManagerConfiguration.HorizontalPodAutoscalerDownscaleForbiddenWindow,
		},
		ResourceQuotaOptions: origincontrollers.ResourceQuotaOptions{
			ConcurrentSyncs: cmserver.KubeControllerManagerConfiguration.ConcurrentResourceQuotaSyncs,
			SyncPeriod:      cmserver.KubeControllerManagerConfiguration.ResourceQuotaSyncPeriod,
			MinResyncPeriod: cmserver.KubeControllerManagerConfiguration.MinResyncPeriod,
		},
		ServiceAccountTokenOptions: origincontrollers.ServiceAccountTokenOptions{
			ConcurrentSyncs: cmserver.KubeControllerManagerConfiguration.ConcurrentSATokenSyncs,
		},
	}, nil
}

// getLeaderElectionOptions parses the CLI args used by the openshift controller leader election (which control these options today), so that
// we can defer making the options structs until we have a better idea what they should look like.
// This does mean we pull in an upstream command that hopefully won't change much.
func getLeaderElectionOptions(args map[string][]string) (componentconfig.LeaderElectionConfiguration, error) {
	cmserver := cmappoptions.NewCMServer()
	cmserver.LeaderElection.RetryPeriod = metav1.Duration{Duration: 3 * time.Second}

	if err := cmdflags.Resolve(args, cm.OriginControllerManagerAddFlags(cmserver)); len(err) > 0 {
		return componentconfig.LeaderElectionConfiguration{}, kutilerrors.NewAggregate(err)
	}

	return cmserver.KubeControllerManagerConfiguration.LeaderElection, nil
}

func waitForHealthyAPIServer(client rest.Interface) error {
	var healthzContent string
	// If apiserver is not running we should wait for some time and fail only then. This is particularly
	// important when we start apiserver and controller manager at the same time.
	err := wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		healthStatus := 0
		resp := client.Get().AbsPath("/healthz").Do().StatusCode(&healthStatus)
		if healthStatus != http.StatusOK {
			glog.Errorf("Server isn't healthy yet. Waiting a little while.")
			return false, nil
		}
		content, _ := resp.Raw()
		healthzContent = string(content)

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("server unhealthy: %v: %v", healthzContent, err)
	}

	return nil
}
