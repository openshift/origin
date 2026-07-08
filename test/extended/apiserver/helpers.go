package apiserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	asAdmin          = true
	withoutNamespace = true
)

var fixturePathCache = make(map[string]string)

type certificateDetails struct {
	Subject string
	Issuer  string
}

func apiserverAuthFixture(filename string) string {
	const apiDirName = "apiserverauth"
	if baseDir, ok := fixturePathCache[apiDirName]; ok {
		return filepath.Join(baseDir, filename)
	}
	baseDir := compat_otp.FixturePath("testdata", apiDirName)
	fixturePathCache[apiDirName] = baseDir
	return filepath.Join(baseDir, filename)
}

func doAction(oc *exutil.CLI, action string, asAdmin, withoutNamespace bool, parameters ...string) (string, error) {
	switch {
	case asAdmin && withoutNamespace:
		return oc.AsAdmin().WithoutNamespace().Run(action).Args(parameters...).Output()
	case asAdmin && !withoutNamespace:
		return oc.AsAdmin().Run(action).Args(parameters...).Output()
	case !asAdmin && withoutNamespace:
		return oc.WithoutNamespace().Run(action).Args(parameters...).Output()
	case !asAdmin && !withoutNamespace:
		return oc.Run(action).Args(parameters...).Output()
	default:
		return "", nil
	}
}

func getResource(oc *exutil.CLI, asAdmin, withoutNamespace bool, parameters ...string) (string, error) {
	return doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
}

func getResourceToBeReady(oc *exutil.CLI, asAdmin, withoutNamespace bool, parameters ...string) string {
	var result string
	var err error
	errPoll := wait.PollUntilContextTimeout(context.Background(), 6*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
		result, err = doAction(oc, "get", asAdmin, withoutNamespace, parameters...)
		if err != nil || len(result) == 0 {
			e2e.Logf("Unable to retrieve the expected resource, retrying...")
			return false, nil
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(errPoll, fmt.Sprintf("Failed to retrieve %v", parameters))
	return result
}

func getGlobalProxy(oc *exutil.CLI) (string, string, string) {
	httpProxy, err := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	httpsProxy, err := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.httpsProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	noProxy, err := getResource(oc, asAdmin, withoutNamespace, "proxy", "cluster", "-o=jsonpath={.status.noProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	return httpProxy, httpsProxy, noProxy
}

func isBaselineCapsSet(oc *exutil.CLI) bool {
	baselineCapabilitySet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "version", "-o=jsonpath={.spec.capabilities.baselineCapabilitySet}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return len(baselineCapabilitySet) != 0
}

func isEnabledCapability(oc *exutil.CLI, component string) bool {
	enabledCapabilities, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion", "-o=jsonpath={.items[*].status.capabilities.enabledCapabilities}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Contains(enabledCapabilities, component)
}

func checkDisconnect(oc *exutil.CLI) bool {
	workNode, err := compat_otp.GetFirstWorkerNode(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	output, err := compat_otp.DebugNode(oc, workNode, "bash", "-c", "curl -I ifconfig.me --connect-timeout 5")
	if !strings.Contains(output, "HTTP") || err != nil {
		e2e.Logf("Unable to access the public Internet from the cluster.")
		return true
	}
	return false
}

func getCoStatus(oc *exutil.CLI, coName string, statusToCompare map[string]string) map[string]string {
	newStatus := make(map[string]string)
	for key := range statusToCompare {
		args := fmt.Sprintf(`-o=jsonpath={.status.conditions[?(.type == '%s')].status}`, key)
		status, _ := getResource(oc, asAdmin, withoutNamespace, "co", coName, args)
		newStatus[key] = status
	}
	return newStatus
}

func isSNOCluster(oc *exutil.CLI) bool {
	masterNodes, _ := compat_otp.GetClusterNodesBy(oc, "master")
	workerNodes, _ := compat_otp.GetClusterNodesBy(oc, "worker")
	return len(masterNodes) == 1 && len(workerNodes) == 1 && masterNodes[0] == workerNodes[0]
}

func waitCoBecomes(oc *exutil.CLI, coName string, baseWaitTime int, expectedStatus map[string]string) error {
	waitTime := baseWaitTime
	const stableDelay = 100 * time.Second

	if isSNOCluster(oc) {
		waitTime = baseWaitTime * 3
	}
	if compat_otp.IsArbiterCluster(oc) {
		waitTime = baseWaitTime * 7 / 10
	}

	errCo := wait.PollUntilContextTimeout(context.Background(), 20*time.Second, time.Duration(waitTime)*time.Second, false, func(ctx context.Context) (bool, error) {
		gottenStatus := getCoStatus(oc, coName, expectedStatus)
		if !reflect.DeepEqual(expectedStatus, gottenStatus) {
			return false, nil
		}
		healthy := reflect.DeepEqual(expectedStatus, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"})
		if healthy {
			time.Sleep(stableDelay)
			gottenStatus = getCoStatus(oc, coName, expectedStatus)
			if !reflect.DeepEqual(expectedStatus, gottenStatus) {
				return false, nil
			}
			e2e.Logf("Given operator %s becomes available/non-progressing/non-degraded", coName)
			return true, nil
		}
		e2e.Logf("Given operator %s becomes %s", coName, gottenStatus)
		return true, nil
	})
	if errCo != nil {
		_ = oc.AsAdmin().WithoutNamespace().Run("get").Args("co").Execute()
	}
	return errCo
}

func getPodsListByLabel(oc *exutil.CLI, namespace, selectorLabel string) []string {
	podsOp := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pod", "-n", namespace, "-l", selectorLabel, "-o=jsonpath={.items[*].metadata.name}")
	o.Expect(podsOp).NotTo(o.BeEmpty())
	return strings.Split(podsOp, " ")
}

func getPodsList(oc *exutil.CLI, namespace string) []string {
	podsOp := getResourceToBeReady(oc, asAdmin, withoutNamespace, "pod", "-n", namespace, "-o=jsonpath={.items[*].metadata.name}")
	return strings.Fields(strings.TrimSpace(podsOp))
}

func execCommandOnPod(oc *exutil.CLI, podName, namespace, command string) string {
	var podOutput string
	var execErr error
	errExec := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, 300*time.Second, false, func(ctx context.Context) (bool, error) {
		podOutput, execErr = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", namespace, podName, "--", "/bin/sh", "-c", command).Output()
		podOutput = strings.TrimSpace(podOutput)
		if execErr != nil || podOutput == "" {
			return false, nil
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(errExec, fmt.Sprintf("Unable to run command on pod %s: %v", podName, execErr))
	return podOutput
}

func copyToFile(fromPath, toFilename string) string {
	srcFileStat, err := os.Stat(fromPath)
	if err != nil {
		e2e.Failf("get source file %s stat failed: %v", fromPath, err)
	}
	if !srcFileStat.Mode().IsRegular() {
		e2e.Failf("source file %s is not a regular file", fromPath)
	}

	source, err := os.Open(fromPath)
	if err != nil {
		e2e.Failf("open source file %s failed: %v", fromPath, err)
	}
	defer source.Close()

	saveTo := filepath.Join(e2e.TestContext.OutputDir, toFilename)
	dest, err := os.Create(saveTo)
	if err != nil {
		e2e.Failf("open destination file %s failed: %v", saveTo, err)
	}
	defer dest.Close()

	if _, err = io.Copy(dest, source); err != nil {
		e2e.Failf("copy file from %s to %s failed: %v", fromPath, saveTo, err)
	}
	return saveTo
}

func getAPIServerFQDNAndPort(oc *exutil.CLI) (string, string) {
	apiServerURL, err := oc.AsAdmin().WithoutNamespace().Run("config").Args("view", "-ojsonpath={.clusters[0].cluster.server}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	parsed, err := url.Parse(apiServerURL)
	o.Expect(err).NotTo(o.HaveOccurred())
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	return parsed.Hostname(), port
}

func getProxyURL() *url.URL {
	proxyURLString := os.Getenv("https_proxy")
	if proxyURLString == "" {
		proxyURLString = os.Getenv("http_proxy")
	}
	if proxyURLString == "" {
		return nil
	}
	proxyURL, err := url.Parse(proxyURLString)
	if err != nil {
		e2e.Failf("error parsing proxy URL: %v", err)
	}
	return proxyURL
}

func urlHealthCheck(fqdnName, port, certPath string, returnValues []string) (*certificateDetails, error) {
	if port == "" {
		port = "443"
	}
	caCert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(getProxyURL()),
		TLSClientConfig: &tls.Config{
			RootCAs: caCertPool,
		},
	}
	client := &http.Client{Transport: transport}
	targetURL := fmt.Sprintf("https://%s:%s/healthz", fqdnName, port)

	var certDetails *certificateDetails
	err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		resp, err := client.Get(targetURL)
		if err != nil {
			e2e.Logf("Error performing HTTP request: %s, retrying...", err)
			return false, nil
		}
		defer resp.Body.Close()
		_, _ = io.ReadAll(resp.Body)

		certDetails = &certificateDetails{}
		if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
			cert := resp.TLS.PeerCertificates[0]
			for _, value := range returnValues {
				switch value {
				case "Subject":
					certDetails.Subject = cert.Subject.String()
				case "Issuer":
					certDetails.Issuer = cert.Issuer.String()
				}
			}
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	return certDetails, nil
}

func skipIfBaselineCapsMissingCapabilities(oc *exutil.CLI, capabilities ...string) {
	if !isBaselineCapsSet(oc) {
		return
	}
	for _, capability := range capabilities {
		if !isEnabledCapability(oc, capability) {
			g.Skip(fmt.Sprintf("Skipping the test as baseline capabilities have been set and required capabilities are not enabled: %s", strings.Join(capabilities, ", ")))
		}
	}
}

func isTechPreviewNoUpgradeCluster(oc *exutil.CLI) bool {
	return exutil.IsTechPreviewNoUpgrade(context.Background(), oc.AdminConfigClient())
}

func skipIfProxyCluster(oc *exutil.CLI) {
	httpProxy, httpsProxy, _ := getGlobalProxy(oc)
	if strings.Contains(httpProxy, "http") || strings.Contains(httpsProxy, "https") {
		g.Skip("Skip for proxy platform")
	}
}

type admissionWebhook struct {
	name             string
	webhookname      string
	servicenamespace string
	servicename      string
	namespace        string
	apigroups        string
	apiversions      string
	operations       string
	resources        string
	singularname     string
	pluralname       string
	kind             string
	shortname        string
	version          string
	template         string
}

type webhookService struct {
	name      string
	clusterip string
	namespace string
	template  string
}

func (w admissionWebhook) createAdmissionWebhookFromTemplate(oc *exutil.CLI) {
	params := []string{
		"-n", w.namespace,
		"-f", w.template,
		"-p",
		"NAME=" + w.name,
		"WEBHOOKNAME=" + w.webhookname,
		"SERVICENAMESPACE=" + w.servicenamespace,
		"SERVICENAME=" + w.servicename,
		"NAMESPACE=" + w.namespace,
		"APIGROUPS=" + w.apigroups,
		"APIVERSIONS=" + w.apiversions,
		"OPERATIONS=" + w.operations,
		"RESOURCES=" + w.resources,
	}
	if w.singularname != "" {
		params = append(params,
			"SINGULARNAME="+w.singularname,
			"PLURALNAME="+w.pluralname,
			"KIND="+w.kind,
			"SHORTNAME="+w.shortname,
			"VERSION="+w.version,
		)
	}
	configFile := compat_otp.ProcessTemplate(oc, params...)
	err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", configFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (s webhookService) createServiceFromTemplate(oc *exutil.CLI) {
	params := []string{
		"-n", s.namespace,
		"-f", s.template,
		"-p",
		"NAME=" + s.name,
		"NAMESPACE=" + s.namespace,
		"CLUSTERIP=" + s.clusterip,
	}
	configFile := compat_otp.ProcessTemplate(oc, params...)
	err := oc.AsAdmin().Run("create").Args("-f", configFile).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkIfResourceAvailable(oc *exutil.CLI, resourceKind string, names []string, namespace string) (string, bool) {
	for _, name := range names {
		args := []string{strings.ToLower(resourceKind), name}
		if namespace != "" {
			args = append([]string{"-n", namespace}, args...)
		}
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Output()
		if err != nil {
			return out, false
		}
	}
	return "", true
}

func kasOperatorCheckForStep(oc *exutil.CLI, preConfigKasStatus map[string]string, step, description string) {
	kubeApiserverCoStatus := map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
	currentStatus := getCoStatus(oc, "kube-apiserver", kubeApiserverCoStatus)
	if !reflect.DeepEqual(currentStatus, preConfigKasStatus) {
		e2e.Failf("Step %s failed after %s: kube-apiserver operator status changed from %v to %v",
			step, description, preConfigKasStatus, currentStatus)
	}
}

func compareAPIServerWebhookConditions(oc *exutil.CLI, reasons interface{}, expectedStatus string, conditionTypes []string) {
	reasonList := normalizeWebhookReasons(reasons)
	for _, conditionType := range conditionTypes {
		statusPath := fmt.Sprintf(`jsonpath={.status.conditions[?(@.type=="%s")].status}`, conditionType)
		reasonPath := fmt.Sprintf(`jsonpath={.status.conditions[?(@.type=="%s")].reason}`, conditionType)

		// Wait for the kube-apiserver operator to detect and report the webhook error
		// The operator needs time to reconcile and update its status conditions
		// Use longer timeout when expecting False (clearing errors takes longer)
		timeout := 120 * time.Second
		if expectedStatus == "False" {
			timeout = 300 * time.Second // 5 minutes for error clearing
		}

		err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, timeout, false, func(ctx context.Context) (bool, error) {
			status, err := getResource(oc, asAdmin, withoutNamespace, "kubeapiserver", "cluster", "-o", statusPath)
			if err != nil {
				return false, nil
			}
			return status == expectedStatus, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(),
			"timed out waiting for kubeapiserver condition %s status to become %s", conditionType, expectedStatus)

		// Verify final status
		status, err := getResource(oc, asAdmin, withoutNamespace, "kubeapiserver", "cluster", "-o", statusPath)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read kubeapiserver condition %s status", conditionType)
		o.Expect(status).To(o.Equal(expectedStatus),
			"expected kubeapiserver condition %s status %s, got %s", conditionType, expectedStatus, status)

		if expectedStatus == "True" && len(reasonList) > 0 {
			reason, err := getResource(oc, asAdmin, withoutNamespace, "kubeapiserver", "cluster", "-o", reasonPath)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to read kubeapiserver condition %s reason", conditionType)
			o.Expect(reasonList).To(o.ContainElement(reason),
				"expected kubeapiserver condition %s reason to be one of %v, got %q", conditionType, reasonList, reason)
		}
	}
}

func normalizeWebhookReasons(reasons interface{}) []string {
	switch v := reasons.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	default:
		return nil
	}
}

func getServiceIP(oc *exutil.CLI, baseIP string) string {
	ip := net.ParseIP(baseIP)
	if ip == nil {
		g.Skip(fmt.Sprintf("unable to parse cluster service IP %q", baseIP))
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		g.Skip(fmt.Sprintf("cluster service IP %q is not IPv4", baseIP))
	}

	for lastOctet := 200; lastOctet < 255; lastOctet++ {
		candidate := fmt.Sprintf("%d.%d.%d.%d", ipv4[0], ipv4[1], ipv4[2], byte(lastOctet))
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("services", "--all-namespaces", "-o", `jsonpath={.items[*].spec.clusterIP}`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(out, candidate) {
			return candidate
		}
	}
	g.Skip("unable to find unused service cluster IP")
	return ""
}

func createSecretsWithQuotaValidation(oc *exutil.CLI, namespace, clusterQuotaName string, crqLimits map[string]string, caseID string) {
	secretsLimit, err := strconv.Atoi(strings.TrimSpace(crqLimits["secrets"]))
	o.Expect(err).NotTo(o.HaveOccurred())

	secretsCount, err := oc.Run("get").Args("-n", namespace, "clusterresourcequota", clusterQuotaName, "-o", `jsonpath={.status.namespaces[*].status.used.secrets}`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	existingCount, _ := strconv.Atoi(strings.TrimSpace(secretsCount))

	for i := existingCount; i <= secretsLimit; i++ {
		secretName := fmt.Sprintf("%s-secret-%d", caseID, i)
		output, createErr := oc.Run("create").Args("-n", namespace, "secret", "generic", secretName, "--from-literal=key=value").Output()
		if i < secretsLimit {
			o.Expect(createErr).NotTo(o.HaveOccurred(), "expected secret %s to be created", secretName)
		} else {
			o.Expect(output).To(o.MatchRegexp(`secrets.*forbidden: exceeded quota`))
		}
	}
}

func checkCoStatus(oc *exutil.CLI, coName string, expectedStatus map[string]string) {
	currentStatus := getCoStatus(oc, coName, expectedStatus)
	if !reflect.DeepEqual(currentStatus, expectedStatus) {
		e2e.Failf("cluster operator %s status %v does not match expected %v", coName, currentStatus, expectedStatus)
	}
}

func clusterHealthcheck(oc *exutil.CLI, logPrefix string) error {
	if err := clusterOperatorHealthcheck(oc, 120, logPrefix); err != nil {
		return err
	}
	return clusterNodesHealthcheck(oc, 120, logPrefix)
}

func clusterOperatorHealthcheck(oc *exutil.CLI, timeoutSeconds int, logPrefix string) error {
	expected := map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
	operators, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "-o", `jsonpath={.items[*].metadata.name}`).Output()
	if err != nil {
		return err
	}
	err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, time.Duration(timeoutSeconds)*time.Second, false, func(ctx context.Context) (bool, error) {
		for _, coName := range strings.Fields(operators) {
			status := getCoStatus(oc, coName, expected)
			if !reflect.DeepEqual(status, expected) {
				e2e.Logf("operator %s status %v while waiting for healthy cluster (%s)", coName, status, logPrefix)
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("cluster operators not healthy: %w", err)
	}
	return nil
}

func clusterNodesHealthcheck(oc *exutil.CLI, timeoutSeconds int, logPrefix string) error {
	err := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, time.Duration(timeoutSeconds)*time.Second, false, func(ctx context.Context) (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o", `jsonpath={.items[*].status.conditions[?(@.type=="Ready")].status}`).Output()
		if err != nil {
			return false, nil
		}
		for _, status := range strings.Fields(output) {
			if status != "True" {
				e2e.Logf("node not ready (%s): %s", logPrefix, output)
				return false, nil
			}
		}
		return len(strings.Fields(output)) > 0, nil
	})
	if err != nil {
		return fmt.Errorf("cluster nodes not healthy: %w", err)
	}
	return nil
}

func checkClusterLoad(oc *exutil.CLI, role, logFile string) (int, int) {
	_, _ = oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node").OutputToFile(logFile)
	output, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("top", "node", "-l", fmt.Sprintf("node-role.kubernetes.io/%s", role), "--no-headers").Output()
	if err != nil {
		e2e.Logf("unable to read node utilization for role %s: %v", role, err)
		return 0, 0
	}

	cpuTotal, memTotal, count := 0, 0, 0
	cpuRe := regexp.MustCompile(`(\d+)%`)
	memRe := regexp.MustCompile(`(\d+)%`)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		cpuMatches := cpuRe.FindStringSubmatch(line)
		memMatches := memRe.FindStringSubmatch(line)
		if len(cpuMatches) < 2 || len(memMatches) < 2 {
			continue
		}
		cpu, _ := strconv.Atoi(cpuMatches[1])
		mem, _ := strconv.Atoi(memMatches[1])
		cpuTotal += cpu
		memTotal += mem
		count++
	}
	if count == 0 {
		return 0, 0
	}
	return cpuTotal / count, memTotal / count
}

func checkResources(oc *exutil.CLI, logFile string) map[string]int {
	_, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-A", "--no-headers").OutputToFile(logFile)
	counts := map[string]int{}
	for _, resource := range []string{"pods", "deployments", "services", "secrets"} {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(resource, "-A", "--no-headers").Output()
		if err != nil {
			continue
		}
		counts[resource] = len(strings.Fields(strings.TrimSpace(output)))
	}
	return counts
}

func loadCPUMemWorkload(oc *exutil.CLI, durationSeconds int) {
	if _, err := exec.LookPath("clusterbuster"); err != nil {
		g.Skip("clusterbuster is not installed in the test environment")
	}
	cpuCmd := fmt.Sprintf("clusterbuster -n 50 -w cpusoaker -B cpuload --runtime=%d", durationSeconds)
	memCmd := fmt.Sprintf("clusterbuster -n 50 -w memory -B memload --runtime=%d", durationSeconds)
	for _, cmd := range []string{cpuCmd, memCmd} {
		_, err := exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			g.Skip(fmt.Sprintf("clusterbuster workload failed to start: %v", err))
		}
	}
}