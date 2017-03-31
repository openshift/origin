package controller

import (
	"time"

	"k8s.io/kubernetes/cmd/kube-controller-manager/app"
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/util/wait"

	serviceaccountcontrollers "github.com/openshift/origin/pkg/serviceaccounts/controllers"

	"github.com/golang/glog"
)

const (
	DockercfgTokenDeletedControllerName = "dockercfg-token-deleted-controller"
)

// ControllerContext is an Origin specific version of
// k8s.io/kubernetes/cmd/kube-controller-manager/app's ControllerContext.
// We may embed the upstream type here in the future.
// For now we have our own custom type as most of the fields in the upstream type do not apply.
type ControllerContext struct {
	ClientBuilder controller.ControllerClientBuilder
	Stop          <-chan struct{}
}

// InitFunc is used to launch a particular controller.
// It may run additional "should I activate checks".
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
// InitFunc should execute the given controller's Run method in a different go routine.
// It is an Origin specific version of
// k8s.io/kubernetes/cmd/kube-controller-manager/app's InitFunc.
// The only difference is that this func uses the Origin version of ControllerContext.
type InitFunc func(ctx *ControllerContext) (bool, error)

// controllers is the global map of controller name to controller initialization func.
// See pkg/cmd/server/bootstrappolicy/controller_policy.go to see what cluster roles apply to each controller.
// All controller roles and bindings are based on the name of the controller.
var controllers = map[string]InitFunc{
	DockercfgTokenDeletedControllerName: startDockercfgTokenDeletedController,
}

func startDockercfgTokenDeletedController(ctx *ControllerContext) (bool, error) {
	go serviceaccountcontrollers.NewDockercfgTokenDeletedController(
		ctx.ClientBuilder.ClientOrDie(DockercfgTokenDeletedControllerName),
		serviceaccountcontrollers.DockercfgTokenDeletedControllerOptions{},
	).Run(ctx.Stop) // launch the controller in a separate go routine so we do not block
	return true, nil
}

func StartControllers(clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}, cm *options.CMServer) error {
	ctx := &ControllerContext{
		ClientBuilder: clientBuilder,
		Stop:          stop,
	}

	for controllerName, initFn := range controllers {
		time.Sleep(wait.Jitter(cm.ControllerStartInterval.Duration, app.ControllerStartJitter))

		glog.V(1).Infof("Starting %q", controllerName)
		started, err := initFn(ctx)
		if err != nil {
			glog.Errorf("Error starting %q", controllerName)
			return err
		}
		if !started {
			glog.Warningf("Skipping %q", controllerName)
			continue
		}
		glog.Infof("Started %q", controllerName)
	}

	return nil
}
