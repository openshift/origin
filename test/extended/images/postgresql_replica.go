package images

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

var (
	postgreSQLReplicationTemplate = "https://raw.githubusercontent.com/openshift/postgresql/master/examples/replica/postgresql_replica.json"
	postgreSQLEphemeralTemplate   = exutil.FixturePath("..", "..", "examples", "db-templates", "postgresql-ephemeral-template.json")
	postgreSQLHelperName          = "postgresql-helper"
	postgreSQLImages              = []string{
		"openshift/postgresql-92-centos7",
		"centos/postgresql-94-centos7",
		// TODO: Uncomment once upstream images are fixed
		// "registry.access.redhat.com/openshift3/postgresql-92-rhel7",
		// "registry.access.redhat.com/rhscl/postgresql-94-rhel7",
	}
)

var _ = g.Describe("images: postgresql: replication", func() {
	defer g.GinkgoRecover()

	for i, image := range postgreSQLImages {
		oc := exutil.NewCLI(fmt.Sprintf("postgresql-replication-%d", i), exutil.KubeConfigPath())
		testFn := PostgreSQLReplicationTestFactory(oc, image)
		g.It(fmt.Sprintf("postgresql replication works for %s", image), testFn)
	}
})

// CreatePostgreSQLReplicationHelpers creates a set of PostgreSQL helpers for master,
// slave an en extra helper that is used for remote login test.
func CreatePostgreSQLReplicationHelpers(c kclient.PodInterface, masterDeployment, slaveDeployment, helperDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
	podNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", masterDeployment)), exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	masterPod := podNames[0]

	slavePods, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", slaveDeployment)), exutil.CheckPodIsRunningFn, slaveCount, 3*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create PostgreSQL helper for master
	master := exutil.NewPostgreSQL(masterPod, "")

	// Create PostgreSQL helpers for slaves
	slaves := make([]exutil.Database, len(slavePods))
	for i := range slavePods {
		slave := exutil.NewPostgreSQL(slavePods[i], masterPod)
		slaves[i] = slave
	}

	helperNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", helperDeployment)), exutil.CheckPodIsRunningFn, 1, 1*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	helper := exutil.NewPostgreSQL(helperNames[0], masterPod)

	return master, slaves, helper
}

func PostgreSQLReplicationTestFactory(oc *exutil.CLI, image string) func() {
	return func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer cleanup(oc)

		_, err := exutil.SetupHostPathVolumes(oc.AdminKubeREST().PersistentVolumes(), oc.Namespace(), "512Mi", 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = testutil.WaitForPolicyUpdate(oc.REST(), oc.Namespace(), "create", "templates", true)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", postgreSQLReplicationTemplate, "-p", fmt.Sprintf("POSTGRESQL_IMAGE=%s", image)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", postgreSQLEphemeralTemplate, "-p", fmt.Sprintf("DATABASE_SERVICE_NAME=%s", postgreSQLHelperName)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.KubeFramework().WaitForAnEndpoint(postgreSQLHelperName)
		o.Expect(err).NotTo(o.HaveOccurred())

		tableCounter := 0
		assertReplicationIsWorking := func(masterDeployment, slaveDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
			tableCounter++
			table := fmt.Sprintf("table_%0.2d", tableCounter)

			master, slaves, helper := CreatePostgreSQLReplicationHelpers(oc.KubeREST().Pods(oc.Namespace()), masterDeployment, slaveDeployment, fmt.Sprintf("%s-1", postgreSQLHelperName), slaveCount)
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, []exutil.Database{master, helper})).NotTo(o.HaveOccurred())
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, slaves)).NotTo(o.HaveOccurred())

			// Test if we can query as admin
			oc.KubeFramework().WaitForAnEndpoint("postgresql-master")
			err := helper.TestRemoteLogin(oc, "postgresql-master")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Create a new table with random name
			_, err = master.Query(oc, fmt.Sprintf("CREATE TABLE %s (col1 VARCHAR(20), col2 VARCHAR(20));", table))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Write new data to the table through master
			_, err = master.Query(oc, fmt.Sprintf("INSERT INTO %s (col1, col2) VALUES ('val1', 'val2');", table))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure data is present on master
			err = exutil.WaitForQueryOutput(oc, master, 10*time.Second, false,
				fmt.Sprintf("SELECT * FROM %s;", table),
				"col1 | val1\ncol2 | val2")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure data was replicated to all slaves
			for _, slave := range slaves {
				err = exutil.WaitForQueryOutput(oc, slave, 90*time.Second, false,
					fmt.Sprintf("SELECT * FROM %s;", table),
					"col1 | val1\ncol2 | val2")
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			return master, slaves, helper
		}

		g.By("after initial deployment")
		master, _, _ := assertReplicationIsWorking("postgresql-master-1", "postgresql-slave-1", 1)

		g.By("after master is restarted by changing the Deployment Config")
		err = oc.Run("env").Args("dc", "postgresql-master", "POSTGRESQL_ADMIN_PASSWORD=newpass").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), master.GetPodName(), 1*time.Minute)
		master, _, _ = assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		g.By("after master is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=postgresql-master-2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), master.GetPodName(), 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, slaves, _ := assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		g.By("after slave is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=postgresql-slave-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), slaves[0].GetPodName(), 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		pods, err := oc.KubeREST().Pods(oc.Namespace()).List(exutil.ParseLabelsOrDie("deployment=postgresql-slave-1"), nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(1))

		g.By("after slave is scaled to 0 and then back to 4 replicas")
		err = oc.Run("scale").Args("dc", "postgresql-slave", "--replicas=0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), pods.Items[0].Name, 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("scale").Args("dc", "postgresql-slave", "--replicas=4").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 4)
	}
}
