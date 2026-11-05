package auditloganalyzer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func TestEvaluateAPILoadBalancerShutdownSkipped(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{name: "ROSA", reason: "not applicable to ROSA"},
		{name: "unknown cluster type", reason: "cluster type could not be determined"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analyzer := &auditLogAnalyzer{
				apiLoadBalancerSkipReason:     test.reason,
				requestsDuringShutdownChecker: CheckForRequestsDuringShutdown(),
			}

			results := analyzer.evaluateAPILoadBalancerShutdown(monitorapi.Intervals{})
			require.Len(t, results, 1)
			require.NotNil(t, results[0].SkipMessage)
			require.Equal(t, test.reason, results[0].SkipMessage.Message)
			require.Nil(t, results[0].FailureOutput)
		})
	}
}

func TestEvaluateExcessiveAppliesUserBoundaries(t *testing.T) {
	const (
		namespace = "openshift-test"
		testName  = "users in ns/openshift-test must not produce too many applies"
	)

	tests := []struct {
		name         string
		users        map[string]int
		wantFailures int
		wantOutput   string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:  "zero offending users passes",
			users: map[string]int{"system:serviceaccount:openshift-test:user-a": 200},
		},
		{
			name:         "many applies from one user flakes under original name",
			users:        map[string]int{"system:serviceaccount:openshift-test:user-a": 500},
			wantFailures: 1,
			wantOutput:   "one user had 500 applies\ndetails in audit log",
			wantAbsent:   []string{"system:serviceaccount:openshift-test:user-a"},
		},
		{
			name: "two offending users flake under original name",
			users: map[string]int{
				"system:serviceaccount:openshift-test:user-a": 201,
				"system:serviceaccount:openshift-test:user-b": 202,
			},
			wantFailures: 1,
			wantContains: []string{
				"user system:serviceaccount:openshift-test:user-a had 201 applies",
				"user system:serviceaccount:openshift-test:user-b had 202 applies",
			},
		},
		{
			name: "many offending users flake under original name",
			users: map[string]int{
				"system:serviceaccount:openshift-test:user-a": 201,
				"system:serviceaccount:openshift-test:user-b": 202,
				"system:serviceaccount:openshift-test:user-c": 203,
				"system:serviceaccount:openshift-test:user-d": 204,
			},
			wantFailures: 1,
			wantContains: []string{
				"user system:serviceaccount:openshift-test:user-a had 201 applies",
				"user system:serviceaccount:openshift-test:user-d had 204 applies",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &auditLogAnalyzer{
				excessiveApplyChecker: &excessiveApplies{
					namespacesToUserToNumberOfApplies: map[string]map[string]int{namespace: tt.users},
					resourcesToNumberOfApplies:        map[string]numberOfAppliesWithExamples{},
				},
			}

			results := analyzer.evaluateExcessiveApplies([]string{namespace})
			output := assertJUnitState(t, results, testName, 1, tt.wantFailures)
			require.NotContains(t, junitNames(results), "grandfather")
			require.Zero(t, downstreamRealFailures(results))
			if tt.wantOutput != "" {
				require.Equal(t, tt.wantOutput, output)
			}
			for _, expected := range tt.wantContains {
				require.Contains(t, output, expected)
			}
			for _, absent := range tt.wantAbsent {
				require.NotContains(t, output, absent)
			}
			if tt.wantFailures > 0 {
				require.True(t, strings.HasSuffix(output, "details in audit log"))
			}
		})
	}
}

func TestEvaluateExcessiveAppliesPreservesExistingFlakes(t *testing.T) {
	const (
		namespace      = "openshift-infra"
		testName       = "users in ns/openshift-infra must not produce too many applies"
		exemptUsername = "system:serviceaccount:openshift-infra:serviceaccount-pull-secrets-controller"
		newUsername    = "system:serviceaccount:openshift-infra:new-controller"
	)

	analyzer := &auditLogAnalyzer{
		excessiveApplyChecker: &excessiveApplies{
			namespacesToUserToNumberOfApplies: map[string]map[string]int{
				namespace: {
					exemptUsername: 301,
					newUsername:    401,
				},
			},
			resourcesToNumberOfApplies: map[string]numberOfAppliesWithExamples{},
		},
	}

	results := analyzer.evaluateExcessiveApplies([]string{namespace})
	output := assertJUnitState(t, results, testName, 1, 1)
	require.Equal(t, "one user had 401 applies\none exempt user had 301 applies\ndetails in audit log", output)
	require.NotContains(t, output, exemptUsername)
	require.NotContains(t, output, newUsername)
	require.NotContains(t, junitNames(results), "grandfather")
	require.Zero(t, downstreamRealFailures(results))
}

func TestEvaluateExcessiveAppliesPreservesDetailedMultiUserFlakes(t *testing.T) {
	const (
		namespace       = "openshift-infra"
		testName        = "users in ns/openshift-infra must not produce too many applies"
		exemptUsernameA = "system:serviceaccount:openshift-infra:serviceaccount-pull-secrets-controller"
		exemptUsernameB = "system:serviceaccount:openshift-infra:podsecurity-admission-label-syncer-controller"
	)

	analyzer := &auditLogAnalyzer{
		excessiveApplyChecker: &excessiveApplies{
			namespacesToUserToNumberOfApplies: map[string]map[string]int{
				namespace: {
					exemptUsernameA: 301,
					exemptUsernameB: 302,
				},
			},
			resourcesToNumberOfApplies: map[string]numberOfAppliesWithExamples{},
		},
	}

	results := analyzer.evaluateExcessiveApplies([]string{namespace})
	output := assertJUnitState(t, results, testName, 1, 1)
	require.Contains(t, output, fmt.Sprintf("user %s had 301 applies", exemptUsernameA))
	require.Contains(t, output, fmt.Sprintf("user %s had 302 applies", exemptUsernameB))
	require.True(t, strings.HasSuffix(output, "details in audit log"))
	require.NotContains(t, junitNames(results), "grandfather")
	require.Zero(t, downstreamRealFailures(results))
}

func TestEvaluateExcessiveAppliesResourceBoundaries(t *testing.T) {
	const testName = `[Jira:"kube-apiserver"] API resources are not updated excessively`

	tests := []struct {
		name         string
		resources    map[string]numberOfAppliesWithExamples
		wantPasses   int
		wantFailures int
		wantOutput   string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:       "empty resource data passes",
			resources:  map[string]numberOfAppliesWithExamples{},
			wantPasses: 1,
		},
		{
			name: "below threshold resource passes",
			resources: map[string]numberOfAppliesWithExamples{
				"deployments/example": newResourceApplies(199, "user-a"),
			},
			wantPasses: 1,
		},
		{
			name: "one excessive resource flakes",
			resources: map[string]numberOfAppliesWithExamples{
				"deployments/example": newResourceApplies(200, "user-a"),
			},
			wantPasses:   1,
			wantFailures: 1,
			wantOutput:   "one resource had 200 applies\ndetails in audit log",
			wantAbsent: []string{
				"deployments/example",
				"user-a",
				"first-audit-id",
				"last-audit-id",
				"2026-01-02",
			},
		},
		{
			name: "two excessive resources retain flake behavior",
			resources: map[string]numberOfAppliesWithExamples{
				"deployments/example-a": newResourceApplies(200, "user-a"),
				"deployments/example-b": newResourceApplies(201, "user-b"),
			},
			wantPasses:   1,
			wantFailures: 1,
			wantContains: []string{
				"resource deployments/example-a had 200 applies",
				"resource deployments/example-b had 201 applies",
				"user-a",
				"user-b",
				"first-audit-id",
				"last-audit-id",
				"2026-01-02",
			},
		},
		{
			name: "many excessive resources retain flake behavior",
			resources: map[string]numberOfAppliesWithExamples{
				"deployments/example-a": newResourceApplies(200, "user-a"),
				"deployments/example-b": newResourceApplies(201, "user-b"),
				"deployments/example-c": newResourceApplies(202, "user-c"),
				"deployments/example-d": newResourceApplies(203, "user-d"),
			},
			wantPasses:   1,
			wantFailures: 1,
			wantContains: []string{
				"resource deployments/example-a had 200 applies",
				"resource deployments/example-d had 203 applies",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &auditLogAnalyzer{
				excessiveApplyChecker: &excessiveApplies{
					namespacesToUserToNumberOfApplies: map[string]map[string]int{},
					resourcesToNumberOfApplies:        tt.resources,
				},
			}

			results := analyzer.evaluateExcessiveApplies(nil)
			output := assertJUnitState(t, results, testName, tt.wantPasses, tt.wantFailures)
			require.NotContains(t, junitNames(results), "grandfather")
			require.Zero(t, downstreamRealFailures(results))
			if tt.wantOutput != "" {
				require.Equal(t, tt.wantOutput, output)
			}
			for _, expected := range tt.wantContains {
				require.Contains(t, output, expected)
			}
			for _, absent := range tt.wantAbsent {
				require.NotContains(t, output, absent)
			}
		})
	}
}

func TestEvaluateInvalidRequestsUserBoundaries(t *testing.T) {
	const (
		namespace = "openshift-test"
		verb      = "update"
		testName  = "users in ns/openshift-test must not produce too many invalid \"update\" requests"
	)
	tests := []struct {
		name         string
		users        map[string]*invalidRequestDetails
		wantFailures int
		wantOutput   string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:  "zero offending users passes",
			users: map[string]*invalidRequestDetails{},
		},
		{
			name: "many invalid requests from one user flake under original name",
			users: map[string]*invalidRequestDetails{
				"system:serviceaccount:openshift-test:user-a": newInvalidRequestDetails(27, "a"),
			},
			wantFailures: 1,
			wantOutput:   "one user had 27 invalid \"update\" requests\nmore details in audit log",
			wantAbsent: []string{
				"system:serviceaccount:openshift-test:user-a",
				"/apis/example/a-first",
				"/apis/example/a-last",
				"a-first",
				"a-last",
				"2026-01-02",
			},
		},
		{
			name: "two offending users flake under original name",
			users: map[string]*invalidRequestDetails{
				"system:serviceaccount:openshift-test:user-a": newInvalidRequestDetails(1, "a"),
				"system:serviceaccount:openshift-test:user-b": newInvalidRequestDetails(2, "b"),
			},
			wantFailures: 1,
			wantContains: []string{
				"system:serviceaccount:openshift-test:user-a",
				"system:serviceaccount:openshift-test:user-b",
				"request=/apis/example/a-first auditID=a-first",
				"request=/apis/example/b-last auditID=b-last",
			},
		},
		{
			name: "many offending users flake under original name",
			users: map[string]*invalidRequestDetails{
				"system:serviceaccount:openshift-test:user-a": newInvalidRequestDetails(1, "a"),
				"system:serviceaccount:openshift-test:user-b": newInvalidRequestDetails(2, "b"),
				"system:serviceaccount:openshift-test:user-c": newInvalidRequestDetails(3, "c"),
				"system:serviceaccount:openshift-test:user-d": newInvalidRequestDetails(4, "d"),
			},
			wantFailures: 1,
			wantContains: []string{
				"system:serviceaccount:openshift-test:user-a",
				"system:serviceaccount:openshift-test:user-d",
				"request=/apis/example/a-first auditID=a-first",
				"request=/apis/example/d-last auditID=d-last",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := newInvalidRequestAnalyzer(verb, namespace, tt.users)

			results := analyzer.evaluateInvalidRequests([]string{namespace})
			output := assertJUnitState(t, results, testName, 1, tt.wantFailures)
			require.NotContains(t, junitNames(results), "grandfather")
			require.Zero(t, downstreamRealFailures(results))
			if tt.wantOutput != "" {
				require.Equal(t, tt.wantOutput, output)
			}
			for _, expected := range tt.wantContains {
				require.Contains(t, output, expected)
			}
			for _, absent := range tt.wantAbsent {
				require.NotContains(t, output, absent)
			}
			if tt.wantFailures > 0 {
				require.True(t, strings.HasSuffix(output, "more details in audit log"))
			}
		})
	}
}

func TestEvaluateInvalidRequestsPreservesExemptions(t *testing.T) {
	tests := []struct {
		name          string
		verb          string
		namespace     string
		username      string
		isTechPreview bool
	}{
		{
			name:      "build config change controller invalid creates",
			verb:      "create",
			namespace: "openshift-infra",
			username:  "system:serviceaccount:openshift-infra:build-config-change-controller",
		},
		{
			name:      "template instance controller invalid creates",
			verb:      "create",
			namespace: "openshift-infra",
			username:  "system:serviceaccount:openshift-infra:template-instance-controller",
		},
		{
			name:          "machine config daemon tech preview invalid applies",
			verb:          "apply",
			namespace:     "openshift-machine-config-operator",
			username:      "system:serviceaccount:openshift-machine-config-operator:machine-config-daemon",
			isTechPreview: true,
		},
		{
			name:          "machine config operator tech preview invalid applies",
			verb:          "apply",
			namespace:     "openshift-machine-config-operator",
			username:      "system:serviceaccount:openshift-machine-config-operator:machine-config-operator",
			isTechPreview: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := newInvalidRequestAnalyzer(tt.verb, tt.namespace, map[string]*invalidRequestDetails{
				tt.username: newInvalidRequestDetails(10, "exempt"),
			})
			analyzer.isTechPreview = tt.isTechPreview

			results := analyzer.evaluateInvalidRequests([]string{tt.namespace})
			testName := fmt.Sprintf("users in ns/%s must not produce too many invalid %q requests", tt.namespace, tt.verb)
			assertJUnitState(t, results, testName, 1, 0)
			require.NotContains(t, junitNames(results), "grandfather")
			require.Zero(t, downstreamRealFailures(results))
		})
	}
}

func newResourceApplies(count int, username string) numberOfAppliesWithExamples {
	return numberOfAppliesWithExamples{
		numberOfApplies:   count,
		users:             map[string]int{username: count},
		firstAuditEventID: "first-audit-id",
		lastAuditEventID:  "last-audit-id",
		firstOccurredAt:   metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
		lastOccurredAt:    metav1.NewTime(time.Date(2026, 1, 2, 3, 5, 5, 0, time.UTC)),
	}
}

func newInvalidRequestDetails(count int, idPrefix string) *invalidRequestDetails {
	firstTimestamp := metav1.NewMicroTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	lastTimestamp := metav1.NewMicroTime(time.Date(2026, 1, 2, 3, 5, 5, 0, time.UTC))
	return &invalidRequestDetails{
		totalNumberOfFailures: count,
		first10Failures: []auditv1.Event{
			{
				AuditID:                  types.UID(idPrefix + "-first"),
				RequestURI:               "/apis/example/" + idPrefix + "-first",
				RequestReceivedTimestamp: firstTimestamp,
			},
		},
		last10Failures: []auditv1.Event{
			{
				AuditID:                  types.UID(idPrefix + "-last"),
				RequestURI:               "/apis/example/" + idPrefix + "-last",
				RequestReceivedTimestamp: lastTimestamp,
			},
		},
	}
}

func newInvalidRequestAnalyzer(verb, namespace string, users map[string]*invalidRequestDetails) *auditLogAnalyzer {
	return &auditLogAnalyzer{
		invalidRequestsChecker: &invalidRequests{
			verbToNamespacesTouserToNumberOf422s: map[string]*namespaceInvalidRequestTracker{
				verb: {
					namespacesToInvalidUserTrackers: map[string]*userInvalidRequestTracker{
						namespace: {usersToInvalidRequests: users},
					},
				},
			},
		},
	}
}

func assertJUnitState(t *testing.T, results []*junitapi.JUnitTestCase, testName string, wantPasses, wantFailures int) string {
	t.Helper()
	passes := 0
	failures := 0
	failureOutput := ""
	for _, result := range results {
		if result.Name != testName {
			continue
		}
		if result.FailureOutput == nil {
			passes++
			continue
		}
		failures++
		failureOutput = result.FailureOutput.Output
	}
	require.Equal(t, wantPasses, passes, "pass count for %q", testName)
	require.Equal(t, wantFailures, failures, "failure count for %q", testName)
	return failureOutput
}

func junitNames(results []*junitapi.JUnitTestCase) string {
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Name)
	}
	return strings.Join(names, "\n")
}

// downstreamRealFailures mirrors the suite runner's same-name pass/failure flake recognition.
func downstreamRealFailures(results []*junitapi.JUnitTestCase) int {
	failures := map[string]struct{}{}
	passes := map[string]struct{}{}
	for _, result := range results {
		if result.FailureOutput != nil {
			failures[result.Name] = struct{}{}
		} else if result.SkipMessage == nil {
			passes[result.Name] = struct{}{}
		}
	}

	realFailures := 0
	for name := range failures {
		if _, ok := passes[name]; !ok {
			realFailures++
		}
	}
	return realFailures
}
