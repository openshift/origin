package auditloganalyzer

import (
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authnv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
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

func Test_loadOperatorWatchLimits(t *testing.T) {
	limits, err := loadOperatorWatchLimits()
	require.NoError(t, err, "embedded operator_watch_limits.json should unmarshal without error")
	require.NotNil(t, limits)

	// Verify HighlyAvailable topology exists and has platforms
	haLimits, ok := limits[configv1.HighlyAvailableTopologyMode]
	require.True(t, ok, "HighlyAvailable topology should exist")
	require.NotEmpty(t, haLimits, "HighlyAvailable should have platform entries")

	// Verify at least one platform has operator limits
	awsLimits, ok := haLimits[configv1.AWSPlatformType]
	require.True(t, ok, "AWS platform should exist in HighlyAvailable")
	require.NotEmpty(t, awsLimits, "AWS should have operator limits")

	// Verify a known operator has a limit
	_, ok = awsLimits["ingress-operator"]
	assert.True(t, ok, "ingress-operator should have a limit defined")
}
