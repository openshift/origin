package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	//	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	dbutil "github.com/openshift/origin/test/extended/util/db"
	//	kapiv1 "k8s.io/api/core/v1"
	//	e2e "k8s.io/kubernetes/test/e2e/framework"
)

/*
var _ = g.Describe("[Conformance][image_ecosystem][mongodb][Slow] openshift mongodb replication (with statefulset)", func() {
	defer g.GinkgoRecover()

	const templatePath = "https://raw.githubusercontent.com/sclorg/mongodb-container/master/examples/petset/mongodb-petset-persistent.yaml"

	oc := exutil.NewCLI("mongodb-petset-replica", exutil.KubeConfigPath()).Verbose()
	var pvs = []*kapiv1.PersistentVolume{}
	var nfspod = &kapiv1.Pod{}
	var cleanup = func() {
		g.By("start cleanup")
		if g.CurrentGinkgoTestDescription().Failed {
			exutil.DumpPodStates(oc)
			exutil.DumpPodLogsStartingWith("", oc)
			exutil.DumpPersistentVolumeInfo(oc)
			for i := 0; i < 3; i++ {
				podLogs, err := oc.Run("logs").Args(fmt.Sprintf("mongodb-replicaset-%d", i), "--timestamps").Output()
				if err != nil {
					e2e.Logf("error retrieving pod logs for %s: %v", fmt.Sprintf("mongodb-replicaset-%d", i), err)
					continue
				}
				e2e.Logf("pod logs for %s:\n%s", podLogs, err)
			}
		}

		client := oc.AsAdmin().KubeFramework().ClientSet
		g.By("removing mongo")
		exutil.RemoveStatefulSets(oc, "mongodb-replicaset")

		g.By("deleting PVCs")
		exutil.DeletePVCsForDeployment(client, oc, "mongo")

		g.By("removing nfs pvs")
		for _, pv := range pvs {
			e2e.DeletePersistentVolume(client, pv.Name)
		}

		g.By("removing nfs pod")
		e2e.DeletePodWithWait(oc.AsAdmin().KubeFramework(), client, nfspod)
	}

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()

			g.By("PV/PVC dump before setup")
			exutil.DumpPersistentVolumeInfo(oc)

			g.By("waiting for default service account")
			err := exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())

			nfspod, pvs, err = exutil.SetupK8SNFSServerAndVolume(oc, 3)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It(fmt.Sprintf("should instantiate the template"), func() {
			// per k8s e2e volume_util.go:VolumeTestCleanup, nuke any client pods
			// before nfs server to assist with umount issues; as such, need to clean
			// up prior to the AfterEach processing, to guaranteed deletion order
			defer cleanup()

			err := oc.Run("create").Args("-f", templatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a new app")
			err = oc.Run("new-app").Args(
				"--template", "mongodb-petset-replication",
				"-p", "VOLUME_CAPACITY=256Mi",
				"-p", "MEMORY_LIMIT=512Mi",
				"-p", "MONGODB_IMAGE=centos/mongodb-32-centos7",
				"-p", "MONGODB_SERVICE_NAME=mongodb-replicaset",
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("PV/PVC dump after setup")
			exutil.DumpPersistentVolumeInfo(oc)

			g.By("waiting for all pods to reach ready status")
			podNames, err := exutil.WaitForPods(
				oc.KubeClient().Core().Pods(oc.Namespace()),
				exutil.ParseLabelsOrDie("name=mongodb-replicaset"),
				exutil.CheckPodIsReady,
				3,
				8*time.Minute,
			)
			if err != nil {
				desc, _ := oc.Run("describe").Args("statefulset").Output()
				e2e.Logf("\n\nStatefulset at failure:\n%s\n\n", desc)
				desc, _ = oc.Run("describe").Args("pods").Output()
				e2e.Logf("\n\nPods at statefulset failure:\n%s\n\n", desc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting that we can insert a new record on primary node")
			mongo := dbutil.NewMongoDB(podNames[0])
			replicaSet := mongo.(exutil.ReplicaSet)
			done := make(chan struct{}, 1)
			go func() {
				defer func() { done <- struct{}{} }()
				out, err := replicaSet.QueryPrimary(oc, `db.test.save({ "status" : "passed" })`)
				e2e.Logf("save result: %s\n", out)
				o.Expect(err).ShouldNot(o.HaveOccurred())
			}()
			select {
			case <-time.After(1 * time.Minute):
				e2e.Failf("timed out waiting for db command to finish")
			case <-done:
			}

			g.By("expecting that we can read a record from all members")
			for _, podName := range podNames {
				o.Expect(readRecordFromPod(oc, podName)).To(o.Succeed())
			}

			g.By("restarting replica set")
			err = exutil.RemovePodsWithPrefixes(oc, "mongodb-replicaset")
			o.Expect(err).ShouldNot(o.HaveOccurred())

			g.By("waiting for all pods to be gracefully deleted")
			podNames, err = exutil.WaitForPods(
				oc.KubeClient().Core().Pods(oc.Namespace()),
				exutil.ParseLabelsOrDie("name=mongodb-replicaset"),
				func(pod kapiv1.Pod) bool { return pod.DeletionTimestamp != nil },
				0,
				4*time.Minute,
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for all pods to reach ready status")
			podNames, err = exutil.WaitForPods(
				oc.KubeClient().Core().Pods(oc.Namespace()),
				exutil.ParseLabelsOrDie("name=mongodb-replicaset"),
				exutil.CheckPodIsReady,
				3,
				4*time.Minute,
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting that we can read a record from all members after its restart")
			for _, podName := range podNames {
				o.Expect(readRecordFromPod(oc, podName)).To(o.Succeed())
			}
		})
	})
})
*/

func readRecordFromPod(oc *exutil.CLI, podName string) error {
	// don't include _id field to output because it changes every time
	findCmd := "rs.slaveOk(); printjson(db.test.find({}, {_id: 0}).toArray())"

	fmt.Fprintf(g.GinkgoWriter, "DEBUG: reading record from the pod %v\n", podName)

	mongoPod := dbutil.NewMongoDB(podName)
	// pod is running but we need to wait when it will be really ready
	// (will become a member of replica set and will finish data sync)
	return exutil.WaitForQueryOutputContains(oc, mongoPod, 1*time.Minute, false, findCmd, `{ "status" : "passed" }`)
}
