package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnsureThanosQueriersConnectedToPromSidecars ensures that all Thanos queriers are connected to all
// Prometheus sidecars before fetching the alerts. This avoids retrieving partial data
// (possibly with gaps) when after an upgrade, one of the Prometheus
// sidecars hasn't been reconnected yet to the Thanos queriers.
func EnsureThanosQueriersConnectedToPromSidecars(ctx context.Context, prometheusClient prometheusv1.API) ([]monitorapi.Interval, error) {
	logger := logrus.WithField("func", "EnsureThanosQueriersConnectedToPromSidecars")
	var err error
	if err = wait.PollImmediateWithContext(ctx, 5*time.Second, 5*time.Minute, func(context.Context) (bool, error) {
		v, warningsForQuery, err := prometheusClient.Query(ctx, `min(count by(pod) (thanos_store_nodes_grpc_connections{store_type="sidecar",external_labels=~".*prometheus=\"openshift-monitoring/k8s\".*"})) == min(kube_statefulset_replicas{statefulset="prometheus-k8s"})`, time.Time{})
		if err != nil {
			return false, err
		}

		if len(warningsForQuery) > 0 {
			for _, w := range warningsForQuery {
				logger.Warn(w)
			}
		}

		if v.Type() != prometheustypes.ValVector {
			return false, fmt.Errorf("expecting a vector type, got %q", v.Type().String())
		}

		if len(v.(prometheustypes.Vector)) == 0 {
			logger.Warn("at least one Prometheus sidecar isn't ready")
			return false, nil
		}

		return true, nil
	}); err != nil {
		err = fmt.Errorf("thanos queriers not connected to all Prometheus sidecars: %w", err)
		logger.WithError(err).Error("timed out")
		return nil, err
	}
	return nil, nil
}
