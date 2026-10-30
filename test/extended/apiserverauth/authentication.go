package apiserverauth

import (
	"path/filepath"
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Cases ported from openshift-tests-private/test/extended/apiserverauth/authentication.go.
// Only the highest-priority cases with no e2e coverage elsewhere are ported here.
var _ = g.Describe("[sig-auth] Authentication", func() {
	defer g.GinkgoRecover()

	var oc = compat_otp.NewCLIWithoutNamespace("authentication")

	// OCP-10662 with the nonroot-v2 SCC assigned to the user, pods must not be able to run as root
	g.It("[Skipped:Disconnected] cannot run process via user root in the container when using MustRunAsNonRoot as the RunAsUserStrategy", func() {
		// ToDo: if we can implement multiple users in external OIDC clusters in future, undo the skip.
		isExternalOIDCCluster, err := compat_otp.IsExternalOIDCCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to determine whether the cluster uses an external OIDC provider")
		if isExternalOIDCCluster {
			g.Skip("Skipping the test as we are running against an external OIDC cluster.")
		}

		oc.SetupProject()
		namespace := oc.Namespace()
		username := oc.Username()

		// "nonroot-v2" SCC has "MustRunAsNonRoot" as the RunAsUserStrategy, assigning it to the user
		err = oc.AsAdmin().Run("adm", "policy").Args("add-scc-to-user", "nonroot-v2", username).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to assign the nonroot-v2 SCC to the user")
		// Register the cleanup immediately after granting so the SCC is removed even if a later step fails.
		defer func() {
			err := oc.AsAdmin().Run("adm", "policy").Args("remove-scc-from-user", "nonroot-v2", username).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to remove the nonroot-v2 SCC from the user during cleanup")
		}()

		baseDir := compat_otp.FixturePath("testdata", "apiserverauth")
		podTemplate := filepath.Join(baseDir, "pod-scc-runAsUser.yaml")

		g.By("Create new pod with runAsUser=0")
		image := "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83"
		err = compat_otp.ApplyResourceFromTemplateWithNonAdminUser(oc, "--ignore-unknown-parameters=true", "-f", podTemplate, "-p", "NAME="+"pod10662-1", "NAMESPACE="+namespace, "IMAGE="+image, "USERID=0")
		o.Expect(err).Should(o.HaveOccurred(), "creating a pod with runAsUser=0 must be rejected by the nonroot-v2 SCC")

		g.By("Create new pod without runAsUser specified and the pod's image is built to run as USER 0 by default")
		// Below image is built to run as USER 0 by default. This can be checked either by "podman inspect <image> | grep User" or "podman run <image> whoami"
		err = oc.Run("run").Args("pod10662-3", "--image", image, `--overrides={"spec":{"securityContext":{"fsGroup":1001}}}`).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create the pod without an explicit runAsUser")

		g.By("Check status of pod without runAsUser specified and the pod's image is built to run as USER 0 by default")
		err = wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
			output, err := oc.Run("describe").Args("pod/pod10662-3").Output()
			if err != nil {
				e2e.Logf("Pod status is not available yet. Trying again")
				return false, nil
			}
			if matched, _ := regexp.MatchString("container has runAsNonRoot and image will run as root", output); matched {
				e2e.Logf(`Got expected output "container has runAsNonRoot and image will run as root"`)
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Timed out to get expected error message")

		g.By("Create new pod with runAsUser != 0")
		pod1 := compat_otp.Pod{Name: "pod10662-2", Namespace: namespace, Template: podTemplate, Parameters: []string{"IMAGE=" + image, "USERID=97777", "NAMESPACE=" + namespace}}
		pod1.Create(oc)
		g.By("Check status of pod with runAsUser != 0")
		output, err := oc.Run("get").Args("pod/pod10662-2", "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get the phase of the pod created with a non-zero runAsUser")
		o.Expect(output).To(o.ContainSubstring("Running"), "the pod created with a non-zero runAsUser should be running")
	})

	// OCP-47941 ephemeral containers that violate the service account's SCC must be rejected
	g.It("[Skipped:Disconnected] should not be allowed to create an ephemeral container that violates the SCC without required privileges", func() {
		// ToDo: if we can implement multiple users in external OIDC clusters in future, undo the skip.
		isExternalOIDCCluster, err := compat_otp.IsExternalOIDCCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to determine whether the cluster uses an external OIDC provider")
		if isExternalOIDCCluster {
			g.Skip("Skipping the test as we are running against an external OIDC cluster.")
		}

		g.By("1. Create a namespace as normal user")
		oc.SetupProject()
		testNamespace := oc.Namespace()
		username := oc.Username()

		g.By("2. Changing the pod security profile to privileged")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", testNamespace, "security.openshift.io/scc.podSecurityLabelSync=false", "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "pod-security.kubernetes.io/warn=privileged", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Adding label to namespace failed")

		g.By("3. Creating new role for ephemeral containers")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("role", "role-ephemeralcontainers", "--verb=get,list,watch,update,patch", "--resource=pods/ephemeralcontainers", "-n", testNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "role role-ephemeralcontainers creation failed")

		g.By("4. Adding role to the user")
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-role-to-user", "role-ephemeralcontainers", username, "--role-namespace", testNamespace, "-n", testNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "policy addition of role-ephemeralcontainers to testuser failed")

		g.By("5. Running the hello-openshift image")
		image := "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83"
		output, err := oc.Run("run").Args("-n", testNamespace, "hello-openshift", "--image="+image).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create the hello-openshift pod")
		o.Expect(output).NotTo(o.ContainSubstring("Warning"), "creating the hello-openshift pod should not emit warnings")
		e2e.Logf("Waiting for hello-openshift pod to be ready ...")
		compat_otp.AssertAllPodsToBeReadyWithPollerParams(oc, testNamespace, 30*time.Second, 10*time.Minute)

		// The ephemeral container is privileged and runs as UID 0, which violates the
		// restricted-v2 SCC held by the default service account.
		baseDir := compat_otp.FixturePath("testdata", "apiserverauth")
		podJson := filepath.Join(baseDir, "sample-pod-ephemeral-container-complex.json")

		//It should fail, currently there is a bug (https://issues.redhat.com/browse/OCPBUGS-7181) associated with it
		err = oc.Run("replace").Args("--raw", "/api/v1/namespaces/"+testNamespace+"/pods/hello-openshift/ephemeralcontainers", "-f", podJson).Execute()
		o.Expect(err).To(o.HaveOccurred(), "Addition of ephemeral containers without required privileges must fail")

		g.By("6. Adding scc to the user")
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-scc-to-user", "privileged", "-z", "default", "-n", testNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "policy addition of scc as privileged in the namespace failed")
		// Register the cleanup immediately after granting so the SCC is removed even if a later step fails.
		// Step 7 below revokes the SCC as part of the test flow; the flag keeps this safety net
		// from removing the SCC a second time on a passing run.
		sccRevoked := false
		defer func() {
			if sccRevoked {
				return
			}
			err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-scc-from-user", "privileged", "-z", "default", "-n", testNamespace).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to remove the privileged SCC from the default service account during cleanup")
		}()

		err = oc.Run("replace").Args("--raw", "/api/v1/namespaces/"+testNamespace+"/pods/hello-openshift/ephemeralcontainers", "-f", podJson).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Addition of ephemeral containers failed")
		// It needs more time to wait for Ephemeral Container to come to Running state, so the Poll parameters are larger
		err = wait.Poll(30*time.Second, 300*time.Second, func() (bool, error) {
			output, err := oc.Run("get").Args("po", "-o", "jsonpath={.items[*].status.ephemeralContainerStatuses}").Output()
			if err != nil {
				e2e.Logf("Ephemeral container status is not available yet. Trying again")
				return false, nil
			}
			if matched, _ := regexp.MatchString("running", output); matched {
				e2e.Logf("Ephemeral Container is in Running state")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "Failed to check the ephemeral container status")

		output, err = oc.Run("rsh").Args("-n", testNamespace, "-c", "ephemeral-pod-debugger", "hello-openshift", "ps", "-eF").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to rsh into the ephemeral container")
		o.Expect(output).To(o.ContainSubstring("sleep 360d"), "the ephemeral container process should be running")
		o.Expect(output).To(o.ContainSubstring("/hello_openshift"), "the target container process should be visible from the ephemeral container")

		output, err = oc.Run("logs").Args("-n", testNamespace, "-c", "ephemeral-pod-debugger", "hello-openshift").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "logging of container ephemeral-pod-debugger failed")
		o.Expect(output).To(o.ContainSubstring("root"), "the ephemeral container should run as root")

		g.By("7. Removing scc from the user")
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "remove-scc-from-user", "privileged", "-z", "default", "-n", testNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "policy removal of scc as privileged in the namespace failed")
		sccRevoked = true

		err = oc.Run("rsh").Args("-n", testNamespace, "-c", "ephemeral-pod-debugger", "hello-openshift").Execute()
		o.Expect(err).To(o.HaveOccurred(), "rsh into ephemeral containers without required privileges should fail")
	})

	// OCP-52452 payload namespaces must keep their privileged pod-security labels even when opted out of the SCC label sync.
	g.It("[Serial] payload namespaces do not respect pod security admission autolabel's opt-in/opt-out", func() {
		// The openshift-monitoring namespace does not exist on MicroShift clusters.
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to determine whether the cluster is a MicroShift cluster")
		if isMicroShift {
			g.Skip("Skipping the test because the openshift-monitoring namespace is not available on MicroShift clusters.")
		}

		g.By("1. Check the project labels for payload namespace openshift-monitoring")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("project", "openshift-monitoring", "-o=jsonpath={.metadata.labels}").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get the labels of the openshift-monitoring namespace")
		o.Expect(output).To(o.ContainSubstring("\"pod-security.kubernetes.io/audit\":\"privileged\""), "the audit label should be privileged")
		o.Expect(output).To(o.ContainSubstring("\"pod-security.kubernetes.io/enforce\":\"privileged\""), "the enforce label should be privileged")
		o.Expect(output).To(o.ContainSubstring("\"pod-security.kubernetes.io/warn\":\"privileged\""), "the warn label should be privileged")

		g.By("2. Opt out of the autolabelling explicitly")
		// Capture the original label values so they can be restored when the test ends.
		originalSyncLabel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", "openshift-monitoring", `-o=jsonpath={.metadata.labels.security\.openshift\.io/scc\.podSecurityLabelSync}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read the original pod security label sync value of openshift-monitoring")
		originalWarnLabel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", "openshift-monitoring", `-o=jsonpath={.metadata.labels.pod-security\.kubernetes\.io/warn}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read the original pod security warn label of openshift-monitoring")
		defer func() {
			var err error
			if originalSyncLabel == "" {
				// the label was originally absent, remove it
				err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", "openshift-monitoring", "security.openshift.io/scc.podSecurityLabelSync-").Execute()
			} else {
				err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", "openshift-monitoring", "security.openshift.io/scc.podSecurityLabelSync="+originalSyncLabel, "--overwrite").Execute()
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore the original pod security label sync value of openshift-monitoring")
			if originalWarnLabel == "" {
				// the label was originally absent, remove it
				err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", "openshift-monitoring", "pod-security.kubernetes.io/warn-").Execute()
			} else {
				err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", "openshift-monitoring", "pod-security.kubernetes.io/warn="+originalWarnLabel, "--overwrite").Execute()
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore the original pod security warn label of openshift-monitoring")
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", "openshift-monitoring", "security.openshift.io/scc.podSecurityLabelSync=false", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Opt out of the autolabelling in openshift-monitoring namespace failed")

		g.By("3. Change any one label's value")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", "openshift-monitoring", "pod-security.kubernetes.io/warn=restricted", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to change the pod security warn label of openshift-monitoring")

		g.By("4. Wait for some time till the project labels for openshift-monitoring namespace are restored to original values")
		err = wait.Poll(15*time.Second, 5*time.Minute, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("project", "openshift-monitoring", "-o=jsonpath={.metadata.labels}").Output()
			if err != nil {
				e2e.Logf("Failed to get the project labels of the openshift-monitoring namespace. Trying again")
				return false, nil
			}
			matched1, _ := regexp.MatchString("\"pod-security.kubernetes.io/audit\":\"privileged\"", output)
			matched2, _ := regexp.MatchString("\"pod-security.kubernetes.io/enforce\":\"privileged\"", output)
			matched3, _ := regexp.MatchString("\"pod-security.kubernetes.io/warn\":\"privileged\"", output)
			if matched1 && matched2 && matched3 {
				e2e.Logf("Changes of pod-security.kubernetes.io labels are auto reverted to original values\n")
				return true, nil
			} else {
				e2e.Logf("Changes of pod-security.kubernetes.io labels are not auto reverted to original values yet\n")
				return false, nil
			}
		})
		compat_otp.AssertWaitPollNoErr(err, "Timed out to check whether the pod-security.kubernetes.io labels of openshift-monitoring namespace are restored or not")
	})
})
