// Package apis provides utilities for interacting with OpenShift API resources.
//
// This file handles job-related operations including pod log extraction.
package apis

import (
	"context"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// dumpJobPodLogsTimeout caps List (and avoids hanging if the API is wedged during log capture).
	dumpJobPodLogsTimeout = 30 * time.Second
)

// DumpJobPodLogs lists pods for the given job (label job-name=jobName), then dumps each pod's
// container logs into the test output. Call this when a job completes so logs are captured even
// if the node is later unavailable (e.g. must-gather cannot collect from a dead node).
func DumpJobPodLogs(jobName, namespace string, oc *exutil.CLI) {
	ctx, cancel := context.WithTimeout(context.Background(), dumpJobPodLogsTimeout)
	defer cancel()
	selector := "job-name=" + jobName
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		e2e.Logf("Failed to list pods for job %s: %v", jobName, err)
		return
	}
	if len(podList.Items) == 0 {
		e2e.Logf("No pods found for job %s (selector %q)", jobName, selector)
		return
	}
	e2e.Logf("Capturing container logs for job %s (%d pod(s))", jobName, len(podList.Items))
	exutil.DumpPodLogs(podList.Items, oc)
}
