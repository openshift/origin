package auditloganalyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/sirupsen/logrus"

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
	requestCountTracking          *countTracking
	invalidRequestsChecker        *invalidRequests
	requestsDuringShutdownChecker *lateRequestTracking
	violationChecker              *auditViolations

	countsForInstall      *CountsForRun
	installCompletionTime time.Time
	updateBegin           time.Time
	updateEnd             time.Time
	runBegin              time.Time
	runEnd                time.Time
}

func NewAuditLogAnalyzer() monitortestframework.MonitorTest {
	return &auditLogAnalyzer{
		summarizer:                    NewAuditLogSummarizer(),
		excessiveApplyChecker:         CheckForExcessiveApplies(),
		invalidRequestsChecker:        CheckForInvalidMutations(),
		requestsDuringShutdownChecker: CheckForRequestsDuringShutdown(),
		violationChecker:              CheckForViolations(),
	}
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
		w.invalidRequestsChecker,
		w.requestsDuringShutdownChecker,
		w.violationChecker,
	}
	if w.requestCountTracking != nil {
		auditLogHandlers = append(auditLogHandlers, w.requestCountTracking)
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
		w.runBegin = beginning
		w.runEnd = end
		if len(clusterVersion.Status.History) > 0 {
			installedLevel := clusterVersion.Status.History[len(clusterVersion.Status.History)-1]
			if installedLevel.CompletionTime != nil {
				w.installCompletionTime = installedLevel.CompletionTime.Time
				w.countsForInstall = w.requestCountTracking.CountsForRun.SubsetDataAtTime(*installedLevel.CompletionTime)
			}
		}
		if len(clusterVersion.Status.History) >= 2 {
			firstUpdateLevel := clusterVersion.Status.History[1]

			// be sure the update happened during the run so we don't double count stuff by accident
			if firstUpdateLevel.StartedTime.Time.After(beginning) && firstUpdateLevel.StartedTime.Time.Before(end) {
				w.updateBegin = firstUpdateLevel.StartedTime.Time

				if firstUpdateLevel.CompletionTime != nil {
					w.updateEnd = firstUpdateLevel.CompletionTime.Time
				} else {
					// if the update didn't end, use the end of the run to include as much data as possible
					w.updateEnd = end
				}
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

func (w *auditLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}
	if w.requestCountTracking != nil {
		requestCountIntervals := func() monitorapi.Intervals {
			w.requestCountTracking.lock.Lock()
			defer w.requestCountTracking.lock.Unlock()

			retIntervals := monitorapi.Intervals{}

			lastSecondSawNoLeader := false
			currTotalRequestsToEtcd := 0
			currTotalRequestsWithNoLeader := 0
			startOfNoLeaderInterval := time.Time{}
			for second, counts := range w.requestCountTracking.CountsForRun.CountsForEachSecond {
				switch {
				case !lastSecondSawNoLeader && counts.NumberOfRequestsReceivedSawNoLeader > 0:
					lastSecondSawNoLeader = true
					currTotalRequestsToEtcd += counts.NumberOfRequestsReceivedThatAccessedEtcd
					currTotalRequestsWithNoLeader += counts.NumberOfRequestsReceivedSawNoLeader
					startOfNoLeaderInterval = w.requestCountTracking.CountsForRun.EstimatedStartOfCluster.Add(time.Second * time.Duration(second))

				case lastSecondSawNoLeader && counts.NumberOfRequestsReceivedSawNoLeader > 0:
					// continuing to have no leader problems
					currTotalRequestsToEtcd += counts.NumberOfRequestsReceivedThatAccessedEtcd
					currTotalRequestsWithNoLeader += counts.NumberOfRequestsReceivedSawNoLeader

				case !lastSecondSawNoLeader && counts.NumberOfRequestsReceivedSawNoLeader == 0:
					// continuing to be no leader problem-free, do nothing

				case lastSecondSawNoLeader && counts.NumberOfRequestsReceivedSawNoLeader == 0:
					endOfNoLeaderInterval := w.requestCountTracking.CountsForRun.EstimatedStartOfCluster.Add(time.Second * time.Duration(second))
					failurePercentage := int((float32(currTotalRequestsWithNoLeader) / float32(currTotalRequestsToEtcd)) * 100)
					retIntervals = append(retIntervals,
						monitorapi.NewInterval(monitorapi.SourceAuditLog, monitorapi.Error).
							Locator(monitorapi.NewLocator().KubeAPIServerWithLB("any")).
							Message(monitorapi.NewMessage().
								Reason(monitorapi.ReasonKubeAPIServerNoLeader).
								WithAnnotation(monitorapi.AnnotationCount, strconv.Itoa(currTotalRequestsWithNoLeader)).
								WithAnnotation(monitorapi.AnnotationPercentage, strconv.Itoa(failurePercentage)).
								HumanMessagef("%d requests made during this time failed with 'no leader' out of %d total", currTotalRequestsWithNoLeader, currTotalRequestsToEtcd),
							).
							Display().
							Build(startOfNoLeaderInterval, endOfNoLeaderInterval))

					lastSecondSawNoLeader = false
					currTotalRequestsToEtcd = 0
					currTotalRequestsWithNoLeader = 0
					startOfNoLeaderInterval = time.Time{}
				}
			}

			return retIntervals
		}()
		ret = append(ret, requestCountIntervals...)
	}

	return ret, nil
}

func (w *auditLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	ret := []*junitapi.JUnitTestCase{}

	{
		testName := "[Jira:kube-apiserver] kube-apiserver should not have internal failures"
		offendingIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.ReasonKubeAPIServer500s
		})
		totalDurationOfOffense := 0
		offenses := []string{}
		for _, interval := range offendingIntervals {
			totalDurationOfOffense += int(interval.To.Sub(interval.From).Seconds()) + 1
			offenses = append(offenses, interval.String())
		}
		if totalDurationOfOffense > 60 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(offenses, "\n"),
					Output:  fmt.Sprintf("kube-apiserver had internal errors for %v seconds total", totalDurationOfOffense),
				},
			})
			// flake for now
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})
		} else {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})
		}
	}

	{
		testName := "[Jira:etcd] kube-apiserver should not have etcd no leader problems"
		offendingIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.ReasonKubeAPIServerNoLeader
		})
		totalDurationOfOffense := 0
		offenses := []string{}
		for _, interval := range offendingIntervals {
			totalDurationOfOffense += int(interval.To.Sub(interval.From).Seconds()) + 1
			offenses = append(offenses, interval.String())
		}
		if totalDurationOfOffense > 100 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(offenses, "\n"),
					Output:  fmt.Sprintf("kube-apiserver had internal errors for %v seconds total", totalDurationOfOffense),
				},
			})
			// surely we don't have more than 100 seconds of no leader, start of failure?
			//ret = append(ret, &junitapi.JUnitTestCase{
			//	Name: testName,
			//})
		} else if totalDurationOfOffense > 0 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(offenses, "\n"),
					Output:  fmt.Sprintf("kube-apiserver had internal errors for %v seconds total", totalDurationOfOffense),
				},
			})
			// flake for now
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})

		} else {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})
		}
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
				switch username {
				case "system:serviceaccount:openshift-infra:serviceaccount-pull-secrets-controller",
					"system:serviceaccount:openshift-network-operator:cluster-network-operator",
					"system:serviceaccount:openshift-infra:podsecurity-admission-label-syncer-controller",
					"system:serviceaccount:openshift-cluster-olm-operator:cluster-olm-operator",
					"system:serviceaccount:openshift-monitoring:prometheus-operator":

					// These usernames are already creating more than 200 applies, so flake instead of fail.
					// We really want to find a way to track namespaces created by the payload versus everything else.
					flakes = append(flakes, fmt.Sprintf("user %v had %d applies, check the audit log and operator log to figure out why", username, numberOfApplies))
				default:
					failures = append(failures, fmt.Sprintf("user %v had %d applies, check the audit log and operator log to figure out why", username, numberOfApplies))
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
						Message: strings.Join(failures, "\n"),
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

	testName := "API LBs follow /readyz of kube-apiserver and stop sending requests before server shutdowns for external clients"
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

	return ret, nil
}

func (w *auditLogAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if currErr := WriteAuditLogSummary(storageDir, timeSuffix, w.summarizer.auditLogSummary); currErr != nil {
		return currErr
	}

	if w.requestCountTracking != nil {
		err := w.requestCountTracking.CountsForRun.WriteContentToStorage(storageDir, "request-counts-by-second", timeSuffix)
		if err != nil {
			return err
		}

		writeNoLeaderSeconds(storageDir, timeSuffix, w.requestCountTracking, w.installCompletionTime, w.updateBegin, w.updateEnd, w.runBegin, w.runEnd)
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

type noLeaderRow struct {
	timeRangeName                          string
	requestsReceivedCount                  int
	requestsReceivedWithEtcdCount          int
	requestsReceivedWithoutEtcdCount       int
	requestsReceivedWithNoLeaderErrorCount int
}

func (r *noLeaderRow) add(counts CountForSecond) {
	r.requestsReceivedCount += counts.NumberOfRequestsReceived
	r.requestsReceivedWithEtcdCount += counts.NumberOfRequestsReceivedThatAccessedEtcd
	r.requestsReceivedWithoutEtcdCount += counts.NumberOfRequestsReceivedDidNotAccessEtcd
	r.requestsReceivedWithNoLeaderErrorCount += counts.NumberOfRequestsReceivedSawNoLeader
}
func (r *noLeaderRow) toData() map[string]string {
	return map[string]string{
		"TimeRangeName":                          r.timeRangeName,
		"RequestsReceivedCount":                  strconv.Itoa(r.requestsReceivedCount),
		"RequestsReceivedWithEtcdCount":          strconv.Itoa(r.requestsReceivedWithEtcdCount),
		"RequestsReceivedWithoutEtcdCount":       strconv.Itoa(r.requestsReceivedWithoutEtcdCount),
		"RequestsReceivedWithNoLeaderErrorCount": strconv.Itoa(r.requestsReceivedWithNoLeaderErrorCount),
	}
}

func writeNoLeaderSeconds(artifactDir, timeSuffix string, countTracking *countTracking, installCompletion, updateBegin, updateEnd, beginning, end time.Time) {
	countTracking.lock.Lock()
	defer countTracking.lock.Unlock()

	hasUpdate := !updateBegin.IsZero()

	installCompletionSecond := int(installCompletion.Sub(countTracking.CountsForRun.EstimatedStartOfCluster.Time).Seconds())
	updateBeginSecond := int(updateBegin.Sub(countTracking.CountsForRun.EstimatedStartOfCluster.Time).Seconds())
	updateEndSecond := int(updateEnd.Sub(countTracking.CountsForRun.EstimatedStartOfCluster.Time).Seconds())
	beginningOfRunSecond := int(beginning.Sub(countTracking.CountsForRun.EstimatedStartOfCluster.Time).Seconds())
	endOfRunSecond := int(end.Sub(countTracking.CountsForRun.EstimatedStartOfCluster.Time).Seconds())

	installCounts := &noLeaderRow{timeRangeName: "Install"}
	updateCounts := &noLeaderRow{timeRangeName: "Update"}
	allKnownCounts := &noLeaderRow{timeRangeName: "AllKnown"}
	duringTests := &noLeaderRow{timeRangeName: "DuringTests"}
	for secondOfRun, counts := range countTracking.CountsForRun.CountsForEachSecond {
		allKnownCounts.add(counts)

		if secondOfRun <= installCompletionSecond {
			installCounts.add(counts)
		}
		if hasUpdate && secondOfRun >= updateBeginSecond && secondOfRun <= updateEndSecond {
			updateCounts.add(counts)
		}
		if secondOfRun >= beginningOfRunSecond && secondOfRun <= endOfRunSecond {
			duringTests.add(counts)
		}
	}

	dataFile := dataloader.DataFile{
		TableName: "audit_no_leader_seconds",
		Schema: map[string]dataloader.DataType{
			"TimeRangeName":                          dataloader.DataTypeString,
			"RequestsReceivedCount":                  dataloader.DataTypeInteger,
			"RequestsReceivedWithEtcdCount":          dataloader.DataTypeInteger,
			"RequestsReceivedWithoutEtcdCount":       dataloader.DataTypeInteger,
			"RequestsReceivedWithNoLeaderErrorCount": dataloader.DataTypeInteger,
		},
		Rows: []map[string]string{
			installCounts.toData(),
			updateCounts.toData(),
			allKnownCounts.toData(),
			duringTests.toData(),
		},
	}
	fileName := filepath.Join(artifactDir, fmt.Sprintf("audit-no-leader-seconds%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}
}
