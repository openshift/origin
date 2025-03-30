package auditloganalyzer

import (
	"github.com/stretchr/testify/assert"
	authnv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"testing"
	"time"
)

func Test_WatchCountTrackerEventHandler(t *testing.T) {
	handler := NewWatchCountTracking()
	mTime := metav1.NewMicroTime(time.Now())

	// auditEvent.Verb != "watch" || auditEvent.Stage != auditv1.StageResponseComplete || !strings.HasSuffix(auditEvent.User.Username, "-operator")

	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		Verb:                     "watch",
		Stage:                    auditv1.StageResponseComplete,
		User:                     authnv1.UserInfo{Username: "test-operator"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{"apiserver.latency.k8s.io/etcd": "15.999592078s", "apiserver.latency.k8s.io/response-write": "780ns", "apiserver.latency.k8s.io/serialize-response-object": "3.746852ms", "apiserver.latency.k8s.io/total": "16.005122724s"},
	}, &mTime, nil, "testNode")

	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		Verb:                     "watch",
		Stage:                    auditv1.StageResponseComplete,
		User:                     authnv1.UserInfo{Username: "test-operator"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{"apiserver.latency.k8s.io/etcd": "15.999592078ms", "apiserver.latency.k8s.io/response-write": "780ns", "apiserver.latency.k8s.io/serialize-response-object": "3.746852ms", "apiserver.latency.k8s.io/total": "16.005122724ms"},
	}, &mTime, nil, "testNode")

	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{"apiserver.latency.k8s.io/etcd": "15.999592078s", "apiserver.latency.k8s.io/response-write": "780ns", "apiserver.latency.k8s.io/serialize-response-object": "3.746852ms", "apiserver.latency.k8s.io/total": "16.005122724s"},
	}, &mTime, nil, "testNode")

	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		Verb:                     "watch",
		Stage:                    auditv1.StageResponseComplete,
		User:                     authnv1.UserInfo{Username: "test-nonoperator"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{},
	}, &mTime, nil, "testNode")

	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		Verb:                     "watch",
		Stage:                    auditv1.StageResponseComplete,
		User:                     authnv1.UserInfo{Username: "test-operator"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{},
	}, &mTime, nil, "testNode2")

	// 3 total operator watch requests, max of 2 for testNode

	watchRequestCounts := handler.SummarizeWatchCountRequests()
	assert.NotNil(t, watchRequestCounts)
	assert.Equal(t, 1, len(watchRequestCounts))
	assert.Equal(t, int64(2), watchRequestCounts[0].Count)
	assert.Equal(t, "testNode", watchRequestCounts[0].NodeName)
}
