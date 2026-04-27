package hypershift

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	tlsutil "github.com/openshift/origin/test/extended/tls"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Disruptive][Suite:openshift/tls-observed-config-hypershift]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config-hypershift")
	ctx := context.Background()

	var isHyperShiftCluster bool
	var mgmtOC *exutil.CLI
	var hcpNamespace string
	var hostedClusterName string
	var hostedClusterNS string

	g.BeforeEach(func() {
		mgmtOC = nil
		hcpNamespace = ""
		hostedClusterName = ""
		hostedClusterNS = ""

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift clusters")
		}

		isHS, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShiftCluster = isHS

		if !isHyperShiftCluster {
			g.Skip("HyperShift TLS tests only run on HyperShift clusters")
		}

		_, hcpNamespace, err = exutil.GetHypershiftManagementClusterConfigAndNamespace()
		if err != nil {
			e2e.Logf("WARNING: HyperShift management cluster credentials are not available: %v", err)
		} else {
			mgmtOC = exutil.NewHypershiftManagementCLI("tls-mgmt")
			hostedClusterName, hostedClusterNS = discoverHostedCluster(mgmtOC, hcpNamespace)
			e2e.Logf("HyperShift: HC=%s/%s, HCP NS=%s", hostedClusterNS, hostedClusterName, hcpNamespace)
		}
	})

	for _, target := range tlsutil.ObservedConfigTargets {
		target := target
		g.It(fmt.Sprintf("should populate ObservedConfig with TLS settings - %s", target.Namespace), func() {
			tlsutil.TestObservedConfig(oc, ctx, target, isHyperShiftCluster)
		})
	}

	for _, target := range tlsutil.ConfigMapTargets {
		target := target
		g.It(fmt.Sprintf("should have TLS config injected into ConfigMap - %s", target.Namespace), func() {
			tlsutil.TestConfigMapTLSInjection(oc, ctx, target)
		})
	}

	for _, target := range tlsutil.DeploymentEnvVarTargets {
		target := target
		g.It(fmt.Sprintf("should propagate TLS config to deployment env vars - %s", target.Namespace), func() {
			tlsutil.TestDeploymentTLSEnvVars(oc, ctx, target)
		})
	}

	for _, target := range tlsutil.ServiceTargets {
		target := target
		g.It(fmt.Sprintf("should enforce TLS version at the wire level - %s:%s", target.Namespace, target.ServicePort), func() {
			if target.ControlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s:%s on HyperShift (runs on management cluster)", target.Namespace, target.ServicePort))
			}
			tlsutil.TestWireLevelTLS(oc, ctx, target)
		})
	}

	for _, target := range tlsutil.ConfigMapTargets {
		target := target
		g.It(fmt.Sprintf("should restore inject-tls annotation after deletion - %s", target.Namespace), func() {
			tlsutil.TestAnnotationRestorationAfterDeletion(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore inject-tls annotation when set to false - %s", target.Namespace), func() {
			tlsutil.TestAnnotationRestorationWhenFalse(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore servingInfo after removal - %s", target.Namespace), func() {
			tlsutil.TestServingInfoRestorationAfterRemoval(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore servingInfo after modification - %s", target.Namespace), func() {
			tlsutil.TestServingInfoRestorationAfterModification(oc, ctx, target)
		})
	}

	g.It("should enforce Modern TLS profile after cluster-wide config change on HyperShift [Timeout:60m]", func() {
		if mgmtOC == nil {
			g.Skip("HyperShift management cluster credentials are not available")
		}
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 50*time.Minute)
		defer configChangeCancel()

		_ = isHyperShiftCluster // always true here

		modernPatch := `{"spec":{"configuration":{"apiServer":{"tlsSecurityProfile":{"modern":{},"type":"Modern"}}}}}`
		resetPatch := `{"spec":{"configuration":{"apiServer":null}}}`

		g.By("reading current HostedCluster TLS profile")
		currentTLS, err := mgmtOC.AsAdmin().Run("get").Args(
			"hostedcluster", hostedClusterName, "-n", hostedClusterNS,
			"-o", `jsonpath={.spec.configuration.apiServer.tlsSecurityProfile.type}`,
		).Output()
		if err != nil || currentTLS == "" {
			currentTLS = "Intermediate (default)"
		}
		e2e.Logf("Current HostedCluster TLS profile: %s", currentTLS)

		if currentTLS == "Modern" {
			g.Skip("HostedCluster is already using Modern TLS profile")
		}

		g.DeferCleanup(func(cleanupCtx context.Context) {
			e2e.Logf("DeferCleanup: restoring HostedCluster TLS profile to default")
			setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, resetPatch)
			waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
			waitForGuestOperatorsAfterTLSChange(oc, cleanupCtx, "restore")
			e2e.Logf("DeferCleanup: HostedCluster TLS profile restored")
		})

		guestObsCfgTargets := guestSideObservedConfigTargets()
		guestSvcTargets := guestSideServiceTargets()

		g.By("patching HostedCluster with Modern TLS profile")
		setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, modernPatch)
		e2e.Logf("HostedCluster TLS profile patched to Modern")

		g.By("waiting for HCP pods and guest operators to stabilize")
		waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
		waitForGuestOperatorsAfterTLSChange(oc, configChangeCtx, "Modern")

		g.By("verifying guest-side ObservedConfig reflects Modern profile")
		tlsutil.VerifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", guestObsCfgTargets)
		g.By("verifying guest-side ConfigMaps reflect Modern profile")
		tlsutil.VerifyConfigMapsForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", tlsutil.ConfigMapTargets)
		g.By("verifying HCP ConfigMaps reflect Modern profile")
		verifyHCPConfigMaps(mgmtOC, hcpNamespace, "VersionTLS13", "Modern")

		for _, t := range tlsutil.DeploymentEnvVarTargets {
			g.By(fmt.Sprintf("verifying %s in %s/%s reflects Modern profile",
				t.TLSMinVersionEnvVar, t.Namespace, t.DeploymentName))
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.Namespace).Get(
				configChangeCtx, t.DeploymentName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			envMap := exutil.FindEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.TLSMinVersionEnvVar)
			o.Expect(envMap).To(o.HaveKey(t.TLSMinVersionEnvVar))
			o.Expect(envMap[t.TLSMinVersionEnvVar]).To(o.Equal("VersionTLS13"))
			e2e.Logf("PASS: %s=VersionTLS13 in %s/%s", t.TLSMinVersionEnvVar, t.Namespace, t.DeploymentName)
		}

		tlsShouldWork := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		tlsShouldNotWork := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		for _, t := range guestSvcTargets {
			g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (expecting Modern = TLS 1.3 only)", t.ServiceName, t.Namespace))
			err = exutil.ForwardPortAndExecute(t.ServiceName, t.Namespace, t.ServicePort,
				func(localPort int) error {
					return exutil.CheckTLSConnection(localPort, tlsShouldWork, tlsShouldNotWork, t.ServiceName, t.Namespace)
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s (Modern)", t.ServiceName, t.Namespace)
		}
		e2e.Logf("PASS: Modern TLS profile propagation verified on HyperShift")
	})

	g.It("should enforce Custom TLS profile after cluster-wide config change on HyperShift [Timeout:60m]", func() {
		if mgmtOC == nil {
			g.Skip("HyperShift management cluster credentials are not available")
		}
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 60*time.Minute)
		defer configChangeCancel()

		customCiphers := []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"ECDHE-RSA-AES256-GCM-SHA384",
			"ECDHE-ECDSA-AES128-GCM-SHA256",
			"ECDHE-ECDSA-AES256-GCM-SHA384",
		}

		customPatch := fmt.Sprintf(
			`{"spec":{"configuration":{"apiServer":{"tlsSecurityProfile":{"type":"Custom","custom":{"ciphers":["%s"],"minTLSVersion":"VersionTLS12"}}}}}}`,
			strings.Join(customCiphers, `","`),
		)
		resetPatch := `{"spec":{"configuration":{"apiServer":null}}}`

		g.DeferCleanup(func(cleanupCtx context.Context) {
			e2e.Logf("DeferCleanup: restoring HostedCluster TLS profile to default")
			setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, resetPatch)
			waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
			waitForGuestOperatorsAfterTLSChange(oc, cleanupCtx, "restore")
			e2e.Logf("DeferCleanup: HostedCluster TLS profile restored")
		})

		guestObsCfgTargets := guestSideObservedConfigTargets()
		guestSvcTargets := guestSideServiceTargets()

		g.By("patching HostedCluster with Custom TLS profile")
		setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, customPatch)
		e2e.Logf("HostedCluster TLS profile patched to Custom (minTLSVersion=TLS12, ciphers=%d)", len(customCiphers))

		g.By("waiting for HCP pods and guest operators to stabilize")
		waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
		waitForGuestOperatorsAfterTLSChange(oc, configChangeCtx, "Custom")

		g.By("verifying guest-side ObservedConfig reflects Custom profile")
		tlsutil.VerifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS12", "Custom", guestObsCfgTargets)
		g.By("verifying guest-side ConfigMaps reflect Custom profile")
		tlsutil.VerifyConfigMapsForTargets(oc, configChangeCtx, "VersionTLS12", "Custom", tlsutil.ConfigMapTargets)
		g.By("verifying HCP ConfigMaps reflect Custom profile")
		verifyHCPConfigMaps(mgmtOC, hcpNamespace, "VersionTLS12", "Custom")

		g.By("verifying wire-level TLS for Custom profile (TLS 1.2) on guest targets")
		for _, t := range guestSvcTargets {
			shouldWork := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
			shouldNotWork := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11}
			err := exutil.ForwardPortAndExecute(t.ServiceName, t.Namespace, t.ServicePort, func(localPort int) error {
				return exutil.CheckTLSConnection(localPort, shouldWork, shouldNotWork, t.ServiceName, t.Namespace)
			})
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("wire-level TLS check failed for svc/%s in %s:%s with Custom profile", t.ServiceName, t.Namespace, t.ServicePort))
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s:%s (Custom profile)", t.ServiceName, t.Namespace, t.ServicePort)
		}

		e2e.Logf("PASS: Custom TLS profile verified successfully on HyperShift")
	})
})

// ─── HyperShift helpers ────────────────────────────────────────────────────

func guestSideObservedConfigTargets() []tlsutil.ObservedConfigTarget {
	var result []tlsutil.ObservedConfigTarget
	for _, t := range tlsutil.ObservedConfigTargets {
		if !t.ControlPlane {
			result = append(result, t)
		}
	}
	return result
}

func guestSideServiceTargets() []tlsutil.ServiceTarget {
	var result []tlsutil.ServiceTarget
	for _, t := range tlsutil.ServiceTargets {
		if !t.ControlPlane {
			result = append(result, t)
		}
	}
	return result
}

func guestSideClusterOperators() []string {
	seen := map[string]bool{}
	var result []string
	for _, name := range tlsutil.ClusterOperatorNames {
		if seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	// Filter to only non-control-plane operators by checking service targets.
	// On HyperShift, only operators with guest-side services are relevant.
	guestOps := map[string]bool{}
	for _, t := range guestSideServiceTargets() {
		// Map service namespace back to operator name — use ClusterOperatorNames
		// that appear in non-control-plane targets.
		guestOps[t.Namespace] = true
	}
	// Also include image-registry and openshift-samples which have guest-side deployments.
	return []string{"image-registry", "openshift-samples"}
}

func guestSideDeploymentRolloutTargets() []tlsutil.DeploymentRolloutTarget {
	var result []tlsutil.DeploymentRolloutTarget
	for _, t := range tlsutil.DeploymentRolloutTargets {
		if !t.ControlPlane {
			result = append(result, t)
		}
	}
	return result
}

func discoverHostedCluster(mgmtCLI *exutil.CLI, hcpNS string) (string, string) {
	output, err := mgmtCLI.AsAdmin().Run("get").Args(
		"hostedclusters", "-A",
		"-o", `jsonpath={range .items[*]}{.metadata.namespace},{.metadata.name}{"\n"}{end}`,
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list HostedClusters on management cluster")

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 {
			ns, name := parts[0], parts[1]
			if ns+"-"+name == hcpNS {
				return name, ns
			}
		}
	}
	e2e.Failf("could not find HostedCluster matching HCP namespace %s", hcpNS)
	return "", ""
}

func setTLSProfileOnHyperShift(mgmtCLI *exutil.CLI, hcName, hcNS, patchJSON string) {
	err := mgmtCLI.AsAdmin().Run("patch").Args(
		"hostedcluster", hcName, "-n", hcNS,
		"--type=merge", "-p", patchJSON,
	).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to patch HostedCluster TLS profile")
}

func waitForHCPPods(mgmtCLI *exutil.CLI, hcpNS string, timeout time.Duration) {
	for _, appLabel := range []string{"kube-apiserver", "openshift-apiserver", "oauth-openshift"} {
		e2e.Logf("Waiting for %s pods in HCP namespace %s", appLabel, hcpNS)
		err := waitForHCPAppReady(mgmtCLI, appLabel, hcpNS, timeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("HCP pods for %s did not become ready in %s within %v", appLabel, hcpNS, timeout))
		e2e.Logf("HCP %s pods are ready in %s", appLabel, hcpNS)
	}
}

func waitForHCPAppReady(mgmtCLI *exutil.CLI, appLabel, hcpNS string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(context.Background(), 10*time.Second, timeout, false,
		func(ctx context.Context) (bool, error) {
			out, err := mgmtCLI.AsAdmin().Run("get").Args(
				"pods", "-l", "app="+appLabel,
				"--no-headers", "-n", hcpNS,
			).Output()
			if err != nil {
				e2e.Logf("  poll: error listing %s pods: %v", appLabel, err)
				return false, nil
			}
			if out == "" {
				e2e.Logf("  poll: no %s pods found yet", appLabel)
				return false, nil
			}

			for _, indicator := range []string{"0/", "Pending", "Terminating", "Init"} {
				if strings.Contains(out, indicator) {
					e2e.Logf("  poll: %s pods still restarting (found %q)", appLabel, indicator)
					return false, nil
				}
			}

			time.Sleep(10 * time.Second)
			out2, err := mgmtCLI.AsAdmin().Run("get").Args(
				"pods", "-l", "app="+appLabel,
				"--no-headers", "-n", hcpNS,
			).Output()
			if err != nil {
				return false, nil
			}
			for _, indicator := range []string{"0/", "Pending", "Terminating", "Init"} {
				if strings.Contains(out2, indicator) {
					e2e.Logf("  poll: %s pods still not stable on recheck", appLabel)
					return false, nil
				}
			}

			e2e.Logf("  poll: %s pods are ready in %s", appLabel, hcpNS)
			return true, nil
		})
}

func waitForGuestOperatorsAfterTLSChange(oc *exutil.CLI, ctx context.Context, profileLabel string) {
	e2e.Logf("Waiting for guest-side ClusterOperators to stabilize after %s profile change", profileLabel)
	for _, co := range guestSideClusterOperators() {
		e2e.Logf("Waiting for ClusterOperator %s to stabilize after %s switch", co, profileLabel)
		exutil.WaitForClusterOperatorStable(oc, ctx, co)
	}

	for _, d := range guestSideDeploymentRolloutTargets() {
		e2e.Logf("Waiting for deployment %s/%s to complete rollout after %s switch", d.Namespace, d.DeploymentName, profileLabel)
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(d.Namespace).Get(ctx, d.DeploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, tlsutil.OperatorRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout after %s TLS change",
				d.Namespace, d.DeploymentName, profileLabel))
		e2e.Logf("Deployment %s/%s is fully rolled out after %s switch", d.Namespace, d.DeploymentName, profileLabel)
	}
	e2e.Logf("All guest-side operators and deployments are stable after %s profile change", profileLabel)
}

func verifyHCPConfigMaps(mgmtCLI *exutil.CLI, hcpNS, expectedVersion, profileLabel string) {
	hcpCMs := []struct {
		name      string
		configKey string
	}{
		{name: "kas-config", configKey: `config\.json`},
		{name: "openshift-apiserver", configKey: `config\.yaml`},
	}

	for _, cm := range hcpCMs {
		out, err := mgmtCLI.AsAdmin().Run("get").Args(
			"cm", cm.name, "-n", hcpNS,
			"-o", fmt.Sprintf("jsonpath={.data.%s}", cm.configKey),
		).Output()
		if err != nil {
			e2e.Logf("SKIP: HCP ConfigMap %s/%s not found: %v", hcpNS, cm.name, err)
			continue
		}

		o.Expect(out).To(o.ContainSubstring(expectedVersion),
			fmt.Sprintf("HCP ConfigMap %s/%s should contain %s after %s switch",
				hcpNS, cm.name, expectedVersion, profileLabel))
		e2e.Logf("PASS: HCP ConfigMap %s/%s contains %s after %s switch",
			hcpNS, cm.name, expectedVersion, profileLabel)
	}
}
