package dr

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/library-go/test/library"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const certRotationTimeout = 60 * time.Minute

func waitForEtcdToStabilizeOnTheSameRevisionLonger(ctx context.Context, logger library.LoggingT, oc *exutil.CLI) error {
	return waitForCertRotationPods(ctx, logger, oc.AdminKubeClient().CoreV1().Pods(openshiftEtcdNamespace), "app=etcd")
}

func waitForApiServerToStabilizeOnTheSameRevisionLonger(ctx context.Context, logger library.LoggingT, oc *exutil.CLI) error {
	return waitForCertRotationPods(ctx, logger, oc.AdminKubeClient().CoreV1().Pods("openshift-kube-apiserver"), "apiserver=true")
}

func waitForCertRotationPods(ctx context.Context, logger library.LoggingT, pods corev1client.PodInterface, selector string) error {
	ctx, cancel := context.WithTimeout(ctx, certRotationTimeout)
	defer cancel()
	return wait.PollUntilContextCancel(ctx, 10*time.Second, false, certRotationPodsStable(logger, pods, selector, time.Now))
}

func certRotationPodsStable(logger library.LoggingT, pods corev1client.PodInterface, selector string, now func() time.Time) wait.ConditionWithContextFunc {
	nextObservation := now().Add(time.Minute)
	successes := 0
	lastRevision := ""
	return func(ctx context.Context) (bool, error) {
		if now().Before(nextObservation) {
			return false, nil
		}
		requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		podList, err := pods.List(requestCtx, metav1.ListOptions{LabelSelector: selector})
		nextObservation = now().Add(time.Minute)
		if err != nil {
			logger.Logf("failed to list pods, err = %v (this error will be retried)", err)
			successes = 0
			return false, ctx.Err()
		}
		revision, ready, err := certRotationPodRevision(podList.Items)
		if err != nil || !ready {
			successes = 0
			return false, err
		}
		if revision != lastRevision {
			successes = 0
			lastRevision = revision
		}
		successes++
		return successes >= 3, nil
	}
}

func certRotationPodRevision(pods []corev1.Pod) (string, bool, error) {
	if len(pods) == 0 {
		return "", false, nil
	}
	goodRevisions, badRevisions := sets.New[string](), sets.New[string]()
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			ready := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					ready = true
				}
			}
			if !ready {
				return "", false, nil
			}
			goodRevisions.Insert(pod.Labels["revision"])
		case corev1.PodPending:
			return "", false, nil
		case corev1.PodSucceeded, corev1.PodFailed:
			badRevisions.Insert(pod.Labels["revision"])
		default:
			return "", false, fmt.Errorf("pod %s has unexpected phase %v", pod.Name, pod.Status.Phase)
		}
	}
	if goodRevisions.Len() != 1 {
		return "", false, nil
	}
	revision, _ := goodRevisions.PopAny()
	if badRevisions.Has(revision) {
		return "", false, fmt.Errorf("revision %s has both running and failed pods", revision)
	}
	return revision, true, nil
}
