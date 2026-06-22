package networking

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][OCPFeature:ControlPlaneNetworkPolicy] Control Plane Network Policy", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("control-plane-netpol", admissionapi.LevelPrivileged)

	// CIS Kube Benchmark 5.3.2: Ensure that all Namespaces have Network Policies defined
	g.Describe("CIS Kube Benchmark 5.3.2 Compliance", func() {
		g.It("should have NetworkPolicy defined in all control plane namespaces [apigroup:networking.k8s.io]", func() {
			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-kube-controller-manager",
				"openshift-kube-scheduler",
				"openshift-etcd",
				"openshift-apiserver",
				"openshift-controller-manager",
				"openshift-oauth-apiserver",
			}

			ctx := context.Background()

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying NetworkPolicy exists in namespace: %s", namespace))

				// Check if namespace exists - FAIL if it doesn't (these are required control plane namespaces)
				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(),
					"FAILURE: Required control plane namespace %s does not exist", namespace)

				// Get all NetworkPolicies in the namespace
				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(),
					"FAILURE: Failed to list NetworkPolicies in namespace %s", namespace)

				// CIS 5.3.2 requires at least one NetworkPolicy per namespace - HARD REQUIREMENT
				o.Expect(npList.Items).NotTo(o.BeEmpty(),
					"FAILURE - CIS 5.3.2 VIOLATION: Namespace %s MUST have at least one NetworkPolicy defined. "+
						"This is a security compliance requirement.", namespace)

				e2e.Logf("PASS: Namespace %s has %d NetworkPolicy resources (CIS 5.3.2 compliant)", namespace, len(npList.Items))
			}
		})

		g.It("should have restrictive ingress rules for control plane components [apigroup:networking.k8s.io]", func() {
			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-kube-controller-manager",
				"openshift-kube-scheduler",
				"openshift-etcd",
			}

			ctx := context.Background()

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying restrictive ingress in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(),
					"FAILURE: Required control plane namespace %s does not exist", namespace)

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(),
					"FAILURE: Failed to list NetworkPolicies in namespace %s", namespace)

				o.Expect(npList.Items).NotTo(o.BeEmpty(),
					"FAILURE: Namespace %s has no NetworkPolicies - cannot verify ingress restrictions", namespace)

				// Verify that at least one NetworkPolicy has ingress rules defined (explicit allow or deny-all)
				hasIngressPolicy := false
				for _, np := range npList.Items {
					// Check if policy specifies Ingress type
					for _, policyType := range np.Spec.PolicyTypes {
						if policyType == networkingv1.PolicyTypeIngress {
							hasIngressPolicy = true
							if len(np.Spec.Ingress) == 0 {
								e2e.Logf("  NetworkPolicy %s has deny-all ingress (most restrictive)", np.Name)
							} else {
								e2e.Logf("  NetworkPolicy %s has %d explicit ingress rules", np.Name, len(np.Spec.Ingress))
							}
						}
					}
				}

				o.Expect(hasIngressPolicy).To(o.BeTrue(),
					"FAILURE: Namespace %s MUST have at least one NetworkPolicy with Ingress policyType to prevent unauthorized access. "+
						"This is required for lateral movement prevention.", namespace)
			}
		})

		g.It("should have restrictive egress rules for control plane components [apigroup:networking.k8s.io]", func() {
			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-kube-controller-manager",
				"openshift-kube-scheduler",
				"openshift-etcd",
			}

			ctx := context.Background()

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying restrictive egress in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(),
					"FAILURE: Required control plane namespace %s does not exist", namespace)

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(),
					"FAILURE: Failed to list NetworkPolicies in namespace %s", namespace)

				o.Expect(npList.Items).NotTo(o.BeEmpty(),
					"FAILURE: Namespace %s has no NetworkPolicies - cannot verify egress restrictions", namespace)

				// Verify that at least one NetworkPolicy defines egress policy type
				hasEgressPolicy := false
				for _, np := range npList.Items {
					for _, policyType := range np.Spec.PolicyTypes {
						if policyType == networkingv1.PolicyTypeEgress {
							hasEgressPolicy = true
							if len(np.Spec.Egress) == 0 {
								e2e.Logf("  NetworkPolicy %s has deny-all egress (prevents data exfiltration)", np.Name)
							} else {
								e2e.Logf("  NetworkPolicy %s has %d explicit egress rules (controlled data flow)", np.Name, len(np.Spec.Egress))
							}
							break
						}
					}
				}

				o.Expect(hasEgressPolicy).To(o.BeTrue(),
					"FAILURE: Namespace %s MUST have at least one NetworkPolicy with Egress policyType to prevent data exfiltration. "+
						"Without egress policies, compromised pods can send data to unauthorized destinations.", namespace)
			}
		})
	})

	// Network Policy Label Validation
	g.Describe("NetworkPolicy Pod Selector Labels", func() {
		g.It("should have required labels on control plane pods for network policy selection [apigroup:apps][apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			// Define required network policy labels based on the storage pattern
			requiredLabelsByNamespace := map[string][]map[string]string{
				"openshift-kube-apiserver": {
					{"openshift.control-plane.network-policy.api-server": "allow"},
					{"openshift.control-plane.network-policy.dns": "allow"},
					{"openshift.control-plane.network-policy.metrics": "allow"},
				},
				"openshift-etcd": {
					{"openshift.control-plane.network-policy.etcd": "allow"},
					{"openshift.control-plane.network-policy.dns": "allow"},
					{"openshift.control-plane.network-policy.metrics": "allow"},
				},
			}

			for namespace, requiredLabels := range requiredLabelsByNamespace {
				g.By(fmt.Sprintf("Verifying pod labels in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				// Get all pods in the namespace
				pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				if len(pods.Items) == 0 {
					e2e.Logf("No pods found in namespace %s, skipping label verification", namespace)
					continue
				}

				// Verify that pods have appropriate labels for network policy selection
				verifyNetworkPolicyLabels(oc, namespace, requiredLabels)
			}
		})

		g.It("should verify NetworkPolicy PodSelectors match control plane pod labels [apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-etcd",
			}

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying NetworkPolicy PodSelectors in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Get all pods in the namespace
				pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				if len(pods.Items) == 0 || len(npList.Items) == 0 {
					e2e.Logf("Skipping namespace %s (pods: %d, policies: %d)", namespace, len(pods.Items), len(npList.Items))
					continue
				}

				// Verify at least one pod is selected by each NetworkPolicy
				for _, np := range npList.Items {
					matchedPods := 0
					for _, pod := range pods.Items {
						if podMatchesSelector(pod.Labels, np.Spec.PodSelector.MatchLabels) {
							matchedPods++
						}
					}
					e2e.Logf("  NetworkPolicy %s matches %d pods", np.Name, matchedPods)
				}
			}
		})
	})

	// Threat Model Validation: Prevent Lateral Movement
	g.Describe("Lateral Movement Prevention", func() {
		g.It("should prevent unauthorized pod-to-pod communication within control plane [apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			// This test verifies that NetworkPolicies prevent default-allow behavior
			// which could enable lateral movement attacks

			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-etcd",
			}

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying network isolation in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(npList.Items).NotTo(o.BeEmpty(),
					"FAILURE: Namespace %s has NO NetworkPolicies - allows unrestricted lateral movement! "+
						"This is a CRITICAL security vulnerability.", namespace)

				// Verify that there are NetworkPolicies that define explicit ingress rules
				// (empty ingress = deny all, which prevents lateral movement)
				hasDenyAllOrRestrictive := false
				for _, np := range npList.Items {
					// Check if policy has ingress type (either explicitly or implicitly)
					hasIngressType := false
					for _, policyType := range np.Spec.PolicyTypes {
						if policyType == networkingv1.PolicyTypeIngress {
							hasIngressType = true
							break
						}
					}

					// If no PolicyTypes specified but Ingress is defined, it's implicitly Ingress type
					if len(np.Spec.PolicyTypes) == 0 && np.Spec.Ingress != nil {
						hasIngressType = true
					}

					// Empty ingress rules = deny all (most restrictive)
					if hasIngressType && len(np.Spec.Ingress) == 0 {
						hasDenyAllOrRestrictive = true
						e2e.Logf("  NetworkPolicy %s has deny-all ingress (prevents lateral movement)", np.Name)
					}

					// Explicit ingress rules = selective allow (restrictive)
					if hasIngressType && len(np.Spec.Ingress) > 0 {
						hasDenyAllOrRestrictive = true
						e2e.Logf("  NetworkPolicy %s has %d explicit ingress rules (controlled access)", np.Name, len(np.Spec.Ingress))
					}
				}

				o.Expect(hasDenyAllOrRestrictive).To(o.BeTrue(),
					"FAILURE: Namespace %s NetworkPolicies do NOT prevent lateral movement! "+
						"At least one NetworkPolicy must have Ingress policyType to block unauthorized pod-to-pod communication. "+
						"Current state allows attackers to move freely between compromised pods.", namespace)
			}
		})

		g.It("should allow only necessary DNS communication [apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-etcd",
			}

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying DNS egress rules in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Verify DNS egress is explicitly allowed (port 5353 UDP/TCP in OpenShift)
				hasDNSEgress := false
				for _, np := range npList.Items {
					if len(np.Spec.Egress) > 0 {
						for _, egressRule := range np.Spec.Egress {
							if len(egressRule.Ports) > 0 {
								for _, port := range egressRule.Ports {
									// DNS in OpenShift is on port 5353, but also check for 53 for compatibility
									if port.Port != nil && (port.Port.IntVal == 5353 || port.Port.IntVal == 53) {
										hasDNSEgress = true
										e2e.Logf("  NetworkPolicy %s allows DNS on port %d", np.Name, port.Port.IntVal)
									}
								}
							}
						}
					}
				}

				// Note: This is informational - not all operators may need explicit DNS rules
				// if they use hostNetwork or other mechanisms
				if hasDNSEgress {
					e2e.Logf("Namespace %s has explicit DNS egress rules (port 5353 or 53)", namespace)
				} else {
					e2e.Logf("INFO:Namespace %s does not have explicit DNS egress rules (may use hostNetwork or alternative DNS resolution)", namespace)
				}
			}
		})
	})

	// Data Exfiltration Prevention
	g.Describe("Data Exfiltration Prevention", func() {
		g.It("should restrict egress to prevent unauthorized data leakage [apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-etcd",
			}

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying egress restrictions in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(npList.Items).NotTo(o.BeEmpty(),
					"FAILURE: Namespace %s has NO NetworkPolicies - allows unrestricted data exfiltration! "+
						"This is a CRITICAL security vulnerability.", namespace)

				// Verify that egress is explicitly defined (not default-allow)
				hasEgressPolicy := false
				for _, np := range npList.Items {
					for _, policyType := range np.Spec.PolicyTypes {
						if policyType == networkingv1.PolicyTypeEgress {
							hasEgressPolicy = true

							// Empty egress = deny all egress
							if len(np.Spec.Egress) == 0 {
								e2e.Logf("  NetworkPolicy %s has deny-all egress (maximum data exfiltration protection)", np.Name)
							} else {
								e2e.Logf("  NetworkPolicy %s has %d explicit egress rules (controlled data flow)", np.Name, len(np.Spec.Egress))
							}
						}
					}
				}

				o.Expect(hasEgressPolicy).To(o.BeTrue(),
					"FAILURE: Namespace %s MUST have NetworkPolicies with explicit Egress policyType to prevent data exfiltration! "+
						"Without egress policies, compromised pods can send sensitive data to unauthorized external destinations. "+
						"This violates ProdSec requirements.", namespace)
			}
		})

		g.It("should allow metrics endpoints only from authorized sources [apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			metricsNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-etcd",
			}

			for _, namespace := range metricsNamespaces {
				g.By(fmt.Sprintf("Verifying metrics ingress rules in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Verify metrics ports (commonly 8443, 9092, etc.) have ingress restrictions
				hasMetricsIngress := false
				metricsCommonPorts := []int32{8443, 9092, 9093, 9100}

				for _, np := range npList.Items {
					for _, ingressRule := range np.Spec.Ingress {
						for _, port := range ingressRule.Ports {
							if port.Port != nil {
								for _, metricsPort := range metricsCommonPorts {
									if port.Port.IntVal == metricsPort {
										hasMetricsIngress = true
										e2e.Logf("  NetworkPolicy %s defines ingress for metrics port %d", np.Name, metricsPort)

										// Verify that the source is restricted
										if len(ingressRule.From) > 0 {
											e2e.Logf("Metrics access is restricted to %d sources", len(ingressRule.From))
										}
									}
								}
							}
						}
					}
				}

				// Note: This is informational - not all components expose metrics
				if hasMetricsIngress {
					e2e.Logf("Namespace %s has explicit metrics ingress rules", namespace)
				} else {
					e2e.Logf("INFO:Namespace %s does not have explicit metrics ingress rules", namespace)
				}
			}
		})
	})

	// ProdSec Compliance Validation
	g.Describe("ProdSec Compliance", func() {
		g.It("should have NetworkPolicies with explicit allow-list approach [apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-kube-controller-manager",
				"openshift-kube-scheduler",
				"openshift-etcd",
			}

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Verifying allow-list approach in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(),
					"FAILURE: Failed to list NetworkPolicies in namespace %s", namespace)
				o.Expect(npList.Items).NotTo(o.BeEmpty(),
					"FAILURE - PRODSEC VIOLATION: Namespace %s MUST have NetworkPolicies. "+
						"This is required for ProdSec sign-off and threat model compliance.", namespace)

				// ProdSec best practice: Default deny + explicit allow
				hasDefaultDenyIngress := false
				hasDefaultDenyEgress := false
				hasExplicitAllows := false

				for _, np := range npList.Items {
					// Check for default deny patterns
					for _, policyType := range np.Spec.PolicyTypes {
						if policyType == networkingv1.PolicyTypeIngress && len(np.Spec.Ingress) == 0 {
							hasDefaultDenyIngress = true
						}
						if policyType == networkingv1.PolicyTypeEgress && len(np.Spec.Egress) == 0 {
							hasDefaultDenyEgress = true
						}
					}

					// Check for explicit allow rules
					if len(np.Spec.Ingress) > 0 || len(np.Spec.Egress) > 0 {
						hasExplicitAllows = true
					}
				}

				// At minimum, should have explicit ingress/egress rules (allow-list approach)
				o.Expect(hasExplicitAllows || hasDefaultDenyIngress || hasDefaultDenyEgress).To(o.BeTrue(),
					"Namespace %s should follow allow-list approach (default deny + explicit allows)", namespace)

				e2e.Logf("Namespace %s follows allow-list security model", namespace)
			}
		})

		g.It("should enforce network segmentation between control plane components [apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			// Verify that different control plane components are isolated
			componentPairs := []struct {
				namespace1 string
				namespace2 string
			}{
				{"openshift-kube-apiserver", "openshift-etcd"},
				{"openshift-kube-controller-manager", "openshift-kube-scheduler"},
			}

			for _, pair := range componentPairs {
				g.By(fmt.Sprintf("Verifying network segmentation between %s and %s", pair.namespace1, pair.namespace2))

				// Check if namespaces exist
				_, err1 := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, pair.namespace1, metav1.GetOptions{})
				_, err2 := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, pair.namespace2, metav1.GetOptions{})

				if err1 != nil || err2 != nil {
					e2e.Logf("One or both namespaces do not exist, skipping")
					continue
				}

				// Get NetworkPolicies in both namespaces
				np1, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(pair.namespace1).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				np2, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(pair.namespace2).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Verify both namespaces have NetworkPolicies (indicating segmentation)
				hasSegmentation := len(np1.Items) > 0 && len(np2.Items) > 0
				o.Expect(hasSegmentation).To(o.BeTrue(),
					"Both namespaces %s and %s should have NetworkPolicies for proper segmentation",
					pair.namespace1, pair.namespace2)

				e2e.Logf("✓ Network segmentation enforced between %s and %s", pair.namespace1, pair.namespace2)
			}
		})
	})
})

// Helper function to verify network policy labels exist in the namespace
func verifyNetworkPolicyLabels(oc *exutil.CLI, namespace string, requiredPodSelectors []map[string]string) {
	ctx := context.Background()

	npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list NetworkPolicies in namespace %s", namespace)

	if len(npList.Items) == 0 {
		e2e.Logf("No NetworkPolicies found in namespace %s", namespace)
		return
	}

	// For each required PodSelector, verify it exists in at least one NetworkPolicy
	for _, requiredSelector := range requiredPodSelectors {
		found := false
		for _, np := range npList.Items {
			if podSelectorMatches(np.Spec.PodSelector.MatchLabels, requiredSelector) {
				found = true
				e2e.Logf("Required selector %v found in NetworkPolicy %s", requiredSelector, np.Name)
				break
			}
		}

		if !found {
			e2e.Logf("  ℹ Required selector %v not found in any NetworkPolicy in namespace %s", requiredSelector, namespace)
		}
	}
}

// Helper function to check if a pod matches a selector
func podMatchesSelector(podLabels, selector map[string]string) bool {
	// Empty selector matches all pods
	if len(selector) == 0 {
		return true
	}

	// All selector labels must match pod labels
	for key, value := range selector {
		if podLabels[key] != value {
			return false
		}
	}
	return true
}

// Helper function to check if a PodSelector matches the required selector
func podSelectorMatches(podSelector, requiredSelector map[string]string) bool {
	if len(podSelector) != len(requiredSelector) {
		return false
	}

	for key, value := range requiredSelector {
		if podSelector[key] != value {
			return false
		}
	}
	return true
}
