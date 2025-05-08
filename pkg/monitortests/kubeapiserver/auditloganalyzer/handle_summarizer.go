package auditloganalyzer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"sync"
)

type summarizer struct {
	lock            sync.Mutex
	auditLogSummary *AuditLogSummary
}

func NewAuditLogSummarizer() *summarizer {
	return &summarizer{
		auditLogSummary: NewAuditLogSummary(),
	}
}

func (s *summarizer) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.auditLogSummary.Add(auditEvent, auditEventInfo{})
}

func (s *summarizer) GetAuditLogSummary() *AuditLogSummary {
	return s.auditLogSummary
}
