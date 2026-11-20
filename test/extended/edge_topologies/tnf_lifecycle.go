package edge_topologies

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
)

const (
	tnfNamespace       = "openshift-etcd"
	tnfGenerationLabel = "tnf.etcd.openshift.io/generation"
	tnfCompletedJob    = "tnf.etcd.openshift.io/completed-job"
	tnfCompletedAt     = "tnf.etcd.openshift.io/completed-at"
	tnfCompletedUID    = "tnf.etcd.openshift.io/completed-record-uid"
	tnfDrainAck        = "machineconfiguration.openshift.io/lastAppliedDrain"
)

var _ = g.Describe("[sig-etcd][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Serial][Disruptive] TNF lifecycle", func() {
	oc := exutil.NewCLI("tnf-lifecycle").AsAdmin()
	var nodes []corev1.Node

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, configv1.DualReplicaTopologyMode)
		utils.SkipIfClusterIsNotHealthy(oc, helpers.NewEtcdClientFactory(oc.KubeClient()))
		o.Expect(apis.ExpectPacemakerBaseline(oc)).To(o.Succeed())
		list, err := utils.GetNodes(oc, utils.LabelNodeRoleControlPlane)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(list.Items).To(o.HaveLen(2))
		nodes = list.Items
		waitForTNFCompletion(oc, nodes...)
	})

	g.It("preserves completion after Job cleanup, CEO restart and executor changes [apigroup:operator.openshift.io]", func() {
		ctx := context.Background()
		record := waitForTNFCompletion(oc, nodes...)
		transition := tnfTransition(oc)
		o.Expect(transition.Status).To(o.Equal(operatorv1.ConditionTrue))
		jobs := oc.AdminKubeClient().BatchV1().Jobs(tnfNamespace)
		list, err := jobs.List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name in (tnf-setup-job,tnf-update-setup-job)"})
		o.Expect(err).NotTo(o.HaveOccurred())
		retained := map[string]types.UID{}
		for _, job := range list.Items {
			if job.Labels["app.kubernetes.io/name"] == "tnf-setup-job" || job.Labels[tnfGenerationLabel] == record.Labels[tnfGenerationLabel] {
				o.Expect(tnfJobComplete(&job)).To(o.BeTrue(), "only completed Jobs may be removed")
				o.Expect(deleteTNFJob(oc, &job)).To(o.Succeed())
			} else {
				retained[job.Name] = job.UID
			}
		}
		checkCompletion := func() error {
			current, err := latestTNFRecord(oc)
			if err != nil {
				return err
			}
			if current.UID != record.UID || !reflect.DeepEqual(current.Data, record.Data) || !reflect.DeepEqual(current.Annotations, record.Annotations) {
				return fmt.Errorf("completed generation changed after Job cleanup")
			}
			currentJobs, err := jobs.List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name in (tnf-setup-job,tnf-update-setup-job)"})
			if err != nil {
				return err
			}
			for _, job := range currentJobs.Items {
				if retained[job.Name] != job.UID {
					return fmt.Errorf("setup work replayed: %s", job.Name)
				}
			}
			o.Expect(tnfTransition(oc)).To(o.Equal(transition))
			return nil
		}
		g.By("Restarting CEO after removing completed setup and generation Jobs")
		o.Expect(oc.Run("rollout").Args("restart", "deployment/etcd-operator", "-n", "openshift-etcd-operator").Execute()).To(o.Succeed())
		o.Expect(oc.Run("rollout").Args("status", "deployment/etcd-operator", "-n", "openshift-etcd-operator", "--timeout=5m").Execute()).To(o.Succeed())
		// Restart before stopping a kubelet so it can terminate the previous operator Pod.
		g.By("Changing executor eligibility without changing desired membership")
		g.DeferCleanup(func() {
			o.Expect(utils.IsClusterHealthyWithTimeout(oc, clusterOperatorStabilizationTimeout)).To(o.Succeed())
		})
		holdTNFKubelet(oc, nodes[0], nodes[1])
		o.Consistently(checkCompletion, time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
		o.Expect(utils.RemoveConstraint(oc, nodes[1].Name, "kubelet-clone")).To(o.Succeed())
		o.Eventually(func() bool { return utils.IsNodeReady(oc, nodes[0].Name) }, 5*time.Minute, utils.FiveSecondPollInterval).Should(o.BeTrue())

		g.By("Re-running after-setup with only the durable completion proof")
		after, err := jobs.List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=tnf-after-setup-job,node=" + string(nodes[0].UID)})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(after.Items).To(o.HaveLen(1))
		o.Expect(tnfJobComplete(&after.Items[0])).To(o.BeTrue())
		o.Expect(deleteTNFJob(oc, &after.Items[0])).To(o.Succeed())
		o.Eventually(func() error {
			if err := checkCompletion(); err != nil {
				return o.StopTrying("completion was lost").Wrap(err)
			}
			current, err := jobs.Get(ctx, after.Items[0].Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if current.UID == after.Items[0].UID || !tnfJobComplete(current) {
				return fmt.Errorf("waiting for after-setup to finish without setup Jobs")
			}
			return nil
		}, 10*time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
		o.Expect(checkCompletion()).To(o.Succeed())
	})

	g.DescribeTable("membership recovery [Requires:HypervisorSSHConfig]", func(interrupt bool) {
		ctx := context.Background()
		target, survivor := nodes[0], nodes[1]
		remoteTarget := tnfNodeSSH(target)
		remoteSurvivor := tnfNodeSSH(survivor)
		g.DeferCleanup(func() {
			_, err := remoteTarget("sudo systemctl start kubelet")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(apis.WaitForAndApproveNodeBootstrapperCSR(ctx, oc, target.Name, 10*time.Minute)).To(o.Succeed())
			o.Expect(utils.IsClusterHealthyWithTimeout(oc, clusterOperatorStabilizationTimeout)).To(o.Succeed())
		})
		holdTNFKubelet(oc, target, survivor)
		setWindow := func(bool) {}
		if !interrupt {
			// Hold only the acknowledgment: the drain controller queues changes to desiredDrain.
			o.Expect(survivor.Annotations["machineconfiguration.openshift.io/desiredDrain"]).NotTo(o.BeEmpty())
			o.Expect(survivor.Annotations[tnfDrainAck]).To(o.Equal(survivor.Annotations["machineconfiguration.openshift.io/desiredDrain"]))
			setWindow = func(active bool) {
				value := survivor.Annotations[tnfDrainAck]
				if active {
					value = "tnf-lifecycle-held"
				}
				o.Expect(oc.Run("annotate").Args("node", survivor.Name, tnfDrainAck+"="+value, "--overwrite").Execute()).To(o.Succeed())
			}
			g.DeferCleanup(func() { setWindow(false) })
			setWindow(true)
		}
		checkWindow := func() error {
			if interrupt {
				return nil
			}
			node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, survivor.Name, metav1.GetOptions{})
			if err == nil && node.Annotations[tnfDrainAck] != "tnf-lifecycle-held" {
				return o.StopTrying("MCO fixture cleared before the guarded operation completed")
			}
			return err
		}
		g.By("Deleting the stopped Node from Kubernetes")
		o.Expect(oc.AdminKubeClient().CoreV1().Nodes().Delete(ctx, target.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &target.UID}})).To(o.Succeed())
		if !interrupt {
			o.Eventually(func() bool {
				cm, err := latestTNFRecord(oc)
				return err == nil && tnfRecordMatches(cm, survivor)
			}, 5*time.Minute, utils.FiveSecondPollInterval).Should(o.BeTrue())
			o.Consistently(func() error {
				if err := checkWindow(); err != nil {
					return err
				}
				cm, err := latestTNFRecord(oc)
				if err != nil {
					return err
				}
				attempts, err := oc.AdminKubeClient().BatchV1().Jobs(tnfNamespace).List(ctx, metav1.ListOptions{LabelSelector: tnfGenerationLabel + "=" + cm.Labels[tnfGenerationLabel]})
				if err != nil {
					return err
				}
				if len(attempts.Items) != 0 {
					return fmt.Errorf("removal admitted during MCO activity")
				}
				return tnfConfiguredMembers(remoteSurvivor, target.Name, survivor.Name)
			}, time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
			setWindow(false)
		}
		g.By("Waiting for the one-node generation to remove the Pacemaker member")
		waitForTNFCompletion(oc, survivor)
		o.Expect(tnfConfiguredMembers(remoteSurvivor, survivor.Name)).To(o.Succeed())
		// Admission must observe removal before the add-only generation is proposed.
		o.Eventually(func() bool {
			pc, err := apis.GetPacemakerCluster(oc)
			return err == nil && apis.ExpectPacemakerCRFresh(pc, 5*time.Minute) == nil && pc.Status.Nodes != nil && len(*pc.Status.Nodes) == 1 && (*pc.Status.Nodes)[0].NodeName == survivor.Name
		}, 5*time.Minute, utils.FiveSecondPollInterval).Should(o.BeTrue())
		var finishInterruption func(*corev1.ConfigMap) *corev1.ConfigMap
		if interrupt {
			finishInterruption = tnfInterruptAdd(oc, remoteSurvivor, target, survivor)
		}
		setWindow(true)
		o.Expect(utils.RemoveConstraint(oc, survivor.Name, "kubelet-clone")).To(o.Succeed())
		if interrupt {
			// Keep the new member ineligible as an executor until retry starts its kubelet.
			// Otherwise a collector refresh can change executors and cancel the held attempt.
			g.By("Publishing a replacement Node identity while its kubelet remains stopped")
			node, err := oc.AdminKubeClient().CoreV1().Nodes().Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: target.Name, Labels: target.Labels, Annotations: target.Annotations},
				Spec:       target.Spec,
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			node.Status = target.Status
			for i := range node.Status.Conditions {
				if node.Status.Conditions[i].Type == corev1.NodeReady {
					node.Status.Conditions[i].Status = corev1.ConditionFalse
				}
			}
			node, err = oc.AdminKubeClient().CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(node.UID).NotTo(o.Equal(target.UID))
			target = *node
		} else {
			g.By("Starting the removed host's kubelet to register a new Kubernetes identity")
			_, err := remoteTarget("sudo systemctl start kubelet")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Eventually(func() error {
				if err := checkWindow(); err != nil {
					return err
				}
				if _, err := apis.ApproveNodeBootstrapperCSRsForNode(ctx, oc, target.Name); err != nil {
					return err
				}
				node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, target.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if node.UID == target.UID || !nodeutil.IsNodeReady(node) {
					return fmt.Errorf("waiting for %s to register with a new UID and become Ready", target.Name)
				}
				target = *node
				return nil
			}, 10*time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
		}
		var record *corev1.ConfigMap
		if interrupt {
			record = finishInterruption(waitForTNFRecord(oc, false, nil, target, survivor))
		} else {
			record = waitForTNFRecord(oc, true, checkWindow, target, survivor)
		}
		o.Expect(tnfConfiguredMembers(remoteSurvivor, target.Name, survivor.Name)).To(o.Succeed())
		if interrupt {
			logs, err := oc.Run("logs").Args("-n", tnfNamespace, "job/"+record.Annotations[tnfCompletedJob], "--all-containers=true").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, command := range []string{"pcs cluster node add ", "pcs cluster node remove ", "--name force_new_cluster --update"} {
				o.Expect(logs).NotTo(o.ContainSubstring(command), "retry must not replay membership work or claim FNC")
			}
		}
		setWindow(false)
	},
		g.Entry("defers removal during MCO activity and permits add-only recovery while it remains active", false),
		g.Entry("finishes an interrupted add without repeating membership changes or claiming force-new-cluster", true),
	)
})

func latestTNFRecord(oc *exutil.CLI) (*corev1.ConfigMap, error) {
	list, err := oc.AdminKubeClient().CoreV1().ConfigMaps(tnfNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app.kubernetes.io/component=tnf-update-setup"})
	if err != nil {
		return nil, err
	}
	var latest *corev1.ConfigMap
	var number int64
	for i := range list.Items {
		var snapshot struct{ Generation int64 }
		if err := json.Unmarshal([]byte(list.Items[i].Data["config.json"]), &snapshot); err != nil {
			return nil, err
		}
		if snapshot.Generation > number {
			latest, number = &list.Items[i], snapshot.Generation
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no TNF generation record")
	}
	return latest, nil
}

func tnfRecordMatches(cm *corev1.ConfigMap, nodes ...corev1.Node) bool {
	var snapshot struct{ Nodes []struct{ Name, UID string } }
	if json.Unmarshal([]byte(cm.Data["config.json"]), &snapshot) != nil || len(snapshot.Nodes) != len(nodes) {
		return false
	}
	for _, node := range nodes {
		if !slices.ContainsFunc(snapshot.Nodes, func(n struct{ Name, UID string }) bool { return n.Name == node.Name && n.UID == string(node.UID) }) {
			return false
		}
	}
	return true
}

func waitForTNFCompletion(oc *exutil.CLI, nodes ...corev1.Node) *corev1.ConfigMap {
	return waitForTNFRecord(oc, true, nil, nodes...)
}

func waitForTNFRecord(oc *exutil.CLI, completed bool, guard func() error, nodes ...corev1.Node) *corev1.ConfigMap {
	var record *corev1.ConfigMap
	o.Eventually(func() error {
		if guard != nil {
			if err := guard(); err != nil {
				return err
			}
		}
		cm, err := latestTNFRecord(oc)
		if err != nil {
			return err
		}
		if !tnfRecordMatches(cm, nodes...) || (completed && (cm.Annotations[tnfCompletedAt] == "" || cm.Annotations[tnfCompletedUID] != string(cm.UID) || cm.Annotations[tnfCompletedJob] == "")) {
			return fmt.Errorf("waiting for matching TNF generation (completed=%t)", completed)
		}
		record = cm
		return nil
	}, 15*time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
	return record
}

func tnfJobComplete(job *batchv1.Job) bool {
	return slices.ContainsFunc(job.Status.Conditions, func(c batchv1.JobCondition) bool {
		return c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue
	})
}

func deleteTNFJob(oc *exutil.CLI, job *batchv1.Job) error {
	policy := metav1.DeletePropagationBackground
	return oc.AdminKubeClient().BatchV1().Jobs(tnfNamespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{PropagationPolicy: &policy, Preconditions: &metav1.Preconditions{UID: &job.UID}})
}

func tnfTransition(oc *exutil.CLI) operatorv1.OperatorCondition {
	etcd, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, condition := range etcd.Status.Conditions {
		if condition.Type == "ExternalEtcdHasCompletedTransition" {
			return condition
		}
	}
	g.Fail("external etcd transition condition is missing")
	return operatorv1.OperatorCondition{}
}

func holdTNFKubelet(oc *exutil.CLI, target, survivor corev1.Node) {
	g.DeferCleanup(func() { o.Expect(utils.RemoveConstraint(oc, survivor.Name, "kubelet-clone")).To(o.Succeed()) })
	o.Expect(utils.AddConstraint(oc, survivor.Name, "kubelet-clone", target.Name)).To(o.Succeed())
	o.Eventually(func() bool {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), target.Name, metav1.GetOptions{})
		return err == nil && !nodeutil.IsNodeReady(node)
	}, 5*time.Minute, utils.FiveSecondPollInterval).Should(o.BeTrue())
}

func tnfNodeSSH(node corev1.Node) func(string) (string, error) {
	config := exutil.GetHypervisorConfig()
	if config == nil {
		g.Skip("hypervisor SSH configuration is required")
	}
	ssh := core.SSHConfig{IP: config.HypervisorIP, User: config.SSHUser, PrivateKeyPath: config.PrivateKeyPath}
	local, err := core.PrepareLocalKnownHostsFile(&ssh)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.DeferCleanup(func() { o.Expect(core.CleanupLocalKnownHostsFile(&ssh, local)).To(o.Succeed()) })
	ip := utils.GetNodeInternalIP(&node)
	o.Expect(ip).NotTo(o.BeEmpty())
	remote, err := core.PrepareRemoteKnownHostsFile(ip, &ssh, local)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.DeferCleanup(func() { o.Expect(core.CleanupRemoteKnownHostsFile(&ssh, local, remote)).To(o.Succeed()) })
	return func(command string) (string, error) {
		stdout, _, err := core.ExecuteRemoteSSHCommand(ip, command, &ssh, local, remote)
		return strings.TrimSpace(stdout), err
	}
}

func tnfConfiguredMembers(remote func(string) (string, error), names ...string) error {
	output, err := remote("sudo pcs cluster config show --output-format json")
	if err != nil {
		return err
	}
	var config struct{ Nodes []struct{ Name string } }
	if err := json.Unmarshal([]byte(output), &config); err != nil {
		return err
	}
	var actual []string
	for _, node := range config.Nodes {
		actual = append(actual, node.Name)
	}
	slices.Sort(actual)
	slices.Sort(names)
	if !slices.Equal(actual, names) {
		return fmt.Errorf("configured Pacemaker members: %v, expected %v", actual, names)
	}
	return nil
}

// The host wrapper holds a successful add response until the test kills that command.
func tnfInterruptAdd(oc *exutil.CLI, remote func(string) (string, error), target, survivor corev1.Node) func(*corev1.ConfigMap) *corev1.ConfigMap {
	dir := "/run/" + oc.Namespace()
	run := func(script string) (string, error) {
		return remote("printf '%s' '" + base64.StdEncoding.EncodeToString([]byte("set -euo pipefail\nd="+dir+"\n"+script)) + "' | base64 -d | sudo bash")
	}
	g.DeferCleanup(func() {
		_, err := run(`if test -f "$d/wrapper" && mountpoint -q /usr/sbin/pcs; then umount /usr/sbin/pcs; fi
if test -s "$d/checkpoint"; then
  pid=$(cat "$d/checkpoint")
  if test -r "/proc/$pid/cmdline"; then
    read -r -d '' argv0 < "/proc/$pid/cmdline" || true
    if test "$argv0" = "$d/checkpoint"; then kill -KILL "$pid"; fi
  fi
fi
if mountpoint -q "$d/pcs"; then umount "$d/pcs"; fi
rm -rf "$d"`)
		o.Expect(err).NotTo(o.HaveOccurred(), "restore the host pcs executable")
	})
	_, err := run(`! mountpoint -q /usr/sbin/pcs
mkdir -m 700 "$d"
touch "$d/pcs"
mount --bind /usr/sbin/pcs "$d/pcs"
cat > "$d/wrapper" <<'WRAPPER'
#!/bin/bash
d=` + dir + `
if [[ "$1 $2 $3" == 'cluster node add' || "$1 $2 $3" == 'cluster node remove' ]]; then
  printf '%s\n' "$*" >> "$d/mutations"
fi
"$d/pcs" "$@"
rc=$?
if [[ "$1 $2 $3 $4" == 'cluster node add ` + target.Name + `' && $rc == 0 && ! -e "$d/checkpoint" ]]; then
  printf '%s\n' "$$" > "$d/checkpoint"
  exec -a "$d/checkpoint" /usr/bin/sleep infinity
fi
exit "$rc"
WRAPPER
chmod 755 "$d/wrapper"
chcon --reference=/usr/sbin/pcs "$d/wrapper"
mount --bind "$d/wrapper" /usr/sbin/pcs`)
	o.Expect(err).NotTo(o.HaveOccurred())
	return func(record *corev1.ConfigMap) *corev1.ConfigMap {
		g.By("Waiting until Pacemaker accepts the add while its response is held")
		o.Eventually(func() error {
			_, err := run(`test -s "$d/checkpoint"
pid=$(cat "$d/checkpoint")
test "$(readlink /proc/$pid/exe)" = /usr/bin/sleep`)
			return err
		}, 10*time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
		o.Expect(tnfConfiguredMembers(remote, target.Name, survivor.Name)).To(o.Succeed())
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), target.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		uid, err := remote("sudo crm_attribute --type crm_config --name tnf_node_uid_" + target.Name + " --query --quiet")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(uid).To(o.Equal(string(node.UID)), "accepted membership must retain the new Kubernetes UID")
		attempts, err := oc.AdminKubeClient().BatchV1().Jobs(tnfNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: tnfGenerationLabel + "=" + record.Labels[tnfGenerationLabel]})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(attempts.Items).To(o.HaveLen(1))
		first := attempts.Items[0]
		o.Expect(first.Spec.Template.Spec.NodeName).To(o.Equal(survivor.Name))
		g.By("Terminating the held host command before allowing automatic retry")
		_, err = run(`pid=$(cat "$d/checkpoint")
read -r -d '' argv0 < "/proc/$pid/cmdline"
test "$argv0" = "$d/checkpoint"
kill -KILL "$pid"`)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(func() error {
			if _, err := run(`test ! -e /proc/$(cat "$d/checkpoint")`); err != nil {
				return err
			}
			pods, err := oc.AdminKubeClient().CoreV1().Pods(tnfNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "batch.kubernetes.io/controller-uid=" + string(first.UID)})
			if err != nil {
				return err
			}
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded {
					return fmt.Errorf("old attempt %s has not stopped", pod.Name)
				}
			}
			return nil
		}, 5*time.Minute, utils.FiveSecondPollInterval).Should(o.Succeed())
		completed := waitForTNFCompletion(oc, *node, survivor)
		o.Expect(completed.UID).To(o.Equal(record.UID), "retry must complete the same recorded generation")
		o.Expect(completed.Annotations[tnfCompletedJob]).NotTo(o.Equal(first.Name))
		mutations, err := run(`cat "$d/mutations"`)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Split(mutations, "\n")).To(o.HaveLen(1), "retry must not repeat any add or remove command")
		return completed
	}
}
