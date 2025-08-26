package auditloganalyzer

import (
	"errors"
	"testing"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/stretchr/testify/assert"
	authnv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func Test_WatchCountTrackerEventHandler(t *testing.T) {
	info := monitortestframework.MonitorTestInitializationInfo{
		ClusterStabilityDuringTest: monitortestframework.Stable,
	}
	handler := NewWatchCountTracking(info)
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

func Test_boundsChecker(t *testing.T) {
	bounds := map[string]int64{
		"test-operator": 100,
	}

	operatorName := "system:serviceaccount:test-namespace:test-operator"

	type testcase struct {
		name         string
		stability    monitortestframework.ClusterStabilityDuringTest
		requestCount *RequestCount
		err          error
	}

	testcases := []testcase{
		{
			name:      "stable cluster, requests does not exceed limit, no error",
			stability: monitortestframework.Stable,
			requestCount: &RequestCount{
				Count:    150,
				Operator: operatorName,
			},
			err: nil,
		},
		{
			name:      "stable cluster, requests does exceed limit, error",
			stability: monitortestframework.Stable,
			requestCount: &RequestCount{
				Count:    250,
				Operator: operatorName,
			},
			err: errors.New("watchrequestcount=250, upperbound=200, ratio=1.25"),
		},
		{
			name:      "disruptive cluster, requests does not exceed base limit, no error",
			stability: monitortestframework.Disruptive,
			requestCount: &RequestCount{
				Count:    150,
				Operator: operatorName,
			},
			err: nil,
		},
		{
			name:      "disruptive cluster, requests does exceed base limit but falls within additional grace buffer, no error",
			stability: monitortestframework.Disruptive,
			requestCount: &RequestCount{
				Count:    215,
				Operator: operatorName,
			},
			err: nil,
		},
		{
			name:      "disruptive cluster, requests does exceed limits with additional grace buffer, error",
			stability: monitortestframework.Disruptive,
			requestCount: &RequestCount{
				Count:    300,
				Operator: operatorName,
			},
			err: errors.New("watchrequestcount=300, upperbound=220, ratio=1.36"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			checker := boundsChecker{
				Bounds: bounds,
				// Platform doesn't matter for testing purposes, just set it to an arbitrary one.
				Platform:         v1.AWSPlatformType,
				ClusterStability: tc.stability,
				UpperBoundMultiple: upperBoundMultiple{
					Base:                  2.0,
					DisruptiveGraceFactor: 0.2,
				},
			}

			err := checker.ensureBoundNotExceeded(tc.requestCount)

			switch {
			case err != nil && tc.err != nil && err.Error() != tc.err.Error():
				t.Fatalf("received error %v does not match expected error %v", err, tc.err)
			case err != nil && tc.err == nil:
				t.Fatalf("received error %v when no error was expected", err)
			case err == nil && tc.err != nil:
				t.Fatalf("no error received when error %v was expected", tc.err)
			}
		})
	}
}
