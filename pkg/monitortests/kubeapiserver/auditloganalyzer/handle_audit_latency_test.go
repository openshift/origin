package auditloganalyzer

import (
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"testing"
	"time"
)

func Test_auditLogEventHandler(t *testing.T) {
	handler := CheckForLatency()
	mTime := metav1.NewMicroTime(time.Now())

	//only one event over 15 seconds
	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		Stage:                    auditv1.StageResponseComplete,
		Verb:                     "list",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{"apiserver.latency.k8s.io/etcd": "15.999592078s", "apiserver.latency.k8s.io/response-write": "780ns", "apiserver.latency.k8s.io/serialize-response-object": "3.746852ms", "apiserver.latency.k8s.io/total": "16.005122724s"},
	}, &mTime, nil)

	// all sub second so default only
	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		Stage:                    auditv1.StageResponseComplete,
		Verb:                     "list",
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{"apiserver.latency.k8s.io/etcd": "5.999592078ms", "apiserver.latency.k8s.io/response-write": "780ns", "apiserver.latency.k8s.io/serialize-response-object": "0.746852ms", "apiserver.latency.k8s.io/total": "6.005122724ms"},
	}, &mTime, nil)

	// total is over 2s but etcd is under
	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		Stage:                    auditv1.StageResponseComplete,
		Verb:                     "list",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{"apiserver.latency.k8s.io/etcd": "1.999592078s", "apiserver.latency.k8s.io/response-write": "780ns", "apiserver.latency.k8s.io/serialize-response-object": "0.746852ms", "apiserver.latency.k8s.io/total": "2.005122724s"},
	}, &mTime, nil)

	// no annotations so only default (0) total is incremented
	handler.HandleAuditLogEvent(&auditv1.Event{
		AuditID:                  "audit-id",
		Stage:                    auditv1.StageResponseComplete,
		Verb:                     "list",
		ObjectRef:                &auditv1.ObjectReference{Name: "testName", Resource: "testResource", Namespace: "testNamespace"},
		RequestReceivedTimestamp: mTime,
		Annotations:              map[string]string{},
	}, &mTime, nil)

	assert.NotNil(t, handler.summary.resourceBuckets)

	assert.Equal(t, int64(1), handler.summary.resourceBuckets["apiserver.latency.k8s.io/total"]["testResource"].buckets[10.0].totalCounts["list"])
	assert.Equal(t, int64(1), handler.summary.resourceBuckets["apiserver.latency.k8s.io/etcd"]["testResource"].buckets[10.0].totalCounts["list"])

	assert.Equal(t, int64(1), handler.summary.resourceBuckets["apiserver.latency.k8s.io/total"]["testResource"].buckets[5.0].totalCounts["list"])
	assert.Equal(t, int64(1), handler.summary.resourceBuckets["apiserver.latency.k8s.io/etcd"]["testResource"].buckets[5.0].totalCounts["list"])

	assert.Equal(t, int64(2), handler.summary.resourceBuckets["apiserver.latency.k8s.io/total"]["testResource"].buckets[2.0].totalCounts["list"])
	assert.Equal(t, int64(1), handler.summary.resourceBuckets["apiserver.latency.k8s.io/etcd"]["testResource"].buckets[2.0].totalCounts["list"])

	assert.Equal(t, int64(2), handler.summary.resourceBuckets["apiserver.latency.k8s.io/total"]["testResource"].buckets[1.0].totalCounts["list"])
	assert.Equal(t, int64(2), handler.summary.resourceBuckets["apiserver.latency.k8s.io/etcd"]["testResource"].buckets[1.0].totalCounts["list"])

	// we only default the total latency when the annotation is missing as we don't know what additional resources it is using
	assert.Equal(t, int64(4), handler.summary.resourceBuckets["apiserver.latency.k8s.io/total"]["testResource"].buckets[0.0].totalCounts["list"])
	assert.Equal(t, int64(3), handler.summary.resourceBuckets["apiserver.latency.k8s.io/etcd"]["testResource"].buckets[0.0].totalCounts["list"])
}
