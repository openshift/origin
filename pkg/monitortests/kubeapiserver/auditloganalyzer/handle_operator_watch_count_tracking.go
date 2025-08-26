package auditloganalyzer

import (
	"context"
	"errors"
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
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

// with https://github.com/openshift/kubernetes/pull/2113 we no longer have the counts used in
// https://github.com/openshift/origin/blob/35b3c221634bcaef9a5148477334c27d166b7838/pkg/monitortests/testframework/watchrequestcountscollector/monitortest.go#L111
// attempt to recreate via audit events

type watchCountTracking struct {
	lock                  sync.Mutex
	startTime             time.Time
	watchRequestCountsMap map[OperatorKey]*RequestCount
	clusterStability      monitortestframework.ClusterStabilityDuringTest
}

func NewWatchCountTracking(info monitortestframework.MonitorTestInitializationInfo) *watchCountTracking {
	return &watchCountTracking{startTime: time.Now(), watchRequestCountsMap: map[OperatorKey]*RequestCount{}, clusterStability: info.ClusterStabilityDuringTest}
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

func (s *watchCountTracking) CreateJunits() ([]*junitapi.JUnitTestCase, error) {
	ret := []*junitapi.JUnitTestCase{}

	testName := "[sig-arch][Late] operators should not create watch channels very often"
	testMinRequestsName := "[sig-arch][Late] operators should have watch channel requests"
	oc := exutil.NewCLIWithoutNamespace("operator-watch")
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Message: err.Error(),
				Output:  err.Error(),
			},
		},
		)

		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testMinRequestsName,
			FailureOutput: &junitapi.FailureOutput{
				Message: err.Error(),
				Output:  err.Error(),
			},
		},
		)

		return ret, nil
	}

	type platformUpperBound map[string]int64

	// See https://issues.redhat.com/browse/WRKLDS-291 for upper bounds computation
	//
	// These values need to be periodically incremented as code evolves, the stated goal of the test is to prevent
	// exponential growth. Original methodology for calculating the values is difficult, iterations since have just
	// been done manually via search.ci. (i.e. https://search.ci.openshift.org/?search=produces+more+watch+requests+than+expected&maxAge=336h&context=0&type=bug%2Bjunit&name=&excludeName=&maxMatches=5&maxBytes=20971520&groupBy=job )
	upperBounds := map[configv1.PlatformType]platformUpperBound{
		configv1.AWSPlatformType: {
			"authentication-operator":                519.0,
			"aws-ebs-csi-driver-operator":            199.0,
			"cloud-credential-operator":              176.0,
			"cluster-autoscaler-operator":            132.0,
			"cluster-baremetal-operator":             125.0,
			"cluster-capi-operator":                  205.0,
			"cluster-image-registry-operator":        189.0,
			"cluster-monitoring-operator":            186.0,
			"cluster-node-tuning-operator":           115.0,
			"cluster-samples-operator":               76.0,
			"cluster-storage-operator":               322.0,
			"console-operator":                       206.0,
			"csi-snapshot-controller-operator":       120.0,
			"dns-operator":                           94.0,
			"etcd-operator":                          245.0,
			"ingress-operator":                       556.0,
			"kube-apiserver-operator":                373.0,
			"kube-controller-manager-operator":       282.0,
			"kube-storage-version-migrator-operator": 111.0,
			"machine-api-operator":                   126.0,
			"marketplace-operator":                   52.0,
			"openshift-apiserver-operator":           419.0,
			"openshift-config-operator":              87.0,
			"openshift-controller-manager-operator":  286.0,
			"openshift-kube-scheduler-operator":      252.0,
			"operator":                               49.0,
			"prometheus-operator":                    222.0,
			"service-ca-operator":                    170.0,
		},
		configv1.AzurePlatformType: {
			"authentication-operator":                527.0,
			"azure-disk-csi-driver-operator":         170.0,
			"cloud-credential-operator":              129.0,
			"cluster-autoscaler-operator":            100.0,
			"cluster-baremetal-operator":             90.0,
			"cluster-capi-operator":                  210.0,
			"cluster-image-registry-operator":        194.0,
			"cluster-monitoring-operator":            191.0,
			"cluster-node-tuning-operator":           110.0,
			"cluster-samples-operator":               59.0,
			"cluster-storage-operator":               322.0,
			"console-operator":                       212.0,
			"csi-snapshot-controller-operator":       130.0,
			"dns-operator":                           104.0,
			"etcd-operator":                          254.0,
			"ingress-operator":                       541.0,
			"kube-apiserver-operator":                392.0,
			"kube-controller-manager-operator":       279.0,
			"kube-storage-version-migrator-operator": 120.0,
			"machine-api-operator":                   97.0,
			"marketplace-operator":                   39.0,
			"openshift-apiserver-operator":           428.0,
			"openshift-config-operator":              105.0,
			"openshift-controller-manager-operator":  296.0,
			"openshift-kube-scheduler-operator":      255.0,
			"operator":                               37.0,
			"prometheus-operator":                    184.0,
			"service-ca-operator":                    180.0,
		},
		configv1.GCPPlatformType: {
			"authentication-operator":                349.0,
			"cloud-credential-operator":              115.0,
			"cluster-autoscaler-operator":            54.0,
			"cluster-baremetal-operator":             125.0,
			"cluster-capi-operator":                  215.0,
			"cluster-image-registry-operator":        121.0,
			"cluster-monitoring-operator":            193.0,
			"cluster-node-tuning-operator":           112.0,
			"cluster-samples-operator":               29.0,
			"cluster-storage-operator":               214.0,
			"console-operator":                       165.0,
			"csi-snapshot-controller-operator":       90.0,
			"dns-operator":                           80.0,
			"etcd-operator":                          220.0,
			"gcp-pd-csi-driver-operator":             114.0,
			"ingress-operator":                       475.0,
			"kube-apiserver-operator":                260.0,
			"kube-controller-manager-operator":       183.0,
			"kube-storage-version-migrator-operator": 130.0,
			"machine-api-operator":                   52.0,
			"marketplace-operator":                   19.0,
			"openshift-apiserver-operator":           284.0,
			"openshift-config-operator":              55.0,
			"openshift-controller-manager-operator":  210.0,
			"openshift-kube-scheduler-operator":      210.0,
			"operator":                               18.0,
			"prometheus-operator":                    127.0,
			"service-ca-operator":                    113.0,
		},
		configv1.BareMetalPlatformType: {
			"authentication-operator":                424.0,
			"cloud-credential-operator":              82.0,
			"cluster-autoscaler-operator":            72.0,
			"cluster-baremetal-operator":             130.0,
			"cluster-image-registry-operator":        160.0,
			"cluster-monitoring-operator":            189.0,
			"cluster-node-tuning-operator":           115.0,
			"cluster-samples-operator":               36.0,
			"cluster-storage-operator":               258.0,
			"console-operator":                       166.0,
			"csi-snapshot-controller-operator":       150.0,
			"dns-operator":                           80.0,
			"etcd-operator":                          240.0,
			"ingress-operator":                       500.0,
			"kube-apiserver-operator":                315.0,
			"kube-controller-manager-operator":       220.0,
			"kube-storage-version-migrator-operator": 110.0,
			"machine-api-operator":                   67.0,
			"marketplace-operator":                   28.0,
			"openshift-apiserver-operator":           349.0,
			"openshift-config-operator":              68.0,
			"openshift-controller-manager-operator":  232.0,
			"openshift-kube-scheduler-operator":      250.0,
			"operator":                               21.0,
			"prometheus-operator":                    165.0,
			"service-ca-operator":                    135.0,
		},
		configv1.VSpherePlatformType: {
			"authentication-operator":                311.0,
			"cloud-credential-operator":              71.0,
			"cluster-autoscaler-operator":            49.0,
			"cluster-baremetal-operator":             125.0,
			"cluster-image-registry-operator":        106.0,
			"cluster-monitoring-operator":            189.0,
			"cluster-node-tuning-operator":           110.0,
			"cluster-samples-operator":               28.0,
			"cluster-storage-operator":               195.0,
			"console-operator":                       160.0,
			"csi-snapshot-controller-operator":       80.0,
			"dns-operator":                           80.0,
			"etcd-operator":                          170.0,
			"ingress-operator":                       470.0,
			"kube-apiserver-operator":                245.0,
			"kube-controller-manager-operator":       175.0,
			"kube-storage-version-migrator-operator": 70.0,
			"machine-api-operator":                   50.0,
			"marketplace-operator":                   17.0,
			"openshift-apiserver-operator":           244.0,
			"openshift-config-operator":              52.0,
			"openshift-controller-manager-operator":  200.0,
			"openshift-kube-scheduler-operator":      146.0,
			"operator":                               16.0,
			"prometheus-operator":                    116.0,
			"service-ca-operator":                    103.0,
			"vmware-vsphere-csi-driver-operator":     120.0,
			"vsphere-problem-detector-operator":      52.0,
		},
		configv1.OpenStackPlatformType: {
			"authentication-operator":                309.0,
			"cloud-credential-operator":              70.0,
			"cluster-autoscaler-operator":            53.0,
			"cluster-baremetal-operator":             125.0,
			"cluster-image-registry-operator":        112.0,
			"cluster-monitoring-operator":            80.0,
			"cluster-node-tuning-operator":           105.0,
			"cluster-samples-operator":               26.0,
			"cluster-storage-operator":               189.0,
			"console-operator":                       150.0,
			"csi-snapshot-controller-operator":       100.0,
			"dns-operator":                           75.0,
			"etcd-operator":                          240.0,
			"ingress-operator":                       430.0,
			"kube-apiserver-operator":                228.0,
			"kube-controller-manager-operator":       160.0,
			"kube-storage-version-migrator-operator": 70.0,
			"machine-api-operator":                   48.0,
			"marketplace-operator":                   19.0,
			"openshift-apiserver-operator":           248.0,
			"openshift-config-operator":              48.0,
			"openshift-controller-manager-operator":  190.0,
			"openshift-kube-scheduler-operator":      230.0,
			"operator":                               15.0,
			"prometheus-operator":                    125.0,
			"service-ca-operator":                    100.0,
			"vmware-vsphere-csi-driver-operator":     111.0,
			"vsphere-problem-detector-operator":      50.0,
		},
	}

	upperBoundsSingleNode := map[configv1.PlatformType]platformUpperBound{
		configv1.AWSPlatformType: {
			"authentication-operator":                360,
			"aws-ebs-csi-driver-operator":            190,
			"cloud-credential-operator":              150,
			"cluster-autoscaler-operator":            60,
			"cluster-baremetal-operator":             125.0,
			"cluster-image-registry-operator":        150.0,
			"cluster-monitoring-operator":            125,
			"cluster-node-tuning-operator":           130,
			"cluster-samples-operator":               40,
			"cluster-storage-operator":               230,
			"console-operator":                       215,
			"csi-snapshot-controller-operator":       120,
			"dns-operator":                           110,
			"etcd-operator":                          245,
			"ingress-operator":                       640,
			"kube-apiserver-operator":                330,
			"kube-controller-manager-operator":       245,
			"kube-storage-version-migrator-operator": 68,
			"machine-api-operator":                   65,
			"marketplace-operator":                   20,
			"openshift-apiserver-operator":           280,
			"openshift-config-operator":              75,
			"openshift-controller-manager-operator":  285,
			"openshift-kube-scheduler-operator":      190,
			"prometheus-operator":                    139,
			"service-ca-operator":                    135,
		},
	}

	var upperBound platformUpperBound

	switch infra.Status.ControlPlaneTopology {
	case configv1.ExternalTopologyMode:
		return []*junitapi.JUnitTestCase{{
			Name:        testName,
			SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported topology: %v", infra.Status.ControlPlaneTopology)},
		}, {
			Name:        testMinRequestsName,
			SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported topology: %v", infra.Status.ControlPlaneTopology)},
		}}, nil

	case configv1.SingleReplicaTopologyMode:
		if _, exists := upperBoundsSingleNode[infra.Spec.PlatformSpec.Type]; !exists {
			return []*junitapi.JUnitTestCase{{
				Name:        testName,
				SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported single node platform type: %v", infra.Spec.PlatformSpec.Type)},
			}, {
				Name:        testMinRequestsName,
				SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported single node platform type: %v", infra.Spec.PlatformSpec.Type)},
			}}, nil
		}
		upperBound = upperBoundsSingleNode[infra.Spec.PlatformSpec.Type]

	default:
		if _, exists := upperBounds[infra.Spec.PlatformSpec.Type]; !exists {
			return []*junitapi.JUnitTestCase{{
				Name:        testName,
				SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported platform type: %v", infra.Spec.PlatformSpec.Type)},
			}, {
				Name:        testMinRequestsName,
				SkipMessage: &junitapi.SkipMessage{Message: fmt.Sprintf("unsupported platform type: %v", infra.Spec.PlatformSpec.Type)},
			}}, nil
		}
		upperBound = upperBounds[infra.Spec.PlatformSpec.Type]
	}

	watchRequestCounts := s.SummarizeWatchCountRequests()

	if len(watchRequestCounts) == 0 {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testMinRequestsName,
				FailureOutput: &junitapi.FailureOutput{
					Message: "Expected at least one watch request count to be present",
				},
			},
		)
		// flake for now
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testMinRequestsName,
			},
		)

	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testMinRequestsName,
			},
		)
	}

	boundsCheck := boundsChecker{
		Bounds:           upperBound,
		Platform:         infra.Spec.PlatformSpec.Type,
		ClusterStability: s.clusterStability,
		UpperBoundMultiple: upperBoundMultiple{
			// The upper bound are measured from CI runs where the tests might be running less than 2h in total.
			// In the worst case half of the requests will be put into each bucket. Thus, multiply the bound by 2
			Base: 2.0,
			// If the cluster stability is marked as disruptive, include an additional 20% grace factor for
			// cases where there is more watch channel churn due to kube-apiserver revision rollouts.
			// The worst case seen so far is roughly an additional 11% (a 1.11 ratio) when the
			// kube-apiserver is at revision 29.
			// Example of this case: https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-cluster-authentication-operator-release-4.20-periodics-e2e-gcp-external-oidc-configure-techpreview/1959933892798451712
			DisruptiveGraceFactor: 0.2,
		},
	}

	operatorBoundExceeded := []string{}
	for _, item := range watchRequestCounts {
		operator := operatorFromRequestCount(item)
		if err := boundsCheck.ensureBoundNotExceeded(item); err != nil {
			framework.Logf("Operator %q produces more watch requests than expected", operator)
			operatorBoundExceeded = append(operatorBoundExceeded, fmt.Sprintf("Operator %q produces more watch requests than expected: %s", operator, err.Error()))
		}
	}

	if len(operatorBoundExceeded) > 0 {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(operatorBoundExceeded, "\n"),
				},
			},
		)
	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	return ret, nil
}

// boundsChecker is a utility type to help with checking if an
// operator has exceeded the upper bounds limit of requests for
// a particular request type.
type boundsChecker struct {
	// Bounds is a mapping of operator to upper bounds for requests.
	Bounds map[string]int64

	// ClusterStability is the stability of the cluster during the test
	// run. This is used to provide some grace in the upper bounds
	// during known disruptive tests that may cut controller connections
	// to the Kubernetes API server (KAS) due to a revisioned rollout,
	// causing additional requests when the KAS is back up.
	ClusterStability monitortestframework.ClusterStabilityDuringTest

	// Platform is the platform that these requests were made against.
	Platform configv1.PlatformType

	// UpperBoundMultiple is the set of multipliers used
	// to provide some grace between the configured Bounds
	// based on various conditions.
	UpperBoundMultiple upperBoundMultiple
}

// upperBoundMultiple is a utility type to hold
// information as to multiples to apply to upper bound
// limits of requests for an operator.
type upperBoundMultiple struct {
	// Base is the base multiplier that should be applied
	// to the upper bounds.
	Base float64

	// DisruptiveGraceFactor is an additional amount added to
	// Base to provide some additional grace in the upper bounds
	// when the cluster stability during the test run is intentionally
	// set as "Disruptive".
	DisruptiveGraceFactor float64
}

// ensureBoundNotExceeded checks the provided RequestCount to see if the operator in question has exceeded
// the upper bounds for the number of requests for the request type it should make.
// It returns an error if the operator has exceeded the upper bounds for the number of requests the operator is
// allowed to make for the given request type.
func (bc boundsChecker) ensureBoundNotExceeded(requestCount *RequestCount) error {
	// nothing to check, return early
	if requestCount == nil {
		return nil
	}

	operator := operatorFromRequestCount(requestCount)

	allowedCount, exists := bc.Bounds[operator]

	if !exists {
		framework.Logf("Operator %v not found in upper bounds for %v", operator, bc.Platform)
		framework.Logf("operator=%v, watchrequestcount=%v", requestCount.Operator, requestCount.Count)
		return nil
	}

	upperBoundMultiple := bc.UpperBoundMultiple.Base

	if bc.ClusterStability == monitortestframework.Disruptive {
		upperBoundMultiple += bc.UpperBoundMultiple.DisruptiveGraceFactor
	}

	allowedCount = int64(float64(allowedCount) * upperBoundMultiple)
	ratio := float64(requestCount.Count) / float64(allowedCount)
	ratio = math.Round(ratio*100) / 100

	framework.Logf("operator=%v, watchrequestcount=%v, upperbound=%v, ratio=%v", operator, requestCount.Count, allowedCount, ratio)

	if requestCount.Count > allowedCount {
		return errors.New(fmt.Sprintf("watchrequestcount=%v, upperbound=%v, ratio=%v", requestCount.Count, allowedCount, ratio))
	}

	return nil
}

// operatorFromRequestCount is a utility function to extract an operator name
// from a RequestCount.
func operatorFromRequestCount(requestCount *RequestCount) string {
	if requestCount == nil {
		return ""
	}

	return strings.Split(requestCount.Operator, ":")[3]
}
