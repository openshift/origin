package auditloganalyzer

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
)

type userApplyCount struct {
	User    string
	Applies int
}

type numberOfAppliesWithExamples struct {
	numberOfApplies   int
	users             map[string]int
	firstAuditEventID string
	lastAuditEventID  string
	firstOccurredAt   metav1.Time
	lastOccurredAt    metav1.Time
}

func (n numberOfAppliesWithExamples) toErrorString() string {
	appliesPerUser := []userApplyCount{}
	for k, v := range n.users {
		appliesPerUser = append(appliesPerUser, userApplyCount{k, v})
	}
	sort.Slice(appliesPerUser, func(i, j int) bool {
		return appliesPerUser[i].Applies > appliesPerUser[j].Applies
	})
	topUsernames := []string{}
	maxUsernames := 5
	if len(appliesPerUser) < maxUsernames {
		maxUsernames = len(appliesPerUser)
	}
	for i := 0; i < maxUsernames; i++ {
		topUsernames = append(topUsernames, appliesPerUser[i].User)
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

type excessiveApplies struct {
	lock                              sync.Mutex
	namespacesToUserToNumberOfApplies map[string]map[string]int
	resourcesToNumberOfApplies        map[string]numberOfAppliesWithExamples
}

func CheckForExcessiveApplies() *excessiveApplies {
	return &excessiveApplies{
		namespacesToUserToNumberOfApplies: map[string]map[string]int{},
		resourcesToNumberOfApplies:        map[string]numberOfAppliesWithExamples{},
	}
}

func (s *excessiveApplies) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	// only SSA
	if !isApply(auditEvent) || auditEvent.Verb != "update" {
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
	users[auditEvent.User.Username] += 1
	s.namespacesToUserToNumberOfApplies[nsName] = users

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
	objApplies, ok := s.resourcesToNumberOfApplies[resource]
	if !ok {
		objApplies = numberOfAppliesWithExamples{
			numberOfApplies:   0,
			firstAuditEventID: string(auditEvent.AuditID),
			lastAuditEventID:  string(auditEvent.AuditID),
			users:             map[string]int{},
			firstOccurredAt:   metav1.Time(auditEvent.RequestReceivedTimestamp),
			lastOccurredAt:    metav1.Time(auditEvent.RequestReceivedTimestamp),
		}
	}
	objApplies.numberOfApplies += 1
	objApplies.lastAuditEventID = string(auditEvent.AuditID)
	objApplies.lastOccurredAt = metav1.Time(auditEvent.RequestReceivedTimestamp)
	userApplies, ok := objApplies.users[auditEvent.User.Username]
	if !ok {
		userApplies = 0
	}
	objApplies.users[auditEvent.User.Username] = userApplies + 1
	s.resourcesToNumberOfApplies[resource] = objApplies
}
