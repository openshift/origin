package controlplane

import (
	"context"
	"fmt"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/disruption/backend"
	disruptionci "github.com/openshift/origin/pkg/disruption/ci"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	imagev1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func StartAPIMonitoringUsingNewBackend(
	ctx context.Context,
	recorder monitorapi.Recorder,
	clusterConfig *rest.Config,
	kubeClient kubernetes.Interface,
	lb backend.LoadBalancerType,
	source string) ([]disruptionci.Sampler, error) {

	samplers := []disruptionci.Sampler{}
	errs := []error{}
	factory := disruptionci.NewDisruptionTestFactory(clusterConfig, kubeClient)

	ns, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), "default", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/api/v1/namespaces/default?resourceVersion=%s", ns.ResourceVersion)

	sampler, err := createKubeAPIMonitoringWithNewConnectionsHTTP2(factory, lb, path, source)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createKubeAPIMonitoringWithConnectionReuseHTTP2(factory, lb, path, source)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createKubeAPIMonitoringWithNewConnectionsHTTP1(factory, lb, path, source)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createKubeAPIMonitoringWithConnectionReuseHTTP1(factory, lb, path, source)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	client, err := imagev1.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}
	streams, err := client.ImageStreams("openshift").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(streams.Items) == 0 {
		return nil, fmt.Errorf("no imagestreams found")
	}
	stream := streams.Items[0]
	path = fmt.Sprintf("/apis/image.openshift.io/v1/namespaces/openshift/imagestreams/%s?resourceVersion=%s", stream.Name, stream.ResourceVersion)

	sampler, err = createOpenShiftAPIMonitoringWithNewConnectionsHTTP2(factory, lb, path, source)
	samplers = append(samplers, sampler)
	errs = append(errs, err)

	sampler, err = createOpenShiftAPIMonitoringWithConnectionReuseHTTP2(factory, lb, path, source)
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

func createKubeAPIMonitoringWithNewConnectionsHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType, path string, source string) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         path,
		Timeout:                      15 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
		Source:                       source,
	})
}

func createKubeAPIMonitoringWithConnectionReuseHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType, path string, source string) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         path,
		Timeout:                      15 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
		Source:                       source,
	})
}

func createKubeAPIMonitoringWithNewConnectionsHTTP1(factory disruptionci.Factory, lb backend.LoadBalancerType, path string, source string) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP1,
		},
		Path:                         path,
		Timeout:                      15 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
		Source:                       source,
	})
}

func createKubeAPIMonitoringWithConnectionReuseHTTP1(factory disruptionci.Factory, lb backend.LoadBalancerType, path string, source string) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.KubeAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP1,
		},
		Path:                         path,
		Timeout:                      15 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
		Source:                       source,
	})
}

func createOpenShiftAPIMonitoringWithNewConnectionsHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType, path string, source string) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.OpenShiftAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         path,
		Timeout:                      15 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
		Source:                       source,
	})
}

func createOpenShiftAPIMonitoringWithConnectionReuseHTTP2(factory disruptionci.Factory, lb backend.LoadBalancerType, path string, source string) (disruptionci.Sampler, error) {
	return factory.New(disruptionci.TestConfiguration{
		TestDescriptor: disruptionci.TestDescriptor{
			TargetServer:     disruptionci.OpenShiftAPIServer,
			LoadBalancerType: lb,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         path,
		Timeout:                      15 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
		Source:                       source,
	})
}
