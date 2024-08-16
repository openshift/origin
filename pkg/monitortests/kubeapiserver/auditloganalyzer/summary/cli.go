package summary

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer/eventsprovider"
	"k8s.io/client-go/kubernetes"
)

func GetKubeAuditLogSummary(ctx context.Context, kubeClient kubernetes.Interface, beginning, end *time.Time) (*AuditLogSummary, error) {
	ret := NewAuditLogSummary()
	for event := range eventsprovider.NewClusterEvents(beginning, end).Events(ctx, kubeClient) {
		ret.Add(event)
	}
	return ret, nil
}
