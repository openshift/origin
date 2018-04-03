// +build !linux

package unidler

import (
	"fmt"

	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/util/iptables"
)

func NewUnidlerProxier(iptables iptables.Interface, queueNumber uint16, markBit uint, signaler NeedPodsSignaler, waitForRelease func()) (*UnidlerProxy, error) {
	return nil, fmt.Errorf("network-based unidling is not supported on this platform")
}

type UnidlerProxy struct {
}

func (p *UnidlerProxy) RunUntil(stopCh <-chan struct{}) error {
	return fmt.Errorf("network-based unidling is not supported on this platform")
}

func (p *UnidlerProxy) OnServiceAdd(service *api.Service) {
}

func (p *UnidlerProxy) OnServiceUpdate(oldService, service *api.Service) {
}

func (p *UnidlerProxy) OnServiceDelete(service *api.Service) {
}

func (p *UnidlerProxy) OnServiceSynced() {}

func CleanupLeftovers(ipt iptables.Interface, markBit uint) error {
	return nil
}
