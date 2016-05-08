package deployments

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

const deploymentRunTimeout = 5 * time.Minute

var _ = g.Describe("deploymentconfigs", func() {
	defer g.GinkgoRecover()
	var (
		deploymentFixture       = exutil.FixturePath("..", "extended", "fixtures", "test-deployment-test.yaml")
		simpleDeploymentFixture = exutil.FixturePath("..", "extended", "fixtures", "deployment-simple.yaml")
		customDeploymentFixture = exutil.FixturePath("..", "extended", "fixtures", "custom-deployment.yaml")
		oc                      = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())
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
})

func deploymentStatuses(rcs []kapi.ReplicationController) []string {
	statuses := []string{}
	for _, rc := range rcs {
		statuses = append(statuses, string(deployutil.DeploymentStatusFor(&rc)))
	}
	return statuses
}

func deploymentPods(pods []kapi.Pod) (map[string][]*kapi.Pod, error) {
	deployers := make(map[string][]*kapi.Pod)
	for i := range pods {
		name, ok := pods[i].Labels[deployapi.DeployerPodForDeploymentLabel]
		if !ok {
			continue
		}
		deployers[name] = append(deployers[name], &pods[i])
	}
	return deployers, nil
}

var completedStatuses = sets.NewString(string(deployapi.DeploymentStatusComplete), string(deployapi.DeploymentStatusFailed))

func checkDeployerPodInvariants(deploymentName string, pods []*kapi.Pod) (isRunning, isCompleted bool, err error) {
	running := false
	completed := false
	succeeded := false
	hasDeployer := false

	// find deployment state
	for _, pod := range pods {
		switch {
		case strings.HasSuffix(pod.Name, "-deploy"):
			if hasDeployer {
				return false, false, fmt.Errorf("multiple deployer pods for %q", deploymentName)
			}
			hasDeployer = true

			switch pod.Status.Phase {
			case kapi.PodSucceeded:
				succeeded = true
				completed = true
			case kapi.PodFailed:
				completed = true
			default:
				running = true
			}
		case strings.HasSuffix(pod.Name, "-pre"), strings.HasSuffix(pod.Name, "-mid"), strings.HasSuffix(pod.Name, "-post"):
		default:
			return false, false, fmt.Errorf("deployer pod %q not recognized as being a valid deployment pod", pod.Name)
		}
	}

	// check hook pods
	for _, pod := range pods {
		switch {
		case strings.HasSuffix(pod.Name, "-pre"), strings.HasSuffix(pod.Name, "-mid"), strings.HasSuffix(pod.Name, "-post"):
			switch pod.Status.Phase {
			case kapi.PodSucceeded:
			case kapi.PodFailed:
				if succeeded {
					return false, false, fmt.Errorf("deployer hook pod %q failed but the deployment %q pod succeeded", pod.Name, deploymentName)
				}
			default:
				if completed {
					// TODO: we need to tighten guarantees around hook pods: https://github.com/openshift/origin/issues/8500
					//for i := range pods {
					//	e2e.Logf("deployment %q pod[%d]: %#v", deploymentName, i, pods[i])
					//}
					//return false, false, fmt.Errorf("deployer hook pod %q is still running but the deployment %q is complete", pod.Name, deploymentName)
					//e2e.Logf("deployer hook pod %q is still running but the deployment %q is complete", pod.Name, deploymentName)
				}
			}
		}
	}
	return running, completed, nil
}

func checkDeploymentInvariants(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController, pods []kapi.Pod) error {
	deployers, err := deploymentPods(pods)
	if err != nil {
		return err
	}
	if len(deployers) > len(rcs) {
		existing := sets.NewString()
		for k := range deployers {
			existing.Insert(k)
		}
		for _, rc := range rcs {
			if existing.Has(rc.Name) {
				existing.Delete(rc.Name)
			} else {
				e2e.Logf("ANOMALY: No deployer pod found for deployment %q", rc.Name)
			}
		}
		for k := range existing {
			// TODO: we are missing RCs? https://github.com/openshift/origin/pull/8483#issuecomment-209150611
			e2e.Logf("ANOMALY: Deployer pod found for %q but no RC exists", k)
			//return fmt.Errorf("more deployer pods found than deployments: %#v %#v", deployers, rcs)
		}
	}
	running := sets.NewString()
	completed := 0
	for k, v := range deployers {
		isRunning, isCompleted, err := checkDeployerPodInvariants(k, v)
		if err != nil {
			return err
		}
		if isCompleted {
			completed++
		}
		if isRunning {
			running.Insert(k)
		}
	}
	if running.Len() > 1 {
		return fmt.Errorf("found multiple running deployments: %v", running.List())
	}
	sawStatus := sets.NewString()
	statuses := []string{}
	for _, rc := range rcs {
		status := deployutil.DeploymentStatusFor(&rc)
		if sawStatus.Len() != 0 {
			switch status {
			case deployapi.DeploymentStatusComplete, deployapi.DeploymentStatusFailed:
				if sawStatus.Difference(completedStatuses).Len() != 0 {
					return fmt.Errorf("rc %s was %s, but earlier RCs were not completed: %v", rc.Name, status, statuses)
				}
			case deployapi.DeploymentStatusRunning, deployapi.DeploymentStatusPending:
				if sawStatus.Has(string(status)) {
					return fmt.Errorf("rc %s was %s, but so was an earlier RC: %v", rc.Name, status, statuses)
				}
				if sawStatus.Difference(completedStatuses).Len() != 0 {
					return fmt.Errorf("rc %s was %s, but earlier RCs were not completed: %v", rc.Name, status, statuses)
				}
			case deployapi.DeploymentStatusNew:
			default:
				return fmt.Errorf("rc %s has unexpected status %s: %v", rc.Name, status, statuses)
			}
		}
		sawStatus.Insert(string(status))
		statuses = append(statuses, string(status))
	}
	return nil
}

func deploymentReachedCompletion(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := deployutil.DeploymentVersionFor(&rc)
	if version != dc.Status.LatestVersion {
		return false, nil
	}

	status := rc.Annotations[deployapi.DeploymentStatusAnnotation]
	if deployapi.DeploymentStatus(status) != deployapi.DeploymentStatusComplete {
		return false, nil
	}
	expectedReplicas := dc.Spec.Replicas
	if dc.Spec.Test {
		expectedReplicas = 0
	}
	if rc.Spec.Replicas != expectedReplicas {
		return false, fmt.Errorf("deployment is complete but doesn't have expected spec replicas: %d %d", rc.Spec.Replicas, expectedReplicas)
	}
	if rc.Status.Replicas != expectedReplicas {
		e2e.Logf("POSSIBLE_ANOMALY: deployment is complete but doesn't have expected status replicas: %d %d", rc.Status.Replicas, expectedReplicas)
		return false, nil
	}
	return true, nil
}

func deploymentRunning(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController) (bool, error) {
	if len(rcs) == 0 {
		return false, nil
	}
	rc := rcs[len(rcs)-1]
	version := deployutil.DeploymentVersionFor(&rc)
	if version != dc.Status.LatestVersion {
		//e2e.Logf("deployment %s is not the latest version on DC: %d", rc.Name, version)
		return false, nil
	}

	status := rc.Annotations[deployapi.DeploymentStatusAnnotation]
	switch deployapi.DeploymentStatus(status) {
	case deployapi.DeploymentStatusFailed:
		if deployutil.IsDeploymentCancelled(&rc) {
			return true, nil
		}
		reason := deployutil.DeploymentStatusReasonFor(&rc)
		if reason == "deployer pod no longer exists" {
			return true, nil
		}
		return false, fmt.Errorf("deployment failed: %v", deployutil.DeploymentStatusReasonFor(&rc))
	case deployapi.DeploymentStatusRunning, deployapi.DeploymentStatusComplete:
		return true, nil
	default:
		return false, nil
	}
}

func deploymentInfo(oc *exutil.CLI, name string) (*deployapi.DeploymentConfig, []kapi.ReplicationController, []kapi.Pod, error) {
	dc, err := oc.REST().DeploymentConfigs(oc.Namespace()).Get(name)
	if err != nil {
		return nil, nil, nil, err
	}

	// get pods before RCs, so we see more RCs than pods.
	pods, err := oc.KubeREST().Pods(oc.Namespace()).List(kapi.ListOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	rcs, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).List(kapi.ListOptions{
		LabelSelector: deployutil.ConfigSelector(name),
	})
	if err != nil {
		return nil, nil, nil, err
	}
	sort.Sort(deployutil.ByLatestVersionAsc(rcs.Items))

	return dc, rcs.Items, pods.Items, nil
}

type deploymentConditionFunc func(dc *deployapi.DeploymentConfig, rcs []kapi.ReplicationController) (bool, error)

func waitForLatestCondition(oc *exutil.CLI, name string, timeout time.Duration, fn deploymentConditionFunc) error {
	return wait.Poll(200*time.Millisecond, timeout, func() (bool, error) {
		dc, rcs, pods, err := deploymentInfo(oc, name)
		if err != nil {
			return false, err
		}
		if err := checkDeploymentInvariants(dc, rcs, pods); err != nil {
			return false, err
		}
		return fn(dc, rcs)
	})
}

var _ = g.Describe("deployments: parallel: deployment generation", func() {
	defer g.GinkgoRecover()
	var (
		generationFixture = exutil.FixturePath("..", "extended", "fixtures", "test-deployment.yaml")
		oc                = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())
	)

	g.Describe("deployment generation", func() {
		g.It("should deploy based on a status version bump", func() {
			resource, err := oc.Run("create").Args("-f", generationFixture, "-o", "name").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			parts := strings.Split(resource, "/")
			if len(parts) != 2 {
				o.Expect(fmt.Errorf("expected type/name syntax, got: %s", resource)).NotTo(o.HaveOccurred())
			}
			dcName := parts[1]

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
				rc, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get(dcName + "-" + version)
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
			_, err = oc.Run("deploy").Args("--latest", dcName).Output()
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

			g.By("verifying the second deployment is marked complete")
			err = wait.Poll(100*time.Millisecond, 1*time.Minute, func() (bool, error) {
				rc, err := oc.KubeREST().ReplicationControllers(oc.Namespace()).Get(dcName + "-" + version)
				o.Expect(err).NotTo(o.HaveOccurred())
				return deployutil.IsTerminatedDeployment(rc), nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
