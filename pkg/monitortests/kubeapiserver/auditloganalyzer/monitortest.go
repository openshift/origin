package auditloganalyzer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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

	summarizer            *summarizer
	excessiveApplyChecker *excessiveApplies
	requestCountTracking  *countTracking

	countsForInstall *CountsForRun
}

func NewAuditLogAnalyzer() monitortestframework.MonitorTest {
	return &auditLogAnalyzer{
		summarizer:            NewAuditLogSummarizer(),
		excessiveApplyChecker: CheckForExcessiveApplies(),
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
			currentNumberOf500s := int(currSecondRequests.NumberOfRequestsReceivedThatLaterGot500.Load())
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
				outageTotalRequests += int(currSecondRequests.NumberOfRequestsReceived.Load())
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
