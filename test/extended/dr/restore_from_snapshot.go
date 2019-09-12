package dr

import (
	"fmt"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

const (
	rollBackMachineConfig = "99-rollback-test"
)

var _ = g.Describe("[Feature:DisasterRecovery][Disruptive]", func() {
	f := e2e.NewDefaultFramework("disaster-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("disaster-recovery")

	g.It("[dr-etcd-snapshot] Cluster should restore itself from etcd snapshot", func() {
		config, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dynamicClient := dynamic.NewForConfigOrDie(config)
		mcps := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigpools",
		})
		mc := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigs",
		})
		coc := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "config.openshift.io",
			Version:  "v1",
			Resource: "clusteroperators",
		})

		setMachineConfig("rollback-A.yaml", oc, mcps)

		masters := masterNodes(oc)
		masterNames := sets.NewString()
		for _, node := range masters {
			masterNames.Insert(node.Name)
		}

		e2e.Logf("masters: %v", masters)
		o.Expect(masters).NotTo(o.BeEmpty())
		firstMaster := masters[0]
		e2e.Logf("first master: %v", firstMaster)

		e2e.Logf("Make etcd backup on first master")
		expectSSH("sudo -i /bin/bash -x /usr/local/bin/etcd-snapshot-backup.sh /root/assets/backup/snapshot.db", firstMaster)
		expectSSH("sudo -i install -o core -g core /root/assets/backup/snapshot.db /tmp/snapshot.db", firstMaster)

		setMachineConfig("rollback-B.yaml", oc, mcps)

		masterHosts := strings.Join(masterNames.List(), " ")
		restoreScriptPath := exutil.FixturePath("testdata", "disaster-recovery", "restore-etcd.sh")
		cmd := fmt.Sprintf("env BASTION_HOST= MASTERHOSTS='%s' KUBE_SSH_KEY_PATH='%s' /bin/bash -x %s ", masterHosts, os.Getenv("KUBE_SSH_KEY_PATH"), restoreScriptPath)
		runCommandAndRetry(cmd)

		time.Sleep(30 * time.Second)
		waitForAPIServer(oc)
		// restartSDNPods(oc)
		restartOpenshiftAPIPods(oc)
		restartMCDPods(oc)
		waitForMastersToUpdate(oc, mcps)
		waitForOperatorsToSettle(coc)

		rollBackInMC := getRollbackContentsInMachineConfig(oc, mc, rollBackMachineConfig)
		o.Expect(rollBackInMC).To(o.BeEquivalentTo("data:,A"))

		for _, master := range masters {
			rollBackFile := fetchFileContents(master, "/etc/rollback-test")
			o.Expect(rollBackFile).To(o.BeEquivalentTo("A"))
		}
	})
})

func setMachineConfig(rollbackFileName string, oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2e.Logf("Update MachineConfig using %s file on masters", rollbackFileName)
	machineConfigTemplate := exutil.FixturePath("testdata", "disaster-recovery", rollbackFileName)
	err := oc.Run("apply").Args("-f", machineConfigTemplate).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	waitForMastersToUpdate(oc, mcps)
}

func getRollbackContentsInMachineConfig(oc *exutil.CLI, mcs dynamic.NamespaceableResourceInterface, mcName string) string {
	e2e.Logf("Reading contents of rollback MachineConfig")
	pool, err := mcs.Get(mcName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	files, found, err := unstructured.NestedSlice(pool.Object, "spec", "config", "storage", "files")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue())
	o.Expect(files).NotTo(o.BeEmpty())

	file := files[0].(map[string]interface{})
	actual, found, err := unstructured.NestedString(file, "contents", "source")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue())

	return actual
}

func waitForAPIServer(oc *exutil.CLI) {
	e2e.Logf("Waiting for API server to restore")
	err := wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		_, err = oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}
