package poll_service

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

type ServicePoller struct {
	Address     string
	Port        string
	serviceName string
	recorder    monitorapi.RecorderWriter
	outFile     io.Writer
}

func NewServicePoller(clusterIP, port string, outFile io.Writer) *ServicePoller {
	return &ServicePoller{
		Address: clusterIP,
		Port:    port,
		outFile: outFile,
	}
}

// Run starts polling and blocks until stopCh is closed.
func (p *ServicePoller) Run(ctx context.Context, finishedCleanup chan struct{}) {
	defer utilruntime.HandleCrash()
	defer close(finishedCleanup)

	logger := klog.FromContext(ctx)
	logger.Info("Starting Polling Service")
	defer logger.Info("Shutting down Service Poller")

	//go wait.UntilWithContext(ctx, p.runServicePoller, time.Second)
	go p.runServicePoller(ctx)

	// do a thing

	<-ctx.Done()

}

func (p *ServicePoller) runServicePoller(ctx context.Context) {
	url := fmt.Sprintf("http://%s", net.JoinHostPort(p.Address, p.Port))
	fmt.Fprintf(p.outFile, "Adding and starting: %v on node\n", url)
	sampler := backenddisruption.NewSimpleBackend(
		url,
		fmt.Sprintf("new-connection-to-cluster-ip-%s", p.Address),
		"",
		monitorapi.NewConnectionType,
	)

	sampler.StartEndpointMonitoring(ctx, p.recorder, nil)

	fmt.Fprintf(p.outFile, "Successfully started polling %s\n", p.Address)

}
