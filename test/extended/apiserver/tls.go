package apiserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/test/extended/networking"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	namespace = "apiserver-tls-test"
)

// This test only checks whether components are serving the proper TLS version based
// on the expected version set in the TLS profile config. It is a part of the
// openshift/conformance/parallel test suite, and it is expected that there are jobs
// which run that entire conformance suite against clusters running any TLS profiles
// that there is a desire to test.
var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	f := e2e.NewDefaultFramework("tls")

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

		ipFamily := networking.GetIPFamilyForCluster(f)

		if ipFamily != networking.IPv4 {
			g.Skip("tls configuration is only tested on IPv4 clusters, skipping")
		}

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

		g.By("Checking the Kube API server")

		err = forwardPortAndExecute(
			"apiserver",
			"openshift-kube-apiserver",
			"443",
			func(port int) error {
				return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking the OAuth server")

		err = forwardPortAndExecute(
			"oauth-openshift",
			"openshift-authentication",
			"443",
			func(port int) error {
				return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking the kube-controller-manager")

		err = forwardPortAndExecute(
			"kube-controller-manager",
			"openshift-kube-controller-manager",
			"443",
			func(port int) error {
				return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking the openshift-kube-scheduler")

		err = forwardPortAndExecute(
			"scheduler",
			"openshift-kube-scheduler",
			"443",
			func(port int) error {
				return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking the openshift-apiserver")

		err = forwardPortAndExecute(
			"api",
			"openshift-apiserver",
			"443",
			func(port int) error {
				return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking the openshift-oauth-apiserver")

		err = forwardPortAndExecute(
			"api",
			"openshift-oauth-apiserver",
			"443",
			func(port int) error {
				return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking the openshift-machine-config-controller")

		err = forwardPortAndExecute(
			"machine-config-controller",
			"openshift-machine-config-operator",
			"9001",
			func(port int) error {
				return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork)
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Checking etcd")

		err = forwardPortAndExecute(
			"etcd",
			"openshift-etcd",
			"2379",
			func(port int) error {
				// We aren't actually going through mTLS authentication with etcd to communicate
				// with it - just checking TLS protocol versions. So, if it throws a "bad certificate"
				// error, we're past the version check and consider it a success for this test.

				conn, err := tls.Dial("tcp", fmt.Sprintf("localhost:%d", port), tlsShouldWork)
				if err != nil {
					if !strings.Contains(err.Error(), "remote error: tls: bad certificate") {
						return fmt.Errorf("should work: %w", err)
					}
				} else {
					err = conn.Close()
					if err != nil {
						return fmt.Errorf("failed to close connection: %w", err)
					}
				}
				conn, err = tls.Dial("tcp", fmt.Sprintf("localhost:%d", port), tlsShouldNotWork)
				if err == nil {
					return fmt.Errorf("should not work: connection unexpectedly succeeded, closing conn status: %v", conn.Close())
				}
				return nil
			},
		)
		o.Expect(err).NotTo(o.HaveOccurred())

	})
})

func forwardPortAndExecute(serviceName string, namespace string, remotePort string, toExecute func(localPort int) error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			localPort := rand.Intn(65534-1025) + 1025
			args := []string{
				"port-forward",
				fmt.Sprintf("svc/%s", serviceName),
				fmt.Sprintf("%d:%s", localPort, remotePort),
				"-n", namespace,
			}

			cmd := exec.CommandContext(ctx, "oc", args...)

			stdout, stderr, err := e2e.StartCmdAndStreamOutput(cmd)
			if err != nil {
				return err
			}
			defer stdout.Close()
			defer stderr.Close()
			defer e2e.TryKill(cmd)

			e2e.Logf("oc port-forward output: %s", readPartialFrom(stdout, 1024))

			return toExecute(localPort)
		}(); err == nil {
			// Success, stop retrying
			return nil
		} else {
			err = fmt.Errorf("failed to start oc port-forward command or test: %w", err)
			e2e.Logf("%v", err)
			time.Sleep(2 * time.Second)
		}
	}

	return err
}

func readPartialFrom(r io.Reader, maxBytes int) string {
	buf := make([]byte, maxBytes)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Sprintf("error reading: %v", err)
	}
	return string(buf[:n])
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

		f := e2e.NewDefaultFramework("tls")

		ipFamily := networking.GetIPFamilyForCluster(f)

		if ipFamily != networking.IPv4 {
			g.Skip("tls configuration is only tested on IPv4 clusters, skipping")
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
					err := conn.Close()
					if err != nil {
						t.Errorf("Failed to close connection: %v", err)
					}
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
					if expectFailure {
						t.Errorf("Expected failure on cipher %s, got success dialing master. closing conn status: %v", cipherName, conn.Close())
					}
				}
			}
		}

	})
})

// checkTLSConnection tries to connect to localhost:port with the provided TLS configs.
func checkTLSConnection(port int, tlsShouldWork, tlsShouldNotWork *tls.Config) error {
	conn, err := tls.Dial("tcp", fmt.Sprintf("localhost:%d", port), tlsShouldWork)
	if err != nil {
		return fmt.Errorf("should work: %w", err)
	}
	err = conn.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	conn, err = tls.Dial("tcp", fmt.Sprintf("localhost:%d", port), tlsShouldNotWork)
	if err == nil {
		return fmt.Errorf("should not work: connection unexpectedly succeeded, closing conn status: %v", conn.Close())
	}
	// Acceptable TLS version mismatch errors
	if !strings.Contains(err.Error(), "protocol version") &&
		!strings.Contains(err.Error(), "no supported versions satisfy") &&
		!strings.Contains(err.Error(), "handshake failure") {
		return fmt.Errorf("should not work: got error, but not a TLS version mismatch: %w", err)
	}
	return nil
}
