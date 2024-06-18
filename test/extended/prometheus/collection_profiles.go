package prometheus

import (
	"context"
	"fmt"
	"strings"
	"time"

	pv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	pov1api "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	pov1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
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
	mclient                       *pov1client.MonitoringV1Client
	pclient                       pv1.API
	originalOperatorConfiguration *v1.ConfigMap
}

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
		r.mclient, err = pov1client.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("failed to create monitoring client: %v", err))
		}
		r.pclient = oc.NewPrometheusClient(tctx)
		operatorConfiguration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		r.originalOperatorConfiguration = operatorConfiguration
	})

	g.AfterAll(func() {
		currentConfiguration, err := r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Get(tctx, operatorConfigurationName, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		currentConfiguration.Data = r.originalOperatorConfiguration.Data
		_, err = r.kclient.CoreV1().ConfigMaps(operatorNamespaceName).Update(tctx, currentConfiguration, metav1.UpdateOptions{})
		o.Expect(err).To(o.BeNil())
	})

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
	g.It("should have at-least one implementation for each collection profile", func() {
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
	g.It("should hide a default-only metric when minimal collection profile is enabled", func() {
		defaultOnlyMetric := "kube_deployment_status_replicas"

		// DEBUG
		defaultOnlyMetricQuery := fmt.Sprintf("absent(%s @ %d) == 1", defaultOnlyMetric, time.Now().Unix())
		queryResponse, err := helper.RunQuery(tctx, r.pclient, defaultOnlyMetricQuery)
		o.Expect(err).To(o.BeNil())
		if len(queryResponse.Data.Result) == 0 {
			fmt.Printf("DEBUG: %q is present\n", defaultOnlyMetric)
		} else {
			fmt.Printf("DEBUG: %q is absent\n", defaultOnlyMetric)
		}

		for i, profile := range []string{collectionProfileFull, collectionProfileMinimal} {
			err := r.makeCollectionProfileConfigurationFor(tctx, profile)
			o.Expect(err).To(o.BeNil())

			o.Eventually(func() error {
				defaultOnlyMetricQuery := fmt.Sprintf("absent(%s @ %d) == 1", defaultOnlyMetric, time.Now().Unix())
				queryResponse, err := helper.RunQuery(tctx, r.pclient, defaultOnlyMetricQuery)
				if err != nil {
					return err
				}
				if i == 0 && len(queryResponse.Data.Result) != 0 {
					return fmt.Errorf("expected %q to be present", defaultOnlyMetric)
				}
				if i == 1 && len(queryResponse.Data.Result) == 0 {
					return fmt.Errorf("expected %q to be absent", defaultOnlyMetric)
				}

				return nil
			}).Should(o.BeNil())
		}
	})
	g.It("should revert back to default collection profile when none is specified", func() {
		err := r.makeCollectionProfileConfigurationFor(tctx, collectionProfileNone)
		o.Expect(err).To(o.BeNil())

		for i, profile := range []string{collectionProfileMinimal, collectionProfileFull} {
			o.Eventually(func() error {
				monitors, err := r.fetchMonitorsFor([2]string{collectionProfileFeatureLabel, profile})
				if err != nil {
					return err
				}
				if i == 0 && len(monitors.Items) != 0 {
					return fmt.Errorf("monitors found with collection profile %q", profile)
				}
				if i == 1 && len(monitors.Items) == 0 {
					return fmt.Errorf("no monitors found with collection profile %q", profile)
				}

				return nil
			}).Should(o.BeNil())

		}
	})
})

func (r *runner) fetchMonitorsFor(selectors ...[2]string) (*pov1api.ServiceMonitorList, error) {
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

func (r *runner) makeCollectionProfileConfigurationFor(ctx context.Context, collectionProfile string) error {
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
