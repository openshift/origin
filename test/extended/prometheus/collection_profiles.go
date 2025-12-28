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
	collectionProfileTelemetry    = "telemetry"
	collectionProfileNone         = ""

	operatorName              = "cluster-monitoring-operator"
	operatorNamespaceName     = "openshift-monitoring"
	operatorConfigurationName = "cluster-monitoring-config"

	pollTimeout  = 15 * time.Minute
	pollInterval = 5 * time.Second
)

var (
	collectionProfilesSupportedList = []string{
		collectionProfileFull,
		collectionProfileMinimal,
		collectionProfileTelemetry,
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
var _ = g.Describe("[sig-instrumentation][OCPFeatureGate:MetricsCollectionProfiles][Serial] The collection profiles feature-set", g.Ordered, func() {
	defer g.GinkgoRecover()

	r := &runner{}
	oc := exutil.NewCLI(projectName)
	tctx := context.Background()

	g.BeforeAll(func() {
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
					operatorConfiguration = nil
					return r.makeCollectionProfileConfigurationFor(tctx, collectionProfileDefault, false)
				}

				return err
			}

			return nil
		}, pollTimeout, pollInterval).Should(o.BeNil())
		r.originalOperatorConfiguration = operatorConfiguration
	})

	g.AfterAll(func() {
		shouldDeleteConfiguration := false
		currentConfiguration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		if r.originalOperatorConfiguration != nil {
			currentConfiguration.Data = r.originalOperatorConfiguration.Data
			g.By("restoring the original configuration for the operator")
			_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Update(tctx, currentConfiguration, metav1.UpdateOptions{})
		} else {
			shouldDeleteConfiguration = true
			g.By("cleaning up the configuration for the operator as it did not exist pre-job")
			err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Delete(tctx, operatorConfigurationName, metav1.DeleteOptions{})
		}
		o.Expect(err).To(o.BeNil())

		o.Eventually(func() error {
			if shouldDeleteConfiguration {
				_, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return nil
				}
				return fmt.Errorf("ConfigMap %q still exists after deletion attempt", operatorConfigurationName)
			}

			return nil
		}, pollTimeout, pollInterval).Should(o.BeNil())
	})

	g.Context("initially, in a homogeneous default environment,", func() {
		profile := collectionProfileDefault

		g.BeforeAll(func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, profile, false)
			o.Expect(err).To(o.BeNil())
			o.Eventually(func() error {
				enabled, err := r.isProfileEnabled(tctx, profile)
				if err != nil {
					return err
				}
				if !enabled {
					return fmt.Errorf("collection profile %q is not enabled", profile)
				}

				return nil
			}, pollTimeout, pollInterval).Should(o.BeNil())
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
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})
	})

	g.Context("in a heterogeneous environment,", func() {
		g.It("should expose information about the applied collection profile using meta-metrics", func() {
			for _, profile := range collectionProfilesSupportedList {
				err := r.makeCollectionProfileConfigurationFor(tctx, profile, false)
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
				}, pollTimeout, pollInterval).Should(o.BeNil())
			}
		})
		g.It("should have at least one implementation for each collection profile", func() {
			for _, profile := range collectionProfilesSupportedList {
				err := r.makeCollectionProfileConfigurationFor(tctx, profile, false)
				o.Expect(err).To(o.BeNil())

				o.Eventually(func() error {
					monitors, err := r.fetchMonitorsFor(tctx, [2]string{collectionProfileFeatureLabel, profile})
					if err != nil {
						return err
					}
					if len(monitors.Items) == 0 {
						return fmt.Errorf("no monitors found with collection profile %q", profile)
					}

					return nil
				}, pollTimeout, pollInterval).Should(o.BeNil())
			}
		})
		g.It("should revert to default collection profile when an empty collection profile value is specified", func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, collectionProfileNone, false)
			o.Expect(err).To(o.BeNil())

			o.Eventually(func() error {
				enabled, err := r.isProfileEnabled(tctx, collectionProfileFull)
				if err != nil {
					return err
				}
				if !enabled {
					return fmt.Errorf("collection profile %q is not enabled", collectionProfileFull)
				}

				return nil
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})
	})

	g.Context("in a homogeneous minimal environment,", func() {
		profile := collectionProfileMinimal

		g.BeforeAll(func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, profile, false)
			o.Expect(err).To(o.BeNil())
			o.Eventually(func() error {
				enabled, err := r.isProfileEnabled(tctx, profile)
				if err != nil {
					return err
				}
				if !enabled {
					return fmt.Errorf("collection profile %q is not enabled", profile)
				}

				return nil
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})

		g.It("should hide default metrics", func() {
			r.compareComponentRelabellingsToCountForProfile(tctx, "app.kubernetes.io/name", "kube-state-metrics", "kube_", profile)
		})
	})

	g.Context("in a homogeneous telemetry environment,", func() {
		profile := collectionProfileTelemetry

		g.BeforeAll(func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, profile, true)
			o.Expect(err).To(o.BeNil())
			o.Eventually(func() error {
				enabled, err := r.isProfileEnabled(tctx, profile)
				if err != nil {
					return fmt.Errorf("encountered error while checking if profile %q is enabled: %v", profile, err)
				}
				if !enabled {
					return fmt.Errorf("collection profile %q is not enabled", profile)
				}
				if enabledErr, err := telemetryIsEnabled(tctx, r.kclient); err != nil {
					return fmt.Errorf("failed to determine if telemetry is enabled: %v", err)
				} else if enabledErr != nil {
					return fmt.Errorf("telemetry is not enabled")
				}

				return nil
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})

		g.It("should hide default metrics", func() {
			r.compareComponentRelabellingsToCountForProfile(tctx, "app.kubernetes.io/name", "kube-state-metrics", "kube_", profile)
		})

		// this test case ensures that the (a) opted-in (in-cluster components or
		// otherwise) or (b) full/none/default collection profile monitors
		// collectively expose the same volume of telemetry metrics as they did
		// before
		g.It("should not drop any telemetry metric", func() {
			telemetryConfigMap, err := r.kclient.CoreV1().ConfigMaps("openshift-monitoring").Get(tctx, "telemetry-config", metav1.GetOptions{})
			o.Expect(err).To(o.BeNil())

			var telemetryConfig struct {
				Matches []string `yaml:"matches"`
			}
			err = yaml.Unmarshal([]byte(telemetryConfigMap.Data["metrics.yaml"]), &telemetryConfig)
			o.Expect(err).To(o.BeNil())

			var telemetryMetricsCountQuery string
			for _, match := range telemetryConfig.Matches {
				telemetryMetricsCountQuery += fmt.Sprintf("%s or ", match)
			}
			telemetryMetricsCountQuery = fmt.Sprintf("count(%s)", telemetryMetricsCountQuery[:len(telemetryMetricsCountQuery)-4])

			o.Eventually(func() error {
				telemetryMetricsCountQueryResponse, err := helper.RunQuery(tctx, r.pclient, telemetryMetricsCountQuery)
				if err != nil {
					return fmt.Errorf("failed to run constructed telemetry metrics query: %v", err)
				}
				if len(telemetryMetricsCountQueryResponse.Data.Result) == 0 {
					return fmt.Errorf("no result found for constructed telemetry metrics query")
				}
				wantCount := int(telemetryMetricsCountQueryResponse.Data.Result[0].Value)

				telemetrySelectedSeriesCountQuery := "cluster:telemetry_selected_series:count"
				telemetrySelectedSeriesCountQueryResponse, err := helper.RunQuery(tctx, r.pclient, telemetrySelectedSeriesCountQuery)
				if err != nil {
					return fmt.Errorf("failed to run metric %q: %v", telemetrySelectedSeriesCountQuery, err)
				}
				if len(telemetrySelectedSeriesCountQueryResponse.Data.Result) == 0 {
					return fmt.Errorf("no result found for metric %q", telemetrySelectedSeriesCountQuery)
				}
				gotCount := int(telemetrySelectedSeriesCountQueryResponse.Data.Result[0].Value)
				if gotCount != wantCount {
					return fmt.Errorf("compared %s against %s: got %v, want %v", telemetrySelectedSeriesCountQuery, telemetryMetricsCountQuery, gotCount, wantCount)
				}

				return nil
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})
	})
})

func (r runner) compareComponentRelabellingsToCountForProfile(tctx context.Context, appNameSelector, appName, metricSubsystem, profile string) {
	var serviceMonitor *prometheusoperatorv1.ServiceMonitor
	o.Eventually(func() error {
		monitors, err := r.fetchMonitorsFor(tctx, [2]string{collectionProfileFeatureLabel, profile}, [2]string{appNameSelector, appName})
		if err != nil {
			return err
		}
		if len(monitors.Items) == 0 {
			return fmt.Errorf("no monitors found with collection profile: %q and %#v=%q", profile, appNameSelector, appName)
		}
		if len(monitors.Items) > 1 {
			return fmt.Errorf("more than one monitor found with collection profile: %q and %#v=%q", profile, appNameSelector, appName)
		}
		serviceMonitor = monitors.Items[0]

		return nil
	}, pollTimeout, pollInterval).Should(o.BeNil())

	var metrics []string
	spec := serviceMonitor.Spec
	specEndpoints := spec.Endpoints
	if len(specEndpoints) != 0 {
		relabelings := specEndpoints[0].MetricRelabelConfigs
		if len(relabelings) != 0 {
			for _, relabeling := range relabelings {
				if relabeling.Action == "keep" &&
					len(relabeling.SourceLabels) == 1 &&
					relabeling.SourceLabels[0] == "__name__" {
					regexpString := relabeling.Regex
					subsystemRegex := regexp.MustCompile("(?U)(" + metricSubsystem + ".*)[|,)]")
					subsystemMetrics := subsystemRegex.FindAllString(regexpString, -1)
					for _, metric := range subsystemMetrics {
						if strings.HasPrefix(metric, strings.ReplaceAll(appName, "-", "_")) {
							continue
						}
						metrics = append(metrics, metric)
					}
				}
			}
		}
	}
	o.Expect(len(metrics)).To(o.BeNumerically(">", 0))

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

		metricsString := strings.Join(metrics, "")
		metricsCountQuery := fmt.Sprintf("count({__name__=~\"%s\"})", metricsString[:len(metricsString)-1 /* drop the last "|" or ")" */])
		queryResponse, err = helper.RunQuery(tctx, r.pclient, metricsCountQuery)
		if err != nil {
			return err
		}
		if len(queryResponse.Data.Result) == 0 {
			return fmt.Errorf("no result found for metric %q", metricsCountQuery)
		}
		gotCount := int(queryResponse.Data.Result[0].Value)

		if gotCount != wantCount {
			return fmt.Errorf("got %v, want %v", gotCount, wantCount)
		}

		return nil
	}, pollTimeout, pollInterval).Should(o.BeNil())
}

func (r runner) isProfileEnabled(ctx context.Context, profile string) (bool, error) {
	vectorExpression := "max(profile:cluster_monitoring_operator_collection_profile:max{profile=\"%s\"}) == 1"
	queryResponse, err := helper.RunQuery(ctx, r.pclient, fmt.Sprintf(vectorExpression, profile))
	if err != nil {
		return false, err
	}
	if len(queryResponse.Data.Result) == 0 {
		return false, nil
	}

	return true, nil
}

func (r runner) fetchMonitorsFor(ctx context.Context, selectors ...[2]string) (*prometheusoperatorv1.ServiceMonitorList, error) {
	managedMonitorsSelectors := []string{
		fmt.Sprintf("%s=%s", "app.kubernetes.io/managed-by", operatorName),
	}
	for _, selector := range selectors {
		managedMonitorsSelectors = append(managedMonitorsSelectors, fmt.Sprintf("%s=%s", selector[0], selector[1]))
	}
	return r.mclient.ServiceMonitors(operatorNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(managedMonitorsSelectors, ","),
	})
}

func (r runner) makeCollectionProfileConfigurationFor(ctx context.Context, collectionProfile string, enableTelemetry bool) error {
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
			if enableTelemetry {
				if _, ok := gotDataConfigYAMLMap["telemeterClient"]; ok {
					gotDataConfigYAMLMap["telemeterClient"].(map[string]interface{})["enabled"] = true
				}
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
