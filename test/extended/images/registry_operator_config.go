package images

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	operatorv1 "github.com/openshift/api/operator/v1"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	imageRegistryNS     = "openshift-image-registry"
	imageRegistryConfig = "cluster"
)

// Important-only QE ports from openshift-tests-private/test/extended/image_registry.
// Phase 1: 34991, 34992, 22056, 43664, 50925, 27613, 46069/49886, 24878, 22945, 72014, 76854, 77160, 30231.
// Phase 2: 27576, 27577, 23651, 27562, 32329, 43717, 82650, 65979, 76094, 76095.
// Phase 3: 43731, 41414, 44113, 35292, 22032/50219, 24160, 10862, 46083 (SNO).
var _ = g.Describe("[sig-imageregistry][OTP][apigroup:imageregistry.operator.openshift.io] Image registry operator config", func() {
	defer g.GinkgoRecover()

	var (
		ctx = context.Background()
		oc  = exutil.NewCLI("image-registry-operator-config")
	)

	g.It("[OTP][OCP-34992] should reject invalid logLevel and operatorLogLevel [Serial]", func() {
		g.By("patching invalid logLevel")
		out, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", `{"spec":{"logLevel":"abc"}}`, "--type=merge",
		).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`Unsupported value: "abc"`))

		g.By("patching invalid operatorLogLevel")
		out, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", `{"spec":{"operatorLogLevel":"abc"}}`, "--type=merge",
		).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`Unsupported value: "abc"`))
	})

	g.It("[OTP][OCP-34991] should apply logLevel to REGISTRY_LOG_LEVEL [Serial]", func() {
		client := imageregistryClient(oc)
		defer setConfigLogLevel(ctx, client, operatorv1.Normal)

		g.By("checking default maps to info")
		env := registryLogLevelFromDeploy(oc)
		o.Expect(env).To(o.Equal("info"))

		for _, tc := range []struct {
			level   operatorv1.LogLevel
			wantEnv string
		}{
			{operatorv1.Debug, "debug"},
			{operatorv1.Trace, "debug"},
			{operatorv1.TraceAll, "debug"},
			{operatorv1.Normal, "info"},
		} {
			g.By("setting logLevel=" + string(tc.level))
			setConfigLogLevel(ctx, client, tc.level)
			o.Eventually(func() string {
				return registryLogLevelFromDeploy(oc)
			}, 3*time.Minute, 5*time.Second).Should(o.Equal(tc.wantEnv))
		}
	})

	g.It("[OTP][OCP-22056][OCP-43664][OCP-50925][OCP-76854][OCP-77160] should expose ServiceMonitor and PrometheusRules", func() {
		g.By("checking operator ServiceMonitor")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"servicemonitor", "image-registry-operator", "-n", imageRegistryNS,
			"-o=jsonpath={.spec.selector.matchLabels.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("image-registry-operator"))

		g.By("checking operand ServiceMonitor")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"servicemonitor", "image-registry", "-n", imageRegistryNS,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking storage alert rules")
		alerts, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"prometheusrule", "image-registry-operator-alerts", "-n", imageRegistryNS, "-o=yaml",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(alerts).To(o.ContainSubstring("ImageRegistryStorageReadOnly"))
		o.Expect(alerts).To(o.ContainSubstring("ImageRegistryStorageFull"))

		g.By("checking recording rules")
		isRules, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"prometheusrule", "imagestreams-rules", "-n", imageRegistryNS, "-o=yaml",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isRules).To(o.ContainSubstring("imageregistry:imagestreamtags_count:sum"))

		opsRules, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"prometheusrule", "image-registry-rules", "-n", imageRegistryNS, "-o=yaml",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(opsRules).To(o.ContainSubstring("imageregistry:operations_count:sum"))
	})

	g.It("[OTP][OCP-27613] should suspend image-pruner CronJob when ImagePruner suspend=true [Serial]", func() {
		originalSuspend, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"imagepruner/cluster", "-o=jsonpath={.spec.suspend}",
		).Output()
		o.Expect(getErr).NotTo(o.HaveOccurred())
		if originalSuspend == "" {
			originalSuspend = "false"
		}
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"imagepruner/cluster", "-p", fmt.Sprintf(`{"spec":{"suspend":%s}}`, originalSuspend), "--type=merge",
			).Execute(), "restore imagepruner suspend")
		}()

		g.By("suspending imagepruner")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"imagepruner/cluster", "-p", `{"spec":{"suspend":true}}`, "--type=merge",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for CronJob suspend")
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get cronjob suspend",
				"cronjob/image-pruner", "-n", imageRegistryNS, "-o=jsonpath={.spec.suspend}")
		}, 2*time.Minute, 2*time.Second).Should(o.Equal("true"))
	})
	g.It("[OTP][OCP-46069][OCP-49886] should override topologySpreadConstraints on the registry deployment [Serial][Disruptive]", func() {
		client := imageregistryClient(oc)
		defer clearTopologySpreadConstraints(ctx, client)

		g.By("patching custom topologySpreadConstraints")
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
			if err != nil {
				return err
			}
			cr.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{{
				MaxSkew:           2,
				TopologyKey:       "zone",
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"docker-registry": "bar"},
				},
			}}
			_, err = client.ImageregistryV1().Configs().Update(ctx, cr, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for deployment to reflect override")
		o.Eventually(func() string {
			out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"deploy/image-registry", "-n", imageRegistryNS,
				"-o=jsonpath={.spec.template.spec.topologySpreadConstraints}",
			).Output()
			if err != nil {
				return ""
			}
			return out
		}, 3*time.Minute, 5*time.Second).Should(o.And(
			o.ContainSubstring(`"topologyKey":"zone"`),
			o.ContainSubstring(`"maxSkew":2`),
			o.ContainSubstring(`"whenUnsatisfiable":"ScheduleAnyway"`),
			o.ContainSubstring(`"docker-registry":"bar"`),
		))
	})

	g.It("[OTP][OCP-24878] should mount proxy trusted CA into the registry", func() {
		trustCAName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"proxy/cluster", "-o=jsonpath={.spec.trustedCA.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if trustCAName == "" {
			g.Skip("cluster proxy has no trustedCA configured")
		}

		g.By("checking proxy CA content is present in registry trusted-ca ConfigMap")
		proxyCA, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"configmap", trustCAName, `-o=jsonpath={.data.ca-bundle\.crt}`, "-n", "openshift-config",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(proxyCA).NotTo(o.BeEmpty())

		regCA, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"configmap", "trusted-ca", `-o=jsonpath={.data.ca-bundle\.crt}`, "-n", imageRegistryNS,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(regCA).NotTo(o.BeEmpty())

		g.By("checking trusted-ca volumeMount on deployment")
		mount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deploy/image-registry", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.template.spec.containers[0].volumeMounts[?(@.name=="trusted-ca")].name}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mount).To(o.Equal("trusted-ca"))
	})

	g.It("[OTP][OCP-30231] should reject invalid virtualHostedStyle value", func() {
		out, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", `{"spec":{"storage":{"s3":{"virtualHostedStyle":"invalid"}}}}`,
			"--type=merge",
		).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.Or(
			o.ContainSubstring("Invalid value"),
			o.ContainSubstring("cannot unmarshal"),
			o.ContainSubstring("unsupported type"),
		))
	})

	g.It("[OTP][OCP-22945] should autoconfigure storage secret and degrade on invalid S3 bucket [Serial][Disruptive]", func() {
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.PlatformStatus == nil || string(infra.Status.PlatformStatus.Type) != "AWS" {
			g.Skip("OCP-22945 is AWS-specific")
		}

		g.By("checking private configuration secret exists")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"secret/image-registry-private-configuration", "-n", imageRegistryNS,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		client := imageregistryClient(oc)
		var originalBucket string
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if cr.Spec.Storage.S3 == nil {
			g.Skip("OCP-22945 requires S3 storage configured in the registry config spec")
		}
		originalBucket = cr.Spec.Storage.S3.Bucket
		defer func() {
			logBestEffort(retry.RetryOnConflict(retry.DefaultRetry, func() error {
				cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if cr.Spec.Storage.S3 == nil {
					return nil
				}
				cr.Spec.Storage.S3.Bucket = originalBucket
				_, err = client.ImageregistryV1().Configs().Update(ctx, cr, metav1.UpdateOptions{})
				return err
			}), "restore S3 bucket")
			// Storage sync failures are reported via Progressing=True; wait for recovery.
			logBestEffort(wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
				co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "image-registry", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				progressingFalse := false
				degradedFalse := false
				for _, c := range co.Status.Conditions {
					if c.Type == "Progressing" && c.Status == "False" {
						progressingFalse = true
					}
					if c.Type == "Degraded" && c.Status == "False" {
						degradedFalse = true
					}
				}
				return progressingFalse && degradedFalse, nil
			}), "wait for image-registry Progressing=False and Degraded=False")
		}()

		// Avoid the literal bucket name "invalid": that name exists as a public bucket in
		// eu-west-1 and yields BucketRegionError rather than a clean missing-bucket failure.
		// The operator reports storage sync failures as Progressing=True (not Degraded=True).
		badBucket := fmt.Sprintf("openshift-qe-nosuch-bucket-%s", rand.String(8))
		g.By(fmt.Sprintf("setting non-existent bucket %q", badBucket))
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", fmt.Sprintf(`{"spec":{"storage":{"s3":{"bucket":%q}}}}`, badBucket),
			"--type=merge",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for Progressing=True after storage sync failure")
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get co/image-registry Progressing",
				"co/image-registry", `-o=jsonpath={.status.conditions[?(@.type=="Progressing")].status}`)
		}, 2*time.Minute, 5*time.Second).Should(o.Equal("True"))
	})

	g.It("[OTP][OCP-72014] should acquire leader lease quickly after operator redeploy [Serial][Disruptive]", func() {
		g.By("checking Recreate strategy")
		strategy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deployment/cluster-image-registry-operator", "-n", imageRegistryNS,
			"-o=jsonpath={.spec.strategy.type}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strategy).To(o.Equal("Recreate"))

		g.By("setting managementState=Unmanaged")
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"configs.imageregistry.operator.openshift.io/cluster",
				"-p", `{"spec":{"managementState":"Managed"}}`, "--type=merge",
			).Execute(), "restore managementState=Managed")
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", `{"spec":{"managementState":"Unmanaged"}}`, "--type=merge",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		oldPod, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"pods", "-n", imageRegistryNS, "-l", "name=cluster-image-registry-operator",
			"--field-selector=status.phase=Running", "-o=name",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldPod).NotTo(o.BeEmpty())

		g.By("annotating operator deployment to force redeploy")
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"deploy/cluster-image-registry-operator", "-n", imageRegistryNS,
				"-p", `{"spec":{"template":{"metadata":{"annotations":{"qe-migration/test":null}}}}}`,
				"--type=merge",
			).Execute(), "clear operator redeploy annotation")
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"deploy/cluster-image-registry-operator", "-n", imageRegistryNS,
			"-p", `{"spec":{"template":{"metadata":{"annotations":{"qe-migration/test":"72014"}}}}}`,
			"--type=merge",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for a new Running operator pod")
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "list Running operator pods",
				"pods", "-n", imageRegistryNS, "-l", "name=cluster-image-registry-operator",
				"--field-selector=status.phase=Running", "-o=name")
		}, 3*time.Minute, 5*time.Second).Should(o.And(o.Not(o.BeEmpty()), o.Not(o.Equal(oldPod))))

		g.By("waiting for leader lease acquisition in new pod logs")
		o.Eventually(func() string {
			logs, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(
				"deployment.apps/cluster-image-registry-operator", "-n", imageRegistryNS,
			).Output()
			if err != nil {
				e2e.Logf("reading operator logs: %v", err)
				return ""
			}
			return logs
		}, 2*time.Minute, 5*time.Second).Should(o.And(
			o.WithTransform(strings.ToLower, o.ContainSubstring("successfully acquired lease")),
			o.ContainSubstring("openshift-image-registry/openshift-master-controllers"),
		))
	})
	// --- Phase 2: more important operator/pruner day-2 (important assertions only) ---

	g.It("[OTP][OCP-27576] should expose ImagePruner defaults and allow field updates [Serial]", func() {
		g.By("checking ImagePruner default fields")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("imagepruner/cluster", "-o=yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("failedJobsHistoryLimit: 3"))
		o.Expect(out).To(o.ContainSubstring("ignoreInvalidImageReferences: true"))
		o.Expect(out).To(o.ContainSubstring("keepTagRevisions: 3"))
		o.Expect(out).To(o.ContainSubstring("successfulJobsHistoryLimit: 3"))
		o.Expect(out).To(o.ContainSubstring("suspend: false"))

		g.By("updating ImagePruner fields")
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"imagepruner/cluster", "--type=merge", "-p",
				`{"spec":{"failedJobsHistoryLimit":3,"keepTagRevisions":3,"ignoreInvalidImageReferences":true,"schedule":"","successfulJobsHistoryLimit":3}}`,
			).Execute(), "restore ImagePruner defaults")
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"imagepruner/cluster", "--type=merge", "-p",
			`{"spec":{"failedJobsHistoryLimit":1,"keepTagRevisions":1,"ignoreInvalidImageReferences":false,"schedule":"0 0 * * *","successfulJobsHistoryLimit":1}}`,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("imagepruner/cluster", "-o=yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("failedJobsHistoryLimit: 1"))
		o.Expect(out).To(o.ContainSubstring("ignoreInvalidImageReferences: false"))
		o.Expect(out).To(o.ContainSubstring("keepTagRevisions: 1"))
		o.Expect(out).To(o.ContainSubstring("successfulJobsHistoryLimit: 1"))

		g.By("checking image-pruner CronJob exists")
		_, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("cronjob/image-pruner", "-n", imageRegistryNS).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("[OTP][OCP-27577][OCP-23651] should explain ImagePruner and Config CRDs", func() {
		g.By("oc explain imagepruners")
		result, err := oc.AsAdmin().WithoutNamespace().Run("explain").Args(
			"imagepruners", "--api-version=imageregistry.operator.openshift.io/v1",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("ImagePruner is the configuration object for an image registry pruner"))
		o.Expect(result).To(o.ContainSubstring("ImagePrunerSpec"))
		o.Expect(result).To(o.ContainSubstring("ImagePrunerStatus"))

		g.By("oc explain configs.imageregistry")
		result, err = oc.AsAdmin().WithoutNamespace().Run("explain").Args(
			"configs", "--api-version=imageregistry.operator.openshift.io/v1",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result).To(o.ContainSubstring("Config is the configuration object for a registry"))
		o.Expect(result).To(o.ContainSubstring("ImageRegistrySpec"))
		o.Expect(result).To(o.ContainSubstring("ImageRegistryStatus"))
	})

	g.It("[OTP][OCP-27562] should validate rolloutStrategy values", func() {
		g.By("checking current rolloutStrategy is valid")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-o=jsonpath={.spec.rolloutStrategy}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Or(o.Equal("RollingUpdate"), o.Equal("Recreate"), o.BeEmpty()))

		g.By("rejecting non-string rolloutStrategy")
		out, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", `{"spec":{"rolloutStrategy":123}}`, "--type=merge",
		).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("rolloutStrategy"))

		g.By("rejecting invalid string rolloutStrategy")
		out, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", `{"spec":{"rolloutStrategy":"test"}}`, "--type=merge",
		).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("RollingUpdate|Recreate"))
	})

	g.It("[OTP][OCP-32329] should honor keepYoungerThanDuration on image-pruner CronJob [Serial][Disruptive]", func() {
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"imagepruner/cluster", "-p", `{"spec":{"keepYoungerThan":null,"keepYoungerThanDuration":null}}`, "--type=merge",
			).Execute(), "clear keepYoungerThan fields")
		}()

		g.By("setting keepYoungerThan")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"imagepruner/cluster", "-p", `{"spec":{"keepYoungerThan":60}}`, "--type=merge",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get image-pruner args",
				"cronjob/image-pruner", "-n", imageRegistryNS,
				`-o=jsonpath={.spec.jobTemplate.spec.template.spec.containers[0].args}`)
		}, 2*time.Minute, 2*time.Second).Should(o.ContainSubstring("keep-younger-than=60ns"))

		g.By("setting keepYoungerThanDuration")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"imagepruner/cluster", "-p", `{"spec":{"keepYoungerThanDuration":"90s"}}`, "--type=merge",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get image-pruner args",
				"cronjob/image-pruner", "-n", imageRegistryNS,
				`-o=jsonpath={.spec.jobTemplate.spec.template.spec.containers[0].args}`)
		}, 2*time.Minute, 2*time.Second).Should(o.ContainSubstring("keep-younger-than=1m30s"))
	})

	g.It("[OTP][OCP-43717] should set system-cluster-critical priorityClass on image-pruner", func() {
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"cronjob/image-pruner", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.jobTemplate.spec.template.spec.priorityClassName}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("system-cluster-critical"))
	})

	g.It("[OTP][OCP-82650] should set readOnlyRootFilesystem on running registry pods", func() {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(imageRegistryNS).List(ctx, metav1.ListOptions{
			LabelSelector: "docker-registry=default",
			FieldSelector: "status.phase=Running",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods.Items).NotTo(o.BeEmpty())
		for _, pod := range pods.Items {
			o.Expect(pod.Spec.Containers).NotTo(o.BeEmpty())
			sc := pod.Spec.Containers[0].SecurityContext
			o.Expect(sc).NotTo(o.BeNil(), "pod %s missing securityContext", pod.Name)
			o.Expect(sc.ReadOnlyRootFilesystem).NotTo(o.BeNil(), "pod %s missing readOnlyRootFilesystem", pod.Name)
			o.Expect(*sc.ReadOnlyRootFilesystem).To(o.BeTrue(), "pod %s", pod.Name)
		}
	})

	g.It("[OTP][OCP-65979] should expose image-registry-ca for MCO without additionalTrustedCA", func() {
		cms, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmap", "-n", "openshift-config-managed").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(cms, "image-registry-ca") {
			g.Skip("No image-registry-ca ConfigMap configured")
		}

		additionalTrustedCA, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"image.config.openshift.io/cluster", "-o=jsonpath={.spec.additionalTrustedCA.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if additionalTrustedCA != "" {
			g.Skip("Cluster has additionalTrustedCA configured")
		}

		imageRegCA, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(ctx, "image-registry-ca", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		regCerts, err := oc.AdminKubeClient().CoreV1().ConfigMaps(imageRegistryNS).Get(ctx, "image-registry-certificates", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(imageRegCA.Data).NotTo(o.BeEmpty())
		o.Expect(regCerts.Data).NotTo(o.BeEmpty())
		for key := range imageRegCA.Data {
			_, found := regCerts.Data[key]
			o.Expect(found).To(o.BeTrue(), "image-registry-certificates is missing key %q", key)
		}
	})

	// CV status.desired.architecture is "" for single-arch and "Multi" for multi-arch payloads.
	// image.config status.imageStreamImportMode is reconciled by the image-registry operator
	// when ImageStreamImportMode is enabled (Legacy vs PreserveOriginal).
	g.It("[OTP][OCP-76094][OCP-76095][OCPFeatureGate:ImageStreamImportMode] should set imageStreamImportMode status for arch", func() {
		if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}
		arch, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"clusterversion/version", "-o=jsonpath={.status.desired.architecture}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		want := "Legacy"
		if arch == "Multi" {
			want = "PreserveOriginal"
			g.By("multi-arch expects PreserveOriginal [OCP-76095]")
		} else {
			g.By("single-arch expects Legacy [OCP-76094]")
		}

		// If running in a cluster where the operator doesn't auto-reconcile
		// from cluster architecture (e.g., some TechPreview setups), explicitly
		// set the spec to trigger reconciliation.
		if ocOutputOrLog(oc, "get image.config import mode",
			"image.config/cluster", "-o=jsonpath={.status.imageStreamImportMode}") == "" {
			g.By(fmt.Sprintf("status empty, setting image.config.spec.imageStreamImportMode to %s", want))
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"image.config.openshift.io/cluster",
				"-p", fmt.Sprintf(`{"spec":{"imageStreamImportMode":%q}}`, want),
				"--type=merge",
			).Execute()
			if err != nil {
				e2e.Logf("failed to patch image.config spec: %v", err)
			}
		}

		// Status is reconciled asynchronously; do not assert a single instantaneous get.
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get image.config import mode",
				"image.config/cluster", "-o=jsonpath={.status.imageStreamImportMode}")
		}, 2*time.Minute, 5*time.Second).Should(o.Equal(want))
	})

	// --- Phase 3: important parts of remaining cases ---

	g.It("[OTP][OCP-43731] should apply required anti-affinity when replicas is 2", func() {
		replicas, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"configs.imageregistry.operator.openshift.io/cluster", "-o=jsonpath={.spec.replicas}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if replicas != "2" {
			g.Skip("Skip when replicas is not 2")
		}
		affinity, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deploy/image-registry", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.template.spec.affinity.podAntiAffinity}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(affinity).To(o.ContainSubstring("requiredDuringSchedulingIgnoredDuringExecution"))
	})

	g.It("[OTP][OCP-41414] should run 2 registry replicas on HA object-storage clusters", func() {
		storage, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			`-o=jsonpath={.status.storage}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(storage, "pvc") || strings.Contains(storage, "emptyDir") {
			g.Skip("Skip for fs volume")
		}
		workers, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"nodes", "-l", "node-role.kubernetes.io/worker", "-o=name",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(strings.Fields(workers)) < 2 {
			g.Skip("Skip for SNO / single-worker cluster")
		}
		replicas, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"configs.imageregistry.operator.openshift.io/cluster", "-o=jsonpath={.spec.replicas}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(replicas).To(o.Equal("2"))
		pods, err := oc.AdminKubeClient().CoreV1().Pods(imageRegistryNS).List(ctx, metav1.ListOptions{
			LabelSelector: "docker-registry=default",
			FieldSelector: "status.phase=Running",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(2))
	})

	g.It("[OTP][OCP-44113] should apply custom tolerations to image-pruner CronJob [Serial]", func() {
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"imagepruner/cluster", "-p", `{"spec":{"tolerations":null}}`, "--type=merge",
			).Execute(), "clear imagepruner tolerations")
		}()
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"imagepruner/cluster", "--type=merge", "-p",
			`{"spec":{"tolerations":[{"effect":"NoSchedule","key":"key","operator":"Equal","value":"value"}]}}`,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get image-pruner tolerations",
				"cronjob/image-pruner", "-n", imageRegistryNS,
				`-o=jsonpath={.spec.jobTemplate.spec.template.spec.tolerations}`)
		}, 2*time.Minute, 2*time.Second).Should(o.And(
			o.ContainSubstring(`"key":"key"`),
			o.ContainSubstring(`"value":"value"`),
			o.ContainSubstring(`"effect":"NoSchedule"`),
			o.ContainSubstring(`"operator":"Equal"`),
		))
	})

	g.It("[OTP][OCP-35292] should map ImagePruner logLevel into CronJob --loglevel [Serial]", func() {
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"imagepruner/cluster", "-p", `{"spec":{"logLevel":"Normal"}}`, "--type=merge",
			).Execute(), "restore imagepruner logLevel")
		}()
		// CIRO uses library-go LogLevelToVerbosity (Debug=4, Trace=6, TraceAll=8)
		// but remaps Normal from 2→1 so `oc adm prune` does not dump stack traces.
		for _, tc := range []struct {
			level string
			want  string
		}{
			{"Debug", "--loglevel=4"},
			{"Trace", "--loglevel=6"},
			{"TraceAll", "--loglevel=8"},
			{"Normal", "--loglevel=1"},
		} {
			g.By("logLevel=" + tc.level)
			err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"imagepruner/cluster", "--type=merge",
				"-p", fmt.Sprintf(`{"spec":{"logLevel":"%s"}}`, tc.level),
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Eventually(func() string {
				return ocOutputOrLog(oc, "get image-pruner args",
					"cronjob/image-pruner", "-n", imageRegistryNS,
					`-o=jsonpath={.spec.jobTemplate.spec.template.spec.containers[0].args}`)
			}, 2*time.Minute, 2*time.Second).Should(o.ContainSubstring(tc.want))
		}
	})

	g.It("[OTP][OCP-22032][OCP-50219] should honor nodeSelector and tolerations and clear topology constraints [Serial][Disruptive]", func() {
		defer func() {
			logBestEffort(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"configs.imageregistry.operator.openshift.io/cluster", "--type=merge",
				"-p", `{"spec":{"nodeSelector":null,"tolerations":null}}`,
			).Execute(), "clear nodeSelector/tolerations")
		}()

		g.By("setting master nodeSelector + tolerations")
		err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster", "--type=merge", "-p",
			`{"spec":{"nodeSelector":{"node-role.kubernetes.io/master":""},"tolerations":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"}]}}`,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get registry nodeSelector",
				"deploy/image-registry", "-n", imageRegistryNS,
				`-o=jsonpath={.spec.template.spec.nodeSelector}`)
		}, 3*time.Minute, 5*time.Second).Should(o.ContainSubstring("node-role.kubernetes.io/master"))

		topo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deploy/image-registry", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.template.spec.topologySpreadConstraints}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(topo).NotTo(o.ContainSubstring("topology.kubernetes.io/zone"))
		o.Expect(topo).NotTo(o.ContainSubstring("kubernetes.io/hostname"))
	})

	g.It("[OTP][OCP-24160] should set ImageStream lookupPolicy.local via oc set image-lookup", func() {
		ns := oc.Namespace()
		err := oc.AsAdmin().WithoutNamespace().Run("import-image").Args(
			"is24160-lookup", "--from=quay.io/openshifttest/base-alpine@sha256:3126e4eed4a3ebd8bf972b2453fa838200988ee07c01b2251e3ea47e4b1f245c",
			"--confirm=true", "-n", ns,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() error {
			return oc.AsAdmin().WithoutNamespace().Run("get").Args("istag/is24160-lookup:latest", "-n", ns).Execute()
		}, 2*time.Minute, 5*time.Second).Should(o.Succeed())

		err = oc.AsAdmin().WithoutNamespace().Run("set").Args("image-lookup", "is24160-lookup", "-n", ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		local, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"imagestreams", "is24160-lookup", "-n", ns, "-o=jsonpath={.spec.lookupPolicy.local}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(local).To(o.Equal("true"))
	})

	g.It("[OTP][OCP-10862] should delete ImageStream spec tags with oc tag --delete", func() {
		ns := oc.Namespace()
		err := oc.AsAdmin().WithoutNamespace().Run("tag").Args(
			"quay.io/openshifttest/base-alpine@sha256:3126e4eed4a3ebd8bf972b2453fa838200988ee07c01b2251e3ea47e4b1f245c",
			"mystream:v1", "mystream:latest", "--import-mode=PreserveOriginal", "--source=docker", "-n", ns,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() error {
			return oc.AsAdmin().WithoutNamespace().Run("get").Args("istag/mystream:v1", "-n", ns).Execute()
		}, 2*time.Minute, 5*time.Second).Should(o.Succeed())

		err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(
			"mystream:v1", "mystream:latest", "--delete=true", "-n", ns,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.AsAdmin().WithoutNamespace().Run("tag").Args(
			"mystream:nonexist", "--delete=true", "-n", ns,
		).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`not found`))
	})

	g.It("[OTP][OCP-46083] should keep default topology constraints on SNO", func() {
		masters, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"nodes", "-l", "node-role.kubernetes.io/master", "-o=jsonpath={.items[*].metadata.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		workers, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"nodes", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[*].metadata.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		mList, wList := strings.Fields(masters), strings.Fields(workers)
		if !(len(mList) == 1 && len(wList) == 1 && mList[0] == wList[0]) {
			g.Skip("Not an SNO cluster")
		}
		topo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deploy/image-registry", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.template.spec.topologySpreadConstraints}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(topo).To(o.ContainSubstring("topology.kubernetes.io/zone"))
		o.Expect(topo).To(o.ContainSubstring("kubernetes.io/hostname"))
		o.Expect(topo).To(o.ContainSubstring("node-role.kubernetes.io/worker"))
	})
})

func imageregistryClient(oc *exutil.CLI) *imageregistry.Clientset {
	client, err := imageregistry.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}

func setConfigLogLevel(ctx context.Context, client *imageregistry.Clientset, level operatorv1.LogLevel) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		if err != nil {
			return err
		}
		cr.Spec.LogLevel = level
		_, err = client.ImageregistryV1().Configs().Update(ctx, cr, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func clearTopologySpreadConstraints(ctx context.Context, client *imageregistry.Clientset) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		if err != nil {
			return err
		}
		cr.Spec.TopologySpreadConstraints = nil
		_, err = client.ImageregistryV1().Configs().Update(ctx, cr, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		e2e.Logf("failed to clear topologySpreadConstraints: %v", err)
	}
}

func registryLogLevelFromDeploy(oc *exutil.CLI) string {
	return ocOutputOrLog(oc, "get REGISTRY_LOG_LEVEL",
		"deploy/image-registry", "-n", imageRegistryNS,
		`-o=jsonpath={.spec.template.spec.containers[0].env[?(@.name=="REGISTRY_LOG_LEVEL")].value}`)
}

func logBestEffort(err error, action string) {
	if err != nil {
		e2e.Logf("best-effort %s failed: %v", action, err)
	}
}

func ocOutputOrLog(oc *exutil.CLI, action string, args ...string) string {
	out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Output()
	if err != nil {
		e2e.Logf("%s: %v", action, err)
		return ""
	}
	return out
}
