package dr

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
)

// Enables forcible redeployment of etcd, kube-apiserver,
// kube-controller-manager, kube-scheduler operands. This is a
// necessary part of restoring a cluster from backup.

const (
	redeployWaitInterval = 5 * time.Second
	redeployWaitTimeout  = 2 * time.Minute
)

// operatorConfigClient supports patching and retrieving the status of
// an operator's 'cluster' config resource to support triggering
// redeployment and watching for a successful rollout.
type operatorConfigClient struct {
	name      string
	patch     func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) error
	getStatus func(ctx context.Context, name string, opts metav1.GetOptions) (*operatorv1.StaticPodOperatorStatus, error)
}

func (c *operatorConfigClient) String() string {
	return c.name
}

// forceOperandRedeployment forces the redeployment the etcd,
// kube-apiserver, kube-controller-manager and kube-scheduler operands
// (in that order).  Only when an operand has been successfully rolled
// out will redeployment of the subsequent operand be attempted.
func forceOperandRedeployment(client operatorv1client.OperatorV1Interface) {
	clients := []*operatorConfigClient{
		{
			name: "etcd",
			patch: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) error {
				_, err := client.Etcds().Patch(ctx, name, pt, data, opts)
				return err
			},
			getStatus: func(ctx context.Context, name string, opts metav1.GetOptions) (*operatorv1.StaticPodOperatorStatus, error) {
				obj, err := client.Etcds().Get(ctx, name, opts)
				if err != nil {
					return nil, err
				}
				return &obj.Status.StaticPodOperatorStatus, nil
			},
		},
		{
			name: "kube-apiserver",
			patch: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) error {
				_, err := client.KubeAPIServers().Patch(ctx, name, pt, data, opts)
				return err
			},
			getStatus: func(ctx context.Context, name string, opts metav1.GetOptions) (*operatorv1.StaticPodOperatorStatus, error) {
				obj, err := client.KubeAPIServers().Get(ctx, name, opts)
				if err != nil {
					return nil, err
				}
				return &obj.Status.StaticPodOperatorStatus, nil
			},
		},
		{
			name: "kube-controller-manager",
			patch: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) error {
				_, err := client.KubeControllerManagers().Patch(ctx, name, pt, data, opts)
				return err
			},
			getStatus: func(ctx context.Context, name string, opts metav1.GetOptions) (*operatorv1.StaticPodOperatorStatus, error) {
				obj, err := client.KubeControllerManagers().Get(ctx, name, opts)
				if err != nil {
					return nil, err
				}
				return &obj.Status.StaticPodOperatorStatus, nil
			},
		},
		{
			name: "kube-scheduler",
			patch: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) error {
				_, err := client.KubeSchedulers().Patch(ctx, name, pt, data, opts)
				return err
			},
			getStatus: func(ctx context.Context, name string, opts metav1.GetOptions) (*operatorv1.StaticPodOperatorStatus, error) {
				obj, err := client.KubeSchedulers().Get(ctx, name, opts)
				if err != nil {
					return nil, err
				}
				return &obj.Status.StaticPodOperatorStatus, nil
			},
		},
	}
	for _, client := range clients {
		forceRedeployOperand(client)
	}
}

// forceRedeployOperand initiates redeployment of an operand and waits for a
// successful rollout.
func forceRedeployOperand(client *operatorConfigClient) {
	// Retrieve the LatestAvailableRevision before rolling out to know
	// what revision not to look for in the subsequent check for
	// rollout success.
	g.By(fmt.Sprintf("Finding LatestAvailableRevision for %s", client))
	var latestAvailableRevision int32
	err := wait.PollImmediate(redeployWaitInterval, redeployWaitTimeout, func() (done bool, err error) {
		status, err := client.getStatus(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error retrieving %s operator status: %v", client, err)
		} else {
			latestAvailableRevision = status.LatestAvailableRevision
		}
		return err == nil, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("LatestAvailableRevision for %s is %d", client, latestAvailableRevision)

	// Ensure a unique forceRedeploymentReason for each test run to
	// ensure rollout is always triggered even if running repeatedly
	// against the same cluster (as when debugging).
	reason := fmt.Sprintf("e2e-cluster-restore-%s", uuid.NewUUID())

	g.By(fmt.Sprintf("Forcing redeployment of %s", client))
	data := fmt.Sprintf(`{"spec": {"forceRedeploymentReason": "%s"}}`, reason)
	err = wait.PollImmediate(redeployWaitInterval, redeployWaitTimeout, func() (done bool, err error) {
		err = client.patch(context.Background(), "cluster", types.MergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			framework.Logf("Error patching %s operator status to set redeploy reason: %v", client, err)
		}
		return err == nil, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("Waiting for %s to be updated on all nodes to a revision greater than %d", client, latestAvailableRevision))
	waitForRollout(client, latestAvailableRevision)
	framework.Logf("Rollout complete for %s", client)
}

// waitForRollout waits for an operator status to indicate that all nodes are
// at a revision greater than that provided.
func waitForRollout(client *operatorConfigClient, previousRevision int32) {
	// Need to wait as long as 15 minutes for rollout of kube apiserver
	err := wait.PollImmediate(redeployWaitInterval, 15*time.Minute, func() (done bool, err error) {
		status, err := client.getStatus(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error retrieving %s operator status: %v", client, err)
			return false, nil
		}
		rolloutComplete := false
		for _, condition := range status.Conditions {
			if condition.Type == "NodeInstallerProgressing" {
				rolloutComplete = condition.Reason == "AllNodesAtLatestRevision" && condition.Status == operatorv1.ConditionFalse
				break
			}
		}
		if !rolloutComplete {
			return false, nil
		}
		// Prevent timing issues by ensuring that the revision of all nodes is
		// greater than the revision observed before rollout was initiated.
		for _, nodeStatus := range status.NodeStatuses {
			if nodeStatus.CurrentRevision == previousRevision {
				return false, nil
			}
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}
