package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/apiserverauth"
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
		var (
			patchAllRequestBodies   = `{"spec":{"audit":{"profile":"AllRequestBodies"}}}`
			patchWriteRequestBodies = `{"spec":{"audit":{"profile":"WriteRequestBodies"}}}`
			podScript               = "grep -r '\"managedFields\":{' /var/log/kube-apiserver | wc -l"
			now                     = time.Now().UTC()
			unixTimestamp           = now.Unix()
		)

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
			waitForAPIServerRollout()
		}()

		g.By("2) Check default audit profile of cluster")
		apiserverauth.CheckApiserversAuditPolicies(oc, "Default")

		g.By("3) Get all master nodes")
		masterNodes, err := compat_otp.GetClusterNodesBy(oc, "master")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masterNodes).NotTo(o.BeEmpty(), "No master nodes found")

		g.By("4) Verify no verbs in kube-apiserver audit logs before profile change")
		script := fmt.Sprintf(`grep -hE "\"verb\":\"(create|delete|patch|update)\",\"user\":.*(requestObject|responseObject)|\"verb\":\"(get|list|watch)\",\"user\":.*(requestObject|responseObject)" /var/log/kube-apiserver/audit.log | jq -r "select (.requestReceivedTimestamp | .[0:19] + \"Z\" | fromdateiso8601 > %v)" | tail -n 1`, unixTimestamp)
		masterNodeLogs, errCount := apiserverauth.CheckAuditLogs(oc, script, masterNodes[0], "openshift-kube-apiserver")
		if errCount > 0 {
			framework.Failf("Verbs found in kube-apiserver audit logs before profile change on master node %v :: %v", masterNodes[0], masterNodeLogs)
		}
		framework.Logf("No verbs in kube-apiserver audit logs before profile change on master node %v", masterNodes[0])

		g.By("5) Set audit profile to WriteRequestBodies")
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster",
			"--type=merge",
			"--patch="+patchWriteRequestBodies,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))
		waitForAPIServerRollout()

		g.By("6) Verify WriteRequestBodies audit profile is active")
		apiserverauth.CheckApiserversAuditPolicies(oc, "WriteRequestBodies")

		g.By("7) Verify verbs and managedFields in kube-apiserver audit logs after WriteRequestBodies profile")
		masterNodeLogs, errCount = apiserverauth.CheckAuditLogs(oc, script, masterNodes[0], "openshift-kube-apiserver")
		if errCount == 0 {
			framework.Failf("Post audit profile to WriteRequestBodies, No Verbs in kube-apiserver audit logs on master node %v :: %v", masterNodes[0], masterNodeLogs)
		}

		podsList := apiserverauth.GetPodsListByLabel(oc.AsAdmin(), "openshift-kube-apiserver", "app=openshift-kube-apiserver")
		execKasOutput := apiserverauth.ExecCommandOnPod(oc, podsList[0], "openshift-kube-apiserver", podScript)
		trimOutput := strings.TrimSpace(execKasOutput)
		count, err := strconv.Atoi(trimOutput)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(count).To(o.Equal(0), "managedFields count should be zero in KAS logs, got: %d", count)
		framework.Logf("Post WriteRequestBodies profile, verbs captured in audit logs and managedFields count is zero")

		g.By("8) Set audit profile to AllRequestBodies")
		output, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"apiserver", "cluster",
			"--type=merge",
			"--patch="+patchAllRequestBodies,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("patched"))
		waitForAPIServerRollout()

		g.By("9) Verify AllRequestBodies audit profile is active")
		apiserverauth.CheckApiserversAuditPolicies(oc, "AllRequestBodies")

		g.By("10) Verify verbs and managedFields in kube-apiserver audit logs after AllRequestBodies profile")
		masterNodeLogs, errCount = apiserverauth.CheckAuditLogs(oc, script, masterNodes[0], "openshift-kube-apiserver")
		if errCount == 0 {
			framework.Failf("Post audit profile to AllRequestBodies, No Verbs in kube-apiserver audit logs on master node %v :: %v", masterNodes[0], masterNodeLogs)
		}

		execKasOutput = apiserverauth.ExecCommandOnPod(oc, podsList[0], "openshift-kube-apiserver", podScript)
		trimOutput = strings.TrimSpace(execKasOutput)
		count, err = strconv.Atoi(trimOutput)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(count).To(o.Equal(0), "managedFields count should be zero in KAS logs, got: %d", count)
		framework.Logf("Post AllRequestBodies profile, verbs captured in audit logs and managedFields count is zero")
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

	// OCP-38865: Examine abnormal errors in openshift-apiserver pod logs and audit logs
	g.It("[OTP][OCP-38865] should not have abnormal errors in openshift-apiserver pod logs and audit logs", ote.Informing(), func() {
		isHyperShift, err := exutil.IsHypershift(context.Background(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("HyperShift hosts the control plane externally; apiserver logs are not accessible via node debug")
		}

		g.By("1) Create log arrays")
		podAbnormalLogs := make([]string, 0)
		externalPanicLogs := make([]string, 0)
		masterNodeAbnormalLogs := make([]string, 0)
		auditAbnormalLogs := make([]string, 0)
		totalAbnormalLogCount := 0

		g.By("2) Setup start/end tags for extracting logs from other unrelated stdout like oc debug warning")
		startTag := "<START_LOG>"
		endTag := "</END_LOG>"
		trimStartTag, regexErr := regexp.Compile(fmt.Sprintf("(.|\n|\r)*%s", startTag))
		o.Expect(regexErr).NotTo(o.HaveOccurred())
		trimEndTag, regexErr := regexp.Compile(fmt.Sprintf("%s(.|\n|\r)*", endTag))
		o.Expect(regexErr).NotTo(o.HaveOccurred())

		g.By("3) Get all master nodes")
		masterNodes, getAllMasterNodesErr := compat_otp.GetClusterNodesBy(oc, "master")
		o.Expect(getAllMasterNodesErr).NotTo(o.HaveOccurred())
		o.Expect(masterNodes).NotTo(o.BeEmpty())

		g.By("4) Check OAS operator pod logs for abnormal (panic/fatal/SHOULD NOT HAPPEN) logs, expect none")
		clusterOperator := "openshift-apiserver-operator"
		keywords := "panic|fatal|SHOULD NOT HAPPEN"
		format := `[0-9TZ.:]{5,30}`
		frontwords := `(\w+?[^0-9a-zA-Z]+?){,3}`
		afterwords := `(\w+?[^0-9a-zA-Z]+?){,30}`
		exceptions := `W[0-9]{4}|panicked: false|e2e-test-|kernel.*-panic|non-fatal|(ocp|OCP)\d{4,}|W\d{4}.*fatal|SHOULD NOT HAPPEN.*(pwomnew|lmnew|pmnew|lwomnew)-(app|build)|SubjectAccessReview|LocalResourceAccessReview|(APIServicesDegraded|ConfigObservationDegraded|APIServerWorkloadDegraded|APIServerDeploymentAvailable|APIServerDeploymentDegraded|APIServicesAvailable|APIServerDeploymentProgressing).*fatal`
		cmd := fmt.Sprintf(`export KUBECONFIG=/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig
grep -hriE "(%s%s%s)+" /var/log/pods/openshift-apiserver-operator* | grep -Ev "%s" > /tmp/OCP-38865-oaso-errors.log
sed -E "s/%s/../g" /tmp/OCP-38865-oaso-errors.log | sort | uniq -c | sort -h | tee /tmp/OCP-38865-oaso-uniq-errors.log | head -10
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-38865-oaso-errors.log | head -1
done < <(grep -oP "\w+?\.go\:[0-9]+" /tmp/OCP-38865-oaso-uniq-errors.log | uniq | head -10)
echo '%s'`, frontwords, keywords, afterwords, exceptions, format, startTag, endTag)
		masterNode, err := oc.WithoutNamespace().Run("get").Args("po", "-n", clusterOperator, "-o", `jsonpath={.items[0].spec.nodeName}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("4.1) Get log file from %s", masterNode))
		podLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
		o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("4.2) Format log file from %s", masterNode))
		podLogs = trimStartTag.ReplaceAllString(podLogs, "")
		podLogs = trimEndTag.ReplaceAllString(podLogs, "")
		for _, line := range strings.Split(podLogs, "\n") {
			if strings.Trim(line, " ") != "" {
				podAbnormalLogs = append(podAbnormalLogs, fmt.Sprintf("> %s", line))
			}
		}
		framework.Logf("OAS-O Pod abnormal Logs -------------------------->\n%s", strings.Join(podAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(podAbnormalLogs)

		g.By("5) On all master nodes, check OAS log files for panic error")
		cmd = fmt.Sprintf(`RETAG="[EW][0-9]{4}\s[0-9]{2}:[0-9]{2}"
PANIC="${RETAG}.*panic"
panic_logfiles=$(grep -riE "${PANIC}" /var/log/pods/openshift-apiserver_apiserver* | grep -Ev "%s" | cut -d ':' -f1 | head -10 | uniq)
echo '%s'
for f in ${panic_logfiles}; do
    echo ">>> Panic log file: ${f}"
    line=$(grep -inE "${PANIC}" "${f}" | grep -m 1 -Ev "%s"  | cut -d ':' -f1)
    endline=$(( line + 20 ))
    sed -n "${line},${endline}p" "${f}"
done
echo '%s'`, exceptions, startTag, exceptions, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("5.%d) Get log file from %s", i+1, masterNode))
			externalLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("5.%d.1) Format log file from %s", i+1, masterNode))
			externalLogs = trimStartTag.ReplaceAllString(externalLogs, "")
			externalLogs = trimEndTag.ReplaceAllString(externalLogs, "")
			for _, line := range strings.Split(externalLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					externalPanicLogs = append(externalPanicLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		framework.Logf("OAS pod panic Logs -------------------------->\n%s", strings.Join(externalPanicLogs, "\n"))

		g.By("6) On all master nodes, check OAS log files for abnormal (fatal/SHOULD NOT HAPPEN) logs, expect none")
		keywords = "fatal|SHOULD NOT HAPPEN"
		cmd = fmt.Sprintf(`grep -hriE "(%s%s%s)+" /var/log/pods/openshift-apiserver_apiserver* | grep -Ev "%s"  > /tmp/OCP-38865-oas-errors.log
sed -E "s/%s/../g" /tmp/OCP-38865-oas-errors.log | sort | uniq -c | sort -h | tee > /tmp/OCP-38865-oas-uniq-errors.log | head -10
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-38865-oas-errors.log | head -1
done < <(grep -oP "\w+?\.go\:[0-9]+" /tmp/OCP-38865-oas-uniq-errors.log | uniq | head -10)
echo '%s'`, frontwords, keywords, afterwords, exceptions, format, startTag, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("6.%d) Get log file from %s", i+1, masterNode))
			masterNodeLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("6.%d.1) Format log file from %s", i+1, masterNode))
			masterNodeLogs = trimStartTag.ReplaceAllString(masterNodeLogs, "")
			masterNodeLogs = trimEndTag.ReplaceAllString(masterNodeLogs, "")
			for _, line := range strings.Split(masterNodeLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					masterNodeAbnormalLogs = append(masterNodeAbnormalLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		framework.Logf("OAS pods abnormal Logs ------------------------->\n%s", strings.Join(masterNodeAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(masterNodeAbnormalLogs)

		g.By("7) On all master nodes, check oas audit logs for abnormal (panic/fatal/SHOULD NOT HAPPEN) logs")
		keywords = "panic|fatal|SHOULD NOT HAPPEN"
		exceptions = "W[0-9]{4}|kernel_config_panic|APIServer panic.*net/http: abort Handler|LogLevelFatal|log.*FATAL"
		cmd = fmt.Sprintf(`grep -ihE '(%s)' /var/log/openshift-apiserver/audit*.log | grep -Ev '%s' > /tmp/OCP-38865-audit-errors.log
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-38865-audit-errors.log | head -1 | jq .
done < <(cat /tmp/OCP-38865-audit-errors.log | jq -r '.responseStatus.status + " - " + .responseStatus.message + " - " + .responseStatus.reason' | uniq | head -5 | cut -d '-' -f2 | awk '{$1=$1;print}')
echo '%s'`, keywords, exceptions, startTag, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("7.%d) Get log file from %s", i+1, masterNode))
			auditLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("7.%d.1) Format log file from %s", i+1, masterNode))
			auditLogs = trimStartTag.ReplaceAllString(auditLogs, "")
			auditLogs = trimEndTag.ReplaceAllString(auditLogs, "")
			for _, line := range strings.Split(auditLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					auditAbnormalLogs = append(auditAbnormalLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		framework.Logf("OAS audit abnormal Logs --------------------->\n%s", strings.Join(auditAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(auditAbnormalLogs)

		g.By("8) Assert if abnormal log exits")
		o.Expect(totalAbnormalLogCount).Should(o.BeZero())
	})
})

var _ = g.Describe("[sig-api-machinery] Kube APIServer Error Checking", func() {
	oc := exutil.NewCLIWithoutNamespace("kube-apiserver-errors")

	// OCP-43889: Examine non critical kube-apiserver errors
	g.It("[OTP][OCP-43889] should examine non critical kube-apiserver errors", ote.Informing(), func() {
		isHyperShift, err := exutil.IsHypershift(context.Background(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("HyperShift hosts the control plane externally; apiserver logs are not accessible via node debug")
		}

		var (
			keywords     = "(error|fail|tcp dial timeout|connect: connection refused|Unable to connect to the server: dial tcp|remote error: tls: bad certificate)"
			exceptions   = "panic|fatal|SHOULD NOT HAPPEN|NotFound|AlreadyExists|not found|already exists|connection refused|certificate|tls"
			format       = "[0-9TZ.:]{2,30}"
			words        = `(\w+?[^0-9a-zA-Z]+?){,5}`
			afterwords   = `(\w+?[^0-9a-zA-Z]+?){,12}`
			co           = "openshift-kube-apiserver-operator"
			dirname      = "/tmp/-OCP-43889/"
			regexToGrep1 = "(" + words + keywords + words + ")" + "+"
			regexToGrep2 = "(" + words + keywords + afterwords + ")" + "+"
		)

		defer os.RemoveAll(dirname)
		err = os.MkdirAll(dirname, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("1) Check the log files of KAS operator")
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", co, "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podlog, errlog := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", co, podname).OutputToFile("OCP-43889/kas-o-grep.log")
		o.Expect(errlog).NotTo(o.HaveOccurred())
		cmd := fmt.Sprintf(`cat %v |grep -ohiE '%s' |grep -iEv '%s' | sed -E 's/%s/../g' | sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, podlog, regexToGrep1, exceptions, format)
		kasOLog, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("%s", kasOLog)

		g.By("2) Check the log files of KAS")
		masterNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		masterName := strings.Fields(masterNode)
		cmd = fmt.Sprintf(`grep -rohiE '%s' |grep -iEv '%s' /var/log/pods/openshift-kube-apiserver_kube-apiserver*/*/* | sed -E 's/%s/../g'`, regexToGrep2, exceptions, format)
		for i := 0; i < len(masterName); i++ {
			_, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", "openshift-kube-apiserver", "node/"+masterName[i], "--", "chroot", "/host", "bash", "-c", cmd).OutputToFile("OCP-43889/kas_pod.log." + masterName[i])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v| sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, dirname+"kas_pod.log.*")
		kasPodlogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("%s", kasPodlogs)

		g.By("3) Check the audit log files of KAS")
		cmd = fmt.Sprintf(`grep -rohiE '%s' /var/log/kube-apiserver/audit*.log |grep -iEv '%s' | sed -E 's/%s/../g'`, regexToGrep2, exceptions, format)
		for i := 0; i < len(masterName); i++ {
			_, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("-n", "openshift-kube-apiserver", "node/"+masterName[i], "--", "chroot", "/host", "bash", "-c", cmd).OutputToFile("OCP-43889/kas_audit.log." + masterName[i])
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		cmd = fmt.Sprintf(`cat %v| sort | uniq -c | sort -rh | awk '$1 >5000 {print}'`, dirname+"kas_audit.log.*")
		kasAuditlogs, err := exec.Command("bash", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("%s", kasAuditlogs)

		g.By("4) Checking pod and audit logs")
		if len(kasOLog) > 0 || len(kasPodlogs) > 0 || len(kasAuditlogs) > 0 {
			var errorDetails strings.Builder
			if len(kasOLog) > 0 {
				errorDetails.WriteString(fmt.Sprintf("\nKAS Operator errors (>5000 occurrences):\n%s", kasOLog))
			}
			if len(kasPodlogs) > 0 {
				errorDetails.WriteString(fmt.Sprintf("\nKAS Pod errors (>5000 occurrences):\n%s", kasPodlogs))
			}
			if len(kasAuditlogs) > 0 {
				errorDetails.WriteString(fmt.Sprintf("\nKAS Audit errors (>5000 occurrences):\n%s", kasAuditlogs))
			}
			framework.Failf("Found high-frequency errors in kube-apiserver logs. Review the errors below and file bugs if they indicate real issues:%s", errorDetails.String())
		} else {
			framework.Logf("Test pass: No errors found from KAS operator, KAS logs/audit logs")
		}
	})
})
