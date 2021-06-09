package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/client-go/tools/leaderelection/resourcelock"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-arch] Leases", func() {
	defer g.GinkgoRecover()

	g.It("should be able to span 60s kube-apiserver disruption", func() {
		ctx := context.Background()

		kubeClient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		shortLeases := []string{}

		configMaps, err := kubeClient.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, configmap := range configMaps.Items {
			leaderElection, ok := configmap.Annotations[resourcelock.LeaderElectionRecordAnnotationKey]
			if !ok {
				continue
			}
			leaderElectionRecord := &resourcelock.LeaderElectionRecord{}
			if err := json.Unmarshal([]byte(leaderElection), leaderElectionRecord); err != nil {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if leaderElectionRecord.LeaseDurationSeconds < 107 {
				shortLeases = append(shortLeases, fmt.Sprintf("configmap/%s used by %q, has too short a lease (%d) to span 60s kube-apiserver disruption.  Try 107s leaseDuration with 13s retryPeriod and a 85s renewDeadline.  Be sure you have the graceful release properly wired.", configmap.Name, leaderElectionRecord.HolderIdentity, leaderElectionRecord.LeaseDurationSeconds))
			}
		}

		endpoints, err := kubeClient.CoreV1().Endpoints("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, endpoint := range endpoints.Items {
			leaderElection, ok := endpoint.Annotations[resourcelock.LeaderElectionRecordAnnotationKey]
			if !ok {
				continue
			}
			leaderElectionRecord := &resourcelock.LeaderElectionRecord{}
			if err := json.Unmarshal([]byte(leaderElection), leaderElectionRecord); err != nil {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if leaderElectionRecord.LeaseDurationSeconds < 107 {
				shortLeases = append(shortLeases, fmt.Sprintf("endpoint/%s used by %q, has too short a lease (%d) to span 60s kube-apiserver disruption.  Try 107s leaseDuration with 13s retryPeriod and a 85s renewDeadline.  Be sure you have the graceful release properly wired.", endpoint.Name, leaderElectionRecord.HolderIdentity, leaderElectionRecord.LeaseDurationSeconds))
			}
		}

		leases, err := kubeClient.CoordinationV1().Leases("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, lease := range leases.Items {
			if lease.Spec.LeaseDurationSeconds != nil && *lease.Spec.LeaseDurationSeconds < 107 {
				identity := "MISSING"
				if lease.Spec.HolderIdentity != nil {
					identity = *lease.Spec.HolderIdentity
				}
				shortLeases = append(shortLeases, fmt.Sprintf("lease/%s used by %q, has too short a lease (%d) to span 60s kube-apiserver disruption.  Try 107s leaseDuration with 13s retryPeriod and a 85s renewDeadline.  Be sure you have the graceful release properly wired.", lease.Name, identity, *lease.Spec.LeaseDurationSeconds))
			}
		}

		if len(shortLeases) > 0 {
			//if suppressPreTestFailure {
			//	result.Flakef("pre-test environment had disruption and limited this test, suppressing failure: %s", failMessage)
			//} else {
			g.Fail(strings.Join(shortLeases, "\n"))
			//}
		}
	})
})
