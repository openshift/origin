package drae2e

import (
	"context"
)

type Driver interface {
	DeviceClassName() string
	Setup() error
	Cleanup(context.Context) error
	Ready() error
}
