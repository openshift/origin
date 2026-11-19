package apiserver

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	logEveryNAttemptsKeyRotation    = 12 // Every 12 attempts * 5s = once per minute
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
			err = exutil.ForwardPortAndExecute(target.name, target.namespace, target.port,
				func(port int) error { return exutil.CheckTLSConnection(port, tlsShouldWork, tlsShouldNotWork) })
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("Checking etcd's TLS behavior")
		err = exutil.ForwardPortAndExecute("etcd", "openshift-etcd", "2379", func(port int) error {
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
		err = exutil.ForwardPortAndExecute("apiserver", "openshift-kube-apiserver", "443", func(port int) error {
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

var _ = g.Describe("[sig-api-machinery] [Jira:apiserver-auth] Operators / Certs", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("apiserver-certs")

	// Add a custom TLS certificate to the cluster API server, verify it is served,
	// then restore the original configuration.
	g.It("[OTP][OCP-70020] should support adding a custom TLS certificate for the cluster API [Disruptive][Slow][Timeout:50m][apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isHyperShift {
				g.Skip("custom serving certificates for the API server are managed by HyperShift — skipping")
			}

			tmpdir := g.GinkgoT().TempDir()
			// Use unique secret name to avoid conflicts with pre-existing secrets
			testSecretName := fmt.Sprintf("custom-api-cert-test-%d", time.Now().UnixNano())

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
				// Use detached context to ensure cleanup runs even if spec context is cancelled
				_, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Minute)
				defer cancel()

				patchOut, patchErr := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
					"apiserver/cluster", "--type=merge", "-p", patchToRecover).Output()
				if patchErr != nil {
					e2e.Logf("error restoring apiserver/cluster: %v, output: %s", patchErr, patchOut)
				}
				o.Expect(patchErr).NotTo(o.HaveOccurred(), "failed to restore apiserver/cluster configuration")

				restoreData, readErr := os.ReadFile(originKubeconfigBkp)
				o.Expect(readErr).NotTo(o.HaveOccurred(), "failed to read kubeconfig backup")
				writeErr := os.WriteFile(originKubeconfig, restoreData, 0600)
				o.Expect(writeErr).NotTo(o.HaveOccurred(), "failed to restore kubeconfig")

				err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("wait-for-stable-cluster").Execute()
				o.Expect(err).NotTo(o.HaveOccurred(), "cluster did not stabilize after restore")

				// Only delete the test-specific secret, not any pre-existing secrets
				err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(
					"secret", testSecretName, "-n", "openshift-config", "--ignore-not-found").Execute()
				o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete test secret")
			}()

			fqdnName, port := exutil.GetAPIServerFQDNAndPort(ctx, oc)

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
			err = exutil.WaitCoBecomes(ctx, oc, "kube-apiserver", 300, map[string]string{"Progressing": "True"})
			o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator did not start progressing within 300s")

			e2e.Logf("waiting for kube-apiserver operator to become stable (≤1500s)")
			err = exutil.WaitCoBecomes(ctx, oc, "kube-apiserver", 1500, map[string]string{
				"Available":   "True",
				"Progressing": "False",
				"Degraded":    "False",
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator did not stabilise within 1500s")

			g.By("8. validate that the custom certificate is now served by the API server")
			// Poll for the custom certificate to be served (may take time for all apiservers to reload)
			var certDetails *exutil.CertInfo
			pollErr := wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, false,
				func(pollCtx context.Context) (bool, error) {
					certDetails, err = exutil.GetServerCertInfo(pollCtx, fqdnName, port, caCertpem)
					if err != nil {
						e2e.Logf("custom certificate not yet served, retrying: %v", err)
						return false, nil
					}
					return true, nil
				})
			o.Expect(pollErr).NotTo(o.HaveOccurred(), "custom certificate was not served within 5 minutes")
			o.Expect(certDetails.Subject).To(o.ContainSubstring("CN=kas-test-cert_server"))
			o.Expect(certDetails.Issuer).To(o.ContainSubstring("CN=kas-test-cert_ca"))

			g.By("9. validate the original CA no longer verifies the new certificate")
			_, err = exutil.GetServerCertInfo(ctx, fqdnName, port, originCA)
			o.Expect(err).To(o.HaveOccurred(), "original CA should not verify the new custom certificate")
		})

	// Delete etcd encryption config and key secrets, then verify the cluster
	// self-heals by recreating them and completing re-encryption.
	g.It("[OTP][OCP-25811] should self-recover when etcd encryption configuration secrets are deleted [Disruptive][Slow][Timeout:50m][apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			g.By("1. ensure etcd encryption is enabled")
			_, cleanup, err := exutil.EnsureEncryptionEnabled(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to ensure encryption is enabled")
			defer cleanup()

			nameUIDMapJSON, err := oc.WithoutNamespace().Run("get").Args(
				"secret",
				"encryption-config-openshift-apiserver",
				"encryption-config-openshift-kube-apiserver",
				"-n", "openshift-config-managed",
				"-o=jsonpath={range .items[*]}{.metadata.name}{\" \"}{.metadata.uid}{\"\\n\"}{end}",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			uidsOldMap := make(map[string]string) // map secret name → UID
			for _, line := range strings.Split(strings.TrimSpace(nameUIDMapJSON), "\n") {
				parts := strings.Fields(line)
				if len(parts) == 2 {
					uidsOldMap[parts[0]] = parts[1]
				}
			}
			e2e.Logf("original secrets captured: %v", uidsOldMap)

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

			// Use an explicit timeout context derived from the spec context so Ginkgo cancellation still works
			pollCtx1, cancel1 := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel1()

			retryCount1 := 0
			errSecret := wait.PollUntilContextCancel(pollCtx1, 3*time.Second, false,
				func(pollCtx context.Context) (bool, error) {
					nameUIDMapJSONNew, err := oc.WithoutNamespace().Run("get").Args(
						"secret",
						"encryption-config-openshift-apiserver",
						"encryption-config-openshift-kube-apiserver",
						"-n", "openshift-config-managed",
						"-o=jsonpath={range .items[*]}{.metadata.name}{\" \"}{.metadata.uid}{\"\\n\"}{end}",
					).Output()
					if err != nil {
						retryCount1++
						// Only log every Nth attempt to reduce noise
						if retryCount1%logEveryNAttemptsSecretRecreate == 1 {
							e2e.Logf("waiting for encryption-config-* secrets to be recreated (attempt %d)", retryCount1)
						}
						return false, nil
					}
					uidsNewMap := make(map[string]string)
					for _, line := range strings.Split(strings.TrimSpace(nameUIDMapJSONNew), "\n") {
						parts := strings.Fields(line)
						if len(parts) == 2 {
							uidsNewMap[parts[0]] = parts[1]
						}
					}
					// Check that both secrets were recreated (different UIDs)
					oasRecreated := uidsOldMap["encryption-config-openshift-apiserver"] != uidsNewMap["encryption-config-openshift-apiserver"]
					kasRecreated := uidsOldMap["encryption-config-openshift-kube-apiserver"] != uidsNewMap["encryption-config-openshift-kube-apiserver"]
					if len(uidsNewMap) >= 2 && oasRecreated && kasRecreated {
						e2e.Logf("encryption-config-* secrets recreated after %d attempts (UIDs changed)", retryCount1)
						return true, nil
					}
					return false, nil
				})
			o.Expect(errSecret).NotTo(o.HaveOccurred(),
				"encryption-config-* secrets were not recreated within 5 minutes")

			oasEncNumber, err := exutil.GetEncryptionKeyNumber(oc, `^encryption-key-openshift-apiserver-\d+$`)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get openshift-apiserver encryption key number")
			kasEncNumber, err := exutil.GetEncryptionKeyNumber(oc, `^encryption-key-openshift-kube-apiserver-\d+$`)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get kube-apiserver encryption key number")

			oldOASEncSecretName := exutil.BuildEncryptionKeySecretName(encryptionKeyOASPrefix, oasEncNumber)
			oldKASEncSecretName := exutil.BuildEncryptionKeySecretName(encryptionKeyKASPrefix, kasEncNumber)

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

			newOASEncSecretName := exutil.BuildEncryptionKeySecretName(encryptionKeyOASPrefix, oasEncNumber+1)
			newKASEncSecretName := exutil.BuildEncryptionKeySecretName(encryptionKeyKASPrefix, kasEncNumber+1)

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
			completedOAS, errOAS := exutil.WaitEncryptionKeyMigration(ctx, oc, newOASEncSecretName)
			o.Expect(errOAS).NotTo(o.HaveOccurred(),
				"encryption key migration did not complete for %s", newOASEncSecretName)
			o.Expect(completedOAS).To(o.BeTrue())

			completedKAS, errKAS := exutil.WaitEncryptionKeyMigration(ctx, oc, newKASEncSecretName)
			o.Expect(errKAS).NotTo(o.HaveOccurred(),
				"encryption key migration did not complete for %s", newKASEncSecretName)
			o.Expect(completedKAS).To(o.BeTrue())
		})
})
