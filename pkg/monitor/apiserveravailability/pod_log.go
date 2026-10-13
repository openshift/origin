package apiserveravailability

import (
	"bufio"
	"context"
	"sort"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func APIServerAvailabilityIntervalsFromCluster(kubeClient kubernetes.Interface, beginning, end time.Time) (monitorapi.Intervals, error) {
	summary, err := SummarizeInterestingPodLogs(context.TODO(), kubeClient)
	if err != nil {
		return nil, err
	}

	allIntervals := monitorapi.Intervals{}
	// as more are added, append
	allIntervals = append(allIntervals, summary.WriteOperationFailures...)

	if beginning.IsZero() || end.IsZero() {
		return allIntervals, nil
	}

	timeFiltered := monitorapi.Intervals{}
	for i := range allIntervals {
		interval := allIntervals[i]
		if interval.From.After(end) || interval.To.Before(beginning) {
			continue
		}
		timeFiltered = append(timeFiltered, interval)
	}

	sort.Sort(timeFiltered)
	return timeFiltered, nil
}

var interestingNamespaces = []string{"openshift-cluster-version"}

func SummarizeInterestingPodLogs(ctx context.Context, client kubernetes.Interface) (*APIServerClientAccessFailureSummary, error) {
	summary := &APIServerClientAccessFailureSummary{}
	wg := sync.WaitGroup{}
	for _, namespace := range interestingNamespaces {
		wg.Add(1)
		go func(ctx context.Context, namespace string) {
			defer wg.Done()

			currSummary, err := summarizeLogsForNamespace(ctx, client, namespace)
			if err != nil {
				// ignore error and continue
				klog.Warningf("error summarizing pod logs: %v", err)
			}
			summary.AddSummary(currSummary)
		}(ctx, namespace)
	}
	wg.Wait()

	return summary, nil
}

func summarizeLogsForNamespace(ctx context.Context, client kubernetes.Interface, namespaceName string) (*APIServerClientAccessFailureSummary, error) {
	podList, err := client.CoreV1().Pods(namespaceName).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	summary := &APIServerClientAccessFailureSummary{}
	wg := sync.WaitGroup{}
	for i := range podList.Items {
		pod := podList.Items[i]

		wg.Add(1)
		go func(ctx context.Context, pod *corev1.Pod) {
			defer wg.Done()

			currSummary, err := summarizeLogsForPod(ctx, client, pod)
			if err != nil {
				// ignore error and continue
				klog.Warningf("error summarizing pod logs: %v", err)
			}
			summary.AddSummary(currSummary)
		}(ctx, &pod)
	}
	wg.Wait()

	return summary, nil
}

func summarizeLogsForPod(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) (*APIServerClientAccessFailureSummary, error) {
	pod, err := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	containerNames := []string{}
	for _, container := range pod.Spec.InitContainers {
		containerNames = append(containerNames, container.Name)
	}
	for _, container := range pod.Spec.Containers {
		containerNames = append(containerNames, container.Name)
	}

	summary := &APIServerClientAccessFailureSummary{}
	wg := sync.WaitGroup{}
	for _, previousContainer := range []bool{true, false} {
		for _, containerName := range containerNames {
			wg.Add(1)
			go func(ctx context.Context, previousContainer bool, containerName string) {
				defer wg.Done()

				locator := monitorapi.NewLocator().ContainerFromPod(pod, containerName)
				currSummary := &APIServerClientAccessFailureSummary{}
				logOptions := &corev1.PodLogOptions{
					Container:                    containerName,
					Follow:                       false,
					Previous:                     previousContainer,
					Timestamps:                   true,
					InsecureSkipTLSVerifyBackend: true, // set in case the kubelet doesn't have valid serving certs
				}
				req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOptions)
				logStream, err := req.Stream(ctx)
				if err != nil {
					// ignore error and continue
					klog.Warningf("error summarizing pod logs: %v", err)
					return
				}

				scanner := bufio.NewScanner(logStream)
				for scanner.Scan() {
					line := scanner.Bytes()
					if len(line) == 0 {
						continue
					}
					currSummary.SummarizeLine(locator, string(line))
				}
				summary.AddSummary(currSummary)
			}(ctx, previousContainer, containerName)
		}
	}
	wg.Wait()

	return summary, nil
}
