package apiserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	namespace = "apiserver-tls-test"

	// Logging frequency constants for polling loops to reduce noise
	logEveryNAttemptsKeyRotation   = 12 // Every 12 attempts * 5s = once per minute
	logEveryNAttemptsSecretRecreate = 20 // Every 20 attempts * 3s = once per minute
	logEveryNAttemptsKeyAppear      = 10 // Every 10 attempts * 6s = once per minute

	// Encryption key secret name prefixes
	encryptionKeyOASPrefix = "encryption-key-openshift-apiserver-"
	encryptionKeyKASPrefix = "encryption-key-openshift-kube-apiserver-"
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

			// Read and discard port-forward output to avoid logging sensitive cluster metadata
			_ = readPartialFrom(stdout, 1024)
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

var _ = g.Describe("[sig-api-machinery] [Jira:apiserver-auth] Operators / Certs", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("apiserver-certs")

	// Verify TLS secrets in openshift-kube-apiserver and CA bundles in
	// openshift-config-managed are all valid (non-expired, not-yet-valid) PEM certificates.
	g.It("[OTP] should have valid TLS certificates for authentication and encryption between API server components [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			g.By("1) check TLS secrets in openshift-kube-apiserver")
			ns := "openshift-kube-apiserver"
			e2e.Logf("==================================== OpenShift TLS Secrets Verification ====================================")

			secretsJSON, err := oc.AsAdmin().WithoutNamespace().
				Run("get").Args("secret", "-n", ns, "-ojson").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			var secretList struct {
				Items []struct {
					Type     string            `json:"type"`
					Data     map[string]string `json:"data"`
					Metadata struct {
						Name string `json:"name"`
					} `json:"metadata"`
				} `json:"items"`
			}
			o.Expect(json.Unmarshal([]byte(secretsJSON), &secretList)).NotTo(o.HaveOccurred(),
				"failed to parse oc secret JSON output")

			for _, s := range secretList.Items {
				if s.Type != "kubernetes.io/tls" {
					continue
				}
				name := s.Metadata.Name
				rawcrt, ok := s.Data["tls.crt"]
				if !ok || rawcrt == "" {
					e2e.Logf("  %s/%s — no tls.crt found, skipping", ns, name)
					continue
				}
				decoded, decErr := base64.StdEncoding.DecodeString(rawcrt)
				if decErr != nil {
					decoded, decErr = base64.RawStdEncoding.DecodeString(rawcrt)
					o.Expect(decErr).NotTo(o.HaveOccurred(),
						"failed to base64-decode tls.crt in secret %s/%s", ns, name)
				}
				e2e.Logf("  checking secret %s/%s", ns, name)
				parseAndCheckPEMs(decoded, fmt.Sprintf("%s/%s", ns, name))
			}

			g.By("2) check CA bundle configmaps in openshift-config-managed")
			e2e.Logf("==================================== OpenShift CA Bundles Verification ====================================")
			ns = "openshift-config-managed"
			cms := []struct{ name, key string }{
				{"kube-apiserver-server-ca", "ca-bundle.crt"},
				{"kube-apiserver-client-ca", "ca-bundle.crt"},
				{"kube-root-ca.crt", "ca.crt"},
				{"trusted-ca-bundle", "ca-bundle.crt"},
				{"service-ca", "ca-bundle.crt"},
			}
			for _, cm := range cms {
				cmJSON, cmErr := oc.AsAdmin().WithoutNamespace().
					Run("get").Args("cm", cm.name, "-n", ns, "-ojson").Output()
				o.Expect(cmErr).NotTo(o.HaveOccurred(),
					"failed to get configmap %s/%s", ns, cm.name)

				var cmObj struct {
					Data map[string]string `json:"data"`
				}
				err := json.Unmarshal([]byte(cmJSON), &cmObj)
				o.Expect(err).NotTo(o.HaveOccurred(),
					"failed to parse configmap JSON for %s/%s", ns, cm.name)

				val, ok := cmObj.Data[cm.key]
				if !ok || val == "" {
					// fallback: search any key containing "ca" or "crt"
					found := false
					for k, v := range cmObj.Data {
						if kl := strings.ToLower(k); (strings.Contains(kl, "ca") || strings.Contains(kl, "crt")) && v != "" {
							e2e.Logf("  checking configmap %s/%s (key=%s)", ns, cm.name, k)
							parseAndCheckPEMs([]byte(v), fmt.Sprintf("%s/%s(%s)", ns, cm.name, k))
							found = true
							break
						}
					}
					o.Expect(found).To(o.BeTrue(),
						"no CA/crt key found in configmap %s/%s", ns, cm.name)
					continue
				}
				e2e.Logf("  checking configmap %s/%s (key=%s)", ns, cm.name, cm.key)
				parseAndCheckPEMs([]byte(val), fmt.Sprintf("%s/%s(%s)", ns, cm.name, cm.key))
			}
			e2e.Logf("Certificate verification complete.")
		})

	// Add a custom TLS certificate to the cluster API server, verify it is served,
	// then restore the original configuration.
	g.It("[OTP] should support adding a custom TLS certificate for the cluster API [Disruptive][Slow][apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isHyperShift {
				g.Skip("custom serving certificates for the API server are managed by HyperShift — skipping")
			}

			tmpdir := g.GinkgoT().TempDir()
			// Use unique secret name to avoid conflicts with pre-existing secrets
			testSecretName := fmt.Sprintf("custom-api-cert-test-%d", time.Now().Unix())

			var (
				originKubeconfig    = os.Getenv("KUBECONFIG")
				originKubeconfigBkp = filepath.Join(tmpdir, "kubeconfig.origin.bkp")
				originCA            = filepath.Join(tmpdir, "certificate-authority-data-origin.crt")
				newCA               = filepath.Join(tmpdir, "certificate-authority-data-origin-new.crt")
				cnBase              = "kas-test-cert"
				caKeypem            = filepath.Join(tmpdir, "caKey.pem")
				caCertpem           = filepath.Join(tmpdir, "caCert.pem")
				serverKeypem        = filepath.Join(tmpdir, "serverKey.pem")
				serverconf          = filepath.Join(tmpdir, "server.conf")
				serverWithSANcsr    = filepath.Join(tmpdir, "serverWithSAN.csr")
				serverCertWithSAN   = filepath.Join(tmpdir, "serverCertWithSAN.pem")
			)

			// Snapshot original apiserver configuration before modification
			g.By("0. snapshot original apiserver configuration")
			origAPIServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
				ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get original apiserver config")

			origNamedCerts := origAPIServer.Spec.ServingCerts.NamedCertificates
			var patchToRecover string
			if origNamedCerts == nil || len(origNamedCerts) == 0 {
				patchToRecover = `{"spec":{"servingCerts": {"namedCertificates": null}}}`
			} else {
				// Marshal original namedCertificates to restore exact state
				certsJSON, err := json.Marshal(origNamedCerts)
				o.Expect(err).NotTo(o.HaveOccurred(), "failed to marshal original namedCertificates")
				patchToRecover = fmt.Sprintf(`{"spec":{"servingCerts": {"namedCertificates": %s}}}`, string(certsJSON))
			}

			defer func() {
				g.By("restoring cluster to original state")
				_, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
					"apiserver/cluster", "--type=merge", "-p", patchToRecover).Output()
				if _, cpErr := exec.Command("bash", "-c",
					fmt.Sprintf("cp %s %s", originKubeconfigBkp, originKubeconfig)).Output(); cpErr != nil {
					e2e.Logf("warning: failed to restore kubeconfig: %v", cpErr)
				}
				err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("wait-for-stable-cluster").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				// Only delete the test-specific secret, not any pre-existing secrets
				err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(
					"secret", testSecretName, "-n", "openshift-config", "--ignore-not-found").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}()

			fqdnName, port := getAPIServerFQDNAndPort(ctx, oc)

			g.By("1. take a backup of the original kubeconfig")
			origData, err := os.ReadFile(originKubeconfig)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(os.WriteFile(originKubeconfigBkp, origData, 0600)).NotTo(o.HaveOccurred())

			g.By("2. extract the original CA certificate from kubeconfig")
			kubeconfigData, err := os.ReadFile(originKubeconfig)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to read kubeconfig")

			// Parse kubeconfig to extract certificate-authority-data
			var caDataBase64 string
			for _, line := range strings.Split(string(kubeconfigData), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "certificate-authority-data:") {
					caDataBase64 = strings.TrimSpace(strings.TrimPrefix(line, "certificate-authority-data:"))
					break
				}
			}
			o.Expect(caDataBase64).NotTo(o.BeEmpty(), "certificate-authority-data not found in kubeconfig")

			// Decode base64 and write to file
			caDataDecoded, err := base64.StdEncoding.DecodeString(caDataBase64)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to decode certificate-authority-data")
			err = os.WriteFile(originCA, caDataDecoded, 0600)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to write CA file")

			g.By("3. generate a new CA key, CA cert, server key, and server cert with SAN")
			// Generate CA private key using ECDSA P-256 instead of RSA-2048
			out, err := exec.CommandContext(ctx, "openssl", "ecparam", "-genkey", "-name", "prime256v1", "-out", caKeypem).CombinedOutput()
			o.Expect(err).NotTo(o.HaveOccurred(), "openssl ecparam CA failed: %s", out)

			// Generate CA certificate
			out, err = exec.CommandContext(ctx, "openssl", "req", "-x509", "-new", "-nodes",
				"-key", caKeypem, "-days", "100000", "-out", caCertpem,
				"-subj", fmt.Sprintf("/CN=%s_ca", cnBase)).CombinedOutput()
			o.Expect(err).NotTo(o.HaveOccurred(), "openssl req CA failed: %s", out)

			// Generate server private key using ECDSA P-256
			out, err = exec.CommandContext(ctx, "openssl", "ecparam", "-genkey", "-name", "prime256v1", "-out", serverKeypem).CombinedOutput()
			o.Expect(err).NotTo(o.HaveOccurred(), "openssl ecparam server failed: %s", out)
			serverconfContent := fmt.Sprintf(`[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = %s`, fqdnName)
			o.Expect(os.WriteFile(serverconf, []byte(serverconfContent), 0644)).NotTo(o.HaveOccurred())

			// Generate server CSR with SAN
			out, err = exec.CommandContext(ctx, "openssl", "req", "-new",
				"-key", serverKeypem, "-out", serverWithSANcsr,
				"-subj", fmt.Sprintf("/CN=%s_server", cnBase),
				"-config", serverconf).CombinedOutput()
			o.Expect(err).NotTo(o.HaveOccurred(), "openssl req server CSR failed: %s", out)

			// Sign server certificate with CA
			out, err = exec.CommandContext(ctx, "openssl", "x509", "-req",
				"-in", serverWithSANcsr, "-CA", caCertpem, "-CAkey", caKeypem,
				"-CAcreateserial", "-out", serverCertWithSAN,
				"-days", "100000", "-extensions", "v3_req", "-extfile", serverconf).CombinedOutput()
			o.Expect(err).NotTo(o.HaveOccurred(), "openssl x509 sign failed: %s", out)

			g.By("4. create a TLS secret for the custom API certificate")
			err = oc.AsAdmin().WithoutNamespace().Run("create").Args(
				"secret", "tls", testSecretName,
				"--cert="+serverCertWithSAN, "--key="+serverKeypem,
				"-n", "openshift-config").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("5. patch apiserver/cluster to use the new named certificate")
			patchCmd := fmt.Sprintf(
				`{"spec":{"servingCerts": {"namedCertificates": [{"names": ["%s"], "servingCertificate": {"name": "%s"}}]}}}`,
				fqdnName, testSecretName)
			err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver/cluster", "--type=merge", "-p", patchCmd).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("6. update kubeconfig to include the new CA alongside the original CA")
			caCertData, err := os.ReadFile(caCertpem)
			o.Expect(err).NotTo(o.HaveOccurred())
			originCAData, err := os.ReadFile(originCA)
			o.Expect(err).NotTo(o.HaveOccurred())
			concatenated := append(caCertData, originCAData...)
			o.Expect(os.WriteFile(newCA, concatenated, 0644)).NotTo(o.HaveOccurred())
			b64Cert := base64.StdEncoding.EncodeToString(concatenated)
			updateKubeconfCmd := fmt.Sprintf(
				`sed -i "s/certificate-authority-data: .*/certificate-authority-data: %s/" %s`,
				b64Cert, originKubeconfig)
			_, err = exec.Command("bash", "-c", updateKubeconfCmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("7. wait for kube-apiserver operator to start progressing (≤300s)")
			err = waitCoBecomes(ctx, oc, "kube-apiserver", 300, map[string]string{"Progressing": "True"})
			o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator did not start progressing within 300s")

			e2e.Logf("waiting for kube-apiserver operator to become stable (≤1500s)")
			err = waitCoBecomes(ctx, oc, "kube-apiserver", 1500, map[string]string{
				"Available":   "True",
				"Progressing": "False",
				"Degraded":    "False",
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator did not stabilise within 1500s")

			g.By("8. validate that the custom certificate is now served by the API server")
			certDetails, err := getServerCertInfo(fqdnName, port, caCertpem)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(certDetails.Subject).To(o.ContainSubstring("CN=kas-test-cert_server"))
			o.Expect(certDetails.Issuer).To(o.ContainSubstring("CN=kas-test-cert_ca"))

			g.By("9. validate the original CA no longer verifies the new certificate")
			_, err = getServerCertInfo(fqdnName, port, originCA)
			o.Expect(err).To(o.HaveOccurred(), "original CA should not verify the new custom certificate")
		})

	// Force encryption key rotation for the etcd datastore by patching both
	// openshiftapiserver and kubeapiserver with an unsupportedConfigOverride, then verify
	// that new encryption keys are generated and the resources are re-encrypted.
	g.It("[OTP] should force etcd encryption key rotation and verify resources are re-encrypted [Disruptive][Slow][apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			g.By("1. ensure etcd encryption is enabled")
			encryptionType, cleanup, err := ensureEncryptionEnabled(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to ensure encryption is enabled")
			defer cleanup()

			g.By("2. record current encryption prefixes")
			oasEncValPrefix1, err := getEncryptionPrefix(ctx, oc, "/openshift.io/routes")
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get encryption prefix for routes")
			e2e.Logf("openshift-apiserver encryption prefix recorded before rotation")

			kasEncValPrefix1, err := getEncryptionPrefix(ctx, oc, "/kubernetes.io/secrets")
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get encryption prefix for secrets")
			e2e.Logf("kube-apiserver encryption prefix recorded before rotation")

			// Record current highest key numbers before rotation
			oasEncNumberBefore, err := getEncryptionKeyNumber(oc, `encryption-key-openshift-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get openshift-apiserver encryption key number")
			kasEncNumberBefore, err := getEncryptionKeyNumber(oc, `encryption-key-openshift-kube-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get kube-apiserver encryption key number")
			e2e.Logf("encryption keys before rotation: openshift-apiserver=%d, kube-apiserver=%d",
				oasEncNumberBefore, kasEncNumberBefore)

			t := time.Now().Format(time.RFC3339)
			patchYamlToRestore := `[{"op":"replace","path":"/spec/unsupportedConfigOverrides","value":null}]`
			patchYaml := `
spec:
  unsupportedConfigOverrides:
    encryption:
      reason: force OAS rotation ` + t

			for i, kind := range []string{"openshiftapiserver", "kubeapiserver"} {
				defer func(k string) {
					e2e.Logf("restoring %s/cluster unsupportedConfigOverrides", k)
					_ = oc.WithoutNamespace().Run("patch").Args(
						k, "cluster", "--type=json", "-p", patchYamlToRestore).Execute()
				}(kind)
				g.By(fmt.Sprintf("3.%d) force %s encryption key rotation", i+1, kind))
				err := oc.WithoutNamespace().Run("patch").Args(
					kind, "cluster", "--type=merge", "-p", patchYaml).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("4. wait for new encryption key secrets to appear (up to 15 minutes)")
			// Use an explicit timeout context derived from the spec context so Ginkgo cancellation still works
			// Increased to 15 minutes because kube-apiserver operator can be throttled and take 12+ minutes
			pollCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
			defer cancel()

			// Pre-compile regexes outside the polling loop for efficiency
			oasPattern := regexp.MustCompile(`encryption-key-openshift-apiserver-[^ ]*`)
			kasPattern := regexp.MustCompile(`encryption-key-openshift-kube-apiserver-[^ ]*`)

			var newOASEncNumber, newKASEncNumber int
			retryCount := 0
			errKey := wait.PollUntilContextCancel(pollCtx, 5*time.Second, false,
				func(pollCtx context.Context) (bool, error) {
					// Dynamically check for new keys instead of pre-calculating expected numbers
					currentOAS, err := getEncryptionKeyNumberWithRegex(oc, oasPattern)
					if err != nil {
						return false, nil
					}
					currentKAS, err := getEncryptionKeyNumberWithRegex(oc, kasPattern)
					if err != nil {
						return false, nil
					}

					// Check if both operators created new keys (number increased)
					if currentOAS > oasEncNumberBefore && currentKAS > kasEncNumberBefore {
						newOASEncNumber = currentOAS
						newKASEncNumber = currentKAS
						e2e.Logf("new encryption keys detected after %d attempts: openshift-apiserver=%d, kube-apiserver=%d",
							retryCount, newOASEncNumber, newKASEncNumber)
						return true, nil
					}

					retryCount++
					// Only log every Nth attempt to reduce noise
					if retryCount%logEveryNAttemptsKeyRotation == 1 {
						e2e.Logf("waiting for new encryption keys (attempt %d): openshift-apiserver=%d (want >%d), kube-apiserver=%d (want >%d)",
							retryCount, currentOAS, oasEncNumberBefore, currentKAS, kasEncNumberBefore)
					}
					return false, nil
				})

			o.Expect(errKey).NotTo(o.HaveOccurred(),
				"new encryption keys not created after 15 minutes (openshift-apiserver: want >%d, kube-apiserver: want >%d)",
				oasEncNumberBefore, kasEncNumberBefore)

			g.By("5. wait for kube-apiserver encryption migration to complete")
			newKASEncSecretName := buildEncryptionKeySecretName(encryptionKeyKASPrefix, newKASEncNumber)
			completed, err := waitEncryptionKeyMigration(ctx, oc, newKASEncSecretName)
			o.Expect(err).NotTo(o.HaveOccurred(),
				"encryption key migration did not complete for %s", newKASEncSecretName)
			o.Expect(completed).To(o.BeTrue())

			g.By("6. verify encryption prefixes changed after rotation")
			oasEncValPrefix2, err := getEncryptionPrefix(ctx, oc, "/openshift.io/routes")
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get encryption prefix for routes after rotation")
			e2e.Logf("openshift-apiserver encryption prefix verified after rotation")

			kasEncValPrefix2, err := getEncryptionPrefix(ctx, oc, "/kubernetes.io/secrets")
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get encryption prefix for secrets after rotation")
			e2e.Logf("kube-apiserver encryption prefix verified after rotation")

			o.Expect(oasEncValPrefix2).To(o.ContainSubstring(fmt.Sprintf("k8s:enc:%s:v1", encryptionType)))
			o.Expect(kasEncValPrefix2).To(o.ContainSubstring(fmt.Sprintf("k8s:enc:%s:v1", encryptionType)))
			o.Expect(oasEncValPrefix2).NotTo(o.Equal(oasEncValPrefix1),
				"encryption prefix for routes did not change after key rotation")
			o.Expect(kasEncValPrefix2).NotTo(o.Equal(kasEncValPrefix1),
				"encryption prefix for secrets did not change after key rotation")
		})

	// Delete etcd encryption config and key secrets, then verify the cluster
	// self-heals by recreating them and completing re-encryption.
	g.It("[OTP] should self-recover when etcd encryption configuration secrets are deleted [Disruptive][Slow][apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			g.By("1. ensure etcd encryption is enabled")
			_, cleanup, err := ensureEncryptionEnabled(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to ensure encryption is enabled")
			defer cleanup()

			uidsOld, err := oc.WithoutNamespace().Run("get").Args(
				"secret",
				"encryption-config-openshift-apiserver",
				"encryption-config-openshift-kube-apiserver",
				"-n", "openshift-config-managed",
				"-o=jsonpath={.items[*].metadata.uid}",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("2. delete encryption-config-* secrets from openshift-config-managed")
			for _, item := range []string{
				"encryption-config-openshift-apiserver",
				"encryption-config-openshift-kube-apiserver",
			} {
				e2e.Logf("removing finalizers from secret %s", item)
				err := oc.WithoutNamespace().Run("patch").Args(
					"secret", item, "-n", "openshift-config-managed",
					`-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				e2e.Logf("deleting secret %s", item)
				err = oc.WithoutNamespace().Run("delete").Args(
					"secret", item, "-n", "openshift-config-managed").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			uidsOldSlice := strings.Fields(uidsOld)
			e2e.Logf("original secret count: %d", len(uidsOldSlice))

			// Use an explicit timeout context derived from the spec context so Ginkgo cancellation still works
			pollCtx1, cancel1 := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel1()

			retryCount1 := 0
			errSecret := wait.PollUntilContextCancel(pollCtx1, 3*time.Second, false,
				func(pollCtx context.Context) (bool, error) {
					uidsNew, err := oc.WithoutNamespace().Run("get").Args(
						"secret",
						"encryption-config-openshift-apiserver",
						"encryption-config-openshift-kube-apiserver",
						"-n", "openshift-config-managed",
						"-o=jsonpath={.items[*].metadata.uid}",
					).Output()
					if err != nil {
						retryCount1++
						// Only log every Nth attempt to reduce noise
						if retryCount1%logEveryNAttemptsSecretRecreate == 1 {
							e2e.Logf("waiting for encryption-config-* secrets to be recreated (attempt %d)", retryCount1)
						}
						return false, nil
					}
					uidsNewSlice := strings.Fields(uidsNew)
					if len(uidsNewSlice) >= 2 && len(uidsOldSlice) >= 2 &&
						uidsNewSlice[0] != uidsOldSlice[0] && uidsNewSlice[1] != uidsOldSlice[1] {
						e2e.Logf("encryption-config-* secrets recreated after %d attempts (UIDs changed)", retryCount1)
						return true, nil
					}
					return false, nil
				})
			o.Expect(errSecret).NotTo(o.HaveOccurred(),
				"encryption-config-* secrets were not recreated within 5 minutes")

			oasEncNumber, err := getEncryptionKeyNumber(oc, `encryption-key-openshift-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get openshift-apiserver encryption key number")
			kasEncNumber, err := getEncryptionKeyNumber(oc, `encryption-key-openshift-kube-apiserver-[^ ]*`)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get kube-apiserver encryption key number")

			oldOASEncSecretName := buildEncryptionKeySecretName(encryptionKeyOASPrefix, oasEncNumber)
			oldKASEncSecretName := buildEncryptionKeySecretName(encryptionKeyKASPrefix, kasEncNumber)

			g.By("3. delete current encryption-key-* secrets from openshift-config-managed")
			for _, item := range []string{oldOASEncSecretName, oldKASEncSecretName} {
				e2e.Logf("removing finalizers from secret %s", item)
				err := oc.WithoutNamespace().Run("patch").Args(
					"secret", item, "-n", "openshift-config-managed",
					`-p={"metadata":{"finalizers":null}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				e2e.Logf("deleting secret %s", item)
				err = oc.WithoutNamespace().Run("delete").Args(
					"secret", item, "-n", "openshift-config-managed").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			newOASEncSecretName := buildEncryptionKeySecretName(encryptionKeyOASPrefix, oasEncNumber+1)
			newKASEncSecretName := buildEncryptionKeySecretName(encryptionKeyKASPrefix, kasEncNumber+1)

			g.By("4. wait for new encryption-key-* secrets to appear (up to 10 minutes)")
			// Use an explicit timeout context derived from the spec context so Ginkgo cancellation still works
			pollCtx2, cancel2 := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel2()

			retryCount2 := 0
			errKey := wait.PollUntilContextCancel(pollCtx2, 6*time.Second, false,
				func(pollCtx context.Context) (bool, error) {
					_, err := oc.WithoutNamespace().Run("get").Args(
						"secrets", newOASEncSecretName, newKASEncSecretName,
						"-n", "openshift-config-managed").Output()
					if err != nil {
						retryCount2++
						// Only log every Nth attempt to reduce noise
						if retryCount2%logEveryNAttemptsKeyAppear == 1 {
							e2e.Logf("waiting for new encryption-key-* secrets (attempt %d)", retryCount2)
						}
						return false, nil
					}
					e2e.Logf("new encryption-key-* secrets found after %d attempts", retryCount2)
					return true, nil
				})
			o.Expect(errKey).NotTo(o.HaveOccurred(),
				"new encryption key secrets %s, %s not found after 10 minutes", newOASEncSecretName, newKASEncSecretName)

			g.By("5. wait for encryption migration to complete for both components")
			completedOAS, errOAS := waitEncryptionKeyMigration(ctx, oc, newOASEncSecretName)
			o.Expect(errOAS).NotTo(o.HaveOccurred(),
				"encryption key migration did not complete for %s", newOASEncSecretName)
			o.Expect(completedOAS).To(o.BeTrue())

			completedKAS, errKAS := waitEncryptionKeyMigration(ctx, oc, newKASEncSecretName)
			o.Expect(errKAS).NotTo(o.HaveOccurred(),
				"encryption key migration did not complete for %s", newKASEncSecretName)
			o.Expect(completedKAS).To(o.BeTrue())
		})
})

// ---- helper functions for Operators / Certs tests ----

// parseAndCheckPEMs decodes all PEM-encoded certificates in data and asserts each one is
// currently valid (NotBefore in the past, NotAfter in the future).
func parseAndCheckPEMs(data []byte, source string) {
	rest := data
	count := 0
	for {
		block, r := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = r
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			e2e.Logf("  [%s] failed to parse certificate block: %v", source, err)
			continue
		}
		count++
		now := time.Now()
		e2e.Logf("  [%s] cert #%d NotBefore=%s NotAfter=%s",
			source, count,
			cert.NotBefore.Format(time.RFC3339), cert.NotAfter.Format(time.RFC3339))
		o.Expect(cert.NotAfter.After(now)).To(o.BeTrue(),
			"certificate %q from %s has expired (NotAfter=%s)", cert.Subject.CommonName, source, cert.NotAfter)
		o.Expect(cert.NotBefore.Before(now)).To(o.BeTrue(),
			"certificate %q from %s is not yet valid (NotBefore=%s)", cert.Subject.CommonName, source, cert.NotBefore)
	}
	e2e.Logf("  [%s] checked %d certificates", source, count)
}

// certInfo holds the Subject and Issuer DN strings from a TLS leaf certificate.
type certInfo struct {
	Subject string
	Issuer  string
}

// getServerCertInfo opens a TLS connection to fqdn:port using caPath as the trusted root CA
// and returns the leaf certificate's Subject and Issuer DNs.
func getServerCertInfo(fqdn, port, caPath string) (*certInfo, error) {
	caData, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file %s: %w", caPath, err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caData)

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%s", fqdn, port),
		&tls.Config{RootCAs: pool, ServerName: fqdn})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return nil, fmt.Errorf("no peer certificates returned by %s:%s", fqdn, port)
	}
	return &certInfo{
		Subject: certs[0].Subject.String(),
		Issuer:  certs[0].Issuer.String(),
	}, nil
}

// getAPIServerFQDNAndPort returns the external API server hostname and port from the
// cluster's infrastructure config.
func getAPIServerFQDNAndPort(ctx context.Context, oc *exutil.CLI) (string, string) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(
		ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	rawURL := infra.Status.APIServerURL
	u, err := url.Parse(rawURL)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to parse API server URL %q", rawURL)

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}
	return host, port
}

// waitCoBecomes polls the named ClusterOperator until all entries in conditions match
// (e.g. {"Available": "True", "Progressing": "False"}) or the timeout elapses.
func waitCoBecomes(ctx context.Context, oc *exutil.CLI, coName string, timeoutSec int, conditions map[string]string) error {
	lastLogTime := time.Time{}
	logInterval := 30 * time.Second
	return wait.PollUntilContextTimeout(
		ctx, 5*time.Second, time.Duration(timeoutSec)*time.Second, false,
		func(pollCtx context.Context) (bool, error) {
			co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(
				pollCtx, coName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("failed to get ClusterOperator %s: %v — retrying", coName, err)
				return false, nil
			}

			// Build a map of observed conditions
			observed := make(map[string]string)
			for _, cond := range co.Status.Conditions {
				observed[string(cond.Type)] = string(cond.Status)
			}

			// Verify all requested conditions are present and match
			for condType, want := range conditions {
				got, found := observed[condType]
				if !found || got != want {
					// Only log every 30 seconds to reduce verbosity
					now := time.Now()
					if now.Sub(lastLogTime) >= logInterval {
						if !found {
							e2e.Logf("ClusterOperator %s: condition %s not found (want %s)", coName, condType, want)
						} else {
							e2e.Logf("ClusterOperator %s: condition %s=%s (want %s)", coName, condType, got, want)
						}
						lastLogTime = now
					}
					return false, nil
				}
			}
			return true, nil
		})
}

// getEncryptionPrefix returns the first 30 bytes (as a string) of the etcd value stored at
// etcdPath. For an encrypted cluster this prefix will contain the encryption scheme identifier,
// e.g. "k8s:enc:aescbc:v1:1:".
func getEncryptionPrefix(ctx context.Context, oc *exutil.CLI, etcdPath string) (string, error) {
	pods, err := oc.AsAdmin().KubeClient().CoreV1().Pods("openshift-etcd").List(
		ctx, metav1.ListOptions{LabelSelector: "app=etcd"})
	if err != nil {
		return "", fmt.Errorf("failed to list etcd pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no etcd pods found in openshift-etcd")
	}
	podName := pods.Items[0].Name

	out, err := oc.AsAdmin().Run("exec").Args(
		"-n", "openshift-etcd", podName, "-c", "etcdctl", "--",
		"etcdctl", "get", etcdPath, "--prefix", "--limit=1", "--print-value-only",
	).Output()
	if err != nil {
		return "", fmt.Errorf("etcdctl get %s failed: %w", etcdPath, err)
	}
	// Trim to the ASCII encryption prefix (at most 30 chars) so the output is safe to compare.
	if len(out) > 30 {
		out = out[:30]
	}
	return out, nil
}

// getEncryptionKeyNumber lists secrets in openshift-config-managed whose names match
// pattern and returns the highest numeric suffix found.  For example, if secrets
// "encryption-key-openshift-apiserver-3" and "-4" exist, it returns 4.
func getEncryptionKeyNumber(oc *exutil.CLI, pattern string) (int, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, fmt.Errorf("invalid pattern %q: %w", pattern, err)
	}
	return getEncryptionKeyNumberWithRegex(oc, re)
}

// getEncryptionKeyNumberWithRegex is like getEncryptionKeyNumber but accepts a pre-compiled
// regex for efficiency when called repeatedly in polling loops.
func getEncryptionKeyNumberWithRegex(oc *exutil.CLI, re *regexp.Regexp) (int, error) {
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"secrets", "-n", "openshift-config-managed",
		"-o=jsonpath={.items[*].metadata.name}",
	).Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list secrets in openshift-config-managed: %w", err)
	}

	maxNum := 0
	found := false
	for _, name := range strings.Fields(out) {
		if !re.MatchString(name) {
			continue
		}
		parts := strings.Split(name, "-")
		if len(parts) == 0 {
			continue
		}
		n, err := strconv.Atoi(parts[len(parts)-1])
		if err == nil {
			found = true
			if n > maxNum {
				maxNum = n
			}
		}
	}
	if !found {
		return 0, fmt.Errorf("no encryption key secrets matching pattern %q found", re.String())
	}
	return maxNum, nil
}

// waitEncryptionKeyMigration polls the named secret in openshift-config-managed until the
// "encryption.apiserver.operator.openshift.io/migrated-resources" annotation is non-empty,
// indicating the encryption key migration has completed (up to 30 minutes).
func waitEncryptionKeyMigration(ctx context.Context, oc *exutil.CLI, secretName string) (bool, error) {
	const migrationAnnotation = "encryption.apiserver.operator.openshift.io/migrated-resources"
	err := wait.PollUntilContextTimeout(
		ctx, 30*time.Second, 30*time.Minute, false,
		func(pollCtx context.Context) (bool, error) {
			secret, err := oc.AsAdmin().KubeClient().CoreV1().Secrets("openshift-config-managed").Get(
				pollCtx, secretName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("failed to get secret %s: %v — retrying", secretName, err)
				return false, nil
			}
			if v := secret.Annotations[migrationAnnotation]; v != "" {
				e2e.Logf("encryption migration complete for %s: %s", secretName, v)
				return true, nil
			}
			e2e.Logf("waiting for migration to complete for secret %s", secretName)
			return false, nil
		})
	if err != nil {
		return false, err
	}
	return true, nil
}

// ensureEncryptionEnabled checks if etcd encryption is enabled, and if not, enables it with aescbc.
// Returns the encryption type and a cleanup function that should be deferred.
// The cleanup function will restore encryption to identity (disabled) if it was originally disabled.
func ensureEncryptionEnabled(ctx context.Context, oc *exutil.CLI) (encryptionType string, cleanup func(), err error) {
	currentType, err := oc.WithoutNamespace().Run("get").Args(
		"apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get encryption type: %w", err)
	}

	wasEnabled := (currentType == "aescbc" || currentType == "aesgcm")
	if wasEnabled {
		e2e.Logf("etcd encryption already enabled with type: %s", currentType)
		return currentType, func() {}, nil
	}

	// Encryption not enabled - enable it
	e2e.Logf("etcd encryption is not enabled (current type: %s), enabling aescbc encryption", currentType)

	err = oc.WithoutNamespace().Run("patch").Args(
		"apiserver", "cluster", "--type=merge",
		"-p", `{"spec":{"encryption":{"type":"aescbc"}}}`).Execute()
	if err != nil {
		return "", nil, fmt.Errorf("failed to enable encryption: %w", err)
	}

	// Wait for kube-apiserver operator to start progressing
	err = waitCoBecomes(ctx, oc, "kube-apiserver", 300, map[string]string{"Progressing": "True"})
	if err != nil {
		return "", nil, fmt.Errorf("kube-apiserver operator did not start progressing within 300s: %w", err)
	}

	// Wait for kube-apiserver to stabilize
	e2e.Logf("waiting for kube-apiserver operator to stabilize (≤1800s)")
	err = waitCoBecomes(ctx, oc, "kube-apiserver", 1800, map[string]string{
		"Available":   "True",
		"Progressing": "False",
		"Degraded":    "False",
	})
	if err != nil {
		return "", nil, fmt.Errorf("kube-apiserver operator did not stabilize within 1800s: %w", err)
	}

	// Wait for openshift-apiserver to stabilize
	e2e.Logf("waiting for openshift-apiserver operator to stabilize (≤1800s)")
	err = waitCoBecomes(ctx, oc, "openshift-apiserver", 1800, map[string]string{
		"Available":   "True",
		"Progressing": "False",
		"Degraded":    "False",
	})
	if err != nil {
		return "", nil, fmt.Errorf("openshift-apiserver operator did not stabilize within 1800s: %w", err)
	}

	e2e.Logf("etcd encryption successfully enabled with type: aescbc")

	// Return cleanup function that restores to identity
	cleanupFunc := func() {
		e2e.Logf("restoring encryption to identity (disabled)")
		_ = oc.WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster", "--type=merge",
			"-p", `{"spec":{"encryption":{"type":"identity"}}}`).Execute()
	}

	return "aescbc", cleanupFunc, nil
}

// buildEncryptionKeySecretName constructs the encryption key secret name from prefix and number.
func buildEncryptionKeySecretName(prefix string, keyNumber int) string {
	return prefix + strconv.Itoa(keyNumber)
}
