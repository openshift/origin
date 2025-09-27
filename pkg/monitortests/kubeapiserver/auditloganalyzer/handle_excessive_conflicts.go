package auditloganalyzer

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
)

type userConflictsCount struct {
	User      string
	Conflicts int
}

type numberOfConflictsWithExamples struct {
	numberOfConflicts int
	users             map[string]int
	firstAuditEventID string
	lastAuditEventID  string
	firstOccurredAt   metav1.Time
	lastOccurredAt    metav1.Time
}

func (n numberOfConflictsWithExamples) toErrorString() string {
	conflictsPerUser := []userConflictsCount{}
	for k, v := range n.users {
		conflictsPerUser = append(conflictsPerUser, userConflictsCount{k, v})
	}
	sort.Slice(conflictsPerUser, func(i, j int) bool {
		return conflictsPerUser[i].Conflicts > conflictsPerUser[j].Conflicts
	})
	topUsernames := []string{}
	maxUsernames := 5
	if len(conflictsPerUser) < maxUsernames {
		maxUsernames = len(conflictsPerUser)
	}
	for i := 0; i < maxUsernames; i++ {
		topUsernames = append(topUsernames, conflictsPerUser[i].User)
	}

	return fmt.Sprintf(`
Time: started %s, finished at %s
Top 5 usernames: %s
Audit Log IDs: %s ... %s
`,
		n.firstOccurredAt,
		n.lastOccurredAt,
		strings.Join(topUsernames, ", "),
		n.firstAuditEventID,
		n.lastAuditEventID,
	)
}

type excessiveConflicts struct {
	lock                                sync.Mutex
	namespacesToUserToNumberOfConflicts map[string]map[string]int
	resourcesToNumberOfConflicts        map[string]numberOfConflictsWithExamples
}

func CheckForExcessiveConflicts() *excessiveConflicts {
	return &excessiveConflicts{
		namespacesToUserToNumberOfConflicts: map[string]map[string]int{},
		resourcesToNumberOfConflicts:        map[string]numberOfConflictsWithExamples{},
	}
}

func (s *excessiveConflicts) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	if auditEvent.ResponseStatus == nil || auditEvent.ResponseStatus.Code != http.StatusConflict {
		return
	}
	// only platform serviceaccounts
	if !strings.Contains(auditEvent.User.Username, ":openshift-") {
		return
	}
	nsName, _, _ := serviceaccount.SplitUsername(auditEvent.User.Username)

	s.lock.Lock()
	defer s.lock.Unlock()

	users, ok := s.namespacesToUserToNumberOfConflicts[nsName]
	if !ok {
		users = map[string]int{}
	}
	users[auditEvent.User.Username] += 1
	s.namespacesToUserToNumberOfConflicts[nsName] = users

	obj := auditEvent.ObjectRef
	if obj == nil {
		return
	}
	resource := fmt.Sprintf("%s/%s", obj.Resource, obj.Name)
	if obj.Namespace != "" {
		resource = fmt.Sprintf("%s -n %s", resource, obj.Namespace)
	}
	if obj.APIGroup != "" {
		resource = fmt.Sprintf("%s.%s", obj.APIGroup, resource)
	}
	objConflicts, ok := s.resourcesToNumberOfConflicts[resource]
	if !ok {
		objConflicts = numberOfConflictsWithExamples{
			numberOfConflicts: 0,
			firstAuditEventID: string(auditEvent.AuditID),
			lastAuditEventID:  string(auditEvent.AuditID),
			users:             map[string]int{},
			firstOccurredAt:   metav1.Time(auditEvent.RequestReceivedTimestamp),
			lastOccurredAt:    metav1.Time(auditEvent.RequestReceivedTimestamp),
		}
	}
	objConflicts.numberOfConflicts += 1
	objConflicts.lastAuditEventID = string(auditEvent.AuditID)
	objConflicts.lastOccurredAt = metav1.Time(auditEvent.RequestReceivedTimestamp)
	userConflicts, ok := objConflicts.users[auditEvent.User.Username]
	if !ok {
		userConflicts = 0
	}
	objConflicts.users[auditEvent.User.Username] = userConflicts + 1
	s.resourcesToNumberOfConflicts[resource] = objConflicts
}
