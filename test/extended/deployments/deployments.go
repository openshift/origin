package deployments

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

const deploymentRunTimeout = 5 * time.Minute

var _ = g.Describe("deploymentconfigs", func() {
	defer g.GinkgoRecover()
	var (
		oc                              = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())
		deploymentFixture               = exutil.FixturePath("testdata", "test-deployment-test.yaml")
		simpleDeploymentFixture         = exutil.FixturePath("testdata", "deployment-simple.yaml")
		customDeploymentFixture         = exutil.FixturePath("testdata", "custom-deployment.yaml")
		generationFixture               = exutil.FixturePath("testdata", "generation-test.yaml")
		pausedDeploymentFixture         = exutil.FixturePath("testdata", "paused-deployment.yaml")
		failedHookFixture               = exutil.FixturePath("testdata", "failing-pre-hook.yaml")
		brokenDeploymentFixture         = exutil.FixturePath("testdata", "test-deployment-broken.yaml")
		historyLimitedDeploymentFixture = exutil.FixturePath("testdata", "deployment-history-limit.yaml")
		minReadySecondsFixture          = exutil.FixturePath("testdata", "deployment-min-ready-seconds.yaml")
		multipleICTFixture              = exutil.FixturePath("testdata", "deployment-example.yaml")
	)

	g.Describe("when run iteratively", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should only deploy the last deployment [Conformance]", func() {
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

		g.It("should immediately start a new deployment [Conformance]", func() {
			resource, name, err := createFixture(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = oc.Run("set", "env").Args(resource, "TRY=ONCE").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the deployment config has the correct version"))
			err = wait.PollImmediate(500*time.Millisecond, 5*time.Second, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, name)
				if err != nil {
					return false, nil
				}
				return dc.Status.LatestVersion == 2, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the second deployment exists"))
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				_, rcs, _, err := deploymentInfo(oc, name)
				if err != nil {
					return false, nil
				}

				secondDeploymentExists := false
				for _, rc := range rcs {
					if rc.Name == deployutil.DeploymentNameForConfigVersion(name, 2) {
						secondDeploymentExists = true
						break
					}
				}

				return secondDeploymentExists, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the first deployer was deleted and the second deployer exists"))
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				_, _, pods, err := deploymentInfo(oc, name)
				if err != nil {
					return false, nil
				}

				deploymentNamesToDeployers, err := deploymentPods(pods)
				if err != nil {
					return false, nil
				}

				firstDeploymentName := deployutil.DeploymentNameForConfigVersion(name, 1)
				firstDeployerRemoved := len(deploymentNamesToDeployers[firstDeploymentName]) == 0

				secondDeploymentName := deployutil.DeploymentNameForConfigVersion(name, 2)
				secondDeployerExists := len(deploymentNamesToDeployers[secondDeploymentName]) == 1

				return firstDeployerRemoved && secondDeployerExists, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with test deployments", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-test", g.CurrentGinkgoTestDescription().Failed)
		})

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
				out, err = oc.Run("deploy").Args("--latest", "--follow", "deployment-test").Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying the deployment is marked complete and scaled to zero")
				o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
				o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("deployment-test-%d up to 1", i+2)))
				o.Expect(out).To(o.ContainSubstring("--> pre: Success"))
				o.Expect(out).To(o.ContainSubstring("test pre hook executed"))
				o.Expect(out).To(o.ContainSubstring("--> Success"))
			}
		})
	})

	g.Describe("with multiple image change triggers", func() {
		g.AfterEach(func() {
			failureTrap(oc, "example", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run a successful deployment [Conformance]", func() {
			_, name, err := createFixture(oc, multipleICTFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with enhanced status", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should include various info in status [Conformance]", func() {
			resource, name, err := createFixture(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that status.replicas is set")
			replicas, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.replicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(replicas).To(o.ContainSubstring("2"))
			g.By("verifying that status.updatedReplicas is set")
			updatedReplicas, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.updatedReplicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(updatedReplicas).To(o.ContainSubstring("2"))
			g.By("verifying that status.availableReplicas is set")
			availableReplicas, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.availableReplicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(availableReplicas).To(o.ContainSubstring("2"))
			g.By("verifying that status.unavailableReplicas is set")
			unavailableReplicas, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.unavailableReplicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(unavailableReplicas).To(o.ContainSubstring("0"))
		})
	})

	g.Describe("with custom deployments", func() {
		g.AfterEach(func() {
			failureTrap(oc, "custom-deployment", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run the custom deployment steps [Conformance]", func() {
			out, err := oc.Run("create").Args("-f", customDeploymentFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, "custom-deployment", deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

			out, err = oc.Run("deploy").Args("--follow", "dc/custom-deployment").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, "custom-deployment", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("--> pre: Running hook pod ..."))
			o.Expect(out).To(o.ContainSubstring("test pre hook executed"))
			o.Expect(out).To(o.ContainSubstring("--> Scaling custom-deployment-1 to 2"))
			o.Expect(out).To(o.ContainSubstring("--> Reached 50%"))
			o.Expect(out).To(o.ContainSubstring("Halfway"))
			o.Expect(out).To(o.ContainSubstring("Finished"))
			o.Expect(out).To(o.ContainSubstring("--> Success"))
		})
	})

	g.Describe("viewing rollout history", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should print the rollout history [Conformance]", func() {
			resource, name, err := createFixture(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			_, err = oc.REST().DeploymentConfigs(oc.Namespace()).Get(name)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = client.UpdateConfigWithRetries(oc.REST(), oc.Namespace(), name, func(dc *deployapi.DeploymentConfig) {
				one := int64(1)
				dc.Spec.Template.Spec.TerminationGracePeriodSeconds = &one
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			out, err := oc.Run("rollout").Args("history", resource).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the history for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("deploymentconfigs \"deployment-simple\" history viewed"))
			o.Expect(out).To(o.ContainSubstring("REVISION	STATUS		CAUSE"))
			o.Expect(out).To(o.ContainSubstring("1		Complete	caused by a config change"))
			o.Expect(out).To(o.ContainSubstring("2		Complete	caused by a config change"))
		})
	})

	g.Describe("generation", func() {
		g.AfterEach(func() {
			failureTrap(oc, "generation-test", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should deploy based on a status version bump [Conformance]", func() {
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
		g.AfterEach(func() {
			failureTrap(oc, "paused", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should disable actions on deployments [Conformance]", func() {
			resource, name, err := createFixture(oc, pausedDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, rcs, _, err := deploymentInfo(oc, name)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}

			g.By("verifying that we cannot start a new deployment")
			out, err := oc.Run("deploy").Args(resource, "--latest").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot deploy a paused deployment config"))

			g.By("verifying that we cannot cancel a deployment")
			out, err = oc.Run("deploy").Args(resource, "--cancel").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot cancel a paused deployment config"))

			g.By("verifying that we cannot retry a deployment")
			out, err = oc.Run("deploy").Args(resource, "--retry").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot retry a paused deployment config"))

			g.By("verifying that we cannot rollback a deployment")
			out, err = oc.Run("rollback").Args(resource, "--to-version", "1").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot rollback a paused deployment config"))

			_, rcs, _, err = deploymentInfo(oc, name)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}

			_, err = client.UpdateConfigWithRetries(oc.REST(), oc.Namespace(), name, func(dc *deployapi.DeploymentConfig) {
				// TODO: oc rollout pause should patch instead of making a full update
				dc.Spec.Paused = false
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with failing hook", func() {
		g.AfterEach(func() {
			failureTrap(oc, "hook", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should get all logs from retried hooks [Conformance]", func() {
			resource, name, err := createFixture(oc, failedHookFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentPreHookRetried)).NotTo(o.HaveOccurred())

			out, err := oc.Run("logs").Args(resource).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("--> pre: Running hook pod ..."))
			o.Expect(out).To(o.ContainSubstring("no such file or directory"))
			o.Expect(out).To(o.ContainSubstring("--> pre: Retrying hook pod (retry #1)"))
		})
	})

	g.Describe("rolled back", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should rollback to an older deployment [Conformance]", func() {
			resource, name, err := createFixture(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			_, err = oc.Run("deploy").Args(name, "--latest").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that we are on the second version")
			version, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			version = strings.Trim(version, "\"")
			o.Expect(version).To(o.ContainSubstring("2"))

			g.By("verifying that we can rollback")
			_, err = oc.Run("rollout").Args("undo", resource).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that we are on the third version")
			version, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			version = strings.Trim(version, "\"")
			o.Expect(version).To(o.ContainSubstring("3"))
		})
	})

	g.Describe("reaper", func() {
		g.AfterEach(func() {
			failureTrap(oc, "brokendeployment", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should delete all failed deployer pods and hook pods [Conformance]", func() {
			resource, name, err := createFixture(oc, brokenDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			err = waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)
			o.Expect(err).To(o.HaveOccurred())

			g.By("fetching the deployer pod")
			out, err := oc.Run("get").Args("pod", fmt.Sprintf("%s-1-deploy", name)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Error"))

			g.By("fetching the pre-hook pod")
			out, err = oc.Run("get").Args("pod", fmt.Sprintf("%s-1-hook-pre", name)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Error"))

			g.By("deleting the deployment config")
			out, err = oc.Run("delete").Args(resource).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("fetching the deployer pod")
			out, err = oc.Run("get").Args("pod", fmt.Sprintf("%s-1-deploy", name)).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("not found"))

			g.By("fetching the pre-hook pod")
			out, err = oc.Run("get").Args("pod", fmt.Sprintf("%s-1-hook-pre", name)).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("not found"))
		})
	})

	g.Describe("with revision history limits", func() {
		g.AfterEach(func() {
			failureTrap(oc, "history-limit", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should never persist more old deployments than acceptable after being observed by the controller [Conformance]", func() {
			revisionHistoryLimit := 3 // as specified in the fixture

			_, err := oc.Run("create").Args("-f", historyLimitedDeploymentFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			iterations := 10
			for i := 0; i < iterations; i++ {
				o.Expect(waitForLatestCondition(oc, "history-limit", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred(),
					"the current deployment needs to have finished before attempting to trigger a new deployment through configuration change")
				e2e.Logf("%02d: triggering a new deployment with config change", i)
				out, err := oc.Run("set", "env").Args("dc/history-limit", fmt.Sprintf("A=%d", i)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("updated"))
			}

			o.Expect(waitForLatestCondition(oc, "history-limit", deploymentRunTimeout, checkDeploymentConfigHasSynced)).NotTo(o.HaveOccurred(),
				"the controller needs to have synced with the updated deployment configuration before checking that the revision history limits are being adhered to")
			deploymentConfig, deployments, _, err := deploymentInfo(oc, "history-limit")
			o.Expect(err).NotTo(o.HaveOccurred())
			// sanity check to ensure that the following asertion on the amount of old deployments is valid
			o.Expect(*deploymentConfig.Spec.RevisionHistoryLimit).To(o.Equal(int32(revisionHistoryLimit)))

			// we need to filter out any deployments that we don't care about,
			// namely the active deployment and any newer deployments
			oldDeployments := deployutil.DeploymentsForCleanup(deploymentConfig, deployments)

			// we should not have more deployments than acceptable
			o.Expect(len(oldDeployments)).To(o.BeNumerically("<=", revisionHistoryLimit))

			// the deployments we continue to keep should be the latest ones
			for _, deployment := range oldDeployments {
				o.Expect(deployutil.DeploymentVersionFor(&deployment)).To(o.BeNumerically(">", iterations-revisionHistoryLimit))
			}
		})
	})

	g.Describe("with minimum ready seconds set", func() {
		g.AfterEach(func() {
			failureTrap(oc, "minreadytest", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should not transition the deployment to Complete before satisfied [Conformance]", func() {
			_, name, err := createFixture(oc, minReadySecondsFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked running")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

			g.By("verifying that all pods are ready")
			config, err := oc.REST().DeploymentConfigs(oc.Namespace()).Get(name)
			o.Expect(err).NotTo(o.HaveOccurred())

			selector := labels.Set(config.Spec.Selector).AsSelector()
			opts := kapi.ListOptions{LabelSelector: selector}
			ready := 0
			if err := wait.PollImmediate(500*time.Millisecond, 1*time.Minute, func() (bool, error) {
				pods, err := oc.KubeREST().Pods(oc.Namespace()).List(opts)
				if err != nil {
					return false, nil
				}

				ready = 0
				for i := range pods.Items {
					pod := pods.Items[i]
					if kapi.IsPodReady(&pod) {
						ready++
					}
				}

				return len(pods.Items) == ready, nil
			}); err != nil {
				o.Expect(fmt.Errorf("deployment config %q never became ready (ready: %d, desired: %d)",
					config.Name, ready, config.Spec.Replicas)).NotTo(o.HaveOccurred())
			}

			g.By("verifying that the deployment is still running")
			latestName := deployutil.DeploymentNameForConfigVersion(name, config.Status.LatestVersion)
			latest, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get(latestName)
			o.Expect(err).NotTo(o.HaveOccurred())
			if deployutil.IsTerminatedDeployment(latest) {
				o.Expect(fmt.Errorf("expected deployment %q not to have terminated", latest.Name)).NotTo(o.HaveOccurred())
			}
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())
		})
	})
})
