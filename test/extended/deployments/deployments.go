package deployments

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	appsv1 "github.com/openshift/api/apps/v1"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/apps/appsutil"
	"github.com/openshift/library-go/pkg/image/imageutil"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const deploymentRunTimeout = 5 * time.Minute
const deploymentChangeTimeout = 30 * time.Second

type dicEntry struct {
	dic    *deployerPodInvariantChecker
	ctx    context.Context
	cancel func()
}

var _ = g.Describe("[sig-apps][Feature:DeploymentConfig] deploymentconfigs", func() {
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

	oc = exutil.NewCLIWithPodSecurityLevel("cli-deployment", admissionapi.LevelBaseline)

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

	g.Describe("when run iteratively", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should only deploy the last deployment [apigroup:apps.openshift.io]", g.Label("Size:L"), func() {
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
							options := *metav1.NewDeleteOptions(0)
							if r.Float32() < 0.5 {
								options = metav1.DeleteOptions{}
							}
							if err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(context.Background(), pod.Name, options); err != nil {
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

		g.It("should immediately start a new deployment [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			dc, err := createDeploymentConfig(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the deployment config has the correct version"))
			err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, dc.Name)
				if err != nil {
					return false, nil
				}
				return dc.Status.LatestVersion == 1, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = oc.Run("set", "env").Args("dc/"+dc.Name, "TRY=ONCE").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the deployment config has the correct version"))
			err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, dc.Name)
				if err != nil {
					return false, nil
				}
				return dc.Status.LatestVersion == 2, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("by checking that the second deployment exists"))
			err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
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
			err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
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
					if deployer.Status.Phase != corev1.PodFailed && deployer.Status.Phase != corev1.PodSucceeded {
						firstDeployerRemoved = false
					}
				}

				secondDeploymentName := appsutil.DeploymentNameForConfigVersion(dcName, 2)
				secondDeployerRemoved := true
				for _, deployer := range deploymentNamesToDeployers[secondDeploymentName] {
					if deployer.Status.Phase != corev1.PodFailed && deployer.Status.Phase != corev1.PodSucceeded {
						secondDeployerRemoved = false
					}
				}

				return firstDeployerRemoved && !secondDeployerRemoved, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("should respect image stream tag reference policy", func() {
		dcName := "deployment-image-resolution"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("resolve the image pull spec [apigroup:apps.openshift.io][apigroup:image.openshift.io]", g.Label("Size:M"), func() {
			// FIXME: Wrap the IS creation into utility helper
			err := oc.Run("create").Args("-f", resolutionIsFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			dc, err := createDeploymentConfig(oc, resolutionFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			name := "deployment-image-resolution"
			o.Expect(waitForLatestCondition(oc, name, deploymentRunTimeout, deploymentImageTriggersResolved(2))).NotTo(o.HaveOccurred())

			is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(context.Background(), name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(is.Status.DockerImageRepository).NotTo(o.BeEmpty())
			directTag, ok := imageutil.StatusHasTag(is, "direct")
			o.Expect(ok).To(o.BeTrue())
			o.Expect(directTag.Items).NotTo(o.BeEmpty())
			pullthroughTag, ok := imageutil.StatusHasTag(is, "pullthrough")
			o.Expect(ok).To(o.BeTrue())
			o.Expect(pullthroughTag.Items).NotTo(o.BeEmpty())

			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(context.Background(), name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Spec.Triggers).To(o.HaveLen(3))

			imageID := pullthroughTag.Items[0].Image
			resolvedReference := fmt.Sprintf("%s@%s", is.Status.DockerImageRepository, imageID)
			directReference := directTag.Items[0].DockerImageReference

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

	g.Describe("with test deployments", func() {
		dcName := "deployment-test"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should run a deployment to completion and then scale to zero [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			namespace := oc.Namespace()

			dc := ReadFixtureOrFail(deploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))
			e2e.Logf("created DC, creationTimestamp: %v", dc.CreationTimestamp)

			o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			out, err := oc.Run("logs").Args("pod/deployment-test-1-deploy").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("oc logs finished")

			e2e.Logf("verifying the deployment is marked complete and scaled to zero")
			o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			e2e.Logf("checking the logs for substrings\n%s", out)
			o.Expect(out).To(o.ContainSubstring("deployment-test-1 to 2"))
			o.Expect(out).To(o.ContainSubstring("--> pre: Success"))
			o.Expect(out).To(o.ContainSubstring("--> Success"))

			e2e.Logf("verifying that scaling does not result in new pods")
			out, err = oc.Run("scale").Args("dc/deployment-test", "--replicas=1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			e2e.Logf("ensuring no scale up of the deployment happens")
			wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				rc, err := oc.KubeClient().CoreV1().ReplicationControllers(oc.Namespace()).Get(context.Background(), "deployment-test-1", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(*rc.Spec.Replicas).Should(o.BeEquivalentTo(0))
				o.Expect(rc.Status.Replicas).Should(o.BeEquivalentTo(0))
				return false, nil
			})

			e2e.Logf("verifying the scale is updated on the deployment config")
			config, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(context.Background(), "deployment-test", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(config.Spec.Replicas).Should(o.BeEquivalentTo(1))
			o.Expect(config.Spec.Test).Should(o.BeTrue())

			e2e.Logf("deploying a few more times")
			for i := 0; i < 3; i++ {
				rolloutCompleteWithLogs := make(chan struct{})
				out := ""
				go func(rolloutNumber int) {
					defer g.GinkgoRecover()
					defer close(rolloutCompleteWithLogs)
					var err error
					dcName := fmt.Sprintf("deployment-test-%d", rolloutNumber)
					_, err = WaitForDeployerToComplete(oc, dcName, deploymentRunTimeout)
					o.Expect(err).NotTo(o.HaveOccurred())
					out, err = oc.Run("logs").Args(fmt.Sprintf("pod/%s-deploy", dcName)).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
				}(i + 2) // we already did 2 rollouts previously.

				// When the rollout latest is called, we already waiting for the replication
				// controller to be created and scrubbing the deployer logs as soon as the
				// deployer container runs.
				_, err := oc.Run("rollout").Args("latest", "deployment-test").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("waiting for the rollout #%d to finish", i+2)
				<-rolloutCompleteWithLogs
				o.Expect(out).NotTo(o.BeEmpty())
				o.Expect(waitForLatestCondition(oc, "deployment-test", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

				e2e.Logf("checking the logs for substrings\n%s", out)
				o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("deployment-test-%d up to 1", i+2)))
				o.Expect(out).To(o.ContainSubstring("--> pre: Success"))
				o.Expect(out).To(o.ContainSubstring("test pre hook executed"))
				o.Expect(out).To(o.ContainSubstring("--> Success"))
			}
		})
	})

	g.Describe("when changing image change trigger", func() {
		dcName := "example"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should successfully trigger from an updated image [apigroup:apps.openshift.io][apigroup:image.openshift.io]", g.Label("Size:M"), func() {
			dc, err := createDeploymentConfig(oc, imageChangeTriggerFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))
			o.Expect(waitForSyncedConfig(oc, dcName, deploymentRunTimeout)).NotTo(o.HaveOccurred())

			g.By("tagging the initial test:v1 image")
			_, err = oc.Run("tag").Args(image.LimitedShellImage(), "test:v1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			expectLatestVersion := func(version int) {
				dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(context.Background(), dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				latestVersion := dc.Status.LatestVersion
				err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
					dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(context.Background(), dcName, metav1.GetOptions{})
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

			g.By("tagging a different image as test:v2")
			_, err = oc.Run("tag").Args(image.ShellImage(), "test:v2").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring the deployment config latest version is 2 and rollout completed")
			expectLatestVersion(2)
		})
	})

	g.Describe("when tagging images", func() {
		dcName := "tag-images"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should successfully tag the deployed image [apigroup:apps.openshift.io][apigroup:authorization.openshift.io][apigroup:image.openshift.io]", g.Label("Size:M"), func() {
			g.By("creating the deployment config fixture")
			dc, err := createDeploymentConfig(oc, tagImagesFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying the deployer service account can update imagestreamtags and user can get them")
			err = exutil.WaitForUserBeAuthorized(oc, oc.Username(), &authorizationv1.ResourceAttributes{Namespace: oc.Namespace(), Group: "image.openshift.io", Verb: "get", Resource: "imagestreamtags"})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForUserBeAuthorized(oc, "system:serviceaccount:"+oc.Namespace()+":deployer", &authorizationv1.ResourceAttributes{Namespace: oc.Namespace(), Group: "image.openshift.io", Verb: "update", Resource: "imagestreamtags"})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the post deployment action happened: tag is set")
			var istag *imagev1.ImageStreamTag
			pollErr := wait.PollImmediate(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				istag, err = oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get(context.Background(), "sample-stream:deployed", metav1.GetOptions{})
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

			if istag.Tag == nil || istag.Tag.From == nil || istag.Tag.From.Name != image.ShellImage() {
				err = fmt.Errorf("expected %q to be part of the image reference in %#v", image.ShellImage(), istag)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	})

	g.Describe("with env in params referencing the configmap", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should expand the config map key to a value [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
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

	g.Describe("with multiple image change triggers", func() {
		dcName := "example"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should run a successful deployment with multiple triggers [apigroup:apps.openshift.io][apigroup:image.openshift.io]", g.Label("Size:M"), func() {
			g.By("creating DC")

			_, err := oc.Run("import-image").Args("registry.redhat.io/ubi8/ruby-30:latest", "--confirm", "--reference-policy=local").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = oc.Run("import-image").Args("registry.redhat.io/rhel8/postgresql-13:latest", "--confirm", "--reference-policy=local").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			dc, err := createDeploymentConfig(oc, multipleICTFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))
			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})

		g.It("should run a successful deployment with a trigger used by different containers [apigroup:apps.openshift.io][apigroup:image.openshift.io]", g.Label("Size:M"), func() {

			_, err := oc.Run("import-image").Args("registry.redhat.io/ubi8/ruby-30:latest", "--confirm", "--reference-policy=local").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			dc, err := createDeploymentConfig(oc, anotherMultiICTFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with enhanced status", func() {
		dcName := "deployment-simple"

		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should include various info in status [apigroup:apps.openshift.io]", g.Label("Size:S"), func() {
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

	g.Describe("with custom deployments", func() {
		dcName := "custom-deployment"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should run the custom deployment steps [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			namespace := oc.Namespace()

			dc := ReadFixtureOrFail(customDeploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))
			e2e.Logf("created DC, creationTimestamp: %v", dc.CreationTimestamp)

			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			out, err := oc.Run("logs").Args("pod/custom-deployment-1-deploy").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("oc logs finished")

			e2e.Logf("verifying the deployment is marked complete")
			o.Expect(waitForLatestCondition(oc, "custom-deployment", deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			e2e.Logf("checking the logs for substrings\n%s", out)
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
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should print the rollout history [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			dc, err := createDeploymentConfig(oc, simpleDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("waiting for the first rollout to complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(context.Background(), dcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("updating the deployment config in order to trigger a new rollout")
			_, err = updateConfigWithRetries(oc.AppsClient().AppsV1(), oc.Namespace(), dcName, func(update *appsv1.DeploymentConfig) {
				one := int64(1)
				update.Spec.Template.Spec.TerminationGracePeriodSeconds = &one
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			// Wait for latestVersion=2 to be surfaced in the API
			latestVersion := dc.Status.LatestVersion
			err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
				dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(context.Background(), dcName, metav1.GetOptions{})
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
			o.Expect(out).To(o.ContainSubstring("deploymentconfig.apps.openshift.io/deployment-simple"))
			o.Expect(out).To(o.ContainSubstring("REVISION	STATUS		CAUSE"))
			o.Expect(out).To(o.ContainSubstring("1		Complete	config change"))
			o.Expect(out).To(o.ContainSubstring("2		Complete	config change"))
		})
	})

	g.Describe("generation", func() {
		dcName := "generation-test"
		g.AfterEach(func() {
			failureTrap(oc, "generation-test", g.CurrentSpecReport().Failed())
		})

		g.It("should deploy based on a status version bump [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
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

	g.Describe("paused", func() {
		dcName := "paused"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should disable actions on deployments [apigroup:apps.openshift.io]", g.Label("Size:S"), func() {
			dc, err := createDeploymentConfig(oc, pausedDeploymentFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			_, rcs, _, err := deploymentInfo(oc, dcName)
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rcs) != 0 {
				o.Expect(fmt.Errorf("expected no deployment, found %#v", rcs[0])).NotTo(o.HaveOccurred())
			}
			resource := "dc/" + dcName

			g.By("verifying that we cannot start a new deployment via oc rollout")
			out, err := oc.Run("rollout").Args("latest", resource).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cannot deploy a paused deployment config"))

			g.By("verifying that we cannot cancel a deployment")
			out, err = oc.Run("rollout").Args("cancel", resource).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("unable to cancel paused deployment"))

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

			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Patch(context.Background(), dcName, types.StrategicMergePatchType, []byte(`{"spec": {"paused": false}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("making sure it updates observedGeneration after being paused")
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Patch(context.Background(), dcName, types.StrategicMergePatchType, []byte(`{"spec": {"paused": true}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), deploymentChangeTimeout)
			defer cancel()
			_, err = waitForDCModification(ctx, oc.AppsClient().AppsV1(), dc.Namespace, dcName, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				if config.Status.ObservedGeneration >= dc.Generation {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to wait on generation >= %d to be observed by DC %s/%s", dc.Generation, dc.Namespace, dcName))
		})
	})

	g.Describe("with failing hook", func() {
		dcName := "hook"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should get all logs from retried hooks [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
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

	g.Describe("rolled back", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should rollback to an older deployment [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
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

	g.Describe("reaper [Slow]", func() {
		dcName := "brokendeployment"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should delete all failed deployer pods and hook pods [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
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

	g.Describe("initially", func() {
		dcName := "readiness"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should not deploy if pods never transition to ready [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			dc, err := createDeploymentConfig(oc, readinessFixture)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("waiting for the deployment to fail")
			err = waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentFailed)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("with revision history limits", func() {
		dcName := "history-limit"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should never persist more old deployments than acceptable after being observed by the controller [apigroup:apps.openshift.io]", g.Label("Size:L"), func() {
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

	g.Describe("with minimum ready seconds set", func() {
		dcName := "minreadytest"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should not transition the deployment to Complete before satisfied [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			dc := ReadFixtureOrFail(minReadySecondsFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))
			o.Expect(dc.Spec.Triggers).To(o.BeNil())

			rcName := func(i int) string { return fmt.Sprintf("%s-%d", dc.Name, i) }
			namespace := oc.Namespace()

			// This is the last place we can safely say that the time was taken before replicas became ready
			startTime := time.Now()
			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that all pods are ready")
			ctx1, ctx1Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx1Cancel()
			rc1, err := waitForRCState(ctx1, oc.KubeClient().CoreV1(), namespace, rcName(1), func(rc *corev1.ReplicationController) (bool, error) {
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
			ctx2, ctx2Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout+time.Duration(dc.Spec.MinReadySeconds)*time.Second)
			defer ctx2Cancel()
			rc1, err = waitForRCChange(ctx2, oc.KubeClient().CoreV1(), namespace, rc1.Name, rc1.GetResourceVersion(), func(rc *corev1.ReplicationController) (bool, error) {
				if rc.Status.AvailableReplicas == dc.Spec.Replicas {
					return true, nil
				}

				if appsutil.DeploymentStatusFor(rc) == appsv1.DeploymentStatusComplete {
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
			o.Expect(appsutil.DeploymentStatusFor(rc1)).To(o.Equal(appsv1.DeploymentStatusRunning))
			// It should finish right after
			ctx3, ctx3Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx3Cancel()
			rc1, err = waitForRCChange(ctx3, oc.KubeClient().CoreV1(), namespace, rc1.Name, rc1.GetResourceVersion(), func(rc *corev1.ReplicationController) (bool, error) {
				e2e.Logf("Deployment status for RC: %#v", appsutil.DeploymentStatusFor(rc))
				return appsutil.DeploymentStatusFor(rc) == appsv1.DeploymentStatusComplete, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// We might check that minReadySecond passed between pods becoming ready
			// and available but I don't think there is a way to get a timestamp from events
			// and other ways are just flaky.
			// But since we are reusing MinReadySeconds and AvailableReplicas from RC it should be tested there
		})
	})

	g.Describe("ignores deployer and lets the config with a NewReplicationControllerCreated reason", func() {
		dcName := "database"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should let the deployment config with a NewReplicationControllerCreated reason [apigroup:apps.openshift.io]", g.Label("Size:S"), func() {
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
			var conditions []appsv1.DeploymentCondition
			err = wait.PollImmediate(500*time.Millisecond, 30*time.Second, func() (bool, error) {
				dc, _, _, err := deploymentInfo(oc, dcName)
				if err != nil {
					return false, nil
				}
				conditions = dc.Status.Conditions
				cond := appsutil.GetDeploymentCondition(dc.Status, appsv1.DeploymentProgressing)
				return cond != nil && cond.Reason == appsutil.NewReplicationControllerReason, nil
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
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
			failureTrapForDetachedRCs(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should adhere to Three Laws of Controllers [apigroup:apps.openshift.io]", g.Label("Size:L"), func() {
			namespace := oc.Namespace()
			rcName := func(i int) string { return fmt.Sprintf("%s-%d", dcName, i) }

			var dc *appsv1.DeploymentConfig
			var rc1 *corev1.ReplicationController
			var err error

			g.By("should create ControllerRef in RCs it creates", func() {
				dc := ReadFixtureOrFail(simpleDeploymentFixture).(*appsv1.DeploymentConfig)
				// Having more replicas will make us more resilient to pod failures
				dc.Spec.Replicas = 3
				dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				err = waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentRunning)
				o.Expect(err).NotTo(o.HaveOccurred())

				rc1, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Get(context.Background(), rcName(1), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				validRef := HasValidDCControllerRef(dc, rc1)
				o.Expect(validRef).To(o.BeTrue())
			})

			err = waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("releasing RCs that no longer match its selector", func() {
				dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Get(context.Background(), dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				patch := []byte(fmt.Sprintf(`{"metadata": {"labels":{"openshift.io/deployment-config.name": "%s-detached"}}}`, dcName))
				rc1, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(context.Background(), rcName(1), types.StrategicMergePatchType, patch, metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				ctx1, ctx1Cancel := context.WithTimeout(context.Background(), deploymentChangeTimeout)
				defer ctx1Cancel()
				rc1, err = waitForRCChange(ctx1, oc.KubeClient().CoreV1(), namespace, rcName(1), rc1.GetResourceVersion(), rCConditionFromMeta(controllerRefChangeCondition(metav1.GetControllerOf(rc1))))
				o.Expect(err).NotTo(o.HaveOccurred())
				controllerRef := metav1.GetControllerOf(rc1)
				o.Expect(controllerRef).To(o.BeNil())

				ctx2, ctx2Cancel := context.WithTimeout(context.Background(), deploymentChangeTimeout)
				defer ctx2Cancel()
				dc, err = waitForDCModification(ctx2, oc.AppsClient().AppsV1(), namespace, dcName, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
					return config.Status.AvailableReplicas == 0, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(dc.Status.AvailableReplicas).To(o.BeZero())
				o.Expect(dc.Status.UnavailableReplicas).To(o.BeZero())
			})

			g.By("adopting RCs that match its selector and have no ControllerRef", func() {
				patch := []byte(fmt.Sprintf(`{"metadata": {"labels":{"openshift.io/deployment-config.name": "%s"}}}`, dcName))
				rc1, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(context.Background(), rcName(1), types.StrategicMergePatchType, patch, metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				ctx1, ctx1Cancel := context.WithTimeout(context.Background(), deploymentChangeTimeout)
				defer ctx1Cancel()
				rc1, err = waitForRCChange(ctx1, oc.KubeClient().CoreV1(), namespace, rcName(1), rc1.GetResourceVersion(), rCConditionFromMeta(controllerRefChangeCondition(metav1.GetControllerOf(rc1))))
				o.Expect(err).NotTo(o.HaveOccurred())
				validRef := HasValidDCControllerRef(dc, rc1)
				o.Expect(validRef).To(o.BeTrue())

				ctx2, ctx2Cancel := context.WithTimeout(context.Background(), deploymentChangeTimeout)
				defer ctx2Cancel()
				dc, err = waitForDCModification(ctx2, oc.AppsClient().AppsV1(), namespace, dcName, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
					return config.Status.AvailableReplicas == dc.Spec.Replicas, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(dc.Status.AvailableReplicas).To(o.Equal(dc.Spec.Replicas))
				o.Expect(dc.Status.UnavailableReplicas).To(o.BeZero())
			})

			g.By("deleting owned RCs when deleted", func() {
				err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Delete(context.Background(), dcName, metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				err = wait.PollImmediate(200*time.Millisecond, 5*time.Minute, func() (bool, error) {
					pods, err := oc.KubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
					if err != nil {
						return false, err
					}
					return len(pods.Items) == 0, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				err = wait.PollImmediate(200*time.Millisecond, 30*time.Second, func() (bool, error) {
					rcs, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).List(context.Background(), metav1.ListOptions{})
					if err != nil {
						return false, err
					}
					return len(rcs.Items) == 0, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})

	g.Describe("keep the deployer pod invariant valid", func() {
		dcName := "deployment-simple"
		const deploymentCancelledAnnotation = "openshift.io/deployment.cancelled"

		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("should deal with cancellation of running deployment [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc := ReadFixtureOrFail(simpleDeploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc.Spec.Replicas = 1
			// Make sure the deployer pod doesn't end too soon
			dc.Spec.MinReadySeconds = 60
			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for RC to be created")
			ctx1, ctx1Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx1Cancel()
			dc, err = waitForDCModification(ctx1, oc.AppsClient().AppsV1(), namespace, dcName, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				cond := appsutil.GetDeploymentCondition(config.Status, appsv1.DeploymentProgressing)
				if cond != nil && cond.Reason == appsutil.NewReplicationControllerReason {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			g.By("waiting for deployer pod to be running")
			ctx2, ctx2Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx2Cancel()
			rc, err := waitForRCState(ctx2, oc.KubeClient().CoreV1(), namespace, appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion), func(currentRC *corev1.ReplicationController) (bool, error) {
				if appsutil.DeploymentStatusFor(currentRC) == appsv1.DeploymentStatusRunning {
					return true, nil
				}
				return false, nil
			})

			g.By("canceling the deployment")
			rc, err = oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(context.Background(),
				appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion), types.StrategicMergePatchType,
				[]byte(fmt.Sprintf(`{"metadata":{"annotations":{%q: %q, %q: %q}}}`,
					deploymentCancelledAnnotation, "true",
					appsv1.DeploymentStatusReasonAnnotation, "cancelled by the user",
				)), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(appsutil.DeploymentVersionFor(rc)).To(o.Equal(dc.Status.LatestVersion))

			g.By("redeploying immediately by config change")
			o.Expect(dc.Spec.Template.Annotations["foo"]).NotTo(o.Equal("bar"))
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(dc.Namespace).Patch(context.Background(), dc.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"metadata":{"annotations":{"foo": "bar"}}}}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ctx3, ctx3Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx3Cancel()
			dc, err = waitForDCModification(ctx3, oc.AppsClient().AppsV1(), namespace, dcName,
				dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
					if config.Status.LatestVersion == 2 {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Wait for deployment pod to be running
			ctx4, ctx4Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx4Cancel()
			rc, err = waitForRCState(ctx4, oc.KubeClient().CoreV1(), namespace, appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion), func(currentRC *corev1.ReplicationController) (bool, error) {
				if appsutil.DeploymentStatusFor(currentRC) == appsv1.DeploymentStatusRunning {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should deal with config change in case the deployment is still running [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc := ReadFixtureOrFail(simpleDeploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc.Spec.Replicas = 1
			// Make sure the deployer pod doesn't end too soon
			dc.Spec.MinReadySeconds = 60
			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for RC to be created")
			ctx1, ctx1Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx1Cancel()
			dc, err = waitForDCModification(ctx1, oc.AppsClient().AppsV1(), namespace, dc.Name, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				cond := appsutil.GetDeploymentCondition(config.Status, appsv1.DeploymentProgressing)
				if cond != nil && cond.Reason == appsutil.NewReplicationControllerReason {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			g.By("waiting for deployer pod to be running")
			ctx2, ctx2Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx2Cancel()
			_, err = waitForRCState(ctx2, oc.KubeClient().CoreV1(), namespace, appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion), func(currentRC *corev1.ReplicationController) (bool, error) {
				if appsutil.DeploymentStatusFor(currentRC) == appsv1.DeploymentStatusRunning {
					return true, nil
				}
				return false, nil
			})

			g.By("redeploying immediately by config change")
			o.Expect(dc.Spec.Template.Annotations["foo"]).NotTo(o.Equal("bar"))
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(dc.Namespace).Patch(context.Background(), dc.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"metadata":{"annotations":{"foo": "bar"}}}}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ctx3, ctx3Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx3Cancel()
			dc, err = waitForDCModification(ctx3, oc.AppsClient().AppsV1(), namespace, dcName, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				if config.Status.LatestVersion == 2 {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Wait for deployment pod to be running
			ctx4, ctx4Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx4Cancel()
			_, err = waitForRCState(ctx4, oc.KubeClient().CoreV1(), namespace, appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion), func(currentRC *corev1.ReplicationController) (bool, error) {
				if appsutil.DeploymentStatusFor(currentRC) == appsv1.DeploymentStatusRunning {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should deal with cancellation after deployer pod succeeded [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			namespace := oc.Namespace()
			const (
				deploymentCancelledAnnotation    = "openshift.io/deployment.cancelled"
				deploymentStatusReasonAnnotation = "openshift.io/deployment.status-reason"
			)

			g.By("creating DC")
			dc := ReadFixtureOrFail(simpleDeploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc.Spec.Replicas = 1
			// Make sure the deployer pod doesn't immediately
			dc.Spec.MinReadySeconds = 3
			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for RC to be created")
			ctx1, ctx1Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx1Cancel()
			dc, err = waitForDCModification(ctx1, oc.AppsClient().AppsV1(), dc.Namespace, dc.Name, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				cond := appsutil.GetDeploymentCondition(config.Status, appsv1.DeploymentProgressing)
				if cond != nil && cond.Reason == appsutil.NewReplicationControllerReason {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			rcName := appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion)

			g.By("waiting for deployer to be completed")
			podName := appsutil.DeployerPodNameForDeployment(rcName)
			ctx, cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer cancel()
			fieldSelector := fields.OneTermEqualSelector("metadata.name", podName).String()
			lw := &cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
					options.FieldSelector = fieldSelector
					return oc.KubeClient().CoreV1().Pods(namespace).List(ctx, options)
				},
				WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
					options.FieldSelector = fieldSelector
					return oc.KubeClient().CoreV1().Pods(namespace).Watch(ctx, options)
				},
			}
			_, err = watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(e watch.Event) (bool, error) {
				switch e.Type {
				case watch.Added, watch.Modified:
					pod := e.Object.(*corev1.Pod)
					switch pod.Status.Phase {
					case corev1.PodSucceeded:
						return true, nil
					case corev1.PodFailed:
						return true, errors.New("pod failed")
					default:
						return false, nil
					}
				default:
					return true, fmt.Errorf("unexpected event %#v", e)
				}
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("canceling the deployment")
			rc, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).Patch(
				context.Background(),
				rcName, types.StrategicMergePatchType,
				[]byte(fmt.Sprintf(`{"metadata":{"annotations":{%q: %q, %q: %q}}}`,
					deploymentCancelledAnnotation, "true",
					deploymentStatusReasonAnnotation, "cancelled by the user",
				)), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(appsutil.DeploymentVersionFor(rc)).To(o.BeEquivalentTo(1))

			g.By("redeploying immediately by config change")
			o.Expect(dc.Spec.Template.Annotations["foo"]).NotTo(o.Equal("bar"))
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(dc.Namespace).Patch(context.Background(), dc.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"metadata":{"annotations":{"foo": "bar"}}}}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			ctx2, ctx2Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx2Cancel()
			dc, err = waitForDCModification(ctx2, oc.AppsClient().AppsV1(), namespace, dcName, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				if config.Status.LatestVersion == 2 {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Wait for deployment pod to be running
			ctx3, ctx3Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx3Cancel()
			_, err = waitForRCChange(ctx3, oc.KubeClient().CoreV1(), namespace, appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion), rc.ResourceVersion, func(currentRC *corev1.ReplicationController) (bool, error) {
				switch appsutil.DeploymentStatusFor(currentRC) {
				case appsv1.DeploymentStatusRunning, appsv1.DeploymentStatusComplete:
					return true, nil
				case appsv1.DeploymentStatusFailed:
					return true, fmt.Errorf("deployment '%s/%s' has failed", currentRC.Namespace, currentRC.Name)
				default:
					return false, nil
				}
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("won't deploy RC with unresolved images", func() {
		dcName := "example"
		rcName := func(i int) string { return fmt.Sprintf("%s-%d", dcName, i) }
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("when patched with empty image [apigroup:apps.openshift.io]", g.Label("Size:M"), func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc := ReadFixtureOrFail(imageChangeTriggerFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			rcList, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			dc.Spec.Replicas = 1
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("tagging the tools image as test:v1 to create ImageStream")
			out, err := oc.Run("tag").Args(image.ShellImage(), "test:v1").Output()
			e2e.Logf("%s", out)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for deployment #1 to complete")
			ctx1, ctx1Cancel := context.WithTimeout(context.Background(), deploymentRunTimeout)
			defer ctx1Cancel()
			_, err = waitForRCChange(ctx1, oc.KubeClient().CoreV1(), namespace, rcName(1), rcList.ResourceVersion, func(currentRC *corev1.ReplicationController) (bool, error) {
				switch appsutil.DeploymentStatusFor(currentRC) {
				case appsv1.DeploymentStatusComplete:
					return true, nil
				case appsv1.DeploymentStatusFailed:
					return true, fmt.Errorf("deployment #1 failed")
				default:
					return false, nil
				}
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("setting DC image repeatedly to empty string to fight with image trigger")
			for i := 0; i < 50; i++ {
				dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Patch(context.Background(), dc.Name, types.StrategicMergePatchType,
					[]byte(`{"spec":{"template":{"spec":{"containers":[{"name":"test","image":""}]}}}}`), metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("waiting to see if it won't deploy RC with invalid revision or the same one multiple times")
			// Wait for image trigger to inject image
			ctx2, ctx2Cancel := context.WithTimeout(context.Background(), deploymentChangeTimeout)
			defer ctx2Cancel()
			dc, err = waitForDCModification(ctx2, oc.AppsClient().AppsV1(), namespace, dc.Name, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				if config.Spec.Template.Spec.Containers[0].Image != "" {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			ctx3, ctx3Cancel := context.WithTimeout(context.Background(), deploymentChangeTimeout)
			defer ctx3Cancel()
			dcTmp, err := waitForDCModification(ctx3, oc.AppsClient().AppsV1(), namespace, dc.Name, dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
				if config.Status.ObservedGeneration >= dc.Generation {
					return true, nil
				}
				return false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to wait on generation >= %d to be observed by DC %s/%s", dc.Generation, dc.Namespace, dc.Name))
			dc = dcTmp

			rcs, err := oc.KubeClient().CoreV1().ReplicationControllers(namespace).List(context.Background(), metav1.ListOptions{
				LabelSelector: appsutil.ConfigSelector(dc.Name).String(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(rcs.Items).To(o.HaveLen(1))
			o.Expect(strings.TrimSpace(rcs.Items[0].Spec.Template.Spec.Containers[0].Image)).NotTo(o.BeEmpty())
		})
	})

	g.Describe("adoption", func() {
		dcName := "deployment-simple"
		g.AfterEach(func() {
			failureTrap(oc, dcName, g.CurrentSpecReport().Failed())
		})

		g.It("will orphan all RCs and adopt them back when recreated [apigroup:apps.openshift.io]", g.Label("Size:L"), func() {
			namespace := oc.Namespace()

			g.By("creating DC")
			dc := ReadFixtureOrFail(simpleDeploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(0))

			g.By("waiting for initial deployment to complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("modifying the template and triggering new deployment")
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Patch(context.Background(), dcName, types.StrategicMergePatchType, []byte(`{"spec": {"template": {"metadata": {"labels": {"rev": "2"}}}}}`), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			// LatestVersion is always 1 behind on api calls before the controller detects the change and raises it
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			g.By("waiting for the second deployment to complete")
			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())

			g.By("verifying the second deployment")
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Get(context.Background(), dc.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(2))

			g.By("deleting the DC and orphaning RCs")
			deletePropagationOrphan := metav1.DeletePropagationOrphan
			err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Delete(context.Background(), dc.Name, metav1.DeleteOptions{
				PropagationPolicy: &deletePropagationOrphan,
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Wait for deletion
			err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
				_, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Get(context.Background(), dc.Name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return true, nil
				}
				return false, err
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("recreating the DC")
			dc.ResourceVersion = ""
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(context.Background(), dc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			// When a DC is recreated it has LatestVersion 0, it will get updated after adopting the Rcs
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(0))

			dcListWatch := &cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					options.FieldSelector = fields.OneTermEqualSelector("metadata.name", dc.Name).String()
					return oc.AppsClient().AppsV1().DeploymentConfigs(namespace).List(context.Background(), options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					options.FieldSelector = fields.OneTermEqualSelector("metadata.name", dc.Name).String()
					return oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Watch(context.Background(), options)
				},
			}
			preconditionFunc := func(store cache.Store) (bool, error) {
				_, exists, err := store.Get(&metav1.ObjectMeta{Namespace: namespace, Name: dc.Name})
				if err != nil {
					return true, err
				}
				if !exists {
					// We need to make sure we see the object in the cache before we start waiting for events
					// or we would be waiting for the timeout if such object didn't exist.
					return true, kerrors.NewNotFound(schema.GroupResource{
						Group:    "apps.openshift.io/v1",
						Resource: "DeploymentConfig",
					}, dc.Name)
				}
				return false, nil
			}

			g.By("waiting for DC.status.latestVersion to be raised after adopting RCs and availableReplicas to match replicas")
			ctx2, cancel2 := context.WithTimeout(context.TODO(), deploymentChangeTimeout)
			defer cancel2()
			event, err := watchtools.UntilWithSync(ctx2, dcListWatch, &appsv1.DeploymentConfig{}, preconditionFunc, func(e watch.Event) (bool, error) {
				switch e.Type {
				case watch.Added, watch.Modified:
					evDC := e.Object.(*appsv1.DeploymentConfig)
					e2e.Logf("wait: LatestVersion: %d", e.Object.(*appsv1.DeploymentConfig).Status.LatestVersion)
					return evDC.Status.LatestVersion == 2 && evDC.Status.AvailableReplicas == evDC.Spec.Replicas, nil
				case watch.Deleted:
					return true, fmt.Errorf("dc deleted while waiting for latestVersion to be raised")
				case watch.Error:
					return true, kerrors.FromObject(e.Object)
				default:
					return true, fmt.Errorf("unexpected event %#v", e)
				}
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			dc = event.Object.(*appsv1.DeploymentConfig)

			g.By("making sure DC can be scaled")
			newScale := dc.Spec.Replicas + 2
			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Patch(context.Background(), dcName, types.StrategicMergePatchType, []byte(fmt.Sprintf(`{"spec": {"replicas": %d}}`, newScale)), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			ctx3, cancel3 := context.WithTimeout(context.TODO(), deploymentRunTimeout)
			defer cancel3()
			event, err = watchtools.UntilWithSync(ctx3, dcListWatch, &appsv1.DeploymentConfig{}, preconditionFunc, func(e watch.Event) (bool, error) {
				switch e.Type {
				case watch.Added, watch.Modified:
					evDC := e.Object.(*appsv1.DeploymentConfig)
					return evDC.Status.AvailableReplicas == evDC.Spec.Replicas, nil
				case watch.Deleted:
					return true, fmt.Errorf("dc deleted while waiting for latestVersion to be raised")
				case watch.Error:
					return true, kerrors.FromObject(e.Object)
				default:
					return true, fmt.Errorf("unexpected event %#v", e)
				}
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			dc = event.Object.(*appsv1.DeploymentConfig)

			g.By("rolling out new version")
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(2))

			dc, err = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Patch(context.Background(), dcName, types.StrategicMergePatchType, []byte(fmt.Sprintf(`{"spec": {"template": {"metadata": {"labels": {"rev": "%d"}}}}}`, dc.Status.LatestVersion+1)), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(waitForLatestCondition(oc, dcName, deploymentRunTimeout, deploymentReachedCompletion)).NotTo(o.HaveOccurred())
		})
	})
})
