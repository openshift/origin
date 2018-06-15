package openshift

import (
	"fmt"
)

type errPortsNotAvailable struct {
	ports []int
}

func (e *errPortsNotAvailable) Error() string {
	return fmt.Sprintf("ports in use: %v", e.ports)
}

func ErrPortsNotAvailable(ports []int) error {
	return &errPortsNotAvailable{
		ports: ports,
	}
}

func IsPortsNotAvailableErr(err error) bool {
	_, ok := err.(*errPortsNotAvailable)
	return ok
}

func UnavailablePorts(err error) []int {
	e, ok := err.(*errPortsNotAvailable)
	if !ok {
		return []int{}
	}
	return e.ports
}
