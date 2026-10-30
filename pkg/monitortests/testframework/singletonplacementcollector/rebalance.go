package singletonplacementcollector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	rebalanceRounds       = 3
	rebalancePollInterval = 5 * time.Second
	rebalanceRoundTimeout = 5 * time.Minute
)

// RebalanceSingletons is an experiment for OCPBUGS-131892. Singletons pile up on whichever master
// was available first during install, or are left off the last master rebooted during an upgrade,
// and nothing moves them afterwards. Once every master is up, deleting the excess pods from
// overloaded masters lets the scheduler spread the replacements: their memory requests are well
// below real usage, but they are low by a similar factor on every master, so the request totals
// still show which master is overloaded.
// It should run before monitor tests start so the disruption it causes is not measured.
func RebalanceSingletons(ctx context.Context, restConfig *rest.Config) error {
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	return rebalanceSingletons(ctx, kubeClient, rebalanceRounds, rebalancePollInterval, rebalanceRoundTimeout)
}

func rebalanceSingletons(ctx context.Context, kubeClient kubernetes.Interface, rounds int, pollInterval, roundTimeout time.Duration) error {
	logger := logrus.WithField("func", "RebalanceSingletons")

	for round := 1; round <= rounds; round++ {
		if notReady, err := unschedulableMasters(ctx, kubeClient); err != nil {
			return err
		} else if len(notReady) > 0 {
			logger.Infof("skipping singleton rebalance, masters not ready or schedulable: %s", strings.Join(notReady, ", "))
			return nil
		}

		masters, pods, err := listSingletonPods(ctx, kubeClient, time.Now())
		if err != nil {
			return err
		}
		if len(masters) < 2 {
			logger.Infof("found %d master nodes, skipping singleton rebalance", len(masters))
			return nil
		}
		logger.Infof("round %d singleton placement: %s", round, formatPlacement(masters, pods))

		toDelete := podsToRebalance(masters, pods)
		if len(toDelete) == 0 {
			logger.Infof("singletons are balanced across masters after %d round(s)", round-1)
			return nil
		}

		for _, pod := range toDelete {
			logger.Infof("deleting singleton pod %s/%s from %s", pod.namespace, pod.podName, pod.nodeName)
			err := kubeClient.CoreV1().Pods(pod.namespace).Delete(ctx, pod.podName, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete pod %s/%s: %w", pod.namespace, pod.podName, err)
			}
		}

		if err := waitForReplacements(ctx, kubeClient, toDelete, pollInterval, roundTimeout); err != nil {
			return err
		}
	}

	masters, pods, err := listSingletonPods(ctx, kubeClient, time.Now())
	if err != nil {
		return err
	}
	logger.Infof("singleton placement after %d rebalance rounds: %s", rounds, formatPlacement(masters, pods))
	return nil
}

// unschedulableMasters returns the masters that are not Ready or are cordoned. Rebalancing onto a
// cluster that is still rolling masters would just recreate the skew.
func unschedulableMasters(ctx context.Context, kubeClient kubernetes.Interface) ([]string, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	var ret []string
	for _, node := range nodes.Items {
		if !isMaster(&node) {
			continue
		}
		if node.Spec.Unschedulable || !isNodeReady(&node) {
			ret = append(ret, node.Name)
		}
	}
	sort.Strings(ret)
	return ret, nil
}

func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// podsToRebalance picks the pods to delete so that no master keeps more than its even share
// (rounded up). Pods with the largest memory requests go first since requests are what the
// scheduler balances on.
func podsToRebalance(masters []string, pods []singletonPod) []singletonPod {
	if len(masters) == 0 {
		return nil
	}
	target := (len(pods) + len(masters) - 1) / len(masters)

	byNode := map[string][]singletonPod{}
	for _, pod := range pods {
		byNode[pod.nodeName] = append(byNode[pod.nodeName], pod)
	}
	var ret []singletonPod
	for _, name := range masters {
		nodePods := byNode[name]
		if len(nodePods) <= target {
			continue
		}
		sort.Slice(nodePods, func(i, j int) bool {
			if nodePods[i].memoryRequestBytes != nodePods[j].memoryRequestBytes {
				return nodePods[i].memoryRequestBytes > nodePods[j].memoryRequestBytes
			}
			return nodePods[i].namespace+"/"+nodePods[i].podName < nodePods[j].namespace+"/"+nodePods[j].podName
		})
		ret = append(ret, nodePods[:len(nodePods)-target]...)
	}
	return ret
}

// waitForReplacements waits until every deleted pod is gone and its Deployment is back to one
// ready, available, up to date replica.
func waitForReplacements(ctx context.Context, kubeClient kubernetes.Interface, deleted []singletonPod, pollInterval, timeout time.Duration) error {
	var pending []string
	err := wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		pending = nil
		for _, pod := range deleted {
			_, err := kubeClient.CoreV1().Pods(pod.namespace).Get(ctx, pod.podName, metav1.GetOptions{})
			if err == nil {
				pending = append(pending, fmt.Sprintf("%s/%s terminating", pod.namespace, pod.podName))
				continue
			} else if !apierrors.IsNotFound(err) {
				return false, err
			}

			d, err := kubeClient.AppsV1().Deployments(pod.namespace).Get(ctx, pod.deployment, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			s := d.Status
			if s.ObservedGeneration < d.Generation || s.Replicas != 1 || s.UpdatedReplicas != 1 || s.ReadyReplicas != 1 || s.AvailableReplicas != 1 {
				pending = append(pending, fmt.Sprintf("deployment %s/%s not ready", pod.namespace, pod.deployment))
			}
		}
		return len(pending) == 0, nil
	})
	if err != nil {
		return fmt.Errorf("singletons did not recover after deletion: %w: %s", err, strings.Join(pending, ", "))
	}
	return nil
}

func formatPlacement(masters []string, pods []singletonPod) string {
	counts := map[string]int{}
	for _, pod := range pods {
		counts[pod.nodeName]++
	}
	parts := make([]string, 0, len(masters))
	for _, name := range masters {
		parts = append(parts, fmt.Sprintf("%s=%d", name, counts[name]))
	}
	return strings.Join(parts, " ")
}
