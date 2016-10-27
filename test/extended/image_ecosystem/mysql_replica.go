package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	templateapi "github.com/openshift/origin/pkg/template/api"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/db"
	testutil "github.com/openshift/origin/test/util"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
)

type testCase struct {
	Version         string
	TemplatePath    string
	SkipReplication bool
}

var (
	testCases = []testCase{
		{
			"5.5",
			"https://raw.githubusercontent.com/sclorg/mysql-container/master/5.5/examples/replica/mysql_replica.json",
			// NOTE: Set to true in case of flakes.
			false,
		},
		{
			"5.6",
			"https://raw.githubusercontent.com/sclorg/mysql-container/master/5.6/examples/replica/mysql_replica.json",
			false,
		},
	}
	helperTemplate = exutil.FixturePath("..", "..", "examples", "db-templates", "mysql-ephemeral-template.json")
	helperName     = "mysql-helper"
)

// CreateMySQLReplicationHelpers creates a set of MySQL helpers for master,
// slave and an extra helper that is used for remote login test.
func CreateMySQLReplicationHelpers(c kclient.PodInterface, masterDeployment, slaveDeployment, helperDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
	podNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", masterDeployment)), exutil.CheckPodIsRunningFn, 1, 1*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	masterPod := podNames[0]

	slavePods, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", slaveDeployment)), exutil.CheckPodIsRunningFn, slaveCount, 2*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create MySQL helper for master
	master := db.NewMysql(masterPod, "")

	// Create MySQL helpers for slaves
	slaves := make([]exutil.Database, len(slavePods))
	for i := range slavePods {
		slave := db.NewMysql(slavePods[i], masterPod)
		slaves[i] = slave
	}

	helperNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", helperDeployment)), exutil.CheckPodIsRunningFn, 1, 1*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	helper := db.NewMysql(helperNames[0], masterPod)

	return master, slaves, helper
}

func cleanup(oc *exutil.CLI) {
	exutil.DumpImageStreams(oc)
	oc.AsAdmin().Run("delete").Args("all", "--all", "-n", oc.Namespace()).Execute()
	exutil.DumpImageStreams(oc)
	oc.AsAdmin().Run("delete").Args("pvc", "--all", "-n", oc.Namespace()).Execute()
	exutil.CleanupHostPathVolumes(oc.AdminKubeREST().PersistentVolumes(), oc.Namespace())
}

func replicationTestFactory(oc *exutil.CLI, tc testCase) func() {
	return func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer cleanup(oc)

		_, err := exutil.SetupHostPathVolumes(oc.AdminKubeREST().PersistentVolumes(), oc.Namespace(), "1Gi", 2)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = testutil.WaitForPolicyUpdate(oc.REST(), oc.Namespace(), "create", templateapi.Resource("templates"), true)
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.CheckOpenShiftNamespaceImageStreams(oc)
		err = oc.Run("new-app").Args("-f", tc.TemplatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", helperTemplate, "-p", fmt.Sprintf("DATABASE_SERVICE_NAME=%s", helperName)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
		// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
		g.By("waiting for the deployment to complete")
		err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), helperName, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for an endpoint")
		err = oc.KubeFramework().WaitForAnEndpoint(helperName)
		o.Expect(err).NotTo(o.HaveOccurred())

		tableCounter := 0
		assertReplicationIsWorking := func(masterDeployment, slaveDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
			tableCounter++
			table := fmt.Sprintf("table_%0.2d", tableCounter)

			g.By("creating replication helpers")
			master, slaves, helper := CreateMySQLReplicationHelpers(oc.KubeREST().Pods(oc.Namespace()), masterDeployment, slaveDeployment, fmt.Sprintf("%s-1", helperName), slaveCount)
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, []exutil.Database{master, helper})).NotTo(o.HaveOccurred())
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, slaves)).NotTo(o.HaveOccurred())

			// Test if we can query as root
			g.By("wait for mysql-master endpoint")
			oc.KubeFramework().WaitForAnEndpoint("mysql-master")
			err := helper.TestRemoteLogin(oc, "mysql-master")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Create a new table with random name
			g.By("create new table")
			_, err = master.Query(oc, fmt.Sprintf("CREATE TABLE %s (col1 VARCHAR(20), col2 VARCHAR(20));", table))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Write new data to the table through master
			_, err = master.Query(oc, fmt.Sprintf("INSERT INTO %s (col1, col2) VALUES ('val1', 'val2');", table))
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure data is present on master
			err = exutil.WaitForQueryOutputContains(oc, master, 10*time.Second, false, fmt.Sprintf("SELECT * FROM %s\\G;", table), "col1: val1\ncol2: val2")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Make sure data was replicated to all slaves
			for _, slave := range slaves {
				err = exutil.WaitForQueryOutputContains(oc, slave, 90*time.Second, false, fmt.Sprintf("SELECT * FROM %s\\G;", table), "col1: val1\ncol2: val2")
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			return master, slaves, helper
		}

		g.By("after initial deployment")
		master, _, _ := assertReplicationIsWorking("mysql-master-1", "mysql-slave-1", 1)

		if tc.SkipReplication {
			return
		}

		g.By("after master is restarted by changing the Deployment Config")
		err = oc.Run("env").Args("dc", "mysql-master", "MYSQL_ROOT_PASSWORD=newpass").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), master.PodName(), 1*time.Minute)
		master, _, _ = assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		g.By("after master is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=mysql-master-2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), master.PodName(), 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, slaves, _ := assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		g.By("after slave is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=mysql-slave-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), slaves[0].PodName(), 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		pods, err := oc.KubeREST().Pods(oc.Namespace()).List(kapi.ListOptions{LabelSelector: exutil.ParseLabelsOrDie("deployment=mysql-slave-1")})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(1))

		// NOTE: Commented out, current template does not support multiple replicas.
		/*
			g.By("after slave is scaled to 0 and then back to 4 replicas")
			err = oc.Run("scale").Args("dc", "mysql-slave", "--replicas=0").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), pods.Items[0].Name, 1*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("scale").Args("dc", "mysql-slave", "--replicas=4").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 4)
		*/
	}
}

var _ = g.Describe("[image_ecosystem][mysql][Slow] openshift mysql replication", func() {
	defer g.GinkgoRecover()

	ocs := make([]*exutil.CLI, len(testCases))
	for i, tc := range testCases {
		ocs[i] = exutil.NewCLI(fmt.Sprintf("mysql-replication-%d", i), exutil.KubeConfigPath())
		g.It(fmt.Sprintf("MySQL replication template for %s: %s", tc.Version, tc.TemplatePath), replicationTestFactory(ocs[i], tc))
	}
})
