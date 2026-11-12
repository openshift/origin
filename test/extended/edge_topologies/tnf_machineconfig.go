package edge_topologies

import (
	"context"
	"fmt"
	"slices"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	etcdv1 "github.com/openshift/api/etcd/v1"
	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
)

var _ = g.Describe("[sig-etcd][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Disruptive] TNF MachineConfig rollout", func() {
	oc := exutil.NewCLI("tnf-machineconfig").AsAdmin()

	g.It("preserves Pacemaker membership through a reboot-required rollout on one node [apigroup:machineconfiguration.openshift.io]", func() {
		utils.SkipIfNotTopology(oc, configv1.DualReplicaTopologyMode)
		utils.SkipIfClusterIsNotHealthy(oc, helpers.NewEtcdClientFactory(oc.KubeClient()))
		nodes, err := utils.GetNodes(oc, utils.LabelNodeRoleControlPlane)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodes.Items).To(o.HaveLen(2))
		o.Expect(apis.ExpectPacemakerBaseline(oc)).To(o.Succeed())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		configs := oc.MachineConfigurationClient().MachineconfigurationV1()
		pool, err := configs.MachineConfigPools().Get(ctx, "master", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pool.Spec.Paused).To(o.BeFalse(), "master pool must start unpaused")
		originalConfig := pool.Spec.Configuration.Name
		o.Expect(originalConfig).NotTo(o.BeEmpty())
		for _, node := range nodes.Items {
			o.Expect(node.Status.NodeInfo.BootID).NotTo(o.BeEmpty())
			o.Expect(node.Annotations["machineconfiguration.openshift.io/currentConfig"]).To(o.Equal(originalConfig))
			o.Expect(node.Annotations["machineconfiguration.openshift.io/desiredConfig"]).To(o.Equal(originalConfig))
		}
		target := nodes.Items[1]
		mc := newMasterRebootRequiredMachineConfig("99-master-"+oc.Namespace(), "/etc/tnf-reboot-test", oc.Namespace()+"\n")
		waitForRender := func(present bool) string {
			var rendered string
			o.Eventually(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				pool, err := configs.MachineConfigPools().Get(ctx, "master", metav1.GetOptions{})
				if err != nil {
					return err
				}
				containsMC := slices.ContainsFunc(pool.Spec.Configuration.Source, func(ref corev1.ObjectReference) bool { return ref.Name == mc.Name })
				rendered = pool.Spec.Configuration.Name
				if containsMC != present || rendered == "" || (present && rendered == originalConfig) || (!present && rendered != originalConfig) {
					return fmt.Errorf("waiting for master config render (configuration=%s, test config present=%t)", rendered, containsMC)
				}
				return nil
			}, 5*time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
			return rendered
		}
		setDesiredConfig := func(config string) {
			o.Expect(oc.Run("annotate").Args("node", target.Name, "machineconfiguration.openshift.io/desiredConfig="+config, "--overwrite").Execute()).To(o.Succeed())
		}
		waitForRollout := func(config string, requireReboot bool) {
			o.Eventually(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				pc, err := apis.GetPacemakerCluster(oc)
				if err != nil {
					return err
				}
				if err := apis.ExpectPacemakerCRFresh(pc, 5*time.Minute); err != nil {
					return o.StopTrying("cannot verify membership from stale Pacemaker status").Wrap(err)
				}
				if pc.Status.Nodes == nil || len(*pc.Status.Nodes) != len(nodes.Items) {
					return o.StopTrying("configured Pacemaker membership changed during the rollout")
				}
				for _, node := range nodes.Items {
					if !slices.ContainsFunc(*pc.Status.Nodes, func(member etcdv1.PacemakerClusterNodeStatus) bool { return member.NodeName == node.Name }) {
						return o.StopTrying(fmt.Sprintf("Pacemaker member %s was removed during the rollout", node.Name))
					}
				}

				for _, original := range nodes.Items {
					node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, original.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					if node.UID != original.UID {
						return o.StopTrying(fmt.Sprintf("Node %s was replaced during the rollout", node.Name))
					}
					if node.Name != target.Name {
						if node.Status.NodeInfo.BootID != original.Status.NodeInfo.BootID || node.Annotations["machineconfiguration.openshift.io/desiredConfig"] != originalConfig {
							return o.StopTrying(fmt.Sprintf("peer %s rebooted or was targeted by the rollout", node.Name))
						}
						continue
					}
					if requireReboot && node.Status.NodeInfo.BootID == original.Status.NodeInfo.BootID {
						return fmt.Errorf("Node %s has not rebooted", node.Name)
					}
					if node.Annotations["machineconfiguration.openshift.io/currentConfig"] != config || node.Annotations["machineconfiguration.openshift.io/desiredConfig"] != config ||
						node.Annotations["machineconfiguration.openshift.io/state"] != "Done" || node.Spec.Unschedulable || !nodeutil.IsNodeReady(node) {
						return fmt.Errorf("Node %s has not finished applying %s", node.Name, config)
					}
				}
				return apis.ExpectClusterHealthy(pc)
			}, 30*time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
			o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, 5*time.Minute)).To(o.Succeed())
		}

		g.DeferCleanup(func() {
			g.By("Restoring the rendered config and target node before unpausing the master pool")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := configs.MachineConfigs().Delete(ctx, mc.Name, metav1.DeleteOptions{})
			if !apierrors.IsNotFound(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			waitForRender(false)
			defer func() {
				o.Expect(oc.Run("patch").Args("machineconfigpool", "master", "--type=merge", "-p", `{"spec":{"paused":false}}`).Execute()).To(o.Succeed())
				o.Eventually(func() bool {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					pool, err := configs.MachineConfigPools().Get(ctx, "master", metav1.GetOptions{})
					return err == nil && pool.Status.ObservedGeneration >= pool.Generation && pool.Status.Configuration.Name == originalConfig &&
						pool.Status.UpdatedMachineCount == 2 && pool.Status.ReadyMachineCount == 2 &&
						slices.ContainsFunc(pool.Status.Conditions, func(condition machineconfigv1.MachineConfigPoolCondition) bool {
							return condition.Type == machineconfigv1.MachineConfigPoolUpdated && condition.Status == corev1.ConditionTrue
						})
				}, 5*time.Minute, utils.FiveSecondPollInterval).Should(o.BeTrue(), "master pool must return to its original config")
				o.Expect(utils.IsClusterHealthyWithTimeout(oc, clusterOperatorStabilizationTimeout)).To(o.Succeed())
			}()
			setDesiredConfig(originalConfig)
			waitForRollout(originalConfig, false)
		})

		// Pausing stops pool-wide targeting; the daemon and drain controller still process Node annotations.
		g.By(fmt.Sprintf("Pausing master targeting and applying a reboot-required MachineConfig to %s", target.Name))
		o.Expect(oc.Run("patch").Args("machineconfigpool", "master", "--type=merge", "-p", `{"spec":{"paused":true}}`).Execute()).To(o.Succeed())
		_, err = configs.MachineConfigs().Create(ctx, mc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		rendered := waitForRender(true)
		setDesiredConfig(rendered)
		waitForRollout(rendered, true)
	})
})
