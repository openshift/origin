package auditloganalyzer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"strings"
	"sync"
)

type excessiveApplies struct {
	lock                  sync.Mutex
	userToNumberOfApplies map[string]int
}

func CheckForExcessiveApplies() *excessiveApplies {
	return &excessiveApplies{
		userToNumberOfApplies: map[string]int{},
	}
}

func (s *excessiveApplies) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	// only SSA
	if auditEvent.Verb != "patch" {
		return
	}
	// only platform serviceaccounts
	if !strings.Contains(auditEvent.User.Username, ":openshift-") {
		return
	}
	// SSA requires a field manager
	if !strings.Contains(auditEvent.RequestURI, "fieldManager=") {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.userToNumberOfApplies[auditEvent.User.Username] = s.userToNumberOfApplies[auditEvent.User.Username] + 1
}
