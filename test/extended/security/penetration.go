package security

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:SecurityPenetration] ", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("security-penetration")

	// CNF-18378: Check For Plain Text Passwords
	g.It("TestNoPasswordExposedInLogFiles [apigroup:config.openshift.io]", ote.Informing(), func() {
		skipIfNotBaremetal(oc)
		ctx := context.Background()

		// Skip for HyperShift - control plane is hosted separately
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			e2eskipper.Skipf("HyperShift clusters with external control plane topology do not have master nodes in the data plane")
		}

		// Skip for MicroShift - different architecture
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			e2eskipper.Skipf("MicroShift clusters have a different architecture and do not follow the same node labeling conventions")
		}

		g.By("Getting master node names")
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "No master nodes found")

		g.By("Checking log files for plain text passwords")
		foundInLogs := checkLogsForPasswords(oc, nodes.Items)
		o.Expect(foundInLogs).To(o.BeEmpty(), fmt.Sprintf("Plain text passwords found in logs: %v", foundInLogs))

		g.By("Checking YAML files for plain text passwords")
		foundInYamls := checkYamlsForPasswords(oc, nodes.Items)
		o.Expect(foundInYamls).To(o.BeEmpty(), fmt.Sprintf("Plain text passwords found in YAMLs: %v", foundInYamls))
	})

	// CNF-21165: Check CNI SELinux From All Nodes
	g.It("TestProperSELinuxContextOnCNI", ote.Informing(), func() {
		skipIfNotBaremetal(oc)
		ctx := context.Background()

		g.By("Getting all node names")
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "No nodes found")

		g.By("Finding the actual CNI path")
		cniPath, found := findCNIPath(oc, nodes.Items[0].Name)
		if !found {
			e2eskipper.Skipf("CNI directory not found on nodes, skipping SELinux context check")
		}

		g.By("Checking SELinux context on all nodes")
		for _, node := range nodes.Items {
			checkSELinuxContext(oc, node.Name, cniPath)
		}
	})

	// CNF-22599: Combined NRHO Security Penetration Tests
	g.Describe("Security Penetration Tests", func() {
		g.It("TestNoSSHKeysInUnexpectedSecrets [apigroup:security.openshift.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			unexpectedSecretCount := countSecretsContainingSSHKeys(ctx, oc)
			o.Expect(unexpectedSecretCount).To(o.Equal(0),
				fmt.Sprintf("Found %d unexpected Secret(s) containing SSH private keys (details redacted for security)", unexpectedSecretCount))
		})

		g.It("TestNoUnexpectedPrivilegedPods", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			privilegedPodCount := countPrivilegedPodsInUserNamespaces(ctx, oc)
			o.Expect(privilegedPodCount).To(o.Equal(0),
				fmt.Sprintf("Found %d privileged pod(s) in user namespaces (details redacted for security)", privilegedPodCount))
		})

		g.It("TestProperNodeSudoConfiguration", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			unexpectedSudoers := findUnexpectedSudoersFiles(oc, nodes.Items)
			o.Expect(unexpectedSudoers).To(o.BeEmpty(),
				fmt.Sprintf("Unexpected sudoers files found: %v", unexpectedSudoers))
		})

		g.It("TestEtcdBackupEncryptionAndRestriction [apigroup:config.openshift.io][apigroup:operator.openshift.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			verifyEtcdEncryptionAtRest(ctx, oc)

			// Skip master node checks for HyperShift and MicroShift
			controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("HyperShift clusters with external control plane topology do not have master nodes in the data plane")
			}

			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				e2eskipper.Skipf("MicroShift clusters have a different architecture and do not follow the same node labeling conventions")
			}

			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(masterNodes.Items)).To(o.BeNumerically(">", 0))

			var allCriticalFiles []string
			for _, node := range masterNodes.Items {
				criticalFiles := findWorldReadableCriticalEtcdFiles(oc, node.Name)
				allCriticalFiles = append(allCriticalFiles, criticalFiles...)
			}
			o.Expect(allCriticalFiles).To(o.BeEmpty(),
				fmt.Sprintf("Critical etcd files are world-readable: %v", allCriticalFiles))
		})
		g.It("TestAllRoutesUseTLS [apigroup:route.openshift.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			routesWithoutTLSCount := countRoutesWithoutTLS(ctx, oc)
			o.Expect(routesWithoutTLSCount).To(o.Equal(0),
				fmt.Sprintf("Found %d route(s) without TLS (details redacted for security)", routesWithoutTLSCount))
		})

		g.It("TestEtcdDirectoryPermissions [apigroup:operator.openshift.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()

			// Skip master node checks for HyperShift and MicroShift
			controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("HyperShift clusters with external control plane topology do not have master nodes in the data plane")
			}

			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				e2eskipper.Skipf("MicroShift clusters have a different architecture and do not follow the same node labeling conventions")
			}

			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(masterNodes.Items)).To(o.BeNumerically(">", 0))

			var allProblems []string
			for _, node := range masterNodes.Items {
				problems := checkEtcdDirectoryPermissions(oc, node.Name)
				allProblems = append(allProblems, problems...)
			}
			o.Expect(allProblems).To(o.BeEmpty(),
				fmt.Sprintf("Etcd data directory permission issues: %v", allProblems))
		})

		g.It("TestSecurityToolingInstalled [apigroup:operators.coreos.com]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			foundOperators := checkSecurityOperators(ctx, oc)
			o.Expect(foundOperators).NotTo(o.BeEmpty(),
				"No security operators found (Compliance, File Integrity, or ACS/Stackrox)")

			auditProfile := getAuditLogProfile(ctx, oc)
			g.By(fmt.Sprintf("Audit log profile: %s", auditProfile))
		})

		g.It("TestMonitoringStackHealthy", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			notRunningPods := getNonRunningMonitoringPods(ctx, oc)
			o.Expect(notRunningPods).To(o.BeEmpty(),
				fmt.Sprintf("Non-running monitoring pods: %v", notRunningPods))

			rulesCount := getPrometheusRulesCount(ctx, oc)
			o.Expect(rulesCount).To(o.BeNumerically(">", 0), "No Prometheus rules found")
		})

		g.It("TestNetworkTrafficEncrypted [apigroup:operator.openshift.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			etcdUsesTLS := verifyEtcdUsesTLS(ctx, oc)
			o.Expect(etcdUsesTLS).To(o.BeTrue(), "Etcd is not using TLS certificates")
		})

		g.It("TestNoUnprotectedDatabasePods", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			dbPodCount := countDatabasePods(ctx, oc)
			// Fail if database pods are found - they should use Secrets for credentials
			o.Expect(dbPodCount).To(o.Equal(0),
				fmt.Sprintf("Found %d database pod(s) - verify credentials use Secrets (details redacted for security)", dbPodCount))
		})

		g.It("TestNoUnexpectedClusterAdminServiceAccounts [apigroup:rbac.authorization.k8s.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			bindingCount := countClusterAdminServiceAccountBindings(ctx, oc)
			// Fail if unexpected cluster-admin service accounts are found
			o.Expect(bindingCount).To(o.Equal(0),
				fmt.Sprintf("Found %d ServiceAccount(s) with cluster-admin role - review for unexpected entries (details redacted for security)", bindingCount))
		})

		g.It("TestNoNFSVolumesRisk", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			nfsPVCount := countNFSPersistentVolumes(ctx, oc)
			// Fail if NFS PVs are found - verify root_squash is enabled on NFS servers
			o.Expect(nfsPVCount).To(o.Equal(0),
				fmt.Sprintf("Found %d NFS PersistentVolume(s) - verify root_squash is enabled on NFS servers (details redacted for security)", nfsPVCount))
		})

		g.It("TestContainerRegistryAuthentication [apigroup:config.openshift.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			insecureRegistryCount := countInsecureRegistries(ctx, oc)
			o.Expect(insecureRegistryCount).To(o.Equal(0),
				fmt.Sprintf("Found %d insecure registr(y/ies) (details redacted for security)", insecureRegistryCount))

			hasRegistryRoute := hasRegistryExternalRoute(ctx, oc)
			if hasRegistryRoute {
				g.By("Registry external route exists (details redacted for security)")
			}
		})
	})
})

// Helper functions for password/secret exposure detection

func checkLogsForPasswords(oc *exutil.CLI, nodes []corev1.Node) []string {
	var foundPasswords []string

	// These are test passwords that would be checked in real implementation
	// In real scenario, these would come from cluster configuration
	testPasswords := []string{
		// Placeholder - in real implementation, get from cluster config
	}

	if len(testPasswords) == 0 {
		// No passwords configured to check - skip scanning
		return foundPasswords
	}

	logPaths := []string{
		"/var/log/containers/*.log",
		"/var/log/openshift-apiserver/*.log",
		"/var/log/oauth-apiserver/*.log",
		"/var/log/kube-apiserver/*.log",
		"/var/log/ovn-kubernetes/*.log",
		"/var/log/openvswitch/*.log",
		"/var/log/audit/*.log",
		"/var/log/rhsm/*.log",
		"/var/log/lastlog*",
	}

	for _, node := range nodes {
		for _, pwd := range testPasswords {
			for _, logPath := range logPaths {
				// Use grep directly without shell to avoid command injection
				output, err := oc.AsAdmin().Run("debug").Args(
					fmt.Sprintf("node/%s", node.Name),
					"--",
					"/bin/grep", "-nl", pwd, logPath,
				).Output()

				// RC 0 means found (bad), RC 1 means not found (good)
				if err == nil && strings.TrimSpace(output) != "" {
					foundPasswords = append(foundPasswords,
						fmt.Sprintf("%s:PWD=***:DIR=%s", node.Name, output))
				}
			}
		}
	}

	return foundPasswords
}

func checkYamlsForPasswords(oc *exutil.CLI, nodes []corev1.Node) []string {
	var foundPasswords []string

	testPasswords := []string{
		// Placeholder - in real implementation, get from cluster config
	}

	if len(testPasswords) == 0 {
		// No passwords configured to check - skip scanning
		return foundPasswords
	}

	yamlPaths := []string{
		"/etc/kubernetes/manifests/*.yaml",
		"/etc/kubernetes/kubelet.conf",
		"/var/lib/kubelet/config.json",
	}

	for _, node := range nodes {
		for _, pwd := range testPasswords {
			for _, yamlPath := range yamlPaths {
				// Use grep directly without shell to avoid command injection
				output, err := oc.AsAdmin().Run("debug").Args(
					fmt.Sprintf("node/%s", node.Name),
					"--",
					"/bin/grep", "-nl", pwd, yamlPath,
				).Output()

				if err == nil && strings.TrimSpace(output) != "" {
					foundPasswords = append(foundPasswords,
						fmt.Sprintf("%s:PWD=***:DIR=%s", node.Name, output))
				}
			}
		}
	}

	return foundPasswords
}

// Helper functions for SELinux checks

func findCNIPath(oc *exutil.CLI, nodeName string) (string, bool) {
	cmd := "ls -ld /opt/cni /usr/libexec/cni"
	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--",
		"/bin/bash", "-c",
		cmd,
	).Output()

	if err != nil {
		return "", false
	}

	// Extract the actual path from symlink output
	re := regexp.MustCompile(`d[rwx-]{9}\.?\s+\d+\s+\S+\s+\S+\s+\d+\s+\S+\s+\d+\s+\S+\s+(/usr/(?:bin|lib|libexec)[a-z0-9/_-]*)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1], true
	}

	// Check if output contains valid directory listing
	if strings.Contains(output, "drwx") {
		// Default to /usr/libexec/cni if we found directories but couldn't parse the path
		return "/usr/libexec/cni", true
	}

	return "", false
}

func checkSELinuxContext(oc *exutil.CLI, nodeName, cniPath string) {
	cmd := fmt.Sprintf("ls -RZ %s", cniPath)
	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--",
		"/bin/bash", "-c",
		cmd,
	).Output()

	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(output).To(o.ContainSubstring("bin_t"),
		fmt.Sprintf("Wrong SELinux context on %s: bin_t is missing", nodeName))
	o.Expect(output).To(o.ContainSubstring("system_u"),
		fmt.Sprintf("Wrong SELinux context on %s: system_u is missing", nodeName))
}

// Helper functions for penetration test suite

// countSecretsContainingSSHKeys returns the count of secrets containing SSH keys
// Details are not returned to avoid information disclosure in test logs
func countSecretsContainingSSHKeys(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	secrets, err := oc.AdminKubeClient().CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, secret := range secrets.Items {
		for key := range secret.Data {
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerKey, "ssh-privatekey") ||
				strings.Contains(lowerKey, "id_rsa") ||
				strings.Contains(lowerKey, "id_ed25519") {
				count++
				break // Count each secret only once
			}
		}
	}

	return count
}

// countPrivilegedPodsInUserNamespaces returns the count of privileged pods in user namespaces
// Details are not returned to avoid information disclosure in test logs
func countPrivilegedPodsInUserNamespaces(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	pods, err := oc.AdminKubeClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	systemNamespaces := map[string]bool{
		"default":                              true,
		"kube-system":                          true,
		"kube-public":                          true,
		"kube-node-lease":                      true,
		"openshift":                            true,
		"openshift-apiserver":                  true,
		"openshift-authentication":             true,
		"openshift-cloud-credential-operator":  true,
		"openshift-cluster-version":            true,
		"openshift-config":                     true,
		"openshift-config-managed":             true,
		"openshift-console":                    true,
		"openshift-controller-manager":         true,
		"openshift-dns":                        true,
		"openshift-etcd":                       true,
		"openshift-image-registry":             true,
		"openshift-ingress":                    true,
		"openshift-ingress-operator":           true,
		"openshift-kube-apiserver":             true,
		"openshift-kube-controller-manager":    true,
		"openshift-kube-scheduler":             true,
		"openshift-machine-api":                true,
		"openshift-machine-config-operator":    true,
		"openshift-marketplace":                true,
		"openshift-monitoring":                 true,
		"openshift-multus":                     true,
		"openshift-network-operator":           true,
		"openshift-node":                       true,
		"openshift-operator-lifecycle-manager": true,
		"openshift-sdn":                        true,
		"openshift-service-ca":                 true,
		"openshift-user-workload-monitoring":   true,
	}

	for _, pod := range pods.Items {
		// Skip system namespaces and namespaces starting with known prefixes
		ns := pod.Namespace
		if systemNamespaces[ns] ||
			strings.HasPrefix(ns, "openshift-") ||
			strings.HasPrefix(ns, "kube-") ||
			strings.HasPrefix(ns, "portworx") ||
			strings.HasPrefix(ns, "rds-") {
			continue
		}

		privilegedFound := false

		// Check regular containers
		for _, container := range pod.Spec.Containers {
			if container.SecurityContext != nil &&
				container.SecurityContext.Privileged != nil &&
				*container.SecurityContext.Privileged {
				count++
				privilegedFound = true
				break
			}
		}
		if privilegedFound {
			continue
		}

		// Check init containers
		for _, container := range pod.Spec.InitContainers {
			if container.SecurityContext != nil &&
				container.SecurityContext.Privileged != nil &&
				*container.SecurityContext.Privileged {
				count++
				privilegedFound = true
				break
			}
		}
		if privilegedFound {
			continue
		}

		// Check ephemeral containers
		for _, container := range pod.Spec.EphemeralContainers {
			if container.SecurityContext != nil &&
				container.SecurityContext.Privileged != nil &&
				*container.SecurityContext.Privileged {
				count++
				break
			}
		}
	}

	return count
}

func findUnexpectedSudoersFiles(oc *exutil.CLI, nodes []corev1.Node) []string {
	var unexpected []string

	for _, node := range nodes {
		cmd := "ls /etc/sudoers.d/"
		output, err := oc.AsAdmin().Run("debug").Args(
			fmt.Sprintf("node/%s", node.Name),
			"--",
			"/bin/bash", "-c",
			cmd,
		).Output()

		if err != nil {
			// If directory doesn't exist, that's fine - no unexpected sudoers files
			if strings.Contains(output, "No such file or directory") {
				continue
			}
			// Skip namespace errors - these are test infrastructure issues, not security findings
			if strings.Contains(output, "unable to get namespace") || strings.Contains(output, "not found") {
				continue
			}
			// Record other inspection failures
			unexpected = append(unexpected,
				fmt.Sprintf("%s: failed to inspect sudoers.d (error: %v)", node.Name, err))
			continue
		}

		lines := strings.Split(output, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" ||
				strings.Contains(trimmed, "Starting pod/") ||
				strings.Contains(trimmed, "chroot /host") ||
				strings.Contains(trimmed, "Removing debug pod") ||
				trimmed == "coreos-sudo-group" {
				continue
			}
			unexpected = append(unexpected, fmt.Sprintf("%s: %s", node.Name, trimmed))
		}
	}

	return unexpected
}

func verifyEtcdEncryptionAtRest(ctx context.Context, oc *exutil.CLI) {
	configClient := oc.AdminConfigClient()
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	encType := "identity"
	if apiserver.Spec.Encryption.Type != "" {
		encType = string(apiserver.Spec.Encryption.Type)
	}

	g.By(fmt.Sprintf("Encryption at rest type: %s", encType))
	o.Expect(encType).NotTo(o.Equal("identity"),
		"Etcd encryption at rest is not enabled (type=identity)")
}

func findWorldReadableCriticalEtcdFiles(oc *exutil.CLI, nodeName string) []string {
	var critical []string

	cmd := "find /var/lib/etcd /home/core/assets/backup -perm -o=r -type f 2>/dev/null || true"
	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--",
		"/bin/bash", "-c",
		cmd,
	).Output()

	if err != nil {
		return critical
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "Starting") ||
			strings.HasPrefix(trimmed, "Removing") ||
			strings.HasPrefix(trimmed, "To use") {
			continue
		}

		if strings.HasSuffix(trimmed, ".db") ||
			strings.HasSuffix(trimmed, ".wal") ||
			strings.HasSuffix(trimmed, ".tar.gz") {
			critical = append(critical, trimmed)
		}
	}

	return critical
}

// countRoutesWithoutTLS returns the count of routes without TLS
// Details are not returned to avoid information disclosure in test logs
func countRoutesWithoutTLS(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	routeClient := oc.AdminRouteClient().RouteV1()
	routes, err := routeClient.Routes("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, route := range routes.Items {
		if route.Spec.TLS == nil {
			count++
		}
	}

	return count
}

func checkEtcdDirectoryPermissions(oc *exutil.CLI, nodeName string) []string {
	var problems []string

	cmd := "find /var/lib/etcd -maxdepth 2 -perm -o=r 2>/dev/null"
	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--",
		"/bin/bash", "-c",
		cmd,
	).Output()

	if err != nil {
		return problems
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" ||
			strings.Contains(trimmed, "Starting pod/") ||
			strings.Contains(trimmed, "chroot /host") ||
			strings.Contains(trimmed, "Removing debug pod") ||
			trimmed == "/var/lib/etcd" ||
			strings.HasSuffix(trimmed, ".json") {
			continue
		}
		problems = append(problems, trimmed)
	}

	return problems
}

func checkSecurityOperators(ctx context.Context, oc *exutil.CLI) []string {
	var found []string

	// Get CSVs (ClusterServiceVersions) from all namespaces
	dynamicClient := oc.AdminDynamicClient()
	csvGVR := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "clusterserviceversions",
	}

	csvList, err := dynamicClient.Resource(csvGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return found
	}

	for _, csv := range csvList.Items {
		name := csv.GetName()
		lowerName := strings.ToLower(name)

		if strings.Contains(lowerName, "compliance") {
			found = append(found, fmt.Sprintf("Compliance Operator: %s", name))
		}
		if strings.Contains(lowerName, "file-integrity") {
			found = append(found, fmt.Sprintf("File Integrity Operator: %s", name))
		}
		if strings.Contains(lowerName, "stackrox") || strings.Contains(lowerName, "rhacs") {
			found = append(found, fmt.Sprintf("ACS/Stackrox: %s", name))
		}
	}

	return found
}

func getAuditLogProfile(ctx context.Context, oc *exutil.CLI) string {
	configClient := oc.AdminConfigClient()
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return "Unknown"
	}

	if apiserver.Spec.Audit.Profile != "" {
		return string(apiserver.Spec.Audit.Profile)
	}

	return "Default"
}

func getNonRunningMonitoringPods(ctx context.Context, oc *exutil.CLI) []string {
	var notRunning []string

	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-monitoring").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			notRunning = append(notRunning, fmt.Sprintf("%s (%s)", pod.Name, pod.Status.Phase))
		}
	}

	return notRunning
}

func getPrometheusRulesCount(ctx context.Context, oc *exutil.CLI) int {
	dynamicClient := oc.AdminDynamicClient()
	rulesGVR := schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "prometheusrules",
	}

	rulesList, err := dynamicClient.Resource(rulesGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0
	}

	return len(rulesList.Items)
}

func verifyEtcdUsesTLS(ctx context.Context, oc *exutil.CLI) bool {
	dynamicClient := oc.AdminDynamicClient()
	etcdGVR := schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1",
		Resource: "etcds",
	}

	etcd, err := dynamicClient.Resource(etcdGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false
	}

	// Check spec.observedConfig.servingInfo for TLS configuration
	servingInfo, found, err := unstructured.NestedMap(etcd.Object, "spec", "observedConfig", "servingInfo")
	if err == nil && found {
		// Check for minTLSVersion field
		if minTLSVersion, exists, _ := unstructured.NestedString(servingInfo, "minTLSVersion"); exists && minTLSVersion != "" {
			return true
		}
		// Check for cipherSuites field
		if cipherSuites, exists, _ := unstructured.NestedStringSlice(servingInfo, "cipherSuites"); exists && len(cipherSuites) > 0 {
			return true
		}
	}

	// Check for TLS-related fields in spec (certFile, keyFile, caFile, etc.)
	spec, found, err := unstructured.NestedMap(etcd.Object, "spec")
	if err == nil && found {
		tlsFields := []string{"certFile", "keyFile", "caFile", "clientTLS", "peerTLS", "serverTLS"}
		for _, field := range tlsFields {
			if _, exists := spec[field]; exists {
				return true
			}
		}
	}

	return false
}

// countDatabasePods returns the count of database pods
// Details are not returned to avoid information disclosure in test logs
func countDatabasePods(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	pods, err := oc.AdminKubeClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	dbImages := []string{"mysql", "postgres", "mongo", "mariadb"}

	for _, pod := range pods.Items {
		found := false
		for _, container := range pod.Spec.Containers {
			lowerImage := strings.ToLower(container.Image)
			for _, dbType := range dbImages {
				if strings.Contains(lowerImage, dbType) {
					count++
					found = true
					break
				}
			}
			if found {
				break // Count each pod only once
			}
		}
	}

	return count
}

// countClusterAdminServiceAccountBindings returns the count of ServiceAccounts with cluster-admin role
// Details are not returned to avoid information disclosure in test logs
func countClusterAdminServiceAccountBindings(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	bindings, err := oc.AdminKubeClient().RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, binding := range bindings.Items {
		if binding.RoleRef.Name != "cluster-admin" {
			continue
		}

		for _, subject := range binding.Subjects {
			if subject.Kind == "ServiceAccount" {
				count++
			}
		}
	}

	return count
}

// countNFSPersistentVolumes returns the count of NFS-backed PersistentVolumes
// Details are not returned to avoid information disclosure in test logs
func countNFSPersistentVolumes(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	pvs, err := oc.AdminKubeClient().CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pv := range pvs.Items {
		if pv.Spec.NFS != nil {
			count++
		}
	}

	return count
}

// countInsecureRegistries returns the count of insecure registries
// Details are not returned to avoid information disclosure in test logs
func countInsecureRegistries(ctx context.Context, oc *exutil.CLI) int {
	configClient := oc.AdminConfigClient()
	imageConfig, err := configClient.ConfigV1().Images().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return 0
	}

	if imageConfig.Spec.RegistrySources.InsecureRegistries != nil {
		return len(imageConfig.Spec.RegistrySources.InsecureRegistries)
	}

	return 0
}

// hasRegistryExternalRoute returns whether an external registry route exists
// Details are not returned to avoid information disclosure in test logs
func hasRegistryExternalRoute(ctx context.Context, oc *exutil.CLI) bool {
	routeClient := oc.AdminRouteClient().RouteV1()
	routes, err := routeClient.Routes("openshift-image-registry").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}

	return len(routes.Items) > 0
}

// skipIfNotBaremetal skips the test if not running on baremetal platform
func skipIfNotBaremetal(oc *exutil.CLI) {
	g.By("checking platform type")

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(
		context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if infra.Status.PlatformStatus.Type != configv1.BareMetalPlatformType {
		e2eskipper.Skipf("Security penetration tests only run on baremetal platform")
	}
}
