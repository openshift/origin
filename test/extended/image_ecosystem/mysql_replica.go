package image_ecosystem

/*
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

type testCase struct {
	Version         string
	TemplatePath    string
	TemplateName    string
	SkipReplication bool
}

var (
	testCases = []testCase{
		{
			"5.7",
			"https://raw.githubusercontent.com/sclorg/mysql-container/master/examples/replica/mysql_replica.json",
			"mysql-replication-example",
			false,
		},
	}
	helperTemplate = exutil.FixturePath("..", "..", "examples", "db-templates", "mysql-ephemeral-template.json")
	helperName     = "mysql-helper"
)

// CreateMySQLReplicationHelpers creates a set of MySQL helpers for master,
// slave and an extra helper that is used for remote login test.
func CreateMySQLReplicationHelpers(c kcoreclient.PodInterface, masterDeployment, slaveDeployment, helperDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
	podNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", masterDeployment)), exutil.CheckPodIsRunning, 1, 4*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	masterPod := podNames[0]

	slavePods, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", slaveDeployment)), exutil.CheckPodIsRunning, slaveCount, 6*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create MySQL helper for master
	master := db.NewMysql(masterPod, "")

	// Create MySQL helpers for slaves
	slaves := make([]exutil.Database, len(slavePods))
	for i := range slavePods {
		slave := db.NewMysql(slavePods[i], masterPod)
		slaves[i] = slave
	}

	helperNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", helperDeployment)), exutil.CheckPodIsRunning, 1, 4*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	helper := db.NewMysql(helperNames[0], masterPod)

	return master, slaves, helper
}

func replicationTestFactory(oc *exutil.CLI, tc testCase) func() {
	return func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		err := testutil.WaitForPolicyUpdate(oc.InternalKubeClient().Authorization(), oc.Namespace(), "create", templateapi.Resource("templates"), true)
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.CheckOpenShiftNamespaceImageStreams(oc)
		err = oc.Run("create").Args("-f", tc.TemplatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.AddNamespaceLabelToPersistentVolumeClaimsInTemplate(oc, tc.TemplateName)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("--template", tc.TemplateName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", helperTemplate, "-p", fmt.Sprintf("MYSQL_VERSION=%s", tc.Version), "-p", fmt.Sprintf("DATABASE_SERVICE_NAME=%s", helperName)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
		// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
		g.By("waiting for the deployment to complete")
		err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), helperName, 1, true, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for an endpoint")
		err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), helperName)
		o.Expect(err).NotTo(o.HaveOccurred())

		tableCounter := 0
		assertReplicationIsWorking := func(masterDeployment, slaveDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
			tableCounter++
			table := fmt.Sprintf("table_%0.2d", tableCounter)

			g.By("creating replication helpers")
			master, slaves, helper := CreateMySQLReplicationHelpers(oc.KubeClient().CoreV1().Pods(oc.Namespace()), masterDeployment, slaveDeployment, fmt.Sprintf("%s-1", helperName), slaveCount)
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, []exutil.Database{master, helper})).NotTo(o.HaveOccurred())
			o.Expect(exutil.WaitUntilAllHelpersAreUp(oc, slaves)).NotTo(o.HaveOccurred())

			// Test if we can query as root
			g.By("wait for mysql-master endpoint")
			err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "mysql-master")
			o.Expect(err).NotTo(o.HaveOccurred())
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
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), master.PodName(), 2*time.Minute)
		if err != nil {
			e2e.Logf("Checking if pod %s still exists", master.PodName())
			oc.Run("get").Args("pod", master.PodName(), "-o", "yaml").Execute()
		}
		master, _, _ = assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		g.By("after master is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=mysql-master-2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), master.PodName(), 2*time.Minute)
		if err != nil {
			e2e.Logf("Checking if pod %s still exists", master.PodName())
			oc.Run("get").Args("pod", master.PodName(), "-o", "yaml").Execute()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		_, slaves, _ := assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		g.By("after slave is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=mysql-slave-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), slaves[0].PodName(), 2*time.Minute)
		if err != nil {
			e2e.Logf("Checking if pod %s still exists", slaves[0].PodName())
			oc.Run("get").Args("pod", slaves[0].PodName(), "-o", "yaml").Execute()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 1)

		pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: exutil.ParseLabelsOrDie("deployment=mysql-slave-1").String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(1))

		// NOTE: Commented out, current template does not support multiple replicas.
		/*
			g.By("after slave is scaled to 0 and then back to 4 replicas")
			err = oc.Run("scale").Args("dc", "mysql-slave", "--replicas=0").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), pods.Items[0].Name, 2*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("scale").Args("dc", "mysql-slave", "--replicas=4").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			assertReplicationIsWorking("mysql-master-2", "mysql-slave-1", 4)
*/
/*	}
}

var _ = g.Describe("[image_ecosystem][mysql][Slow] openshift mysql replication", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("mysql-replication", exutil.KubeConfigPath())
	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()

			_, err := exutil.SetupNFSBackedPersistentVolumes(oc, "1Gi", 5)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			defer exutil.RemoveNFSBackedPersistentVolumes(oc)
			defer exutil.RemoveDeploymentConfigs(oc, "mysql-master", "mysql-slave")

			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
		})

		for _, tc := range testCases {
			g.It(fmt.Sprintf("MySQL replication template for %s: %s", tc.Version, tc.TemplatePath), replicationTestFactory(oc, tc))
		}
	})
})
*/
