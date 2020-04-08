package clusteroperatormetric

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
)

var clusterOperatorStateMetric *metrics.GaugeVec

func init() {
	clusterOperatorStateMetric = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Name: "openshift_ci_monitor_operator_cluster_operator_status",
			Help: "A metric that tracks individual cluster operator status.",
		},
		[]string{"name", "condition", "status"},
	)

	legacyregistry.MustRegister(clusterOperatorStateMetric)
}

type ClusterOperatorMetricController struct {
	clusterOperatorClient configv1client.ClusterOperatorsGetter
}

func NewClusterOperatorMetricController(clusterOperatorInformer cache.SharedInformer, clusterOperatorGetter configv1client.ClusterOperatorsGetter, recorder events.Recorder) factory.Controller {
	c := &ClusterOperatorMetricController{
		clusterOperatorClient: clusterOperatorGetter,
	}
	return factory.New().WithInformers(clusterOperatorInformer).WithSync(c.sync).ResyncEvery(1*time.Minute).ToController("ClusterOperatorMetricController", recorder.WithComponentSuffix("cluster-operator-metric"))
}

func (c *ClusterOperatorMetricController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	clusterOperators, err := c.clusterOperatorClient.ClusterOperators().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, operator := range clusterOperators.Items {
		for _, condition := range operator.Status.Conditions {
			clusterOperatorStateMetric.WithLabelValues(operator.Name, string(condition.Type), string(condition.Status)).Set(float64(condition.LastTransitionTime.Unix()))
		}
	}
	return nil
}
