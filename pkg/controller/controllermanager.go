package controller

import (
	serviceaccountcontrollers "github.com/openshift/origin/pkg/serviceaccounts/controllers"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	DockercfgTokenDeletedControllerName = "dockercfg-token-deleted-controller"
)

type ControllerContext struct {
	ClientBuilder controller.ControllerClientBuilder
	Stop          <-chan struct{}
}

// InitFunc is used to launch a particular controller.  It may run additional "should I activate checks".
// Any error returned will cause the controller process to `Fatal`
// The bool indicates whether the controller was enabled.
type InitFunc func(ctx ControllerContext) (bool, error)

var controllers = map[string]InitFunc{
	DockercfgTokenDeletedControllerName: startDockercfgTokenDeletedController,
}

func GetControllerInitializers() map[string]InitFunc {
	return controllers
}

func startDockercfgTokenDeletedController(ctx ControllerContext) (bool, error) {
	go serviceaccountcontrollers.NewDockercfgTokenDeletedController(
		ctx.ClientBuilder.ClientOrDie(DockercfgTokenDeletedControllerName),
		serviceaccountcontrollers.DockercfgTokenDeletedControllerOptions{},
	).Run(ctx.Stop)
	return true, nil
}
