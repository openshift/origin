package service

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/service"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// UpgradeTest tests that a service is available before, during, and
// after a cluster upgrade.
type UpgradeTest struct {
	jig        *service.TestJig
	tcpService *v1.Service
}

func (UpgradeTest) Name() string { return "k8s-service-lb-available" }
func (UpgradeTest) DisplayName() string {
	return "Application behind service load balancer with PDB is not disrupted"
}

func shouldTestPDBs() bool { return true }

// Setup creates a service with a load balancer and makes sure it's reachable.
func (t *UpgradeTest) Setup(f *framework.Framework) {
	serviceName := "service-test"
	jig := service.NewTestJig(f.ClientSet, f.Namespace.Name, serviceName)

	ns := f.Namespace
	cs := f.ClientSet

	ginkgo.By("creating a TCP service " + serviceName + " with type=LoadBalancer in namespace " + ns.Name)
	tcpService, err := jig.CreateTCPService(func(s *v1.Service) {
		s.Spec.Type = v1.ServiceTypeLoadBalancer
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		// We tune the LB checks to match the longest intervals available so that interactions between
		// upgrading components and the service are more obvious.
		// - AWS allows configuration, default is 70s (6 failed with 10s interval in 1.17) set to match GCP
		s.Annotations["service.beta.kubernetes.io/aws-load-balancer-healthcheck-interval"] = "8"
		s.Annotations["service.beta.kubernetes.io/aws-load-balancer-healthcheck-unhealthy-threshold"] = "3"
		s.Annotations["service.beta.kubernetes.io/aws-load-balancer-healthcheck-healthy-threshold"] = "2"
		// - Azure is hardcoded to 15s (2 failed with 5s interval in 1.17) and is sufficient
		// - GCP has a non-configurable interval of 32s (3 failed health checks with 8s interval in 1.17)
		//   - thus pods need to stay up for > 32s, so pod shutdown period will will be 45s
	})
	framework.ExpectNoError(err)
	tcpService, err = jig.WaitForLoadBalancer(service.GetServiceLoadBalancerCreationTimeout(cs))
	framework.ExpectNoError(err)

	// Get info to hit it with
	tcpIngressIP := service.GetIngressPoint(&tcpService.Status.LoadBalancer.Ingress[0])
	svcPort := int(tcpService.Spec.Ports[0].Port)

	ginkgo.By("creating RC to be part of service " + serviceName)
	rc, err := jig.Run(func(rc *v1.ReplicationController) {
		// ensure the pod waits long enough for most LBs to take it out of rotation, which has to be
		// longer than the LB failed health check interval
		rc.Spec.Template.Spec.Containers[0].Lifecycle = &v1.Lifecycle{
			PreStop: &v1.Handler{
				Exec: &v1.ExecAction{Command: []string{"sleep", "45"}},
			},
		}
		// ensure the pod is not forcibly deleted at 30s, but waits longer than the graceful sleep
		minute := int64(60)
		rc.Spec.Template.Spec.TerminationGracePeriodSeconds = &minute

		jig.AddRCAntiAffinity(rc)
	})
	framework.ExpectNoError(err)

	if shouldTestPDBs() {
		ginkgo.By("creating a PodDisruptionBudget to cover the ReplicationController")
		_, err = jig.CreatePDB(rc)
		framework.ExpectNoError(err)
	}

	// Hit it once before considering ourselves ready
	ginkgo.By("hitting pods through the service's LoadBalancer")
	timeout := service.LoadBalancerLagTimeoutAWS
	// require five passing requests to continue (in case the SLB becomes available and then degrades)
	for i := 0; i < 5; i++ {
		service.TestReachableHTTP(tcpIngressIP, svcPort, timeout)
	}

	t.jig = jig
	t.tcpService = tcpService
}

// Test runs a connectivity check to the service.
func (t *UpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	client, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	stopCh := make(chan struct{})
	defer close(stopCh)
	newBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: client.EventsV1beta1().Events("")})
	r := newBroadcaster.NewRecorder(scheme.Scheme, "openshift.io/upgrade-test-service")
	newBroadcaster.StartRecordingToSink(stopCh)

	ginkgo.By("continuously hitting pods through the service's LoadBalancer")

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(1 * time.Second)
	err = startEndpointMonitoring(ctx, m, t.tcpService, r)
	framework.ExpectNoError(err, "unable to monitor API")

	start := time.Now()
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	ginkgo.By("waiting for any post disruption failures")
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	disruption.ExpectNoDisruption(f, 0.02, end.Sub(start), m.Events(time.Time{}, time.Time{}), "Service was unreachable during disruption")

	// verify finalizer behavior
	defer func() {
		ginkgo.By("Check that service can be deleted with finalizer")
		service.WaitForServiceDeletedWithFinalizer(t.jig.Client, t.tcpService.Namespace, t.tcpService.Name)
	}()
	ginkgo.By("Check that finalizer is present on loadBalancer type service")
	service.WaitForServiceUpdatedWithFinalizer(t.jig.Client, t.tcpService.Namespace, t.tcpService.Name, true)
}

// Teardown cleans up any remaining resources.
func (t *UpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

func startEndpointMonitoring(ctx context.Context, m *monitor.Monitor, svc *v1.Service, r events.EventRecorder) error {
	tcpIngressIP := service.GetIngressPoint(&svc.Status.LoadBalancer.Ingress[0])
	svcPort := int(svc.Spec.Ports[0].Port)

	// this client reuses connections and detects abrupt breaks
	continuousClient, err := rest.UnversionedRESTClientFor(&rest.Config{
		Host:    fmt.Sprintf("http://%s", net.JoinHostPort(tcpIngressIP, strconv.Itoa(svcPort))),
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 15 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 15 * time.Second,
		},
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(scheme.Codecs.SupportedMediaTypes()[0]),
		},
	})
	if err != nil {
		return err
	}
	m.AddSampler(
		monitor.StartSampling(ctx, m, time.Second, func(previous bool) (condition *monitor.Condition, next bool) {
			data, err := continuousClient.Get().AbsPath("echo").Param("msg", "Hello").DoRaw()
			if err == nil && !bytes.Contains(data, []byte("Hello")) {
				err = fmt.Errorf("service returned success but did not contain the correct body contents: %q", string(data))
			}
			switch {
			case err == nil && !previous:
				condition = &monitor.Condition{
					Level:   monitor.Info,
					Locator: locateService(svc),
					Message: "Service started responding to GET requests on reused connections",
				}
			case err != nil && previous:
				framework.Logf("Service %s is unreachable on reused connections: %v", svc.Name, err)
				r.Eventf(&v1.ObjectReference{Kind: "Service", Namespace: "kube-system", Name: "service-upgrade-test"}, nil, v1.EventTypeWarning, "Unreachable", "detected", "on reused connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locateService(svc),
					Message: "Service stopped responding to GET requests on reused connections",
				}
			case err != nil:
				framework.Logf("Service %s is unreachable on reused connections: %v", svc.Name, err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locateService(svc),
			Message: "Service is not responding to GET requests on reused connections",
		}),
	)

	// this client creates fresh connections and detects failure to establish connections
	client, err := rest.UnversionedRESTClientFor(&rest.Config{
		Host:    fmt.Sprintf("http://%s", net.JoinHostPort(tcpIngressIP, strconv.Itoa(svcPort))),
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: -1,
			}).Dial,
			TLSHandshakeTimeout: 15 * time.Second,
			IdleConnTimeout:     15 * time.Second,
			DisableKeepAlives:   true,
		},
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(scheme.Codecs.SupportedMediaTypes()[0]),
		},
	})
	if err != nil {
		return err
	}
	m.AddSampler(
		monitor.StartSampling(ctx, m, time.Second, func(previous bool) (condition *monitor.Condition, next bool) {
			data, err := client.Get().AbsPath("echo").Param("msg", "Hello").DoRaw()
			if err == nil && !bytes.Contains(data, []byte("Hello")) {
				err = fmt.Errorf("service returned success but did not contain the correct body contents: %q", string(data))
			}
			switch {
			case err == nil && !previous:
				condition = &monitor.Condition{
					Level:   monitor.Info,
					Locator: locateService(svc),
					Message: "Service started responding to GET requests over new connections",
				}
			case err != nil && previous:
				framework.Logf("Service %s is unreachable on new connections: %v", svc.Name, err)
				r.Eventf(&v1.ObjectReference{Kind: "Service", Namespace: "kube-system", Name: "service-upgrade-test"}, nil, v1.EventTypeWarning, "Unreachable", "detected", "on new connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locateService(svc),
					Message: "Service stopped responding to GET requests over new connections",
				}
			case err != nil:
				framework.Logf("Service %s is unreachable on new connections: %v", svc.Name, err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locateService(svc),
			Message: "Service is not responding to GET requests over new connections",
		}),
	)
	return nil
}

func locateService(svc *v1.Service) string {
	return fmt.Sprintf("ns/%s svc/%s", svc.Namespace, svc.Name)
}
