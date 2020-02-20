package manager

import (
	"context"

	"github.com/openshift/library-go/pkg/controller/factory"
)

type ControllerManager interface {
	Run(ctx context.Context)
}

type controllerManager struct {
	controllers []factory.Controller
}

func (c *controllerManager) Run(ctx context.Context) {
	for i := range c.controllers {
		go func(index int) {
			// TODO: Usually
			c.controllers[index].Run(ctx, 1)
		}(i)
	}
	panic("implement me")
}
