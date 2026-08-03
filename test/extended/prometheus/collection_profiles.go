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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
)

// These constants are defined in the Cluster Monitoring Operator and need to
// be kept in sync.
const (
	// collectionProfileFeatureLabel is the Kubernetes label identifying the
	// collection profile associated to the monitoring resource (ServiceMonitor
	// or PodMonitor)
	collectionProfileFeatureLabel = "monitoring.openshift.io/collection-profile"

	// collectionProfileFull is the profile enabling the collection of all metrics.
	collectionProfileFull = "full"

	// collectionProfileMinimal is the profile enabling the collection of
	// metrics used for Telemetry, alerting and dashboards.
	collectionProfileMinimal = "minimal"

	collectionProfileEmpty = ""

	// collectionProfileDefault is the default collection profile (currently: full).
	collectionProfileDefault = collectionProfileFull
)

const (
	projectName = "monitoring-collection-profiles"

	operatorName              = "cluster-monitoring-operator"
	operatorNamespaceName     = "openshift-monitoring"
	operatorConfigurationName = "cluster-monitoring-config"

	pollTimeout  = 15 * time.Minute
	pollInterval = 5 * time.Second
)

var (
	// collectionProfilesSupportedList is the list of all collection profiles
	// supported by the Cluster Monitoring Operator. It is populated at runtime
	// to account for new profiles being added over time.
	collectionProfilesSupportedList []string
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

		// Save the current configuration and enabled the default collection profile.
		var operatorConfiguration *v1.ConfigMap
		o.Eventually(func() error {
			operatorConfiguration, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					g.By("initially, creating a configuration for the operator as it did not exist")
					operatorConfiguration = nil
					return r.configureCollectionProfile(tctx, collectionProfileDefault)
				}

				return err
			}

			return nil
		}, pollTimeout, pollInterval).Should(o.BeNil())
		r.originalOperatorConfiguration = operatorConfiguration

		// Discover all supported collection profiles.
		o.Eventually(func() error {
			var err error
			collectionProfilesSupportedList, err = r.getSupportedCollectionProfiles(tctx)
			return err
		}, pollTimeout, pollInterval).Should(o.BeNil())
	})

	// Restore the Cluster Monitoring Operator's configuration.
	g.AfterAll(func() {
		currentConfiguration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())

		if r.originalOperatorConfiguration != nil {
			currentConfiguration.Data = r.originalOperatorConfiguration.Data
			g.By("restoring the original configuration for the operator")
			_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Update(tctx, currentConfiguration, metav1.UpdateOptions{})
		} else {
			g.By("deleting the cluster monitoring operator's configuration since it did not exist pre-job")
			err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Delete(tctx, operatorConfigurationName, metav1.DeleteOptions{})
		}
		o.Expect(err).To(o.BeNil())

		o.Eventually(func() error {
			if r.originalOperatorConfiguration != nil {
				return nil
			}

			_, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					return nil
				}
				return err
			}

			return fmt.Errorf("ConfigMap %q still exists after deletion attempt", operatorConfigurationName)
		}, pollTimeout, pollInterval).Should(o.BeNil())
	})

	g.Context("initially, in a homogeneous default environment,", func() {
		profile := collectionProfileDefault

		g.BeforeAll(func() {
			err := r.configureCollectionProfile(tctx, profile)
			o.Expect(err).To(o.BeNil())
			o.Eventually(func() error {
				return r.assertCollectionProfileEnabled(tctx, profile)
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
				err := r.configureCollectionProfile(tctx, profile)
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
				err := r.configureCollectionProfile(tctx, profile)
				o.Expect(err).To(o.BeNil())

				o.Eventually(func() error {
					monitors, err := r.fetchMonitorsFor(tctx, label{key: collectionProfileFeatureLabel, value: profile})
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
			err := r.configureCollectionProfile(tctx, collectionProfileEmpty)
			o.Expect(err).To(o.BeNil())

			o.Eventually(func() error {
				return r.assertCollectionProfileEnabled(tctx, collectionProfileFull)
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})
	})

	g.Context("in a homogeneous minimal environment,", func() {
		profile := collectionProfileMinimal

		g.BeforeAll(func() {
			err := r.configureCollectionProfile(tctx, profile)
			o.Expect(err).To(o.BeNil())
			o.Eventually(func() error {
				return r.assertCollectionProfileEnabled(tctx, profile)
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})

		g.It("should hide default metrics", func() {
			appNameSelector := "app.kubernetes.io/name"
			appName := "kube-state-metrics"

			var kubeStateMetricsMonitor *prometheusoperatorv1.ServiceMonitor
			o.Eventually(func() error {
				monitors, err := r.fetchMonitorsFor(tctx, label{key: collectionProfileFeatureLabel, value: profile}, label{key: appNameSelector, value: appName})
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
			}, pollTimeout, pollInterval).Should(o.BeNil())

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
			}, pollTimeout, pollInterval).Should(o.BeNil())
		})
	})
})

func (r runner) assertCollectionProfileEnabled(ctx context.Context, profile string) error {
	vectorExpression := "max(profile:cluster_monitoring_operator_collection_profile:max{profile=\"%s\"}) == 1"
	queryResponse, err := helper.RunQuery(ctx, r.pclient, fmt.Sprintf(vectorExpression, profile))
	if err != nil {
		return err
	}
	if len(queryResponse.Data.Result) == 0 {
		return fmt.Errorf("collection profile %q is not enabled", profile)
	}

	return nil
}

type label struct {
	key   string
	value string
}

func (r runner) fetchMonitorsFor(ctx context.Context, selectors ...label) (*prometheusoperatorv1.ServiceMonitorList, error) {
	managedMonitorsSelectors := []string{
		fmt.Sprintf("%s=%s", "app.kubernetes.io/managed-by", operatorName),
	}
	for _, selector := range selectors {
		managedMonitorsSelectors = append(managedMonitorsSelectors, fmt.Sprintf("%s=%s", selector.key, selector.value))
	}
	return r.mclient.ServiceMonitors(operatorNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: strings.Join(managedMonitorsSelectors, ","),
	})
}

// getSupportedCollectionProfiles returns the list of supported collection
// profiles interpolating from the monitor resources installed by the Cluster
// Monitoring Operator.
func (r runner) getSupportedCollectionProfiles(ctx context.Context) ([]string, error) {
	monitors, err := r.fetchMonitorsFor(ctx)
	if err != nil {
		return nil, err
	}

	seen := sets.New[string]()
	for _, monitor := range monitors.Items {
		if profile, ok := monitor.Labels[collectionProfileFeatureLabel]; ok && profile != collectionProfileEmpty {
			seen.Insert(profile)
		}
	}

	profiles := sets.List(seen)
	if len(profiles) < 2 {
		return nil, fmt.Errorf("expected at least 2 supported collection profiles, got %d: %v", len(profiles), profiles)
	}

	return profiles, nil
}

// configureCollectionProfile udpates the Cluster Monitoring
// Operator's configuration to enable a given collection profile.
func (r runner) configureCollectionProfile(ctx context.Context, collectionProfile string) error {
	configuration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(ctx, operatorConfigurationName, metav1.GetOptions{})
	create := errors.IsNotFound(err)
	if err != nil && !create {
		return err
	}

	if create {
		configuration = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      operatorConfigurationName,
				Namespace: operatorNamespaceName,
			},
			Data: map[string]string{},
		}
	}

	var configMap map[string]interface{}
	if raw, ok := configuration.Data["config.yaml"]; ok {
		if err := yaml.Unmarshal([]byte(raw), &configMap); err != nil {
			return err
		}
	}
	if configMap == nil {
		configMap = make(map[string]interface{})
	}

	if err := unstructured.SetNestedField(configMap, collectionProfile, "prometheusK8s", "collectionProfile"); err != nil {
		return err
	}

	raw, err := yaml.Marshal(configMap)
	if err != nil {
		return err
	}
	configuration.Data["config.yaml"] = string(raw)

	if create {
		_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Create(ctx, configuration, metav1.CreateOptions{})
	} else {
		_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Update(ctx, configuration, metav1.UpdateOptions{})
	}
	return err
}
