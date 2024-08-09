package controlplane

import (
	"context"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/disruption/backend"

	disruptionci "github.com/openshift/origin/pkg/disruption/ci"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/client-go/rest"
)

func StartAPIMonitoringUsingNewBackend(
	ctx context.Context,
	recorder monitorapi.Recorder,
	clusterConfig *rest.Config,
	kubeClient kubernetes.Interface,
	lb backend.LoadBalancerType) ([]disruptionci.Sampler, error) {

	samplers := []disruptionci.Sampler{}
	errs := []error{}
	factory := disruptionci.NewDisruptionTestFactory(clusterConfig, kubeClient)

	sampler, err := createKubeAPIMonitoringWithNewConnectionsHTTP2(factory, lb)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createKubeAPIMonitoringWithConnectionReuseHTTP2(factory, lb)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createKubeAPIMonitoringWithNewConnectionsHTTP1(factory, lb)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createKubeAPIMonitoringWithConnectionReuseHTTP1(factory, lb)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createOpenShiftAPIMonitoringWithNewConnectionsHTTP2(factory, lb)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createOpenShiftAPIMonitoringWithConnectionReuseHTTP2(factory, lb)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	if combinedErr := utilerrors.NewAggregate(errs); combinedErr != nil {
		return nil, combinedErr
	}

	for _, currSampler := range samplers {
		localErr := currSampler.StartEndpointMonitoring(ctx, recorder, nil)
		errs = append(errs, localErr)
	}

	return samplers, utilerrors.NewAggregate(errs)
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
		Timeout:                      15 * time.Second,
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
		Timeout:                      15 * time.Second,
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
		Timeout:                      15 * time.Second,
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
		Timeout:                      15 * time.Second,
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
		Timeout:                      15 * time.Second,
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
		Timeout:                      15 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
}
