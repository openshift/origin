package adm_upgrade

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/blang/semver/v4"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Serial][sig-cli] oc adm upgrade recommend", g.Ordered, func() {
	defer g.GinkgoRecover()
	ctx := context.Background()

	f := framework.NewDefaultFramework("oc-adm-upgrade-recommend")
	oc := exutil.NewCLIWithFramework(f).AsAdmin()
	var cv *configv1.ClusterVersion
	var restoreChannel, restoreUpstream bool
	var caBundleFilePath string

	g.BeforeAll(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("MicroShift does not have a ClusterVersion resource")
		}

		cv, err = oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
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

			graph := fmt.Sprintf(`{"nodes": [{"version": "%s","payload": "%s", "metadata": {"io.openshift.upgrades.graph.release.channels": "test-channel,other-channel"}}]}`, cv.Status.Desired.Version, cv.Status.Desired.Image)
			newUpstream, err := runUpdateService(ctx, oc, graph)
			o.Expect(err).NotTo(o.HaveOccurred())

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
			isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isHyperShift {
				g.Skip("HyperShift does not support configuring the upstream OpenShift Update Service directoly via ClusterVersion (it must be configured via HostedCluster on the management cluster)")
			}

			if curVer, err := semver.Parse(cv.Status.Desired.Version); err != nil {
				o.Expect(err).NotTo(o.HaveOccurred())
			} else {
				currentVersion = &curVer
			}

			var buf strings.Builder
			err = template.Must(template.New("letter").Parse(`{
  "nodes": [
    {"version": "{{.CurrentVersion}}","payload": "{{.CurrentImage}}", "metadata": {"io.openshift.upgrades.graph.release.channels": "test-channel,other-channel"}},
    {"version": "4.{{.CurrentMinor}}.998","payload": "example.com/test@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
    {"version": "4.{{.CurrentMinor}}.999","payload": "example.com/test@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
    {"version": "4.{{.NextMinor}}.0","payload": "example.com/test@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", "metadata": {"url": "https://example.com/release/4.{{.NextMinor}}.0"}}
  ],
  "edges": [[0, 1], [0, 2]],
  "conditionalEdges": [
    {
      "edges": [{"from": "{{.CurrentVersion}}", "to": "4.{{.NextMinor}}.0"}],
      "risks": [
        {
          "url": "https://example.com/testRiskA",
          "name": "TestRiskA",
          "message": "This is a test risk.",
          "matchingRules": [{"type": "PromQL", "promql": {"promql": "group(cluster_version)"}}]
        }
      ]
    }
  ]
}`)).Execute(&buf, struct {
				CurrentVersion string
				CurrentImage   string
				CurrentMinor   uint64
				NextMinor      uint64
			}{
				CurrentVersion: cv.Status.Desired.Version,
				CurrentImage:   cv.Status.Desired.Image,
				CurrentMinor:   currentVersion.Minor,
				NextMinor:      currentVersion.Minor + 1,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			graph := buf.String()

			newUpstream, err := runUpdateService(ctx, oc, graph)
			o.Expect(err).NotTo(o.HaveOccurred())

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
				err = matchRegexp(out, `The following conditions found no cause for concern in updating this cluster to later releases.*

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
  4[.][0-9]*[.]998  *no known issues relevant to this cluster`)
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})

		g.It("runs successfully with conditional recommendations to the --version target", func() {
			oc.WithKubeConfigCopy(func(oc *exutil.CLI) {
				o.Expect(oc.Run("config", "set-credentials").Args("test", "--token", token).Execute()).To(o.Succeed())
				o.Expect(oc.Run("config", "set-context").Args("--current", "--user", "test").Execute()).To(o.Succeed())

				out, err := oc.Run("--certificate-authority", caBundleFilePath, "adm", "upgrade", "recommend", "--version", fmt.Sprintf("4.%d.0", currentVersion.Minor+1), "--accept", "ConditionalUpdateRisk,Failing").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_PRECHECK", "true").EnvVar("OC_ENABLE_CMD_UPGRADE_RECOMMEND_ACCEPT", "true").Output()

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
			})
		})
	})
})

func runUpdateService(ctx context.Context, oc *exutil.CLI, graph string) (*url.URL, error) {
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()).Create(ctx,
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-update-service-",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-update-service",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-update-service",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "update-service",
							Image: image.ShellImage(),
							Args: []string{
								"/bin/sh",
								"-c",
								fmt.Sprintf(`DIR="$(mktemp -d)" &&
cd "${DIR}" &&
printf '%%s' '%s' >graph &&
python3 -m http.server --bind ::
`, graph),
							},
							Ports: []corev1.ContainerPort{{
								Name:          "update-service",
								ContainerPort: 8000,
							}},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("20Mi"),
								},
							},
						}},
					},
				},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	service, err := oc.AdminKubeClient().CoreV1().Services(oc.Namespace()).Create(ctx,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: deployment.ObjectMeta.Name,
			},
			Spec: corev1.ServiceSpec{
				Selector: deployment.Spec.Template.ObjectMeta.Labels,
				Ports: []corev1.ServicePort{{
					Name: deployment.Spec.Template.Spec.Containers[0].Ports[0].Name,
					Port: deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort,
				}},
			},
		}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	if err = exutil.WaitForDeploymentReady(oc, deployment.ObjectMeta.Name, oc.Namespace(), -1); err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(service.Spec.ClusterIP, strconv.Itoa(int(service.Spec.Ports[0].Port))),
		Path:   "graph",
	}, nil
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
