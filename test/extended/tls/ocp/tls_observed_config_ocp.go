package ocp

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	tlsutil "github.com/openshift/origin/test/extended/tls"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Disruptive][Suite:openshift/tls-observed-config-ocp]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config-ocp")
	ctx := context.Background()

	var isHyperShiftCluster bool

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift clusters")
		}

		isHS, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShiftCluster = isHS
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
			if isHyperShiftCluster && target.ControlPlane {
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

	g.It("should enforce Modern TLS profile after cluster-wide config change [Timeout:60m]", func() {
		if isHyperShiftCluster {
			g.Skip("HyperShift Modern TLS profile test runs in the hypershift suite")
		}

		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 50*time.Minute)
		defer configChangeCancel()

		g.By("reading current APIServer TLS profile")
		originalConfig, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		originalProfile := originalConfig.Spec.TLSSecurityProfile
		profileDesc := "nil (Intermediate default)"
		if originalProfile != nil {
			profileDesc = string(originalProfile.Type)
		}
		e2e.Logf("Current TLS profile: %s", profileDesc)

		if originalProfile != nil && originalProfile.Type == configv1.TLSProfileModernType {
			g.Skip("Cluster is already using Modern TLS profile; config-change test is not applicable")
		}

		g.DeferCleanup(func(cleanupCtx context.Context) {
			e2e.Logf("DeferCleanup: restoring original TLS profile: %s", profileDesc)
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(cleanupCtx, "cluster", metav1.GetOptions{})
				if err != nil {
					return err
				}
				current.Spec.TLSSecurityProfile = originalProfile
				_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(cleanupCtx, current, metav1.UpdateOptions{})
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore original TLS profile")

			e2e.Logf("DeferCleanup: waiting for all operators to stabilize after restoring profile")
			tlsutil.WaitForOperatorsAfterTLSChange(oc, cleanupCtx, "restore", tlsutil.ClusterOperatorNames, tlsutil.DeploymentRolloutTargets)
			e2e.Logf("DeferCleanup: original TLS profile restored and cluster is stable")
		})

		g.By("setting APIServer TLS profile to Modern")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type:   configv1.TLSProfileModernType,
				Modern: &configv1.ModernTLSProfile{},
			}
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(configChangeCtx, apiServer, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Modern")
		e2e.Logf("APIServer TLS profile updated to Modern")

		g.By("waiting for all operators to stabilize after TLS profile change to Modern")
		tlsutil.WaitForOperatorsAfterTLSChange(oc, configChangeCtx, "Modern", tlsutil.ClusterOperatorNames, tlsutil.DeploymentRolloutTargets)

		for _, t := range tlsutil.DeploymentEnvVarTargets {
			g.By(fmt.Sprintf("verifying %s in %s/%s reflects Modern profile",
				t.TLSMinVersionEnvVar, t.Namespace, t.DeploymentName))
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.Namespace).Get(
				configChangeCtx, t.DeploymentName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(deployment.Spec.Template.Spec.Containers).NotTo(o.BeEmpty())

			envMap := exutil.FindEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.TLSMinVersionEnvVar)
			o.Expect(envMap).To(o.HaveKey(t.TLSMinVersionEnvVar))
			o.Expect(envMap[t.TLSMinVersionEnvVar]).To(o.Equal("VersionTLS13"),
				fmt.Sprintf("expected %s=VersionTLS13 in %s/%s after Modern profile, got %s",
					t.TLSMinVersionEnvVar, t.Namespace, t.DeploymentName,
					envMap[t.TLSMinVersionEnvVar]))
			e2e.Logf("PASS: %s=VersionTLS13 in %s/%s", t.TLSMinVersionEnvVar, t.Namespace, t.DeploymentName)

			if t.CipherSuitesEnvVar != "" {
				o.Expect(envMap).To(o.HaveKey(t.CipherSuitesEnvVar),
					fmt.Sprintf("expected %s to be set in %s/%s after Modern profile",
						t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName))
				e2e.Logf("PASS: %s is set in %s/%s after Modern profile (value length=%d)",
					t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName, len(envMap[t.CipherSuitesEnvVar]))
			}
		}

		g.By("verifying ObservedConfig reflects Modern profile (VersionTLS13)")
		tlsutil.VerifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", tlsutil.ObservedConfigTargets)

		g.By("verifying ConfigMaps reflect Modern profile (VersionTLS13)")
		tlsutil.VerifyConfigMapsForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", tlsutil.ConfigMapTargets)

		tlsShouldWork := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		tlsShouldNotWork := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

		for _, t := range tlsutil.ServiceTargets {
			g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (expecting Modern = TLS 1.3 only)",
				t.ServiceName, t.Namespace))
			err = exutil.ForwardPortAndExecute(t.ServiceName, t.Namespace, t.ServicePort,
				func(localPort int) error {
					return exutil.CheckTLSConnection(localPort, tlsShouldWork, tlsShouldNotWork, t.ServiceName, t.Namespace)
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("wire-level TLS check failed for svc/%s in %s after switching to Modern",
					t.ServiceName, t.Namespace))
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s (Modern)", t.ServiceName, t.Namespace)
		}

		e2e.Logf("PASS: Modern TLS profile propagation verified (restore handled by DeferCleanup)")
	})

	g.It("should enforce Custom TLS profile after cluster-wide config change [Timeout:60m]", func() {
		if isHyperShiftCluster {
			g.Skip("HyperShift Custom TLS profile test runs in the hypershift suite")
		}

		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 60*time.Minute)
		defer configChangeCancel()

		customCiphers := []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"ECDHE-RSA-AES256-GCM-SHA384",
			"ECDHE-ECDSA-AES128-GCM-SHA256",
			"ECDHE-ECDSA-AES256-GCM-SHA384",
		}
		customCiphersIANA := []string{
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		}

		g.By("reading current APIServer TLS profile")
		originalAPIServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get APIServer cluster config")

		originalProfile := originalAPIServer.Spec.TLSSecurityProfile
		profileDesc := "nil (Intermediate default)"
		if originalProfile != nil {
			profileDesc = fmt.Sprintf("%v", originalProfile.Type)
		}
		e2e.Logf("Current TLS profile: %s", profileDesc)

		g.DeferCleanup(func(cleanupCtx context.Context) {
			e2e.Logf("DeferCleanup: restoring original TLS profile: %s", profileDesc)
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(cleanupCtx, "cluster", metav1.GetOptions{})
				if err != nil {
					return err
				}
				apiServer.Spec.TLSSecurityProfile = originalProfile
				_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(cleanupCtx, apiServer, metav1.UpdateOptions{})
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore original TLS profile")

			e2e.Logf("DeferCleanup: waiting for all operators to stabilize after restoring profile")
			tlsutil.WaitForOperatorsAfterTLSChange(oc, cleanupCtx, "restore", tlsutil.ClusterOperatorNames, tlsutil.DeploymentRolloutTargets)
			e2e.Logf("DeferCleanup: original TLS profile restored and cluster is stable")
		})

		g.By("setting APIServer TLS profile to Custom (TLS 1.2 with specific ciphers)")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers:       customCiphers,
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			}
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(configChangeCtx, apiServer, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Custom")
		e2e.Logf("APIServer TLS profile updated to Custom (minTLSVersion=TLS12, ciphers=%d)", len(customCiphers))

		g.By("waiting for all operators to stabilize after TLS profile change to Custom")
		tlsutil.WaitForOperatorsAfterTLSChange(oc, configChangeCtx, "Custom", tlsutil.ClusterOperatorNames, tlsutil.DeploymentRolloutTargets)

		g.By("verifying ObservedConfig reflects Custom profile (VersionTLS12)")
		tlsutil.VerifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS12", "Custom", tlsutil.ObservedConfigTargets)

		g.By("verifying ConfigMaps reflect Custom profile (VersionTLS12)")
		for _, t := range tlsutil.ConfigMapTargets {
			ns := t.ResolvedNamespace()
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(ns).Get(configChangeCtx, t.ConfigMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("SKIP: ConfigMap %s/%s not found: %v", ns, t.ConfigMapName, err)
				continue
			}
			configData := cm.Data[t.ResolvedKey()]
			o.Expect(cm.Annotations).To(o.HaveKey(tlsutil.InjectTLSAnnotation),
				fmt.Sprintf("ConfigMap %s/%s is missing %s annotation", ns, t.ConfigMapName, tlsutil.InjectTLSAnnotation))
			o.Expect(configData).To(o.ContainSubstring("VersionTLS12"),
				fmt.Sprintf("ConfigMap %s/%s should have VersionTLS12 for Custom profile", ns, t.ConfigMapName))
			e2e.Logf("PASS: ConfigMap %s/%s has VersionTLS12 for Custom profile", ns, t.ConfigMapName)

			for i := 0; i < 2; i++ {
				found := strings.Contains(configData, customCiphers[i]) || strings.Contains(configData, customCiphersIANA[i])
				o.Expect(found).To(o.BeTrue(),
					fmt.Sprintf("ConfigMap %s/%s should contain cipher %s (or IANA equivalent %s)", ns, t.ConfigMapName, customCiphers[i], customCiphersIANA[i]))
			}
			e2e.Logf("PASS: ConfigMap %s/%s has custom cipher suites", ns, t.ConfigMapName)
		}

		g.By("verifying wire-level TLS for Custom profile (TLS 1.2)")
		for _, t := range tlsutil.ServiceTargets {
			g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (expecting Custom = TLS 1.2+)",
				t.ServiceName, t.Namespace))

			shouldWork := &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			}
			shouldNotWork := &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS10,
				MaxVersion:         tls.VersionTLS11,
			}

			err := exutil.ForwardPortAndExecute(t.ServiceName, t.Namespace, t.ServicePort, func(localPort int) error {
				return exutil.CheckTLSConnection(localPort, shouldWork, shouldNotWork, t.ServiceName, t.Namespace)
			})
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("wire-level TLS check failed for svc/%s in %s:%s with Custom profile", t.ServiceName, t.Namespace, t.ServicePort))
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s:%s (Custom profile)", t.ServiceName, t.Namespace, t.ServicePort)
		}

		e2e.Logf("PASS: Custom TLS profile verified successfully")
	})
})
