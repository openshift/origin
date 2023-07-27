package controlplane

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/disruption/backend"

	disruptionci "github.com/openshift/origin/pkg/disruption/ci"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/client-go/rest"
)

func StartAPIMonitoringUsingNewBackend(ctx context.Context, recorder monitorapi.Recorder, clusterConfig *rest.Config, lb backend.LoadBalancerType) error {
	factory := disruptionci.NewDisruptionTestFactory(clusterConfig)
	if err := startKubeAPIMonitoringWithNewConnectionsHTTP2(ctx, recorder, factory, lb); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuseHTTP2(ctx, recorder, factory, lb); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithNewConnectionsHTTP1(ctx, recorder, factory, lb); err != nil {
		return err
	}
	if err := startKubeAPIMonitoringWithConnectionReuseHTTP1(ctx, recorder, factory, lb); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithNewConnectionsHTTP2(ctx, recorder, factory, lb); err != nil {
		return err
	}
	if err := startOpenShiftAPIMonitoringWithConnectionReuseHTTP2(ctx, recorder, factory, lb); err != nil {
		return err
	}
	return nil
}

func startKubeAPIMonitoringWithNewConnectionsHTTP2(ctx context.Context, recorder monitorapi.Recorder, factory disruptionci.Factory, lb backend.LoadBalancerType) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnectionsHTTP2(factory, lb)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, recorder, nil)
}

func startKubeAPIMonitoringWithConnectionReuseHTTP2(ctx context.Context, recorder monitorapi.Recorder, factory disruptionci.Factory, lb backend.LoadBalancerType) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuseHTTP2(factory, lb)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, recorder, nil)
}

func startKubeAPIMonitoringWithNewConnectionsHTTP1(ctx context.Context, recorder monitorapi.Recorder, factory disruptionci.Factory, lb backend.LoadBalancerType) error {
	backendSampler, err := createKubeAPIMonitoringWithNewConnectionsHTTP1(factory, lb)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, recorder, nil)
}

func startKubeAPIMonitoringWithConnectionReuseHTTP1(ctx context.Context, recorder monitorapi.Recorder, factory disruptionci.Factory, lb backend.LoadBalancerType) error {
	backendSampler, err := createKubeAPIMonitoringWithConnectionReuseHTTP1(factory, lb)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, recorder, nil)
}

func startOpenShiftAPIMonitoringWithNewConnectionsHTTP2(ctx context.Context, recorder monitorapi.Recorder, factory disruptionci.Factory, lb backend.LoadBalancerType) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithNewConnectionsHTTP2(factory, lb)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, recorder, nil)
}

func startOpenShiftAPIMonitoringWithConnectionReuseHTTP2(ctx context.Context, recorder monitorapi.Recorder, factory disruptionci.Factory, lb backend.LoadBalancerType) error {
	backendSampler, err := createOpenShiftAPIMonitoringWithConnectionReuseHTTP2(factory, lb)
	if err != nil {
		return err
	}
	return backendSampler.StartEndpointMonitoring(ctx, recorder, nil)
}

func createKubeAPIMonitoringWithNewConnectionsHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createKubeAPIMonitoringWithConnectionReuseHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createKubeAPIMonitoringWithNewConnectionsHTTP1(factory disruptionci.Factory, lb backend.LoadBalancerType) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP1,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createKubeAPIMonitoringWithConnectionReuseHTTP1(factory disruptionci.Factory, lb backend.LoadBalancerType) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP1,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createOpenShiftAPIMonitoringWithNewConnectionsHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.OpenShiftAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/apis/image.openshift.io/v1/namespaces/default/imagestreams",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}

func createOpenShiftAPIMonitoringWithConnectionReuseHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.OpenShiftAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/apis/image.openshift.io/v1/namespaces/default/imagestreams",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}
