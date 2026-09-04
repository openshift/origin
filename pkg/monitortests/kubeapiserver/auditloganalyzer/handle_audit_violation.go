package auditloganalyzer

import (
	"fmt"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"strings"
	"sync"
)

func CheckForViolations() *auditViolations {
	return &auditViolations{}
}

type auditViolations struct {
	lock    sync.Mutex
	records []auditViolationRecord
}

type auditViolationRecord struct {
	auditId   string
	violation string
	resource  string
	namespace string
	name      string
	username  string
}

func (v *auditViolations) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	if violation, ok := auditEvent.Annotations["pod-security.kubernetes.io/audit-violations"]; ok {
		v.records = append(v.records, auditViolationRecord{
			auditId:   string(auditEvent.AuditID),
			violation: violation,
			resource:  auditEvent.ObjectRef.Resource,
			namespace: auditEvent.ObjectRef.Namespace,
			name:      auditEvent.ObjectRef.Name,
			username:  auditEvent.User.Username,
		})
	}
}

func (v *auditViolations) CreateJunits() []*junitapi.JUnitTestCase {
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
					Output: fmt.Sprintf("%s\ndetails from audit log", strings.Join(messages, "\n")),
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
