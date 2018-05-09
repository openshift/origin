package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/db"
	testutil "github.com/openshift/origin/test/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	postgreSQLReplicationTemplate = "https://raw.githubusercontent.com/sclorg/postgresql-container/master/examples/replica/postgresql_replica.json"
	postgreSQLEphemeralTemplate   = exutil.FixturePath("..", "..", "examples", "db-templates", "postgresql-ephemeral-template.json")
	postgreSQLHelperName          = "postgresql-helper"
	postgreSQLImages              = []string{
		"postgresql:9.6",
	}
)

var _ = g.Describe("[image_ecosystem][postgresql][Slow][local] openshift postgresql replication", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("postgresql-replication", exutil.KubeConfigPath())

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()

			_, err := exutil.SetupNFSBackedPersistentVolumes(oc, "1Gi", 5)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			defer exutil.RemoveNFSBackedPersistentVolumes(oc)
			defer exutil.RemoveDeploymentConfigs(oc, "postgresql-master", "postgresql-slave")

			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpImageStreams(oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
		})

		for _, image := range postgreSQLImages {
			g.It(fmt.Sprintf("postgresql replication works for %s", image), PostgreSQLReplicationTestFactory(oc, image))
		}
	})
})

// CreatePostgreSQLReplicationHelpers creates a set of PostgreSQL helpers for master,
// slave an en extra helper that is used for remote login test.
func CreatePostgreSQLReplicationHelpers(c kcoreclient.PodInterface, masterDeployment, slaveDeployment, helperDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
	podNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", masterDeployment)), exutil.CheckPodIsRunning, 1, 4*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	masterPod := podNames[0]

	slavePods, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", slaveDeployment)), exutil.CheckPodIsRunning, slaveCount, 6*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create PostgreSQL helper for master
	master := db.NewPostgreSQL(masterPod, "")

	// Create PostgreSQL helpers for slaves
	slaves := make([]exutil.Database, len(slavePods))
	for i := range slavePods {
		slave := db.NewPostgreSQL(slavePods[i], masterPod)
		slaves[i] = slave
	}

	helperNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", helperDeployment)), exutil.CheckPodIsRunning, 1, 4*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	helper := db.NewPostgreSQL(helperNames[0], masterPod)

	return master, slaves, helper
}

func PostgreSQLReplicationTestFactory(oc *exutil.CLI, image string) func() {
	return func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		err := testutil.WaitForPolicyUpdate(oc.InternalKubeClient().Authorization(), oc.Namespace(), "create", templateapi.Resource("templates"), true)
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.CheckOpenShiftNamespaceImageStreams(oc)

		err = oc.Run("create").Args("-f", postgreSQLReplicationTemplate).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.AddNamespaceLabelToPersistentVolumeClaimsInTemplate(oc, "pg-replica-example")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("--template", "pg-replica-example", "-p", fmt.Sprintf("IMAGESTREAMTAG=%s", image)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", postgreSQLEphemeralTemplate, "-p", fmt.Sprintf("DATABASE_SERVICE_NAME=%s", postgreSQLHelperName)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
		// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
		err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), postgreSQLHelperName, 1, true, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), postgreSQLHelperName)
		o.Expect(err).NotTo(o.HaveOccurred())

		tableCounter := 0
		assertReplicationIsWorking := func(masterDeployment, slaveDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
			check := func(err error) {
				if err != nil {
					exutil.DumpApplicationPodLogs("postgresql-master", oc)
					exutil.DumpApplicationPodLogs("postgresql-slave", oc)
				}
				o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
			}

			tableCounter++
			table := fmt.Sprintf("table_%0.2d", tableCounter)

			master, slaves, helper := CreatePostgreSQLReplicationHelpers(oc.KubeClient().CoreV1().Pods(oc.Namespace()), masterDeployment, slaveDeployment, fmt.Sprintf("%s-1", postgreSQLHelperName), slaveCount)
			err := exutil.WaitUntilAllHelpersAreUp(oc, []exutil.Database{master, helper})
			if err != nil {
				exutil.DumpApplicationPodLogs("postgresql-master", oc)
				exutil.DumpApplicationPodLogs("postgresql-helper", oc)
			}
			o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

			err = exutil.WaitUntilAllHelpersAreUp(oc, slaves)
			check(err)

			// Test if we can query as admin
			err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "postgresql-master")
			check(err)
			err = helper.TestRemoteLogin(oc, "postgresql-master")
			check(err)

			// Create a new table with random name
			_, err = master.Query(oc, fmt.Sprintf("CREATE TABLE %s (col1 VARCHAR(20), col2 VARCHAR(20));", table))
			check(err)

			// Write new data to the table through master
			_, err = master.Query(oc, fmt.Sprintf("INSERT INTO %s (col1, col2) VALUES ('val1', 'val2');", table))
			check(err)

			// Make sure data is present on master
			err = exutil.WaitForQueryOutputContains(oc, master, 10*time.Second, false,
				fmt.Sprintf("SELECT * FROM %s;", table),
				"col1 | val1\ncol2 | val2")
			check(err)

			// Make sure data was replicated to all slaves
			for _, slave := range slaves {
				err = exutil.WaitForQueryOutputContains(oc, slave, 90*time.Second, false,
					fmt.Sprintf("SELECT * FROM %s;", table),
					"col1 | val1\ncol2 | val2")
				check(err)
			}

			return master, slaves, helper
		}

		g.By("after initial deployment")
		master, _, _ := assertReplicationIsWorking("postgresql-master-1", "postgresql-slave-1", 1)

		g.By("after master is restarted by changing the Deployment Config")
		err = oc.Run("env").Args("dc", "postgresql-master", "POSTGRESQL_ADMIN_PASSWORD=newpass").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), master.PodName(), 2*time.Minute)
		if err != nil {
			e2e.Logf("Checking if pod %s still exists", master.PodName())
			oc.Run("get").Args("pod", master.PodName(), "-o", "yaml").Execute()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		master, _, _ = assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		g.By("after master is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=postgresql-master-2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), master.PodName(), 2*time.Minute)
		if err != nil {
			e2e.Logf("Checking if pod %s still exists", master.PodName())
			oc.Run("get").Args("pod", master.PodName(), "-o", "yaml").Execute()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		_, slaves, _ := assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		g.By("after slave is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=postgresql-slave-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), slaves[0].PodName(), 2*time.Minute)
		if err != nil {
			e2e.Logf("Checking if pod %s still exists", slaves[0].PodName())
			oc.Run("get").Args("pod", slaves[0].PodName(), "-o", "yaml").Execute()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: exutil.ParseLabelsOrDie("deployment=postgresql-slave-1").String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(1))

		g.By("after slave is scaled to 0 and then back to 4 replicas")
		err = oc.Run("scale").Args("dc", "postgresql-slave", "--replicas=0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), pods.Items[0].Name, 2*time.Minute)
		if err != nil {
			e2e.Logf("Checking if pod %s still exists", pods.Items[0].Name)
			oc.Run("get").Args("pod", pods.Items[0].Name, "-o", "yaml").Execute()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("scale").Args("dc", "postgresql-slave", "--replicas=4").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 4)
	}
}
