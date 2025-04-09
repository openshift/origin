package auditloganalyzer

import (
	"net/http"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
)

type invalidRequests struct {
	lock sync.Mutex

	verbToNamespacesTouserToNumberOf422s map[string]*namespaceInvalidRequestTracker
}

type namespaceInvalidRequestTracker struct {
	namespacesToInvalidUserTrackers map[string]*userInvalidRequestTracker
}

func (a *namespaceInvalidRequestTracker) addFailure(event auditv1.Event) {
	if a.namespacesToInvalidUserTrackers == nil {
		a.namespacesToInvalidUserTrackers = map[string]*userInvalidRequestTracker{}
	}
	nsName, _, _ := serviceaccount.SplitUsername(event.User.Username)
	if _, ok := a.namespacesToInvalidUserTrackers[nsName]; !ok {
		a.namespacesToInvalidUserTrackers[nsName] = &userInvalidRequestTracker{}
	}
	a.namespacesToInvalidUserTrackers[nsName].addFailure(event)
}

type userInvalidRequestTracker struct {
	usersToInvalidRequests map[string]*invalidRequestDetails
}

func (a *userInvalidRequestTracker) addFailure(event auditv1.Event) {
	if a.usersToInvalidRequests == nil {
		a.usersToInvalidRequests = map[string]*invalidRequestDetails{}
	}
	username := event.User.Username
	if _, ok := a.usersToInvalidRequests[username]; !ok {
		a.usersToInvalidRequests[username] = &invalidRequestDetails{}
	}
	a.usersToInvalidRequests[username].addFailure(event)
}

type invalidRequestDetails struct {
	first10Failures       []auditv1.Event
	last10Failures        []auditv1.Event
	totalNumberOfFailures int
}

func (a *invalidRequestDetails) addFailure(event auditv1.Event) {
	a.totalNumberOfFailures++
	if len(a.first10Failures) < 10 {
		a.first10Failures = append(a.first10Failures, event)
	}
	if len(a.last10Failures) < 10 {
		a.last10Failures = append(a.last10Failures, event)
	} else {
		a.last10Failures = append(a.last10Failures[1:], event)
	}
}

func CheckForInvalidMutations() *invalidRequests {
	return &invalidRequests{
		verbToNamespacesTouserToNumberOf422s: map[string]*namespaceInvalidRequestTracker{
			"apply":  {},
			"update": {},
			"patch":  {},
			"create": {},
		},
	}
}

func isApply(auditEvent *auditv1.Event) bool {
	// only SSA
	if auditEvent.Verb != "patch" || auditEvent.Verb == "apply" {
		return false
	}
	// SSA requires a field manager
	if !strings.Contains(auditEvent.RequestURI, "fieldManager=") {
		return false
	}
	return true
}

func (s *invalidRequests) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	// only platform serviceaccounts
	if !strings.Contains(auditEvent.User.Username, ":openshift-") {
		return
	}
	// only 422 (invalid)
	if auditEvent.ResponseStatus == nil || auditEvent.ResponseStatus.Code != http.StatusUnprocessableEntity {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	verb := ""
	switch {
	case isApply(auditEvent):
		verb = "apply"
	case auditEvent.Verb == "update":
		verb = "update"
	case auditEvent.Verb == "patch":
		verb = "patch"
	case auditEvent.Verb == "create":
		verb = "create"
	default:
		return
	}
	s.verbToNamespacesTouserToNumberOf422s[verb].addFailure(*auditEvent)
}
