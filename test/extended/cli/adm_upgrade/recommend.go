package adm_upgrade

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	"github.com/openshift/origin/test/shared/updateservice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Serial][sig-cli] oc adm upgrade recommend", g.Ordered, func() {
	defer g.GinkgoRecover()
	ctx := context.Background()

	f := framework.NewDefaultFramework("oc-adm-upgrade-recommend")
	oc := exutil.NewCLIWithFramework(f).AsAdmin()
	var cv *configv1.ClusterVersion
	var restoreChannel, restoreUpstream, alertsByCVO, isHyperShift bool
	var caBundleFilePath string

	g.BeforeAll(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("MicroShift does not have a ClusterVersion resource")
		}

		cv, err = oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		isHyperShift, err = exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		alertsByCVO, err = alertsEvaluatedByCVO(ctx, cv, isHyperShift, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterAll(func() {
		if restoreChannel {
			oc.Run("adm", "upgrade", "channel", cv.Spec.Channel).Execute()
		}

		if restoreUpstream {
			oc.Run("patch", "clusterversions.config.openshift.io", "version", "--type", "json", "-p", fmt.Sprintf(`[{"op": "add", "path": "/spec/upstream", "value": "%s"}]`, cv.Spec.Upstream)).Execute()
		}

		if caBundleFilePath != "" {
			os.Remove(caBundleFilePath)
		}
	})

	g.It("runs successfully, even without upstream OpenShift Update Service customization", func() {
		_, err := oc.Run("adm", "upgrade", "recommend").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_PRECHECK", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_ACCEPT", "true").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("runs successfully with an empty channel", func() {
		err := oc.Run("adm", "upgrade", "channel").Execute()
		if err != nil {
			g.Skip(fmt.Sprintf("failed to update the ClusterVersion channel (perhaps we are on a HyperShift cluster): %s", err))
		}
		restoreChannel = true

		time.Sleep(16 * time.Second) // Give the CVO time to reconcile the empty channel and push to status

		out, err := oc.Run("adm", "upgrade", "recommend").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_PRECHECK", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_ACCEPT", "true").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = matchRegexp(out, `.*The update channel has not been configured.*`)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Context("When the update service has no recommendations", func() {
		g.BeforeAll(func() {
			isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isHyperShift {
				g.Skip("HyperShift does not support configuring the upstream OpenShift Update Service directoly via ClusterVersion (it must be configured via HostedCluster on the management cluster)")
			}

			updateService, err := updateservice.Run(ctx, oc.AdminKubeClient(), oc.Namespace(), image.ShellImage(), updateservice.Graph{
				Nodes: []updateservice.Node{{
					Version: cv.Status.Desired.Version,
					Payload: cv.Status.Desired.Image,
					Metadata: map[string]string{
						"io.openshift.upgrades.graph.release.channels": "test-channel,other-channel",
					},
				}},
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			g.DeferCleanup(updateService.Cleanup, ctx)
			newUpstream := updateService.URL

			err = oc.Run("adm", "upgrade", "channel", "test-channel").Execute()
			if err != nil {
				g.Skip(fmt.Sprintf("failed to update the ClusterVersion channel: %s", err))
			}
			restoreChannel = true

			err = oc.Run("patch", "clusterversions.config.openshift.io", "version", "--type", "json", "-p", fmt.Sprintf(`[{"op": "add", "path": "/spec/upstream", "value": "%s"}]`, newUpstream.String())).Execute()
			if err != nil {
				g.Skip(fmt.Sprintf("failed to update the ClusterVersion upstream: %s", err))
			}
			restoreUpstream = true

			time.Sleep(16 * time.Second) // Give the CVO time to retrieve recommendations and push to status
		})

		g.It("runs successfully", func() {
			out, err := oc.Run("adm", "upgrade", "recommend").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_PRECHECK", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_ACCEPT", "true").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = matchRegexp(out, `.*Upstream update service: http://.*
Channel: test-channel [(]available channels: other-channel, test-channel[)]
No updates available. You may still upgrade to a specific release image.*`)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Context("When the update service has conditional recommendations", func() {
		var currentVersion *semver.Version
		var token string

		g.BeforeAll(func() {
			if isHyperShift {
				g.Skip("HyperShift does not support configuring the upstream OpenShift Update Service directoly via ClusterVersion (it must be configured via HostedCluster on the management cluster)")
			}

			if curVer, err := semver.Parse(cv.Status.Desired.Version); err != nil {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				currentVersion = &curVer
			}

			targetVersion := fmt.Sprintf("4.%d.0", currentVersion.Minor+1)
			graph := updateservice.Graph{
				Nodes: []updateservice.Node{
					{
						Version: cv.Status.Desired.Version,
						Payload: cv.Status.Desired.Image,
						Metadata: map[string]string{
							"io.openshift.upgrades.graph.release.channels": "test-channel,other-channel",
						},
					},
					{
						Version: fmt.Sprintf("4.%d.998", currentVersion.Minor),
						Payload: "example.com/test@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
					{
						Version: fmt.Sprintf("4.%d.999", currentVersion.Minor),
						Payload: "example.com/test@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					},
					{
						Version:  targetVersion,
						Payload:  "example.com/test@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
						Metadata: map[string]string{"url": fmt.Sprintf("https://example.com/release/%s", targetVersion)},
					},
				},
				Edges: []updateservice.Edge{{0, 1}, {0, 2}},
				ConditionalEdges: []updateservice.ConditionalEdge{{
					Edges: []updateservice.VersionEdge{{
						From: cv.Status.Desired.Version,
						To:   targetVersion,
					}},
					Risks: []updateservice.Risk{{
						URL:     "https://example.com/testRiskA",
						Name:    "TestRiskA",
						Message: "This is a test risk.",
						MatchingRules: []updateservice.MatchingRule{{
							Type:   "PromQL",
							PromQL: &updateservice.PromQL{PromQL: "group(max_over_time(cluster_version[1h]))"},
						}},
					}},
				}},
			}

			updateService, err := updateservice.Run(ctx, oc.AdminKubeClient(), oc.Namespace(), image.ShellImage(), graph)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.DeferCleanup(updateService.Cleanup, ctx)
			newUpstream := updateService.URL

			err = oc.Run("adm", "upgrade", "channel", "test-channel").Execute()
			if err != nil {
				g.Skip(fmt.Sprintf("failed to update the ClusterVersion channel: %s", err))
			}
			restoreChannel = true

			err = oc.Run("patch", "clusterversions.config.openshift.io", "version", "--type", "json", "-p", fmt.Sprintf(`[{"op": "add", "path": "/spec/upstream", "value": "%s"}]`, newUpstream.String())).Execute()
			if err != nil {
				g.Skip(fmt.Sprintf("failed to update the ClusterVersion upstream: %s", err))
			}
			restoreUpstream = true

			defaultIngressCert, err := getDefaultIngressCertificate(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			kubeCerts, err := getKubernetesAPIServerCertificates(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			caBundleFile, err := os.CreateTemp("", "ca-bundle")
			caBundleFilePath = caBundleFile.Name()
			_, err = caBundleFile.WriteString(fmt.Sprintf("%s\n%s", defaultIngressCert, kubeCerts))
			o.Expect(err).NotTo(o.HaveOccurred())

			// alert retrieval requires a token-based kubeconfig to avoid:
			//   Failed to check for at least some preconditions: failed to get alerts from Thanos: no token is currently in use for this session
			o.Expect(oc.Run("create").Args("serviceaccount", "test").Execute()).To(o.Succeed())
			o.Expect(oc.Run("create").Args("clusterrolebinding", fmt.Sprintf("%s-test", oc.Namespace()), "--clusterrole=cluster-admin", fmt.Sprintf("--serviceaccount=%s:test", oc.Namespace())).Execute()).To(o.Succeed())
			token, err = oc.Run("create").Args("token", "test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			time.Sleep(16 * time.Second) // Give the CVO time to retrieve recommendations and push to status
		})

		g.AfterAll(func() {
			// apparently ClusterRoleBindings are not automatically garbage-collected after the referenced service-account is removed (as part of namespace removal).
			oc.Run("delete").Args("clusterrolebinding", fmt.Sprintf("%s-test", oc.Namespace())).Execute()
		})

		g.It("runs successfully when listing all updates", func() {
			oc.WithKubeConfigCopy(func(oc *exutil.CLI) {
				o.Expect(oc.Run("config", "set-credentials").Args("test", "--token", token).Execute()).To(o.Succeed())
				o.Expect(oc.Run("config", "set-context").Args("--current", "--user", "test").Execute()).To(o.Succeed())

				out, err := oc.Run("--certificate-authority", caBundleFilePath, "adm", "upgrade", "recommend").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_PRECHECK", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_ACCEPT", "true").Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				pattern := `The following conditions found no cause for concern in updating this cluster to later releases.*

Upstream update service: http://.*
Channel: test-channel [(]available channels: other-channel, test-channel[)]

Updates to 4[.][0-9]*:

  Version: 4[.][0-9]*[.]0
  Image: example[.]com/test@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
  Reason: (TestRiskA|MultipleReasons)
  Message: (?s:.*)This is a test risk[.] https://example[.]com/testRiskA

Updates to 4[.][0-9]*:
  VERSION  *ISSUES
  4[.][0-9]*[.]999  *no known issues relevant to this cluster
  4[.][0-9]*[.]998  *no known issues relevant to this cluster`
				if alertsByCVO {
					pattern = `Upstream update service: http://.*
Channel: test-channel [(]available channels: other-channel, test-channel[)]

Updates to 4[.][0-9]*:

  Version: 4[.][0-9]*[.]0
  Image: example[.]com/test@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
  Reason: (TestRiskA|MultipleReasons)
  Message: (?s:.*)This is a test risk[.] https://example[.]com/testRiskA

Updates to 4[.][0-9]*:
  VERSION  *ISSUES
  4[.][0-9]*[.]999  *no known issues relevant to this cluster
  4[.][0-9]*[.]998  *no known issues relevant to this cluster`
				}
				err = matchRegexp(out, pattern)
				o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("the actual output is \n%s", out))
			})
		})

		g.It("runs successfully with conditional recommendations to the --version target", func() {
			oc.WithKubeConfigCopy(func(oc *exutil.CLI) {
				o.Expect(oc.Run("config", "set-credentials").Args("test", "--token", token).Execute()).To(o.Succeed())
				o.Expect(oc.Run("config", "set-context").Args("--current", "--user", "test").Execute()).To(o.Succeed())

				out, err := oc.Run("--certificate-authority", caBundleFilePath, "adm", "upgrade", "recommend", "--version", fmt.Sprintf("4.%d.0", currentVersion.Minor+1), "--accept", "ConditionalUpdateRisk,Failing").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_PRECHECK", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_ACCEPT", "true").Output()
				if alertsByCVO {
					o.Expect(err).NotTo(o.BeNil(), fmt.Sprintf("the actual output is \n%s", out))
					o.Expect(err.Error()).To(o.ContainSubstring("`oc adm upgrade accept` is used to accept when the feature gate ClusterUpdateAcceptRisks is enabled"))
				} else {
					o.Expect(err).NotTo(o.HaveOccurred())
					err = matchRegexp(out, `The following conditions found no cause for concern in updating this cluster to later releases.*

Upstream update service: http://.*
Channel: test-channel [(]available channels: other-channel, test-channel[)]

Update to 4[.][0-9]*[.]0 Recommended=False:
Image: example.com/test@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
Release URL: https://example.com/release/4[.][0-9]*[.]0
Reason: accepted (TestRiskA|MultipleReasons) via ConditionalUpdateRisk
Message: (?s:.*)This is a test risk[.] https://example.com/testRiskA
Update to 4[.][0-9]*[.]0 has no known issues relevant to this cluster other than the accepted ConditionalUpdateRisk(|,Failing).`)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
			})
		})
	})
})

// alertsEvaluatedByCVO returns true if features.FeatureGateClusterUpdateAcceptRisks is enabled, and it is not a hosted cluster
// ref. https://github.com/openshift/cluster-version-operator/blob/97a5bd23f73a649f1ca8d0364eca57318f3ed274/pkg/cvo/cvo.go#L1235
func alertsEvaluatedByCVO(ctx context.Context, cv *configv1.ClusterVersion, isHyperShift bool, client configv1client.Interface) (bool, error) {
	if isHyperShift {
		return false, nil
	}
	featureGate, err := client.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	for _, fg := range featureGate.Status.FeatureGates {
		if fg.Version == cv.Status.Desired.Version {
			for _, enabledGate := range fg.Enabled {
				if enabledGate.Name == features.FeatureGateClusterUpdateAcceptRisks {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func getDefaultIngressCertificate(ctx context.Context, oc *exutil.CLI) (string, error) {
	defaultIngressSecretName, err := oc.Run("get").Args("--namespace=openshift-ingress-operator", "-o", "jsonpath={.spec.defaultCertificate.name}", "ingresscontroller.operator.openshift.io", "default").Output()
	if err != nil {
		return "", err
	}

	if defaultIngressSecretName == "" {
		defaultIngressSecretName = "router-certs-default"
	}

	ingressNamespace := "openshift-ingress"
	defaultIngressCert, err := oc.Run("extract").Args("--namespace", ingressNamespace, fmt.Sprintf("secret/%s", defaultIngressSecretName), "--keys=tls.crt", "--to=-").Output()
	if err != nil {
		return "", err
	}
	defaultIngressCert = fmt.Sprintf("%s\n", defaultIngressCert) // ensure a trailing newline, even if the earlier Output() stripped trailing newlines
	framework.Logf("default ingress certificate from the %s secret in the %s namespace: %q", defaultIngressSecretName, ingressNamespace, fmt.Sprintf("%s...", defaultIngressCert[:30]))
	return defaultIngressCert, nil
}

func getKubernetesAPIServerCertificates(ctx context.Context, oc *exutil.CLI) (string, error) {
	kubeNamespace := "openshift-kube-apiserver"
	secrets, err := oc.AdminKubeClient().CoreV1().Secrets(kubeNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	certs := make([]string, 0, len(secrets.Items))
	for _, secret := range secrets.Items {
		if secret.Type != corev1.SecretTypeTLS {
			continue
		}
		certs = append(certs, string(secret.Data["tls.crt"]))
	}

	kubeCerts := strings.Join(certs, "")
	framework.Logf("default Kubernetes certificates from TLS secrets in the %s namespace: %q", kubeNamespace, fmt.Sprintf("%s...", kubeCerts[:30]))
	return kubeCerts, nil
}
