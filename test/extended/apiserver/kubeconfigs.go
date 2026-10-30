package apiserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/apiserverauth"
	exutil "github.com/openshift/origin/test/extended/util"
)

var kcLocations = map[string]string{
	"localhost.kubeconfig":          "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig",
	"lb-ext.kubeconfig":             "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig",
	"lb-int.kubeconfig":             "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-int.kubeconfig",
	"localhost-recovery.kubeconfig": "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost-recovery.kubeconfig",
}
var kubeApiserverLocations = map[string]string{
	"check-endpoints.kubeconfig":    "/etc/kubernetes/static-pod-certs/configmaps/check-endpoints-kubeconfig/kubeconfig",
	"control-plane-node.kubeconfig": "/etc/kubernetes/static-pod-certs/configmaps/control-plane-node-kubeconfig/kubeconfig",
}

var _ = g.Describe("[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("apiserver", admissionapi.LevelPrivileged)

	for kubeconfig := range kcLocations {
		g.It(fmt.Sprintf("%q should be present on all masters and work", kubeconfig), func() {
			testKubeConfig(oc, kubeconfig, testNode)
		})
	}

	for kubeconfig := range kubeApiserverLocations {
		g.It(fmt.Sprintf("%q should be present in all kube-apiserver containers", kubeconfig), func() {
			// skip on microshift
			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				g.Skip("Not supported on Microshift")
			}
			testKubeConfig(oc, kubeconfig, testKubeApiserverContainer)
		})
	}
})

func testKubeConfig(oc *exutil.CLI, kubeconfig string, testFn func(oc *exutil.CLI, kubeconfig, masterName string) error) {
	// external controlplane topology doesn't have master nodes
	controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	if *controlPlaneTopology == configv1.ExternalTopologyMode {
		g.Skip("ExternalControlPlaneTopology doesn't have master node kubeconfigs")
	}

	masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: `node-role.kubernetes.io/master`,
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Discovered %d master nodes.", len(masterNodes.Items))
	o.Expect(masterNodes.Items).NotTo(o.HaveLen(0))
	for _, master := range masterNodes.Items {
		err := retry.OnError(
			wait.Backoff{
				Duration: 2 * time.Second,
				Steps:    3,
				Factor:   5.0,
				Jitter:   0.1,
			},
			func(err error) bool {
				// retry error when kube-apiserver was temporarily unavailable, this matches oc error coming from:
				// https://github.com/kubernetes/kubernetes/blob/cbb5ea8210596ada1efce7e7a271ca4217ae598e/staging/src/k8s.io/kubectl/pkg/cmd/util/helpers.go#L237-L243
				matched, _ := regexp.MatchString("The connection to the server .+ was refused - did you specify the right host or port", err.Error())
				return !matched
			},
			func() error {
				return testFn(oc, kubeconfig, master.Name)
			})
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func testNode(oc *exutil.CLI, kubeconfig, masterName string) error {
	g.By("Testing master node " + masterName)
	kubeconfigPath, ok := kcLocations[kubeconfig]
	if !ok {
		return fmt.Errorf("location for %s kubeconfig not found", kubeconfig)
	}
	framework.Logf("Verifying kubeconfig %q on master %q", kubeconfig, masterName)
	out, err := oc.AsAdmin().Run("debug").Args("node/"+masterName, "--", "chroot", "/host", "/bin/bash", "-euxo", "pipefail", "-c",
		fmt.Sprintf(`oc --kubeconfig "%s" get namespace kube-system`, kubeconfigPath)).Output()
	framework.Logf("%s", out)
	if err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}

func testKubeApiserverContainer(oc *exutil.CLI, kubeconfig, masterName string) error {
	g.By("Testing kube-apiserver container on master node " + masterName)
	kubeconfigPath, ok := kubeApiserverLocations[kubeconfig]
	if !ok {
		return fmt.Errorf("location for %s kubeconfig not found", kubeconfig)
	}

	framework.Logf("Verifying kubeconfig %q in kube-apiserver container in master %q", kubeconfig, masterName)
	// Use curl to verify the kubeconfig is present and functional, extracting
	// cert paths directly from the kubeconfig file. This avoids copying the oc
	// binary from the host into the container, which fails when the host OS
	// (e.g., RHCOS 10 with glibc 2.38) has a newer glibc than the container.
	out, err := oc.AsAdmin().Run("exec").Args("-n", "openshift-kube-apiserver", "kube-apiserver-"+masterName, "--", "/bin/bash", "-euxo", "pipefail", "-c",
		fmt.Sprintf(`for k in server client-certificate client-key certificate-authority; do
  count=$(grep -Ec "^[[:space:]]*${k}:[[:space:]]+" "%[1]s")
  [ "$count" -eq 1 ] || { echo "expected exactly one ${k} in %[1]s, got $count" >&2; exit 1; }
done
server=$(grep '^[[:space:]]*server:' "%[1]s" | head -1 | awk '{print $2}')
cert=$(grep '^[[:space:]]*client-certificate:' "%[1]s" | head -1 | awk '{print $2}')
key=$(grep '^[[:space:]]*client-key:' "%[1]s" | head -1 | awk '{print $2}')
ca=$(grep '^[[:space:]]*certificate-authority:' "%[1]s" | head -1 | awk '{print $2}')
for v in server cert key ca; do
  eval "val=\$$v"
  [ -n "$val" ] || { echo "${v} field extraction failed from %[1]s" >&2; exit 1; }
done
curl -Ssf --connect-timeout 10 --max-time 45 --cert "$cert" --key "$key" --cacert "$ca" "${server}/api?timeout=32s"`, kubeconfigPath)).Output()
	framework.Logf("%s", out)
	if err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}

var _ = g.Describe("[Conformance][sig-api-machinery][Feature:APIServer] admin kubeconfig replacement", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("kubeconfig-replace", admissionapi.LevelPrivileged)

	g.It("[OTP][OCP-41899] should support replacing the admin kubeconfig generated at install time [APIServer_Disruptive][Disruptive][Slow][apigroup:config.openshift.io]", ote.Informing(), func() {
		var (
			dirname        = "/tmp/-OCP-41899-ca/"
			name           = dirname + "custom"
			validity       = 3650
			caSubj         = dirname + "/OU=openshift/CN=admin-kubeconfig-signer-custom"
			user           = "system:admin"
			userCert       = dirname + "system-admin"
			group          = "system:masters"
			userSubj       = dirname + "/O=" + group + "/CN=" + user
			newKubeconfig  = dirname + "kubeconfig." + user
			patch          = `[{"op": "add", "path": "/spec/clientCA", "value":{"name":"client-ca-custom"}}]`
			patchToRecover = `[{"op": "replace", "path": "/spec/clientCA", "value":}]`
			configmapBkp   = dirname + "OCP-41899-bkp.yaml"
		)

		defer os.RemoveAll(dirname)
		defer func() {
			g.By("Restoring cluster")
			output, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
			if strings.Contains(string(output), "Unauthorized") {
				err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("--kubeconfig", newKubeconfig, "-f", configmapBkp).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 100*time.Second, false, func(cxt context.Context) (bool, error) {
					output, _ := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
					if output == "system:admin" {
						e2e.Logf("Old kubeconfig is restored : %s", output)
						// Adding wait time to ensure old kubeconfig restored properly
						time.Sleep(60 * time.Second)
						return true, nil
					} else if output == "error: You must be logged in to the server (Unauthorized)" {
						return false, nil
					}
					return false, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "Old kubeconfig is not restored")
				apiserverauth.RestoreClusterOcp41899(oc)
				e2e.Logf("Cluster recovered")
			} else if err == nil {
				output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patchToRecover).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Contains(output, "patched (no change)") {
					e2e.Logf("Apiserver/cluster is not changed from the default values")
					apiserverauth.RestoreClusterOcp41899(oc)
				} else {
					err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patchToRecover).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					apiserverauth.RestoreClusterOcp41899(oc)
				}
			}
		}()

		// Taking backup of configmap "admin-kubeconfig-client-ca" to restore old kubeconfig
		err := os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Get the default CA backup")
		configmapBkp, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", "admin-kubeconfig-client-ca", "-n", "openshift-config", "-o", "yaml").OutputToFile("OCP-41899-bkp.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		sedCmd := fmt.Sprintf(`sed -i '/creationTimestamp:\|resourceVersion:\|uid:/d' %s`, configmapBkp)
		_, err = exec.Command("bash", "-c", sedCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Generation of a new self-signed CA, in case a corporate or another CA is already existing can be used.
		g.By("Generation of a new self-signed CA")
		e2e.Logf("Generate the CA private key")
		opensslCmd := fmt.Sprintf(`openssl genrsa -out %s-ca.key 4096`, name)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create the CA certificate")
		opensslCmd = fmt.Sprintf(`openssl req -x509 -new -nodes -key %s-ca.key -sha256 -days %d -out %s-ca.crt -subj %s`, name, validity, name, caSubj)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Generation of a new system:admin certificate. The client certificate must have the user into the x.509 subject CN field and the group into the O field.
		g.By("Generation of a new system:admin certificate")
		e2e.Logf("Create the user CSR")
		opensslCmd = fmt.Sprintf(`openssl req -nodes -newkey rsa:2048 -keyout %s.key -subj %s -out %s.csr`, userCert, userSubj, userCert)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// sign the user CSR and generate the certificate, the certificate must have the `clientAuth` extension
		e2e.Logf("Sign the user CSR and generate the certificate")
		opensslCmd = fmt.Sprintf(`openssl x509 -extfile <(printf "extendedKeyUsage = clientAuth") -req -in %s.csr -CA %s-ca.crt -CAkey %s-ca.key -CAcreateserial -out %s.crt -days %d -sha256`, userCert, name, name, userCert, validity)
		_, err = exec.Command("bash", "-c", opensslCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// In order to have a safe replacement, before removing the default CA the new certificate is added as an additional clientCA.
		g.By("Create the client-ca ConfigMap")
		caFile := fmt.Sprintf(`--from-file=ca-bundle.crt=%s-ca.crt`, name)
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("configmap", "client-ca-custom", "-n", "openshift-config", caFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Patching apiserver")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("apiserver/cluster", "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Checking openshift-controller-manager operator should be in Progressing in 150 seconds")
		expectedStatus := map[string]string{"Progressing": "True"}
		// Increasing wait time for prow ci failures
		apiserverauth.WaitCoBecomes(oc, "openshift-controller-manager", 150, expectedStatus) // Wait it to become Progressing=True
		e2e.Logf("Checking openshift-controller-manager operator should be Available in 500 seconds")
		expectedStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		err = apiserverauth.WaitCoBecomes(oc, "openshift-controller-manager", 500, expectedStatus) // Wait it to become Available=True and Progressing=False and Degraded=False
		o.Expect(err).NotTo(o.HaveOccurred(), "openshift-controller-manager operator is not available in 500 seconds")

		g.By("Create the new kubeconfig")
		e2e.Logf("Add system:admin credentials, context to the kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-credentials", user, "--client-certificate="+userCert+".crt", "--client-key="+userCert+".key", "--embed-certs", "--kubeconfig="+newKubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create context for the user")
		clusterName, _ := oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "-o", `jsonpath={.clusters[0].name}`).Output()
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-context", user, "--cluster="+clusterName, "--namespace=default", "--user="+user, "--kubeconfig="+newKubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Extract certificate authority")
		podnames, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-authentication", "-o", "name").Output()
		podname := strings.Fields(podnames)
		ingressCrt, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", "openshift-authentication", podname[0], "cat", "/run/secrets/kubernetes.io/serviceaccount/ca.crt").OutputToFile("OCP-41899-ingress-ca.crt")
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Set certificate authority data")
		serverName, _ := oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "-o", `jsonpath={.clusters[0].cluster.server}`).Output()
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("set-cluster", clusterName, "--server="+serverName, "--certificate-authority="+ingressCrt, "--kubeconfig="+newKubeconfig, "--embed-certs").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Set current context")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("use-context", user, "--kubeconfig="+newKubeconfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Test the new kubeconfig, be aware that the following command may requires some seconds for let the operator reconcile the newly added CA.
		g.By("Testing the new kubeconfig")
		errUser := oc.AsAdmin().WithoutNamespace().Run("login").Args("--kubeconfig", newKubeconfig, "-u", user).NotShowInfo().Execute()
		if errUser != nil {
			if exitErr, ok := errUser.(*exutil.ExitError); ok {
				e2e.Failf("oc login command failed for user %s. Stderr: %s", user, exitErr.StdErr)
			} else {
				e2e.Failf("oc login command failed for user %s with a non-ExitError: %v", user, errUser)
			}
		}
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("--kubeconfig", newKubeconfig, "node").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// If the previous commands are successful is possible to replace the default CA.
		e2e.Logf("Replace the default CA")
		configmapYaml, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("--kubeconfig", newKubeconfig, "configmap", "admin-kubeconfig-client-ca", "-n", "openshift-config", caFile, "--dry-run=client", "-o", "yaml").OutputToFile("OCP-41899.yaml")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().WithoutNamespace().Run("replace").Args("--kubeconfig", newKubeconfig, "-f", configmapYaml).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Is now possible to remove the additional CA which we set earlier.
		e2e.Logf("Removing the additional CA")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("--kubeconfig", newKubeconfig, "apiserver/cluster", "--type=json", "-p", patchToRecover).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Now the old kubeconfig should be invalid, the following command is expected to fail (make sure to set the proper kubeconfig path).
		e2e.Logf("Testing old kubeconfig")
		err = oc.AsAdmin().WithoutNamespace().Run("config").Args("use-context", "admin").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 100*time.Second, false, func(cxt context.Context) (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("whoami").Args("").Output()
			if strings.Contains(string(output), "Unauthorized") {
				e2e.Logf("Test pass: Old kubeconfig not working!")
				// Adding wait time to ensure new kubeconfig work properly
				time.Sleep(60 * time.Second)
				return true, nil
			} else if err == nil {
				e2e.Logf("Still Old kubeconfig is working!")
				return false, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Test failed: Old kubeconfig is working!")
	})
})
