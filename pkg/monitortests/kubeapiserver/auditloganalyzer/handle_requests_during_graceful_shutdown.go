package auditloganalyzer

import (
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

type lateRequestTracking struct {
	lock     sync.Mutex
	auditIDs []string
}

func CheckForRequestsDuringShutdown() *lateRequestTracking {
	return &lateRequestTracking{}
}

func (l *lateRequestTracking) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}
	if value, ok := auditEvent.Annotations["openshift.io/during-graceful"]; ok {
		// Ignore disruptions done over loopback interface
		value_slice := strings.Split(value, ",")
		if len(value_slice) == 0 {
			return
		}
		if value_slice[0] == "loopback=true" {
			return
		}
		l.auditIDs = append(l.auditIDs, string(auditEvent.AuditID))
	}
}
