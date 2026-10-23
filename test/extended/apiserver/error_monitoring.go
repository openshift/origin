package apiserver

import (
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery] [Jira:apiserver-auth] API Server Error Monitoring", func() {
	oc := exutil.NewCLIWithoutNamespace("error-monitoring")

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Error monitoring tests are not supported on MicroShift")
		}
	})

	g.It("[OTP][OCP-39601] should not have critical errors in openshift-kube-apiserver related log files", ote.Informing(), func() {
		g.By("1) Create log arrays")
		podAbnormalLogs := make([]string, 0)
		masterNodeAbnormalLogs := make([]string, 0)
		externalPanicLogs := make([]string, 0)
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

		g.By("4) Check KAS operator pod logs for abnormal (panic/fatal/SHOULD NOT HAPPEN) logs, expect none")
		clusterOperator := "openshift-kube-apiserver-operator"
		keywords := "panic|fatal|SHOULD NOT HAPPEN|duplicate entry"
		format := `[0-9TZ.:]{5,30}`
		frontwords := `(\w+?[^0-9a-zA-Z]+?){,3}`
		afterwords := `(\w+?[^0-9a-zA-Z]+?){,30}`
		// Exception list for known non-critical errors
		exceptionsList := []string{
			`W[0-9]{4}`,
			`SHOULD NOT HAPPEN.*Kind=CertificateSigningRequest`,
			`merge.go:121] Should`,
			`Should not happen: Open`,
			`testsource-user-build-volume`,
			`test.tectonic.com`,
			`virtualHostedStyle.*{invalid}`,
			`Kind=MachineHealthCheck.*smd typed.*spec.unhealthyConditions.*timeout`,
			`Kind=MachineHealthCheck.*openshift-machine-api.*mhc-malformed`,
			`OpenAPI.*`,
			`panicked: false`,
			`e2e-test-`,
			`kernel.*-panic`,
			`non-fatal`,
			`(ocp|OCP)[0-9]{4,}`,
			`managedFields.*(imageregistry|marketplace)`,
			`W[0-9]{4}.*fatal`,
			`SHOULD NOT HAPPEN.*Kind=BGPPeer.*failed to convert`,
			`description.*fatal`,
		}
		exceptions := strings.Join(exceptionsList, "|")
		cmd := fmt.Sprintf(`export KUBECONFIG=/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/lb-ext.kubeconfig
grep -hriE "(%s%s%s)+" /var/log/pods/openshift-kube-apiserver-operator* | grep -iEv "%s" > /tmp/OCP-39601-kaso-errors.log
sed -E "s/%s/../g" /tmp/OCP-39601-kaso-errors.log | sort | uniq -c | sort -h | tee /tmp/OCP-39601-kaso-uniq-errors.log | head -10
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-39601-kaso-errors.log | head -1
done < <(grep -oP "\w+?\.go\:[0-9]+" /tmp/OCP-39601-kaso-uniq-errors.log | uniq | head -10)
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
		e2e.Logf("KAS-O Pod abnormal Logs -------------------------->\n%s", strings.Join(podAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(podAbnormalLogs)

		g.By("5) On all master nodes, check KAS log files for abnormal (fatal/SHOULD NOT HAPPEN) logs, expect none")
		keywords = "fatal|SHOULD NOT HAPPEN"
		cmd = fmt.Sprintf(`grep -hriE "(%s%s%s)+" /var/log/pods/openshift-kube-apiserver_kube-apiserver* | grep -iEv "%s" > /tmp/OCP-39601-kas-errors.log
sed -E "s/%s/../g" /tmp/OCP-39601-kas-errors.log | sort | uniq -c | sort -h | tee > /tmp/OCP-39601-kas-uniq-errors.log | head -10
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-39601-kas-errors.log | head -1
done < <(grep -oP "\w+?\.go\:[0-9]+" /tmp/OCP-39601-kas-uniq-errors.log | uniq | head -10)
echo '%s'`, frontwords, keywords, afterwords, exceptions, format, startTag, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("5.%d) Get log file from %s", i+1, masterNode))
			masterNodeLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("5.%d.1) Format log file from %s", i+1, masterNode))
			masterNodeLogs = trimStartTag.ReplaceAllString(masterNodeLogs, "")
			masterNodeLogs = trimEndTag.ReplaceAllString(masterNodeLogs, "")
			for _, line := range strings.Split(masterNodeLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					masterNodeAbnormalLogs = append(masterNodeAbnormalLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		e2e.Logf("KAS pods abnormal Logs ------------------------->\n%s", strings.Join(masterNodeAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(masterNodeAbnormalLogs)

		g.By("6) On all master nodes, check KAS log files for panic error")
		cmd = fmt.Sprintf(`RETAG="[EW][0-9]{4}\s[0-9]{2}:[0-9]{2}"
PANIC="${RETAG}.*panic"
panic_logfiles=$(grep -riE "${PANIC}" /var/log/pods/openshift-kube-apiserver_kube-apiserver* | grep -iEv "%s" | cut -d ':' -f1 | head -10 | uniq)
echo '%s'
for f in ${panic_logfiles}; do
    echo ">>> Panic log file: ${f}"
    line=$(grep -inE "${PANIC}" "${f}" | grep -m 1 -iEv "%s"  | cut -d ':' -f1)
    endline=$(( line + 20 ))
    sed -n "${line},${endline}p" "${f}"
done
echo '%s'`, exceptions, startTag, exceptions, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("6.%d) Get panic log file from %s", i+1, masterNode))
			externalLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("6.%d.1) Format panic log file from %s", i+1, masterNode))
			externalLogs = trimStartTag.ReplaceAllString(externalLogs, "")
			externalLogs = trimEndTag.ReplaceAllString(externalLogs, "")
			for _, line := range strings.Split(externalLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					externalPanicLogs = append(externalPanicLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		e2e.Logf("KAS pod panic Logs -------------------------->\n%s", strings.Join(externalPanicLogs, "\n"))
		totalAbnormalLogCount += len(externalPanicLogs)

		g.By("7) On all master nodes, check kas audit logs for abnormal (panic/fatal/SHOULD NOT HAPPEN) logs")
		keywords = "panic|fatal|SHOULD NOT HAPPEN"
		exceptionsList = []string{
			`"kind": "Event"`,
			`W[0-9]{4}`,
			`kernel_config_panic`,
			`allowWatchBookmarks=true.*panic`,
			`fieldSelector.*watch=true.*panic`,
			`APIServer panic.*net/http: abort Handler`,
			`stage.*Panic`,
			`context deadline exceeded - InternalError`,
			`panicked: false`,
			`e2e-test-.*`,
			`kernel.*panic`,
			`(ocp|OCP)[0-9]{4,}`,
			`49167-fatal`,
			`LogLevelFatal`,
			`log.*FATAL`,
			`Force kernel panic`,
			`"Fatal"`,
			`fatal conditions`,
			`OCP-38865-audit-errors`,
			`message.*check fatal=true`,
		}
		exceptions = strings.Join(exceptionsList, "|")
		cmd = fmt.Sprintf(`grep -ihE '(%s)' /var/log/kube-apiserver/audit*.log | grep -iEv '%s' > /tmp/OCP-39601-audit-errors.log
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-39601-audit-errors.log | head -1 | jq .
done < <(cat /tmp/OCP-39601-audit-errors.log | jq -r '.responseStatus.status + " - " + .responseStatus.message + " - " + .responseStatus.reason' | uniq | head -5 | cut -d '-' -f2 | awk '{$1=$1;print}')
echo '%s'`, keywords, exceptions, startTag, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("7.%d) Get audit log file from %s", i+1, masterNode))
			auditLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("7.%d.1) Format audit log file from %s", i+1, masterNode))
			auditLogs = trimStartTag.ReplaceAllString(auditLogs, "")
			auditLogs = trimEndTag.ReplaceAllString(auditLogs, "")
			for _, line := range strings.Split(auditLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					auditAbnormalLogs = append(auditAbnormalLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		e2e.Logf("KAS audit abnormal Logs --------------------->\n%s", strings.Join(auditAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(auditAbnormalLogs)

		g.By("8) Assert if abnormal log exists")
		o.Expect(totalAbnormalLogCount).Should(o.BeZero(), "Found %d critical errors in kube-apiserver logs", totalAbnormalLogCount)
	})

	g.It("[OTP][OCP-42937] should not have critical errors in oauth-apiserver related log files", ote.Informing(), func() {
		g.By("1) Create log arrays")
		masterNodeAbnormalLogs := make([]string, 0)
		externalPanicLogs := make([]string, 0)
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

		g.By("4) On all master nodes, check Oauth-apiserver log files for abnormal (fatal/SHOULD NOT HAPPEN) logs, expect none")
		keywords := "fatal|SHOULD NOT HAPPEN"
		format := `[0-9TZ.:]{5,30}`
		frontwords := `(\w+?[^0-9a-zA-Z]+?){,3}`
		afterwords := `(\w+?[^0-9a-zA-Z]+?){,30}`
		exceptions := "SHOULD NOT HAPPEN.*Kind=CertificateSigningRequest|merge.go:121] Should|Should not happen: OpenAPI|testsource-user-build-volume|test.tectonic.com|virtualHostedStyle.*{invalid}|Kind=MachineHealthCheck.*smd typed.*spec.unhealthyConditions.*timeout|Kind=MachineHealthCheck.*openshift-machine-api.*mhc-malformed|OpenAPI.*)|panicked: false|e2e-test-.*|kernel.*-panic|non-fatal|(ocp|OCP)[0-9]{4,}"
		cmd := fmt.Sprintf(`grep -hriE "(%s%s%s)+" /var/log/pods/openshift-oauth-apiserver_apiserver* | grep -Ev "%s" > /tmp/OCP-42937-oauthas-errors.log
sed -E "s/%s/../g" /tmp/OCP-42937-oauthas-errors.log | sort | uniq -c | sort -h | tee > /tmp/OCP-42937-oauthas-uniq-errors.log | head -10
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-42937-oauthas-errors.log | head -1
done < <(grep -oP "\w+?\.go\:[0-9]+" /tmp/OCP-42937-oauthas-uniq-errors.log | uniq | head -10)
echo '%s'`, frontwords, keywords, afterwords, exceptions, format, startTag, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("4.%d) Get log file from %s", i+1, masterNode))
			masterNodeLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("4.%d.1) Format log file from %s", i+1, masterNode))
			masterNodeLogs = trimStartTag.ReplaceAllString(masterNodeLogs, "")
			masterNodeLogs = trimEndTag.ReplaceAllString(masterNodeLogs, "")
			for _, line := range strings.Split(masterNodeLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					masterNodeAbnormalLogs = append(masterNodeAbnormalLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		e2e.Logf("Oauth-apiserver pods abnormal Logs ------------------------->\n%s", strings.Join(masterNodeAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(masterNodeAbnormalLogs)

		g.By("5) On all master nodes, check Oauth-apiserver log files for panic error")
		cmd = fmt.Sprintf(`RETAG="[EW][0-9]{4}\s[0-9]{2}:[0-9]{2}"
PANIC="${RETAG}.*panic"
panic_logfiles=$(grep -riE "${PANIC}" /var/log/pods/openshift-oauth-apiserver_apiserver* | grep -Ev "%s" | cut -d ':' -f1 | head -10 | uniq)
echo '%s'
for f in ${panic_logfiles}; do
    echo ">>> Panic log file: ${f}"
    line=$(grep -inE "${PANIC}" "${f}" | grep -m 1 -Ev "%s"  | cut -d ':' -f1)
    endline=$(( line + 20 ))
    sed -n "${line},${endline}p" "${f}"
done
echo '%s'`, exceptions, startTag, exceptions, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("5.%d) Get panic log file from %s", i+1, masterNode))
			externalLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("5.%d.1) Format panic log file from %s", i+1, masterNode))
			externalLogs = trimStartTag.ReplaceAllString(externalLogs, "")
			externalLogs = trimEndTag.ReplaceAllString(externalLogs, "")
			for _, line := range strings.Split(externalLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					externalPanicLogs = append(externalPanicLogs, line)
				}
			}
		}
		e2e.Logf("Oauth-apiserver pod panic Logs -------------------------->\n%s", strings.Join(externalPanicLogs, "\n"))
		totalAbnormalLogCount += len(externalPanicLogs)

		g.By("6) On all master nodes, check oauthas audit logs for abnormal (panic/fatal/SHOULD NOT HAPPEN) logs")
		keywords = "panic|fatal|SHOULD NOT HAPPEN"
		exceptions = "allowWatchBookmarks=true.*panic|fieldSelector.*watch=true.*panic|APIServer panic.*:.*(net/http: abort Handler - InternalError|context deadline exceeded - InternalError)|panicked: false|kernel.*-panic|e2e-test-.*|(ocp|OCP)[0-9]{4,}"
		cmd = fmt.Sprintf(`grep -ihE '(%s)' /var/log/oauth-apiserver/audit*.log | grep -Ev '%s' > /tmp/OCP-42937-audit-errors.log
echo '%s'
while read line; do
    grep "$line" /tmp/OCP-42937-audit-errors.log | head -1 | jq .
done < <(cat /tmp/OCP-42937-audit-errors.log | jq -r '.responseStatus.status + " - " + .responseStatus.message + " - " + .responseStatus.reason' | uniq | head -5 | cut -d '-' -f2 | awk '{$1=$1;print}')
echo '%s'`, keywords, exceptions, startTag, endTag)

		for i, masterNode := range masterNodes {
			g.By(fmt.Sprintf("6.%d) Get audit log file from %s", i+1, masterNode))
			auditLogs, checkLogFileErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, masterNode, []string{"--quiet=true", "--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
			o.Expect(checkLogFileErr).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("6.%d.1) Format audit log file from %s", i+1, masterNode))
			auditLogs = trimStartTag.ReplaceAllString(auditLogs, "")
			auditLogs = trimEndTag.ReplaceAllString(auditLogs, "")
			for _, line := range strings.Split(auditLogs, "\n") {
				if strings.Trim(line, " ") != "" {
					auditAbnormalLogs = append(auditAbnormalLogs, fmt.Sprintf("> %s", line))
				}
			}
		}
		e2e.Logf("Oauth-apiserver audit abnormal Logs --------------------->\n%s", strings.Join(auditAbnormalLogs, "\n"))
		totalAbnormalLogCount += len(auditAbnormalLogs)

		g.By("7) Assert if abnormal log exists")
		o.Expect(totalAbnormalLogCount).Should(o.BeZero(), "Found %d critical errors in oauth-apiserver logs", totalAbnormalLogCount)
	})
})
