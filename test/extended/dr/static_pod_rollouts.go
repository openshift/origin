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
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Disruptive] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-static-pod-rollouts").AsAdmin()

	etcdNamespace := "openshift-etcd"
	var initialEtcdPodCount int   // the number of etcd pods in the initial state of the cluster
	var etcdTargetNodeName string // node from which the etcd pod is to be removed

	g.BeforeEach(func() {
		isSingleNode, err := exutil.IsSingleNode(context.Background(), oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSingleNode {
			g.Skip("the test is for etcd peer communication which is not valid for single node")
		}

		etcdPods, err := e2epod.GetPods(context.Background(), oc.AdminKubeClient(), etcdNamespace, map[string]string{"app": "etcd"})
		o.Expect(err).ToNot(o.HaveOccurred())
		initialEtcdPodCount = len(etcdPods)

		masterNodes := masterNodes(oc)
		etcdTargetNodeName = masterNodes[0].Name
	})

	g.AfterEach(func(ctx context.Context) {

		// Reset the hardware speed back to default to leave the cluster in the same state.
		g.GinkgoT().Log("resetting the hardware speed back to default")
		data := fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": ""}}`)
		_, err := oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("debugging into the same node to add back the etcd static pod manifest")
		err = oc.AsAdmin().Run("debug").Args("-n", etcdNamespace, "node/"+etcdTargetNodeName, "--", "chroot", "/host", "/bin/bash", "-c", "mv /var/lib/etcd-backup/etcd-pod.yaml /etc/kubernetes/manifests/").Execute()
		o.Expect(err).ToNot(o.HaveOccurred(), "failed to add etcd static pod back to the node")

		g.GinkgoT().Log("waiting for all the etcd instances to be available to return the cluster to its original state")
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), initialEtcdPodCount)

	})

	g.It("is able to block the rollout of a revision when the quorum is not safe", func(ctx context.Context) {
		var err error

		g.GinkgoT().Log("debugging into the node to remove the etcd static pod manifest")
		err = oc.AsAdmin().Run("debug").Args("-n", etcdNamespace, "node/"+etcdTargetNodeName, "--", "chroot", "/host", "/bin/bash", "-c", "mkdir -p /var/lib/etcd-backup && mv /etc/kubernetes/manifests/etcd-pod.yaml /var/lib/etcd-backup").Execute()
		err = errors.Wrap(err, "failed to remove etcd static pod from the node")
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("waiting for the etcd pod to be removed")
		waitForReadyEtcdStaticPods(oc.AdminKubeClient(), initialEtcdPodCount-1)

		g.GinkgoT().Log("ensuring the quorum guard dependent controllers have degraded")
		conditionTypes := []string{
			"EtcdCertSignerControllerDegraded",
			"EtcdEndpointsDegraded",
			// "TargetConfigControllerDegraded",
		}
		o.Expect(wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			// retrieve the operator status
			etcdCluster, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				return true, err
			}

			for _, conditionType := range conditionTypes {
				isConditionTrue := librarygov1helpers.IsOperatorConditionTrue(etcdCluster.Status.Conditions, conditionType)
				if !isConditionTrue {
					return false, nil
				}
			}
			return true, nil
		})).ToNot(o.HaveOccurred(), "expected the quorum guard dependent controllers to be degraded")

		// getting the count of installer pods before triggering a rollout
		installerPodsPreRolloutTrigger, err := e2epod.GetPods(ctx, oc.AdminKubeClient(), etcdNamespace, map[string]string{"app": "installer"})
		o.Expect(err).ToNot(o.HaveOccurred())

		g.GinkgoT().Log("setting the hardware speed to Slower to trigger a revision rollout")
		data := fmt.Sprintf(`{"spec": {"controlPlaneHardwareSpeed": "Slower"}}`)
		_, err = oc.AdminOperatorClient().OperatorV1().Etcds().Patch(ctx, "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		// allow some time after triggering a rollout before observing the changes in the cluster
		time.Sleep(60 * time.Second)

		g.GinkgoT().Log("ensuring that triggering a rollout doesn't create any new installer pods due to insufficient quorum")
		installerPodsPostRolloutTrigger, err := e2epod.GetPods(ctx, oc.AdminKubeClient(), etcdNamespace, map[string]string{"app": "installer"})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(len(installerPodsPostRolloutTrigger)).To(o.Equal(len(installerPodsPreRolloutTrigger)), "triggering a rollout should not create any new installer pods due to insufficient quorum")

	})
})
