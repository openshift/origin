package ha

/*
This controllers test suite is not part of the "default" group, because its
testing involves terminating and launching controllers components of OpenShift.
Therefore it can't be run in parallel.
*/

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
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
		podConfig = exutil.FixturePath("fixtures", "hello-world-pod.json")
		oc        = exutil.NewCLI("controller-leases", exutil.KubeConfigPath())
	)

	// WaitForPod blocks until given pod terminates.
	// FIXME: this is probably too complicated
	// FIXME: consider using oc logs directly
	WaitForPod := func(podName string, timeout time.Duration) {
		start := time.Now()
		cmd := exec.Command("oc", "get", "-n", oc.Namespace(), "--watch", "--no-headers", "pod", podName)
		stdout, err := cmd.StdoutPipe()
		o.Expect(err).NotTo(o.HaveOccurred())
		stderr, err := cmd.StderrPipe()
		o.Expect(err).NotTo(o.HaveOccurred())
		ec := make(chan error)
		err = cmd.Start()
		o.Expect(err).NotTo(o.HaveOccurred())

		// log stderr
		go func() {
			io.Copy(g.GinkgoWriter, stderr)
		}()

		// log stdout and look for exited state
		go func() {
			bufReader := bufio.NewReader(stdout)
			for {
				line, err := bufReader.ReadString('\n')
				if err != nil {
					ec <- err
					return
				}
				g.GinkgoWriter.Write([]byte(fmt.Sprintf("`oc get --watch` output: %s", line)))
				submatch := reExitCode.FindStringSubmatch(line)
				if len(submatch) > 1 {
					if submatch[1] != "0" {
						ec <- fmt.Errorf("Pod terminated with unexpected exit code (%s)", submatch[1])
					} else {
						ec <- nil
					}
					return
				}
			}
		}()

		select {
		case err := <-ec:
			if err != nil && err != io.EOF {
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			if err == nil {
				cmd.Process.Kill()
				cmd.Wait()
			} else {
				o.Expect(cmd.Wait()).NotTo(o.HaveOccurred())
			}
		case <-time.After(start.Add(timeout).Sub(time.Now())):
			g.Fail(fmt.Sprintf("Timeout (%s) occurred while waiting for pod's exit", timeout.String()))
		}
	}

	StartPod := func() {
		g.By(fmt.Sprintf("Creating a pod from %s", podConfig))
		err := oc.Run("create").Args("-f", podConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for pod's exit")
		WaitForPod("helloworld-pod", time.Second*50)

		g.By("Check pod's output")
		out, err := oc.Run("logs").Args("--interactive=false", "helloworld-pod").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(strings.ToLower(out), "hello from docker"))
	}

	// EnsureOneInstanceActive releases any inactive controllers instances and
	// makes sure that one is active.
	EnsureOneInstanceActive := func() {
		l := m.Len()
		switch {
		case l > 1:
			g.By("Releasing inactive controllers")
			m.ReleaseControllers(m.GetInactive()...)
		case l == 0:
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

				StartPod()
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

				StartPod()
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
