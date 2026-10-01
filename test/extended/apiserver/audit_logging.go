package apiserver

import (
	"encoding/json"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery] [Jira:apiserver-auth] Audit Logging and Configuration", func() {
	oc := exutil.NewCLIWithoutNamespace("apiserver-audit")

	// OCP-33427: Customize audit config of apiservers [Serial]
	g.It("[OTP][OCP-33427] Customize audit config of apiservers [Serial]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit profile")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalProfile := apiServer.Spec.Audit.Profile
		framework.Logf("Current audit profile: %s", originalProfile)

		defer func() {
			g.By("Cleanup: Revert to original audit profile")
			patch := fmt.Sprintf(`{"spec":{"audit":{"profile":%q}}}`, originalProfile)
			_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver", "cluster",
				"--type=merge",
				"--patch="+patch,
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
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

		g.By("3) Verify audit profile was updated")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(apiServer.Spec.Audit.Profile).To(o.Equal(configv1.AllRequestBodiesAuditProfileType))
	})

	// OCP-43261: APIServer Support None audit policy [Serial]
	g.It("[OTP][OCP-43261] APIServer Support None audit policy [Serial]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit profile")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalProfile := apiServer.Spec.Audit.Profile
		framework.Logf("Current audit profile: %s", originalProfile)

		defer func() {
			g.By("Cleanup: Revert to original audit profile")
			patch := fmt.Sprintf(`{"spec":{"audit":{"profile":%q}}}`, originalProfile)
			_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver", "cluster",
				"--type=merge",
				"--patch="+patch,
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
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

		g.By("3) Verify audit profile is set to None")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(apiServer.Spec.Audit.Profile).To(o.Equal(configv1.NoneAuditProfileType))
	})

	// OCP-43336: Support customRules list for by-group profiles [Serial]
	g.It("[OTP][OCP-43336] Support customRules list for by-group profiles [Serial]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit config")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalCustomRules := apiServer.Spec.Audit.CustomRules
		framework.Logf("Current custom rules: %v", originalCustomRules)

		defer func() {
			g.By("Cleanup: Revert to original custom rules")
			var patch string
			if len(originalCustomRules) > 0 {
				rulesJSON, err := json.Marshal(originalCustomRules)
				o.Expect(err).NotTo(o.HaveOccurred())
				patch = fmt.Sprintf(`{"spec":{"audit":{"customRules":%s}}}`, string(rulesJSON))
			} else {
				patch = `{"spec":{"audit":{"customRules":null}}}`
			}
			_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver", "cluster",
				"--type=merge",
				"--patch="+patch,
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
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

		g.By("3) Verify custom rule was added")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(apiServer.Spec.Audit.CustomRules)).To(o.BeNumerically(">", 0))
	})

	// OCP-73410: Support customRules list for by-group with none profile [Serial]
	g.It("[OTP][OCP-73410] Support customRules list for by-group with none profile [Serial]", ote.Informing(), func(ctx g.SpecContext) {
		g.By("1) Get current audit config")
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalCustomRules := apiServer.Spec.Audit.CustomRules
		framework.Logf("Current custom rules: %v", originalCustomRules)

		defer func() {
			g.By("Cleanup: Revert to original custom rules")
			var patch string
			if len(originalCustomRules) > 0 {
				rulesJSON, err := json.Marshal(originalCustomRules)
				o.Expect(err).NotTo(o.HaveOccurred())
				patch = fmt.Sprintf(`{"spec":{"audit":{"customRules":%s}}}`, string(rulesJSON))
			} else {
				patch = `{"spec":{"audit":{"customRules":null}}}`
			}
			_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver", "cluster",
				"--type=merge",
				"--patch="+patch,
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
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

		g.By("3) Verify custom rule with None profile was added")
		apiServer, err = oc.AdminConfigClient().ConfigV1().APIServers().Get(
			ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(apiServer.Spec.Audit.CustomRules)).To(o.BeNumerically(">", 0))
	})

	// OCP-68629: Audit log files should not have too permissive mode
	g.It("[OTP][OCP-68629] Audit log files should not have too permissive mode", ote.Informing(), func(ctx g.SpecContext) {
		if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
			g.Skip("HyperShift hosts the control plane externally; master node logs are not accessible via node-logs")
		}

		g.By("1) Get list of master nodes")
		masters, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(
			ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masters.Items).NotTo(o.BeEmpty(), "expected at least one master node")

		g.By("2) Check audit log file permissions on master nodes")
		for _, master := range masters.Items {
			framework.Logf("Checking audit log permissions on master node %s", master.Name)
			output, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args(
				"node-logs", master.Name, "--path=kube-apiserver/audit.log",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred(),
				"failed to read kube-apiserver audit log from master node %s", master.Name)

			// Verify that audit logs are present
			lines := strings.Split(strings.TrimSpace(output), "\n")
			o.Expect(len(lines)).To(o.BeNumerically(">", 0),
				"expected at least one audit log line on node %s", master.Name)
		}
	})
})
