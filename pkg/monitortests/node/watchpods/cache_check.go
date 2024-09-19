package watchpods

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
)

func checkCacheState(ctx context.Context, kubeClient kubernetes.Interface, podInformer coreinformers.PodInformer) ([]string, error) {
	// We are running into problems where the cached list of pods contains pods that don't exist in the kube-apiserver.
	// Let's try to check exactly.
	cachedPods, err := podInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error listing cached pods: %w", err)
	}

	return checkCacheStateFromList(ctx, kubeClient, cachedPods, 0)
}

func checkCacheStateFromList(ctx context.Context, kubeClient kubernetes.Interface, cachedPods []*corev1.Pod, actualResourceVersion int) ([]string, error) {
	if len(cachedPods) == 0 {
		return nil, fmt.Errorf("somehow the lister is missing pods")
	}

	// if we have a real resourceVersion we can enforce different tests
	hasActualResourceVersion := actualResourceVersion > 0

	biggestResourceVersion := -1
	if hasActualResourceVersion {
		biggestResourceVersion = actualResourceVersion
	} else {
		for _, cachedPod := range cachedPods {
			if len(cachedPod.ResourceVersion) == 0 {
				return nil, fmt.Errorf("encounted cached pod/%s -n %s without a resourceversion", cachedPod.Namespace, cachedPod.Name)
			}
			currRV, err := strconv.Atoi(cachedPod.ResourceVersion)
			if err != nil {
				return nil, fmt.Errorf("cached pod/%s -n %s has a non-integer RV %q: %w", cachedPod.Namespace, cachedPod.Name, cachedPod.ResourceVersion, err)
			}
			if currRV > biggestResourceVersion {
				biggestResourceVersion = currRV
			}
		}
	}

	livePodsAtSameRV, err := kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		ResourceVersion:      strconv.Itoa(biggestResourceVersion),
		ResourceVersionMatch: metav1.ResourceVersionMatchExact,
	})
	if err != nil && strings.Contains(err.Error(), "The resourceVersion for the provided list is too old") {
		// if the resourceversion is too old (this happened a bunch), just skip the test.
		// Again, we'll write a better test with a reflector that isn't vulnerable to this, but to gain some insight early we'll skip this case.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error listing pods resourceVersion=%v: %w", biggestResourceVersion, err)
	}

	failures := []string{}
	for _, livePod := range livePodsAtSameRV.Items {
		found := false
		for _, cachedPod := range cachedPods {
			if cachedPod.Namespace == livePod.Namespace && cachedPod.Name == livePod.Name {
				found = true
				if cachedPod.ResourceVersion != livePod.ResourceVersion {
					failures = append(failures, fmt.Sprintf("matching pod/%s -n %s has different resourceVersions: live=%v cached=%v", livePod.Name, livePod.Namespace, livePod.ResourceVersion, cachedPod.ResourceVersion))
				}
			}
		}
		if !found {
			// We cannot fail in this case unless we have an actual resourceVersion because of a scenario like this
			/*
				rv-1: create a
				rv-2: create b
				rv-3: create c
				rv-4: delete a
				rv-5: delete b

				In this scenario, the biggest RV is 3.  When we live list RV 3, the live list has [a, b, c].  The cache only has [c].
				To get this test off the ground and check the case where the cache has more items than the live list (the current failure
				we're checking), we will exclude this case from the inital check.
			*/
			// if we have the actual resoruceversion, then we can fail in this case.
			if hasActualResourceVersion {
				failures = append(failures, "live pod/%s -n %s RV=%v is not present in the cached results", livePod.Name, livePod.Namespace, livePod.ResourceVersion)
			}
		}
	}
	for _, cachedPod := range cachedPods {
		found := false
		for _, livePod := range livePodsAtSameRV.Items {
			if cachedPod.Namespace == livePod.Namespace && cachedPod.Name == livePod.Name {
				found = true
			}
		}
		if !found {
			failures = append(failures, fmt.Sprintf("cached pod/%s -n %s RV=%v is not present in the live results", cachedPod.Name, cachedPod.Namespace, cachedPod.ResourceVersion))
		}
	}

	return failures, nil
}
