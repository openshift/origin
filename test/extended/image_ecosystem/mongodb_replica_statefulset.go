package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	dbutil "github.com/openshift/origin/test/extended/util/db"
	kapi "k8s.io/kubernetes/pkg/api"
)

var _ = g.Describe("[Conformance][image_ecosystem][mongodb][Slow] openshift mongodb replication (with statefulset)", func() {
	defer g.GinkgoRecover()

	const templatePath = "https://raw.githubusercontent.com/sclorg/mongodb-container/master/examples/petset/mongodb-petset-persistent.yaml"

	oc := exutil.NewCLI("mongodb-petset-replica", exutil.KubeConfigPath()).Verbose()

	g.Describe("creating from a template", func() {
		g.AfterEach(func() {
			for i := 0; i < 3; i++ {
				pod := fmt.Sprintf("mongodb-replicaset-%d", i)
				podLogs, err := oc.Run("logs").Args(pod, "--timestamps").Output()
				if err != nil {
					ginkgolog("error retrieving pod logs for %s: %v", pod, err)
					continue
				}
				ginkgolog("pod logs for %s:\n%s", podLogs, err)
			}
		})
		g.It(fmt.Sprintf("should instantiate the template"), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By("creating persistent volumes")
			_, err := exutil.SetupHostPathVolumes(
				oc.AdminKubeClient().Core().PersistentVolumes(),
				oc.Namespace(),
				"256Mi",
				3,
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			defer func() {
				// We're removing only PVs because all other things will be removed
				// together with namespace.
				err := exutil.CleanupHostPathVolumes(oc.AdminKubeClient().Core().PersistentVolumes(), oc.Namespace())
				if err != nil {
					ginkgolog("WARNING: couldn't cleanup persistent volumes: %v", err)
				}
			}()

			g.By("creating a new app")
			o.Expect(
				oc.Run("new-app").Args(
					"-f", templatePath,
					"-p", "VOLUME_CAPACITY=256Mi",
					"-p", "MEMORY_LIMIT=512Mi",
					"-p", "MONGODB_IMAGE=centos/mongodb-32-centos7",
					"-p", "MONGODB_SERVICE_NAME=mongodb-replicaset",
				).Execute(),
			).Should(o.Succeed())

			g.By("waiting for all pods to reach ready status")
			podNames, err := exutil.WaitForPods(
				oc.KubeClient().Core().Pods(oc.Namespace()),
				exutil.ParseLabelsOrDie("name=mongodb-replicaset"),
				exutil.CheckPodIsReadyFn,
				3,
				4*time.Minute,
			)
			if err != nil {
				desc, _ := oc.Run("describe").Args("statefulset").Output()
				ginkgolog("\n\nStatefulset at failure:\n%s\n\n", desc)
				desc, _ = oc.Run("describe").Args("pods").Output()
				ginkgolog("\n\nPods at statefulset failure:\n%s\n\n", desc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting that we can insert a new record on primary node")
			mongo := dbutil.NewMongoDB(podNames[0])
			replicaSet := mongo.(exutil.ReplicaSet)
			out, err := replicaSet.QueryPrimary(oc, `db.test.save({ "status" : "passed" })`)
			ginkgolog("save result: %s\n", out)
			o.Expect(err).ShouldNot(o.HaveOccurred())

			g.By("expecting that we can read a record from all members")
			for _, podName := range podNames {
				o.Expect(readRecordFromPod(oc, podName)).To(o.Succeed())
			}

			g.By("restarting replica set")
			err = oc.Run("delete").Args("pods", "--all", "-n", oc.Namespace()).Execute()
			o.Expect(err).ShouldNot(o.HaveOccurred())

			g.By("waiting for all pods to be gracefully deleted")
			podNames, err = exutil.WaitForPods(
				oc.KubeClient().Core().Pods(oc.Namespace()),
				exutil.ParseLabelsOrDie("name=mongodb-replicaset"),
				func(pod kapi.Pod) bool { return pod.DeletionTimestamp != nil },
				0,
				2*time.Minute,
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for all pods to reach ready status")
			podNames, err = exutil.WaitForPods(
				oc.KubeClient().Core().Pods(oc.Namespace()),
				exutil.ParseLabelsOrDie("name=mongodb-replicaset"),
				exutil.CheckPodIsReadyFn,
				3,
				2*time.Minute,
			)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting that we can read a record from all members after its restart")
			for _, podName := range podNames {
				o.Expect(readRecordFromPod(oc, podName)).To(o.Succeed())
			}
		})
	})

})

func readRecordFromPod(oc *exutil.CLI, podName string) error {
	// don't include _id field to output because it changes every time
	findCmd := "rs.slaveOk(); printjson(db.test.find({}, {_id: 0}).toArray())"

	fmt.Fprintf(g.GinkgoWriter, "DEBUG: reading record from the pod %v\n", podName)

	mongoPod := dbutil.NewMongoDB(podName)
	// pod is running but we need to wait when it will be really ready
	// (will become a member of replica set and will finish data sync)
	return exutil.WaitForQueryOutputContains(oc, mongoPod, 1*time.Minute, false, findCmd, `{ "status" : "passed" }`)
}
