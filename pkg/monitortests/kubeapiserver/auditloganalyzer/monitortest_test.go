package auditloganalyzer

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func TestEvaluateExcessiveAppliesUserBoundaries(t *testing.T) {
	const (
		namespace = "openshift-test"
		testName  = "users in ns/openshift-test must not produce too many applies"
	)
	grandfatherName := testName + " [grandfather]"

	tests := []struct {
		name                   string
		usersToApplies         map[string]int
		wantOriginalPasses     int
		wantOriginalFailures   int
		wantGrandfatherPasses  int
		wantGrandfatherFailure int
	}{
		{
			name:                 "zero offending users passes",
			usersToApplies:       map[string]int{"system:serviceaccount:openshift-test:user-a": 200},
			wantOriginalPasses:   1,
			wantOriginalFailures: 0,
		},
		{
			name:                   "many applies from one user flakes under grandfather name",
			usersToApplies:         map[string]int{"system:serviceaccount:openshift-test:user-a": 500},
			wantOriginalPasses:     1,
			wantOriginalFailures:   0,
			wantGrandfatherPasses:  1,
			wantGrandfatherFailure: 1,
		},
		{
			name: "two offending users hard fail under original name",
			usersToApplies: map[string]int{
				"system:serviceaccount:openshift-test:user-a": 201,
				"system:serviceaccount:openshift-test:user-b": 202,
			},
			wantOriginalPasses:   0,
			wantOriginalFailures: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &auditLogAnalyzer{
				excessiveApplyChecker: &excessiveApplies{
					namespacesToUserToNumberOfApplies: map[string]map[string]int{namespace: tt.usersToApplies},
					resourcesToNumberOfApplies:        map[string]numberOfAppliesWithExamples{},
				},
			}

			results := analyzer.evaluateExcessiveApplies([]string{namespace})
			assertJUnitState(t, results, testName, tt.wantOriginalPasses, tt.wantOriginalFailures)
			grandfatherOutput := assertJUnitState(t, results, grandfatherName, tt.wantGrandfatherPasses, tt.wantGrandfatherFailure)

			if tt.wantGrandfatherFailure == 1 {
				require.Equal(t,
					"user system:serviceaccount:openshift-test:user-a had 500 applies, check the audit log and operator log to figure out why\ndetails in audit log",
					grandfatherOutput,
				)
			}
			if tt.wantOriginalFailures == 1 {
				originalOutput := junitFailureOutput(results, testName)
				require.Contains(t, originalOutput, "user system:serviceaccount:openshift-test:user-a had 201 applies")
				require.Contains(t, originalOutput, "user system:serviceaccount:openshift-test:user-b had 202 applies")
				require.True(t, strings.HasSuffix(originalOutput, "details in audit log"))
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
	originalOutput := assertJUnitState(t, results, testName, 1, 1)
	require.Equal(t,
		fmt.Sprintf("user %s had 301 applies, check the audit log and operator log to figure out why\ndetails in audit log", exemptUsername),
		originalOutput,
	)
	grandfatherOutput := assertJUnitState(t, results, testName+" [grandfather]", 1, 1)
	require.Equal(t,
		fmt.Sprintf("user %s had 401 applies, check the audit log and operator log to figure out why\ndetails in audit log", newUsername),
		grandfatherOutput,
	)
}

func TestEvaluateExcessiveAppliesResourceBoundaries(t *testing.T) {
	const testName = `[Jira:"kube-apiserver"] API resources are not updated excessively`

	tests := []struct {
		name         string
		resources    map[string]numberOfAppliesWithExamples
		wantPasses   int
		wantFailures int
	}{
		{
			name: "zero excessive resources passes",
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
		},
		{
			name: "two excessive resources retain flake behavior",
			resources: map[string]numberOfAppliesWithExamples{
				"deployments/example-a": newResourceApplies(200, "user-a"),
				"deployments/example-b": newResourceApplies(201, "user-b"),
			},
			wantPasses:   1,
			wantFailures: 1,
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
			if tt.wantFailures == 1 {
				require.Contains(t, output, "details in audit log")
				for resource := range tt.resources {
					if tt.resources[resource].numberOfApplies >= 200 {
						require.Contains(t, output, fmt.Sprintf("resource %s had", resource))
					}
				}
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
	grandfatherName := testName + " [grandfather]"

	tests := []struct {
		name                   string
		users                  map[string]*invalidRequestDetails
		wantOriginalPasses     int
		wantOriginalFailures   int
		wantGrandfatherPasses  int
		wantGrandfatherFailure int
	}{
		{
			name:                 "zero offending users passes",
			users:                map[string]*invalidRequestDetails{},
			wantOriginalPasses:   1,
			wantOriginalFailures: 0,
		},
		{
			name: "many invalid requests from one user flake under grandfather name",
			users: map[string]*invalidRequestDetails{
				"system:serviceaccount:openshift-test:user-a": newInvalidRequestDetails(27, "a"),
			},
			wantOriginalPasses:     1,
			wantOriginalFailures:   0,
			wantGrandfatherPasses:  1,
			wantGrandfatherFailure: 1,
		},
		{
			name: "two offending users hard fail under original name",
			users: map[string]*invalidRequestDetails{
				"system:serviceaccount:openshift-test:user-a": newInvalidRequestDetails(1, "a"),
				"system:serviceaccount:openshift-test:user-b": newInvalidRequestDetails(2, "b"),
			},
			wantOriginalPasses:   0,
			wantOriginalFailures: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := newInvalidRequestAnalyzer(verb, namespace, tt.users)

			results := analyzer.evaluateInvalidRequests([]string{namespace})
			originalOutput := assertJUnitState(t, results, testName, tt.wantOriginalPasses, tt.wantOriginalFailures)
			grandfatherOutput := assertJUnitState(t, results, grandfatherName, tt.wantGrandfatherPasses, tt.wantGrandfatherFailure)

			if tt.wantGrandfatherFailure == 1 {
				require.Contains(t, grandfatherOutput, "user system:serviceaccount:openshift-test:user-a had 27 invalid \"update\"")
				require.Contains(t, grandfatherOutput, "request=/apis/example/a-first auditID=a-first")
				require.Contains(t, grandfatherOutput, "request=/apis/example/a-last auditID=a-last")
				require.True(t, strings.HasSuffix(grandfatherOutput, "more details in audit log"))
			}
			if tt.wantOriginalFailures == 1 {
				require.Contains(t, originalOutput, "system:serviceaccount:openshift-test:user-a")
				require.Contains(t, originalOutput, "system:serviceaccount:openshift-test:user-b")
				require.True(t, strings.HasSuffix(originalOutput, "more details in audit log"))
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
			name:          "machine config daemon tech preview invalid applies",
			verb:          "apply",
			namespace:     "openshift-machine-config-operator",
			username:      "system:serviceaccount:openshift-machine-config-operator:machine-config-daemon",
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

func junitFailureOutput(results []*junitapi.JUnitTestCase, testName string) string {
	for _, result := range results {
		if result.Name == testName && result.FailureOutput != nil {
			return result.FailureOutput.Output
		}
	}
	return ""
}

func junitNames(results []*junitapi.JUnitTestCase) string {
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Name)
	}
	return strings.Join(names, "\n")
}
