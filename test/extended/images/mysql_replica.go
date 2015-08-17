package images

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/rand"
)

var (
	templatePaths = []string{
		"https://raw.githubusercontent.com/openshift/mysql/master/5.5/examples/replica/mysql_replica.json",
		"https://raw.githubusercontent.com/openshift/mysql/master/5.6/examples/replica/mysql_replica.json",
	}
	helperTemplate = exutil.FixturePath("..", "..", "examples", "db-templates", "mysql-ephemeral-template.json")
	helperName     = "mysql-helper"
)

// CreateMySQLReplicationHelpers creates a set of MySQL helpers for master,
// slave an en extra helper that is used for remote login test.
func CreateMySQLReplicationHelpers(c kclient.PodInterface, masterDeployment, slaveDeployment, helperDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
	podNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", masterDeployment)), exutil.CheckPodIsRunningFunc, 1, 60*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	masterPod := podNames[0]

	slavePods, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", slaveDeployment)), exutil.CheckPodIsRunningFunc, slaveCount, 120*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create MySQL helper for master
	master := exutil.NewMysql(c, masterPod, "")

	// Create MySQL helpers for slaves
	slaves := make([]exutil.Database, len(slavePods))
	for i := range slavePods {
		slave := exutil.NewMysql(c, slavePods[i], masterPod)
		slaves[i] = slave
	}

	helperNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", helperDeployment)), exutil.CheckPodIsRunningFunc, 1, 60*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	helper := exutil.NewMysql(c, helperNames[0], masterPod)

	return master, slaves, helper
}

func cleanup(oc *exutil.CLI) {
	oc.AsAdmin().Run("delete").Args("all", "--all", "-n", oc.Namespace()).Execute()
	oc.AsAdmin().Run("delete").Args("pvc", "--all", "-n", oc.Namespace()).Execute()
	exutil.CleanupHostPathVolumes(oc.AdminKubeREST().PersistentVolumes(), oc.Namespace())
}

func replicationTestFactory(oc *exutil.CLI, template string) func() {
	return func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer cleanup(oc)

		_, err := exutil.SetupHostPathVolumes(oc.AdminKubeREST().PersistentVolumes(), oc.Namespace(), "512Mi", 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", template).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", helperTemplate, "-p", fmt.Sprintf("DATABASE_SERVICE_NAME=%s", helperName)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.KubeFramework().WaitForAnEndpoint(helperName)
		o.Expect(err).NotTo(o.HaveOccurred())

		assertReplicationIsWorking := func(masterDeployment, slaveDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
			table := fmt.Sprintf("table_%s", rand.String(10))

			master, slaves, helper := CreateMySQLReplicationHelpers(oc.KubeREST().Pods(oc.Namespace()), masterDeployment, slaveDeployment, fmt.Sprintf("%s-1", helperName), slaveCount)
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, []exutil.Database{master, helper})).NotTo(o.HaveOccurred())
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, slaves)).NotTo(o.HaveOccurred())

			// Test if we can query as root
			oc.KubeFramework().WaitForAnEndpoint("mysql-master")
			err := helper.TestRemoteLogin(oc, "mysql-master")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Create a new table with random name
			_, err = master.Query(oc, fmt.Sprintf("CREATE TABLE %s (col1 VARCHAR(20), col2 VARCHAR(20));", table))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Write new data to the table through master
			_, err = master.Query(oc, fmt.Sprintf("INSERT INTO %s (col1, col2) VALUES ('val1', 'val2');", table))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure data is present on master
			err = exutil.WaitForQueryOutput(oc, master, 10*time.Second, false, fmt.Sprintf("SELECT * FROM %s\\G;", table), "col1: val1\ncol2: val2")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure data was replicated to all slaves
			for _, slave := range slaves {
				err = exutil.WaitForQueryOutput(oc, slave, 90*time.Second, false, fmt.Sprintf("SELECT * FROM %s\\G;", table), "col1: val1\ncol2: val2")
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			return master, slaves, helper
		}

		g.By("after initial deployment")
		master, _, _ := assertReplicationIsWorking("mysql-master-1", "mysql-slave-1", 1)

		g.By("after master is restarted by changing the Deployment Config")
		err = oc.Run("env").Args("dc", "mysql-master", "MYSQL_ROOT_PASSWORD=newpass").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), master.GetPodName(), 60*time.Second)
		master, _, _ = assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		g.By("after master is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=mysql-master-2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), master.GetPodName(), 60*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, slaves, _ := assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		g.By("after slave is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=mysql-slave-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), slaves[0].GetPodName(), 60*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		g.By("after slave is scaled to 0 and then back to 4 replicas")
		err = oc.Run("scale").Args("dc", "mysql-slave", "--replicas=0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("scale").Args("dc", "mysql-slave", "--replicas=4").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 4)
	}
}

var _ = g.Describe("images: mysql: replication", func() {
	defer g.GinkgoRecover()

	ocs := make([]*exutil.CLI, len(templatePaths))
	for i, template := range templatePaths {
		ocs[i] = exutil.NewCLI(fmt.Sprintf("mysql-replication-%d", i), exutil.KubeConfigPath())
		g.It(fmt.Sprintf("MySQL replication template %s", template), replicationTestFactory(ocs[i], template))
	}
})
