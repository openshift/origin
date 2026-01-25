package auditloganalyzer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchnamespaces"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type auditLogAnalyzer struct {
	adminRESTConfig *rest.Config

	isTechPreview bool

	summarizer                    *summarizer
	excessiveApplyChecker         *excessiveApplies
	excessiveConflictsChecker     *excessiveConflicts
	requestCountTracking          *countTracking
	invalidRequestsChecker        *invalidRequests
	requestsDuringShutdownChecker *lateRequestTracking
	violationChecker              *auditViolations
	watchCountTracking            *watchCountTracking
	latencyChecker                *auditLatencyRecords

	countsForInstall *CountsForRun

	clusterStability monitortestframework.ClusterStabilityDuringTest
}

func NewAuditLogAnalyzer(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTest {
	return &auditLogAnalyzer{
		summarizer:                    NewAuditLogSummarizer(),
		excessiveApplyChecker:         CheckForExcessiveApplies(),
		excessiveConflictsChecker:     CheckForExcessiveConflicts(),
		invalidRequestsChecker:        CheckForInvalidMutations(),
		requestsDuringShutdownChecker: CheckForRequestsDuringShutdown(),
		violationChecker:              CheckForViolations(),
		watchCountTracking:            NewWatchCountTracking(),
		latencyChecker:                CheckForLatency(),
		clusterStability:              info.ClusterStabilityDuringTest,
	}
}

func (w *auditLogAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *auditLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig

	configClient, err := configclient.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return err
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
	// do nothing, microshift I think
	case err != nil:
		return err
	default:
		w.requestCountTracking = CountsOverTime(clusterVersion.CreationTimestamp)
	}

	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return err
	}
	if isMicroshift, _ := exutil.IsMicroShiftCluster(kubeClient); !isMicroshift {
		w.isTechPreview = exutil.IsTechPreviewNoUpgrade(ctx, configClient)
	}

	return nil
}

func (w *auditLogAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}

	auditLogHandlers := []AuditEventHandler{
		w.summarizer,
		w.excessiveApplyChecker,
		w.excessiveConflictsChecker,
		w.invalidRequestsChecker,
		w.requestsDuringShutdownChecker,
		w.violationChecker,
		w.latencyChecker,
	}
	if w.requestCountTracking != nil {
		auditLogHandlers = append(auditLogHandlers, w.requestCountTracking)
	}

	if w.clusterStability == monitortestframework.Stable {
		auditLogHandlers = append(auditLogHandlers, w.watchCountTracking)
	}

	err = GetKubeAuditLogSummary(ctx, kubeClient, &beginning, &end, auditLogHandlers)

	retIntervals := monitorapi.Intervals{}

	if w.requestCountTracking != nil {
		w.requestCountTracking.CountsForRun.TruncateDataAfterLastValue()

		// now make a smaller line chart that only includes installation so the plot will be a little easier to read
		configClient, err := configclient.NewForConfig(w.adminRESTConfig)
		if err != nil {
			return nil, nil, err
		}
		clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		if len(clusterVersion.Status.History) > 0 {
			installedLevel := clusterVersion.Status.History[len(clusterVersion.Status.History)-1]
			if installedLevel.CompletionTime != nil {
				w.countsForInstall = w.requestCountTracking.CountsForRun.SubsetDataAtTime(*installedLevel.CompletionTime)
			}
		}

		// at this point we have the ability to create intervals for every period where there are more than zero requests resulting in 500s
		startOfCurrentProblems := -1
		outageTotalNumberOf500s := 0
		outageTotalRequests := 0
		for i, currSecondRequests := range w.requestCountTracking.CountsForRun.CountsForEachSecond {
			currentNumberOf500s := currSecondRequests.NumberOfRequestsReceivedThatLaterGot500
			if currentNumberOf500s == 0 {
				if startOfCurrentProblems >= 0 { // we're at the end of a trouble period
					from := w.requestCountTracking.CountsForRun.EstimatedStartOfCluster.Add(time.Duration(startOfCurrentProblems) * time.Second)
					to := w.requestCountTracking.CountsForRun.EstimatedStartOfCluster.Add(time.Duration(i) * time.Second)
					failurePercentage := int((float32(outageTotalNumberOf500s) / float32(outageTotalRequests)) * 100)
					retIntervals = append(retIntervals,
						monitorapi.NewInterval(monitorapi.SourceAuditLog, monitorapi.Error).
							Locator(monitorapi.NewLocator().KubeAPIServerWithLB("any")).
							Message(monitorapi.NewMessage().
								Reason(monitorapi.ReasonKubeAPIServer500s).
								WithAnnotation(monitorapi.AnnotationCount, strconv.Itoa(outageTotalNumberOf500s)).
								WithAnnotation(monitorapi.AnnotationPercentage, strconv.Itoa(failurePercentage)).
								HumanMessagef("%d requests made during this time failed out of %d total", outageTotalNumberOf500s, outageTotalRequests),
							).
							Display().
							Build(from, to))

					startOfCurrentProblems = -1
					outageTotalNumberOf500s = 0
					outageTotalRequests = 0
				}
				continue
			}
			if startOfCurrentProblems < 0 {
				startOfCurrentProblems = i
				outageTotalNumberOf500s += currentNumberOf500s
				outageTotalRequests += currSecondRequests.NumberOfRequestsReceived
			}
		}
	}

	return retIntervals, nil, err
}

func (*auditLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *auditLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	ret := []*junitapi.JUnitTestCase{}

	fiveHundredsTestName := "[Jira:kube-apiserver] kube-apiserver should not have internal failures"
	apiserver500s := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.ReasonKubeAPIServer500s
	})
	totalDurationOf500s := 0
	fiveHundredsFailures := []string{}
	for _, interval := range apiserver500s {
		totalDurationOf500s += int(interval.To.Sub(interval.From).Seconds()) + 1
		fiveHundredsFailures = append(fiveHundredsFailures, interval.String())
	}
	if totalDurationOf500s > 60 {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: fiveHundredsTestName,
			FailureOutput: &junitapi.FailureOutput{
				Message: strings.Join(fiveHundredsFailures, "\n"),
				Output:  fmt.Sprintf("kube-apiserver had internal errors for %v seconds total", totalDurationOf500s),
			},
		})
		// flake for now
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: fiveHundredsTestName,
		})
	} else {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: fiveHundredsTestName,
		})
	}

	allPlatformNamespaces, err := watchnamespaces.GetAllPlatformNamespaces()
	if err != nil {
		return nil, fmt.Errorf("problem getting platform namespaces: %w", err)
	}

	for _, namespace := range allPlatformNamespaces {
		testName := fmt.Sprintf("users in ns/%s must not produce too many applies", namespace)
		usersToApplies := w.excessiveApplyChecker.namespacesToUserToNumberOfApplies[namespace]

		failures := []string{}
		flakes := []string{}
		for username, numberOfApplies := range usersToApplies {
			if numberOfApplies > 200 {
				errorMessage := fmt.Sprintf("user %v had %d applies, check the audit log and operator log to figure out why", username, numberOfApplies)
				switch username {
				case "system:serviceaccount:openshift-infra:serviceaccount-pull-secrets-controller",
					"system:serviceaccount:openshift-network-operator:cluster-network-operator",
					"system:serviceaccount:openshift-infra:podsecurity-admission-label-syncer-controller",
					"system:serviceaccount:openshift-cluster-olm-operator:cluster-olm-operator",
					"system:serviceaccount:openshift-monitoring:prometheus-operator":

					// These usernames are already creating more than 200 applies, so flake instead of fail.
					// We really want to find a way to track namespaces created by the payload versus everything else.
					flakes = append(flakes, errorMessage)
				default:
					failures = append(failures, errorMessage)
				}
			}
		}

		switch {
		case len(failures) > 1:
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
					FailureOutput: &junitapi.FailureOutput{
						Message: strings.Join(failures, "\n"),
						Output:  "details in audit log",
					},
				},
			)

		case len(flakes) > 1:
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
					FailureOutput: &junitapi.FailureOutput{
						Message: strings.Join(flakes, "\n"),
						Output:  "details in audit log",
					},
				},
			)
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
				},
			)

		default:
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
				},
			)
		}

	}

	testName := `[Jira:"kube-apiserver"] API resources are not updated excessively`
	flakes := []string{}
	for resource, applies := range w.excessiveApplyChecker.resourcesToNumberOfApplies {
		if applies.numberOfApplies < 200 {
			continue
		}
		errorMessage := fmt.Sprintf("resource %s had %d applies, %s", resource, applies.numberOfApplies, applies.toErrorString())
		flakes = append(flakes, errorMessage)
	}
	switch {
	case len(flakes) > 1:
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(flakes, "\n"),
					Output:  "details in audit log",
				},
			},
		)
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)

	default:
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	for _, namespace := range allPlatformNamespaces {
		testName := fmt.Sprintf("users in ns/%s must not produce too many conflicts", namespace)
		usersToConflicts := w.excessiveConflictsChecker.namespacesToUserToNumberOfConflicts[namespace]

		failures := []string{}
		flakes := []string{}
		for username, numberOfConflicts := range usersToConflicts {
			if numberOfConflicts > 10 {
				errorMessage := fmt.Sprintf("user %v had %d update conflicts, check the audit log and operator log to figure out why", username, numberOfConflicts)
				switch username {
				case "system:serviceaccount:openshift-infra:serviceaccount-controller",
					"system:serviceaccount:openshift-infra:deploymentconfig-controller",
					"system:serviceaccount:openshift-infra:deployer-controller",
					"system:serviceaccount:openshift-infra:namespace-security-allocation-controller",
					"system:serviceaccount:openshift-infra:resourcequota-controller",
					"system:serviceaccount:openshift-infra:origin-namespace-controller",
					"system:serviceaccount:openshift-infra:serviceaccount-pull-secrets-controller":

					// These usernames are already creating more than 10 conflicts, so flake instead of fail.
					flakes = append(flakes, errorMessage)
				default:
					failures = append(failures, errorMessage)
				}
			}
		}

		switch {
		case len(failures) > 1:
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
					FailureOutput: &junitapi.FailureOutput{
						Message: strings.Join(failures, "\n"),
						Output:  "details in audit log",
					},
				},
			)

		case len(flakes) > 1:
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
					FailureOutput: &junitapi.FailureOutput{
						Message: strings.Join(flakes, "\n"),
						Output:  "details in audit log",
					},
				},
			)
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
				},
			)

		default:
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
				},
			)
		}

	}

	testName = `[Jira:"kube-apiserver"] API resources are not conflicting excessively`
	flakes = []string{}
	for resource, conflicts := range w.excessiveConflictsChecker.resourcesToNumberOfConflicts {
		if conflicts.numberOfConflicts < 10 {
			continue
		}
		errorMessage := fmt.Sprintf("resource %s had %d conflicts, %s", resource, conflicts.numberOfConflicts, conflicts.toErrorString())
		flakes = append(flakes, errorMessage)
	}
	switch {
	case len(flakes) > 1:
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(flakes, "\n"),
					Output:  "details in audit log",
				},
			},
		)
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)

	default:
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	for verb, namespacesToUserToNumberOf422s := range w.invalidRequestsChecker.verbToNamespacesTouserToNumberOf422s {
		for _, namespace := range allPlatformNamespaces {
			testName := fmt.Sprintf("users in ns/%s must not produce too many invalid %q requests", namespace, verb)
			usersTo422s := namespacesToUserToNumberOf422s.namespacesToInvalidUserTrackers[namespace]

			failures := []string{}
			flakes := []string{}
			if usersTo422s != nil {
				for username, userInvalidRequestDetails := range usersTo422s.usersToInvalidRequests {
					switch {
					// this user appears to make invalid creates as part of e2e tests
					case verb == "create" && username == "system:serviceaccount:openshift-infra:build-config-change-controller":
						continue
					// this user appears to make invalid creates as part of e2e tests
					case verb == "create" && username == "system:serviceaccount:openshift-infra:template-instance-controller":
						continue
					}

					if w.isTechPreview {
						switch {
						// this user is bugged: https://issues.redhat.com/browse/OCPBUGS-42816
						// we must not allow this bug to be promoted to default. we'd make the exclusion even tighter if we could do so easily.
						case verb == "apply" && username == "system:serviceaccount:openshift-machine-config-operator:machine-config-daemon":
							continue
						// this user is bugged: https://issues.redhat.com/browse/OCPBUGS-42816
						// we must not allow this bug to be promoted to default. we'd make the exclusion even tighter if we could do so easily.
						case verb == "apply" && username == "system:serviceaccount:openshift-machine-config-operator:machine-config-operator":
							continue
						}
					}

					if userInvalidRequestDetails.totalNumberOfFailures > 0 {
						first10FailureStrings := []string{}
						last10FailureStrings := []string{}
						for _, curr := range userInvalidRequestDetails.first10Failures {
							first10FailureStrings = append(first10FailureStrings, fmt.Sprintf("%v request=%v auditID=%v", curr.RequestReceivedTimestamp.Round(time.Second), curr.RequestURI, curr.AuditID))
						}
						for _, curr := range userInvalidRequestDetails.last10Failures {
							last10FailureStrings = append(last10FailureStrings, fmt.Sprintf("%v request=%v auditID=%v", curr.RequestReceivedTimestamp.Round(time.Second), curr.RequestURI, curr.AuditID))
						}
						failures = append(failures, fmt.Sprintf(
							"user %v had %d invalid %q, check the audit log and operator log to figure out why.\n%v\n%v",
							username,
							userInvalidRequestDetails.totalNumberOfFailures,
							verb,
							first10FailureStrings,
							last10FailureStrings,
						))
					}
				}
			}

			switch {
			case len(failures) > 1:
				ret = append(ret,
					&junitapi.JUnitTestCase{
						Name: testName,
						FailureOutput: &junitapi.FailureOutput{
							Message: strings.Join(failures, "\n"),
							Output:  "more details in audit log",
						},
					},
				)

			case len(flakes) > 1:
				ret = append(ret,
					&junitapi.JUnitTestCase{
						Name: testName,
						FailureOutput: &junitapi.FailureOutput{
							Message: strings.Join(failures, "\n"),
							Output:  "more details in audit log",
						},
					},
				)
				ret = append(ret,
					&junitapi.JUnitTestCase{
						Name: testName,
					},
				)

			default:
				ret = append(ret,
					&junitapi.JUnitTestCase{
						Name: testName,
					},
				)
			}
		}
	}

	testName = "[sig-api-machinery][Feature:APIServer] API LBs follow /readyz of kube-apiserver and stop sending requests before server shutdowns for external clients"
	switch {
	case len(w.requestsDuringShutdownChecker.auditIDs) > 0:
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: fmt.Sprintf("The following requests arrived when apiserver was gracefully shutting down:\n%s", strings.Join(w.requestsDuringShutdownChecker.auditIDs, "\n")),
					Output:  "more details in audit log",
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

	ret = append(ret, w.violationChecker.CreateJunits()...)

	if w.clusterStability == monitortestframework.Stable {
		junits, err := w.watchCountTracking.CreateJunits()
		if err == nil {
			ret = append(ret, junits...)
		}
	}

	return ret, err
}

func (w *auditLogAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if currErr := WriteAuditLogSummary(storageDir, timeSuffix, w.summarizer.auditLogSummary); currErr != nil {
		return currErr
	}

	if w.watchCountTracking != nil && w.clusterStability == monitortestframework.Stable {
		err := w.watchCountTracking.WriteAuditLogSummary(ctx, storageDir, "watch-requests", timeSuffix)
		if err != nil {
			// print any error and continue processing
			fmt.Printf("unable to write audit log summary for %s - %v\n", "watch-requests", err)
		}
	}

	if w.latencyChecker != nil {
		err := w.latencyChecker.WriteAuditLogSummary(storageDir, "audit-latency", timeSuffix)
		if err != nil {
			// print any error and continue processing
			fmt.Printf("unable to write audit log summary for %s - %v\n", "audit-latency", err)
		}
	}

	if w.requestCountTracking != nil {
		err := w.requestCountTracking.CountsForRun.WriteContentToStorage(storageDir, "request-counts-by-second", timeSuffix)
		if err != nil {
			return err
		}
	}
	if w.countsForInstall != nil {
		err := w.countsForInstall.WriteContentToStorage(storageDir, "request-counts-for-install", timeSuffix)
		if err != nil {
			return err
		}
	}

	return nil
}

func (*auditLogAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
