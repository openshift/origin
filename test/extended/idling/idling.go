package idling

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	unidlingproxy "github.com/openshift/origin/pkg/proxy/unidler"
	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

func tryEchoUDPOnce(ip net.IP, udpPort int, expectedBuff []byte, readTimeout time.Duration) ([]byte, error) {
	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: ip, Port: udpPort})
	if err != nil {
		return nil, fmt.Errorf("unable to connect to service: %v", err)
	}
	defer conn.Close()

	var n int
	if n, err = conn.Write(expectedBuff); err != nil {
		// It's technically possible to get some errors on write while switching over
		return nil, nil
	} else if n != len(expectedBuff) {
		return nil, fmt.Errorf("unable to write entire %v bytes to UDP echo server socket", len(expectedBuff))
	}

	if err = conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return nil, fmt.Errorf("unable to set deadline on read from echo server: %v", err)
	}

	actualBuff := make([]byte, n)
	var amtRead int
	amtRead, _, err = conn.ReadFromUDP(actualBuff)
	if err != nil {
		return nil, fmt.Errorf("unable to read from UDP echo server: %v", err)
	} else if amtRead != n {
		// we should never read back the *wrong* thing
		return nil, fmt.Errorf("read back incorrect number of bytes from echo server")
	}

	if string(expectedBuff) != string(actualBuff) {
		return nil, fmt.Errorf("written contents %q didn't equal read contents %q from echo server: %v", string(expectedBuff), string(actualBuff), err)
	}

	return actualBuff, nil
}

func tryEchoUDP(svc *kapiv1.Service) error {
	rawIP := svc.Spec.ClusterIP
	o.Expect(rawIP).NotTo(o.BeEmpty(), "The service should have a cluster IP set")
	ip := net.ParseIP(rawIP)
	o.Expect(ip).NotTo(o.BeNil(), "The service should have a valid cluster IP, but %q was not valid", rawIP)

	var udpPort int
	for _, port := range svc.Spec.Ports {
		if port.Protocol == "UDP" {
			udpPort = int(port.Port)
			break
		}
	}
	o.Expect(udpPort).NotTo(o.Equal(0), "The service should have a UDP port exposed")

	// For UDP, we just drop packets on the floor rather than queue them up
	readTimeout := 5 * time.Second

	expectedBuff := []byte("It's time to UDP!\n")
	o.Eventually(func() ([]byte, error) { return tryEchoUDPOnce(ip, udpPort, expectedBuff, readTimeout) }, 2*time.Minute, readTimeout).Should(o.Equal(expectedBuff))

	return nil
}

func tryEchoTCP(svc *kapiv1.Service) error {
	rawIP := svc.Spec.ClusterIP
	if rawIP == "" {
		return fmt.Errorf("no ClusterIP specified on service %s", svc.Name)
	}
	ip := net.ParseIP(rawIP)

	var tcpPort int
	for _, port := range svc.Spec.Ports {
		if port.Protocol == "TCP" {
			tcpPort = int(port.Port)
			break
		}
	}

	if tcpPort == 0 {
		return fmt.Errorf("Unable to find any TCP ports on service %s", svc.Name)
	}

	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: ip, Port: tcpPort})
	if err != nil {
		return fmt.Errorf("unable to connect to service %s: %v", svc.Name, err)
	}

	if err = conn.SetDeadline(time.Now().Add(2 * time.Minute)); err != nil {
		return fmt.Errorf("unable to set timeout on TCP connection to service %s: %v", svc.Name, err)
	}

	expectedBuff := []byte("It's time to TCP!\n")
	var n int
	if n, err = conn.Write(expectedBuff); err != nil {
		return fmt.Errorf("unable to write data to echo server for service %s: %v", svc.Name, err)
	} else if n != len(expectedBuff) {
		return fmt.Errorf("unable to write all data to echo server for service %s", svc.Name)
	}

	actualBuff := make([]byte, n)
	var amtRead int
	amtRead, err = conn.Read(actualBuff)
	if err != nil {
		return fmt.Errorf("unable to read data from echo server for service %s: %v", svc.Name, err)
	} else if amtRead != n {
		return fmt.Errorf("unable to read all data written from echo server for service %s: %v", svc.Name, err)
	}

	if string(expectedBuff) != string(actualBuff) {
		return fmt.Errorf("written contents %q didn't equal read contents %q from echo server for service %s: %v", string(expectedBuff), string(actualBuff), svc.Name, err)
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

func checkSingleIdle(oc *exutil.CLI, idlingFile string, resources map[string][]string, resourceName string, kind string) {
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
	endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	o.Expect(endpoints.Annotations).To(o.HaveKey(unidlingapi.IdledAtAnnotation))
	o.Expect(endpoints.Annotations).To(o.HaveKey(unidlingapi.UnidleTargetAnnotation))

	g.By("Checking the idled-at time")
	idledAtAnnotation := endpoints.Annotations[unidlingapi.IdledAtAnnotation]
	idledAtTime, err := time.Parse(time.RFC3339, idledAtAnnotation)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(idledAtTime).To(o.BeTemporally("~", time.Now(), 5*time.Minute))

	g.By("Checking the idle targets")
	unidleTargetAnnotation := endpoints.Annotations[unidlingapi.UnidleTargetAnnotation]
	unidleTargets := []unidlingapi.RecordedScaleReference{}
	err = json.Unmarshal([]byte(unidleTargetAnnotation), &unidleTargets)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(unidleTargets).To(o.Equal([]unidlingapi.RecordedScaleReference{
		{
			Replicas: 2,
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Name: resources[resourceName][0],
				Kind: kind,
			},
		},
	}))
}

var _ = g.Describe("idling and unidling", func() {
	defer g.GinkgoRecover()
	var (
		oc                  = exutil.NewCLI("cli-idling", exutil.KubeConfigPath()).Verbose()
		echoServerFixture   = exutil.FixturePath("testdata", "idling-echo-server.yaml")
		echoServerRcFixture = exutil.FixturePath("testdata", "idling-echo-server-rc.yaml")
		framework           = oc.KubeFramework()
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

		targetFile, err := ioutil.TempFile(exutil.TestContext.OutputDir, "idling-services-")
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

	g.Describe("idling [local]", func() {
		g.Context("with a single service and DeploymentConfig [Conformance]", func() {
			g.BeforeEach(func() {
				framework.BeforeEach()
				fixture = echoServerFixture
			})

			g.It("should idle the service and DeploymentConfig properly", func() {
				checkSingleIdle(oc, idlingFile, resources, "deploymentconfig", "DeploymentConfig")
			})
		})

		g.Context("with a single service and ReplicationController", func() {
			g.BeforeEach(func() {
				framework.BeforeEach()
				fixture = echoServerRcFixture
			})

			g.It("should idle the service and ReplicationController properly", func() {
				checkSingleIdle(oc, idlingFile, resources, "replicationcontroller", "ReplicationController")
			})
		})
	})

	g.Describe("unidling", func() {
		g.BeforeEach(func() {
			framework.BeforeEach()
			fixture = echoServerFixture
		})

		g.It("should work with TCP (when fully idled) [Conformance] [local]", func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsAvailable(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			err = tryEchoTCP(svc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		g.It("should work with TCP (while idling) [local]", func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and repeatedly connecting, making sure we seamlessly idle and come back up")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			o.Consistently(func() error { return tryEchoTCP(svc) }, 10*time.Second, 500*time.Millisecond).ShouldNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		g.It("should handle many TCP connections by dropping those under a certain bound [local]", func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			serviceName := resources["service"][0]
			err = exutil.WaitForNoPodsAvailable(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP many times and checking the echo")
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			connectionsToStart := 100

			errors := make([]error, connectionsToStart)
			var connWG sync.WaitGroup
			// spawn many connections
			for i := 0; i < connectionsToStart; i++ {
				connWG.Add(1)
				go func(ind int) {
					defer connWG.Done()
					err = tryEchoTCP(svc)
					errors[ind] = err
				}(i)
			}

			connWG.Wait()

			g.By(fmt.Sprintf("Expecting all but %v of those connections to fail", unidlingproxy.MaxHeldConnections))
			errCount := 0
			for _, err := range errors {
				if err != nil {
					errCount++
				}
			}
			o.Expect(errCount).To(o.Equal(connectionsToStart - unidlingproxy.MaxHeldConnections))

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		g.It("should work with UDP [local]", func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsAvailable(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			err = tryEchoUDP(svc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting until we have endpoints")
			err = exutil.WaitForEndpointsAvailable(oc, serviceName)
			o.Expect(err).ToNot(o.HaveOccurred())

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})

		// TODO: Work out how to make this test work correctly when run on AWS
		g.XIt("should handle many UDP senders (by continuing to drop all packets on the floor) [local]", func() {
			g.By("Idling the service")
			_, err := oc.Run("idle").Args("--resource-names-file", idlingFile).Output()
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Waiting for the pods to have terminated")
			err = exutil.WaitForNoPodsAvailable(oc)
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Connecting to the service IP many times and checking the echo")
			serviceName := resources["service"][0]
			svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			connectionsToStart := 100
			errors := make([]error, connectionsToStart)
			var connWG sync.WaitGroup
			// spawn many connectors
			for i := 0; i < connectionsToStart; i++ {
				connWG.Add(1)
				go func(ind int) {
					defer g.GinkgoRecover()
					defer connWG.Done()
					err = tryEchoUDP(svc)
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

			endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Making sure the endpoints are no longer marked as idled")
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.IdledAtAnnotation))
			o.Expect(endpoints.Annotations).NotTo(o.HaveKey(unidlingapi.UnidleTargetAnnotation))
		})
	})
})
