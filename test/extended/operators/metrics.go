package operators

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	config "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
)

var _ = g.Describe("OpenShift Operators", func() {

	var (
		clusterOperatorInfos []clusterOperatorInfo
		kubeClient           kubernetes.Interface
		dynamicClient        dynamic.Interface
		serviceMonitorGvr    schema.GroupVersionResource = schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}
		skip                 sets.String                 = sets.NewString(
			"cluster-autoscaler",
			"cloud-credential",
			"cluster-autoscaler",
			"image-registry",
			"insights",
			"machine-api",
			"machine-config",
			"marketplace",
			"monitoring",
			"network",
			"node-tuning",
			"openshift-samples",
			"operator-lifecycle-manager",
			"operator-lifecycle-manager-catalog",
			"operator-lifecycle-manager-packageserver",
			"service-ca",
			"service-catalog-apiserver",
			"service-catalog-controller-manager",
			"storage",
			"support",
		)
	)

	g.BeforeEach(func() {
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).ToNot(o.HaveOccurred())
		kubeClient, err = kubernetes.NewForConfig(kubeConfig)
		configClient, err := configclient.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		dynamicClient, err = dynamic.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterOperatorsList, err := configClient.ClusterOperators().List(metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		for _, clusterOperator := range clusterOperatorsList.Items {
			if !skip.Has(clusterOperator.Name) {
				clusterOperatorInfos = append(clusterOperatorInfos, newClusterOperatorInfo(clusterOperator))
			}
		}
	})

	var _ = g.It("must define a service to be monitored", func() {
		var errs []error
		for _, clusterOperatorInfo := range clusterOperatorInfos {
			operatorNamespace := clusterOperatorInfo.ExpectedNamespace()
			serviceMonitorName := clusterOperatorInfo.ExpectedDeploymentName()
			_, err := dynamicClient.Resource(serviceMonitorGvr).Namespace(operatorNamespace).Get(serviceMonitorName, metav1.GetOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}
		o.Expect(errs).To(o.BeEmpty())

		for _, clusterOperatorInfo := range clusterOperatorInfos {
			operatorNamespace := clusterOperatorInfo.ExpectedNamespace()
			serviceMonitorName := clusterOperatorInfo.ExpectedDeploymentName()

			e2e.Logf("ClusterOperatorInfo: %s", clusterOperatorInfo.Name())
			e2e.Logf("ClusterOperatorInfo: %s", clusterOperatorInfo.ExpectedDeploymentName())

			g.By("Defining a ServiceMonitor")
			serviceMonitor, err := dynamicClient.Resource(serviceMonitorGvr).Namespace(operatorNamespace).Get(serviceMonitorName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			g.By("Defining a ServiceMonitor endpoint to a 'metrics' service")
			endpoints, _, err := unstructured.NestedSlice(serviceMonitor.UnstructuredContent(), "spec", "endpoints")
			o.Expect(err).ToNot(o.HaveOccurred())
			o.Expect(endpoints).To(o.HaveLen(1))
			metricsServerName, _, err := unstructured.NestedString(endpoints[0].(map[string]interface{}), "tlsConfig", "serverName")
			o.Expect(err).ToNot(o.HaveOccurred())
			o.Expect(metricsServerName).To(o.Equal("metrics." + operatorNamespace + ".svc"))

			g.By("Defining a 'metrics' service")
			_, err = kubeClient.CoreV1().Services(operatorNamespace).Get("metrics", metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())
		}
	})

	var _ = g.It("metrics must be successfully scraped by prometheus", func() {
		for _, clusterOperatorInfo := range clusterOperatorInfos {
			// TODO query prometheus for some expected metrics and decide if they are being updated or not
			e2e.Logf("TODO verify metrics scrapped for %s", clusterOperatorInfo.ExpectedDeploymentName())
		}
	})

})

type clusterOperatorInfo struct {
	clusterOperator config.ClusterOperator
}

func newClusterOperatorInfo(clusterOperator config.ClusterOperator) clusterOperatorInfo {
	return clusterOperatorInfo{
		clusterOperator: clusterOperator,
	}
}

func (c *clusterOperatorInfo) Name() string {
	return c.clusterOperator.Name
}

func (c *clusterOperatorInfo) ExpectedNamespace() string {
	namespace := c.clusterOperator.Name + "-operator"
	if !strings.HasPrefix(namespace, "openshift-") {
		namespace = "openshift-" + namespace
	}
	return namespace
}

func (c *clusterOperatorInfo) ExpectedDeploymentName() string {
	return c.clusterOperator.Name + "-operator"
}
