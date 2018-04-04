package idling

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	// NB: the default policy does not allow normal users to create or update idlers,
	// so we need admin here
	output, err := oc.AsAdmin().Run("create").Args("-f", path, "-o", "name").Output()
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

var _ = g.Describe("idling and unidling [local]", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLI("cli-idling", exutil.KubeConfigPath()).Verbose()
		echoServerFixture = exutil.FixturePath("testdata", "idling-echo-server.yaml")
	)

	// map of all resources created from the fixtures
	var resources map[string][]string

	g.JustBeforeEach(func() {
		g.By("Creating the resources")
		rawResources, rawResourceNames, err := createFixture(oc, echoServerFixture)
		o.Expect(err).ToNot(o.HaveOccurred())

		resources = make(map[string][]string)
		for i, resource := range rawResources {
			resources[resource] = append(resources[resource], rawResourceNames[i])
		}

		fmt.Fprintf(g.GinkgoWriter, "created resources %v", resources)

		g.By("Waiting for the endpoints to exist")
		serviceName := resources["service"][0]
		g.By("Waiting for endpoints to be up")
		err = exutil.WaitForEndpointsAvailable(oc, serviceName)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("should work with TCP (when fully idled) [Conformance]", func() {
		g.By("Idling the service")
		// NB: the default policy does not allow normal users to create or update idlers,
		// so we need admin here
		_, err := oc.AsAdmin().Run("idle").Args(resources["idler.idling.openshift.io"][0]).Output()
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
	})

	g.It("should work with TCP (while idling)", func() {
		g.By("Idling the service")
		_, err := oc.AsAdmin().Run("idle").Args(resources["idler.idling.openshift.io"][0]).Output()
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("Connecting to the service IP and repeatedly connecting, making sure we seamlessly idle and come back up")
		serviceName := resources["service"][0]
		svc, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(serviceName, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		o.Consistently(func() error { return tryEchoTCP(svc) }, 10*time.Second, 500*time.Millisecond).ShouldNot(o.HaveOccurred())

		g.By("Waiting until we have endpoints")
		err = exutil.WaitForEndpointsAvailable(oc, serviceName)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.It("should handle many TCP connections", func() {
		g.By("Idling the service")
		_, err := oc.AsAdmin().Run("idle").Args(resources["idler.idling.openshift.io"][0]).Output()
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

		g.By("Expecting all of those connections to succeed")
		errCount := 0
		for _, err := range errors {
			if err != nil {
				errCount++
			}
		}
		o.Expect(errCount).To(o.Equal(0))

		g.By("Waiting until we have endpoints")
		err = exutil.WaitForEndpointsAvailable(oc, serviceName)
	})

	g.It("should work with UDP", func() {
		g.By("Idling the service")
		// NB: the default policy does not allow normal users to create or update idlers,
		// so we need admin here
		_, err := oc.AsAdmin().Run("idle").Args(resources["idler.idling.openshift.io"][0]).Output()
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
	})
})
