package image_ecosystem

import (
	"fmt"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/db"
)

var _ = g.Describe("[image_ecosystem][mongodb] openshift mongodb replication", func() {
	defer g.GinkgoRecover()

	const (
		templatePath         = "https://raw.githubusercontent.com/sclorg/mongodb-container/master/2.4/examples/replica/mongodb-clustered.json"
		deploymentConfigName = "mongodb"
		expectedValue        = `{ "status" : "passed" }`
		insertCmd            = "db.bar.save(" + expectedValue + ")"
	)

	const (
		expectedReplicasAfterDeployment = 3
		expectedReplicasAfterScalingUp  = expectedReplicasAfterDeployment + 2
	)

	oc := exutil.NewCLI("mongodb-replica", exutil.KubeConfigPath()).Verbose()

	g.Describe("creating from a template", func() {
		g.It(fmt.Sprintf("should process and create the %q template", templatePath), func() {

			exutil.CheckOpenShiftNamespaceImageStreams(oc)
			g.By("creating a new app")
			o.Expect(oc.Run("new-app").Args("-f", templatePath).Execute()).Should(o.Succeed())

			g.By("waiting for the deployment to complete")
			err := exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), deploymentConfigName, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			podNames := waitForNumberOfPodsWithLabel(oc, expectedReplicasAfterDeployment, "mongodb-replica")
			mongo := db.NewMongoDB(podNames[0])

			g.By(fmt.Sprintf("expecting that replica set have %d members", expectedReplicasAfterDeployment))
			assertMembersInReplica(oc, mongo, expectedReplicasAfterDeployment)

			g.By("expecting that we can insert a new record on primary node")
			replicaSet := mongo.(exutil.ReplicaSet)
			_, err = replicaSet.QueryPrimary(oc, insertCmd)
			o.Expect(err).ShouldNot(o.HaveOccurred())

			g.By("expecting that we can read a record from all members")
			for _, podName := range podNames {
				tryToReadFromPod(oc, podName, expectedValue)
			}

			g.By(fmt.Sprintf("scaling deployment config %s to %d replicas", deploymentConfigName, expectedReplicasAfterScalingUp))

			err = oc.Run("scale").Args("dc", deploymentConfigName, "--replicas="+fmt.Sprint(expectedReplicasAfterScalingUp), "--timeout=30s").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			podNames = waitForNumberOfPodsWithLabel(oc, expectedReplicasAfterScalingUp, "mongodb-replica")
			mongo = db.NewMongoDB(podNames[0])

			g.By("expecting that scaling replica set up should have more members")
			assertMembersInReplica(oc, mongo, expectedReplicasAfterScalingUp)
		})
	})

})

func tryToReadFromPod(oc *exutil.CLI, podName, expectedValue string) {
	// don't include _id field to output because it changes every time
	findCmd := "rs.slaveOk(); printjson(db.bar.find({}, {_id: 0}).toArray())"

	fmt.Fprintf(g.GinkgoWriter, "DEBUG: reading record from pod %v\n", podName)

	mongoPod := db.NewMongoDB(podName)
	result, err := mongoPod.Query(oc, findCmd)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	o.Expect(result).Should(o.ContainSubstring(expectedValue))
}

func waitForNumberOfPodsWithLabel(oc *exutil.CLI, number int, label string) []string {
	g.By(fmt.Sprintf("expecting that there are %d running pods with label name=%s", number, label))

	podNames, err := exutil.WaitForPods(
		oc.KubeREST().Pods(oc.Namespace()),
		exutil.ParseLabelsOrDie("name="+label),
		exutil.CheckPodIsRunningFn,
		number,
		1*time.Minute,
	)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	o.Expect(podNames).Should(o.HaveLen(number))

	return podNames
}

func assertMembersInReplica(oc *exutil.CLI, db exutil.Database, expectedReplicas int) {
	isMasterCmd := "printjson(db.isMaster())"
	getReplicaHostsCmd := "print(db.isMaster().hosts.length)"

	// pod is running but we need to wait when it will be really ready (became member of the replica)
	err := exutil.WaitForQueryOutputSatisfies(oc, db, 1*time.Minute, false, isMasterCmd, func(commandOutput string) bool {
		return commandOutput != ""
	})
	o.Expect(err).ShouldNot(o.HaveOccurred())

	isMasterOutput, _ := db.Query(oc, isMasterCmd)
	fmt.Fprintf(g.GinkgoWriter, "DEBUG: Output of the db.isMaster() command: %v\n", isMasterOutput)

	members, err := db.Query(oc, getReplicaHostsCmd)
	o.Expect(err).ShouldNot(o.HaveOccurred())
	o.Expect(members).Should(o.Equal(strconv.Itoa(expectedReplicas)))
}
