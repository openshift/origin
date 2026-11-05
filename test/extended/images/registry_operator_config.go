package images

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	imageregistryv1 "github.com/openshift/api/imageregistry/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	imageRegistryNS     = "openshift-image-registry"
	imageRegistryConfig = "cluster"
)

// Important-only QE ports from openshift-tests-private/test/extended/image_registry.
// Dropped during review: 34992, 30231, 27562 (pure CRD schema validation, better
// suited to unit tests than a live cluster), 76094/76095 (merged into
// tagimportmode.go), 24160 (covered by the [Feature:ImageLookup] tests in resolve.go).
// Phase 1: 34991, 22056, 43664, 50925, 27613, 46069/49886, 24878, 22945, 72014, 76854, 77160.
// Phase 2: 27576, 27577, 23651, 32329, 43717, 82650, 65979.
// Phase 3: 43731, 41414, 44113, 35292, 22032/50219, 10862, 46083 (SNO).
var _ = g.Describe("[sig-imageregistry][OTP][apigroup:imageregistry.operator.openshift.io] Image registry operator config", func() {
	var (
		ctx = context.Background()
		oc  = exutil.NewCLI("image-registry-operator-config")
	)

	g.It("[OTP][OCP-34991] should apply logLevel to REGISTRY_LOG_LEVEL [Serial]", ote.Informing(), func() {
		client := imageregistryClient(oc)
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalLogLevel := cr.Spec.LogLevel
		defer setConfigLogLevel(ctx, client, originalLogLevel)

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

	g.It("[OTP][OCP-22056][OCP-43664][OCP-50925][OCP-76854][OCP-77160] should expose ServiceMonitor and PrometheusRules", ote.Informing(), func() {
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

	g.It("[OTP][OCP-27613] should suspend image-pruner CronJob when ImagePruner suspend=true [Serial]", ote.Informing(), func() {
		originalSuspend, getErr := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"imagepruner/cluster", "-o=jsonpath={.spec.suspend}",
		).Output()
		o.Expect(getErr).NotTo(o.HaveOccurred())
		if originalSuspend == "" {
			originalSuspend = "false"
		}
		defer func() {
			o.Expect(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"imagepruner/cluster", "-p", fmt.Sprintf(`{"spec":{"suspend":%s}}`, originalSuspend), "--type=merge",
			).Execute()).NotTo(o.HaveOccurred(), "should restore imagepruner suspend")
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
	g.It("[OTP][OCP-46069][OCP-49886] should override topologySpreadConstraints on the registry deployment [Serial]", ote.Informing(), func() {
		client := imageregistryClient(oc)
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalConstraints := cr.Spec.TopologySpreadConstraints
		defer func() {
			o.Expect(updateConfigSpec(ctx, client, func(spec *imageregistryv1.ImageRegistrySpec) {
				spec.TopologySpreadConstraints = originalConstraints
			})).NotTo(o.HaveOccurred(), "should restore topologySpreadConstraints")
		}()

		g.By("patching custom topologySpreadConstraints")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
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

	g.It("[OTP][OCP-24878] should mount proxy trusted CA into the registry", ote.Informing(), func() {
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

	g.It("[OTP][OCP-22945] should autoconfigure storage secret and fail storage sync on invalid S3 bucket [Serial]", ote.Informing(), func() {
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
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if cr.Spec.Storage.S3 == nil {
			g.Skip("OCP-22945 requires S3 storage configured in the registry config spec")
		}
		// Snapshot the whole storage spec: during CreateStorage the operator
		// writes back spec.storage (e.g. managementState and the effective S3
		// configuration), so restoring only the bucket would leave those
		// write-backs behind.
		originalStorage := cr.Spec.Storage.DeepCopy()
		defer func() {
			o.Expect(updateConfigSpec(ctx, client, func(spec *imageregistryv1.ImageRegistrySpec) {
				spec.Storage = *originalStorage.DeepCopy()
			})).NotTo(o.HaveOccurred(), "should restore the registry storage spec")
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

		// S3 bucket names are limited to 63 characters, so a longer name can
		// neither exist nor be created: AWS rejects it with InvalidBucketName
		// and the operator's storage sync fails. A syntactically valid
		// "nonexistent" name would instead be provisioned by the operator.
		// The operator reports storage sync failures as Progressing=True
		// (not Degraded=True) with the sync error in the condition message.
		badBucket := fmt.Sprintf("openshift-qe-nosuch-bucket-%s", strings.Repeat("x", 40))
		g.By(fmt.Sprintf("setting invalid bucket name %q", badBucket))
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster",
			"-p", fmt.Sprintf(`{"spec":{"storage":{"s3":{"bucket":%q}}}}`, badBucket),
			"--type=merge",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the storage sync failure to surface in Progressing")
		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get co/image-registry Progressing condition",
				"co/image-registry", `-o=jsonpath={.status.conditions[?(@.type=="Progressing")].status}{" "}{.status.conditions[?(@.type=="Progressing")].message}`)
		}, 2*time.Minute, 5*time.Second).Should(o.And(
			o.ContainSubstring("True"),
			o.ContainSubstring("unable to sync storage configuration"),
			o.ContainSubstring("InvalidBucketName"),
		))
	})

	g.It("[OTP][OCP-72014] should acquire leader lease quickly after operator redeploy [Serial]", ote.Informing(), func() {
		g.By("checking Recreate strategy")
		strategy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deployment/cluster-image-registry-operator", "-n", imageRegistryNS,
			"-o=jsonpath={.spec.strategy.type}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strategy).To(o.Equal("Recreate"))

		oldPod, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"pods", "-n", imageRegistryNS, "-l", "name=cluster-image-registry-operator",
			"--field-selector=status.phase=Running", "-o=name",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oldPod).NotTo(o.BeEmpty())

		g.By("annotating operator deployment to force redeploy")
		defer func() {
			o.Expect(oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"deploy/cluster-image-registry-operator", "-n", imageRegistryNS,
				"-p", `{"spec":{"template":{"metadata":{"annotations":{"qe-migration/test":null}}}}}`,
				"--type=merge",
			).Execute()).NotTo(o.HaveOccurred(), "should clear operator redeploy annotation")
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

	g.It("[OTP][OCP-27576] should expose ImagePruner defaults and allow field updates [Serial]", ote.Informing(), func() {
		g.By("checking ImagePruner default fields")
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("imagepruner/cluster", "-o=yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("failedJobsHistoryLimit: 3"))
		o.Expect(out).To(o.ContainSubstring("ignoreInvalidImageReferences: true"))
		o.Expect(out).To(o.ContainSubstring("keepTagRevisions: 3"))
		o.Expect(out).To(o.ContainSubstring("successfulJobsHistoryLimit: 3"))
		o.Expect(out).To(o.ContainSubstring("suspend: false"))

		g.By("updating ImagePruner fields")
		prunerClient := imageregistryClient(oc)
		originalPrunerSpec := imagePrunerSpec(ctx, prunerClient)
		defer restoreImagePrunerSpec(ctx, prunerClient, originalPrunerSpec)
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
		o.Expect(out).To(o.ContainSubstring("schedule: 0 0 * * *"))
		o.Expect(out).To(o.ContainSubstring("successfulJobsHistoryLimit: 1"))

		g.By("checking the updates are reflected in the image-pruner CronJob")
		o.Eventually(func(gomega o.Gomega) {
			cronjob, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"cronjob/image-pruner", "-n", imageRegistryNS, "-o=yaml",
			).Output()
			gomega.Expect(err).NotTo(o.HaveOccurred())
			gomega.Expect(cronjob).To(o.ContainSubstring("schedule: 0 0 * * *"))
			gomega.Expect(cronjob).To(o.ContainSubstring("failedJobsHistoryLimit: 1"))
			gomega.Expect(cronjob).To(o.ContainSubstring("successfulJobsHistoryLimit: 1"))
			gomega.Expect(cronjob).To(o.ContainSubstring("--keep-tag-revisions=1"))
			gomega.Expect(cronjob).To(o.ContainSubstring("--ignore-invalid-refs=false"))
		}, 2*time.Minute, 5*time.Second).Should(o.Succeed())
	})

	g.It("[OTP][OCP-27577][OCP-23651] should explain ImagePruner and Config CRDs", ote.Informing(), func() {
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

	g.It("[OTP][OCP-32329] should honor keepYoungerThanDuration on image-pruner CronJob [Serial]", ote.Informing(), func() {
		prunerClient := imageregistryClient(oc)
		originalPrunerSpec := imagePrunerSpec(ctx, prunerClient)
		defer restoreImagePrunerSpec(ctx, prunerClient, originalPrunerSpec)

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

	g.It("[OTP][OCP-43717] should set system-cluster-critical priorityClass on image-pruner", ote.Informing(), func() {
		out, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"cronjob/image-pruner", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.jobTemplate.spec.template.spec.priorityClassName}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("system-cluster-critical"))
	})

	g.It("[OTP][OCP-82650] should set readOnlyRootFilesystem on running registry pods", ote.Informing(), func() {
		pods, err := oc.AdminKubeClient().CoreV1().Pods(imageRegistryNS).List(ctx, metav1.ListOptions{
			LabelSelector: "docker-registry=default",
			FieldSelector: "status.phase=Running",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(pods.Items) == 0 {
			g.Skip("no running registry pods; the registry may be scaled to zero or its storage unconfigured")
		}
		for _, pod := range pods.Items {
			o.Expect(pod.Spec.Containers).NotTo(o.BeEmpty())
			sc := pod.Spec.Containers[0].SecurityContext
			o.Expect(sc).NotTo(o.BeNil(), "pod %s missing securityContext", pod.Name)
			o.Expect(sc.ReadOnlyRootFilesystem).NotTo(o.BeNil(), "pod %s missing readOnlyRootFilesystem", pod.Name)
			o.Expect(*sc.ReadOnlyRootFilesystem).To(o.BeTrue(), "pod %s", pod.Name)
		}
	})

	g.It("[OTP][OCP-65979] should expose image-registry-ca for MCO without additionalTrustedCA", ote.Informing(), func() {
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

	// --- Phase 3: important parts of remaining cases ---

	g.It("[OTP][OCP-43731] should apply required anti-affinity when replicas is 2", ote.Informing(), func() {
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

	g.It("[OTP][OCP-41414] should run 2 registry replicas on HA object-storage clusters", ote.Informing(), func() {
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
		// The replica count settles as the deployment rolls out; an
		// instantaneous read can transiently observe the surge or a
		// replacement pod.
		o.Eventually(func(gomega o.Gomega) {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(imageRegistryNS).List(ctx, metav1.ListOptions{
				LabelSelector: "docker-registry=default",
				FieldSelector: "status.phase=Running",
			})
			gomega.Expect(err).NotTo(o.HaveOccurred())
			gomega.Expect(pods.Items).To(o.HaveLen(2), "expected 2 running registry pods")
		}, 3*time.Minute, 5*time.Second).Should(o.Succeed())
	})

	g.It("[OTP][OCP-44113] should apply custom tolerations to image-pruner CronJob [Serial]", ote.Informing(), func() {
		prunerClient := imageregistryClient(oc)
		originalPrunerSpec := imagePrunerSpec(ctx, prunerClient)
		defer restoreImagePrunerSpec(ctx, prunerClient, originalPrunerSpec)
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

	g.It("[OTP][OCP-35292] should map ImagePruner logLevel into CronJob --loglevel [Serial]", ote.Informing(), func() {
		prunerClient := imageregistryClient(oc)
		originalPrunerSpec := imagePrunerSpec(ctx, prunerClient)
		defer restoreImagePrunerSpec(ctx, prunerClient, originalPrunerSpec)
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

	g.It("[OTP][OCP-22032][OCP-50219] should honor nodeSelector and tolerations and clear topology constraints [Serial]", ote.Informing(), func() {
		client := imageregistryClient(oc)
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		originalNodeSelector := cr.Spec.NodeSelector
		originalTolerations := cr.Spec.Tolerations
		defer func() {
			o.Expect(updateConfigSpec(ctx, client, func(spec *imageregistryv1.ImageRegistrySpec) {
				spec.NodeSelector = originalNodeSelector
				spec.Tolerations = originalTolerations
			})).NotTo(o.HaveOccurred(), "should restore nodeSelector and tolerations")
		}()

		g.By("setting master nodeSelector + tolerations")
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args(
			"configs.imageregistry.operator.openshift.io/cluster", "--type=merge", "-p",
			`{"spec":{"nodeSelector":{"node-role.kubernetes.io/master":""},"tolerations":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/master","operator":"Exists"}]}}`,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get registry nodeSelector",
				"deploy/image-registry", "-n", imageRegistryNS,
				`-o=jsonpath={.spec.template.spec.nodeSelector}`)
		}, 3*time.Minute, 5*time.Second).Should(o.ContainSubstring("node-role.kubernetes.io/master"))

		o.Eventually(func() string {
			return ocOutputOrLog(oc, "get registry tolerations",
				"deploy/image-registry", "-n", imageRegistryNS,
				`-o=jsonpath={.spec.template.spec.tolerations}`)
		}, 3*time.Minute, 5*time.Second).Should(o.And(
			o.ContainSubstring(`"key":"node-role.kubernetes.io/master"`),
			o.ContainSubstring(`"effect":"NoSchedule"`),
			o.ContainSubstring(`"operator":"Exists"`),
		))

		topo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deploy/image-registry", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.template.spec.topologySpreadConstraints}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(topo).NotTo(o.ContainSubstring("topology.kubernetes.io/zone"))
		o.Expect(topo).NotTo(o.ContainSubstring("kubernetes.io/hostname"))
	})

	g.It("[OTP][OCP-10862] should delete ImageStream spec tags with oc tag --delete [Skipped:Disconnected]", ote.Informing(), func() {
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

		g.By("verifying the latest tag is removed from the ImageStream")
		o.Eventually(func(gomega o.Gomega) {
			tags, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"imagestreams", "mystream", "-n", ns, `-o=jsonpath={.spec.tags[*].name}`,
			).Output()
			gomega.Expect(err).NotTo(o.HaveOccurred())
			gomega.Expect(tags).NotTo(o.ContainSubstring("latest"))
		}, 2*time.Minute, 5*time.Second).Should(o.Succeed())

		err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(
			"mystream:v1", "--delete=true", "-n", ns,
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("verifying the v1 tag is removed from the ImageStream")
		o.Eventually(func(gomega o.Gomega) {
			tags, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"imagestreams", "mystream", "-n", ns, `-o=jsonpath={.spec.tags[*].name}`,
			).Output()
			gomega.Expect(err).NotTo(o.HaveOccurred())
			gomega.Expect(tags).NotTo(o.ContainSubstring("v1"))
		}, 2*time.Minute, 5*time.Second).Should(o.Succeed())

		out, err := oc.AsAdmin().WithoutNamespace().Run("tag").Args(
			"mystream:nonexist", "--delete=true", "-n", ns,
		).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`not found`))
	})

	g.It("[OTP][OCP-46083] should keep default topology constraints on SNO", ote.Informing(), func() {
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
		// A custom nodeSelector or topologySpreadConstraints in the registry
		// config replaces the operator's default constraints.
		if ocOutputOrLog(oc, "get registry nodeSelector",
			"configs.imageregistry.operator.openshift.io/cluster", "-o=jsonpath={.spec.nodeSelector}") != "" ||
			ocOutputOrLog(oc, "get registry topologySpreadConstraints",
				"configs.imageregistry.operator.openshift.io/cluster", "-o=jsonpath={.spec.topologySpreadConstraints}") != "" {
			g.Skip("registry config has a custom nodeSelector or topologySpreadConstraints")
		}
		topo, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"deploy/image-registry", "-n", imageRegistryNS,
			`-o=jsonpath={.spec.template.spec.topologySpreadConstraints}`,
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// The operator only emits the zone constraint when at least one node
		// carries the zone label; bare-metal/agent-based SNO clusters commonly
		// have none, which is a valid configuration.
		zonedNodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"nodes", "-l", "topology.kubernetes.io/zone", "-o=name",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(strings.Fields(zonedNodes)) > 0 {
			o.Expect(topo).To(o.ContainSubstring("topology.kubernetes.io/zone"))
		}
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
	o.Expect(updateConfigSpec(ctx, client, func(spec *imageregistryv1.ImageRegistrySpec) {
		spec.LogLevel = level
	})).NotTo(o.HaveOccurred())
}

// updateConfigSpec retries on conflict while applying mutate to the spec of
// the cluster image registry config.
func updateConfigSpec(ctx context.Context, client *imageregistry.Clientset, mutate func(spec *imageregistryv1.ImageRegistrySpec)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cr, err := client.ImageregistryV1().Configs().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		if err != nil {
			return err
		}
		mutate(&cr.Spec)
		_, err = client.ImageregistryV1().Configs().Update(ctx, cr, metav1.UpdateOptions{})
		return err
	})
}

// imagePrunerSpec snapshots the spec of the cluster ImagePruner so tests can
// restore the exact original state, including unset fields.
func imagePrunerSpec(ctx context.Context, client *imageregistry.Clientset) *imageregistryv1.ImagePrunerSpec {
	pruner, err := client.ImageregistryV1().ImagePruners().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return pruner.Spec.DeepCopy()
}

// restoreImagePrunerSpec restores a snapshot taken by imagePrunerSpec, failing
// the test when the pruner cannot be returned to its original state.
func restoreImagePrunerSpec(ctx context.Context, client *imageregistry.Clientset, original *imageregistryv1.ImagePrunerSpec) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pruner, err := client.ImageregistryV1().ImagePruners().Get(ctx, imageRegistryConfig, metav1.GetOptions{})
		if err != nil {
			return err
		}
		pruner.Spec = *original.DeepCopy()
		_, err = client.ImageregistryV1().ImagePruners().Update(ctx, pruner, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "should restore the imagepruner spec")
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
