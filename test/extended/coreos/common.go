package coreos

import (
	"context"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"

	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"
)

func runCommandAndRetry(command string) string {
	const (
		maxRetries = 10
		pause      = 10
	)
	var (
		retryCount = 0
		out        []byte
		err        error
	)
	for retryCount = 0; retryCount <= maxRetries; retryCount++ {
		out, err = exec.Command("bash", "-c", command).CombinedOutput()
		e2elog.Logf("output:\n%s", out)
		if err == nil {
			break
		}
		e2elog.Logf("%v", err)
		time.Sleep(time.Second * pause)
	}
	o.Expect(retryCount).NotTo(o.Equal(maxRetries + 1))
	return string(out)
}

func clusterNodes(oc *exutil.CLI) (masters, workers []*corev1.Node) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			masters = append(masters, node)
		} else {
			workers = append(workers, node)
		}
	}
	return
}

func waitForInfraToUpdate(oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2elog.Logf("Waiting for updates to be finished on infra pool")
	err := wait.Poll(30*time.Second, 15*time.Minute, func() (done bool, err error) {
		done, _ = upgrade.IsPoolUpdated(mcps, "infra")
		return done, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForWorkerToUpdate(oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2elog.Logf("Waiting for updates to be finished on worker pool")
	err := wait.Poll(30*time.Second, 15*time.Minute, func() (done bool, err error) {
		done, _ = upgrade.IsPoolUpdated(mcps, "worker")
		return done, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}
