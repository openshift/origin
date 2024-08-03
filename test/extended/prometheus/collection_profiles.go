package prometheus

import (
	"context"
	"fmt"
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
		if !exutil.IsTechPreviewNoUpgrade(oc) {
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
			return err
		}).Should(o.BeNil())
		r.originalOperatorConfiguration = operatorConfiguration
	})

	g.AfterAll(func() {
		currentConfiguration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		currentConfiguration.Data = r.originalOperatorConfiguration.Data
		_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Update(tctx, currentConfiguration, metav1.UpdateOptions{})
		o.Expect(err).To(o.BeNil())
	})

	var _ = g.Context("in an environment that migrates from default to minimal collection profile", func() {
		g.It("should apply default collection profile initially", func() {
			o.Eventually(func() error {
				monitors, err := r.fetchMonitorsFor([2]string{collectionProfileFeatureLabel, collectionProfileDefault})
				if err != nil {
					return err
				}
				if len(monitors.Items) == 0 {
					return fmt.Errorf("no monitors found with collection profile %q", collectionProfileDefault)
				}

				return nil
			}).Should(o.BeNil())
		})
		g.It("should expose information about the applied collection profile using meta-metrics", func() {
			vectorExpression := "profile:cluster_monitoring_operator_collection_profile:max{profile=\"%s\"} == 1"

			for _, profile := range collectionProfilesSupportedList {
				err := r.makeCollectionProfileConfigurationFor(tctx, profile)
				o.Expect(err).To(o.BeNil())

				o.Eventually(func() error {
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
	})

	var _ = g.Context("in an environment that has the minimal profile pre-set", func() {
		g.It("should hide all default-only metrics when minimal collection profile is enabled", func() {
			profile := collectionProfileMinimal
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

			var regexpStringWholeMetricNamesCount int
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
							regexpStringWholeMetricNamesCount += strings.Count(regexpString, `|`) + 1
						}
					}
				}
			}
			o.Expect(regexpStringWholeMetricNamesCount).To(o.BeNumerically(">", 0))

			o.Eventually(func() error {
				preRelabelingMetric := "scrape_samples_scraped"
				var relabelledMetricsCountPre int
				postRelabelingMetric := "scrape_samples_post_metric_relabeling"
				var relabelledMetricsCountPost int
				for j, metric := range []string{preRelabelingMetric, postRelabelingMetric} {
					relabelingMetricQuery := fmt.Sprintf("max(%s{job=\"%s\",endpoint=\"https-main\"})", metric, appName)
					queryResponse, err := helper.RunQuery(tctx, r.pclient, relabelingMetricQuery)
					if err != nil {
						return err
					}
					if len(queryResponse.Data.Result) == 0 {
						return fmt.Errorf("no result found for metric %q", metric)
					}
					if j == 0 {
						relabelledMetricsCountPre = int(queryResponse.Data.Result[0].Value)
					}
					if j == 1 {
						relabelledMetricsCountPost = int(queryResponse.Data.Result[0].Value)
					}
				}
				if profile == collectionProfileMinimal && relabelledMetricsCountPost-relabelledMetricsCountPre != regexpStringWholeMetricNamesCount {
					return fmt.Errorf("relabelled metrics count mismatch for profile %q, pre: %d, post: %d", profile, relabelledMetricsCountPre, relabelledMetricsCountPost)
				}

				return nil
			}).Should(o.BeNil())
		})
	})

	var _ = g.Context("in an environment that migrates from minimal to default collection profile", func() {
		g.It("should revert back to default collection profile when none is specified", func() {
			err := r.makeCollectionProfileConfigurationFor(tctx, collectionProfileNone)
			o.Expect(err).To(o.BeNil())

			o.Eventually(func() error {
				respectsProfile, err := r.respectsProfileInPodMonitorSelector(collectionProfileFull)
				if err != nil {
					return err
				}
				if !respectsProfile {
					return fmt.Errorf("collection profile %q is not respected", collectionProfileFull)
				}

				return nil
			}).Should(o.BeNil())
		})
	})
})

func (r runner) respectsProfileInPodMonitorSelector(profile string) (bool, error) {
	p, err := r.mclient.Prometheuses(operatorNamespaceName).Get(tctx, "k8s", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	podMonitorSelectors := p.Spec.PodMonitorSelector
	for _, podMonitorSelector := range podMonitorSelectors.MatchExpressions {
		if podMonitorSelector.Key == collectionProfileFeatureLabel {
			if podMonitorSelector.Operator == metav1.LabelSelectorOpNotIn {
				for _, value := range podMonitorSelector.Values {
					if value == profile {
						return false, nil
					}
				}
				return true, nil
			} else if podMonitorSelector.Operator == metav1.LabelSelectorOpIn {
				for _, value := range podMonitorSelector.Values {
					if value == profile {
						return true, nil
					}
				}
				return false, nil
			} else {
				return false, fmt.Errorf("unexpected operator: %#q", podMonitorSelector.Operator)
			}
		}
	}

	return false, nil
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
