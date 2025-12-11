package dr

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	librarygov1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Disruptive] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-static-pod-rollouts").AsAdmin()

	etcdNamespace := "openshift-etcd"
	var initialEtcdPodCount int   // the number of etcd pods in the initial state of the cluster
	var etcdTargetNodeName string // node from which the etcd pod is to be removed
	var initialLogLevel string

	g.BeforeEach(func() {
		isSingleNode, err := exutil.IsSingleNode(context.Background(), oc.AdminConfigClient())
		o.Expect(err).ToNot(o.HaveOccurred())
		if isSingleNode {
			g.Skip("the test is for etcd peer communication which is not valid for single node")
		}

		etcdCluster, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		initialLogLevel = string(etcdCluster.Spec.LogLevel)
		o.Expect(initialLogLevel).ToNot(o.Equal("Debug"), "log level is not different from Debug, which is the change being made to trigger a rollout")

		isUnsupportedUnsafeEtcd, err := isUnsupportedUnsafeEtcd(&etcdCluster.Spec.StaticPodOperatorSpec)
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(isUnsupportedUnsafeEtcd).To(o.BeFalse(), "quorum guard must not be off, expected useUnsupportedUnsafeNonHANonProductionUnstableEtcd: false")

		etcdPods, err := e2epod.GetPods(context.Background(), oc.AdminKubeClient(), etcdNamespace, map[string]string{"app": "etcd"})
		o.Expect(err).ToNot(o.HaveOccurred())
		initialEtcdPodCount = len(etcdPods)

		masterNodes := masterNodes(oc)
		etcdTargetNodeName = masterNodes[0].Name
	})

	g.AfterEach(func(ctx context.Context) {

		// Reset the log level back to initial log level to leave the cluster in the same state.
		g.GinkgoT().Log("resetting the log level back to intial log level")
		data := fmt.Sprintf(`{"spec": {"logLevel": "%s"}}`, initialLogLevel)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Logf("debugging into the same node %s to add back the etcd static pod manifest", etcdTargetNodeName)
		err = oc.AsAdmin().Run("debug").Args("-n", etcdNamespace, "node/"+etcdTargetNodeName, "--", "chroot", "/host", "/bin/bash", "-c", "mv /var/lib/etcd-backup/etcd-pod.yaml /etc/kubernetes/manifests/").Execute()
		o.Expect(err).ToNot(o.HaveOccurred(), fmt.Sprintf("failed to add etcd static pod back to the node %s", etcdTargetNodeName))

		g.GinkgoT().Log("waiting for all the etcd instances to be available to return the cluster to its original state")
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), initialEtcdPodCount)

	})

	g.It("is able to block the rollout of a revision when the quorum is not safe", g.Label("Size:L"), func(ctx context.Context) {
		var err error

		g.GinkgoT().Logf("debugging into node %s to remove the etcd static pod manifest", etcdTargetNodeName)
		err = oc.AsAdmin().Run("debug").Args("-n", etcdNamespace, "node/"+etcdTargetNodeName, "--", "chroot", "/host", "/bin/bash", "-c", "mkdir -p /var/lib/etcd-backup && mv /etc/kubernetes/manifests/etcd-pod.yaml /var/lib/etcd-backup").Execute()
		err = errors.Wrapf(err, "failed to remove etcd static pod from the node %s", etcdTargetNodeName)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for the etcd pod to be removed")
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), initialEtcdPodCount-1)

		g.GinkgoT().Log("ensuring the EtcdMembersDegraded condition type reports True")
		o.Expect(wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			// retrieve the operator status
			etcdCluster, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				klog.Errorf("error while getting operator status: %v", err)
				return false, nil
			}

			if isConditionTrue := librarygov1helpers.IsOperatorConditionTrue(etcdCluster.Status.Conditions, "EtcdMembersDegraded"); !isConditionTrue {
				return false, nil
			}
			return true, nil
		})).ToNot(o.HaveOccurred(), "expected the EtcdMembersDegraded condition type to report True")

		// getting the count of installer pods before triggering a rollout
		installerPodsPreRolloutTrigger, err := e2epod.GetPods(ctx, oc.AdminKubeClient(), etcdNamespace, map[string]string{"app": "installer"})
		o.Expect(err).ToNot(o.HaveOccurred())

		// getting the LatestAvailableRevision before triggering a rollout
		etcdCluster, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		latestAvailableRevisionPreRolloutTrigger := etcdCluster.Status.LatestAvailableRevision

		g.GinkgoT().Log("setting the log level to DEBUG to trigger a revision rollout")
		data := fmt.Sprintf(`{"spec": {"logLevel": "Debug"}}`)
		_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("ensuring that triggering a rollout doesn't change the LatestAvailableRevision") //this would also allow some time after triggering a rollout before observing the installer pods
		o.Expect(wait.PollUntilContextTimeout(ctx, 10*time.Second, 1*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			// retrieve the operator status
			etcdCluster, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(context.Background(), "cluster", metav1.GetOptions{})
			if err != nil {
				klog.Errorf("error while getting operator status: %v", err)
				return false, nil
			}

			if latestAvailableRevisionPreRolloutTrigger != etcdCluster.Status.LatestAvailableRevision {
				return true, nil
			}
			return false, nil
		})).To(o.HaveOccurred(), "LatestAvailableRevision shouldn't change as the rollout of a new revision is blocked due to insufficient quorum")

		g.GinkgoT().Log("ensuring that triggering a rollout doesn't create any new installer pods due to insufficient quorum")
		installerPodsPostRolloutTrigger, err := e2epod.GetPods(ctx, oc.AdminKubeClient(), etcdNamespace, map[string]string{"app": "installer"})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(len(installerPodsPostRolloutTrigger)).To(o.Equal(len(installerPodsPreRolloutTrigger)), "triggering a rollout shouldn't create any new installer pods due to insufficient quorum")

	})
})
