package summary

import (
	"context"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func NewAnalyzer() auditloganalyzer.AuditEventCollector {
	return &analyzer{AuditLogSummary: NewAuditLogSummary()}
}

type analyzer struct {
	*AuditLogSummary
}

var _ auditloganalyzer.StorageContentWriter = &analyzer{}

func (a *analyzer) Collect(event *auditv1.Event) {
	a.Add(event)
}

func (a *analyzer) SaveArtifacts(_ context.Context, storageDir, timeSuffix string, _ monitorapi.Intervals, _ monitorapi.ResourcesMap) error {
	return WriteAuditLogSummary(storageDir, timeSuffix, a.AuditLogSummary)
}
