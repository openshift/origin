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
	"github.com/openshift/origin/test/extended/apiserverauth"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
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
	g.It("[OTP][OCP-70020] should support adding a custom TLS certificate for the cluster API [APIServer_Disruptive][Disruptive][Slow][Timeout:50m][apigroup:config.openshift.io]",
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
	g.It("[OTP][OCP-25811] should self-recover when etcd encryption configuration secrets are deleted [APIServer_Disruptive][Disruptive][Slow][Timeout:50m][apigroup:config.openshift.io]",
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

	g.It("[OTP][OCP-25926] should propagate TLS cipher configuration from apiservers/cluster to control plane operators [APIServer_Disruptive][Slow][Disruptive]", ote.Informing(), func() {
		var (
			availableWaitTime = 1800
			cipherToRecover   = `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":}]`
			cipherOps         = []string{"openshift-authentication", "openshiftapiservers.operator", "kubeapiservers.operator"}
			cipherToMatch     = `["TLS_AES_128_GCM_SHA256","TLS_AES_256_GCM_SHA384","TLS_CHACHA20_POLY1305_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256"] VersionTLS12`
		)

		cipherItems := []struct {
			cipherType    string
			cipherToCheck string
			patch         string
		}{
			{
				cipherType:    "custom",
				cipherToCheck: `["TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"] VersionTLS11`,
				patch:         `[{"op": "add", "path": "/spec/tlsSecurityProfile", "value":{"custom":{"ciphers":["ECDHE-ECDSA-CHACHA20-POLY1305","ECDHE-RSA-CHACHA20-POLY1305","ECDHE-RSA-AES128-GCM-SHA256","ECDHE-ECDSA-AES128-GCM-SHA256"],"minTLSVersion":"VersionTLS11"},"type":"Custom"}}]`,
			},
			{
				cipherType:    "Intermediate",
				cipherToCheck: cipherToMatch,
				patch:         `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":{"intermediate":{},"type":"Intermediate"}}]`,
			},
			{
				cipherType:    "Old",
				cipherToCheck: `["TLS_AES_128_GCM_SHA256","TLS_AES_256_GCM_SHA384","TLS_CHACHA20_POLY1305_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256","TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384","TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256","TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256","TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA","TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA","TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA","TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA","TLS_RSA_WITH_AES_128_GCM_SHA256","TLS_RSA_WITH_AES_256_GCM_SHA384","TLS_RSA_WITH_AES_128_CBC_SHA256","TLS_RSA_WITH_AES_128_CBC_SHA","TLS_RSA_WITH_AES_256_CBC_SHA","TLS_RSA_WITH_3DES_EDE_CBC_SHA"] VersionTLS10`,
				patch:         `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":{"old":{},"type":"Old"}}]`,
			},
			{
				cipherType:    "Modern",
				cipherToCheck: `["TLS_AES_128_GCM_SHA256","TLS_AES_256_GCM_SHA384","TLS_CHACHA20_POLY1305_SHA256"] VersionTLS13`,
				patch:         `[{"op": "replace", "path": "/spec/tlsSecurityProfile", "value":{"modern":{},"type":"Modern"}}]`,
			},
		}

		g.By("Check if it's a proxy cluster")
		httpProxy, httpsProxy, _ := getGlobalProxy(oc)
		if strings.Contains(httpProxy, "http") || strings.Contains(httpsProxy, "https") {
			g.Skip("Skip for proxy platform")
		}

		g.By("Verify initial cipher configuration for all control plane operators")
		for _, op := range cipherOps {
			err := apiserverauth.VerifyCiphers(oc, cipherToMatch, op)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Initial cipher verification failed for %s", op))
		}

		defer func() {
			g.By("Restoring apiserver/cluster's TLS security profile to default")
			output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", cipherToRecover).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if strings.Contains(output, "patched (no change)") {
				e2e.Logf("APIServer TLS security profile was not changed from default")
			} else {
				e2e.Logf("Verifying ciphers are restored to default")
				for _, op := range cipherOps {
					err := apiserverauth.VerifyCiphers(oc, cipherToMatch, op)
					o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to restore ciphers for %s", op))
				}

				g.By("Waiting for operators to rollout and recover")
				waitForOperatorsAvailable(oc, availableWaitTime)
			}
		}()

		for _, cipherItem := range cipherItems {
			g.By(fmt.Sprintf("Testing TLS security profile: %s", cipherItem.cipherType))

			g.By(fmt.Sprintf("Patching apiserver/cluster with %s profile", cipherItem.cipherType))
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", cipherItem.patch).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("Verifying cipher configuration propagated to all operators for %s profile", cipherItem.cipherType))
			for _, op := range cipherOps {
				err := apiserverauth.VerifyCiphers(oc, cipherItem.cipherToCheck, op)
				o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Cipher verification failed for %s with profile %s", op, cipherItem.cipherType))
			}

			g.By("Waiting for control plane operators to complete rollout")
			waitForOperatorsAvailable(oc, availableWaitTime)

			if cipherItem.cipherType == "Modern" {
				g.By("Verifying Modern profile propagated to kubelet configuration on node")
				verifyKubeletModernTLSConfig(oc)
			}

			e2e.Logf("Successfully validated %s TLS profile", cipherItem.cipherType)
		}
	})
})

// waitForOperatorsAvailable waits for kube-apiserver, openshift-apiserver, and authentication operators to become available
func waitForOperatorsAvailable(oc *exutil.CLI, availableWaitTime int) {
	operators := []struct {
		name         string
		waitTime     int
		progressWait int
	}{
		{"kube-apiserver", availableWaitTime, 100},
		{"openshift-apiserver", 300, 0},
		{"authentication", 500, 0},
	}

	for _, op := range operators {
		if op.progressWait > 0 {
			e2e.Logf("Checking %s operator should be Progressing in %d seconds", op.name, op.progressWait)
			expectedStatus := map[string]string{"Progressing": "True"}
			err := waitCoBecomes(oc, op.name, op.progressWait, expectedStatus)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("%s operator did not start progressing in %d seconds", op.name, op.progressWait))
		}

		e2e.Logf("Checking %s operator should be Available in %d seconds", op.name, op.waitTime)
		expectedStatus := map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err := waitCoBecomes(oc, op.name, op.waitTime, expectedStatus)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("%s operator did not become available in %d seconds", op.name, op.waitTime))
	}

	e2e.Logf("All control plane operators are available")
}

// verifyKubeletModernTLSConfig verifies that Modern TLS profile settings are applied to kubelet
func verifyKubeletModernTLSConfig(oc *exutil.CLI) {
	g.By("Get a master node")
	masterNode, err := compat_otp.GetFirstMasterNode(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(masterNode).NotTo(o.Equal(""))

	g.By("Get host IP from openshift-apiserver pod")
	podsList := getPodsListByLabel(oc.AsAdmin(), "openshift-apiserver", "apiserver")
	o.Expect(podsList).ShouldNot(o.BeEmpty())

	hostIP, err := oc.Run("get").Args("pods", podsList[0], "-o=jsonpath={.status.hostIP}", "-n", "openshift-apiserver").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hostIP).NotTo(o.Equal(""))

	var host string
	if strings.Contains(hostIP, ":") {
		host = "[::1]:10250"
	} else {
		host = "127.0.0.1:10250"
	}

	g.By("Checking kubelet TLS version on node using OpenSSL")
	runOpenSSL := func(tlsVersion string) {
		cmd := fmt.Sprintf("openssl s_client -connect %s -%s 2>/dev/null || true", host, tlsVersion)
		result, debugErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
		o.Expect(debugErr).NotTo(o.HaveOccurred())

		if strings.Contains(result, "Certificate chain") {
			e2e.Logf("OpenSSL %s supported! Certificate chain available.", tlsVersion)
		} else if strings.Contains(result, "no peer certificate available") {
			e2e.Logf("OpenSSL %s connected, but no peer certificate available.", tlsVersion)
		} else {
			e2e.Logf("OpenSSL %s not supported!", tlsVersion)
		}
	}

	runOpenSSL("tls1_3")
	runOpenSSL("tls1_2")
	runOpenSSL("tls1_1")

	g.By("Verify kubelet config shows Modern TLS version (VersionTLS13)")
	cmd := "grep tlsMinVersion /etc/kubernetes/kubelet.conf"
	output, debugErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
	o.Expect(debugErr).NotTo(o.HaveOccurred())
	o.Expect(output).Should(o.ContainSubstring("tlsMinVersion: VersionTLS13"), "Kubelet should be configured with TLS 1.3 for Modern profile")

	e2e.Logf("Kubelet Modern TLS configuration verified successfully")
}

// Additional encryption and TLS tests
var _ = g.Describe("[sig-api-machinery][Feature:APIServer] Encryption and TLS advanced scenarios", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("apiserver-encryption-tls")
	ctx := context.Background()

	g.It("[OTP][OCP-63273] should support etcd encryption type migration between aescbc and aesgcm [APIServer_Disruptive][Slow][Disruptive][apigroup:config.openshift.io]", ote.Informing(), func() {
		g.By("1) Check if cluster has Etcd Encryption enabled")
		encryptionType, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		originalEncryptionType := encryptionType
		if encryptionType != "aescbc" && encryptionType != "aesgcm" {
			e2e.Logf("Etcd Encryption is currently disabled, enabling it with aescbc first")

			g.By("1a) Enable encryption with aescbc")
			patchArg := `{"spec":{"encryption": {"type":"aescbc"}}}`
			_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=merge", "-p", patchArg).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("1b) Wait for encryption to be enabled")
			err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, false, func(cxt context.Context) (bool, error) {
				currentType, err := oc.WithoutNamespace().Run("get").Args("apiserver/cluster", "-o=jsonpath={.spec.encryption.type}").Output()
				if err != nil {
					return false, nil
				}
				return currentType == "aescbc", nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "encryption was not enabled within 10 minutes")

			g.By("1c) Wait for kube-apiserver operator to become available after enabling encryption")
			err = waitCoBecomes(oc, "kube-apiserver", 3000, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"})
			o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator not available after enabling encryption")

			encryptionType = "aescbc"
			e2e.Logf("Etcd Encryption enabled successfully with type: aescbc")
		} else {
			e2e.Logf("Etcd Encryption is already enabled with type: %s", encryptionType)
		}

		defer func() {
			if originalEncryptionType != "aescbc" && originalEncryptionType != "aesgcm" {
				g.By("Cleanup) Disable encryption at end of test")
				patchArg := `{"spec":{"encryption": {"type":""}}}`
				_, _ = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=merge", "-p", patchArg).Output()
			}
		}()

		g.By("2) Check encryption-config and key secrets before migration")
		encSecretOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", "openshift-config-managed", "-l", "encryption.apiserver.operator.openshift.io/component", "-o", `jsonpath={.items[*].metadata.name}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		encSecretCount := strings.Count(encSecretOut, "encryption")
		o.Expect(encSecretCount).To(o.BeNumerically(">", 0), "no encryption secrets found")
		e2e.Logf("Found %d encryption-related secrets", encSecretCount)

		g.By("3) Create secret and verify it's encrypted in etcd database before migration")
		testSecretName := "secret-63273-" + compat_otp.GetRandomString()
		defer oc.WithoutNamespace().Run("delete").Args("-n", "default", "secret", testSecretName, "--ignore-not-found").Execute()

		err = oc.WithoutNamespace().Run("create").Args("-n", "default", "secret", "generic", testSecretName, "--from-literal", "pass=secret123").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		etcdPods := getPodsListByLabel(oc, "openshift-etcd", "etcd=true")
		o.Expect(etcdPods).ShouldNot(o.BeEmpty(), "no etcd pods found")

		execCmdOutput := execCommandOnPod(oc, etcdPods[0], "openshift-etcd", "etcdctl get /kubernetes.io/secrets/default/"+testSecretName)
		o.Expect(execCmdOutput).ShouldNot(o.ContainSubstring("secret123"), "secret should be encrypted in etcd, but plaintext was found")
		e2e.Logf("Verified secret is encrypted in etcd")

		g.By("4) Determine target encryption type for migration")
		migrateEncTo := "aesgcm"
		if encryptionType == "aesgcm" {
			migrateEncTo = "aescbc"
		}
		e2e.Logf("Will migrate encryption from %s to %s", encryptionType, migrateEncTo)

		oasEncNumber, err := exutil.GetEncryptionKeyNumber(oc, `^encryption-key-openshift-apiserver-\d+$`)
		o.Expect(err).NotTo(o.HaveOccurred())
		kasEncNumber, err := exutil.GetEncryptionKeyNumber(oc, `^encryption-key-openshift-kube-apiserver-\d+$`)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Current encryption key numbers: OAS=%d, KAS=%d", oasEncNumber, kasEncNumber)

		g.By("5) Trigger encryption type migration")
		patchArg := fmt.Sprintf(`{"spec":{"encryption": {"type":"%s"}}}`, migrateEncTo)
		encMigrateOut, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=merge", "-p", patchArg).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(encMigrateOut).Should(o.ContainSubstring("patched"))
		e2e.Logf("Started encryption migration to %s", migrateEncTo)

		g.By("6) Wait for new encryption key secrets to appear")
		newOASEncSecretName := exutil.BuildEncryptionKeySecretName(encryptionKeyOASPrefix, oasEncNumber+1)
		newKASEncSecretName := exutil.BuildEncryptionKeySecretName(encryptionKeyKASPrefix, kasEncNumber+1)

		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, false, func(cxt context.Context) (bool, error) {
			_, err := oc.WithoutNamespace().Run("get").Args("secrets", newOASEncSecretName, newKASEncSecretName, "-n", "openshift-config-managed").Output()
			if err != nil {
				e2e.Logf("Waiting for new encryption-key-* secrets, error: %s", err)
				return false, nil
			}
			e2e.Logf("New encryption-key-* secrets found")
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "new encryption key secrets %s, %s not found within 5 minutes", newOASEncSecretName, newKASEncSecretName)

		g.By("7) Wait for encryption migration to complete")
		completed, errOAS := exutil.WaitEncryptionKeyMigration(ctx, oc, newOASEncSecretName)
		o.Expect(errOAS).NotTo(o.HaveOccurred(), "encryption migration failed for %s", newOASEncSecretName)
		o.Expect(completed).Should(o.BeTrue())

		completed, errKAS := exutil.WaitEncryptionKeyMigration(ctx, oc, newKASEncSecretName)
		o.Expect(errKAS).NotTo(o.HaveOccurred(), "encryption migration failed for %s", newKASEncSecretName)
		o.Expect(completed).Should(o.BeTrue())

		e2e.Logf("Encryption migration completed successfully")

		g.By("8) Verify kube-apiserver operator is available after migration")
		err = waitCoBecomes(oc, "kube-apiserver", 1500, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"})
		o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator not available after encryption migration")

		g.By("9) Verify secret is still encrypted in etcd after migration")
		etcdPods = getPodsListByLabel(oc, "openshift-etcd", "etcd=true")
		execCmdOutput = execCommandOnPod(oc, etcdPods[0], "openshift-etcd", "etcdctl get /kubernetes.io/secrets/default/"+testSecretName)
		o.Expect(execCmdOutput).ShouldNot(o.ContainSubstring("secret123"), "secret should remain encrypted after migration")

		e2e.Logf("Encryption type migration from %s to %s completed successfully", encryptionType, migrateEncTo)
	})

	g.It("[OTP][OCP-65924] should handle non-existent secret for API namedCertificates gracefully [APIServer_Disruptive][Disruptive][apigroup:config.openshift.io]", ote.Informing(), func() {
		var (
			apiserver           = "apiserver/cluster"
			kasOpExpectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			kasOpDegradedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "True"}
		)

		g.By("1) Get API server FQDN")
		apiServerFQDN, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("--show-server").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		apiServerFQDN = strings.TrimPrefix(apiServerFQDN, "https://")
		apiServerFQDN = strings.Split(apiServerFQDN, ":")[0]
		e2e.Logf("API Server FQDN: %s", apiServerFQDN)

		g.By("2) Get current kube-apiserver revision")
		out, err := oc.AsAdmin().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", "jsonpath={.items[*].metadata.labels.revision}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		revisions := strings.Split(out, " ")
		preRevisionSum := 0
		for _, valueStr := range revisions {
			valueInt, _ := strconv.Atoi(valueStr)
			preRevisionSum += valueInt
		}
		e2e.Logf("Current kube-apiserver revisions sum: %d", preRevisionSum)

		defer func() {
			g.By("Cleanup: Remove the non-existent secret configuration")
			patchToRecover := `[{ "op": "remove", "path": "/spec/servingCerts" }]`
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(apiserver, "-p", patchToRecover, "--type=json").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			e2e.Logf("Waiting for kube-apiserver operator to recover")
			err = waitCoBecomes(oc, "kube-apiserver", 300, kasOpExpectedStatus)
			o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator did not recover")
		}()

		g.By("3) Apply non-existent secret for API namedCertificates")
		patch := fmt.Sprintf(`{"spec":{"servingCerts": {"namedCertificates": [{"names": ["%s"], "servingCertificate": {"name": "client-ca-custom"}}]}}}`, apiServerFQDN)
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(apiserver, "-p", patch, "--type=merge").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Patched apiserver with non-existent secret reference")

		g.By("4) Wait and verify kube-apiserver cluster operator becomes degraded")
		err = waitCoBecomes(oc, "kube-apiserver", 300, kasOpDegradedStatus)
		o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator should become degraded")

		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "kube-apiserver").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("ConfigObservationDegraded"), "operator should show ConfigObservationDegraded condition")
		e2e.Logf("kube-apiserver operator correctly degraded with ConfigObservationDegraded")

		g.By("5) Verify cluster does nothing and no kube-apiserver pod crash-looping")
		out, err = oc.AsAdmin().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", "jsonpath={.items[*].metadata.labels.revision}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		revisions = strings.Split(out, " ")
		postRevisionSum := 0
		for _, valueStr := range revisions {
			valueInt, _ := strconv.Atoi(valueStr)
			postRevisionSum += valueInt
		}
		o.Expect(postRevisionSum).Should(o.Equal(preRevisionSum), "kube-apiserver pods should not roll out with invalid config")
		e2e.Logf("Revisions unchanged - no rollout occurred as expected")

		kasPodsOutput := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pods", "-l apiserver", "--no-headers", "-n", "openshift-kube-apiserver")
		o.Expect(kasPodsOutput).ShouldNot(o.ContainSubstring("CrashLoopBackOff"), "kube-apiserver pods should not crash-loop")
		e2e.Logf("kube-apiserver pods are stable - no crash-looping")
	})

	g.It("[OTP][OCP-84783] should verify certificates for authentication and encryption of communications [apigroup:config.openshift.io]", ote.Informing(), func() {
		g.By("1) Verify TLS secrets in openshift-kube-apiserver namespace")
		ns := "openshift-kube-apiserver"
		e2e.Logf("OpenShift TLS Secrets Verification")

		secretsJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "-n", ns, "-ojson").Output()
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

		err = json.Unmarshal([]byte(secretsJSON), &secretList)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to parse secrets JSON")

		secretCount := 0
		for _, s := range secretList.Items {
			if s.Type != "kubernetes.io/tls" {
				continue
			}
			name := s.Metadata.Name
			o.Expect(name).ShouldNot(o.BeEmpty(), "secret name should not be empty")

			rawcrt, ok := s.Data["tls.crt"]
			if !ok || rawcrt == "" {
				e2e.Logf("%s/%s - No tls.crt found", ns, name)
				continue
			}

			decoded, err := base64.StdEncoding.DecodeString(rawcrt)
			if err != nil {
				e2e.Logf("%s/%s - failed to base64-decode tls.crt: %v", ns, name, err)
				continue
			}

			e2e.Logf("Checking %s/%s", ns, name)
			apiserverauth.ParseAndCheckPEMs(decoded, fmt.Sprintf("%s/%s", ns, name))
			secretCount++
		}
		e2e.Logf("Verified %d TLS secrets in %s namespace", secretCount, ns)

		g.By("2) Verify CA bundle ConfigMaps in openshift-config-managed namespace")
		e2e.Logf("OpenShift CA Bundles Verification")

		ns = "openshift-config-managed"
		configMaps := []struct {
			name string
			key  string
		}{
			{"kube-apiserver-server-ca", "ca-bundle.crt"},
			{"kube-apiserver-client-ca", "ca-bundle.crt"},
			{"kube-root-ca.crt", "ca.crt"},
			{"trusted-ca-bundle", "ca-bundle.crt"},
			{"service-ca", "ca-bundle.crt"},
		}

		cmCount := 0
		for _, cm := range configMaps {
			cmJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cm", cm.name, "-n", ns, "-ojson").Output()
			if err != nil {
				e2e.Logf("%s/%s (%s) - failed to get configmap: %v", ns, cm.name, cm.key, err)
				continue
			}

			var cmObj struct {
				Data map[string]string `json:"data"`
			}
			err = json.Unmarshal([]byte(cmJSON), &cmObj)
			if err != nil {
				e2e.Logf("%s/%s (%s) - failed to parse configmap JSON: %v", ns, cm.name, cm.key, err)
				continue
			}

			val, ok := cmObj.Data[cm.key]
			if !ok || val == "" {
				found := false
				for k, v := range cmObj.Data {
					kl := strings.ToLower(k)
					if (strings.Contains(kl, "ca") || strings.Contains(kl, "crt")) && v != "" {
						e2e.Logf("%s/%s (using key %s)", ns, cm.name, k)
						apiserverauth.ParseAndCheckPEMs([]byte(v), fmt.Sprintf("%s/%s(%s)", ns, cm.name, k))
						found = true
						cmCount++
						break
					}
				}
				if !found {
					e2e.Logf("%s/%s (%s) - key not found in configmap", ns, cm.name, cm.key)
				}
				continue
			}

			e2e.Logf("Checking %s/%s (%s)", ns, cm.name, cm.key)
			apiserverauth.ParseAndCheckPEMs([]byte(val), fmt.Sprintf("%s/%s(%s)", ns, cm.name, cm.key))
			cmCount++
		}
		e2e.Logf("Verified %d CA bundle ConfigMaps in %s namespace", cmCount, ns)
		e2e.Logf("Certificate verification complete")
	})
})
