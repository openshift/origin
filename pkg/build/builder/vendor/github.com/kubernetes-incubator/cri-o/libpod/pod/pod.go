package pod

import (
	"github.com/kubernetes-incubator/cri-o/libpod/ctr"
)

// Pod represents a group of containers that may share namespaces
type Pod struct {
	// TODO populate
}

// Start starts all containers within a pod that are not already running
func (p *Pod) Start() error {
	return ctr.ErrNotImplemented
}

// Stop stops all containers within a pod that are not already stopped
func (p *Pod) Stop() error {
	return ctr.ErrNotImplemented
}

// Kill sends a signal to all running containers within a pod
func (p *Pod) Kill(signal uint) error {
	return ctr.ErrNotImplemented
}

// GetContainers retrieves the containers in the pod
func (p *Pod) GetContainers() ([]*ctr.Container, error) {
	return nil, ctr.ErrNotImplemented
}

// Status gets the status of all containers in the pod
// TODO This should return a summary of the states of all containers in the pod
func (p *Pod) Status() error {
	return ctr.ErrNotImplemented
}
