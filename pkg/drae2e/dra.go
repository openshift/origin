package drae2e

import (
	"context"
)

type Driver interface {
	Name() string
	Setup() error
	Cleanup(context.Context) error
	Ready() error
}

type Holder struct {
	Driver Driver
}
