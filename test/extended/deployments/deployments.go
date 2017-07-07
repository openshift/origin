package deployments

import (
	//"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	exutil "github.com/openshift/origin/test/extended/util"
)

const deploymentRunTimeout = 5 * time.Minute
const deploymentChangeTimeout = 30 * time.Second

var _ = g.Describe("deploymentconfigs", func() {
	defer g.GinkgoRecover()
	var (
		oc                              = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())
		deploymentFixture               = exutil.FixturePath("testdata", "deployments", "test-deployment-test.yaml")
		simpleDeploymentFixture         = exutil.FixturePath("testdata", "deployments", "deployment-simple.yaml")
		customDeploymentFixture         = exutil.FixturePath("testdata", "deployments", "custom-deployment.yaml")
		generationFixture               = exutil.FixturePath("testdata", "deployments", "generation-test.yaml")
		pausedDeploymentFixture         = exutil.FixturePath("testdata", "deployments", "paused-deployment.yaml")
		failedHookFixture               = exutil.FixturePath("testdata", "deployments", "failing-pre-hook.yaml")
		brokenDeploymentFixture         = exutil.FixturePath("testdata", "deployments", "test-deployment-broken.yaml")
		historyLimitedDeploymentFixture = exutil.FixturePath("testdata", "deployments", "deployment-history-limit.yaml")
		minReadySecondsFixture          = exutil.FixturePath("testdata", "deployments", "deployment-min-ready-seconds.yaml")
		multipleICTFixture              = exutil.FixturePath("testdata", "deployments", "deployment-example.yaml")
		resolutionFixture               = exutil.FixturePath("testdata", "deployments", "deployment-image-resolution.yaml")
		anotherMultiICTFixture          = exutil.FixturePath("testdata", "deployments", "multi-ict-deployment.yaml")
		tagImagesFixture                = exutil.FixturePath("testdata", "deployments", "tag-images-deployment.yaml")
		readinessFixture                = exutil.FixturePath("testdata", "deployments", "readiness-test.yaml")
		envRefDeploymentFixture         = exutil.FixturePath("testdata", "deployments", "deployment-with-ref-env.yaml")
		ignoresDeployersFixture         = exutil.FixturePath("testdata", "deployments", "deployment-ignores-deployer.yaml")
		imageChangeTriggerFixture       = exutil.FixturePath("testdata", "deployments", "deployment-trigger.yaml")
	)

	g.Describe("when run iteratively [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should only deploy the last deployment", func() {
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
					if out, err := oc.Run("rollout").Args("cancel", "dc/deployment-simple").Output(); err != nil {
						// TODO: we should fix this
						if !strings.Contains(out, "the object has been modified") {
							o.Expect(err).NotTo(o.HaveOccurred())
						}
						e2e.Logf("rollout cancel deployment failed due to conflict: %v", err)
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
							options := metav1.NewDeleteOptions(0)
							if rand.Float32() < 0.5 {
								options = nil
							}
							if err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(pod.Name, options); err != nil {
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

		g.It("should immediately start a new deployment", func() {
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
			// TODO when #11016 gets fixed this can be reverted to 30seconds
			err = wait.PollImmediate(500*time.Millisecond, 5*time.Minute, func() (bool, error) {
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

	g.Describe("should respect image stream tag reference policy [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-image-resolution", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("resolve the image pull spec", func() {
			o.Expect(oc.Run("create").Args("-f", resolutionFixture).Execute()).NotTo(o.HaveOccurred())

			name := "deployment-image-resolution"
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentImageTriggersResolved(2))).NotTo(o.HaveOccurred())

			is, err := oc.Client().ImageStreams(oc.Namespace()).Get(name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(is.Status.DockerImageRepository).NotTo(o.BeEmpty())
			o.Expect(is.Status.Tags["direct"].Items).NotTo(o.BeEmpty())
			o.Expect(is.Status.Tags["pullthrough"].Items).NotTo(o.BeEmpty())

			dc, err := oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Spec.Triggers).To(o.HaveLen(3))

			imageID := is.Status.Tags["pullthrough"].Items[0].Image
			resolvedReference := fmt.Sprintf("%s@%s", is.Status.DockerImageRepository, imageID)
			directReference := is.Status.Tags["direct"].Items[0].DockerImageReference

			// controller should be using pullthrough for this (pointing to local registry)
			o.Expect(dc.Spec.Triggers[1].ImageChangeParams).NotTo(o.BeNil())
			o.Expect(dc.Spec.Triggers[1].ImageChangeParams.LastTriggeredImage).To(o.Equal(resolvedReference))
			o.Expect(dc.Spec.Template.Spec.Containers[0].Image).To(o.Equal(resolvedReference))

			// controller should have preferred the base image
			o.Expect(dc.Spec.Triggers[2].ImageChangeParams).NotTo(o.BeNil())
			o.Expect(dc.Spec.Triggers[2].ImageChangeParams.LastTriggeredImage).To(o.Equal(directReference))
			o.Expect(dc.Spec.Template.Spec.Containers[1].Image).To(o.Equal(directReference))
		})
	})

	g.Describe("with test deployments [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-test", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run a deployment to completion and then scale to zero", func() {
			out, err := oc.Run("create").Args("-f", deploymentFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

			out, err = oc.Run("logs").Args("-f", "dc/deployment-test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("deployment-test-1 to 2"))
			o.Expect(out).To(o.ContainSubstring("--> pre: Success"))
			// FIXME: In some cases the last log messages is lost because of the journald rate
			// limiter bug. For this test it should be enough to verify the deployment is marked
			// as complete. We should uncomment this once the rate-limiter issues are fixed.
			// o.Expect(out).To(o.ContainSubstring("--> Success"))

			g.By("verifying the deployment is marked complete and scaled to zero")
			o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that scaling does not result in new pods")
			out, err = oc.Run("scale").Args("dc/deployment-test", "--replicas=1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring no scale up of the deployment happens")
			wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				rc, err := oc.KubeClient().CoreV1().ReplicationControllers(oc.Namespace()).Get("deployment-test-1", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(*rc.Spec.Replicas).Should(o.BeEquivalentTo(0))
				o.Expect(rc.Status.Replicas).Should(o.BeEquivalentTo(0))
				return false, nil
			})

			g.By("verifying the scale is updated on the deployment config")
			config, err := oc.Client().DeploymentConfigs(oc.Namespace()).Get("deployment-test", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(config.Spec.Replicas).Should(o.BeEquivalentTo(1))
			o.Expect(config.Spec.Test).Should(o.BeTrue())

			g.By("deploying a few more times")
			for i := 0; i < 3; i++ {
				rolloutCompleteWithLogs := make(chan struct{})
				out := ""
				go func(rolloutNumber int) {
					defer g.GinkgoRecover()
					defer close(rolloutCompleteWithLogs)
					var err error
					out, err = waitForDeployerToComplete(oc, fmt.Sprintf("deployment-test-%d", rolloutNumber), deploymentRunTimeout)
					o.Expect(err).NotTo(o.HaveOccurred())
				}(i + 2) // we already did 2 rollouts previously.

				// When the rollout latest is called, we already waiting for the replication
				// controller to be created and scrubbing the deployer logs as soon as the
				// deployer container runs.
				_, err := oc.Run("rollout").Args("latest", "deployment-test").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By(fmt.Sprintf("waiting for the rollout #%d to finish", i+2))
				<-rolloutCompleteWithLogs
				o.Expect(out).NotTo(o.BeEmpty())
				o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
				o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("deployment-test-%d up to 1", i+2)))
				o.Expect(out).To(o.ContainSubstring("--> pre: Success"))
				o.Expect(out).To(o.ContainSubstring("test pre hook executed"))
				// FIXME: In some cases the last log messages is lost because of the journald rate
				// limiter bug. For this test it should be enough to verify the deployment is marked
				// as complete. We should uncomment this once the rate-limiter issues are fixed.
				// o.Expect(out).To(o.ContainSubstring("--> Success"))
			}
		})
	})

	g.Describe("when changing image change trigger [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "example", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should successfully trigger from an updated image", func() {
			_, name, err := createFixture(oc, imageChangeTriggerFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForSyncedConfig(oc, name, deploymentRunTimeout)).NotTo(o.HaveOccurred())

			g.By("tagging the busybox:latest as test:v1 image")
			_, err = oc.Run("tag").Args("docker.io/busybox:latest", "test:v1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			expectLatestVersion := func(version int) {
				dc, err := oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				latestVersion := dc.Status.LatestVersion
				err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
					dc, err = oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					latestVersion = dc.Status.LatestVersion
					return latestVersion == int64(version), nil
				})
				if err == wait.ErrWaitTimeout {
					err = fmt.Errorf("expected latestVersion: %d, got: %d", version, latestVersion)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
			}

			g.By("ensuring the deployment config latest version is 1 and rollout completed")
			expectLatestVersion(1)

			g.By("updating the image change trigger to point to test:v2 image")
			_, err = oc.Run("set").Args("triggers", "dc/"+name, "--remove-all").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.Run("set").Args("triggers", "dc/"+name, "--from-image", "test:v2", "--auto", "-c", "test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForSyncedConfig(oc, name, deploymentRunTimeout)).NotTo(o.HaveOccurred())

			g.By("tagging the busybox:1.25 as test:v2 image")
			_, err = oc.Run("tag").Args("docker.io/busybox:1.25", "test:v2").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring the deployment config latest version is 2 and rollout completed")
			expectLatestVersion(2)
		})
	})

	g.Describe("when tagging images [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "tag-images", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should successfully tag the deployed image", func() {
			_, name, err := createFixture(oc, tagImagesFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying the post deployment action happened: tag is set")
			var istag *imageapi.ImageStreamTag
			pollErr := wait.PollImmediate(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				istag, err = oc.Client().ImageStreamTags(oc.Namespace()).Get("sample-stream", "deployed")
				if kerrors.IsNotFound(err) {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				return true, nil
			})
			if pollErr == wait.ErrWaitTimeout {
				pollErr = err
			}
			o.Expect(pollErr).NotTo(o.HaveOccurred())

			if istag.Tag == nil || istag.Tag.From == nil || istag.Tag.From.Name != "openshift/origin-pod" {
				err = fmt.Errorf("expected %q to be part of the image reference in %#v", "openshift/origin-pod", istag)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	})

	g.Describe("with env in params referencing the configmap [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})
		g.It("should expand the config map key to a value", func() {
			_, err := oc.Run("create").Args("configmap", "test", "--from-literal=foo=bar").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			_, name, err := createFixture(oc, envRefDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForSyncedConfig(oc, name, deploymentRunTimeout)).NotTo(o.HaveOccurred())

			_, err = oc.Run("rollout").Args("latest", "dc/"+name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			out, _ := oc.Run("rollout").Args("status", "dc/"+name).Output()
			o.Expect(out).To(o.ContainSubstring("has failed progressing"))

			out, err = oc.Run("logs").Args("dc/" + name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("hello bar"))
		})
	})

	g.Describe("with multiple image change triggers [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run a successful deployment with multiple triggers", func() {
			_, name, err := createFixture(oc, multipleICTFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})

		g.It("should run a successful deployment with a trigger used by different containers", func() {
			_, name, err := createFixture(oc, anotherMultiICTFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with enhanced status [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should include various info in status", func() {
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

	g.Describe("with custom deployments [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "custom-deployment", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run the custom deployment steps", func() {
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
			// FIXME: In some cases the last log messages is lost because of the journald rate
			// limiter bug. For this test it should be enough to verify the deployment is marked
			// as complete. We should uncomment this once the rate-limiter issues are fixed.
			// o.Expect(out).To(o.ContainSubstring("--> Success"))
		})
	})

	g.Describe("viewing rollout history [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should print the rollout history", func() {
			resource, name, err := createFixture(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for the first rollout to complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			dc, err := oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("updating the deployment config in order to trigger a new rollout")
			_, err = client.UpdateConfigWithRetries(oc.Client(), oc.Namespace(), name, func(update *deployapi.DeploymentConfig) {
				one := int64(1)
				update.Spec.Template.Spec.TerminationGracePeriodSeconds = &one
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			// Wait for latestVersion=2 to be surfaced in the API
			latestVersion := dc.Status.LatestVersion
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				dc, err = oc.Client().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				latestVersion = dc.Status.LatestVersion
				return latestVersion == 2, nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected latestVersion: 2, got: %d", latestVersion)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the second rollout to complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			out, err := oc.Run("rollout").Args("history", resource).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the history for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("deploymentconfigs \"deployment-simple\""))
			o.Expect(out).To(o.ContainSubstring("REVISION	STATUS		CAUSE"))
			o.Expect(out).To(o.ContainSubstring("1		Complete	config change"))
			o.Expect(out).To(o.ContainSubstring("2		Complete	config change"))
		})
	})

	g.Describe("generation [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "generation-test", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should deploy based on a status version bump", func() {
			resource, name, err := createFixture(oc, generationFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that both latestVersion and generation are updated")
			var generation, version string
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				version, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
				if err != nil {
					return false, nil
				}
				version = strings.Trim(version, "\"")
				g.By(fmt.Sprintf("checking the latest version for %s: %s", resource, version))

				generation, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.metadata.generation}\"").Output()
				if err != nil {
					return false, nil
				}
				generation = strings.Trim(generation, "\"")
				g.By(fmt.Sprintf("checking the generation for %s: %s", resource, generation))

				return strings.Contains(generation, "2") && strings.Contains(version, "1"), nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected generation: 2, got: %s, expected latestVersion: 1, got: %s", generation, version)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that scaling updates the generation")
			_, err = oc.Run("scale").Args(resource, "--replicas=2").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				generation, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.metadata.generation}\"").Output()
				if err != nil {
					return false, nil
				}
				generation = strings.Trim(generation, "\"")
				g.By(fmt.Sprintf("checking the generation for %s: %s", resource, generation))

				return strings.Contains(generation, "3"), nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected generation: 3, got: %s", generation)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploying a second time [new client]")
			_, err = oc.Run("rollout").Args("latest", name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that both latestVersion and generation are updated")
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				version, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
				if err != nil {
					return false, nil
				}
				version = strings.Trim(version, "\"")
				g.By(fmt.Sprintf("checking the latest version for %s: %s", resource, version))

				generation, err = oc.Run("get").Args(resource, "--output=jsonpath=\"{.metadata.generation}\"").Output()
				if err != nil {
					return false, nil
				}
				generation = strings.Trim(generation, "\"")
				g.By(fmt.Sprintf("checking the generation for %s: %s", resource, generation))

				return strings.Contains(generation, "4") && strings.Contains(version, "2"), nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected generation: 4, got: %s, expected latestVersion: 2, got: %s", generation, version)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that observedGeneration equals generation")
			o.Expect(waitForSyncedConfig(oc, name, deploymentRunTimeout)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("paused [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "paused", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should disable actions on deployments", func() {
			resource, name, err := createFixture(oc, pausedDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, rcs, _, err := deploymentInfo(oc, name)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}

			g.By("verifying that we cannot start a new deployment via oc deploy")
			out, err := oc.Run("deploy").Args(resource, "--latest").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot deploy a paused deployment config"))

			g.By("verifying that we cannot start a new deployment via oc rollout")
			out, err = oc.Run("rollout").Args("latest", resource).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot deploy a paused deployment config"))

			g.By("verifying that we cannot cancel a deployment")
			out, err = oc.Run("rollout").Args("cancel", resource).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("unable to cancel paused deployment"))

			g.By("verifying that we cannot retry a deployment")
			out, err = oc.Run("deploy").Args(resource, "--retry").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot retry a paused deployment config"))

			g.By("verifying that we cannot rollout retry a deployment")
			out, err = oc.Run("rollout").Args("retry", resource).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("unable to retry paused deployment"))

			g.By("verifying that we cannot rollback a deployment")
			out, err = oc.Run("rollback").Args(resource, "--to-version", "1").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot rollback a paused deployment config"))

			_, rcs, _, err = deploymentInfo(oc, name)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}

			_, err = client.UpdateConfigWithRetries(oc.Client(), oc.Namespace(), name, func(dc *deployapi.DeploymentConfig) {
				// TODO: oc rollout pause should patch instead of making a full update
				dc.Spec.Paused = false
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with failing hook [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "hook", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should get all logs from retried hooks", func() {
			resource, name, err := createFixture(oc, failedHookFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentPreHookRetried)).NotTo(o.HaveOccurred())

			out, err := oc.Run("logs").Args(resource).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("--> pre: Running hook pod ..."))
			o.Expect(out).To(o.ContainSubstring("pre hook logs"))
			o.Expect(out).To(o.ContainSubstring("--> pre: Retrying hook pod (retry #1)"))
		})
	})

	g.Describe("rolled back [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "deployment-simple", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should rollback to an older deployment", func() {
			resource, name, err := createFixture(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			_, err = oc.Run("rollout").Args("latest", name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that we are on the second version")
			version := "1"
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				latestVersion, err := oc.Run("get").Args(resource, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
				if err != nil {
					return false, err
				}
				version = strings.Trim(latestVersion, "\"")
				return strings.Contains(version, "2"), nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected latestVersion: 2, got: %s", version)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

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

	g.Describe("reaper [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "brokendeployment", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should delete all failed deployer pods and hook pods", func() {
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

	g.Describe("initially [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "readiness", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should not deploy if pods never transition to ready", func() {
			_, name, err := createFixture(oc, readinessFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to fail")
			err = waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentFailed)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with revision history limits [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "history-limit", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should never persist more old deployments than acceptable after being observed by the controller", func() {
			revisionHistoryLimit := 3 // as specified in the fixture

			dc, err := createDeploymentConfig(oc, historyLimitedDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			deploymentTimeout := time.Duration(*dc.Spec.Strategy.RollingParams.TimeoutSeconds) * time.Second

			iterations := 10
			for i := 0; i < iterations; i++ {
				o.Expect(waitForLatestCondition(oc, "history-limit", deploymentTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred(),
					"the current deployment needs to have finished before attempting to trigger a new deployment through configuration change")
				e2e.Logf("%02d: triggering a new deployment with config change", i)
				out, err := oc.Run("set", "env").Args("dc/history-limit", fmt.Sprintf("A=%d", i)).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("updated"))
			}

			o.Expect(waitForSyncedConfig(oc, "history-limit", deploymentRunTimeout)).NotTo(o.HaveOccurred())
			g.By("waiting for the deployment to complete")
			o.Expect(waitForLatestCondition(oc, "history-limit", deploymentTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
			o.Expect(waitForSyncedConfig(oc, "history-limit", deploymentRunTimeout)).NotTo(o.HaveOccurred(),
				"the controller needs to have synced with the updated deployment configuration before checking that the revision history limits are being adhered to")
			var pollErr error
			err = wait.PollImmediate(1*time.Second, 1*time.Minute, func() (bool, error) {
				deploymentConfig, deployments, _, err := deploymentInfo(oc, "history-limit")
				if err != nil {
					pollErr = err
					return false, nil
				}

				// we need to filter out any deployments that we don't care about,
				// namely the active deployment and any newer deployments
				oldDeployments := deployutil.DeploymentsForCleanup(deploymentConfig, deployments)

				// we should not have more deployments than acceptable
				if len(oldDeployments) != revisionHistoryLimit {
					pollErr = fmt.Errorf("expected len of old deployments: %d to equal dc revisionHistoryLimit: %d", len(oldDeployments), revisionHistoryLimit)
					return false, nil
				}

				// the deployments we continue to keep should be the latest ones
				for _, deployment := range oldDeployments {
					o.Expect(deployutil.DeploymentVersionFor(&deployment)).To(o.BeNumerically(">=", iterations-revisionHistoryLimit))
				}
				return true, nil
			})
			if err == wait.ErrWaitTimeout {
				err = pollErr
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with minimum ready seconds set [Conformance]", func() {
		dc := readDCFixtureOrDie(minReadySecondsFixture)
		rcName := func(i int) string { return fmt.Sprintf("%s-%d", dc.Name, i) }
		g.AfterEach(func() {
			failureTrap(oc, dc.Name, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should not transition the deployment to Complete before satisfied", func() {
			namespace := oc.Namespace()
			watcher, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: rcName(1), ResourceVersion: ""}))
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(dc.Spec.Triggers).To(o.BeNil())
			// FIXME: remove when tests are migrated to the new client
			// (the old one incorrectly translates nil into an empty array)
			dc.Spec.Triggers = append(dc.Spec.Triggers, deployapi.DeploymentTriggerPolicy{Type: deployapi.DeploymentTriggerOnConfigChange})
			dc, err = oc.Client().DeploymentConfigs(namespace).Create(dc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is created")
			rcEvent, err := watch.Until(deploymentChangeTimeout, watcher, func(event watch.Event) (bool, error) {
				if event.Type == watch.Added {
					return true, nil
				}
				return false, fmt.Errorf("different kind of event appeared while waiting for Added event: %#v", event)
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			rc1 := rcEvent.Object.(*kapiv1.ReplicationController)

			g.By("verifying that all pods are ready")
			rc1, err = waitForRCModification(oc, namespace, rc1.Name, deploymentRunTimeout,
				rc1.GetResourceVersion(), func(rc *kapiv1.ReplicationController) (bool, error) {
					return rc.Status.ReadyReplicas == dc.Spec.Replicas, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(rc1.Status.AvailableReplicas).To(o.BeZero())

			g.By("verifying that the deployment is still running")
			if deployutil.IsTerminatedDeployment(rc1) {
				o.Expect(fmt.Errorf("expected deployment %q not to have terminated", rc1.Name)).NotTo(o.HaveOccurred())
			}

			g.By("waiting for the deployment to finish")
			rc1, err = waitForRCModification(oc, namespace, rc1.Name,
				deploymentChangeTimeout+time.Duration(dc.Spec.MinReadySeconds)*time.Second,
				rc1.GetResourceVersion(), func(rc *kapiv1.ReplicationController) (bool, error) {
					if rc.Status.AvailableReplicas == dc.Spec.Replicas {
						return true, nil
					}

					// FIXME: There is a race between deployer pod updating phase and RC updating AvailableReplicas
					// FIXME: Enable this when we switch pod acceptors to use RC AvailableReplicas with MinReadySecondsSet
					//if deployutil.DeploymentStatusFor(rc) == deployapi.DeploymentStatusComplete {
					//	e2e.Logf("Failed RC: %#v", rc)
					//	return false, errors.New("deployment shouldn't be completed before ReadyReplicas become AvailableReplicas")
					//}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(rc1.Status.AvailableReplicas).To(o.Equal(dc.Spec.Replicas))
			// FIXME: There is a race between deployer pod updating phase and RC updating AvailableReplicas
			// FIXME: Enable this when we switch pod acceptors to use RC AvailableReplicas with MinReadySecondsSet
			//// Deployment status can't be updated yet but should be right after
			//o.Expect(deployutil.DeploymentStatusFor(rc1)).To(o.Equal(deployapi.DeploymentStatusRunning))
			// It should finish right after
			// FIXME: remove this condition when the above is fixed
			if deployutil.DeploymentStatusFor(rc1) != deployapi.DeploymentStatusComplete {
				// FIXME: remove this assertion when the above is fixed
				o.Expect(deployutil.DeploymentStatusFor(rc1)).To(o.Equal(deployapi.DeploymentStatusRunning))
				rc1, err = waitForRCModification(oc, namespace, rc1.Name, deploymentChangeTimeout,
					rc1.GetResourceVersion(), func(rc *kapiv1.ReplicationController) (bool, error) {
						return deployutil.DeploymentStatusFor(rc) == deployapi.DeploymentStatusComplete, nil
					})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// We might check that minReadySecond passed between pods becoming ready
			// and available but I don't think there is a way to get a timestamp from events
			// and other ways are just flaky.
			// But since we are reusing MinReadySeconds and AvailableReplicas from RC it should be tested there
		})
	})

	g.Describe("ignores deployer and lets the config with a NewReplicationControllerCreated reason [Conformance]", func() {
		g.AfterEach(func() {
			failureTrap(oc, "database", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should let the deployment config with a NewReplicationControllerCreated reason", func() {
			_, name, err := createFixture(oc, ignoresDeployersFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that the deployment config is bumped to the first version")
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, name)
				if err != nil {
					return false, nil
				}
				return dc.Status.LatestVersion == 1, nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("deployment config %q never incremented to the first version", name)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that the deployment config has the desired condition and reason")
			var conditions []deployapi.DeploymentCondition
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, name)
				if err != nil {
					return false, nil
				}
				conditions = dc.Status.Conditions
				cond := deployutil.GetDeploymentCondition(dc.Status, deployapi.DeploymentProgressing)
				return cond != nil && cond.Reason == deployapi.NewReplicationControllerReason, nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("deployment config %q never updated its conditions: %#v", name, conditions)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
			failureTrapForDetachedRCs(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should adhere to Three Laws of Controllers [Conformance]", func() {
			namespace := oc.Namespace()
			rcName := func(i int) string { return fmt.Sprintf("%s-%d", dcName, i) }

			var dc *deployapi.DeploymentConfig
			var rc1 *kapiv1.ReplicationController
			var err error

			g.By("should create ControllerRef in RCs it creates", func() {
				dc, err = readDCFixture(simpleDeploymentFixture)
				o.Expect(err).NotTo(o.HaveOccurred())
				dc, err = oc.Client().DeploymentConfigs(namespace).Create(dc)
				o.Expect(err).NotTo(o.HaveOccurred())

				err = waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentRunning)
				o.Expect(err).NotTo(o.HaveOccurred())

				rc1, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Get(rcName(1), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				validRef := HasValidDCControllerRef(dc, rc1)
				o.Expect(validRef).To(o.BeTrue())
			})

			err = waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("releasing RCs that no longer match its selector", func() {
				dc, err = oc.Client().DeploymentConfigs(namespace).Get(dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				patch := []byte(fmt.Sprintf(`{"metadata": {"labels":{"openshift.io/deployment-config.name": "%s-detached"}}}`, dcName))
				rc1, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(rcName(1), types.StrategicMergePatchType, patch)
				o.Expect(err).NotTo(o.HaveOccurred())

				rc1, err = waitForRCModification(oc, namespace, rcName(1), deploymentChangeTimeout,
					rc1.GetResourceVersion(), rCConditionFromMeta(controllerRefChangeCondition(kcontroller.GetControllerOf(rc1))))
				o.Expect(err).NotTo(o.HaveOccurred())
				controllerRef := kcontroller.GetControllerOf(rc1)
				o.Expect(controllerRef).To(o.BeNil())

				dc, err = waitForDCModification(oc, namespace, dcName, deploymentChangeTimeout,
					dc.GetResourceVersion(), func(config *deployapi.DeploymentConfig) (bool, error) {
						return config.Status.AvailableReplicas != dc.Status.AvailableReplicas, nil
					})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(dc.Status.AvailableReplicas).To(o.BeZero())
				o.Expect(dc.Status.UnavailableReplicas).To(o.BeZero())
			})

			g.By("adopting RCs that match its selector and have no ControllerRef", func() {
				patch := []byte(fmt.Sprintf(`{"metadata": {"labels":{"openshift.io/deployment-config.name": "%s"}}}`, dcName))
				rc1, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(rcName(1), types.StrategicMergePatchType, patch)
				o.Expect(err).NotTo(o.HaveOccurred())

				rc1, err = waitForRCModification(oc, namespace, rcName(1), deploymentChangeTimeout,
					rc1.GetResourceVersion(), rCConditionFromMeta(controllerRefChangeCondition(kcontroller.GetControllerOf(rc1))))
				o.Expect(err).NotTo(o.HaveOccurred())
				validRef := HasValidDCControllerRef(dc, rc1)
				o.Expect(validRef).To(o.BeTrue())

				dc, err = waitForDCModification(oc, namespace, dcName, deploymentChangeTimeout,
					dc.GetResourceVersion(), func(config *deployapi.DeploymentConfig) (bool, error) {
						return config.Status.AvailableReplicas != dc.Status.AvailableReplicas, nil
					})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(dc.Status.AvailableReplicas).To(o.Equal(dc.Spec.Replicas))
				o.Expect(dc.Status.UnavailableReplicas).To(o.BeZero())
			})

			g.By("deleting owned RCs when deleted", func() {
				// FIXME: Add delete option when we have new client available.
				// This is working fine now because of finalizers on RCs but when GC gets fixed
				// and we remove them this will probably break and will require setting deleteOptions
				// to achieve cascade delete
				err = oc.Client().DeploymentConfigs(namespace).Delete(dcName)
				o.Expect(err).NotTo(o.HaveOccurred())

				err = wait.PollImmediate(200*time.Millisecond, 5*time.Minute, func() (bool, error) {
					pods, err := oc.KubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
					if err != nil {
						return false, err
					}
					return len(pods.Items) == 0, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				err = wait.PollImmediate(200*time.Millisecond, 30*time.Second, func() (bool, error) {
					rcs, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).List(metav1.ListOptions{})
					if err != nil {
						return false, err
					}
					return len(rcs.Items) == 0, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})
