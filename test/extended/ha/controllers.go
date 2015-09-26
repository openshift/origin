package ha

/*
This controllers test suite is not part of the "default" group, because its
testing involves terminating and launching controllers components of OpenShift.
Therefore it can't be run in parallel.
*/

import (
	"fmt"
	"os/exec"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	reExitCode *regexp.Regexp
	m          *exutil.CMonitor
)

func init() {
	reExitCode = regexp.MustCompile(`(?i)exitcode:(\d+)`)
}

var _ = g.BeforeSuite(func() {
	mgr, err := exutil.NewCMonitor(exutil.MasterConfigPath(), 0, 8444, exutil.TestContext.OutputDir)
	o.Expect(err).NotTo(o.HaveOccurred())
	m = mgr

	// One controllers instance needs to be running before CLI can be initialized
	_, err = m.StartNewInstance()
	o.Expect(err).NotTo(o.HaveOccurred())
	_, _, err = m.WaitForActive(time.Second * 2)
	o.Expect(err).NotTo(o.HaveOccurred())

	fmt.Fprintf(g.GinkgoWriter, "Deploying internal registry\n")
	deployRegistry()
})

var _ = g.AfterSuite(func() {
	fmt.Fprintf(g.GinkgoWriter, "Releasing all controllers\n")
	m.ReleaseControllers(m.GetAlive()...)
})

var _ = g.Describe("ha: Election of OpenShift controllers", func() {
	defer g.GinkgoRecover()
	var (
		dcName           = "recreate-example"
		deploymentConfig = exutil.FixturePath("..", "..", "examples", "deployment", dcName+".yaml")
		oc               = exutil.NewCLI("controller-leases", exutil.KubeConfigPath())
	)

	// Check whether a pod can be deployed
	TestDeployment := func() {
		g.By(fmt.Sprintf("Creating a pod from %s", deploymentConfig))
		err := oc.Run("create").Args("-f", deploymentConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("Waiting for pod %s to be deployed", dcName))
		err = exutil.WaitForADeployment(oc.KubeREST().ReplicationControllers(oc.Namespace()), dcName,
			exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	// EnsureOneInstanceActive releases any inactive controllers instances and
	// makes sure that one is active.
	EnsureOneInstanceActive := func() {
		cnt := m.Len()
		if cnt > 1 {
			g.By("Releasing inactive controllers")
			m.ReleaseControllers(m.GetInactive()...)
		} else if cnt == 0 {
			g.By("Launching initial controllers instance")
			// Speed up controllers election by deleting current lease
			m.DeleteLease()
			_, err := m.StartNewInstance()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(m.Len()).To(o.Equal(1))
			_, _, err = m.WaitForActive(time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		o.Expect(m.Len()).To(o.Equal(1))
	}

	TerminateActive := func() *exutil.Controllers {
		g.By("Terminating active controllers instance")
		ctrls, err := m.GetActive()
		o.Expect(err).NotTo(o.HaveOccurred())
		m.ReleaseControllers(ctrls)
		o.Expect(ctrls.Exited()).To(o.BeTrue())
		return ctrls
	}

	g.JustBeforeEach(func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		o.Expect(m.Len()).To(o.Equal(1))
		g.By("Waiting for deployer service account")
		err := exutil.WaitForDeployerAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for internal docker registry")
		waitForRegistry(oc)
	})

	g.AfterEach(func() {
		EnsureOneInstanceActive()
	})

	g.Describe("Delete an active lease", func() {
		g.Context("When there are waiting controllers instances", func() {
			g.JustBeforeEach(func() {
				g.By("Launch another 2 controllers instances")
				m.StartNewInstance()
				m.StartNewInstance()
				o.Expect(m.Len()).To(o.Equal(3))
			})

			g.It("should terminate an active instance and cause a waiting one to get a lease", func() {
				ctrls, _, err := m.WaitForActive(time.Second)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Deleting current controllers lease")
				o.Expect(m.DeleteLease()).NotTo(o.HaveOccurred())
				newCtrls, delay, err := m.WaitForActive(time.Duration(m.LeaseTTL) * time.Second)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(delay < time.Duration(m.LeaseTTL)*time.Second).To(o.BeTrue())
				o.Expect(ctrls).NotTo(o.Equal(newCtrls))
				oldLid, err := ctrls.GetLeaseID(false)
				o.Expect(err).NotTo(o.HaveOccurred())
				newLid, err := newCtrls.GetLeaseID(false)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(oldLid).NotTo(o.Equal(newLid))

				TestDeployment()
			})
		})

		g.Context("When there is no waiting instance left", func() {
			g.It("should terminate an active instance", func() {
				o.Expect(m.Len()).To(o.Equal(1))
				ctrls, delay, err := m.WaitForActive(time.Second)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(delay <= time.Second).To(o.BeTrue())

				g.By("Deleting current controllers lease")
				o.Expect(m.DeleteLease()).NotTo(o.HaveOccurred())
				err = ctrls.WaitWithTimeout(time.Second)
				o.Expect(err).To(o.HaveOccurred())
				_, ok := err.(*exec.ExitError)
				o.Expect(ok).To(o.BeTrue())
				o.Expect(ctrls.Exited()).To(o.BeTrue())
				o.Expect(m.GetAlive()).To(o.Equal([]*exutil.Controllers{}))

				// One controllers instance must be active for e2e's cleanup to work
				EnsureOneInstanceActive()
			})
		})
	})

	g.Describe("Terminate an active controllers instance", func() {
		g.Context("When there are waiting controllers instances", func() {
			g.JustBeforeEach(func() {
				g.By("Launch another 2 controllers instances")
				m.StartNewInstance()
				m.StartNewInstance()
				o.Expect(m.Len()).To(o.Equal(3))
			})

			g.It("sould cause a waiting one to get a lease", func() {
				prevCtrls := TerminateActive()

				currCtrls, delay, err := m.WaitForActive(time.Duration(m.LeaseTTL) * time.Second)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(delay < time.Duration(m.LeaseTTL)*time.Second).To(o.BeTrue())
				o.Expect(prevCtrls).NotTo(o.Equal(currCtrls))
				oldLid, err := prevCtrls.GetLeaseID(false)
				o.Expect(err).NotTo(o.HaveOccurred())
				newLid, err := currCtrls.GetLeaseID(false)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(oldLid).NotTo(o.Equal(newLid))

				TestDeployment()
			})
		})

		g.Context("When there is no waiting instance left", func() {
			g.It("should terminate an active instance", func() {
				o.Expect(m.Len()).To(o.Equal(1))

				TerminateActive()

				o.Expect(m.GetAlive()).To(o.Equal([]*exutil.Controllers{}))

				// One controllers instance must be active for e2e's cleanup to work
				EnsureOneInstanceActive()
			})
		})
	})

})

func deployRegistry() {
	cmd := exec.Command("openshift", "admin", "registry",
		"--config="+exutil.KubeConfigPath(),
		"--credentials="+exutil.RegistryKubeConfig(),
		"--images="+exutil.UseImages(),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		g.Fail(fmt.Sprintf("Failed to create registry: %v, output:\n%s\n", err, out))
	}
}

func waitForRegistry(oc *exutil.CLI) {
	username := oc.Username()
	namespace := oc.Namespace()
	oc.ChangeUser("admin")
	defer oc.ChangeUser(username)
	oc.SetNamespace("default")
	defer oc.SetNamespace(namespace)
	err := oc.KubeFramework().WaitForAnEndpoint("docker-registry")
	o.Expect(err).NotTo(o.HaveOccurred())
}
