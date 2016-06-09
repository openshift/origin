package deployments

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e"

	deployutil "github.com/openshift/origin/pkg/deploy/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

const deploymentRunTimeout = 5 * time.Minute

var _ = g.Describe("deploymentconfigs", func() {
	defer g.GinkgoRecover()
	var (
		oc                      = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())
		deploymentFixture       = exutil.FixturePath("..", "extended", "fixtures", "test-deployment-test.yaml")
		simpleDeploymentFixture = exutil.FixturePath("..", "extended", "fixtures", "deployment-simple.yaml")
		customDeploymentFixture = exutil.FixturePath("..", "extended", "fixtures", "custom-deployment.yaml")
		generationFixture       = exutil.FixturePath("..", "extended", "fixtures", "test-deployment.yaml")
		pausedDeploymentFixture = exutil.FixturePath("..", "extended", "fixtures", "paused-deployment.yaml")
	)

	g.Describe("when run iteratively", func() {
		g.It("should only deploy the last deployment [Conformance]", func() {
			// print some debugging output if the deploymeent fails
			defer func() {
				if !g.CurrentGinkgoTestDescription().Failed {
					return
				}
				if dc, rcs, pods, err := deploymentInfo(oc, "deployment-simple"); err == nil {
					e2e.Logf("DC: %#v", dc)
					e2e.Logf("  RCs: %#v", rcs)
					p, _ := deploymentPods(pods)
					e2e.Logf("  Deployers: %#v", p)
				}
			}()

			_, err := oc.Run("create").Args("-f", simpleDeploymentFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			iterations := 15
			for i := 0; i < iterations; i++ {
				if rand.Float32() < 0.2 {
					time.Sleep(time.Duration(rand.Float32() * rand.Float32() * float32(time.Second)))
				}
				switch n := rand.Float32(); {

				case n < 0.4:
					// trigger a new deployment
					e2e.Logf("%02d: triggering a new deployment with config change", i)
					out, err := oc.Run("set", "env").Args("dc/deployment-simple", fmt.Sprintf("A=%d", i)).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(out).To(o.ContainSubstring("updated"))

				case n < 0.7:
					// cancel any running deployment
					e2e.Logf("%02d: cancelling deployment", i)
					if out, err := oc.Run("deploy").Args("dc/deployment-simple", "--cancel").Output(); err != nil {
						// TODO: we should fix this
						if !strings.Contains(out, "the object has been modified") {
							o.Expect(err).NotTo(o.HaveOccurred())
						}
						e2e.Logf("--cancel deployment failed due to conflict: %v", err)
					}

				case n < 0.0:
					// delete the deployer pod - disabled because it forces the system to wait for the sync loop
					e2e.Logf("%02d: deleting one or more deployer pods", i)
					_, rcs, pods, err := deploymentInfo(oc, "deployment-simple")
					if err != nil {
						e2e.Logf("%02d: unable to get deployment info: %v", i, err)
						continue
					}
					all, err := deploymentPods(pods)
					if err != nil {
						e2e.Logf("%02d: unable to get deployment pods: %v", i, err)
						continue
					}
					if len(all) == 0 {
						e2e.Logf("%02d: no deployer pods", i)
						continue
					}
					top := len(rcs) - 1
					for j := top; i >= top-1 && j >= 0; j-- {
						pods, ok := all[rcs[j].Name]
						if !ok {
							e2e.Logf("%02d: no deployer pod for rc %q", i, rcs[j].Name)
							continue
						}
						for _, pod := range pods {
							e2e.Logf("%02d: deleting deployer pod %s", i, pod.Name)
							options := kapi.NewDeleteOptions(0)
							if rand.Float32() < 0.5 {
								options = nil
							}
							if err := oc.KubeREST().Pods(oc.Namespace()).Delete(pod.Name, options); err != nil {
								e2e.Logf("%02d: unable to delete deployer pod %q: %v", i, pod.Name, err)
							}
						}
					}
					e2e.Logf("%02d: triggering a new deployment with config change", i)
					out, err := oc.Run("set", "env").Args("dc/deployment-simple", fmt.Sprintf("A=%d", i)).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(out).To(o.ContainSubstring("updated"))

				default:
					// wait for the deployment to be running
					e2e.Logf("%02d: waiting for current deployment to start running", i)
					o.Expect(waitForLatestCondition(oc, "deployment-simple", deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())
				}
			}

			// trigger one more deployment, just in case we cancelled the latest output
			out, err := oc.Run("set", "env").Args("dc/deployment-simple", fmt.Sprintf("A=%d", iterations)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("updated"))

			g.By("verifying all but terminal deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, "deployment-simple", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with test deployments", func() {
		g.It("should run a deployment to completion and then scale to zero [Conformance]", func() {
			out, err := oc.Run("create").Args("-f", deploymentFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

			out, err = oc.Run("logs").Args("-f", "dc/deployment-test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("deployment-test-1 to 2"))
			o.Expect(out).To(o.ContainSubstring("--> pre: Success"))
			o.Expect(out).To(o.ContainSubstring("--> Success"))

			g.By("verifying the deployment is marked complete and scaled to zero")
			o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that scaling does not result in new pods")
			out, err = oc.Run("scale").Args("dc/deployment-test", "--replicas=1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring no scale up of the deployment happens")
			wait.Poll(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				rc, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get("deployment-test-1")
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(rc.Spec.Replicas).Should(o.BeEquivalentTo(0))
				o.Expect(rc.Status.Replicas).Should(o.BeEquivalentTo(0))
				return false, nil
			})

			g.By("verifying the scale is updated on the deployment config")
			config, err := oc.REST().DeploymentConfigs(oc.Namespace()).Get("deployment-test")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(config.Spec.Replicas).Should(o.BeEquivalentTo(1))
			o.Expect(config.Spec.Test).Should(o.BeTrue())

			g.By("deploying a few more times")
			for i := 0; i < 3; i++ {
				out, err = oc.Run("deploy").Args("--latest", "deployment-test").Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

				out, err = oc.Run("logs").Args("-f", "dc/deployment-test").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
				o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("deployment-test-%d up to 1", i+2)))
				o.Expect(out).To(o.ContainSubstring("--> pre: Success"))
				o.Expect(out).To(o.ContainSubstring("test pre hook executed"))
				o.Expect(out).To(o.ContainSubstring("--> Success"))

				g.By("verifying the deployment is marked complete and scaled to zero")
				o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
			}
		})
	})

	g.Describe("with custom deployments", func() {
		g.It("should run the custom deployment steps [Conformance]", func() {
			out, err := oc.Run("create").Args("-f", customDeploymentFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, "custom-deployment", deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

			out, err = oc.Run("logs").Args("-f", "dc/custom-deployment").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("--> pre: Running hook pod ..."))
			o.Expect(out).To(o.ContainSubstring("test pre hook executed"))
			o.Expect(out).To(o.ContainSubstring("--> Scaling custom-deployment-1 to 2"))
			o.Expect(out).To(o.ContainSubstring("--> Reached 50%"))
			o.Expect(out).To(o.ContainSubstring("Halfway"))
			o.Expect(out).To(o.ContainSubstring("Finished"))
			o.Expect(out).To(o.ContainSubstring("--> Success"))

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, "custom-deployment", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("generation", func() {
		g.It("should deploy based on a status version bump", func() {
			resource, name, err := createFixture(oc, generationFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that both latestVersion and generation are updated")
			version, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
			version = strings.Trim(version, "\"")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the latest version for %s: %s", resource, version))
			o.Expect(version).To(o.ContainSubstring("1"))
			generation, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.metadata.generation}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the generation for %s: %s", resource, generation))
			o.Expect(generation).To(o.ContainSubstring("1"))

			g.By("verifying the deployment is marked complete")
			err = wait.Poll(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				rc, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get(name + "-" + version)
				o.Expect(err).NotTo(o.HaveOccurred())
				return deployutil.IsTerminatedDeployment(rc), nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that scaling updates the generation")
			_, err = oc.Run("scale").Args(resource, "--replicas=2").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			generation, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.metadata.generation}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the generation for %s: %s", resource, generation))
			o.Expect(generation).To(o.ContainSubstring("2"))

			g.By("deploying a second time [new client]")
			_, err = oc.Run("deploy").Args("--latest", name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that both latestVersion and generation are updated")
			version, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
			version = strings.Trim(version, "\"")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the latest version for %s: %s", resource, version))
			o.Expect(version).To(o.ContainSubstring("2"))
			generation, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.metadata.generation}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the generation for %s: %s", resource, generation))
			o.Expect(generation).To(o.ContainSubstring("3"))

			g.By("verifying that observedGeneration equals generation")
			err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, name)
				o.Expect(err).NotTo(o.HaveOccurred())
				return deployutil.HasSynced(dc), nil
			})
		})
	})

	g.Describe("paused", func() {
		g.It("should never run a new deployment", func() {
			resource, name, err := createFixture(oc, pausedDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, rcs, _, err := deploymentInfo(oc, name)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}

			out, err := oc.Run("deploy").Args(resource, "--latest").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot deploy a paused deploymentconfig"))

			out, err = oc.Run("deploy").Args(resource, "--cancel").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot cancel a paused deploymentconfig"))

			out, err = oc.Run("deploy").Args(resource, "--retry").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot retry a paused deploymentconfig"))

			out, err = oc.Run("rollback").Args(resource, "--to-version", "1").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot rollback a paused deploymentconfig"))

			dc, rcs, _, err := deploymentInfo(oc, name)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}

			dc.Spec.Paused = false
			_, err = oc.REST().DeploymentConfigs(dc.Namespace).Update(dc)
			// TODO: Retry on update conflicts
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})
})
