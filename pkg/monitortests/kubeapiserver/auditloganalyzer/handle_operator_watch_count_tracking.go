package auditloganalyzer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

//go:embed operator_watch_limits.json
var operatorWatchLimitsJSON []byte

// platformUpperBound maps operator service account names to their upper bound limits
type platformUpperBound map[string]int64

// loadOperatorWatchLimits loads and parses the embedded operator_watch_limits.json file
func loadOperatorWatchLimits() (map[configv1.TopologyMode]map[configv1.PlatformType]platformUpperBound, error) {
	var limits map[string]map[string]map[string]float64
	if err := json.Unmarshal(operatorWatchLimitsJSON, &limits); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operator watch limits: %w", err)
	}

	result := make(map[configv1.TopologyMode]map[configv1.PlatformType]platformUpperBound)

	// Map string topology names to TopologyMode constants
	topologyMapping := map[string]configv1.TopologyMode{
		"HighlyAvailable": configv1.HighlyAvailableTopologyMode,
		"SingleReplica":   configv1.SingleReplicaTopologyMode,
	}

	// Map string platform names to PlatformType constants
	platformMapping := map[string]configv1.PlatformType{
		"AWS":       configv1.AWSPlatformType,
		"Azure":     configv1.AzurePlatformType,
		"GCP":       configv1.GCPPlatformType,
		"BareMetal": configv1.BareMetalPlatformType,
		"vSphere":   configv1.VSpherePlatformType,
		"OpenStack": configv1.OpenStackPlatformType,
	}

	for topologyStr, platforms := range limits {
		topology, ok := topologyMapping[topologyStr]
		if !ok {
			continue
		}

		if result[topology] == nil {
			result[topology] = make(map[configv1.PlatformType]platformUpperBound)
		}

		for platformStr, operators := range platforms {
			platform, ok := platformMapping[platformStr]
			if !ok {
				continue
			}

			bounds := make(platformUpperBound)
			for operator, limit := range operators {
				bounds[operator] = int64(limit)
			}

			result[topology][platform] = bounds
		}
	}

	return result, nil
}

// with https://github.com/openshift/kubernetes/pull/2113 we no longer have the counts used in
// https://github.com/openshift/kubernetes/pull/2113 we no longer have the counts used in
// https://github.com/openshift/origin/blob/35b3c221634bcaef9a5148477334c27d166b7838/pkg/monitortests/testframework/watchrequestcountscollector/monitortest.go#L111
// attempt to recreate via audit events

type watchCountTracking struct {
	lock                  sync.Mutex
	startTime             time.Time
	watchRequestCountsMap map[OperatorKey]*RequestCount
}

func NewWatchCountTracking() *watchCountTracking {
	return &watchCountTracking{startTime: time.Now(), watchRequestCountsMap: map[OperatorKey]*RequestCount{}}
}

type OperatorKey struct {
	NodeName string
	Operator string
	Hour     int
}

type RequestCount struct {
	Count    int64
	Operator string
	NodeName string
	Hour     int
}

func (s *watchCountTracking) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	// only want verb watch and response complete events
	if auditEvent.Verb != "watch" || auditEvent.Stage != auditv1.StageResponseComplete || !strings.HasSuffix(auditEvent.User.Username, "-operator") {
		return
	}

	requestHour := int(auditEvent.RequestReceivedTimestamp.Sub(s.startTime).Round(time.Hour))
	key := OperatorKey{Hour: requestHour, Operator: auditEvent.User.Username, NodeName: nodeName}

	var counter *RequestCount
	var ok bool
	if counter, ok = s.watchRequestCountsMap[key]; !ok {
		counter = &RequestCount{NodeName: key.NodeName, Hour: key.Hour, Operator: key.Operator, Count: 0}
		s.watchRequestCountsMap[key] = counter
	}
	counter.Count++

}

func (s *watchCountTracking) SummarizeWatchCountRequests() []*RequestCount {
	// take maximum from all hours through all nodes
	watchRequestCountsMapMax := map[OperatorKey]*RequestCount{}
	for _, requestCount := range s.watchRequestCountsMap {
		key := OperatorKey{
			Operator: requestCount.Operator,
		}
		if _, exists := watchRequestCountsMapMax[key]; exists {
			if watchRequestCountsMapMax[key].Count < requestCount.Count {
				watchRequestCountsMapMax[key].Count = requestCount.Count
				watchRequestCountsMapMax[key].NodeName = requestCount.NodeName
				watchRequestCountsMapMax[key].Hour = requestCount.Hour
			}
		} else {
			watchRequestCountsMapMax[key] = requestCount
		}
	}

	// sort the requests counts so it's easy to see the biggest offenders
	watchRequestCounts := []*RequestCount{}
	for _, requestCount := range watchRequestCountsMapMax {
		watchRequestCounts = append(watchRequestCounts, requestCount)
	}

	sort.Slice(watchRequestCounts, func(i int, j int) bool {
		return watchRequestCounts[i].Count > watchRequestCounts[j].Count
	})

	return watchRequestCounts
}

func (s *watchCountTracking) WriteAuditLogSummary(ctx context.Context, artifactDir, name, timeSuffix string) error {
	oc := exutil.NewCLIWithoutNamespace(name)

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		logrus.WithError(err).Warn("unable to get cluster infrastructure")
		return nil
	}

	watchRequestCounts := s.SummarizeWatchCountRequests()

	// infra.Status.ControlPlaneTopology, infra.Spec.PlatformSpec.Type, operator, value
	rows := make([]map[string]string, 0)
	for _, item := range watchRequestCounts {
		operator := strings.Split(item.Operator, ":")[3]
		rows = append(rows, map[string]string{"ControlPlaneTopology": string(infra.Status.ControlPlaneTopology), "PlatformType": string(infra.Spec.PlatformSpec.Type), "Operator": operator, "WatchRequestCount": strconv.FormatInt(item.Count, 10)})
	}

	dataFile := dataloader.DataFile{
		TableName: "operator_watch_requests",
		Schema:    map[string]dataloader.DataType{"ControlPlaneTopology": dataloader.DataTypeString, "PlatformType": dataloader.DataTypeString, "Operator": dataloader.DataTypeString, "WatchRequestCount": dataloader.DataTypeInteger},
		Rows:      rows,
	}
	fileName := filepath.Join(artifactDir, fmt.Sprintf("operator-%s%s-%s", name, timeSuffix, dataloader.AutoDataLoaderSuffix))
	err = dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
		return nil
	}

	return nil
}

// serviceAccountNameToOperatorName converts a service account name from audit logs to the ClusterOperator name.
// Service account names come from watch requests in audit logs like:
//   - "marketplace-operator" -> "marketplace"
//   - "cluster-baremetal-operator" -> "baremetal"
//   - "cluster-monitoring-operator" -> "monitoring"
//   - "cluster-autoscaler-operator" -> "cluster-autoscaler" (exception: this operator keeps cluster- prefix)
//
// This is needed because the upperBounds map keys are service account names (what appears in audit logs),
// but we need to map to ClusterOperator names for component mapping and display.
func serviceAccountNameToOperatorName(serviceAccountName string) string {
	// Check override map for service accounts that don't cleanly map to operator names
	if override, ok := serviceAccountToOperatorNameOverrides[serviceAccountName]; ok {
		return override
	}

	// First strip -operator suffix
	name := strings.TrimSuffix(serviceAccountName, "-operator")

	// Strip the cluster- prefix if present.
	// This handles service accounts like "cluster-baremetal-operator" -> "baremetal"
	name = strings.TrimPrefix(name, "cluster-")

	return name
}

// serviceAccountToJiraComponent maps service account names that don't automatically map to operator names.
// From operator name we can get to jira component in operator_mapping.go.
var serviceAccountToOperatorNameOverrides = map[string]string{
	"prometheus-operator":               "monitoring",
	"cluster-samples-operator":          "openshift-samples",
	"openshift-kube-scheduler-operator": "kube-scheduler",
	"openshift-config-operator":         "config-operator",
	"cluster-autoscaler-operator":       "cluster-autoscaler",
	"capi-operator":                     "machine-api", // probably need a new operator defined in operator_mapping
	"cluster-capi-operator":             "machine-api", // probably need a new operator defined in operator_mapping
	"gcp-pd-csi-driver-operator":        "storage",
}

// getJiraComponentForOperator returns a JIRA component name for a service account name.
func getJiraComponentForOperator(serviceAccountName string) string {
	operatorName := serviceAccountNameToOperatorName(serviceAccountName)
	component := platformidentification.GetBugzillaComponentForOperator(operatorName)
	if component == "Unknown" {
		// Return a generic component if not mapped
		return "Test Framework"
	}
	return component
}

// makeTestName creates the test name with JIRA component for a service account name
func makeTestName(serviceAccountName string) string {
	component := getJiraComponentForOperator(serviceAccountName)
	return fmt.Sprintf("[Jira:%q] operator service account %s should not create excessive watch requests", component, serviceAccountName)
}

func (s *watchCountTracking) CreateJunits() ([]*junitapi.JUnitTestCase, error) {
	ret := []*junitapi.JUnitTestCase{}

	testMinRequestsName := "[Jira:\"Test Framework\"] operators should have watch channel requests"
	oc := exutil.NewCLIWithoutNamespace("operator-watch")
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		// If we can't get infrastructure info, return a single error test
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: "[Jira:\"Test Framework\"] operator watch request tracking infrastructure check",
			FailureOutput: &junitapi.FailureOutput{
				Message: fmt.Sprintf("Failed to get cluster infrastructure: %v", err),
				Output:  err.Error(),
			},
		})
		return ret, nil
	}

	// Load operator watch limits from JSON file
	// See https://issues.redhat.com/browse/WRKLDS-291 for upper bounds computation
	//
	// These values need to be periodically incremented as code evolves, the stated goal of the test is to prevent
	// exponential growth. Original methodology for calculating the values is difficult, iterations since have just
	// been done manually via search.ci. (i.e. https://search.ci.openshift.org/?search=produces+more+watch+requests+than+expected&maxAge=336h&context=0&type=bug%2Bjunit&name=&excludeName=&maxMatches=5&maxBytes=20971520&groupBy=job )
	//
	// IMPORTANT: The keys in the JSON are SERVICE ACCOUNT NAMES from audit log watch requests, NOT ClusterOperator names.
	// Service account names come from audit logs like: system:serviceaccount:openshift-marketplace:marketplace-operator
	// Examples:
	//   - "marketplace-operator" (service account) -> maps to "marketplace" ClusterOperator
	//   - "cluster-baremetal-operator" (service account) -> maps to "baremetal" ClusterOperator
	//   - "cluster-monitoring-operator" (service account) -> maps to "monitoring" ClusterOperator
	//   - "cluster-autoscaler-operator" (service account) -> maps to "cluster-autoscaler" ClusterOperator (exception)
	allLimits, err := loadOperatorWatchLimits()
	if err != nil {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: "[Jira:\"Test Framework\"] operator watch request tracking limits load",
			FailureOutput: &junitapi.FailureOutput{
				Message: fmt.Sprintf("Failed to load operator watch limits: %v", err),
				Output:  err.Error(),
			},
		})
		return ret, nil
	}

	// Select the appropriate limits based on topology and platform
	var upperBound platformUpperBound

	switch infra.Status.ControlPlaneTopology {
	case configv1.ExternalTopologyMode:
		// For unsupported topology, return a single skip test
		ret = append(ret, &junitapi.JUnitTestCase{
			Name:        "[Jira:\"Test Framework\"] operator watch request tracking topology check",
			SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported topology: %v", infra.Status.ControlPlaneTopology)},
		})
		return ret, nil

	case configv1.SingleReplicaTopologyMode:
		topologyLimits, exists := allLimits[configv1.SingleReplicaTopologyMode]
		if !exists {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name:        "[Jira:\"Test Framework\"] operator watch request tracking topology check",
				SkipMessage: &junitapi.SkipMessage{Message: "SingleReplica topology not found in limits file"},
			})
			return ret, nil
		}

		platformLimits, exists := topologyLimits[infra.Spec.PlatformSpec.Type]
		if !exists {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name:        "[Jira:\"Test Framework\"] operator watch request tracking platform check",
				SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported single node platform type: %v", infra.Spec.PlatformSpec.Type)},
			})
			return ret, nil
		}
		upperBound = platformLimits

	default:
		topologyLimits, exists := allLimits[configv1.HighlyAvailableTopologyMode]
		if !exists {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name:        "[Jira:\"Test Framework\"] operator watch request tracking topology check",
				SkipMessage: &junitapi.SkipMessage{Message: "HighlyAvailable topology not found in limits file"},
			})
			return ret, nil
		}

		platformLimits, exists := topologyLimits[infra.Spec.PlatformSpec.Type]
		if !exists {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name:        "[Jira:\"Test Framework\"] operator watch request tracking platform check",
				SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported platform type: %v", infra.Spec.PlatformSpec.Type)},
			})
			return ret, nil
		}
		upperBound = platformLimits
	}

	watchRequestCounts := s.SummarizeWatchCountRequests()

	// Sanity check: ensure we have at least some watch request data
	if len(watchRequestCounts) == 0 {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testMinRequestsName,
			FailureOutput: &junitapi.FailureOutput{
				Message: "Expected at least one watch request count to be present. This indicates the audit log analysis may not be working correctly.",
			},
		})
		// Return early - without watch data we can't run per-operator tests
		return ret, nil
	}

	// If we have watch data, add a passing test
	ret = append(ret, &junitapi.JUnitTestCase{
		Name: testMinRequestsName,
	})

	// Create a map of operator -> watch count for easy lookup
	watchCountByOperator := make(map[string]*RequestCount)
	for _, item := range watchRequestCounts {
		operator := strings.Split(item.Operator, ":")[3]
		watchCountByOperator[operator] = item
	}

	// Create one test case per operator in the upper bounds
	for operator, allowedCount := range upperBound {
		testName := makeTestName(operator)

		item, hasWatchData := watchCountByOperator[operator]

		if !hasWatchData {
			// Operator has no watch data - this might be normal for some platforms
			framework.Logf("Operator %v not found in watch request data", operator)
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})
			continue
		}

		// The upper bound are measured from CI runs where the tests might be running less than 2h in total.
		// In the worst case half of the requests will be put into each bucket. Thus, multiply the bound by 2
		allowedCount = allowedCount * 2
		ratio := float64(item.Count) / float64(allowedCount)
		ratio = math.Round(ratio*100) / 100
		framework.Logf("operator=%v, watchrequestcount=%v, upperbound=%v, ratio=%v", operator, item.Count, allowedCount, ratio)

		if item.Count > allowedCount {
			framework.Logf("Operator %q produces more watch requests than expected", operator)

			topology := "HA"
			if infra.Status.ControlPlaneTopology == configv1.SingleReplicaTopologyMode {
				topology = "single"
			}

			failureMessage := fmt.Sprintf(`TEST PURPOSE:
This test monitors watch request counts from operators to detect explosive growth in watch channel usage.
Excessive watch requests can overload the kube-apiserver and usually indicate operator bugs.

WHAT THIS FAILURE MEANS:
The %s operator has exceeded its expected watch request limit. This is often normal as operators
evolve and may require periodic limit increases. However, investigate before updating:

1. Check if the increase is reasonable (<50%% and <10x current limit)
2. Verify there are recent code changes that explain the increase
3. Review recent commits to this operator for watch-related changes

FAILURE DETAILS:
Operator: %s
Watch request count: %v
Upper bound (enforced): %v
Base limit (in code): %v
Ratio: %.2f

FAILURES TO INVESTIGATE:
- Increases >10x the current limit (probable bug)
- Unexplained increases with no recent code changes
- Multiple operators suddenly increasing (systemic issue)

HOW TO UPDATE LIMITS:
If the increase is reasonable and explainable, update the limit using:

  /update-operator-watch-request-limits %s %s %d --topology=%s

Or manually edit: pkg/monitortests/kubeapiserver/auditloganalyzer/handle_operator_watch_count_tracking.go

For investigation guidance, see: https://search.ci.openshift.org/?search=produces+more+watch+requests+than+expected

Platform: %v, Topology: %v
`, operator, operator, item.Count, allowedCount, allowedCount/2, ratio,
				operator, infra.Spec.PlatformSpec.Type, int64(math.Ceil(float64(item.Count)/2)), topology,
				infra.Spec.PlatformSpec.Type, infra.Status.ControlPlaneTopology)

			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: failureMessage,
				},
			})
		} else {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})
		}
	}

	return ret, nil
}
