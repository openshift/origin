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
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	postgreSQLReplicationTemplate = "https://raw.githubusercontent.com/sclorg/postgresql-container/master/examples/replica/postgresql_replica.json"
	postgreSQLEphemeralTemplate   = exutil.FixturePath("..", "..", "examples", "db-templates", "postgresql-ephemeral-template.json")
	postgreSQLHelperName          = "postgresql-helper"
	postgreSQLImages              = []string{
		"postgresql:9.2",
		"postgresql:9.4",
		"postgresql:9.5",
	}
)

var _ = g.Describe("[image_ecosystem][postgresql][Slow][local] openshift postgresql replication", func() {
	defer g.GinkgoRecover()

	for i, image := range postgreSQLImages {
		oc := exutil.NewCLI(fmt.Sprintf("postgresql-replication-%d", i), exutil.KubeConfigPath())
		testFn := PostgreSQLReplicationTestFactory(oc, image)
		g.It(fmt.Sprintf("postgresql replication works for %s", image), testFn)
	}
})

// CreatePostgreSQLReplicationHelpers creates a set of PostgreSQL helpers for master,
// slave an en extra helper that is used for remote login test.
func CreatePostgreSQLReplicationHelpers(c kcoreclient.PodInterface, masterDeployment, slaveDeployment, helperDeployment string, slaveCount int) (exutil.Database, []exutil.Database, exutil.Database) {
	podNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", masterDeployment)), exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	masterPod := podNames[0]

	slavePods, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", slaveDeployment)), exutil.CheckPodIsRunningFn, slaveCount, 3*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Create PostgreSQL helper for master
	master := db.NewPostgreSQL(masterPod, "")

	// Create PostgreSQL helpers for slaves
	slaves := make([]exutil.Database, len(slavePods))
	for i := range slavePods {
		slave := db.NewPostgreSQL(slavePods[i], masterPod)
		slaves[i] = slave
	}

	helperNames, err := exutil.WaitForPods(c, exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", helperDeployment)), exutil.CheckPodIsRunningFn, 1, 1*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
	helper := db.NewPostgreSQL(helperNames[0], masterPod)

	return master, slaves, helper
}

func PostgreSQLReplicationTestFactory(oc *exutil.CLI, image string) func() {
	return func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer cleanup(oc)

		_, err := exutil.SetupHostPathVolumes(oc.AdminKubeClient().CoreV1().PersistentVolumes(), oc.Namespace(), "512Mi", 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = testutil.WaitForPolicyUpdate(oc.Client(), oc.Namespace(), "create", templateapi.Resource("templates"), true)
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.CheckOpenShiftNamespaceImageStreams(oc)
		err = oc.Run("new-app").Args("-f", postgreSQLReplicationTemplate, "-p", fmt.Sprintf("IMAGESTREAMTAG=%s", image)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-app").Args("-f", postgreSQLEphemeralTemplate, "-p", fmt.Sprintf("DATABASE_SERVICE_NAME=%s", postgreSQLHelperName)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
		// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
		err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.Client(), oc.Namespace(), postgreSQLHelperName, 1, oc)
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
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			tableCounter++
			table := fmt.Sprintf("table_%0.2d", tableCounter)

			master, slaves, helper := CreatePostgreSQLReplicationHelpers(oc.KubeClient().CoreV1().Pods(oc.Namespace()), masterDeployment, slaveDeployment, fmt.Sprintf("%s-1", postgreSQLHelperName), slaveCount)
			err := exutil.WaitUntilAllHelpersAreUp(oc, []exutil.Database{master, helper})
			if err != nil {
				exutil.DumpApplicationPodLogs("postgresql-master", oc)
				exutil.DumpApplicationPodLogs("postgresql-helper", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

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
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), master.PodName(), 1*time.Minute)
		master, _, _ = assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		g.By("after master is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=postgresql-master-2").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), master.PodName(), 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		_, slaves, _ := assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		g.By("after slave is restarted by deleting the pod")
		err = oc.Run("delete").Args("pod", "-l", "deployment=postgresql-slave-1").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), slaves[0].PodName(), 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 1)

		pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: exutil.ParseLabelsOrDie("deployment=postgresql-slave-1").String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods.Items)).To(o.Equal(1))

		g.By("after slave is scaled to 0 and then back to 4 replicas")
		err = oc.Run("scale").Args("dc", "postgresql-slave", "--replicas=0").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitUntilPodIsGone(oc.KubeClient().CoreV1().Pods(oc.Namespace()), pods.Items[0].Name, 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("scale").Args("dc", "postgresql-slave", "--replicas=4").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		assertReplicationIsWorking("postgresql-master-2", "postgresql-slave-1", 4)
	}
}
