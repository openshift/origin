package dr

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	promclient "github.com/openshift/origin/test/extended/prometheus/client"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	apps "k8s.io/kubernetes/test/e2e/upgrades/apps"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Disruptive]", func() {
	f := framework.NewDefaultFramework("leader-change")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("leader-change")

	g.It("[Feature:EtcdLeaderChange] Cluster should remain functional through etcd leader change", func() {
		framework.Logf("Verify SSH is available before restart")
		masters, _ := clusterNodes(oc)
		o.Expect(len(masters)).To(o.BeNumerically(">=", 3))

		disruption.Run(f, "etcd Leader Coup d'etat", "leader-change",
			disruption.TestData{},
			[]upgrades.Test{
				&upgrades.ServiceUpgradeTest{},
				&upgrades.SecretUpgradeTest{},
				&apps.ReplicaSetUpgradeTest{},
				&apps.StatefulSetUpgradeTest{},
				&apps.DeploymentUpgradeTest{},
				&apps.DaemonSetUpgradeTest{},
			},
			func() {
				prometheus, err := promclient.NewE2EPrometheusRouterClient(oc)
				o.Expect(err).ToNot(o.HaveOccurred())
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					framework.Logf("Checking for etcdLeader at %v)", time.Now())
					etcdLeaderPodName, etcdLeaderContainerID, err := getEtcdLeader(oc, prometheus)
					if err != nil {
						framework.Logf("getEtcdLeader: error %v)", err)
						return false, nil
					}
					etcdLeaderNodeName := strings.TrimPrefix(etcdLeaderPodName, "etcd-")
					framework.Logf("Removing etcd leader from pod %q on node %q with containerID %q)", etcdLeaderPodName, etcdLeaderNodeName, etcdLeaderContainerID)
					for _, node := range masters {
						framework.Logf("Checking master: error %v)", err)
						if node.Name == etcdLeaderNodeName {
							framework.Logf("Removing etcd leader from pod %q on node %q with containerID %q)", etcdLeaderPodName, node.Name, etcdLeaderContainerID)
							cmd := fmt.Sprintf("sudo -i /bin/bash -cx 'crictl stop %v'", etcdLeaderContainerID)
							expectSSH(cmd, node)
							return false, nil
						}
					}
					framework.Logf("leader %s not removed", etcdLeaderPodName)
					return false, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			},
		)

	},
	)
})

func getEtcdLeader(oc *exutil.CLI, client prometheusv1.API) (string, string, error) {
	var etcdLeaderPodName string
	err := wait.Poll(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		resp, _, err := client.Query(context.Background(), "etcd_server_is_leader", time.Now())
		if err != nil {
			framework.Logf("Failed to query Prometheus: %v", err)
			return false, nil
		}

		for _, member := range resp.(model.Vector) {
			if member.Value == 1 {
				if member.Metric["pod"] != "" {
					etcdLeaderPodName = string(member.Metric["pod"])
					return true, nil
				}
			}
		}
		return false, nil
	})
	if err != nil {
		return "", "", err
	}

	pod, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").Get(context.Background(), etcdLeaderPodName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	if pod.Status.ContainerStatuses != nil {
		return etcdLeaderPodName, pod.Status.ContainerStatuses[0].ContainerID[8:], nil
	}

	return "", "", fmt.Errorf("getEtcdLeader: no containerID found for pod %v", pod)
}
