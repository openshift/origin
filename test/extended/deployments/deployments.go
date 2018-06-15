package deployments

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	exutil "github.com/openshift/origin/test/extended/util"
)

const deploymentRunTimeout = 5 * time.Minute
const deploymentChangeTimeout = 30 * time.Second

type dicEntry struct {
	dic    *deployerPodInvariantChecker
	ctx    context.Context
	cancel func()
}

var _ = g.Describe("[Feature:DeploymentConfig] deploymentconfigs", func() {
	defer g.GinkgoRecover()

	dicMap := make(map[string]dicEntry)
	var oc *exutil.CLI

	g.JustBeforeEach(func() {
		namespace := oc.Namespace()
		o.Expect(namespace).NotTo(o.BeEmpty())
		o.Expect(dicMap).NotTo(o.HaveKey(namespace))

		dic := NewDeployerPodInvariantChecker(namespace, oc.AdminKubeClient())
		ctx, cancel := context.WithCancel(context.Background())
		dic.Start(ctx)

		dicMap[namespace] = dicEntry{
			dic:    dic,
			ctx:    ctx,
			cancel: cancel,
		}
	})

	// This have to be registered before we create kube framework (NewCLI).
	// It is probably a bug with Ginkgo because AfterEach description say innermost will be run first
	// but it runs outermost first.
	g.AfterEach(func() {
		namespace := oc.Namespace()
		o.Expect(namespace).NotTo(o.BeEmpty(), "There is something wrong with testing framework or the AfterEach functions have been registered in wrong order")
		o.Expect(dicMap).To(o.HaveKey(namespace))

		// Give some time to the checker to catch up
		time.Sleep(2 * time.Second)

		entry := dicMap[namespace]
		delete(dicMap, namespace)

		entry.cancel()
		entry.dic.Wait()
	})

	oc = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())

	var (
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
		resolutionIsFixture             = exutil.FixturePath("testdata", "deployments", "deployment-image-resolution-is.yaml")
		anotherMultiICTFixture          = exutil.FixturePath("testdata", "deployments", "multi-ict-deployment.yaml")
		tagImagesFixture                = exutil.FixturePath("testdata", "deployments", "tag-images-deployment.yaml")
		readinessFixture                = exutil.FixturePath("testdata", "deployments", "readiness-test.yaml")
		envRefDeploymentFixture         = exutil.FixturePath("testdata", "deployments", "deployment-with-ref-env.yaml")
		ignoresDeployersFixture         = exutil.FixturePath("testdata", "deployments", "deployment-ignores-deployer.yaml")
		imageChangeTriggerFixture       = exutil.FixturePath("testdata", "deployments", "deployment-trigger.yaml")
	)

	g.Describe("when run iteratively [Conformance]", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should only deploy the last deployment", func() {
			dc, err := createDeploymentConfig(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			r := rand.New(rand.NewSource(g.GinkgoRandomSeed()))
			iterations := 15
			for i := 0; i < iterations; i++ {
				if r.Float32() < 0.2 {
					time.Sleep(time.Duration(r.Float32() * r.Float32() * float32(time.Second)))
				}
				switch n := r.Float32(); {

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
						if !strings.Contains(out, "the object has been modified") &&
							!strings.Contains(out, "there have been no replication controllers") &&
							!strings.Contains(out, "there is a meaningful conflict") {
							o.Expect(err).NotTo(o.HaveOccurred())
						}
						e2e.Logf("rollout cancel deployment failed due to known safe error: %v", err)
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
							if r.Float32() < 0.5 {
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
			dc, err := createDeploymentConfig(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = oc.Run("set", "env").Args("dc/"+dc.Name, "TRY=ONCE").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the deployment config has the correct version"))
			err = wait.PollImmediate(500*time.Millisecond, 5*time.Second, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, dc.Name)
				if err != nil {
					return false, nil
				}
				return dc.Status.LatestVersion == 2, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the second deployment exists"))
			// TODO when #11016 gets fixed this can be reverted to 30seconds
			err = wait.PollImmediate(500*time.Millisecond, 5*time.Minute, func() (bool, error) {
				_, rcs, _, err := deploymentInfo(oc, dcName)
				if err != nil {
					return false, nil
				}

				secondDeploymentExists := false
				for _, rc := range rcs {
					if rc.Name == appsutil.DeploymentNameForConfigVersion(dcName, 2) {
						secondDeploymentExists = true
						break
					}
				}

				return secondDeploymentExists, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the first deployer was deleted and the second deployer exists"))
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				_, _, pods, err := deploymentInfo(oc, dcName)
				if err != nil {
					return false, nil
				}

				deploymentNamesToDeployers, err := deploymentPods(pods)
				if err != nil {
					return false, nil
				}

				firstDeploymentName := appsutil.DeploymentNameForConfigVersion(dcName, 1)
				firstDeployerRemoved := true
				for _, deployer := range deploymentNamesToDeployers[firstDeploymentName] {
					if deployer.Status.Phase != kapiv1.PodFailed && deployer.Status.Phase != kapiv1.PodSucceeded {
						firstDeployerRemoved = false
					}
				}

				secondDeploymentName := appsutil.DeploymentNameForConfigVersion(dcName, 2)
				secondDeployerRemoved := true
				for _, deployer := range deploymentNamesToDeployers[secondDeploymentName] {
					if deployer.Status.Phase != kapiv1.PodFailed && deployer.Status.Phase != kapiv1.PodSucceeded {
						secondDeployerRemoved = false
					}
				}

				return firstDeployerRemoved && !secondDeployerRemoved, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("should respect image stream tag reference policy [Conformance]", func() {
		dcName := "deployment-image-resolution"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("resolve the image pull spec", func() {
			// FIXME: Wrap the IS creation into utility helper
			err := oc.Run("create").Args("-f", resolutionIsFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			dc, err := createDeploymentConfig(oc, resolutionFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			name := "deployment-image-resolution"
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentImageTriggersResolved(2))).NotTo(o.HaveOccurred())

			is, err := oc.ImageClient().Image().ImageStreams(oc.Namespace()).Get(name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(is.Status.DockerImageRepository).NotTo(o.BeEmpty())
			o.Expect(is.Status.Tags["direct"].Items).NotTo(o.BeEmpty())
			o.Expect(is.Status.Tags["pullthrough"].Items).NotTo(o.BeEmpty())

			dc, err = oc.AppsClient().Apps().DeploymentConfigs(oc.Namespace()).Get(name, metav1.GetOptions{})
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
		dcName := "deployment-test"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run a deployment to completion and then scale to zero", func() {
			dc, err := createDeploymentConfig(oc, deploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

			out, err := oc.Run("logs").Args("-f", "dc/deployment-test").Output()
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
			wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				rc, err := oc.KubeClient().CoreV1().ReplicationControllers(oc.Namespace()).Get("deployment-test-1", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(*rc.Spec.Replicas).Should(o.BeEquivalentTo(0))
				o.Expect(rc.Status.Replicas).Should(o.BeEquivalentTo(0))
				return false, nil
			})

			g.By("verifying the scale is updated on the deployment config")
			config, err := oc.AppsClient().Apps().DeploymentConfigs(oc.Namespace()).Get("deployment-test", metav1.GetOptions{})
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
				o.Expect(out).To(o.ContainSubstring("--> Success"))
			}
		})
	})

	g.Describe("when changing image change trigger [Conformance]", func() {
		dcName := "example"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should successfully trigger from an updated image", func() {
			dc, err := createDeploymentConfig(oc, imageChangeTriggerFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))
			o.Expect(waitForSyncedConfig(oc, dcName, deploymentRunTimeout)).NotTo(o.HaveOccurred())

			g.By("tagging the busybox:latest as test:v1 image")
			_, err = oc.Run("tag").Args("docker.io/busybox:latest", "test:v1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			expectLatestVersion := func(version int) {
				dc, err := oc.AppsClient().Apps().DeploymentConfigs(oc.Namespace()).Get(dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				latestVersion := dc.Status.LatestVersion
				err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
					dc, err = oc.AppsClient().Apps().DeploymentConfigs(oc.Namespace()).Get(dcName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					latestVersion = dc.Status.LatestVersion
					return latestVersion == int64(version), nil
				})
				if err == wait.ErrWaitTimeout {
					err = fmt.Errorf("expected latestVersion: %d, got: %d", version, latestVersion)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
			}

			g.By("ensuring the deployment config latest version is 1 and rollout completed")
			expectLatestVersion(1)

			g.By("updating the image change trigger to point to test:v2 image")
			_, err = oc.Run("set").Args("triggers", "dc/"+dcName, "--remove-all").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = oc.Run("set").Args("triggers", "dc/"+dcName, "--from-image", "test:v2", "--auto", "-c", "test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForSyncedConfig(oc, dcName, deploymentRunTimeout)).NotTo(o.HaveOccurred())

			g.By("tagging the busybox:1.25 as test:v2 image")
			_, err = oc.Run("tag").Args("docker.io/busybox:1.25", "test:v2").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring the deployment config latest version is 2 and rollout completed")
			expectLatestVersion(2)
		})
	})

	g.Describe("when tagging images [Conformance]", func() {
		dcName := "tag-images"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should successfully tag the deployed image", func() {
			g.By("creating the deployment config fixture")
			dc, err := createDeploymentConfig(oc, tagImagesFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying the deployer service account can update imagestreamtags and user can get them")
			err = exutil.WaitForUserBeAuthorized(oc, oc.Username(), "get", "imagestreamtags")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForUserBeAuthorized(oc, "system:serviceaccount:"+oc.Namespace()+":deployer", "update", "imagestreamtags")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the post deployment action happened: tag is set")
			var istag *imageapi.ImageStreamTag
			pollErr := wait.PollImmediate(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				istag, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("sample-stream:deployed", metav1.GetOptions{})
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
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should expand the config map key to a value", func() {
			_, err := oc.Run("create").Args("configmap", "test", "--from-literal=foo=bar").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			dc, err := createDeploymentConfig(oc, envRefDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))
			o.Expect(waitForSyncedConfig(oc, dcName, deploymentRunTimeout)).NotTo(o.HaveOccurred())

			_, err = oc.Run("rollout").Args("latest", "dc/"+dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			out, _ := oc.Run("rollout").Args("status", "dc/"+dcName).Output()
			o.Expect(out).To(o.ContainSubstring("has failed progressing"))

			out, err = oc.Run("logs").Args("dc/" + dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("hello bar"))
		})
	})

	g.Describe("with multiple image change triggers [Conformance]", func() {
		dcName := "example"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run a successful deployment with multiple triggers", func() {
			g.By("creating DC")
			dc, err := createDeploymentConfig(oc, multipleICTFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))
			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})

		g.It("should run a successful deployment with a trigger used by different containers", func() {
			dc, err := createDeploymentConfig(oc, anotherMultiICTFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with enhanced status [Conformance]", func() {
		dcName := "deployment-simple"

		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should include various info in status", func() {
			dc, err := createDeploymentConfig(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that status.replicas is set")
			replicas, err := oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.replicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(replicas).To(o.ContainSubstring("1"))
			g.By("verifying that status.updatedReplicas is set")
			updatedReplicas, err := oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.updatedReplicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(updatedReplicas).To(o.ContainSubstring("1"))
			g.By("verifying that status.availableReplicas is set")
			availableReplicas, err := oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.availableReplicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(availableReplicas).To(o.ContainSubstring("1"))
			g.By("verifying that status.unavailableReplicas is set")
			unavailableReplicas, err := oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.unavailableReplicas}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(unavailableReplicas).To(o.ContainSubstring("0"))
		})
	})

	g.Describe("with custom deployments [Conformance]", func() {
		dcName := "custom-deployment"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should run the custom deployment steps", func() {
			dc, err := createDeploymentConfig(oc, customDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentRunning)).NotTo(o.HaveOccurred())

			out, err := oc.Run("deploy").Args("--follow", "dc/custom-deployment").Output()
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

	g.Describe("viewing rollout history [Conformance]", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should print the rollout history", func() {
			dc, err := createDeploymentConfig(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("waiting for the first rollout to complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			dc, err = oc.AppsClient().Apps().DeploymentConfigs(oc.Namespace()).Get(dcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("updating the deployment config in order to trigger a new rollout")
			_, err = updateConfigWithRetries(oc.AppsClient().Apps(), oc.Namespace(), dcName, func(update *appsapi.DeploymentConfig) {
				one := int64(1)
				update.Spec.Template.Spec.TerminationGracePeriodSeconds = &one
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			// Wait for latestVersion=2 to be surfaced in the API
			latestVersion := dc.Status.LatestVersion
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				dc, err = oc.AppsClient().Apps().DeploymentConfigs(oc.Namespace()).Get(dcName, metav1.GetOptions{})
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
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			out, err := oc.Run("rollout").Args("history", "dc/"+dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("checking the history for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("deploymentconfigs \"deployment-simple\""))
			o.Expect(out).To(o.ContainSubstring("REVISION	STATUS		CAUSE"))
			o.Expect(out).To(o.ContainSubstring("1		Complete	config change"))
			o.Expect(out).To(o.ContainSubstring("2		Complete	config change"))
		})
	})

	g.Describe("generation [Conformance]", func() {
		dcName := "generation-test"
		g.AfterEach(func() {
			failureTrap(oc, "generation-test", g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should deploy based on a status version bump", func() {
			dc, err := createDeploymentConfig(oc, generationFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("verifying that both latestVersion and generation are updated")
			var generation, version string
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				version, err = oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
				if err != nil {
					return false, nil
				}
				version = strings.Trim(version, "\"")
				g.By(fmt.Sprintf("checking the latest version for deployment config %q: %s", dcName, version))

				generation, err = oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.metadata.generation}\"").Output()
				if err != nil {
					return false, nil
				}
				generation = strings.Trim(generation, "\"")
				g.By(fmt.Sprintf("checking the generation for deployment config %q: %s", dcName, generation))

				return strings.Contains(generation, "1") && strings.Contains(version, "1"), nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected generation: 1, got: %s, expected latestVersion: 1, got: %s", generation, version)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that scaling updates the generation")
			_, err = oc.Run("scale").Args("dc/"+dcName, "--replicas=2").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				generation, err = oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.metadata.generation}\"").Output()
				if err != nil {
					return false, nil
				}
				generation = strings.Trim(generation, "\"")
				g.By(fmt.Sprintf("checking the generation for deployment config %s: %s", dcName, generation))

				return strings.Contains(generation, "2"), nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected generation: 2, got: %s", generation)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploying a second time [new client]")
			_, err = oc.Run("rollout").Args("latest", "dc/"+dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that both latestVersion and generation are updated")
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				version, err = oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
				if err != nil {
					return false, nil
				}
				version = strings.Trim(version, "\"")
				g.By(fmt.Sprintf("checking the latest version for deployment config %q: %s", dcName, version))

				generation, err = oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.metadata.generation}\"").Output()
				if err != nil {
					return false, nil
				}
				generation = strings.Trim(generation, "\"")
				g.By(fmt.Sprintf("checking the generation for deployment config %q: %s", dcName, generation))

				return strings.Contains(generation, "3") && strings.Contains(version, "2"), nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("expected generation: 3, got: %s, expected latestVersion: 2, got: %s", generation, version)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that observedGeneration equals generation")
			o.Expect(waitForSyncedConfig(oc, dcName, deploymentRunTimeout)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("paused [Conformance]", func() {
		dcName := "paused"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should disable actions on deployments", func() {
			dc, err := createDeploymentConfig(oc, pausedDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			_, rcs, _, err := deploymentInfo(oc, dcName)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}
			resource := "dc/" + dcName

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

			_, rcs, _, err = deploymentInfo(oc, dcName)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}

			_, err = updateConfigWithRetries(oc.AppsClient().Apps(), oc.Namespace(), dcName, func(dc *appsapi.DeploymentConfig) {
				// TODO: oc rollout pause should patch instead of making a full update
				dc.Spec.Paused = false
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("making sure it updates observedGeneration after being paused")
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(oc.Namespace()).Patch(dcName,
				types.StrategicMergePatchType, []byte(`{"spec": {"paused": true}}`))
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = waitForDCModification(oc, dc.Namespace, dcName, deploymentChangeTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					if config.Status.ObservedGeneration >= dc.Generation {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to wait on generation >= %d to be observed by DC %s/%s", dc.Generation, dc.Namespace, dcName))
		})
	})

	g.Describe("with failing hook [Conformance]", func() {
		dcName := "hook"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should get all logs from retried hooks", func() {
			dc, err := createDeploymentConfig(oc, failedHookFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentPreHookRetried)).NotTo(o.HaveOccurred())

			out, err := oc.Run("logs").Args("dc/" + dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("checking the logs for substrings\n%s", out))
			o.Expect(out).To(o.ContainSubstring("--> pre: Running hook pod ..."))
			o.Expect(out).To(o.ContainSubstring("pre hook logs"))
			o.Expect(out).To(o.ContainSubstring("--> pre: Retrying hook pod (retry #1)"))
		})
	})

	g.Describe("rolled back [Conformance]", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should rollback to an older deployment", func() {
			dc, err := createDeploymentConfig(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			_, err = oc.Run("rollout").Args("latest", dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that we are on the second version")
			version := "1"
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				latestVersion, err := oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
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

			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that we can rollback")
			_, err = oc.Run("rollout").Args("undo", "dc/"+dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying that we are on the third version")
			version, err = oc.Run("get").Args("dc/"+dcName, "--output=jsonpath=\"{.status.latestVersion}\"").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			version = strings.Trim(version, "\"")
			o.Expect(version).To(o.ContainSubstring("3"))
		})
	})

	g.Describe("reaper [Conformance][Slow]", func() {
		dcName := "brokendeployment"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should delete all failed deployer pods and hook pods", func() {
			dc, err := createDeploymentConfig(oc, brokenDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("waiting for the deployment to complete")
			err = waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)
			o.Expect(err).To(o.HaveOccurred())

			g.By("fetching the deployer pod")
			out, err := oc.Run("get").Args("pod", fmt.Sprintf("%s-1-deploy", dcName)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Error"))

			g.By("fetching the pre-hook pod")
			out, err = oc.Run("get").Args("pod", fmt.Sprintf("%s-1-hook-pre", dcName)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Error"))

			g.By("deleting the deployment config")
			out, err = oc.Run("delete").Args("dc/" + dcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("fetching the deployer pod")
			out, err = oc.Run("get").Args("pod", fmt.Sprintf("%s-1-deploy", dcName)).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("not found"))

			g.By("fetching the pre-hook pod")
			out, err = oc.Run("get").Args("pod", fmt.Sprintf("%s-1-hook-pre", dcName)).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("not found"))
		})
	})

	g.Describe("initially [Conformance]", func() {
		dcName := "readiness"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should not deploy if pods never transition to ready", func() {
			dc, err := createDeploymentConfig(oc, readinessFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("waiting for the deployment to fail")
			err = waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentFailed)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with revision history limits [Conformance]", func() {
		dcName := "history-limit"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should never persist more old deployments than acceptable after being observed by the controller", func() {
			revisionHistoryLimit := 3 // as specified in the fixture

			dc, err := createDeploymentConfig(oc, historyLimitedDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

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
				oldDeployments := appsutil.DeploymentsForCleanup(deploymentConfig, deployments)

				// we should not have more deployments than acceptable
				if len(oldDeployments) != revisionHistoryLimit {
					pollErr = fmt.Errorf("expected len of old deployments: %d to equal dc revisionHistoryLimit: %d", len(oldDeployments), revisionHistoryLimit)
					return false, nil
				}

				// the deployments we continue to keep should be the latest ones
				for _, deployment := range oldDeployments {
					o.Expect(appsutil.DeploymentVersionFor(&deployment)).To(o.BeNumerically(">=", iterations-revisionHistoryLimit))
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
		dcName := "minreadytest"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should not transition the deployment to Complete before satisfied", func() {
			dc, err := readDCFixture(minReadySecondsFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			rcName := func(i int) string { return fmt.Sprintf("%s-%d", dc.Name, i) }
			namespace := oc.Namespace()
			watcher, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: rcName(1), ResourceVersion: ""}))
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(dc.Spec.Triggers).To(o.BeNil())
			// FIXME: remove when tests are migrated to the new client
			// (the old one incorrectly translates nil into an empty array)
			dc.Spec.Triggers = append(dc.Spec.Triggers, appsapi.DeploymentTriggerPolicy{Type: appsapi.DeploymentTriggerOnConfigChange})
			// This is the last place we can safely say that the time was taken before replicas became ready
			startTime := time.Now()
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Create(dc)
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
			o.Expect(rc1.Status.AvailableReplicas).To(o.BeNumerically("<", rc1.Status.ReadyReplicas))
			// We need to log here to have a timestamp to compare with master logs if something goes wrong
			e2e.Logf("All replicas are ready.")

			g.By("verifying that the deployment is still running")
			if appsutil.IsTerminatedDeployment(rc1) {
				o.Expect(fmt.Errorf("expected deployment %q not to have terminated", rc1.Name)).NotTo(o.HaveOccurred())
			}

			g.By("waiting for the deployment to finish")
			rc1, err = waitForRCModification(oc, namespace, rc1.Name,
				deploymentRunTimeout+time.Duration(dc.Spec.MinReadySeconds)*time.Second,
				rc1.GetResourceVersion(), func(rc *kapiv1.ReplicationController) (bool, error) {
					if rc.Status.AvailableReplicas == dc.Spec.Replicas {
						return true, nil
					}

					if appsutil.DeploymentStatusFor(rc) == appsapi.DeploymentStatusComplete {
						e2e.Logf("Failed RC: %#v", rc)
						return false, errors.New("deployment shouldn't be completed before ReadyReplicas become AvailableReplicas")
					}
					return false, nil
				})
			// We need to log here to have a timestamp to compare with master logs if something goes wrong
			e2e.Logf("Finished waiting for deployment.")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(time.Since(startTime)).To(o.BeNumerically(">=", time.Duration(dc.Spec.MinReadySeconds)*time.Second),
				"Deployment shall not finish before MinReadySeconds elapse.")
			o.Expect(rc1.Status.AvailableReplicas).To(o.Equal(dc.Spec.Replicas))
			// Deployment status can't be updated yet but should be right after
			o.Expect(appsutil.DeploymentStatusFor(rc1)).To(o.Equal(appsapi.DeploymentStatusRunning))
			// It should finish right after
			rc1, err = waitForRCModification(oc, namespace, rc1.Name, deploymentChangeTimeout,
				rc1.GetResourceVersion(), func(rc *kapiv1.ReplicationController) (bool, error) {
					return appsutil.DeploymentStatusFor(rc) == appsapi.DeploymentStatusComplete, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			// We might check that minReadySecond passed between pods becoming ready
			// and available but I don't think there is a way to get a timestamp from events
			// and other ways are just flaky.
			// But since we are reusing MinReadySeconds and AvailableReplicas from RC it should be tested there
		})
	})

	g.Describe("ignores deployer and lets the config with a NewReplicationControllerCreated reason [Conformance]", func() {
		dcName := "database"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should let the deployment config with a NewReplicationControllerCreated reason", func() {
			dc, err := createDeploymentConfig(oc, ignoresDeployersFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("verifying that the deployment config is bumped to the first version")
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, dcName)
				if err != nil {
					return false, nil
				}
				return dc.Status.LatestVersion == 1, nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("deployment config %q never incremented to the first version", dcName)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that the deployment config has the desired condition and reason")
			var conditions []appsapi.DeploymentCondition
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, dcName)
				if err != nil {
					return false, nil
				}
				conditions = dc.Status.Conditions
				cond := appsutil.GetDeploymentCondition(dc.Status, appsapi.DeploymentProgressing)
				return cond != nil && cond.Reason == appsapi.NewReplicationControllerReason, nil
			})
			if err == wait.ErrWaitTimeout {
				err = fmt.Errorf("deployment config %q never updated its conditions: %#v", dcName, conditions)
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

			var dc *appsapi.DeploymentConfig
			var rc1 *kapiv1.ReplicationController
			var err error

			g.By("should create ControllerRef in RCs it creates", func() {
				dc, err = readDCFixture(simpleDeploymentFixture)
				o.Expect(err).NotTo(o.HaveOccurred())
				// Having more replicas will make us more resilient to pod failures
				dc.Spec.Replicas = 3
				dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Create(dc)
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
				dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Get(dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				patch := []byte(fmt.Sprintf(`{"metadata": {"labels":{"openshift.io/deployment-config.name": "%s-detached"}}}`, dcName))
				rc1, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(rcName(1), types.StrategicMergePatchType, patch)
				o.Expect(err).NotTo(o.HaveOccurred())

				rc1, err = waitForRCModification(oc, namespace, rcName(1), deploymentChangeTimeout,
					rc1.GetResourceVersion(), rCConditionFromMeta(controllerRefChangeCondition(metav1.GetControllerOf(rc1))))
				o.Expect(err).NotTo(o.HaveOccurred())
				controllerRef := metav1.GetControllerOf(rc1)
				o.Expect(controllerRef).To(o.BeNil())

				dc, err = waitForDCModification(oc, namespace, dcName, deploymentChangeTimeout,
					dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
						return config.Status.AvailableReplicas == 0, nil
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
					rc1.GetResourceVersion(), rCConditionFromMeta(controllerRefChangeCondition(metav1.GetControllerOf(rc1))))
				o.Expect(err).NotTo(o.HaveOccurred())
				validRef := HasValidDCControllerRef(dc, rc1)
				o.Expect(validRef).To(o.BeTrue())

				dc, err = waitForDCModification(oc, namespace, dcName, deploymentChangeTimeout,
					dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
						return config.Status.AvailableReplicas == dc.Spec.Replicas, nil
					})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(dc.Status.AvailableReplicas).To(o.Equal(dc.Spec.Replicas))
				o.Expect(dc.Status.UnavailableReplicas).To(o.BeZero())
			})

			g.By("deleting owned RCs when deleted", func() {
				err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Delete(dcName, &metav1.DeleteOptions{})
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

	g.Describe("keep the deployer pod invariant valid [Conformance]", func() {
		dcName := "deployment-simple"

		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should deal with cancellation of running deployment", func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc, err := readDCFixture(simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc.Spec.Replicas = 1
			// Make sure the deployer pod doesn't end too soon
			dc.Spec.MinReadySeconds = 60
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Create(dc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for RC to be created")
			dc, err = waitForDCModification(oc, namespace, dcName, deploymentRunTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					cond := appsutil.GetDeploymentCondition(config.Status, appsapi.DeploymentProgressing)
					if cond != nil && cond.Reason == appsapi.NewReplicationControllerReason {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			g.By("waiting for deployer pod to be running")
			rc, err := waitForRCModification(oc, namespace, appsutil.LatestDeploymentNameForConfig(dc), deploymentRunTimeout,
				"", func(currentRC *kapiv1.ReplicationController) (bool, error) {
					if appsutil.DeploymentStatusFor(currentRC) == appsapi.DeploymentStatusRunning {
						return true, nil
					}
					return false, nil
				})

			g.By("canceling the deployment")
			rc, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(
				appsutil.LatestDeploymentNameForConfig(dc), types.StrategicMergePatchType,
				[]byte(fmt.Sprintf(`{"metadata":{"annotations":{%q: %q, %q: %q}}}`,
					appsapi.DeploymentCancelledAnnotation, appsapi.DeploymentCancelledAnnotationValue,
					appsapi.DeploymentStatusReasonAnnotation, appsapi.DeploymentCancelledByUser,
				)))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(appsutil.DeploymentVersionFor(rc)).To(o.Equal(dc.Status.LatestVersion))

			g.By("redeploying immediately by config change")
			o.Expect(dc.Spec.Template.Annotations["foo"]).NotTo(o.Equal("bar"))
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(dc.Namespace).Patch(dc.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"metadata":{"annotations":{"foo": "bar"}}}}}`))
			o.Expect(err).NotTo(o.HaveOccurred())
			dc, err = waitForDCModification(oc, namespace, dcName, deploymentRunTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					if config.Status.LatestVersion == 2 {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Wait for deployment pod to be running
			rc, err = waitForRCModification(oc, namespace, appsutil.LatestDeploymentNameForConfig(dc), deploymentRunTimeout,
				"", func(currentRC *kapiv1.ReplicationController) (bool, error) {
					if appsutil.DeploymentStatusFor(currentRC) == appsapi.DeploymentStatusRunning {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should deal with config change in case the deployment is still running", func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc, err := readDCFixture(simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc.Spec.Replicas = 1
			// Make sure the deployer pod doesn't end too soon
			dc.Spec.MinReadySeconds = 60
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Create(dc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for RC to be created")
			dc, err = waitForDCModification(oc, namespace, dc.Name, deploymentRunTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					cond := appsutil.GetDeploymentCondition(config.Status, appsapi.DeploymentProgressing)
					if cond != nil && cond.Reason == appsapi.NewReplicationControllerReason {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			g.By("waiting for deployer pod to be running")
			_, err = waitForRCModification(oc, namespace, appsutil.LatestDeploymentNameForConfig(dc), deploymentRunTimeout,
				"", func(currentRC *kapiv1.ReplicationController) (bool, error) {
					if appsutil.DeploymentStatusFor(currentRC) == appsapi.DeploymentStatusRunning {
						return true, nil
					}
					return false, nil
				})

			g.By("redeploying immediately by config change")
			o.Expect(dc.Spec.Template.Annotations["foo"]).NotTo(o.Equal("bar"))
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(dc.Namespace).Patch(dc.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"metadata":{"annotations":{"foo": "bar"}}}}}`))
			o.Expect(err).NotTo(o.HaveOccurred())
			dc, err = waitForDCModification(oc, namespace, dcName, deploymentRunTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					if config.Status.LatestVersion == 2 {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Wait for deployment pod to be running
			_, err = waitForRCModification(oc, namespace, appsutil.LatestDeploymentNameForConfig(dc), deploymentRunTimeout,
				"", func(currentRC *kapiv1.ReplicationController) (bool, error) {
					if appsutil.DeploymentStatusFor(currentRC) == appsapi.DeploymentStatusRunning {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should deal with cancellation after deployer pod succeeded", func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc, err := readDCFixture(simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc.Spec.Replicas = 1
			// Make sure the deployer pod doesn't immediately
			dc.Spec.MinReadySeconds = 3
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Create(dc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for RC to be created")
			dc, err = waitForDCModification(oc, namespace, dc.Name, deploymentRunTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					cond := appsutil.GetDeploymentCondition(config.Status, appsapi.DeploymentProgressing)
					if cond != nil && cond.Reason == appsapi.NewReplicationControllerReason {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			rcName := appsutil.LatestDeploymentNameForConfig(dc)

			g.By("waiting for deployer to be completed")
			_, err = waitForPodModification(oc, namespace,
				appsutil.DeployerPodNameForDeployment(rcName),
				deploymentRunTimeout, "",
				func(pod *kapiv1.Pod) (bool, error) {
					switch pod.Status.Phase {
					case kapiv1.PodSucceeded:
						return true, nil
					case kapiv1.PodFailed:
						return true, errors.New("pod failed")
					default:
						return false, nil
					}
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("canceling the deployment")
			rc, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(
				rcName, types.StrategicMergePatchType,
				[]byte(fmt.Sprintf(`{"metadata":{"annotations":{%q: %q, %q: %q}}}`,
					appsapi.DeploymentCancelledAnnotation, appsapi.DeploymentCancelledAnnotationValue,
					appsapi.DeploymentStatusReasonAnnotation, appsapi.DeploymentCancelledByUser,
				)))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(appsutil.DeploymentVersionFor(rc)).To(o.BeEquivalentTo(1))

			g.By("redeploying immediately by config change")
			o.Expect(dc.Spec.Template.Annotations["foo"]).NotTo(o.Equal("bar"))
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(dc.Namespace).Patch(dc.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"metadata":{"annotations":{"foo": "bar"}}}}}`))
			o.Expect(err).NotTo(o.HaveOccurred())
			dc, err = waitForDCModification(oc, namespace, dcName, deploymentRunTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					if config.Status.LatestVersion == 2 {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Wait for deployment pod to be running
			_, err = waitForRCModification(oc, namespace, appsutil.LatestDeploymentNameForConfig(dc), deploymentRunTimeout,
				rc.ResourceVersion, func(currentRC *kapiv1.ReplicationController) (bool, error) {
					switch appsutil.DeploymentStatusFor(currentRC) {
					case appsapi.DeploymentStatusRunning, appsapi.DeploymentStatusComplete:
						return true, nil
					case appsapi.DeploymentStatusFailed:
						return true, fmt.Errorf("deployment '%s/%s' has failed", currentRC.Namespace, currentRC.Name)
					default:
						return false, nil
					}
				})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("won't deploy RC with unresolved images [Conformance]", func() {
		dcName := "example"
		rcName := func(i int) string { return fmt.Sprintf("%s-%d", dcName, i) }
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("when patched with empty image", func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc, err := readDCFixture(imageChangeTriggerFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			rcList, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).List(metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			dc.Spec.Replicas = 1
			dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Create(dc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("tagging the busybox:latest as test:v1 image to create ImageStream")
			out, err := oc.Run("tag").Args("docker.io/busybox:latest", "test:v1").Output()
			e2e.Logf("%s", out)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for deployment #1 to complete")
			_, err = waitForRCModification(oc, namespace, rcName(1), deploymentRunTimeout,
				rcList.ResourceVersion, func(currentRC *kapiv1.ReplicationController) (bool, error) {
					switch appsutil.DeploymentStatusFor(currentRC) {
					case appsapi.DeploymentStatusComplete:
						return true, nil
					case appsapi.DeploymentStatusFailed:
						return true, fmt.Errorf("deployment #1 failed")
					default:
						return false, nil
					}
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("setting DC image repeatedly to empty string to fight with image trigger")
			for i := 0; i < 50; i++ {
				dc, err = oc.AppsClient().Apps().DeploymentConfigs(namespace).Patch(dc.Name, types.StrategicMergePatchType,
					[]byte(`{"spec":{"template":{"spec":{"containers":[{"name":"test","image":""}]}}}}`))
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("waiting to see if it won't deploy RC with invalid revision or the same one multiple times")
			// Wait for image trigger to inject image
			dc, err = waitForDCModification(oc, namespace, dc.Name, deploymentChangeTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					if config.Spec.Template.Spec.Containers[0].Image != "" {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			dcTmp, err := waitForDCModification(oc, namespace, dc.Name, deploymentChangeTimeout,
				dc.GetResourceVersion(), func(config *appsapi.DeploymentConfig) (bool, error) {
					if config.Status.ObservedGeneration >= dc.Generation {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to wait on generation >= %d to be observed by DC %s/%s", dc.Generation, dc.Namespace, dc.Name))
			dc = dcTmp

			rcs, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).List(metav1.ListOptions{
				LabelSelector: appsutil.ConfigSelector(dc.Name).String(),
			})
			o.Expect(rcs.Items).To(o.HaveLen(1))
			o.Expect(strings.TrimSpace(rcs.Items[0].Spec.Template.Spec.Containers[0].Image)).NotTo(o.BeEmpty())
		})
	})
})
