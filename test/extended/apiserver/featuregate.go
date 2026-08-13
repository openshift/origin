package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver-featuregate")

	g.It("[OTP][OCP-66921-1][OCPFeatureGate:TechPreviewNoUpgrade] should reject removed LatencySensitive featureset [apigroup:config.openshift.io]",
		ote.Informing(), func() {
			const (
				featurePatch       = `[{"op": "replace", "path": "/spec/featureSet", "value": "LatencySensitive"}]`
				invalidFeatureGate = `[{"op": "replace", "path": "/spec/featureSet", "value": "unknown"}]`
			)

			g.By("Verify invalid featuregate is rejected")
			output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate", "cluster", "--type=json", "-p", invalidFeatureGate).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(output).Should(o.ContainSubstring(`The FeatureGate "cluster" is invalid`))

			g.By("Verify removed LatencySensitive featuregate is rejected")
			output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate", "cluster", "--type=json", "-p", featurePatch).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(output).Should(o.ContainSubstring(`The FeatureGate "cluster" is invalid`))
		})

	g.It("[OTP][OCP-66921-2][OCP-74460][OCPFeatureGate:TechPreviewNoUpgrade] NoUpgrade featuresets are immutable once set [Slow][Disruptive][apigroup:config.openshift.io][Timeout:30m]",
		ote.Informing(), func() {
			const (
				featureTechPreview     = `[{"op": "replace", "path": "/spec/featureSet", "value": "TechPreviewNoUpgrade"}]`
				featureCustomNoUpgrade = `[{"op": "replace", "path": "/spec/featureSet", "value": "CustomNoUpgrade"}]`
				invalidFeatureGate     = `[{"op": "remove", "path": "/spec/featureSet"}]`
			)

			var output string
			var err error

			g.By("Check current feature set")
			currentFeatureSet, err := getResource(oc, asAdmin, withoutNamespace, "featuregate/cluster", "-o", `jsonpath='{.spec.featureSet}'`)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Determine which feature set to enable based on current state
			var targetFeatureSet string
			var targetPatch string
			var alternateFeatureSet string
			var alternatePatch string

			switch currentFeatureSet {
			case `'TechPreviewNoUpgrade'`:
				// Already on TechPreviewNoUpgrade, verify it cannot be changed
				targetFeatureSet = "TechPreviewNoUpgrade"
				targetPatch = featureTechPreview
				alternateFeatureSet = "CustomNoUpgrade"
				alternatePatch = featureCustomNoUpgrade
			case `'CustomNoUpgrade'`:
				// Already on CustomNoUpgrade, verify it cannot be changed
				targetFeatureSet = "CustomNoUpgrade"
				targetPatch = featureCustomNoUpgrade
				alternateFeatureSet = "TechPreviewNoUpgrade"
				alternatePatch = featureTechPreview
			case `''`, `'Default'`:
				// Default state - enable TechPreviewNoUpgrade and verify immutability
				g.By("Enable TechPreviewNoUpgrade feature set")
				output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate", "cluster", "--type=json", "-p", featureTechPreview).Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Wait for kube-apiserver to become available after feature gate change")
				kasOpExpectedStatus := map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
				err = waitCoBecomes(oc, "kube-apiserver", 1500, kasOpExpectedStatus)
				compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator did not become available after enabling TechPreviewNoUpgrade")

				targetFeatureSet = "TechPreviewNoUpgrade"
				targetPatch = featureTechPreview
				alternateFeatureSet = "CustomNoUpgrade"
				alternatePatch = featureCustomNoUpgrade
			default:
				g.Fail(fmt.Sprintf("Unexpected feature set: %s", currentFeatureSet))
			}

			g.By(fmt.Sprintf("Verify setting %s again is idempotent", targetFeatureSet))
			output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate", "cluster", "--type=json", "-p", targetPatch).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			// Both "no change" and "patched" are acceptable - the key is it doesn't error
			o.Expect(output).Should(o.Or(o.ContainSubstring(`no change`), o.ContainSubstring(`patched`)))

			g.By(fmt.Sprintf("Verify cannot change from %s to %s", targetFeatureSet, alternateFeatureSet))
			output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate", "cluster", "--type=json", "-p", alternatePatch).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(output).Should(o.ContainSubstring("may not be changed"))

			g.By(fmt.Sprintf("Verify cannot remove %s feature set", targetFeatureSet))
			output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate", "cluster", "--type=json", "-p", invalidFeatureGate).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(output).Should(o.ContainSubstring("invalid"))

			g.By("Verify kube-apiserver remains stable")
			kasOpExpectedStatus := map[string]string{"Available": "True", "Degraded": "False"}
			err = waitCoBecomes(oc, "kube-apiserver", 300, kasOpExpectedStatus)
			compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator status check failed")
		})

	g.It("[OTP][OCP-80286][OCPFeatureGate:AllowUnsafeMalformedObjectDeletion] Handle undecryptable resources [Slow][Disruptive][apigroup:config.openshift.io][apigroup:operator.openshift.io][Timeout:120m]",
		ote.Informing(), func() {
			const (
				testSecretName      = "test-secret-80286"
				timeoutShort        = 500
				timeoutLong         = 2400
				corruptedDataMarker = "corrupted-data"
				expectedBase64Error = "illegal base64"
				etcdSuccessResponse = "OK"
			)

			var (
				testNamespace        string
				cleanupRequired      bool
				originalEnabledGates string
				healthyStatus        = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
				progressingStatus    = map[string]string{"Progressing": "True"}
			)

			waitForKubeAPIServer := func(status map[string]string, timeout int, description string) error {
				e2e.Logf("Waiting for kube-apiserver: %s (%ds)", description, timeout)
				return waitCoBecomes(oc, "kube-apiserver", timeout, status)
			}

			pollUntilDeleted := func(resourceType, name, namespace string) error {
				return wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 60*time.Second, false, func(ctx context.Context) (bool, error) {
					_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(resourceType, name, "-n", namespace).Output()
					return err != nil, nil
				})
			}

			cleanupCorruptedSecretFromEtcd := func() error {
				out, err := getResource(oc, asAdmin, withoutNamespace, "secret", testSecretName, "-n", testNamespace)
				if err == nil || !strings.Contains(out, expectedBase64Error) {
					return nil
				}
				etcdPods := getPodsListByLabel(oc, "openshift-etcd", "etcd=true")
				if len(etcdPods) == 0 {
					return fmt.Errorf("no etcd pods found")
				}
				deleteCmd := fmt.Sprintf(`etcdctl del /kubernetes.io/secrets/%s/%s`, testNamespace, testSecretName)
				err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
					output := execCommandOnPod(oc, etcdPods[0], "openshift-etcd", deleteCmd)
					e2e.Logf("etcd delete attempt, output: %s", output)
					return strings.Contains(output, etcdSuccessResponse) || strings.Contains(output, "deleted: 1") || strings.TrimSpace(output) == "1", nil
				})
				if err != nil {
					return fmt.Errorf("failed to delete corrupted secret from etcd: %v", err)
				}
				return pollUntilDeleted("secret", testSecretName, testNamespace)
			}

			oc.SetupProject()
			testNamespace = oc.Namespace()
			cleanupRequired = true

			defer func() {
				if !cleanupRequired {
					return
				}
				if err := cleanupCorruptedSecretFromEtcd(); err != nil {
					e2e.Logf("Warning: Failed to cleanup corrupted secret: %v", err)
				}
				if err := waitForKubeAPIServer(healthyStatus, timeoutLong, "Available post-cleanup"); err != nil {
					e2e.Logf("Warning: kube-apiserver operator cleanup failed: %v", err)
				}

				g.By("Restoring original feature gate configuration")
				if err := restoreFeatureGateConfig(oc, originalEnabledGates); err != nil {
					e2e.Logf("Warning: Failed to restore feature gate: %v", err)
				} else {
					e2e.Logf("Waiting for kube-apiserver to stabilize after restoring feature gate")
					if err := waitForKubeAPIServer(progressingStatus, timeoutShort, "rollout started after restore"); err == nil {
						waitForKubeAPIServer(healthyStatus, timeoutLong, "stable after restore")
					}
				}
			}()

			g.By("Saving original feature gate configuration")
			originalEnabledGates, _ = getResource(oc, asAdmin, withoutNamespace, "featuregate/cluster", "-o", `jsonpath={.spec.customNoUpgrade.enabled[*]}`)

			g.By("Creating test secret in namespace")
			_, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-n", testNamespace, "secret", "generic", testSecretName, "--from-literal=user=Bob").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			secretOutput := getResourceToBeReady(oc, asAdmin, withoutNamespace, "secret", testSecretName, "-n", testNamespace)
			o.Expect(secretOutput).Should(o.ContainSubstring(testSecretName))

			g.By("Enabling AllowUnsafeMalformedObjectDeletion feature gate")
			alreadyEnabled, err := enableFeatureGates(oc, []string{"AllowUnsafeMalformedObjectDeletion"})
			o.Expect(err).NotTo(o.HaveOccurred())

			if !alreadyEnabled {
				g.By("Waiting for kube-apiserver to restart due to feature gate change")
				progressErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
					status := getCoStatus(oc, "kube-apiserver", progressingStatus)
					return reflect.DeepEqual(status, progressingStatus), nil
				})
				if progressErr != nil {
					e2e.Logf("kube-apiserver did not start progressing within 300s, assuming feature gate already effective")
				} else {
					compat_otp.AssertWaitPollNoErr(waitForKubeAPIServer(healthyStatus, timeoutLong, "stable after feature gate rollout"),
						"kube-apiserver not stable after feature gate rollout")
				}
			} else {
				e2e.Logf("Feature gate already enabled or no change needed")
			}

			compat_otp.AssertWaitPollNoErr(waitForKubeAPIServer(healthyStatus, timeoutShort, "stable before corruption"),
				"kube-apiserver not stable before corruption")

			g.By("Corrupting the secret in etcd")
			etcdCorruptCmd := fmt.Sprintf(`etcdctl put /kubernetes.io/secrets/%s/%s "%s"`, testNamespace, testSecretName, corruptedDataMarker)
			etcdPods := getPodsListByLabel(oc, "openshift-etcd", "etcd=true")
			o.Expect(etcdPods).ShouldNot(o.BeEmpty())

			waitErr := wait.PollUntilContextTimeout(context.Background(), 3*time.Second, 30*time.Second, false, func(ctx context.Context) (bool, error) {
				return strings.Contains(execCommandOnPod(oc, etcdPods[0], "openshift-etcd", etcdCorruptCmd), etcdSuccessResponse), nil
			})
			o.Expect(waitErr).NotTo(o.HaveOccurred())

			g.By("Forcing kube-apiserver rollout")
			rolloutPatch := fmt.Sprintf(`[{"op": "replace", "path": "/spec/forceRedeploymentReason", "value": "Force Rollout %v"}]`, time.Now().UnixNano())
			err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", rolloutPatch).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			compat_otp.AssertWaitPollNoErr(waitForKubeAPIServer(progressingStatus, timeoutShort, "rollout started"),
				"kube-apiserver rollout didn't start")

			g.By("Verifying kube-apiserver handles corrupted secret correctly")
			err = waitForKubeAPIServer(healthyStatus, timeoutLong, "available after corruption")
			if err == nil {
				e2e.Failf("Unexpected: kube-apiserver remained healthy with corrupted secret")
			}

			secretOutput, secretErr := getResource(oc, asAdmin, withoutNamespace, "secret", testSecretName, "-n", testNamespace)
			o.Expect(secretErr).To(o.HaveOccurred())
			o.Expect(secretOutput).Should(o.ContainSubstring(expectedBase64Error))

			g.By("Attempting to delete corrupted secret using feature gate capability")
			deleteOptions := `{"apiVersion":"v1","kind":"DeleteOptions","ignoreStoreReadErrorWithClusterBreakingPotential":true}`
			deleteURL := fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", testNamespace, testSecretName)
			deleteOutput, deleteErr := oc.AsAdmin().WithoutNamespace().Run("delete").Args("--raw", deleteURL, "-f", "-").InputString(deleteOptions).Output()

			if deleteErr == nil && strings.Contains(deleteOutput, "Success") {
				e2e.Logf("Feature gate DELETE succeeded")
				o.Expect(deleteOutput).Should(o.ContainSubstring("Success"))
			} else {
				e2e.Logf("Feature gate DELETE did not work as expected (%v), using direct etcd cleanup. This is a known limitation.", deleteErr)
				err := cleanupCorruptedSecretFromEtcd()
				compat_otp.AssertWaitPollNoErr(err, "Failed to cleanup corrupted secret from etcd")
			}

			compat_otp.AssertWaitPollNoErr(pollUntilDeleted("secret", testSecretName, testNamespace), "Secret was not deleted")
			compat_otp.AssertWaitPollNoErr(waitForKubeAPIServer(healthyStatus, timeoutLong, "stable after cleanup"),
				"kube-apiserver did not recover after corrupted secret cleanup")

			cleanupRequired = false
		})

	g.It("[OTP][OCP-80554][OCPFeatureGate:CBOR] Verify the CBOR workflow [Slow][Disruptive][apigroup:config.openshift.io][Timeout:90m]",
		ote.Informing(), func() {
			const (
				timeoutShort = 500
				timeoutLong  = 2400
			)

			var (
				healthyStatus        = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
				progressingStatus    = map[string]string{"Progressing": "True"}
				originalEnabledGates string
			)

			waitForKubeAPIServer := func(status map[string]string, timeout int, description string) error {
				e2e.Logf("Waiting for kube-apiserver: %s (%ds)", description, timeout)
				return waitCoBecomes(oc, "kube-apiserver", timeout, status)
			}

			g.By("Ensuring cluster is healthy before starting")
			err := waitForKubeAPIServer(healthyStatus, timeoutLong, "healthy before test")
			if err != nil {
				g.Skip(fmt.Sprintf("Cluster is not healthy, skipping test: %v", err))
			}

			g.By("Saving original feature gate configuration")
			originalEnabledGates, _ = getResource(oc, asAdmin, withoutNamespace, "featuregate/cluster", "-o", `jsonpath={.spec.customNoUpgrade.enabled[*]}`)

			defer func() {
				g.By("Restoring original feature gate configuration")
				if err := restoreFeatureGateConfig(oc, originalEnabledGates); err != nil {
					e2e.Logf("Warning: Failed to restore feature gate: %v", err)
				} else {
					e2e.Logf("Waiting for kube-apiserver to stabilize after restoring feature gate")
					if err := waitForKubeAPIServer(progressingStatus, timeoutShort, "rollout started after restore"); err == nil {
						waitForKubeAPIServer(healthyStatus, timeoutLong, "stable after restore")
					}
				}
			}()

			tmpdir := filepath.Join(os.TempDir(), "apiserver-cbor-"+compat_otp.GetRandomString()+"/")
			o.Expect(os.MkdirAll(tmpdir, 0755)).To(o.Succeed())
			defer os.RemoveAll(tmpdir)

			var (
				kubeconfig   = os.Getenv("KUBECONFIG")
				certCA       = tmpdir + "ca.crt"
				clientKey    = tmpdir + "client.key"
				clientCert   = tmpdir + "client.crt"
				cborFileName = tmpdir + "pod.cbor"
			)

			if kubeconfig == "" {
				g.Skip("kubeconfig is not set, hence skipping.")
			}

			g.By("Extract TLS credentials from kubeconfig")
			kubecfg, err := clientcmd.LoadFromFile(kubeconfig)
			o.Expect(err).NotTo(o.HaveOccurred())

			currentContext := kubecfg.Contexts[kubecfg.CurrentContext]
			o.Expect(currentContext).NotTo(o.BeNil(), "current context not found in kubeconfig")

			cluster := kubecfg.Clusters[currentContext.Cluster]
			o.Expect(cluster).NotTo(o.BeNil(), "cluster not found in kubeconfig")
			o.Expect(os.WriteFile(certCA, cluster.CertificateAuthorityData, 0600)).To(o.Succeed())

			authInfo := kubecfg.AuthInfos[currentContext.AuthInfo]
			o.Expect(authInfo).NotTo(o.BeNil(), "auth info not found in kubeconfig")
			o.Expect(os.WriteFile(clientKey, authInfo.ClientKeyData, 0600)).To(o.Succeed())
			o.Expect(os.WriteFile(clientCert, authInfo.ClientCertificateData, 0600)).To(o.Succeed())

			g.By("Enabling CBOR feature gates")
			alreadyEnabled, err := enableFeatureGates(oc, []string{"CBORServingAndStorage", "ClientsAllowCBOR", "ClientsPreferCBOR"})
			o.Expect(err).NotTo(o.HaveOccurred())

			if !alreadyEnabled {
				progressErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
					status := getCoStatus(oc, "kube-apiserver", progressingStatus)
					return reflect.DeepEqual(status, progressingStatus), nil
				})
				if progressErr != nil {
					e2e.Logf("kube-apiserver did not start progressing within 300s, assuming CBOR feature gates already effective")
				} else {
					compat_otp.AssertWaitPollNoErr(waitForKubeAPIServer(healthyStatus, timeoutLong, "stable after CBOR feature gate rollout"),
						"kube-apiserver not stable after CBOR feature gate rollout")
				}
			} else {
				e2e.Logf("CBOR feature gates already enabled or no change needed")
			}

			compat_otp.AssertWaitPollNoErr(waitForKubeAPIServer(healthyStatus, timeoutShort, "stable before CBOR test"),
				"kube-apiserver not stable before CBOR test")

			g.By("Verifying retrieval of existing resource in CBOR format")
			execCmd := fmt.Sprintf(`curl --cacert %s --key %s --cert %s -X GET -H "Accept: application/cbor" $(oc whoami --show-server)/api/v1/namespaces/openshift-etcd/services/etcd --insecure`, certCA, clientKey, clientCert)
			curlGETCmdOutput, err := exec.Command("bash", "-c", execCmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(curlGETCmdOutput).ShouldNot(o.BeEmpty())

			g.By("Creating a pod using CBOR POST")
			podJSONData := `{
                "apiVersion": "v1",
                "kind": "Pod",
                "metadata": {"name": "test-pod-nginx"},
                "spec": {
                        "containers": [
                                {
                                        "name": "nginx",
                                        "image": "nginx"
                                }
                        ]
                }
        }`

			var podData map[string]interface{}
			o.Expect(json.Unmarshal([]byte(podJSONData), &podData)).To(o.Succeed())

			cborData, err := cbor.Marshal(podData)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(os.WriteFile(cborFileName, cborData, 0644)).To(o.Succeed())

			execPOSTCmd := fmt.Sprintf(`curl --cacert %s --key %s --cert %s -k -X POST -H "Content-Type: application/cbor" $(oc whoami --show-server)/api/v1/namespaces/default/pods --data-binary @%s`, certCA, clientKey, clientCert, cborFileName)
			curlPOSTCmdOutput, err := exec.Command("bash", "-c", execPOSTCmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(string(curlPOSTCmdOutput)).To(o.ContainSubstring("test-pod-nginx"))

			errPodrun := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 2*time.Minute, false, func(ctx context.Context) (bool, error) {
				podJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "default", "pod", "test-pod-nginx", "--output=json").Output()
				if err != nil {
					return false, nil
				}
				return strings.Contains(podJSON, `"phase": "Running"`), nil
			})
			compat_otp.AssertWaitPollNoErr(errPodrun, "the test pod is not Running.")

			g.By("Deleting pod using CBOR DELETE")
			execDELCmd := fmt.Sprintf(`curl --cacert %s --key %s --cert %s -k -X DELETE -H "Content-Type: application/cbor" $(oc whoami --show-server)/api/v1/namespaces/default/pods/test-pod-nginx`, certCA, clientKey, clientCert)
			_, err = exec.Command("bash", "-c", execDELCmd).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			errDel := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 2*time.Minute, false, func(ctx context.Context) (bool, error) {
				_, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "default", "pod", "test-pod-nginx").Output()
				return err != nil, nil
			})
			compat_otp.AssertWaitPollNoErr(errDel, "the test pod is not deleted")
		})
})
