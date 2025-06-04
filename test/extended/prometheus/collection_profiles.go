package prometheus

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	prometheusoperatorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	prometheusoperatorv1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	projectName = "monitoring-collection-profiles"

	collectionProfileFeatureLabel = "monitoring.openshift.io/collection-profile"
	collectionProfileFull         = "full"
	collectionProfileDefault      = collectionProfileFull
	collectionProfileMinimal      = "minimal"
	collectionProfileNone         = ""

	operatorName              = "cluster-monitoring-operator"
	operatorNamespaceName     = "openshift-monitoring"
	operatorConfigurationName = "cluster-monitoring-config"
)

var (
	oc   = exutil.NewCLI(projectName)
	tctx = context.Background()

	collectionProfilesSupportedList = []string{
		collectionProfileFull,
		collectionProfileMinimal,
	}
)

type runner struct {
	kclient                       kubernetes.Interface
	mclient                       *prometheusoperatorv1client.MonitoringV1Client
	pclient                       prometheusv1.API
	originalOperatorConfiguration *v1.ConfigMap
}

// NOTE: The nested `Context` containers inside the following `Describe` container are used to group certain tests based on the environments they demand.
// NOTE: When adding a test-case, ensure that the test-case is placed in the appropriate `Context` container.
// NOTE: The containers themselves are guaranteed to run in the order in which they appear.
var _ = g.Describe("[sig-instrumentation][OCPFeatureGate:MetricsCollectionProfiles] The collection profiles feature-set", g.Ordered, func() {
	defer g.GinkgoRecover()

	o.SetDefaultEventuallyTimeout(15 * time.Minute)
	o.SetDefaultEventuallyPollingInterval(5 * time.Second)

	r := &runner{}

	g.BeforeAll(func() {
		if !exutil.IsTechPreviewNoUpgrade(tctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}
		var err error
		r.kclient, err = kubernetes.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("failed to create kubernetes client: %v", err))
		}
		r.mclient, err = prometheusoperatorv1client.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("failed to create monitoring client: %v", err))
		}
		r.pclient = oc.NewPrometheusClient(tctx)

		var operatorConfiguration *v1.ConfigMap
		o.Eventually(func() error {
			operatorConfiguration, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					g.By("initially, creating a configuration for the operator as it did not exist")
					err = r.makeCollectionProfileConfigurationFor(tctx, collectionProfileDefault)
				}
				if err != nil {
					return err
				}
			}

			return nil
		}).Should(o.BeNil())
		r.originalOperatorConfiguration = operatorConfiguration
	})

	g.AfterAll(func() {
		currentConfiguration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if r.originalOperatorConfiguration != nil {
			currentConfiguration.Data = r.originalOperatorConfiguration.Data
			g.By("restoring the original configuration for the operator")
			_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Update(tctx, currentConfiguration, metav1.UpdateOptions{})
		} else {
			g.By("cleaning up the configuration for the operator as it did not exist pre-job")
			err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Delete(tctx, operatorConfigurationName, metav1.DeleteOptions{})
		}
		o.Expect(err).To(o.BeNil())
	})

	g.Context("initially, in a homogeneous default environment,", func() {
		profile := collectionProfileDefault

		g.BeforeAll(func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, profile)
			o.Expect(err).To(o.BeNil())
			o.Eventually(func() error {
				enabled, err := r.isProfileEnabled(profile)
				if err != nil {
					return err
				}
				if !enabled {
					return fmt.Errorf("collection profile %q is not enabled", profile)
				}

				return nil
			}).Should(o.BeNil())
		})

		g.It("should expose default metrics", func() {
			o.Eventually(func() error {
				defaultOnlyMetric := "prometheus_engine_query_log_enabled"
				defaultMetricQuery := fmt.Sprintf("max(%s)", defaultOnlyMetric)
				queryResponse, err := helper.RunQuery(tctx, r.pclient, defaultMetricQuery)
				if err != nil {
					return err
				}
				if len(queryResponse.Data.Result) == 0 {
					return fmt.Errorf("expected %q to be present", defaultOnlyMetric)
				}

				return nil
			}).Should(o.BeNil())
		})
	})

	g.Context("in a heterogeneous environment,", func() {
		g.It("should expose information about the applied collection profile using meta-metrics", func() {
			for _, profile := range collectionProfilesSupportedList {
				err := r.makeCollectionProfileConfigurationFor(tctx, profile)
				o.Expect(err).To(o.BeNil())

				o.Eventually(func() error {
					vectorExpression := "max(profile:cluster_monitoring_operator_collection_profile:max{profile=\"%s\"}) == 1"
					queryResponse, err := helper.RunQuery(tctx, r.pclient, fmt.Sprintf(vectorExpression, profile))
					if err != nil {
						return err
					}
					if len(queryResponse.Data.Result) == 0 {
						return fmt.Errorf("no result found for profile %q", profile)
					}

					return nil
				}).Should(o.BeNil())
			}
		})
		g.It("should have at least one implementation for each collection profile", func() {
			for _, profile := range collectionProfilesSupportedList {
				err := r.makeCollectionProfileConfigurationFor(tctx, profile)
				o.Expect(err).To(o.BeNil())

				o.Eventually(func() error {
					monitors, err := r.fetchMonitorsFor([2]string{collectionProfileFeatureLabel, profile})
					if err != nil {
						return err
					}
					if len(monitors.Items) == 0 {
						return fmt.Errorf("no monitors found with collection profile %q", profile)
					}

					return nil
				}).Should(o.BeNil())
			}
		})
		g.It("should revert to default collection profile when an empty collection profile value is specified", func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, collectionProfileNone)
			o.Expect(err).To(o.BeNil())

			o.Eventually(func() error {
				enabled, err := r.isProfileEnabled(collectionProfileFull)
				if err != nil {
					return err
				}
				if !enabled {
					return fmt.Errorf("collection profile %q is not enabled", collectionProfileFull)
				}

				return nil
			}).Should(o.BeNil())
		})
	})

	g.Context("in a homogeneous minimal environment,", func() {
		profile := collectionProfileMinimal

		g.BeforeAll(func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, profile)
			o.Expect(err).To(o.BeNil())
			o.Eventually(func() error {
				enabled, err := r.isProfileEnabled(profile)
				if err != nil {
					return err
				}
				if !enabled {
					return fmt.Errorf("collection profile %q is not enabled", profile)
				}

				return nil
			}).Should(o.BeNil())
		})

		g.It("should hide default metrics", func() {
			appNameSelector := "app.kubernetes.io/name"
			appName := "kube-state-metrics"

			var kubeStateMetricsMonitor *prometheusoperatorv1.ServiceMonitor
			o.Eventually(func() error {
				monitors, err := r.fetchMonitorsFor([2]string{collectionProfileFeatureLabel, profile}, [2]string{appNameSelector, appName})
				if err != nil {
					return err
				}
				if len(monitors.Items) == 0 {
					return fmt.Errorf("no monitors found with collection profile: %q and %#v=%q", profile, appNameSelector, appName)
				}
				if len(monitors.Items) > 1 {
					return fmt.Errorf("more than one monitor found with collection profile: %q and %#v=%q", profile, appNameSelector, appName)
				}
				kubeStateMetricsMonitor = monitors.Items[0]

				return nil
			}).Should(o.BeNil())

			var kubeStateMetricsMainMetrics []string
			kubeStateMetricsMonitorSpec := kubeStateMetricsMonitor.Spec
			kubeStateMetricsMonitorSpecEndpoints := kubeStateMetricsMonitorSpec.Endpoints
			if len(kubeStateMetricsMonitorSpecEndpoints) != 0 {
				kubeStateMetricsMonitorSpecEndpoints0Relabelings := kubeStateMetricsMonitorSpecEndpoints[0].MetricRelabelConfigs
				if len(kubeStateMetricsMonitorSpecEndpoints0Relabelings) != 0 {
					for _, relabeling := range kubeStateMetricsMonitorSpecEndpoints0Relabelings {
						// NOTE: This should accommodate for future changes to the relabeling scope.
						if relabeling.Action == "keep" &&
							len(relabeling.SourceLabels) == 1 &&
							relabeling.SourceLabels[0] == "__name__" {
							regexpString := relabeling.Regex
							kubeRegex := regexp.MustCompile(`(?U)(kube_.*)[|,)]`)
							kubeMetrics := kubeRegex.FindAllString(regexpString, -1)
							for _, metric := range kubeMetrics {
								// Golang doesn't support negative lookaheads.
								if strings.HasPrefix(metric, "kube_state_metrics") {
									continue
								}
								kubeStateMetricsMainMetrics = append(kubeStateMetricsMainMetrics, metric)
							}
						}
					}
				}
			}
			o.Expect(len(kubeStateMetricsMainMetrics)).To(o.BeNumerically(">", 0))

			o.Eventually(func() error {
				postRelabelingMetric := "scrape_samples_post_metric_relabeling"
				relabelingMetricQuery := fmt.Sprintf("sum(%s{job=\"%s\",endpoint=\"https-main\",namespace=\"%s\"})", postRelabelingMetric, appName, operatorNamespaceName)
				queryResponse, err := helper.RunQuery(tctx, r.pclient, relabelingMetricQuery)
				if err != nil {
					return err
				}
				if len(queryResponse.Data.Result) == 0 {
					return fmt.Errorf("no result found for metric %q", postRelabelingMetric)
				}
				wantCount := int(queryResponse.Data.Result[0].Value)

				kubeStateMetricsMainMetricsString := strings.Join(kubeStateMetricsMainMetrics, "")
				kubeStateMetricsMainMetricsCountQuery := fmt.Sprintf("count({__name__=~\"%s\"})", kubeStateMetricsMainMetricsString[:len(kubeStateMetricsMainMetricsString)-1 /* drop the last "|" or ")" */])
				queryResponse, err = helper.RunQuery(tctx, r.pclient, kubeStateMetricsMainMetricsCountQuery)
				if err != nil {
					return err
				}
				if len(queryResponse.Data.Result) == 0 {
					return fmt.Errorf("no result found for metric %q", kubeStateMetricsMainMetricsCountQuery)
				}
				gotCount := int(queryResponse.Data.Result[0].Value)

				if gotCount != wantCount {
					return fmt.Errorf("got %v, want %v", gotCount, wantCount)
				}

				return nil
			}).Should(o.BeNil())
		})
	})
})

func (r runner) isProfileEnabled(profile string) (bool, error) {
	vectorExpression := "max(profile:cluster_monitoring_operator_collection_profile:max{profile=\"%s\"}) == 1"
	queryResponse, err := helper.RunQuery(tctx, r.pclient, fmt.Sprintf(vectorExpression, profile))
	if err != nil {
		return false, err
	}
	if len(queryResponse.Data.Result) == 0 {
		return false, nil
	}

	return true, nil
}

func (r runner) fetchMonitorsFor(selectors ...[2]string) (*prometheusoperatorv1.ServiceMonitorList, error) {
	managedMonitorsSelectors := []string{
		fmt.Sprintf("%s=%s", "app.kubernetes.io/managed-by", operatorName),
	}
	for _, selector := range selectors {
		managedMonitorsSelectors = append(managedMonitorsSelectors, fmt.Sprintf("%s=%s", selector[0], selector[1]))
	}
	return r.mclient.ServiceMonitors(operatorNamespaceName).List(tctx, metav1.ListOptions{
		LabelSelector: strings.Join(managedMonitorsSelectors, ","),
	})
}

func (r runner) makeCollectionProfileConfigurationFor(ctx context.Context, collectionProfile string) error {
	dataConfigYAMLPrometheusK8s := fmt.Sprintf("collectionProfile: %s", collectionProfile)
	dataConfigYAMLPrometheusK8sStructured := map[string]interface{}{
		"collectionProfile": collectionProfile,
	}
	dataConfigYAML := fmt.Sprintf("prometheusK8s:\n  %s", dataConfigYAMLPrometheusK8s)
	configurationEnableCollectionProfiles := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorConfigurationName,
			Namespace: operatorNamespaceName,
		},
		Data: map[string]string{
			"config.yaml": dataConfigYAML,
		},
	}

	configuration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(ctx, operatorConfigurationName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Create(ctx, configurationEnableCollectionProfiles, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		gotDataConfigYAML, ok := configuration.Data["config.yaml"]
		if !ok {
			configuration.Data = make(map[string]string)
			configuration.Data["config.yaml"] = dataConfigYAML
		} else {
			var gotDataConfigYAMLMap map[string]interface{}
			err = yaml.Unmarshal([]byte(gotDataConfigYAML), &gotDataConfigYAMLMap)
			if err != nil {
				return err
			}
			if _, ok := gotDataConfigYAMLMap["prometheusK8s"]; !ok {
				gotDataConfigYAMLMap["prometheusK8s"] = dataConfigYAMLPrometheusK8sStructured
			} else {
				gotDataConfigYAMLMap["prometheusK8s"].(map[string]interface{})["collectionProfile"] = collectionProfile
			}
			gotDataConfigYAMLRaw, err := yaml.Marshal(gotDataConfigYAMLMap)
			if err != nil {
				return err
			}
			gotDataConfigYAML = string(gotDataConfigYAMLRaw)
			configuration.Data["config.yaml"] = gotDataConfigYAML
		}
		currentConfiguration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(ctx, operatorConfigurationName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		currentConfiguration.Data = configuration.Data
		_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Update(ctx, currentConfiguration, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
