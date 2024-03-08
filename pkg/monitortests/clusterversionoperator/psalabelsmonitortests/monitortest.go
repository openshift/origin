package psalabelsmonitortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	psapi "k8s.io/pod-security-admission/api"
)

type namespacePSaLabelsChecker struct {
	kubeClient kubernetes.Interface
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &namespacePSaLabelsChecker{}
}

func (w *namespacePSaLabelsChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

func (w *namespacePSaLabelsChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.kubeClient == nil {
		return nil, nil, nil
	}

	namespaces, err := w.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	junits := []*junitapi.JUnitTestCase{}
	for _, ns := range namespaces.Items {
		if !strings.HasPrefix(ns.Name, "openshift") {
			continue
		}

		failureMessages := make([]string, 0)
		for _, requiredLabel := range []string{psapi.EnforceLevelLabel, psapi.AuditLevelLabel, psapi.WarnLevelLabel} {
			if len(ns.Labels[requiredLabel]) == 0 {
				failureMessages = append(failureMessages, fmt.Sprintf("missing label '%s'", requiredLabel))
			}
		}

		junit := &junitapi.JUnitTestCase{Name: fmt.Sprintf("ns/%s must have all PSa labels defined", ns.Name)}
		if len(failureMessages) > 0 {
			msg := strings.Join(failureMessages, "\n")
			junit.SystemOut = msg
			junit.FailureOutput = &junitapi.FailureOutput{
				Output: msg,
			}
		}

		junits = append(junits, junit)
	}

	return nil, junits, nil
}

func (w *namespacePSaLabelsChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *namespacePSaLabelsChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *namespacePSaLabelsChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *namespacePSaLabelsChecker) Cleanup(ctx context.Context) error {
	return nil
}
