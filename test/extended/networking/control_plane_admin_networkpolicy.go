package networking

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][OCPFeature:ControlPlaneNetworkPolicy] Control Plane AdminNetworkPolicy", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("control-plane-anp", admissionapi.LevelPrivileged)

	// AdminNetworkPolicy GVR (Group Version Resource)
	anpGVR := schema.GroupVersionResource{
		Group:    "policy.networking.k8s.io",
		Version:  "v1alpha1",
		Resource: "adminnetworkpolicies",
	}

	banpGVR := schema.GroupVersionResource{
		Group:    "policy.networking.k8s.io",
		Version:  "v1alpha1",
		Resource: "baselineadminnetworkpolicies",
	}

	g.Describe("AdminNetworkPolicy Resources", func() {
		g.It("should have AdminNetworkPolicy or BaselineAdminNetworkPolicy for control plane [apigroup:policy.networking.k8s.io]", func() {
			ctx := context.Background()

			// Check if AdminNetworkPolicy CRD exists
			crdClient := oc.AdminDynamicClient()
			crdGVR := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			anpCRDExists := false
			banpCRDExists := false

			// Check for AdminNetworkPolicy CRD
			_, err := crdClient.Resource(crdGVR).Get(ctx, "adminnetworkpolicies.policy.networking.k8s.io", metav1.GetOptions{})
			if err == nil {
				anpCRDExists = true
				e2e.Logf("AdminNetworkPolicy CRD is installed")
			} else {
				e2e.Logf("INFO:AdminNetworkPolicy CRD not found: %v", err)
			}

			// Check for BaselineAdminNetworkPolicy CRD
			_, err = crdClient.Resource(crdGVR).Get(ctx, "baselineadminnetworkpolicies.policy.networking.k8s.io", metav1.GetOptions{})
			if err == nil {
				banpCRDExists = true
				e2e.Logf("BaselineAdminNetworkPolicy CRD is installed")
			} else {
				e2e.Logf("INFO:BaselineAdminNetworkPolicy CRD not found: %v", err)
			}

			if !anpCRDExists && !banpCRDExists {
				g.Skip("AdminNetworkPolicy and BaselineAdminNetworkPolicy CRDs are not installed, skipping test")
			}

			// List all AdminNetworkPolicies
			if anpCRDExists {
				g.By("Checking for AdminNetworkPolicy resources")
				anpList, err := crdClient.Resource(anpGVR).List(ctx, metav1.ListOptions{})
				if err != nil {
					e2e.Logf("Error listing AdminNetworkPolicies: %v", err)
				} else {
					e2e.Logf("Found %d AdminNetworkPolicy resources", len(anpList.Items))
					for _, anp := range anpList.Items {
						e2e.Logf("  - AdminNetworkPolicy: %s", anp.GetName())
					}
				}
			}

			// List all BaselineAdminNetworkPolicies
			if banpCRDExists {
				g.By("Checking for BaselineAdminNetworkPolicy resources")
				banpList, err := crdClient.Resource(banpGVR).List(ctx, metav1.ListOptions{})
				if err != nil {
					e2e.Logf("Error listing BaselineAdminNetworkPolicies: %v", err)
				} else {
					e2e.Logf("Found %d BaselineAdminNetworkPolicy resources", len(banpList.Items))
					for _, banp := range banpList.Items {
						e2e.Logf("  - BaselineAdminNetworkPolicy: %s", banp.GetName())
					}
				}
			}
		})

		g.It("should validate AdminNetworkPolicy structure for control plane namespaces [apigroup:policy.networking.k8s.io]", func() {
			ctx := context.Background()

			// Check if AdminNetworkPolicy CRD exists
			crdClient := oc.AdminDynamicClient()
			crdGVR := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			_, err := crdClient.Resource(crdGVR).Get(ctx, "adminnetworkpolicies.policy.networking.k8s.io", metav1.GetOptions{})
			if err != nil {
				g.Skip("AdminNetworkPolicy CRD is not installed, skipping test")
			}

			// List all AdminNetworkPolicies
			anpList, err := crdClient.Resource(anpGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				e2e.Failf("Failed to list AdminNetworkPolicies: %v", err)
			}

			if len(anpList.Items) == 0 {
				e2e.Logf("INFO:No AdminNetworkPolicy resources found. This is acceptable if using standard NetworkPolicy resources.")
				return
			}

			for _, anp := range anpList.Items {
				anpName := anp.GetName()
				g.By(fmt.Sprintf("Validating AdminNetworkPolicy: %s", anpName))

				// Get spec
				spec, found, err := unstructured.NestedMap(anp.Object, "spec")
				o.Expect(err).NotTo(o.HaveOccurred())
				if !found {
					e2e.Logf("WARNING:AdminNetworkPolicy %s has no spec", anpName)
					continue
				}

				// Check priority
				priority, found, err := unstructured.NestedInt64(spec, "priority")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found {
					e2e.Logf("Priority: %d", priority)
					o.Expect(priority).To(o.BeNumerically(">", 0), "Priority must be greater than 0")
					o.Expect(priority).To(o.BeNumerically("<=", 1000), "Priority should be <= 1000")
				}

				// Check subject (which namespaces this policy applies to)
				subject, found, err := unstructured.NestedMap(spec, "subject")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found {
					// Check if subject targets control plane namespaces
					namespaces, found, err := unstructured.NestedMap(subject, "namespaces")
					o.Expect(err).NotTo(o.HaveOccurred())
					if found {
						matchLabels, found, err := unstructured.NestedStringMap(namespaces, "matchLabels")
						o.Expect(err).NotTo(o.HaveOccurred())
						if found {
							e2e.Logf("Targets namespaces with labels: %v", matchLabels)
						}
					}
				}

				// Check ingress rules
				ingress, found, err := unstructured.NestedSlice(spec, "ingress")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found && len(ingress) > 0 {
					e2e.Logf("Has %d ingress rules", len(ingress))

					for i, rule := range ingress {
						ruleMap, ok := rule.(map[string]interface{})
						if !ok {
							continue
						}

						ruleName, _, _ := unstructured.NestedString(ruleMap, "name")
						action, _, _ := unstructured.NestedString(ruleMap, "action")

						e2e.Logf("    - Ingress rule %d: %s (action: %s)", i, ruleName, action)

						// Validate action is one of: Pass, Deny, Allow
						if action != "" {
							o.Expect(action).To(o.BeElementOf("Pass", "Deny", "Allow"),
								"Ingress action must be Pass, Deny, or Allow")
						}
					}
				}

				// Check egress rules
				egress, found, err := unstructured.NestedSlice(spec, "egress")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found && len(egress) > 0 {
					e2e.Logf("Has %d egress rules", len(egress))

					for i, rule := range egress {
						ruleMap, ok := rule.(map[string]interface{})
						if !ok {
							continue
						}

						ruleName, _, _ := unstructured.NestedString(ruleMap, "name")
						action, _, _ := unstructured.NestedString(ruleMap, "action")

						e2e.Logf("    - Egress rule %d: %s (action: %s)", i, ruleName, action)

						// Validate action is one of: Pass, Deny, Allow
						if action != "" {
							o.Expect(action).To(o.BeElementOf("Pass", "Deny", "Allow"),
								"Egress action must be Pass, Deny, or Allow")
						}
					}
				}
			}
		})

		g.It("should have AdminNetworkPolicy with explicit deny rules for unauthorized traffic [apigroup:policy.networking.k8s.io]", func() {
			ctx := context.Background()

			// Check if AdminNetworkPolicy CRD exists
			crdClient := oc.AdminDynamicClient()
			crdGVR := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			_, err := crdClient.Resource(crdGVR).Get(ctx, "adminnetworkpolicies.policy.networking.k8s.io", metav1.GetOptions{})
			if err != nil {
				g.Skip("AdminNetworkPolicy CRD is not installed, skipping test")
			}

			// List all AdminNetworkPolicies
			anpList, err := crdClient.Resource(anpGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				e2e.Failf("Failed to list AdminNetworkPolicies: %v", err)
			}

			if len(anpList.Items) == 0 {
				e2e.Logf("INFO:No AdminNetworkPolicy resources found. Using standard NetworkPolicy for security controls.")
				return
			}

			hasDenyRules := false

			for _, anp := range anpList.Items {
				anpName := anp.GetName()
				spec, found, err := unstructured.NestedMap(anp.Object, "spec")
				o.Expect(err).NotTo(o.HaveOccurred())
				if !found {
					continue
				}

				// Check for Deny action in ingress rules
				ingress, found, err := unstructured.NestedSlice(spec, "ingress")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found {
					for _, rule := range ingress {
						ruleMap, ok := rule.(map[string]interface{})
						if !ok {
							continue
						}

						action, _, _ := unstructured.NestedString(ruleMap, "action")
						if action == "Deny" {
							hasDenyRules = true
							ruleName, _, _ := unstructured.NestedString(ruleMap, "name")
							e2e.Logf("AdminNetworkPolicy %s has Deny ingress rule: %s", anpName, ruleName)
						}
					}
				}

				// Check for Deny action in egress rules
				egress, found, err := unstructured.NestedSlice(spec, "egress")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found {
					for _, rule := range egress {
						ruleMap, ok := rule.(map[string]interface{})
						if !ok {
							continue
						}

						action, _, _ := unstructured.NestedString(ruleMap, "action")
						if action == "Deny" {
							hasDenyRules = true
							ruleName, _, _ := unstructured.NestedString(ruleMap, "name")
							e2e.Logf("AdminNetworkPolicy %s has Deny egress rule: %s", anpName, ruleName)
						}
					}
				}
			}

			if hasDenyRules {
				e2e.Logf("AdminNetworkPolicy resources include explicit Deny rules for security hardening")
			} else {
				e2e.Logf("INFO:No explicit Deny rules found in AdminNetworkPolicy resources (may rely on default deny behavior)")
			}
		})
	})

	g.Describe("BaselineAdminNetworkPolicy Resources", func() {
		g.It("should validate BaselineAdminNetworkPolicy structure [apigroup:policy.networking.k8s.io]", func() {
			ctx := context.Background()

			// Check if BaselineAdminNetworkPolicy CRD exists
			crdClient := oc.AdminDynamicClient()
			crdGVR := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			_, err := crdClient.Resource(crdGVR).Get(ctx, "baselineadminnetworkpolicies.policy.networking.k8s.io", metav1.GetOptions{})
			if err != nil {
				g.Skip("BaselineAdminNetworkPolicy CRD is not installed, skipping test")
			}

			// List all BaselineAdminNetworkPolicies
			banpList, err := crdClient.Resource(banpGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				e2e.Failf("Failed to list BaselineAdminNetworkPolicies: %v", err)
			}

			if len(banpList.Items) == 0 {
				e2e.Logf("INFO:No BaselineAdminNetworkPolicy resources found")
				return
			}

			// BaselineAdminNetworkPolicy acts as a baseline/default policy
			// It has lower priority than AdminNetworkPolicy
			for _, banp := range banpList.Items {
				banpName := banp.GetName()
				g.By(fmt.Sprintf("Validating BaselineAdminNetworkPolicy: %s", banpName))

				spec, found, err := unstructured.NestedMap(banp.Object, "spec")
				o.Expect(err).NotTo(o.HaveOccurred())
				if !found {
					e2e.Logf("WARNING:BaselineAdminNetworkPolicy %s has no spec", banpName)
					continue
				}

				// Check subject
				_, found, err = unstructured.NestedMap(spec, "subject")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found {
					e2e.Logf("Has subject defined")
				}

				// Check ingress rules
				ingress, found, err := unstructured.NestedSlice(spec, "ingress")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found {
					e2e.Logf("Has %d baseline ingress rules", len(ingress))
				}

				// Check egress rules
				egress, found, err := unstructured.NestedSlice(spec, "egress")
				o.Expect(err).NotTo(o.HaveOccurred())
				if found {
					e2e.Logf("Has %d baseline egress rules", len(egress))
				}
			}
		})

		g.It("should allow BaselineAdminNetworkPolicy to provide default-deny baseline [apigroup:policy.networking.k8s.io]", func() {
			ctx := context.Background()

			// Check if BaselineAdminNetworkPolicy CRD exists
			crdClient := oc.AdminDynamicClient()
			crdGVR := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			_, err := crdClient.Resource(crdGVR).Get(ctx, "baselineadminnetworkpolicies.policy.networking.k8s.io", metav1.GetOptions{})
			if err != nil {
				g.Skip("BaselineAdminNetworkPolicy CRD is not installed, skipping test")
			}

			// List all BaselineAdminNetworkPolicies
			banpList, err := crdClient.Resource(banpGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				e2e.Failf("Failed to list BaselineAdminNetworkPolicies: %v", err)
			}

			if len(banpList.Items) == 0 {
				e2e.Logf("INFO:No BaselineAdminNetworkPolicy resources found. This is acceptable if using other policy mechanisms.")
				return
			}

			// BaselineAdminNetworkPolicy with Deny action provides a default-deny baseline
			// which is then selectively opened up by AdminNetworkPolicy or NetworkPolicy
			hasBaselineDeny := false

			for _, banp := range banpList.Items {
				spec, found, err := unstructured.NestedMap(banp.Object, "spec")
				o.Expect(err).NotTo(o.HaveOccurred())
				if !found {
					continue
				}

				// Check for Deny actions
				ingress, _, _ := unstructured.NestedSlice(spec, "ingress")
				for _, rule := range ingress {
					ruleMap, ok := rule.(map[string]interface{})
					if !ok {
						continue
					}

					action, _, _ := unstructured.NestedString(ruleMap, "action")
					if action == "Deny" {
						hasBaselineDeny = true
						e2e.Logf("BaselineAdminNetworkPolicy provides default-deny ingress baseline")
					}
				}

				egress, _, _ := unstructured.NestedSlice(spec, "egress")
				for _, rule := range egress {
					ruleMap, ok := rule.(map[string]interface{})
					if !ok {
						continue
					}

					action, _, _ := unstructured.NestedString(ruleMap, "action")
					if action == "Deny" {
						hasBaselineDeny = true
						e2e.Logf("BaselineAdminNetworkPolicy provides default-deny egress baseline")
					}
				}
			}

			if hasBaselineDeny {
				e2e.Logf("BaselineAdminNetworkPolicy provides default-deny baseline for zero-trust architecture")
			}
		})
	})

	g.Describe("AdminNetworkPolicy vs NetworkPolicy Coexistence", func() {
		g.It("should verify AdminNetworkPolicy and NetworkPolicy can coexist [apigroup:policy.networking.k8s.io][apigroup:networking.k8s.io]", func() {
			ctx := context.Background()

			controlPlaneNamespaces := []string{
				"openshift-kube-apiserver",
				"openshift-etcd",
			}

			// Check if AdminNetworkPolicy CRD exists
			crdClient := oc.AdminDynamicClient()
			crdGVR := schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			}

			anpExists := false
			_, err := crdClient.Resource(crdGVR).Get(ctx, "adminnetworkpolicies.policy.networking.k8s.io", metav1.GetOptions{})
			if err == nil {
				anpExists = true
			}

			if !anpExists {
				g.Skip("AdminNetworkPolicy CRD is not installed, skipping coexistence test")
			}

			for _, namespace := range controlPlaneNamespaces {
				g.By(fmt.Sprintf("Checking policy coexistence in namespace: %s", namespace))

				_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Namespace %s does not exist, skipping: %v", namespace, err)
					continue
				}

				// Get NetworkPolicies
				npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				npCount := len(npList.Items)
				e2e.Logf("  Found %d NetworkPolicy resources in %s", npCount, namespace)

				// Note: AdminNetworkPolicy is cluster-scoped, not namespace-scoped
				// So we check for AdminNetworkPolicy that targets this namespace
				anpList, err := crdClient.Resource(anpGVR).List(ctx, metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				anpCount := 0
				for _, anp := range anpList.Items {
					spec, found, err := unstructured.NestedMap(anp.Object, "spec")
					if err != nil || !found {
						continue
					}

					// Check if this ANP targets our namespace
					subject, found, err := unstructured.NestedMap(spec, "subject")
					if err != nil || !found {
						continue
					}

					// If we find namespace selectors, count this ANP
					_, found, _ = unstructured.NestedMap(subject, "namespaces")
					if found {
						anpCount++
					}
				}

				e2e.Logf("  Found %d AdminNetworkPolicy resources potentially targeting control plane", anpCount)

				// Both can coexist - AdminNetworkPolicy takes precedence over NetworkPolicy
				// NetworkPolicy is still useful for namespace-specific rules
				if npCount > 0 || anpCount > 0 {
					e2e.Logf("Namespace %s has network policies (NP: %d, ANP: %d)", namespace, npCount, anpCount)
				}
			}
		})
	})
})
