package auditloganalyzer

import (
	"fmt"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"strings"
	"sync"
)

func CheckForViolations() *violations {
	return &violations{}
}

type violations struct {
	lock    sync.Mutex
	records []violationRecord
}

type violationRecord struct {
	auditId   string
	violation string
	resource  string
	namespace string
	name      string
	username  string
}

func (v *violations) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	if violation, ok := auditEvent.Annotations["pod-security.kubernetes.io/audit-violations"]; ok {
		v.records = append(v.records, violationRecord{
			auditId:   string(auditEvent.AuditID),
			violation: violation,
			resource:  auditEvent.ObjectRef.Resource,
			namespace: auditEvent.ObjectRef.Namespace,
			name:      auditEvent.ObjectRef.Namespace,
			username:  auditEvent.User.Username,
		})
	}
}

func (v *violations) CreateJunits() []*junitapi.JUnitTestCase {
	ret := []*junitapi.JUnitTestCase{}

	testName := " [bz-apiserver-auth][invariant] audit analysis PodSecurityViolation"
	switch {
	case len(v.records) > 0:
		messages := []string{}
		for _, v := range v.records {
			messages = append(messages, fmt.Sprintf("%s: %s %s/%s: %s - %s", v.auditId, v.resource, v.namespace, v.name, v.username, v.violation))
		}
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: fmt.Sprintf("%s", strings.Join(messages, "\n")),
					Output:  "details from audit log",
				},
			},
		)
	default:
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	return ret
}
