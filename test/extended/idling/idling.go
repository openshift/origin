package idling

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	admissionapi "k8s.io/pod-security-admission/api"

	unidlingapi "github.com/openshift/api/unidling/v1alpha1"

	exutil "github.com/openshift/origin/test/extended/util"
)

func tryEchoUDP(svc *kapiv1.Service, execPod *kapiv1.Pod) error {
	rawIP := svc.Spec.ClusterIP
	o.Expect(rawIP).NotTo(o.BeEmpty(), "The service should have a cluster IP set")

	var udpPort int
	for _, port := range svc.Spec.Ports {
		if port.Protocol == "UDP" {
			udpPort = int(port.Port)
			break
		}
	}
	o.Expect(udpPort).NotTo(o.Equal(0), "The service should have a UDP port exposed")

	// NB: netexec's UDP echo test lowercasifies the input string, so we just pass
	// an all-lowercase string for simplicity.
	expected := "it is time to udp."

	// For UDP, we just drop packets on the floor rather than queue them up
	// so use a shorter timeout
	readTimeout := 5 * time.Second
	cmd := fmt.Sprintf("echo -n \"echo %s\" | nc -w 5 -u %s %d", expected, rawIP, udpPort)

	o.Eventually(func() (string, error) {
		return e2eoutput.RunHostCmd(execPod.Namespace, execPod.Name, cmd)
	}, 2*time.Minute, readTimeout).Should(o.Equal(expected))

	return nil
}

func tryEchoHTTP(svc *kapiv1.Service, execPod *kapiv1.Pod) error {
	rawIP := svc.Spec.ClusterIP
	if rawIP == "" {
		return fmt.Errorf("no ClusterIP specified on service %s", svc.Name)
	}

	var tcpPort string
	for _, port := range svc.Spec.Ports {
		if port.Protocol == "TCP" {
			tcpPort = fmt.Sprintf("%d", port.Port)
			break
		}
	}

	if tcpPort == "" {
		return fmt.Errorf("Unable to find any TCP ports on service %s", svc.Name)
	}

	expected := "It is time to TCP."
	cmd := fmt.Sprintf("curl --retry-max-time 120 --retry-connrefused --retry 20 --max-time 5 -s -g http://%s/echo?msg=%s",
		net.JoinHostPort(rawIP, tcpPort),
		url.QueryEscape(expected),
	)
	out, err := e2eoutput.RunHostCmd(execPod.Namespace, execPod.Name, cmd)
	if err != nil {
		return fmt.Errorf("exec failed: %v\noutput: %s", err, out)
	}

	if out != expected {
		return fmt.Errorf("written contents %q didn't equal read contents %q from echo server for service %s: %v", string(expected), string(out), svc.Name, err)
	}

	return nil
}

func createFixture(oc *exutil.CLI, path string) ([]string, []string, error) {
	output, err := oc.Run("create").Args("-f", path, "-o", "name").Output()
	if err != nil {
		return nil, nil, err
	}

	lines := strings.Split(output, "\n")

	resources := make([]string, 0, len(lines)-1)
	names := make([]string, 0, len(lines)-1)

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("expected type/name syntax, got: %q", line)
		}
		resources = append(resources, parts[0])
		names = append(names, parts[1])
	}

	return resources, names, nil
}

func checkSingleIdle(oc *exutil.CLI, idlingFile string, resources map[string][]string, resourceName, kind, group string) {
	g.By("Idling the service")
	_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
	o.Expect(err).ToNot(o.HaveOccurred())

	g.By("Ensuring the scale is zero (and stays zero)")
	objName := resources[resourceName][0]
	// make sure we don't get woken up by an incorrect router health check or anything like that
	o.Consistently(func() (string, error) {
		return oc.Run("get").Args(resourceName+"/"+objName, "--output=jsonpath=\"{.spec.replicas}\"").Output()
	}, 20*time.Second, 500*time.Millisecond).Should(o.ContainSubstring("0"))

	g.By("Fetching the service and checking the annotations are present")
	serviceName := resources["service"][0]
	services, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(services.Annotations).To(o.HaveKey(unidlingapi.IdledAtAnnotation))
	o.Expect(services.Annotations).To(o.HaveKey(unidlingapi.UnidleTargetAnnotation))

	g.By("Checking the idled-at time")
	idledAtAnnotation := services.Annotations[unidlingapi.IdledAtAnnotation]
	idledAtTime, err := time.Parse(time.RFC3339, idledAtAnnotation)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(idledAtTime).To(o.BeTemporally("~", time.Now(), 5*time.Minute))

	g.By("Checking the idle targets")
	unidleTargetAnnotation := services.Annotations[unidlingapi.UnidleTargetAnnotation]
	unidleTargets := []unidlingapi.RecordedScaleReference{}
	err = json.Unmarshal([]byte(unidleTargetAnnotation), &unidleTargets)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(unidleTargets).To(o.Equal([]unidlingapi.RecordedScaleReference{
		{
			Replicas: 2,
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Name:  resources[resourceName][0],
				Kind:  kind,
				Group: group,
			},
		},
	}))
}

var _ = g.Describe("[sig-network-edge][Feature:Idling]", func() {
	defer g.GinkgoRecover()
	var (
		oc                          = exutil.NewCLIWithPodSecurityLevel("cli-idling", admissionapi.LevelBaseline).Verbose()
		echoServerFixture           = exutil.FixturePath("testdata", "idling-echo-server.yaml")
		echoServerFixtureDeployment = exutil.FixturePath("testdata", "idling-echo-server-deployment.yaml")
		echoServerRcFixture         = exutil.FixturePath("testdata", "idling-echo-server-rc.yaml")
		framework                   = oc.KubeFramework()
	)

	const (
		connectionsToStart       = 20
		numExecPods              = 5
		minSuccessfulConnections = 16
	)

	// path to the fixture
	var fixture string

	// path to the idling file
	var idlingFile string

	// map of all resources created from the fixtures
	var resources map[string][]string

	g.JustBeforeEach(func() {
		g.By("Creating the resources")
		rawResources, rawResourceNames, err := createFixture(oc, fixture)
		o.Expect(err).ToNot(o.HaveOccurred())

		resources = make(map[string][]string)
		for i, resource := range rawResources {
			resources[resource] = append(resources[resource], rawResourceNames[i])
		}

		g.By("Creating the idling file")
		serviceNames := resources["service"]

		targetFile, err := ioutil.TempFile("", "idling-services-")
		o.Expect(err).ToNot(o.HaveOccurred())
		defer targetFile.Close()
		idlingFile = targetFile.Name()
		_, err = targetFile.Write([]byte(strings.Join(serviceNames, "\n")))
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("Waiting for the endpoints to exist")
		serviceName := resources["service"][0]
		g.By("Waiting for endpoints to be up")
		err = exutil.WaitForEndpointsAvailable(oc, serviceName)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.AfterEach(func() {
		g.By("Cleaning up the idling file")
		os.Remove(idlingFile)
	})

	g.Describe("Idling", func() {
		g.Context("with a single service and DeploymentConfig [apigroup:route.openshift.io]", func() {
			g.BeforeEach(func() {
				fixture = echoServerFixture
			})

			g.It("should idle the service and DeploymentConfig properly [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
				checkSingleIdle(oc, idlingFile, resources, "deploymentconfig.apps.openshift.io", "DeploymentConfig", "apps.openshift.io")
			})
		})

		g.Context("with a single service and ReplicationController", func() {
			g.BeforeEach(func() {
				fixture = echoServerRcFixture
			})

			g.It("should idle the service and ReplicationController properly", g.Label("Size:M"), func() {
				checkSingleIdle(oc, idlingFile, resources, "replicationcontroller", "ReplicationController", "")
			})
		})
	})

	g.Describe("Unidling [apigroup:apps.openshift.io][apigroup:route.openshift.io]", func() {
		g.BeforeEach(func() {
			fixture = echoServerFixture
		})

		g.It("should work with TCP (when fully idled)", g.Label("Size:M"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			execPod := e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, "execpod", nil)
			err = tryEchoHTTP(svc, execPod)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		g.It("should work with TCP (while idling)", g.Label("Size:M"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and repeatedly connecting, making sure we seamlessly idle and come back up")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			execPod := e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, "execpod", nil)
			o.Consistently(func() error { return tryEchoHTTP(svc, execPod) }, 10*time.Second, 500*time.Millisecond).ShouldNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		// This is [Serial] because we really want to spam the service, and that
		// seems to disrupt the cluster if we do it in the parallel suite.
		g.It("should handle many TCP connections by possibly dropping those over a certain bound [Serial]", g.Label("Size:L"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			serviceName := resources["service"][0]
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP many times and checking the echo")
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			var execPods [numExecPods]*kapiv1.Pod
			for i := range execPods {
				execPods[i] = e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, fmt.Sprintf("execpod-%d", i+1), nil)
			}

			errors := make([]error, connectionsToStart)
			var connWG sync.WaitGroup
			// spawn many connections over a span of 1 second
			for i := 0; i < connectionsToStart; i++ {
				connWG.Add(1)
				go func(ind int) {
					defer g.GinkgoRecover()
					defer connWG.Done()
					time.Sleep(time.Duration(ind) * (time.Second / connectionsToStart))
					err = tryEchoHTTP(svc, execPods[ind%numExecPods])
					errors[ind] = err
				}(i)
			}

			connWG.Wait()

			g.By(fmt.Sprintf("Expecting at least %d of those connections to succeed", minSuccessfulConnections))
			successCount := 0
			for _, err := range errors {
				if err == nil {
					successCount++
				}
			}
			o.Expect(successCount).To(o.BeNumerically(">=", minSuccessfulConnections))

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		g.It("should work with UDP", g.Label("Size:M"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			execPod := e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, "execpod", nil)
			err = tryEchoUDP(svc, execPod)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		// This is [Serial] because we really want to spam the service, and that
		// seems to disrupt the cluster if we do it in the parallel suite.
		g.It("should handle many UDP senders (by continuing to drop all packets on the floor) [Serial]", g.Label("Size:L"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP many times and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			var execPods [numExecPods]*kapiv1.Pod
			for i := range execPods {
				execPods[i] = e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, fmt.Sprintf("execpod-%d", i+1), nil)
			}

			errors := make([]error, connectionsToStart)
			var connWG sync.WaitGroup
			// spawn many connections over a span of 1 second
			for i := 0; i < connectionsToStart; i++ {
				connWG.Add(1)
				go func(ind int) {
					defer g.GinkgoRecover()
					defer connWG.Done()
					time.Sleep(time.Duration(ind) * (time.Second / connectionsToStart))
					err = tryEchoUDP(svc, execPods[ind%numExecPods])
					errors[ind] = err
				}(i)
			}

			connWG.Wait()

			// all of the echoers should eventually succeed
			errCount := 0
			for _, err := range errors {
				if err != nil {
					errCount++
				}
			}
			o.Expect(errCount).To(o.Equal(0))

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})
	})

	g.Describe("Unidling with Deployments [apigroup:route.openshift.io]", func() {
		g.BeforeEach(func() {
			fixture = echoServerFixtureDeployment
		})

		g.It("should work with TCP (when fully idled)", g.Label("Size:M"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			execPod := e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, "execpod", nil)
			err = tryEchoHTTP(svc, execPod)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		g.It("should work with TCP (while idling)", g.Label("Size:M"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and repeatedly connecting, making sure we seamlessly idle and come back up")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			execPod := e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, "execpod", nil)
			o.Consistently(func() error { return tryEchoHTTP(svc, execPod) }, 10*time.Second, 500*time.Millisecond).ShouldNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		// This is [Serial] because we really want to spam the service, and that
		// seems to disrupt the cluster if we do it in the parallel suite.
		g.It("should handle many TCP connections by possibly dropping those over a certain bound [Serial]", g.Label("Size:L"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			serviceName := resources["service"][0]
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP many times and checking the echo")
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			var execPods [numExecPods]*kapiv1.Pod
			for i := range execPods {
				execPods[i] = e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, fmt.Sprintf("execpod-%d", i+1), nil)
			}

			errors := make([]error, connectionsToStart)
			var connWG sync.WaitGroup
			// spawn many connections over a span of 1 second
			for i := 0; i < connectionsToStart; i++ {
				connWG.Add(1)
				go func(ind int) {
					defer g.GinkgoRecover()
					defer connWG.Done()
					time.Sleep(time.Duration(ind) * (time.Second / connectionsToStart))
					err = tryEchoHTTP(svc, execPods[ind%numExecPods])
					errors[ind] = err
				}(i)
			}

			connWG.Wait()

			g.By(fmt.Sprintf("Expecting at least %d of those connections to succeed", minSuccessfulConnections))
			successCount := 0
			for _, err := range errors {
				if err == nil {
					successCount++
				}
			}
			o.Expect(successCount).To(o.BeNumerically(">=", minSuccessfulConnections))

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		g.It("should work with UDP", g.Label("Size:M"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			execPod := e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, "execpod", nil)
			err = tryEchoUDP(svc, execPod)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		// This is [Serial] because we really want to spam the service, and that
		// seems to disrupt the cluster if we do it in the parallel suite.
		g.It("should handle many UDP senders (by continuing to drop all packets on the floor) [Serial]", g.Label("Size:L"), func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsRunning(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP many times and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			var execPods [numExecPods]*kapiv1.Pod
			for i := range execPods {
				execPods[i] = e2epod.CreateExecPodOrFail(context.TODO(), framework.ClientSet, framework.Namespace.Name, fmt.Sprintf("execpod-%d", i+1), nil)
			}

			errors := make([]error, connectionsToStart)
			var connWG sync.WaitGroup
			// spawn many connections over a span of 1 second
			for i := 0; i < connectionsToStart; i++ {
				connWG.Add(1)
				go func(ind int) {
					defer g.GinkgoRecover()
					defer connWG.Done()
					time.Sleep(time.Duration(ind) * (time.Second / connectionsToStart))
					err = tryEchoUDP(svc, execPods[ind%numExecPods])
					errors[ind] = err
				}(i)
			}

			connWG.Wait()

			// all of the echoers should eventually succeed
			errCount := 0
			for _, err := range errors {
				if err != nil {
					errCount++
				}
			}
			o.Expect(errCount).To(o.Equal(0))

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})
	})
})
