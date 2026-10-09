package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery] [Jira:apiserver-auth] Audit Logging and Configuration", func() {
	oc := exutil.NewCLIWithoutNamespace("apiserver-audit")

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Audit logging tests are not supported on MicroShift")
		}
	})

	// waitForAPIServerRollout waits for kube-apiserver to complete rollout after configuration changes
	waitForAPIServerRollout := func() {
		err := waitCoBecomes(oc, "kube-apiserver", 100, map[string]string{"Progressing": "True"})
		compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator did not start progressing in 100 seconds")
		err = waitCoBecomes(oc, "kube-apiserver", 1500, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"})
		compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator rollout not completed")
	}

	// OCP-33427: Customize audit config of apiservers [Serial]
	g.It("[OTP][OCP-33427] Customize audit config of apiservers [APIServer_Disruptive][Serial][Disruptive]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit profile")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalProfile := apiServer.Spec.Audit.Profile
		framework.Logf("Current audit profile: %s", originalProfile)

		defer func() {
			g.By("Cleanup: Revert to original audit profile")
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// Get current profile before cleanup
			currentServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
				cleanupCtx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			patch := fmt.Sprintf(`{"spec":{"audit":{"profile":%q}}}`, originalProfile)
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Patch(
				cleanupCtx, "cluster", types.MergePatchType, []byte(patch), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Only wait for rollout if profile actually changed
			if currentServer.Spec.Audit.Profile != originalProfile {
				waitForAPIServerRollout()
			}
		}()

		g.By("2) Update audit profile to AllRequestBodies")
		patch := fmt.Sprintf(`{"spec":{"audit":{"profile":%q}}}`, configv1.AllRequestBodiesAuditProfileType)
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster",
			"--type=merge",
			"--patch="+patch,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("2.1) Wait for API server rollout to complete")
		// Wait for rollout since we're changing to a different profile
		waitForAPIServerRollout()

		g.By("3) Verify audit profile was updated")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(apiServer.Spec.Audit.Profile).To(o.Equal(configv1.AllRequestBodiesAuditProfileType))
	})

	// OCP-43261: APIServer Support None audit policy [Serial]
	g.It("[OTP][OCP-43261] APIServer Support None audit policy [APIServer_Disruptive][Serial][Disruptive]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit profile")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalProfile := apiServer.Spec.Audit.Profile
		framework.Logf("Current audit profile: %s", originalProfile)

		defer func() {
			g.By("Cleanup: Revert to original audit profile")
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// Get current profile before cleanup
			currentServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
				cleanupCtx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			patch := fmt.Sprintf(`{"spec":{"audit":{"profile":%q}}}`, originalProfile)
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Patch(
				cleanupCtx, "cluster", types.MergePatchType, []byte(patch), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Only wait for rollout if profile actually changed
			if currentServer.Spec.Audit.Profile != originalProfile {
				waitForAPIServerRollout()
			}
		}()

		g.By("2) Set audit profile to None")
		patch := fmt.Sprintf(`{"spec":{"audit":{"profile":%q}}}`, configv1.NoneAuditProfileType)
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster",
			"--type=merge",
			"--patch="+patch,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("2.1) Wait for API server rollout to complete")
		// Wait for rollout since we're changing to a different profile
		waitForAPIServerRollout()

		g.By("3) Verify audit profile is set to None")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(apiServer.Spec.Audit.Profile).To(o.Equal(configv1.NoneAuditProfileType))
	})

	// OCP-43336: Support customRules list for by-group profiles [Serial]
	g.It("[OTP][OCP-43336] Support customRules list for by-group profiles [APIServer_Disruptive][Serial][Disruptive]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit config")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if apiServer.Spec.Audit.Profile == configv1.NoneAuditProfileType {
			g.Skip("Custom audit rules are not evaluated when the top-level audit profile is None")
		}
		originalCustomRules := apiServer.Spec.Audit.CustomRules
		framework.Logf("Current custom rules: %v", originalCustomRules)

		defer func() {
			g.By("Cleanup: Revert to original custom rules")
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// Get current custom rules before cleanup
			currentServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
				cleanupCtx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			var patch string
			if len(originalCustomRules) > 0 {
				rulesJSON, err := json.Marshal(originalCustomRules)
				o.Expect(err).NotTo(o.HaveOccurred())
				patch = fmt.Sprintf(`{"spec":{"audit":{"customRules":%s}}}`, string(rulesJSON))
			} else {
				patch = `{"spec":{"audit":{"customRules":null}}}`
			}
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Patch(
				cleanupCtx, "cluster", types.MergePatchType, []byte(patch), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Only wait for rollout if custom rules actually changed
			rulesChanged := len(currentServer.Spec.Audit.CustomRules) != len(originalCustomRules)
			if !rulesChanged && len(originalCustomRules) > 0 {
				// Deep compare if lengths match and not both empty
				originalJSON, _ := json.Marshal(originalCustomRules)
				currentJSON, _ := json.Marshal(currentServer.Spec.Audit.CustomRules)
				rulesChanged = string(originalJSON) != string(currentJSON)
			}
			if rulesChanged {
				waitForAPIServerRollout()
			}
		}()

		g.By("2) Add custom rule for system:authenticated group")
		customRule := fmt.Sprintf(`{"spec":{"audit":{"customRules":[{"group":"system:authenticated","profile":"Default"}]}}}`)
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster",
			"--type=merge",
			"--patch="+customRule,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("2.1) Wait for API server rollout to complete")
		// Wait for rollout since we're adding custom rules
		waitForAPIServerRollout()

		g.By("3) Verify custom rule was added")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(apiServer.Spec.Audit.CustomRules).To(o.HaveLen(1))
		o.Expect(apiServer.Spec.Audit.CustomRules[0].Group).To(o.Equal("system:authenticated"))
		o.Expect(apiServer.Spec.Audit.CustomRules[0].Profile).To(o.Equal(configv1.DefaultAuditProfileType))
	})

	// OCP-73410: Support customRules list for by-group with none profile [Serial]
	g.It("[OTP][OCP-73410] Support customRules list for by-group with none profile [APIServer_Disruptive][Serial][Disruptive]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit config")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if apiServer.Spec.Audit.Profile == configv1.NoneAuditProfileType {
			g.Skip("Custom audit rules are not evaluated when the top-level audit profile is None")
		}
		originalCustomRules := apiServer.Spec.Audit.CustomRules
		framework.Logf("Current custom rules: %v", originalCustomRules)

		defer func() {
			g.By("Cleanup: Revert to original custom rules")
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			// Get current custom rules before cleanup
			currentServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
				cleanupCtx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			var patch string
			if len(originalCustomRules) > 0 {
				rulesJSON, err := json.Marshal(originalCustomRules)
				o.Expect(err).NotTo(o.HaveOccurred())
				patch = fmt.Sprintf(`{"spec":{"audit":{"customRules":%s}}}`, string(rulesJSON))
			} else {
				patch = `{"spec":{"audit":{"customRules":null}}}`
			}
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Patch(
				cleanupCtx, "cluster", types.MergePatchType, []byte(patch), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Only wait for rollout if custom rules actually changed
			rulesChanged := len(currentServer.Spec.Audit.CustomRules) != len(originalCustomRules)
			if !rulesChanged && len(originalCustomRules) > 0 {
				// Deep compare if lengths match and not both empty
				originalJSON, _ := json.Marshal(originalCustomRules)
				currentJSON, _ := json.Marshal(currentServer.Spec.Audit.CustomRules)
				rulesChanged = string(originalJSON) != string(currentJSON)
			}
			if rulesChanged {
				waitForAPIServerRollout()
			}
		}()

		g.By("2) Add custom rule with None profile for specific group")
		customRule := fmt.Sprintf(`{"spec":{"audit":{"customRules":[{"group":"system:serviceaccounts","profile":"None"}]}}}`)
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster",
			"--type=merge",
			"--patch="+customRule,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))

		g.By("2.1) Wait for API server rollout to complete")
		// Wait for rollout since we're adding custom rules
		waitForAPIServerRollout()

		g.By("3) Verify custom rule with None profile was added")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(apiServer.Spec.Audit.CustomRules).To(o.HaveLen(1))
		o.Expect(apiServer.Spec.Audit.CustomRules[0].Group).To(o.Equal("system:serviceaccounts"))
		o.Expect(apiServer.Spec.Audit.CustomRules[0].Profile).To(o.Equal(configv1.NoneAuditProfileType))
	})

	// OCP-68629: Audit log files should not have too permissive mode
	g.It("[OTP][OCP-68629] Audit log files should not have too permissive mode", ote.Informing(), func(ctx g.SpecContext) {
		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("HyperShift hosts the control plane externally; master node logs are not accessible via node-logs")
		}

		g.By("1) Get list of master nodes")
		masters, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(
			ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masters.Items).NotTo(o.BeEmpty(), "expected at least one master node")

		g.By("2) Check audit log file permissions on master nodes")
		for _, master := range masters.Items {
			framework.Logf("Checking audit log file permissions on a master node")
			output, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(
				"-n", "default",
				"node/"+master.Name,
				"--",
				"chroot", "/host", "stat", "-c", "%a", "/var/log/kube-apiserver/audit.log",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred(),
				"failed to check kube-apiserver audit log permissions")

			modes := regexp.MustCompile(`(?m)^[0-7]{3,4}$`).FindAllString(output, -1)
			o.Expect(modes).NotTo(o.BeEmpty(), "expected octal file mode output from stat command")
			trimmed := modes[len(modes)-1]

			framework.Logf("Audit log file mode: %s", trimmed)

			// Parse the octal permission mode and verify it's no more permissive than 600
			mode, parseErr := strconv.ParseInt(trimmed, 8, 32)
			o.Expect(parseErr).NotTo(o.HaveOccurred(), "failed to parse octal file mode")

			// Check that mode is no more permissive than 0600
			// 0600 = owner read/write only, no group or other permissions
			// Valid modes: 0600, 0400, 0200, 0000 (owner has at most read+write, no group/other)
			// Invalid: 0644, 0666, 0777 (group or other has permissions), 0700 (owner has execute)
			// Mask 0177 covers owner-execute (0100), group permissions (0070), and other permissions (0007)
			o.Expect(mode&0177).To(o.Equal(int64(0)),
				"audit log file permissions should have no owner-execute, group, or other permissions (mode: %s)", trimmed)
			o.Expect(mode).To(o.BeNumerically("<=", 0600),
				"audit log file permissions should be no more permissive than 0600 (mode: %s)", trimmed)
		}
	})
})
