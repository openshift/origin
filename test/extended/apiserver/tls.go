package apiserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/test/extended/scheme"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
)

type IPFamily string

const (
	namespace = "apiserver-tls-test"

	// How often to poll pods and nodes.
	poll = 5 * time.Second

	// IPFamily constants
	IPv4      IPFamily = "ipv4"
	IPv6      IPFamily = "ipv6"
	DualStack IPFamily = "dual"
	Unknown   IPFamily = "unknown"
)

// This test only checks whether components are serving the proper TLS version based
// on the expected version set in the TLS profile config. It is a part of the
// openshift/conformance/parallel test suite, and it is expected that there are jobs
// which run that entire conformance suite against clusters running any TLS profiles
// that there is a desire to test.
var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI(namespace)

	g.It("TestTLSMinimumVersions", func() {
		ctx := context.TODO()

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		if isMicroShift || isHyperShift {
			g.Skip("tls configuration for the apiserver resource is not applicable to microshift or hypershift clusters - skipping")
		}

		ipFamily := getIPFamilyForCluster(*oc, oc.Namespace())

		if ipFamily != IPv4 {
			g.Skip("tls configuration is only tested on IPv4 clusters, skipping")
		}

		insecure := true
		configFlags := &genericclioptions.ConfigFlags{}
		configFlags.Insecure = &insecure
		configFlags.APIServer = &oc.AdminConfig().Host
		configFlags.BearerToken = &oc.AdminConfig().BearerToken

		restConfig, err := configFlags.ToRESTConfig()
		o.Expect(err).NotTo(o.HaveOccurred())

		config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var tlsShouldWork, tlsShouldNotWork *tls.Config

		if config.Spec.TLSSecurityProfile == nil {
			// default to intermediate profile, which requires 1.2
			tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}
		} else {
			switch config.Spec.TLSSecurityProfile.Type {
			case configv1.TLSProfileIntermediateType:
				tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
				tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}

			case configv1.TLSProfileModernType:
				tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
				tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

			default:
				g.Skip("only intermediate or modern profiles are tested")
			}
		}

		//////

		g.By("Checking the Kube API server")

		pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-kube-apiserver").List(ctx, metav1.ListOptions{
			LabelSelector: "apiserver=true",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"6443"},
			3,
			200*time.Millisecond,
			func() {
				conn, err := tls.Dial("tcp", "localhost:6443", tlsShouldWork)
				o.Expect(err).NotTo(o.HaveOccurred())

				conn.Close()

				_, err = tls.Dial("tcp", "localhost:6443", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		//////

		g.By("Checking the OAuth server")

		pods, err = oc.AdminKubeClient().CoreV1().Pods("openshift-authentication").List(ctx, metav1.ListOptions{
			LabelSelector: "app=oauth-openshift",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"6443"},
			3,
			200*time.Millisecond,
			func() {
				conn, err := tls.Dial("tcp", "localhost:6443", tlsShouldWork)
				o.Expect(err).NotTo(o.HaveOccurred())

				conn.Close()

				_, err = tls.Dial("tcp", "localhost:6443", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		//////

		g.By("Checking the kube-controller-manager")

		pods, err = oc.AdminKubeClient().CoreV1().Pods("openshift-kube-controller-manager").List(ctx, metav1.ListOptions{
			LabelSelector: "app=kube-controller-manager",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"10357"},
			3,
			200*time.Millisecond,
			func() {
				conn, err := tls.Dial("tcp", "localhost:10357", tlsShouldWork)
				o.Expect(err).NotTo(o.HaveOccurred())

				conn.Close()

				_, err = tls.Dial("tcp", "localhost:10357", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		//////

		g.By("Checking the openshift-kube-scheduler")

		pods, err = oc.AdminKubeClient().CoreV1().Pods("openshift-kube-scheduler").List(ctx, metav1.ListOptions{
			LabelSelector: "app=openshift-kube-scheduler",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"10259"},
			3,
			200*time.Millisecond,
			func() {
				conn, err := tls.Dial("tcp", "localhost:10259", tlsShouldWork)
				o.Expect(err).NotTo(o.HaveOccurred())

				conn.Close()

				_, err = tls.Dial("tcp", "localhost:10259", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		//////

		g.By("Checking the openshift-apiserver")

		pods, err = oc.AdminKubeClient().CoreV1().Pods("openshift-apiserver").List(ctx, metav1.ListOptions{
			LabelSelector: "apiserver=true",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"8443"},
			3,
			200*time.Millisecond,
			func() {
				conn, err := tls.Dial("tcp", "localhost:8443", tlsShouldWork)
				o.Expect(err).NotTo(o.HaveOccurred())

				conn.Close()

				_, err = tls.Dial("tcp", "localhost:8443", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		//////

		g.By("Checking the openshift-oauth-apiserver")

		pods, err = oc.AdminKubeClient().CoreV1().Pods("openshift-oauth-apiserver").List(ctx, metav1.ListOptions{
			LabelSelector: "app=openshift-oauth-apiserver",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"8443"},
			3,
			200*time.Millisecond,
			func() {
				conn, err := tls.Dial("tcp", "localhost:8443", tlsShouldWork)
				o.Expect(err).NotTo(o.HaveOccurred())

				conn.Close()

				_, err = tls.Dial("tcp", "localhost:8443", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		//////

		g.By("Checking the openshift-machine-config-controller")

		pods, err = oc.AdminKubeClient().CoreV1().Pods("openshift-machine-config-operator").List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=machine-config-controller",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"9001"},
			3,
			200*time.Millisecond,
			func() {
				conn, err := tls.Dial("tcp", "localhost:9001", tlsShouldWork)
				o.Expect(err).NotTo(o.HaveOccurred())

				conn.Close()

				_, err = tls.Dial("tcp", "localhost:9001", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		//////

		g.By("Checking etcd")

		pods, err = oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(ctx, metav1.ListOptions{
			LabelSelector: "app=etcd",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = ForwardPortsAndExecute(
			*restConfig,
			&pods.Items[0],
			[]string{"2379"},
			3,
			200*time.Millisecond,
			func() {
				// We aren't actually going through mTLS authentication with etcd to communicate
				// with it - just checking TLS protocol versions. So, if it throws a "bad certificate"
				// error, we're past the version check and consider it a success for this test.

				conn, err := tls.Dial("tcp", "localhost:2379", tlsShouldWork)
				if err != nil {
					o.Expect(err.Error()).To(o.ContainSubstring("remote error: tls: bad certificate"))
				} else {
					conn.Close()
				}

				_, err = tls.Dial("tcp", "localhost:2379", tlsShouldNotWork)
				o.Expect(err).To(o.HaveOccurred())
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

	})
})

func ForwardPortsAndExecute(restConfig rest.Config, pod *v1.Pod, ports []string, maxConnectRetries int, initialBackoff time.Duration, toExecute func()) error {
	if len(ports) < 1 {
		return fmt.Errorf("at least 1 PORT is required for port-forward")
	}

	restClient, err := rest.RESTClientFor(defaultRESTConfigs(restConfig))
	if err != nil {
		return err
	}

	stdout := bytes.NewBuffer(nil)
	req := restClient.Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(&restConfig)
	if err != nil {
		return err
	}

	stopChannel := make(chan struct{})
	readyChannel := make(chan struct{})
	errorChannel := make(chan error, 1)
	defer close(stopChannel)

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	fw, err := portforward.New(dialer, ports, stopChannel, readyChannel, stdout, stdout)
	if err != nil {
		return err
	}

	go func() {
		var err error

		for attempt := 0; attempt < maxConnectRetries; attempt++ {
			err = fw.ForwardPorts()
			if err == nil {
				break
			}

			if attempt < maxConnectRetries-1 {
				backoff := initialBackoff * time.Duration(math.Pow(2, float64(attempt)))
				time.Sleep(backoff)
			}
		}

		if err != nil {
			errorChannel <- err
			close(stopChannel)
		}
	}()

	select {
	case <-readyChannel:
		toExecute()
	case err := <-errorChannel:
		return fmt.Errorf("port forwarding failed: %w", err)
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timed out waiting for port forwarding to be ready")
	}

	return nil
}

func defaultRESTConfigs(config rest.Config) *rest.Config {
	if config.GroupVersion == nil {
		config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	}
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = scheme.Codecs
	}
	if len(config.UserAgent) == 0 {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	config.APIPath = "/api"
	return &config
}

func getIPFamilyForCluster(client exutil.CLI, namespace string) IPFamily {
	podIPs, err := createPod(client, namespace, "test-ip-family-pod")
	o.Expect(err).NotTo(o.HaveOccurred())
	return getIPFamily(podIPs)
}

func getIPFamily(podIPs []v1.PodIP) IPFamily {
	switch len(podIPs) {
	case 1:
		ip := net.ParseIP(podIPs[0].IP)
		if ip.To4() != nil {
			return IPv4
		} else {
			return IPv6
		}
	case 2:
		ip1 := net.ParseIP(podIPs[0].IP)
		ip2 := net.ParseIP(podIPs[1].IP)
		if ip1 == nil || ip2 == nil {
			return Unknown
		}
		if (ip1.To4() == nil) == (ip2.To4() == nil) {
			return Unknown
		}
		return DualStack
	default:
		return Unknown
	}
}

func createPod(client exutil.CLI, ns, generateName string) ([]corev1.PodIP, error) {
	pod := frameworkpod.NewAgnhostPod(ns, "", nil, nil, nil)
	pod.ObjectMeta.GenerateName = generateName
	pod.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
	pod.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		ReadOnlyRootFilesystem: ptr.To(true),
		Privileged:             ptr.To(false),
	}

	execPod, err := client.AdminKubeClient().CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	var podIPs []corev1.PodIP
	err = wait.PollImmediate(poll, 2*time.Minute, func() (bool, error) {
		retrievedPod, err := client.AdminKubeClient().CoreV1().Pods(execPod.Namespace).Get(context.TODO(), execPod.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		podIPs = retrievedPod.Status.PodIPs
		return retrievedPod.Status.Phase == corev1.PodRunning, nil
	})
	return podIPs, err
}

var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("apiserver")

	g.It("TestTLSDefaults", func() {
		t := g.GinkgoT()

		coreClient, err := e2e.LoadClientset(true)
		o.Expect(err).NotTo(o.HaveOccurred())

		isMicroShift, err := exutil.IsMicroShiftCluster(coreClient)
		o.Expect(err).NotTo(o.HaveOccurred())

		if isMicroShift {
			g.Skip("apiserver resource for configuring tls profiles does not exist in microshift clusters - skipping")
		}

		config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if config.Spec.TLSSecurityProfile != nil && config.Spec.TLSSecurityProfile.Type != configv1.TLSProfileIntermediateType {
			g.Skip("the cluster's tls profile is in a non-default state, not testing cipher defaults")
		}

		// Verify we fail with TLS versions less than the default, and work with TLS versions >= the default
		for _, tlsVersionName := range crypto.ValidTLSVersions() {
			tlsVersion := crypto.TLSVersionOrDie(tlsVersionName)
			expectSuccess := tlsVersion >= crypto.DefaultTLSVersion()
			config := &tls.Config{MinVersion: tlsVersion, MaxVersion: tlsVersion, InsecureSkipVerify: true}

			// We're going to be dialing TCP directly, not connecting over HTTP as usual, so we don't want the protocol on the host.
			host := strings.TrimPrefix(oc.AdminConfig().Host, "https://")

			{
				conn, err := tls.Dial("tcp", host, config)
				if err == nil {
					conn.Close()
				}
				if success := err == nil; success != expectSuccess {
					t.Errorf("Expected success %v, got %v with TLS version %s dialing master", expectSuccess, success, tlsVersionName)
				}
			}
		}

		// Verify the only ciphers we work with are in the default set.
		// Not all default ciphers will succeed because they depend on the serving cert type.
		defaultCiphers := map[uint16]bool{}
		for _, defaultCipher := range crypto.DefaultCiphers() {
			defaultCiphers[defaultCipher] = true
		}
		for _, cipherName := range crypto.ValidCipherSuites() {
			cipher, err := crypto.CipherSuite(cipherName)
			if err != nil {
				t.Fatal(err)
			}
			expectFailure := !defaultCiphers[cipher]
			config := &tls.Config{CipherSuites: []uint16{cipher}, InsecureSkipVerify: true}

			{
				conn, err := tls.Dial("tcp", oc.AdminConfig().Host, config)
				if err == nil {
					conn.Close()
					if expectFailure {
						t.Errorf("Expected failure on cipher %s, got success dialing master", cipherName)
					}
				}
			}
		}

	})
})
