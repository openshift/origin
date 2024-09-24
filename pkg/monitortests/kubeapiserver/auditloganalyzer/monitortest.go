package auditloganalyzer

import (
	"context"
	"fmt"
	"os"
	"path"
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

	return nil, nil, err
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
		csvContent, err := w.requestCountTracking.CountsForRun.ToCSV()
		if err != nil {
			return err
		}
		requestCountsByTimeFile := path.Join(storageDir, fmt.Sprintf("request-counts-by-second_%s.csv", timeSuffix))
		if err := os.WriteFile(requestCountsByTimeFile, csvContent, 0644); err != nil {
			return err
		}
	}

	return nil
}

func (*auditLogAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
