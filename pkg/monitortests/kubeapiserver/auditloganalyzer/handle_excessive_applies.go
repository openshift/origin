package auditloganalyzer

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"strings"
	"sync"
)

type excessiveApplies struct {
	lock                              sync.Mutex
	namespacesToUserToNumberOfApplies map[string]map[string]int
}

func CheckForExcessiveApplies() *excessiveApplies {
	return &excessiveApplies{
		namespacesToUserToNumberOfApplies: map[string]map[string]int{},
	}
}

func (s *excessiveApplies) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	// only SSA
	if !isApply(auditEvent) {
		return
	}
	// only platform serviceaccounts
	if !strings.Contains(auditEvent.User.Username, ":openshift-") {
		return
	}
	nsName, _, _ := serviceaccount.SplitUsername(auditEvent.User.Username)

	s.lock.Lock()
	defer s.lock.Unlock()

	users, ok := s.namespacesToUserToNumberOfApplies[nsName]
	if !ok {
		users = map[string]int{}
	}
	users[auditEvent.User.Username] = users[auditEvent.User.Username] + 1
	s.namespacesToUserToNumberOfApplies[nsName] = users
}
