package prometheus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test"

	"golang.org/x/sync/errgroup"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	v1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/yaml"

	configv1 "github.com/openshift/api/config/v1"

	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
)

// ClusterMonitoringConfiguration is a subset of https://github.com/openshift/cluster-monitoring-operator/blob/8d331d78b22948d36c20da0552763ddd8a4e2093/pkg/manifests/config.go#L124-L136
type ClusterMonitoringConfiguration struct {
	TelemeterClientConfig *TelemeterClientConfig `json:"telemeterClient"`
}

// TelemeterClientConfig is a subset of https://github.com/openshift/cluster-monitoring-operator/blob/8d331d78b22948d36c20da0552763ddd8a4e2093/pkg/manifests/config.go#L335-L342
type TelemeterClientConfig struct {
	Enabled *bool `json:"enabled"`
}

// Set $MONITORING_AUTH_TEST_NAMESPACE to focus on the targets from a single namespace
var namespaceUnderTest = os.Getenv("MONITORING_AUTH_TEST_NAMESPACE")

var _ = g.Describe("[sig-instrumentation][Late] Platform Prometheus targets", func() {
	defer g.GinkgoRecover()
	var (
		oc                         = exutil.NewCLIWithPodSecurityLevel("prometheus", admissionapi.LevelBaseline)
		prometheusURL, bearerToken string

		// TODO: remove the namespace when the bug is fixed.
		namespacesToSkip = sets.New[string]("openshift-marketplace", // https://issues.redhat.com/browse/OCPBUGS-59763
			"openshift-image-registry",               // https://issues.redhat.com/browse/OCPBUGS-59767
			"openshift-operator-lifecycle-manager",   // https://issues.redhat.com/browse/OCPBUGS-59768
			"openshift-cluster-samples-operator",     // https://issues.redhat.com/browse/OCPBUGS-59769
			"openshift-cluster-csi-drivers",          // https://issues.redhat.com/browse/OCPBUGS-60159
			"openshift-cluster-node-tuning-operator", // https://issues.redhat.com/browse/OCPBUGS-60258
			"openshift-etcd",                         // https://issues.redhat.com/browse/OCPBUGS-60263
		)
	)

	g.BeforeEach(func(ctx g.SpecContext) {
		var err error

		kubeClient, err := kubernetes.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		nsExist, err := exutil.IsNamespaceExist(kubeClient, "openshift-monitoring")
		o.Expect(err).NotTo(o.HaveOccurred())
		if !nsExist {
			g.Skip("openshift-monitoring namespace does not exist, skipping")
		}

		prometheusURL, err = helper.PrometheusRouteURL(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Get public url of prometheus")
		bearerToken, err = helper.RequestPrometheusServiceAccountAPIToken(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Request prometheus service account API token")

		if namespacesToSkip.Has(namespaceUnderTest) {
			e2e.Logf("The namespace %s is not skipped because $MONITORING_AUTH_TEST_NAMESPACE is set to it", namespaceUnderTest)
			namespacesToSkip.Delete(namespaceUnderTest)
		}
	})

	g.It("should not be accessible without auth [Serial]", test.ExtendedDuration(), func() {
		expectedStatusCodes := sets.New(http.StatusUnauthorized, http.StatusForbidden)

		g.By("checking that targets reject the requests with 401 or 403")
		execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), oc.Namespace(), "execpod-targets-authorization")
		defer func() {
			err := oc.AdminKubeClient().CoreV1().Pods(execPod.Namespace).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Delete pod %s/%s", execPod.Namespace, execPod.Name))
		}()

		promTargets := func() (*prometheusTargets, error) {
			contents, err := helper.GetURLWithToken(helper.MustJoinUrlPath(prometheusURL, "api/v1/targets"), bearerToken)
			if err != nil {
				return nil, err
			}
			targets := &prometheusTargets{}
			err = json.Unmarshal([]byte(contents), targets)
			if err != nil {
				return nil, err
			}
			// sanity check.
			if len(targets.Data.ActiveTargets) < 5 {
				return nil, fmt.Errorf("only got %d targets, something is wrong", len(targets.Data.ActiveTargets))
			}
			return targets, nil
		}

		initialPromTargets, err := promTargets()
		o.Expect(err).NotTo(o.HaveOccurred())
		eg := errgroup.Group{}
		eg.SetLimit(runtime.GOMAXPROCS(0))
		errChan := make(chan error, len(initialPromTargets.Data.ActiveTargets))
		for _, target := range initialPromTargets.Data.ActiveTargets {
			eg.Go(func() error {
				targetNs, targetJob, targetPod, targetScrapeURL := target.Labels["namespace"], target.Labels["job"], target.Labels["pod"], target.ScrapeUrl
				o.Expect(targetNs).NotTo(o.BeEmpty())
				if namespaceUnderTest != "" && targetNs != namespaceUnderTest {
					return nil
				}
				scrapeErr := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 5*time.Minute, true, func(context.Context) (bool, error) {
					statusCode, err := helper.URLStatusCodeExecViaPod(execPod.Namespace, execPod.Name, targetScrapeURL)
					e2e.Logf("scraping target %s of pod %s/%s/%s without auth returned %d, err: %v (skip=%t)", targetScrapeURL, targetNs, targetJob, targetPod, statusCode, err, namespacesToSkip.Has(targetNs))
					if expectedStatusCodes.Has(statusCode) {
						return true, nil
					}

					// retry
					if err != nil ||
						statusCode/100 == 5 ||
						statusCode == http.StatusRequestTimeout ||
						statusCode == http.StatusTooManyRequests {
						return false, nil
					}
					return false, fmt.Errorf("expecting status code %v but returned %d", expectedStatusCodes.UnsortedList(), statusCode)
				})

				// Ignoring targets that Prometheus no longer scrapes or fails to scrape.
				// These may be leftovers from earlier tests.
				// Reference: https://issues.redhat.com/browse/OCPBUGS-61193
				if scrapeErr != nil && !namespacesToSkip.Has(targetNs) {
					targets, err := promTargets()
					if err != nil {
						e2e.Logf("refreshing state of target %s of pod %s/%s/%s failed, err: %v (skip=%t)", targetScrapeURL, targetNs, targetJob, targetPod, err, namespacesToSkip.Has(targetNs))
						targets = initialPromTargets
					}
					idx := slices.IndexFunc(targets.Data.ActiveTargets, func(t prometheusTarget) bool {
						return t.Labels["namespace"] == targetNs &&
							t.Labels["job"] == targetJob &&
							t.Labels["pod"] == targetPod &&
							t.ScrapeUrl == targetScrapeURL
					})
					if idx >= 0 && targets.Data.ActiveTargets[idx].Health == "up" {
						errChan <- fmt.Errorf("failed to ensure scraping target %s of pod %s/%s/%s requires auth: %w", targetScrapeURL, targetNs, targetJob, targetPod, scrapeErr)
					}
				}
				return nil
			})
		}
		err = eg.Wait()
		o.Expect(err).NotTo(o.HaveOccurred())
		close(errChan)

		var errs []error
		for err := range errChan {
			errs = append(errs, err)
		}
		o.Expect(errs).To(o.BeEmpty())
	})

})

var _ = g.Describe("[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io]", func() {
	defer g.GinkgoRecover()

	criticalAlertsMissingRunbookURLExceptions := sets.NewString(
		// Repository: https://github.com/openshift/cluster-ingress-operator
		// Issue: https://issues.redhat.com/browse/OCPBUGS-14057
		"HAProxyDown",

		// Repository: https://github.com/openshift/cluster-version-operator
		// Issue: https://issues.redhat.com/browse/OCPBUGS-14246
		"ClusterOperatorDown",
		"ClusterVersionOperatorDown",

		// Repository: https://github.com/openshift/managed-cluster-config
		// Issue: https://issues.redhat.com/browse/OSD-21709
		"AlertmanagerClusterCrashlooping",
		"AlertmanagerClusterDown",
		"AlertmanagerClusterFailedToSendAlerts",
		"AlertmanagerConfigInconsistent",
		"AlertmanagerFailedReload",
		"AlertmanagerMembersInconsistent",
		"CannotRetrieveUpdatesSRE",
		"CloudIngressOperatorOfflineSRE",
		"ClusterMonitoringErrorBudgetBurnSRE",
		"ConfigureAlertmanagerOperatorOfflineSRE",
		"ControlPlaneNodeFileDescriptorLimitSRE",
		"ControlPlaneNodeFilesystemAlmostOutOfFiles",
		"ControlPlaneNodeFilesystemSpaceFillingUp",
		"ControlPlaneNodeUnschedulableSRE",
		"ControlPlaneNodesNeedResizingSRE",
		"CustomerWorkloadPreventingDrainSRE",
		"EbsVolumeBurstBalanceLT20PctSRE",
		"EbsVolumeStuckAttaching10MinSRE",
		"EbsVolumeStuckDetaching10MinSRE",
		"ExcessiveContainerMemoryCriticalSRE",
		"HAProxyDownSRE",
		"InfraNodesNeedResizingSRE",
		"InsightsOperatorDownSRE",
		"KubeControllerManagerCrashloopingSRE",
		"KubeControllerManagerMissingOnNode60Minutes",
		"KubePersistentVolumeUsageCriticalCustomer",
		"KubePersistentVolumeUsageCriticalLayeredProduct",
		"MachineHealthCheckUnterminatedShortCircuitSRE",
		"MetricsClientSendFailingSRE",
		"MultipleVersionsOfEFSCSIDriverInstalled",
		"OCMAgentResponseFailureServiceLogsSRE",
		"ObservabilityOperatorBacklogNotDrained",
		"PodDisruptionBudgetLimitSRE",
		"PrometheusBadConfig",
		"PrometheusErrorSendingAlertsToAnyAlertmanager",
		"PrometheusRemoteStorageFailures",
		"PrometheusRemoteWriteBehind",
		"PrometheusRuleFailures",
		"PrometheusTargetSyncFailure",
		"PruningCronjobErrorSRE",
		"RouterAvailabilityLT30PctSRE",
		"RunawaySDNPreventingContainerCreationSRE",
		"SLAUptimeSRE",
		"UpgradeConfigSyncFailureOver4HrSRE",
		"UpgradeConfigValidationFailedSRE",
		"UpgradeControlPlaneUpgradeTimeoutSRE",
		"UpgradeNodeDrainFailedSRE",
		"UpgradeNodeUpgradeTimeoutSRE",
		"UserWorkloadMonitoringErrorBudgetBurn",
		"WorkerNodeFileDescriptorLimitSRE",
		"WorkerNodeFilesystemAlmostOutOfFiles",
		"WorkerNodeFilesystemSpaceFillingUp",
		"api-ErrorBudgetBurn",
		"console-ErrorBudgetBurn",
	)

	alertsMissingValidSeverityLevel := sets.NewString(
		// Repository: https://github.com/openshift/managed-cluster-config
		// Issue: https://issues.redhat.com/browse/OSD-21709
		"AdditionalTrustBundleCAExpiredNotificationSRE",
		"AdditionalTrustBundleCAExpiringNotificationSRE",
		"AdditionalTrustBundleCAInvalidNotificationSRE",
		"ClusterProxyNetworkDegradedNotificationSRE",
		"ElasticsearchClusterNotHealthyNotificationSRE",
		"ElasticsearchDiskSpaceRunningLowNotificationSRE",
		"ElasticsearchNodeDiskWatermarkReachedNotificationSRE",
		"KubeNodeUnschedulableSRE",
		"KubePersistentVolumeFillingUpSRE",
		"LoggingVolumeFillingUpNotificationSRE",
		"MultipleDefaultStorageClassesNotificationSRE",
		"NonSystemChangeValidatingWebhookConfigurationsNotificationSRE",
	)

	alertsMissingValidSummaryOrDescription := sets.NewString(
		// Repository: https://github.com/openshift/managed-cluster-config
		// Issue: https://issues.redhat.com/browse/OSD-21709
		"AdditionalTrustBundleCAExpiredNotificationSRE",
		"AdditionalTrustBundleCAExpiringNotificationSRE",
		"AdditionalTrustBundleCAInvalidNotificationSRE",
		"AlertmanagerSilencesActiveSRE",
		"APISchemeStatusFailing",
		"APISchemeStatusUnavailable",
		"CSRPendingLongDurationSRE",
		"ClusterMonitoringErrorBudgetBurnSRE",
		"ClusterProxyNetworkDegradedNotificationSRE",
		"ConfigureAlertmanagerOperatorOfflineSRE",
		"ControlPlaneLeaderElectionFailingSRE",
		"ControlPlaneNodeFileDescriptorLimitSRE",
		"ControlPlaneNodeFilesystemAlmostOutOfFiles",
		"ControlPlaneNodeFilesystemSpaceFillingUp",
		"ControlPlaneNodeUnschedulableSRE",
		"ControlPlaneNodesNeedResizingSRE",
		"CustomerWorkloadPreventingDrainSRE",
		"EbsVolumeStuckAttaching10MinSRE",
		"EbsVolumeStuckAttaching5MinSRE",
		"EbsVolumeStuckDetaching10MinSRE",
		"EbsVolumeStuckDetaching5MinSRE",
		"ElasticsearchClusterNotHealthyNotificationSRE",
		"ElasticsearchDiskSpaceRunningLowNotificationSRE",
		"ElasticsearchJobFailedSRE",
		"ElasticsearchNodeDiskWatermarkReachedNotificationSRE",
		"ElevatingClusterAdminRHMISRE",
		"ElevatingClusterAdminRHOAMSRE",
		"ExcessiveContainerMemoryCriticalSRE",
		"ExcessiveContainerMemoryWarningSRE",
		"HAProxyReloadFailSRE",
		"InfraNodesNeedResizingSRE",
		"KubeAPIServerMissingOnNode60Minutes",
		"KubeControllerManagerCrashloopingSRE",
		"KubeControllerManagerMissingOnNode60Minutes",
		"KubeNodeStuckWithCreatingAndTerminatingPodsSRE",
		"KubeNodeUnschedulableSRE",
		"KubePersistentVolumeFillingUpSRE",
		"KubePersistentVolumeFullInFourDaysCustomer",
		"KubePersistentVolumeFullInFourDaysLayeredProduct",
		"KubePersistentVolumeUsageCriticalCustomer",
		"KubePersistentVolumeUsageCriticalLayeredProduct",
		"KubeQuotaExceededSRE",
		"KubeSchedulerMissingOnNode60Minutes",
		"LoggingVolumeFillingUpNotificationSRE",
		"MNMOTooManyReconcileErrors15MinSRE",
		"MetricsClientSendFailingSRE",
		"MultipleDefaultStorageClassesNotificationSRE",
		"MultipleVersionsOfEFSCSIDriverInstalled",
		"NodeConditionDiskPressureNotificationSRE",
		"NodeConditionMemoryPressureNotificationSRE",
		"NodeConditionNetworkUnavailableNotificationSRE",
		"NodeConditionPIDPressureNotificationSRE",
		"NonSystemChangeValidatingWebhookConfigurationsNotificationSRE",
		"OCMAgentOperatorPullSecretInvalidSRE",
		"OCMAgentPullSecretInvalidSRE",
		"OCMAgentResponseFailureServiceLogsSRE",
		"OCMAgentServiceLogsSentExceedingLimit",
		"PodDisruptionBudgetLimitSRE",
		"PruningCronjobErrorSRE",
		"RouterAvailabilityLT30PctSRE",
		"RouterAvailabilityLT50PctSRE",
		"RunawaySDNPreventingContainerCreationSRE",
		"SLAUptimeSRE",
		"SplunkForwarderComponentUnhealthy",
		"UserWorkloadMonitoringErrorBudgetBurn",
		"VeleroDailyFullBackupMissed",
		"VeleroHourlyObjectBackupsMissedConsecutively",
		"VeleroWeeklyFullBackupMissed",
		"WorkerNodeFileDescriptorLimitSRE",
		"WorkerNodeFilesystemAlmostOutOfFiles",
		"WorkerNodeFilesystemSpaceFillingUp",
		"api-ErrorBudgetBurn",
		"console-ErrorBudgetBurn",
		"cpu-InfraNodesExcessiveResourceConsumptionSRE",
		"cpu-InfraNodesExcessiveResourceConsumptionSRE1h",
		"memory-InfraNodesExcessiveResourceConsumptionSRE",
	)

	var alertingRules map[string][]promv1.AlertingRule
	oc := exutil.NewCLIWithoutNamespace("prometheus")

	g.BeforeEach(func(ctx context.Context) {
		err := exutil.WaitForAnImageStream(
			oc.AdminImageClient().ImageV1().ImageStreams("openshift"), "tools",
			exutil.CheckImageStreamLatestTagPopulated, exutil.CheckImageStreamTagNotFound)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = helper.PrometheusServiceURL(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Verify prometheus service exists")

		if alertingRules == nil {
			url, err := helper.PrometheusRouteURL(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Get public url of prometheus")
			token, err := helper.RequestPrometheusServiceAccountAPIToken(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Request prometheus service account API token")
			alertingRules, err = helper.FetchAlertingRules(url, token)
			o.Expect(err).NotTo(o.HaveOccurred(), "Fetching alerting rules")
		}
	})

	g.It("should have a valid severity label", func() {
		err := helper.ForEachAlertingRule(alertingRules, func(alert promv1.AlertingRule) sets.String {
			if alertsMissingValidSeverityLevel.Has(alert.Name) {
				framework.Logf("Alerting rule %q is known to have invalid severity", alert.Name)
				return nil
			}
			severityRe := regexp.MustCompile("^critical|warning|info$")

			severity, found := alert.Labels["severity"]
			if !found {
				return sets.NewString("has no 'severity' label")
			}

			if !severityRe.MatchString(string(severity)) {
				return sets.NewString(
					fmt.Sprintf("has a 'severity' label value of %q which doesn't match %q",
						severity, severityRe.String(),
					),
				)
			}

			return nil
		})
		if err != nil {
			e2e.Fail(err.Error())
		}
	})

	g.It("should have description and summary annotations", func() {
		err := helper.ForEachAlertingRule(alertingRules, func(alert promv1.AlertingRule) sets.String {
			if alertsMissingValidSummaryOrDescription.Has(alert.Name) {
				framework.Logf("Alerting rule %q is known to have invalid summary or description", alert.Name)
				return nil
			}
			violations := sets.NewString()

			if _, found := alert.Annotations["description"]; !found {
				// If there's no 'description' annotation, but there is a
				// 'message' annotation, suggest renaming it.
				if _, found := alert.Annotations["message"]; found {
					violations.Insert("has no 'description' annotation, but has a 'message' annotation." +
						" OpenShift alerts must use 'description' -- consider renaming the annotation")
				} else {
					violations.Insert("has no 'description' annotation")
				}
			}

			if _, found := alert.Annotations["summary"]; !found {
				violations.Insert("has no 'summary' annotation")
			}

			return violations
		})
		if err != nil {
			e2e.Fail(err.Error())
		}
	})

	g.It("should have a runbook_url annotation if the alert is critical", func() {
		err := helper.ForEachAlertingRule(alertingRules, func(alert promv1.AlertingRule) sets.String {
			if criticalAlertsMissingRunbookURLExceptions.Has(alert.Name) {
				framework.Logf("Critical alerting rule %q is known to have missing runbook_url.", alert.Name)
				return nil
			}
			violations := sets.NewString()
			severity := string(alert.Labels["severity"])
			runbook := string(alert.Annotations["runbook_url"])

			if severity == "critical" && runbook == "" {
				violations.Insert(
					fmt.Sprintf("WARNING: Alert %q is critical and has no 'runbook_url' annotation", alert.Name),
				)
			}

			return violations
		})
		if err != nil {
			e2e.Fail(err.Error())
		}
	})

	g.It("should link to an HTTP(S) location if the runbook_url annotation is defined", func() {
		err := helper.ForEachAlertingRule(alertingRules, func(alert promv1.AlertingRule) sets.String {
			violations := sets.NewString()
			runbook_url := string(alert.Annotations["runbook_url"])

			if runbook_url != "" {
				// If there's a 'runbook_url' annotation, make sure that it is a valid URL
				if err := helper.ValidateURL(runbook_url); err != nil {
					violations.Insert(
						fmt.Sprintf("has an 'runbook_url' annotation which is not valid: %v", err),
					)
				}
			}

			return violations
		})
		if err != nil {
			e2e.Fail(err.Error())
		}
	})

	g.It("should link to a valid URL if the runbook_url annotation is defined", func() {
		err := helper.ForEachAlertingRule(alertingRules, func(alert promv1.AlertingRule) sets.String {
			violations := sets.NewString()
			runbook_url := string(alert.Annotations["runbook_url"])

			if runbook_url == "" {
				return nil
			}
			// If there's a 'runbook_url' annotation, make sure that we can fetch the contents.
			if err := helper.QueryURL(runbook_url, 10*time.Second); err != nil {
				violations.Insert(
					fmt.Sprintf("has a runbook URL which cannot be fetched: %v", err),
				)
			}

			return violations
		})
		if err != nil {
			// We can't fail the test because the runbook URLs might be temporarily unavailable.
			// At least we can manually check the CI logs to identify buggy URLs.
			testresult.Flakef("%s", err.Error())
		}
	})
})

var _ = g.Describe("[sig-instrumentation][Late] Alerts", func() {
	defer g.GinkgoRecover()
	ctx := context.TODO()
	oc := exutil.NewCLIWithoutNamespace("prometheus")

	g.BeforeEach(func() {
		kubeClient, err := kubernetes.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		nsExist, err := exutil.IsNamespaceExist(kubeClient, "openshift-monitoring")
		o.Expect(err).NotTo(o.HaveOccurred())
		if !nsExist {
			g.Skip("openshift-monitoring namespace does not exist, skipping")
		}
	})

	g.It("shouldn't exceed the series limit of total series sent via telemetry from each cluster", func() {
		if enabledErr, err := telemetryIsEnabled(ctx, oc.AdminKubeClient()); err != nil {
			e2e.Failf("could not determine if Telemetry is enabled: %v", err)
		} else if enabledErr != nil {
			e2eskipper.Skipf("Telemetry is disabled: %v", enabledErr)
		}

		// we only consider series sent since the beginning of the test
		testDuration := exutil.DurationSinceStartInSeconds().String()

		tests := map[string]bool{
			// We want to limit the number of total series sent, the cluster:telemetry_selected_series:count
			// rule contains the count of the all the series that are sent via telemetry. It is permissible
			// for some scenarios to generate more series than the limit, we just want the basic state to be below
			// a threshold.

			// The following query can be executed against the telemetry server
			// to reevaluate the threshold value (replace the matcher on the version label accordingly):
			//
			// quantile(0.99,
			//   avg_over_time(
			//     (
			//       cluster:telemetry_selected_series:count
			//       *
			//       on (_id) group_left group by(_id) (cluster_version{version=~"4.11.0-0.ci.+"})
			//     )[30m:1m]
			//   )
			// )
			fmt.Sprintf(`avg_over_time(cluster:telemetry_selected_series:count[%s]) >= 1000`, testDuration): false,
			fmt.Sprintf(`max_over_time(cluster:telemetry_selected_series:count[%s]) >= 1200`, testDuration): false,
		}
		err := helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Total number of series sent via telemetry is below the limit")
	})
})

var _ = g.Describe("[sig-instrumentation] Prometheus [apigroup:image.openshift.io]", func() {
	defer g.GinkgoRecover()
	ctx := context.TODO()
	var (
		oc                                                                  = exutil.NewCLIWithPodSecurityLevel("prometheus", admissionapi.LevelBaseline)
		queryURL, prometheusURL, querySvcURL, prometheusSvcURL, bearerToken string
	)

	g.BeforeEach(func(ctx g.SpecContext) {
		err := exutil.WaitForAnImageStream(
			oc.AdminImageClient().ImageV1().ImageStreams("openshift"), "tools",
			exutil.CheckImageStreamLatestTagPopulated, exutil.CheckImageStreamTagNotFound)
		o.Expect(err).NotTo(o.HaveOccurred())

		queryURL, err = helper.ThanosQuerierRouteURL(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Get public url of thanos querier")
		prometheusURL, err = helper.PrometheusRouteURL(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Get public url of prometheus")
		querySvcURL, err = helper.ThanosQuerierServiceURL(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Get url of thanos querier service")
		prometheusSvcURL, err = helper.PrometheusServiceURL(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Get url of prometheus service")
		bearerToken, err = helper.RequestPrometheusServiceAccountAPIToken(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Request prometheus service account API token")
	})

	g.Describe("when installed on the cluster", func() {
		g.It("should report telemetry [Serial] [Late]", func() {
			if enabledErr, err := telemetryIsEnabled(ctx, oc.AdminKubeClient()); err != nil {
				e2e.Failf("could not determine if Telemetry is enabled: %v", err)
			} else if enabledErr != nil {
				e2eskipper.Skipf("Telemetry is disabled: %v", enabledErr)
			}

			tests := map[string]bool{}
			if hasTelemeterClient(oc.AdminKubeClient()) {
				e2e.Logf("Found telemeter-client pod")
				tests = map[string]bool{
					// should have successfully sent at least once to remote
					`metricsclient_request_send{client="federate_to",job="telemeter-client",status_code="200"} >= 1`: true,
					// should have scraped some metrics from prometheus
					`federate_samples{job="telemeter-client"} >= 10`: true,
				}
			} else {
				e2e.Logf("Found no telemeter-client pod, assuming prometheus remote_write")
				tests = map[string]bool{
					// Should have successfully sent at least some metrics to
					// remote write endpoint
					`prometheus_remote_storage_succeeded_samples_total{job="prometheus-k8s"} >= 1`: true,
				}
			}
			err := helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			e2e.Logf("Telemetry is enabled")

			if err != nil {
				// Making the test flaky until monitoring team fixes the rate limit issue.
				testresult.Flakef("%s", err.Error())
			}
		})

		g.It("should start and expose a secured proxy and unsecured metrics [apigroup:config.openshift.io]", func() {
			ns := oc.Namespace()
			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			g.By("checking the prometheus metrics path")
			var metrics map[string]*dto.MetricFamily
			o.Expect(wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
				results, err := helper.GetBearerTokenURLViaPod(oc, execPod.Name, fmt.Sprintf("%s/metrics", prometheusSvcURL), bearerToken)
				if err != nil {
					e2e.Logf("unable to get metrics: %v", err)
					return false, nil
				}

				p := expfmt.TextParser{}
				metrics, err = p.TextToMetricFamilies(bytes.NewBufferString(results))
				o.Expect(err).NotTo(o.HaveOccurred())
				// original field in 2.0.0-beta
				counts := findCountersWithLabels(metrics["tsdb_samples_appended_total"], labels{})
				if len(counts) != 0 && counts[0] > 0 {
					return true, nil
				}
				// 2.0.0-rc.0
				counts = findCountersWithLabels(metrics["tsdb_head_samples_appended_total"], labels{})
				if len(counts) != 0 && counts[0] > 0 {
					return true, nil
				}
				// 2.0.0-rc.2
				counts = findCountersWithLabels(metrics["prometheus_tsdb_head_samples_appended_total"], labels{})
				if len(counts) != 0 && counts[0] > 0 {
					return true, nil
				}
				return false, nil
			})).NotTo(o.HaveOccurred(), fmt.Sprintf("Did not find tsdb_samples_appended_total, tsdb_head_samples_appended_total, or prometheus_tsdb_head_samples_appended_total"))

			g.By("verifying the Thanos querier service requires authentication")
			err := helper.ExpectURLStatusCodeExecViaPod(ns, execPod.Name, querySvcURL, 401, 403)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to authenticate")
			err = helper.ExpectHTTPStatusCode(helper.MustJoinUrlPath(queryURL, "api/v1/targets"), bearerToken, 200)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying a service account token is able to access the Prometheus API")
			// expect all endpoints within 60 seconds
			var lastErrs []error
			o.Expect(wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
				contents, err := helper.GetURLWithToken(helper.MustJoinUrlPath(prometheusURL, "api/v1/targets"), bearerToken)
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := &prometheusTargets{}
				err = json.Unmarshal([]byte(contents), targets)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying all expected jobs have a working target")

				controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				// For External clusters, skip control plane components and the CVO
				if *controlPlaneTopology != configv1.ExternalTopologyMode {
					lastErrs = all(
						// The OpenShift control plane
						targets.Expect(labels{"job": "api"}, "up", "^https://.*/metrics$"),
						targets.Expect(labels{"job": "controller-manager"}, "up", "^https://.*/metrics$"),

						// The kube control plane
						targets.Expect(labels{"job": "etcd"}, "up", "^https://.*/metrics$"),
						targets.Expect(labels{"job": "apiserver"}, "up", "^https://.*/metrics$"),
						targets.Expect(labels{"job": "kube-controller-manager"}, "up", "^https://.*/metrics$"),
						targets.Expect(labels{"job": "scheduler"}, "up", "^https://.*/metrics$"),
						targets.Expect(labels{"job": "kube-state-metrics"}, "up", "^https://.*/metrics$"),

						// Cluster version operator
						targets.Expect(labels{"job": "cluster-version-operator"}, "up", "^https://.*/metrics$"),
					)
				}

				lastErrs = append(lastErrs, all(
					targets.Expect(labels{"job": "prometheus-k8s", "namespace": "openshift-monitoring", "pod": "prometheus-k8s-0"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kubelet"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "kubelet"}, "up", "^https://.*/metrics/cadvisor$"),
					targets.Expect(labels{"job": "node-exporter"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "prometheus-operator"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "alertmanager-main"}, "up", "^https://.*/metrics$"),
					targets.Expect(labels{"job": "crio"}, "up", "^http(s)?://.*/metrics$"),
				)...)
				if len(lastErrs) > 0 {
					e2e.Logf("missing some targets: %v", lastErrs)
					return false, nil
				}
				return true, nil
			})).NotTo(o.HaveOccurred(), "possibly some services didn't register ServiceMonitors to allow metrics collection")

			g.By("verifying all targets are exposing metrics over secure channel")
			var insecureTargets []error
			contents, err := helper.GetURLWithToken(helper.MustJoinUrlPath(prometheusURL, "api/v1/targets"), bearerToken)
			o.Expect(err).NotTo(o.HaveOccurred())

			targets := &prometheusTargets{}
			err = json.Unmarshal([]byte(contents), targets)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Currently following targets do not secure their /metrics endpoints:
			// job="crio" - https://issues.redhat.com/browse/MON-1034 + https://issues.redhat.com/browse/OCPNODE-321
			// job="ovnkube-master" - https://issues.redhat.com/browse/SDN-912
			// job="ovnkube-node" - https://issues.redhat.com/browse/SDN-912
			// Exclude list should be reduced to 0
			exclude := map[string]bool{
				"crio":           true,
				"ovnkube-master": true,
				"ovnkube-node":   true,

				// Managed services OSD-26070
				"managed-node-metadata-operator-metrics-service": true,
				"managed-upgrade-operator-custom-metrics":        true,
				"managed-upgrade-operator-metrics":               true,
				"configure-alertmanager-operator":                true,
				"must-gather-operator":                           true,
				"ocm-agent-metrics":                              true,
				"ocm-agent-operator":                             true,
				"osd-metrics-exporter":                           true,
				"package-operator-metrics":                       true,
				"rbac-permissions-operator":                      true,
				"blackbox-exporter":                              true,
				"splunk-forwarder":                               true,
				"validation-webhook-metrics":                     true,
				"cloud-ingress-operator":                         true,
				"managed-velero-operator-metrics":                true,
				"velero-metrics":                                 true,
			}

			pattern := regexp.MustCompile("^https://.*")
			for _, t := range targets.Data.ActiveTargets {
				if exclude[t.Labels["job"]] {
					continue
				}
				if !pattern.MatchString(t.ScrapeUrl) {
					msg := fmt.Errorf("following target does not secure metrics endpoint: %v", t.Labels["job"])
					insecureTargets = append(insecureTargets, msg)
				}
			}
			o.Expect(insecureTargets).To(o.BeEmpty(), "some services expose metrics over insecure channel")
		})

		g.It("should have a AlertmanagerReceiversNotConfigured alert in firing state", func() {
			tests := map[string]bool{
				`ALERTS{alertstate=~"firing|pending",alertname="AlertmanagerReceiversNotConfigured"} == 1`: true,
			}
			err := helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			e2e.Logf("AlertmanagerReceiversNotConfigured alert is firing")
		})

		g.It("should have important platform topology metrics [apigroup:config.openshift.io]", func() {
			exutil.SkipIfExternalControlplaneTopology(oc, "topology metrics are not available for clusters with external controlPlaneTopology")

			tests := map[string]bool{
				// track infrastructure type
				`cluster_infrastructure_provider{type!=""}`: true,
				`cluster_feature_set`:                       true,

				// track installer type
				`cluster_installer{type!="",invoker!=""}`: true,

				// track sum of etcd
				`instance:etcd_object_counts:sum > 0`: true,

				// track cores and sockets across node types
				`sum(node_role_os_version_machine:cpu_capacity_cores:sum{label_kubernetes_io_arch!="",label_node_role_kubernetes_io_master!=""}) > 0`:                                      true,
				`sum(node_role_os_version_machine:cpu_capacity_sockets:sum{label_kubernetes_io_arch!="",label_node_hyperthread_enabled!="",label_node_role_kubernetes_io_master!=""}) > 0`: true,
			}
			err := helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should have non-Pod host cAdvisor metrics", func() {
			tests := map[string]bool{
				`container_cpu_usage_seconds_total{id!~"/kubepods.slice/.*"} >= 1`: true,
			}
			err := helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("shouldn't have failing rules evaluation", func() {
			// we only consider samples since the beginning of the test
			testDuration := exutil.DurationSinceStartInSeconds().String()

			tests := map[string]bool{
				fmt.Sprintf(`increase(prometheus_rule_evaluation_failures_total[%s]) >= 1`, testDuration): false,
			}
			err := helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("shouldn't report any alerts in firing state apart from Watchdog and AlertmanagerReceiversNotConfigured [Early][apigroup:config.openshift.io]", func() {
			// Copy so we can expand:
			allowedAlertNames := make([]string, len(allowedalerts.AllowedAlertNames))
			copy(allowedAlertNames, allowedalerts.AllowedAlertNames)

			// Checking Watchdog alert state is done in "should have a Watchdog alert in firing state".
			// we exclude alerts that have their own separate tests.
			for _, alertTest := range allowedalerts.AllAlertTests(&platformidentification.JobType{}, nil, allowedalerts.DefaultAllowances) {
				allowedAlertNames = append(allowedAlertNames, alertTest.AlertName())
			}

			if exutil.IsNoUpgradeFeatureSet(oc) {
				// On a TechPreviewNoUpgrade or CustomNoUpgrade cluster we must ignore the TechPreviewNoUpgrade and ClusterNotUpgradeable alerts generated by the CVO.
				// These two alerts are expected in this case when a cluster is configured to enable Tech Preview features,
				// as they were intended to be "gentle reminders" to the cluster admins of the ramifications of enabling Tech Preview
				allowedAlertNames = append(allowedAlertNames, "TechPreviewNoUpgrade", "ClusterNotUpgradeable")
			}

			// OSD-26887: managed services taints several nodes as infrastructure.  This taint appears to be applied
			// after some of the platform DS are scheduled there, causing this alert to fire.  Managed services
			// rebalances the DS after the taint is added, and the alert clears, but origin fails this test. Allowing
			// this alert to fire while we investigate why the taint is not added at node birth.
			isManagedService, err := exutil.IsManagedServiceCluster(ctx, oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isManagedService {
				allowedAlertNames = append(allowedAlertNames, "KubeDaemonSetMisScheduled")
			}
			// https://issues.redhat.com/browse/OCPBUGS-48340
			if SkipOperatorHubMetricsCheck(oc) {
				allowedAlertNames = append(allowedAlertNames, "OperatorHubSourceError")
			}

			tests := map[string]bool{
				// openshift-e2e-loki alerts should never fail this test, we've seen this happen on daemon set rollout stuck when CI loki was down.
				fmt.Sprintf(`ALERTS{alertname!~"%s",alertstate="firing",severity!="info",namespace!="openshift-e2e-loki"} >= 1`, strings.Join(allowedAlertNames, "|")): false,
			}
			err = helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), tests, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should provide ingress metrics", func() {
			var lastErrs []error
			o.Expect(wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 4*time.Minute, true, func(ctx context.Context) (bool, error) {
				contents, err := helper.GetURLWithToken(helper.MustJoinUrlPath(prometheusURL, "api/v1/targets"), bearerToken)
				o.Expect(err).NotTo(o.HaveOccurred())

				targets := &prometheusTargets{}
				err = json.Unmarshal([]byte(contents), targets)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying all expected jobs have a working target")
				lastErrs = all(
					// Is there a good way to discover the name and thereby avoid leaking the naming algorithm?
					targets.Expect(labels{"job": "router-internal-default"}, "up", "^https://.*/metrics$"),
				)
				if len(lastErrs) > 0 {
					e2e.Logf("missing some targets: %v", lastErrs)
					return false, nil
				}
				return true, nil
			})).NotTo(o.HaveOccurred(), "ingress router cannot report metrics to monitoring system")

			g.By("verifying standard metrics keys")
			queries := map[string]bool{
				`template_router_reload_seconds_count{job="router-internal-default"} >= 1`: true,
				`haproxy_server_up{job="router-internal-default"} >= 1`:                    true,
			}
			err := helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), queries, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should provide named network metrics [apigroup:project.openshift.io]", func() {
			ns := oc.SetupProject()

			cs, err := newDynClientSet()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = addNetwork(cs, "secondary", ns)
			o.Expect(err).NotTo(o.HaveOccurred())

			defer func() {
				err := removeNetwork(cs, "secondary", ns)
				o.Expect(err).NotTo(o.HaveOccurred())
			}()

			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) {
				pod.Annotations = map[string]string{
					"k8s.v1.cni.cncf.io/networks": "secondary",
				}
			})

			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			g.By("verifying named metrics keys")
			queries := map[string]bool{
				fmt.Sprintf(`pod_network_name_info{pod="%s",namespace="%s",interface="eth0"} == 0`, execPod.Name, execPod.Namespace):                true,
				fmt.Sprintf(`pod_network_name_info{pod="%s",namespace="%s",network_name="%s/secondary"} == 0`, execPod.Name, execPod.Namespace, ns): true,
			}
			err = helper.RunQueries(context.TODO(), oc.NewPrometheusClient(context.TODO()), queries, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func all(errs ...error) []error {
	var result []error
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}

type prometheusTarget struct {
	Labels    map[string]string
	Health    string
	ScrapeUrl string
}

type prometheusTargets struct {
	Data struct {
		ActiveTargets []prometheusTarget
	}
	Status string
}

func (t *prometheusTargets) Expect(l labels, health, scrapeURLPattern string) error {
	for _, target := range t.Data.ActiveTargets {
		match := true
		for k, v := range l {
			if target.Labels[k] != v {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if health != target.Health {
			continue
		}
		if !regexp.MustCompile(scrapeURLPattern).MatchString(target.ScrapeUrl) {
			continue
		}
		return nil
	}
	return fmt.Errorf("no match for %v with health %s and scrape URL %s", l, health, scrapeURLPattern)
}

type labels map[string]string

func (l labels) With(name, value string) labels {
	n := make(labels)
	for k, v := range l {
		n[k] = v
	}
	n[name] = value
	return n
}

func findEnvVar(vars []kapi.EnvVar, key string) string {
	for _, v := range vars {
		if v.Name == key {
			return v.Value
		}
	}
	return ""
}

func findMetricsWithLabels(f *dto.MetricFamily, labels map[string]string) []*dto.Metric {
	var result []*dto.Metric
	if f == nil {
		return result
	}
	for _, m := range f.Metric {
		matched := map[string]struct{}{}
		for _, l := range m.Label {
			if expect, ok := labels[l.GetName()]; ok {
				if expect != l.GetValue() {
					break
				}
				matched[l.GetName()] = struct{}{}
			}
		}
		if len(matched) != len(labels) {
			continue
		}
		result = append(result, m)
	}
	return result
}

func findCountersWithLabels(f *dto.MetricFamily, labels map[string]string) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, labels) {
		result = append(result, m.Counter.GetValue())
	}
	return result
}

func findGaugesWithLabels(f *dto.MetricFamily, labels map[string]string) []float64 {
	var result []float64
	for _, m := range findMetricsWithLabels(f, labels) {
		result = append(result, m.Gauge.GetValue())
	}
	return result
}

func findMetricLabels(f *dto.MetricFamily, labels map[string]string, match string) []string {
	var result []string
	for _, m := range findMetricsWithLabels(f, labels) {
		for _, l := range m.Label {
			if l.GetName() == match {
				result = append(result, l.GetValue())
				break
			}
		}
	}
	return result
}

// telemetryIsEnabled returns (nil, nil) if Telemetry is enabled,
// (error, nil) if Telemetry is not enabled, and (_, error) if it fails
// to determine whether or not Telemetry is enabled.
func telemetryIsEnabled(ctx context.Context, client clientset.Interface) (enabled error, err error) {
	domain := "cloud.openshift.com"
	if hasSecret, err := hasPullSecret(ctx, client, domain); err != nil || hasSecret != nil {
		return hasSecret, err
	}

	return isTelemeterClientEnabled(ctx, client)
}

func hasPullSecret(ctx context.Context, client clientset.Interface, name string) (enabled error, err error) {
	scrt, err := client.CoreV1().Secrets("openshift-config").Get(ctx, "pull-secret", metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			return fmt.Errorf("openshift-config/pull-secret not found"), nil
		}
		return nil, fmt.Errorf("could not retrieve pull-secret: %w", err)
	}

	if scrt.Type != v1.SecretTypeDockerConfigJson {
		return nil, fmt.Errorf("error expecting openshift-config/pull-secret type %s got %s", v1.SecretTypeDockerConfigJson, scrt.Type)
	}

	ps := struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}{}

	if err := json.Unmarshal(scrt.Data[v1.DockerConfigJsonKey], &ps); err != nil {
		return nil, fmt.Errorf("could not unmarshal pullSecret from openshift-config/pull-secret: %w", err)
	}

	if len(ps.Auths[name].Auth) == 0 {
		return fmt.Errorf("openshift-config/pull-secret does not contain auth for %s", name), nil
	}

	return nil, nil
}

func isTelemeterClientEnabled(ctx context.Context, client clientset.Interface) (enabled error, err error) {
	config, err := client.CoreV1().ConfigMaps("openshift-monitoring").Get(ctx, "cluster-monitoring-config", metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			return nil, nil // Telemetry is enabled by default
		}
		return nil, fmt.Errorf("could not retrieve monitoring configuration: %w", err)
	}
	var structuredConfig ClusterMonitoringConfiguration
	if yamlConfig, ok := config.Data["config.yaml"]; !ok {
		return nil, fmt.Errorf("openshift-monitoring/cluster-monitoring-config data lacks a config.yaml key: %v", config.Data)
	} else if err := yaml.Unmarshal([]byte(yamlConfig), &structuredConfig); err != nil {
		return nil, fmt.Errorf("error unmarshalling openshift-monitoring/cluster-monitoring-config config.yaml: %w", err)
	}
	if structuredConfig.TelemeterClientConfig == nil || structuredConfig.TelemeterClientConfig.Enabled == nil {
		return nil, nil // Telemetry is enabled by default
	}
	if !*structuredConfig.TelemeterClientConfig.Enabled {
		return fmt.Errorf("openshift-monitoring/cluster-monitoring-config telemeterClient enabled is: %t", *structuredConfig.TelemeterClientConfig.Enabled), nil
	}
	return nil, nil
}

func hasTelemeterClient(client clientset.Interface) bool {
	_, err := client.CoreV1().Pods("openshift-monitoring").List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=telemeter-client",
	})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			return false
		}
		e2e.Failf("could not list pods: %v", err)
	}
	return true
}

func SkipOperatorHubMetricsCheck(oc *exutil.CLI) bool {
	stdout, stderr, err := oc.AsAdmin().Run("get").Args("operatorhub", "cluster", "-o=jsonpath={.spec.disableAllDefaultSources}").Outputs()
	if err != nil {
		fmt.Printf("command failed: %v\nstderr: %s\nstdout:%s", err, stderr, stdout)
	}
	return stdout == "true"
}
