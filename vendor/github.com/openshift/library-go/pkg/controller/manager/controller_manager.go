package manager

import (
	"context"
	"sync"

	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/controller/factory"
)

type ControllerManager interface {
	Start(ctx context.Context)
	WithController(controller factory.Controller, workers int) ControllerManager
}

// NewControllerManager returns new controller manager.
func NewControllerManager() ControllerManager {
	return &controllerManager{}
}

// runnableController represents single controller runnable configuration.
type runnableController struct {
	run          func(ctx context.Context, workers int)
	workersCount int
	name         string
}

type controllerManager struct {
	controllers []runnableController
}

var _ ControllerManager = &controllerManager{}

func (c *controllerManager) WithController(controller factory.Controller, workers int) ControllerManager {
	c.controllers = append(c.controllers, runnableController{
		run:          controller.Run,
		workersCount: workers,
		name:         controller.Name(),
	})
	return c
}

// Start will run all managed controllers and block until all controllers shutdown.
// When the context passed is cancelled, all controllers are signalled to shutdown.
func (c controllerManager) Start(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(c.controllers))
	for i := range c.controllers {
		go func(index int) {
			defer klog.Infof("%s controller terminated", c.controllers[index].name)
			defer wg.Done()
			c.controllers[index].run(ctx, c.controllers[index].workersCount)
		}(i)
	}
	wg.Wait()
}
