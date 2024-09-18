package auditloganalyzer

import (
	"context"
	"fmt"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type auditLogAnalyzer struct {
	adminRESTConfig *rest.Config

	summarizer            *summarizer
	excessiveApplyChecker *excessiveApplies
}

func NewAuditLogAnalyzer() monitortestframework.MonitorTest {
	return &auditLogAnalyzer{
		summarizer:            NewAuditLogSummarizer(),
		excessiveApplyChecker: CheckForExcessiveApplies(),
	}
}

func (w *auditLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
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
	err = GetKubeAuditLogSummary(ctx, kubeClient, &beginning, &end, auditLogHandlers)

	return nil, nil, err
}

func (*auditLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *auditLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	ret := []*junitapi.JUnitTestCase{}

	for username, numberOfApplies := range w.excessiveApplyChecker.userToNumberOfApplies {
		namespace, _, _ := serviceaccount.SplitUsername(username)
		testName := fmt.Sprintf("user %v in ns/%s must not produce too many applies", username, namespace)

		if numberOfApplies > 200 {
			ret = append(ret,
				&junitapi.JUnitTestCase{
					Name: testName,
					FailureOutput: &junitapi.FailureOutput{
						Message: fmt.Sprintf("had %d applies, check the audit log and operator log to figure out why", numberOfApplies),
						Output:  "details in audit log",
					},
				},
			)
			switch username {
			case "system:serviceaccount:openshift-infra:serviceaccount-pull-secrets-controller",
				"system:serviceaccount:openshift-network-operator:cluster-network-operator",
				"system:serviceaccount:openshift-infra:podsecurity-admission-label-syncer-controller",
				"system:serviceaccount:openshift-monitoring:prometheus-operator":

				// These usernames are already creating more than 200 applies, so flake instead of fail.
				// We really want to find a way to track namespaces created by the payload versus everything else.
				ret = append(ret,
					&junitapi.JUnitTestCase{
						Name: testName,
					},
				)
			}

		} else {
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
	return nil
}

func (*auditLogAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
