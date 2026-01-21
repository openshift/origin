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

	var oc = exutil.NewCLI(namespace)
	var ctx = context.Background()

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		if isMicroShift || isHyperShift {
			g.Skip("TLS configuration for the apiserver resource is not applicable to MicroShift or HyperShift clusters - skipping")
		}
	})

	g.It("TestTLSMinimumVersions", func() {

		g.By("Getting the APIServer configuration")
		config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Determining expected TLS behavior based on the cluster's TLS profile")
		var tlsShouldWork, tlsShouldNotWork *tls.Config
		switch {
		case config.Spec.TLSSecurityProfile == nil,
			config.Spec.TLSSecurityProfile.Type == configv1.TLSProfileIntermediateType:
			tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}
			g.By("Using intermediate TLS profile: connections with TLS ≥1.2 should work, <1.2 should fail")
		case config.Spec.TLSSecurityProfile.Type == configv1.TLSProfileModernType:
			tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
			g.By("Using modern TLS profile: only TLS 1.3 connections should succeed")
		default:
			g.Skip("Only intermediate or modern profiles are tested")
		}

		targets := []struct {
			name, namespace, port string
		}{
			{"apiserver", "openshift-kube-apiserver", "443"},
			{"oauth-openshift", "openshift-authentication", "443"},
			{"kube-controller-manager", "openshift-kube-controller-manager", "443"},
			{"scheduler", "openshift-kube-scheduler", "443"},
			{"api", "openshift-apiserver", "443"},
			{"api", "openshift-oauth-apiserver", "443"},
			{"machine-config-controller", "openshift-machine-config-operator", "9001"},
		}

		g.By("Verifying TLS behavior for core control plane components")
		for _, target := range targets {
			g.By(fmt.Sprintf("Checking %s/%s on port %s", target.namespace, target.name, target.port))
			err = forwardPortAndExecute(target.name, target.namespace, target.port,
				func(port int) error { return checkTLSConnection(port, tlsShouldWork, tlsShouldNotWork) })
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Checking etcd's TLS behavior")
		err = forwardPortAndExecute("etcd", "openshift-etcd", "2379", func(port int) error {
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
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("TestTLSDefaults", func() {
		t := g.GinkgoT()

		_, err := e2e.LoadClientset(true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Getting the APIServer config")
		config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		if config.Spec.TLSSecurityProfile != nil &&
			config.Spec.TLSSecurityProfile.Type != configv1.TLSProfileIntermediateType {
			g.Skip("Cluster TLS profile is not default (intermediate), skipping cipher defaults check")
		}

		g.By("Verifying TLS version and cipher behavior via port-forward to apiserver")
		err = forwardPortAndExecute("apiserver", "openshift-kube-apiserver", "443", func(port int) error {
			host := fmt.Sprintf("localhost:%d", port)
			t.Logf("Testing TLS versions and ciphers against %s", host)

			// Test TLS versions
			for _, tlsVersionName := range crypto.ValidTLSVersions() {
				tlsVersion := crypto.TLSVersionOrDie(tlsVersionName)
				expectSuccess := tlsVersion >= crypto.DefaultTLSVersion()
				cfg := &tls.Config{MinVersion: tlsVersion, MaxVersion: tlsVersion, InsecureSkipVerify: true}

				t.Logf("Testing TLS version %s (0x%04x), expectSuccess=%v", tlsVersionName, tlsVersion, expectSuccess)
				conn, dialErr := tls.Dial("tcp", host, cfg)
				if dialErr == nil {
					t.Logf("TLS %s succeeded, negotiated version: 0x%04x", tlsVersionName, conn.ConnectionState().Version)
					closeErr := conn.Close()
					if closeErr != nil {
						return fmt.Errorf("failed to close connection: %v", closeErr)
					}
				} else {
					t.Logf("TLS %s failed with error: %v", tlsVersionName, dialErr)
				}
				if success := dialErr == nil; success != expectSuccess {
					return fmt.Errorf("expected success %v, got %v with TLS version %s", expectSuccess, success, tlsVersionName)
				}
			}

			// Test cipher suites
			defaultCiphers := map[uint16]bool{}
			for _, c := range crypto.DefaultCiphers() {
				defaultCiphers[c] = true
			}

			for _, cipherName := range crypto.ValidCipherSuites() {
				cipher, err := crypto.CipherSuite(cipherName)
				if err != nil {
					return err
				}
				expectFailure := !defaultCiphers[cipher]
				// Constrain to TLS 1.2 because the intermediate profile allows both TLS 1.2 and TLS 1.3.
				// If MaxVersion is unspecified, the client negotiates TLS 1.3 when the server supports it.
				// TLS 1.3 does not support configuring cipher suites (predetermined by the spec), so
				// specifying any cipher suite (RC4 or otherwise) has no effect with TLS 1.3.
				// By forcing TLS 1.2, we can actually test the cipher suite restrictions.
				cfg := &tls.Config{
					CipherSuites:       []uint16{cipher},
					MinVersion:         tls.VersionTLS12,
					MaxVersion:         tls.VersionTLS12,
					InsecureSkipVerify: true,
				}

				conn, dialErr := tls.Dial("tcp", host, cfg)
				if dialErr == nil {
					closeErr := conn.Close()
					if expectFailure {
						return fmt.Errorf("expected failure on cipher %s, got success. Closing conn: %v", cipherName, closeErr)
					}
				}
			}

			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func forwardPortAndExecute(serviceName, namespace, remotePort string, toExecute func(localPort int) error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			localPort := rand.Intn(65534-1025) + 1025
			args := []string{"port-forward", fmt.Sprintf("svc/%s", serviceName), fmt.Sprintf("%d:%s", localPort, remotePort), "-n", namespace}

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
			return nil
		} else {
			e2e.Logf("failed to start oc port-forward command or test: %v", err)
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
	if !strings.Contains(err.Error(), "protocol version") &&
		!strings.Contains(err.Error(), "no supported versions satisfy") &&
		!strings.Contains(err.Error(), "handshake failure") {
		return fmt.Errorf("should not work: got error, but not a TLS version mismatch: %w", err)
	}
	return nil
}
