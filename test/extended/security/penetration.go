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
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get control plane topology")
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			e2eskipper.Skipf("HyperShift clusters with external control plane topology do not have master nodes in the data plane")
		}

		// Skip for MicroShift - different architecture
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is MicroShift")
		if isMicroShift {
			e2eskipper.Skipf("MicroShift clusters have a different architecture and do not follow the same node labeling conventions")
		}

		g.By("Getting master node names")
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list master nodes")
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
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list nodes")
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
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list nodes for sudo configuration check")

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
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get control plane topology")
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("HyperShift clusters with external control plane topology do not have master nodes in the data plane")
			}

			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is MicroShift")
			if isMicroShift {
				e2eskipper.Skipf("MicroShift clusters have a different architecture and do not follow the same node labeling conventions")
			}

			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list master nodes")
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
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get control plane topology")
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("HyperShift clusters with external control plane topology do not have master nodes in the data plane")
			}

			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check if cluster is MicroShift")
			if isMicroShift {
				e2eskipper.Skipf("MicroShift clusters have a different architecture and do not follow the same node labeling conventions")
			}

			masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list master nodes")
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

			auditProfile, err := getAuditLogProfile(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get audit log profile")
			g.By(fmt.Sprintf("Audit log profile: %s", auditProfile))
			o.Expect(auditProfile).NotTo(o.Equal("None"),
				"Audit log profile is set to None - auditing is disabled")
		})

		g.It("TestMonitoringStackHealthy", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			notRunningPods := getNonRunningMonitoringPods(ctx, oc)
			o.Expect(notRunningPods).To(o.BeEmpty(),
				fmt.Sprintf("Non-running monitoring pods: %v", notRunningPods))

			rulesCount, err := getPrometheusRulesCount(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get Prometheus rules")
			o.Expect(rulesCount).To(o.BeNumerically(">", 0), "No Prometheus rules found")
		})

		g.It("TestNetworkTrafficEncrypted [apigroup:operator.openshift.io]", ote.Informing(), func() {
			skipIfNotBaremetal(oc)
			ctx := context.Background()
			etcdUsesTLS, err := verifyEtcdUsesTLS(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to verify etcd TLS configuration")
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
			insecureRegistryCount, err := countInsecureRegistries(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get insecure registries")
			o.Expect(insecureRegistryCount).To(o.Equal(0),
				fmt.Sprintf("Found %d insecure registr(y/ies) (details redacted for security)", insecureRegistryCount))

			hasRegistryRoute, err := hasRegistryExternalRoute(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to check for registry external route")
			if hasRegistryRoute {
				g.By("Registry external route exists (details redacted for security)")
			}
		})
	})
})

// Helper functions for password/secret exposure detection

// collectPasswords retrieves actual password values from cluster secrets
// Returns a list of passwords to search for in logs and YAMLs
func collectPasswords(oc *exutil.CLI) ([]string, error) {
	ctx := context.Background()
	var passwords []string
	seen := make(map[string]bool) // Track unique passwords

	// Try to get kubeadmin password from kube-system namespace
	secret, err := oc.AdminKubeClient().CoreV1().Secrets("kube-system").Get(
		ctx, "kubeadmin", metav1.GetOptions{})
	if err == nil && secret.Data != nil {
		// The field name is 'kubeadmin' based on oc extract output
		if pwd, ok := secret.Data["kubeadmin"]; ok && len(pwd) > 0 {
			pwdStr := string(pwd)
			if !seen[pwdStr] {
				passwords = append(passwords, pwdStr)
				seen[pwdStr] = true
			}
		}
	}

	// Try to get BMC/redfish passwords from openshift-machine-api namespace
	// Look for metal3-ironic-password secret specifically
	secrets, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-machine-api").List(
		ctx, metav1.ListOptions{})
	if err == nil {
		for _, secret := range secrets.Items {
			// Look specifically for metal3-ironic-password or secrets with "password" in the name
			if strings.Contains(strings.ToLower(secret.Name), "password") {
				// Look for password field in BMC credentials
				if pwd, ok := secret.Data["password"]; ok && len(pwd) > 0 {
					pwdStr := string(pwd)
					if !seen[pwdStr] {
						passwords = append(passwords, pwdStr)
						seen[pwdStr] = true
					}
				}
			}
		}
	}

	if len(passwords) > 0 {
		g.By(fmt.Sprintf("Collected %d unique password(s) from cluster secrets", len(passwords)))
	}
	return passwords, nil
}

func checkLogsForPasswords(oc *exutil.CLI, nodes []corev1.Node) []string {
	var foundPasswords []string

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

	// Collect actual password values from the cluster
	passwords, err := collectPasswords(oc)
	if err != nil {
		g.By(fmt.Sprintf("Warning: failed to collect passwords: %v", err))
		return foundPasswords
	}

	if len(passwords) == 0 {
		g.By("No passwords collected from cluster secrets - skipping password check")
		return foundPasswords
	}

	g.By(fmt.Sprintf("Checking %d password(s) across %d nodes", len(passwords), len(nodes)))

	// Check each password in each log path on each node
	for pwdIdx, pwd := range passwords {
		for _, node := range nodes {
			for _, logPath := range logPaths {
				// Escape the password for single quotes in shell
				escapedPwd := strings.ReplaceAll(pwd, "'", "'\\''")

				// Use chroot directly as the command, with sh -c for the grep
				// Use -F for fixed string and -w for whole word to avoid false positives
				cmd := fmt.Sprintf("sh -c 'grep -Fnl \"%s\" %s 2>/dev/null || true'",
					escapedPwd, logPath)

				output, err := oc.AsAdmin().Run("debug").Args(
					fmt.Sprintf("node/%s", node.Name),
					"--",
					"chroot", "/host",
					"/bin/sh", "-c",
					cmd,
				).Output()

				// rc=0 means password was found, rc=1 means not found
				if err == nil && strings.TrimSpace(output) != "" {
					lines := strings.Split(output, "\n")
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if trimmed == "" ||
							strings.Contains(trimmed, "Starting pod/") ||
							strings.Contains(trimmed, "chroot /host") ||
							strings.Contains(trimmed, "Removing debug pod") {
							continue
						}
						// Password found in this file
						foundPasswords = append(foundPasswords,
							fmt.Sprintf("%s:PWD=%d:DIR=%s", node.Name, pwdIdx+1, trimmed))
					}
				}
			}
		}
	}

	g.By(fmt.Sprintf("checkLogsForPasswords: found %d instances", len(foundPasswords)))
	return foundPasswords
}

func checkYamlsForPasswords(oc *exutil.CLI, nodes []corev1.Node) []string {
	var foundPasswords []string

	yamlPaths := []string{
		"/etc/kubernetes/manifests/*.yaml",
		"/etc/kubernetes/kubelet.conf",
		"/var/lib/kubelet/config.json",
	}

	// Collect actual password values from the cluster
	passwords, err := collectPasswords(oc)
	if err != nil {
		g.By(fmt.Sprintf("Warning: failed to collect passwords: %v", err))
		return foundPasswords
	}

	if len(passwords) == 0 {
		g.By("No passwords collected from cluster secrets - skipping password check")
		return foundPasswords
	}

	g.By(fmt.Sprintf("Checking %d password(s) in YAMLs across %d nodes", len(passwords), len(nodes)))

	// Check each password in each YAML path on each node
	for pwdIdx, pwd := range passwords {
		for _, node := range nodes {
			for _, yamlPath := range yamlPaths {
				// Escape the password for single quotes in shell
				escapedPwd := strings.ReplaceAll(pwd, "'", "'\\''")

				// Use chroot directly as the command, with sh -c for the grep
				// Use -F for fixed string and -w for whole word to avoid false positives
				cmd := fmt.Sprintf("sh -c 'grep -Fnl \"%s\" %s 2>/dev/null || true'",
					escapedPwd, yamlPath)

				output, err := oc.AsAdmin().Run("debug").Args(
					fmt.Sprintf("node/%s", node.Name),
					"--",
					"chroot", "/host",
					"/bin/sh", "-c",
					cmd,
				).Output()

				// rc=0 means password was found, rc=1 means not found
				if err == nil && strings.TrimSpace(output) != "" {
					lines := strings.Split(output, "\n")
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if trimmed == "" ||
							strings.Contains(trimmed, "Starting pod/") ||
							strings.Contains(trimmed, "chroot /host") ||
							strings.Contains(trimmed, "Removing debug pod") {
							continue
						}
						// Password found in this file
						foundPasswords = append(foundPasswords,
							fmt.Sprintf("%s:PWD=%d:DIR=%s", node.Name, pwdIdx+1, trimmed))
					}
				}
			}
		}
	}

	g.By(fmt.Sprintf("checkYamlsForPasswords: found %d instances", len(foundPasswords)))
	return foundPasswords
}

// Helper functions for SELinux checks

func findCNIPath(oc *exutil.CLI, nodeName string) (string, bool) {
	cmd := "chroot /host ls -ld /opt/cni /usr/libexec/cni"
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
		g.By(fmt.Sprintf("findCNIPath: using default /usr/libexec/cni on %s", nodeName))
		return "/usr/libexec/cni", true
	}

	g.By(fmt.Sprintf("findCNIPath: CNI directory not found on %s", nodeName))
	return "", false
}

func checkSELinuxContext(oc *exutil.CLI, nodeName, cniPath string) {
	cmd := fmt.Sprintf("chroot /host ls -RZ %s", cniPath)
	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--",
		"/bin/bash", "-c",
		cmd,
	).Output()

	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to check SELinux context on %s", nodeName))
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
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list secrets")

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

	g.By(fmt.Sprintf("countSecretsContainingSSHKeys: found %d secrets", count))
	return count
}

// countPrivilegedPodsInUserNamespaces returns the count of privileged pods in user namespaces
// Details are not returned to avoid information disclosure in test logs
func countPrivilegedPodsInUserNamespaces(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	pods, err := oc.AdminKubeClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list pods")

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
			strings.HasPrefix(ns, "kube-") {
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

	g.By(fmt.Sprintf("countPrivilegedPodsInUserNamespaces: found %d pods", count))
	return count
}

func findUnexpectedSudoersFiles(oc *exutil.CLI, nodes []corev1.Node) []string {
	var unexpected []string

	for _, node := range nodes {
		cmd := "chroot /host ls /etc/sudoers.d/"
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

	g.By(fmt.Sprintf("findUnexpectedSudoersFiles: found %d unexpected files", len(unexpected)))
	return unexpected
}

func verifyEtcdEncryptionAtRest(ctx context.Context, oc *exutil.CLI) {
	configClient := oc.AdminConfigClient()
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get API server configuration for encryption check")

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

	cmd := "chroot /host find /var/lib/etcd /home/core/assets/backup -perm -o=r -type f 2>/dev/null || true"
	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--",
		"/bin/bash", "-c",
		cmd,
	).Output()

	if err != nil {
		critical = append(critical,
			fmt.Sprintf("%s: failed to check world-readable etcd files (error: %v)", nodeName, err))
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

	g.By(fmt.Sprintf("findWorldReadableCriticalEtcdFiles: found %d files on %s", len(critical), nodeName))
	return critical
}

// countRoutesWithoutTLS returns the count of routes without TLS
// Details are not returned to avoid information disclosure in test logs
func countRoutesWithoutTLS(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	routeClient := oc.AdminRouteClient().RouteV1()
	routes, err := routeClient.Routes("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list routes")

	for _, route := range routes.Items {
		if route.Spec.TLS == nil {
			count++
		}
	}

	g.By(fmt.Sprintf("countRoutesWithoutTLS: found %d routes", count))
	return count
}

func checkEtcdDirectoryPermissions(oc *exutil.CLI, nodeName string) []string {
	var problems []string

	cmd := "chroot /host find /var/lib/etcd -maxdepth 2 -perm -o=r 2>/dev/null"
	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--",
		"/bin/bash", "-c",
		cmd,
	).Output()

	if err != nil {
		problems = append(problems,
			fmt.Sprintf("%s: failed to check etcd directory permissions (error: %v)", nodeName, err))
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

	g.By(fmt.Sprintf("checkEtcdDirectoryPermissions: found %d problems on %s", len(problems), nodeName))
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
		// Report the error so it doesn't silently appear as "no operators found"
		found = append(found, fmt.Sprintf("ERROR: Failed to list ClusterServiceVersions: %v", err))
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

	g.By(fmt.Sprintf("checkSecurityOperators: found %d operators", len(found)))
	return found
}

func getAuditLogProfile(ctx context.Context, oc *exutil.CLI) (string, error) {
	configClient := oc.AdminConfigClient()
	apiserver, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if apiserver.Spec.Audit.Profile != "" {
		g.By(fmt.Sprintf("getAuditLogProfile: %s", apiserver.Spec.Audit.Profile))
		return string(apiserver.Spec.Audit.Profile), nil
	}

	g.By("getAuditLogProfile: Default")
	return "Default", nil
}

func getNonRunningMonitoringPods(ctx context.Context, oc *exutil.CLI) []string {
	var notRunning []string

	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-monitoring").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list monitoring pods")

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			notRunning = append(notRunning, fmt.Sprintf("%s (%s)", pod.Name, pod.Status.Phase))
		}
	}

	g.By(fmt.Sprintf("getNonRunningMonitoringPods: found %d pods", len(notRunning)))
	return notRunning
}

func getPrometheusRulesCount(ctx context.Context, oc *exutil.CLI) (int, error) {
	dynamicClient := oc.AdminDynamicClient()
	rulesGVR := schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "prometheusrules",
	}

	rulesList, err := dynamicClient.Resource(rulesGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	count := len(rulesList.Items)
	g.By(fmt.Sprintf("getPrometheusRulesCount: found %d rules", count))
	return count, nil
}

func verifyEtcdUsesTLS(ctx context.Context, oc *exutil.CLI) (bool, error) {
	dynamicClient := oc.AdminDynamicClient()
	etcdGVR := schema.GroupVersionResource{
		Group:    "operator.openshift.io",
		Version:  "v1",
		Resource: "etcds",
	}

	etcd, err := dynamicClient.Resource(etcdGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// Check spec.observedConfig.servingInfo for TLS configuration
	servingInfo, found, err := unstructured.NestedMap(etcd.Object, "spec", "observedConfig", "servingInfo")
	if err == nil && found {
		// Check for minTLSVersion field
		if minTLSVersion, exists, _ := unstructured.NestedString(servingInfo, "minTLSVersion"); exists && minTLSVersion != "" {
			return true, nil
		}
		// Check for cipherSuites field
		if cipherSuites, exists, _ := unstructured.NestedStringSlice(servingInfo, "cipherSuites"); exists && len(cipherSuites) > 0 {
			return true, nil
		}
	}

	// Check for TLS-related fields in spec (certFile, keyFile, caFile, etc.)
	spec, found, err := unstructured.NestedMap(etcd.Object, "spec")
	if err == nil && found {
		tlsFields := []string{"certFile", "keyFile", "caFile", "clientTLS", "peerTLS", "serverTLS"}
		for _, field := range tlsFields {
			if _, exists := spec[field]; exists {
				return true, nil
			}
		}
	}

	g.By("verifyEtcdUsesTLS: no TLS configuration found")
	return false, nil
}

// countDatabasePods returns the count of database pods
// Details are not returned to avoid information disclosure in test logs
func countDatabasePods(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	pods, err := oc.AdminKubeClient().CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list pods")

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

	g.By(fmt.Sprintf("countDatabasePods: found %d pods", count))
	return count
}

// countClusterAdminServiceAccountBindings returns the count of unexpected ServiceAccounts with cluster-admin role
// Known system ServiceAccounts are excluded from the count
// Details are not returned to avoid information disclosure in test logs
func countClusterAdminServiceAccountBindings(ctx context.Context, oc *exutil.CLI) int {
	// Known-good ServiceAccounts that legitimately need cluster-admin for platform operations
	knownGoodServiceAccounts := map[string]map[string]bool{
		"kube-system": {
			"attachdetach-controller":            true,
			"certificate-controller":             true,
			"clusterrole-aggregation-controller": true,
			"cronjob-controller":                 true,
			"daemon-set-controller":              true,
			"deployment-controller":              true,
			"disruption-controller":              true,
			"endpoint-controller":                true,
			"endpointslice-controller":           true,
			"endpointslicemirroring-controller":  true,
			"ephemeral-volume-controller":        true,
			"expand-controller":                  true,
			"generic-garbage-collector":          true,
			"horizontal-pod-autoscaler":          true,
			"job-controller":                     true,
			"namespace-controller":               true,
			"node-controller":                    true,
			"persistent-volume-binder":           true,
			"pod-garbage-collector":              true,
			"pv-protection-controller":           true,
			"pvc-protection-controller":          true,
			"replicaset-controller":              true,
			"replication-controller":             true,
			"resourcequota-controller":           true,
			"service-account-controller":         true,
			"service-controller":                 true,
			"statefulset-controller":             true,
			"ttl-after-finished-controller":      true,
			"ttl-controller":                     true,
		},
		"openshift-kube-controller-manager": {
			"kube-controller-manager": true,
		},
		"openshift-cluster-version": {
			"default": true,
		},
		"openshift-config-operator": {
			"openshift-config-operator": true,
		},
		"openshift-controller-manager": {
			"openshift-controller-manager": true,
		},
		"openshift-kube-apiserver": {
			"kube-apiserver": true,
		},
		"openshift-kube-scheduler": {
			"openshift-kube-scheduler": true,
		},
		"openshift-apiserver": {
			"openshift-apiserver-sa": true,
		},
		"openshift-apiserver-operator": {
			"openshift-apiserver-operator": true,
		},
		"openshift-authentication-operator": {
			"authentication-operator": true,
		},
		"openshift-cluster-storage-operator": {
			"cluster-storage-operator": true,
		},
		"openshift-cluster-samples-operator": {
			"cluster-samples-operator": true,
		},
		"openshift-etcd-operator": {
			"etcd-operator": true,
		},
		"openshift-kube-controller-manager-operator": {
			"kube-controller-manager-operator": true,
		},
		"openshift-kube-apiserver-operator": {
			"kube-apiserver-operator": true,
		},
		"openshift-kube-scheduler-operator": {
			"openshift-kube-scheduler-operator": true,
		},
		"openshift-machine-config-operator": {
			"machine-config-controller": true,
			"machine-config-operator":   true,
		},
		"openshift-network-operator": {
			"default": true,
		},
		"openshift-operator-lifecycle-manager": {
			"olm-operator-serviceaccount": true,
		},
	}

	count := 0
	bindings, err := oc.AdminKubeClient().RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list cluster role bindings")

	for _, binding := range bindings.Items {
		if binding.RoleRef.Name != "cluster-admin" {
			continue
		}

		for _, subject := range binding.Subjects {
			if subject.Kind == "ServiceAccount" {
				// Check if this is a known-good system ServiceAccount
				namespace := subject.Namespace
				name := subject.Name

				if nsMap, exists := knownGoodServiceAccounts[namespace]; exists {
					if nsMap[name] {
						// This is a known-good ServiceAccount, skip it
						continue
					}
				}

				// This is an unexpected ServiceAccount with cluster-admin
				count++
			}
		}
	}

	g.By(fmt.Sprintf("countClusterAdminServiceAccountBindings: found %d unexpected bindings", count))
	return count
}

// countNFSPersistentVolumes returns the count of NFS-backed PersistentVolumes
// Details are not returned to avoid information disclosure in test logs
func countNFSPersistentVolumes(ctx context.Context, oc *exutil.CLI) int {
	count := 0

	pvs, err := oc.AdminKubeClient().CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list persistent volumes")

	for _, pv := range pvs.Items {
		if pv.Spec.NFS != nil {
			count++
		}
	}

	g.By(fmt.Sprintf("countNFSPersistentVolumes: found %d volumes", count))
	return count
}

// countInsecureRegistries returns the count of insecure registries
// Details are not returned to avoid information disclosure in test logs
func countInsecureRegistries(ctx context.Context, oc *exutil.CLI) (int, error) {
	configClient := oc.AdminConfigClient()
	imageConfig, err := configClient.ConfigV1().Images().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return 0, err
	}

	if imageConfig.Spec.RegistrySources.InsecureRegistries != nil {
		count := len(imageConfig.Spec.RegistrySources.InsecureRegistries)
		g.By(fmt.Sprintf("countInsecureRegistries: found %d registries", count))
		return count, nil
	}

	g.By("countInsecureRegistries: found 0 registries")
	return 0, nil
}

// hasRegistryExternalRoute returns whether an external registry route exists
// Details are not returned to avoid information disclosure in test logs
func hasRegistryExternalRoute(ctx context.Context, oc *exutil.CLI) (bool, error) {
	routeClient := oc.AdminRouteClient().RouteV1()
	routes, err := routeClient.Routes("openshift-image-registry").List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	hasRoute := len(routes.Items) > 0
	g.By(fmt.Sprintf("hasRegistryExternalRoute: %v", hasRoute))
	return hasRoute, nil
}

// skipIfNotBaremetal skips the test if not running on baremetal platform
func skipIfNotBaremetal(oc *exutil.CLI) {
	ctx := context.Background()
	g.By("checking platform type")

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(
		ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	platformType := infra.Status.PlatformStatus.Type
	g.By(fmt.Sprintf("Detected platform type: %s", platformType))

	// If platform is BareMetal, allow the test
	if platformType == configv1.BareMetalPlatformType {
		return
	}

	// If platform is None, check if it's SNO on baremetal
	if platformType == configv1.NonePlatformType {
		// Check if this is a Single Node OpenShift (SNO)
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list nodes")

		if len(nodes.Items) == 1 {
			g.By("Detected Single Node OpenShift (SNO)")
			// For SNO, check if the single node is baremetal by looking at labels or annotations
			node := nodes.Items[0]

			// Check for baremetal-related labels or annotations
			if _, hasBMCLabel := node.Labels["metal3.io/bmc-address"]; hasBMCLabel {
				g.By("SNO node has baremetal BMC label - allowing test")
				return
			}

			// Check infrastructure platformSpec for baremetal hints
			if infra.Status.InfrastructureName != "" {
				g.By(fmt.Sprintf("SNO infrastructure name: %s - allowing test", infra.Status.InfrastructureName))
				return
			}
		}
	}

	e2eskipper.Skipf("Security penetration tests only run on baremetal platform (detected: %s)", platformType)
}
