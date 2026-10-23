package apiserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	"github.com/openshift/origin/test/extended/single_node"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const desiredTestDuration = 1 * time.Hour

var (
	flakeOnConnection = false
)

func init() {
	// we are delaying these failures because https://github.com/kubernetes/kubernetes/pull/126954 needs time to merge,
	// get backported, then get vendored.
	whenToStartFailingAgain, err := time.Parse(time.RFC3339, "2024-12-02T15:04:05Z")
	if err != nil {
		panic(err)
	}
	flakeOnConnection = time.Now().After(whenToStartFailingAgain)
}

var _ = ginkgo.Describe("[Conformance][Suite:openshift/kube-apiserver/rollout][Jira:kube-apiserver][sig-kube-apiserver] kube-apiserver", func() {
	f := framework.NewDefaultFramework("rollout-resiliency")
	f.SkipNamespaceCreation = true

	oc := exutil.NewCLIWithoutNamespace("rollout-resiliency")

	ginkgo.It("should roll out new revisions without disruption [apigroup:config.openshift.io][apigroup:operator.openshift.io]", func() {
		ctx := context.Background()

		// separate context so we exit our loop, but it is still possible to use the main context for client calls
		shouldEndTestCtx, shouldEndCancelFn := context.WithTimeout(ctx, desiredTestDuration)
		defer shouldEndCancelFn()

		controlPlaneTopology, _ := single_node.GetTopologies(f)
		if controlPlaneTopology == configv1.SingleReplicaTopologyMode {
			e2eskipper.Skipf("SNO always faces disruption on restart")
		}

		operatorClient := oc.AdminOperatorClient()

		kasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
		framework.ExpectNoError(err)
		previousLatestRevision := kasStatus.Status.LatestAvailableRevision - 1

		errs := []error{}
		flakes := []error{}
		for i := 1; i < 1000; i++ { // we exit early when our desired duration finishes, but this gives us a nice counter for output.
			if shouldEndTestCtx.Err() != nil {
				break
			}

			// ensure the kube-apiserver operator is stable
			nextLogTime := time.Now().Add(time.Minute)
			for {
				rolloutNumberWaitForStability := i - 1

				// prevent hot loops, the extra delay doesn't really matter
				time.Sleep(10 * time.Second)
				if shouldEndTestCtx.Err() != nil {
					break
				}

				// this may actually be flaky if the kube-apiserver is rolling out badly.  Keep track of failures so we can
				// fail the run, but don't exit the test here.
				kasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
				if err != nil {
					reportedErr := fmt.Errorf("failed reading clusteroperator, run=%d, time=%v, err=%w", i, time.Now(), err)
					if flakeOnConnection && strings.Contains(err.Error(), "http2: client connection lost") {
						flakes = append(flakes, reportedErr)
						continue
					}
					errs = append(errs, reportedErr)
					continue
				}

				// check to see that every node is at the latest revision
				latestRevision := kasStatus.Status.LatestAvailableRevision
				if latestRevision <= previousLatestRevision {
					framework.Logf("kube-apiserver still has not observed rollout %d: previousLatestRevision=%d, latestRevision=%d", rolloutNumberWaitForStability, previousLatestRevision, latestRevision)
					continue
				}

				nodeNotAtRevisionReasons := []string{}
				for _, nodeStatus := range kasStatus.Status.NodeStatuses {
					if nodeStatus.CurrentRevision != latestRevision {
						nodeNotAtRevisionReasons = append(nodeNotAtRevisionReasons, fmt.Sprintf("node/%v is at revision %d, not %d", nodeStatus.NodeName, nodeStatus.CurrentRevision, latestRevision))
					}
				}
				if len(nodeNotAtRevisionReasons) == 0 {
					break
				}
				if time.Now().After(nextLogTime) {
					framework.Logf("kube-apiserver still not stable after rollout %d: %v", rolloutNumberWaitForStability, strings.Join(nodeNotAtRevisionReasons, "; "))
					nextLogTime = time.Now().Add(time.Minute)
				}
			}
			if shouldEndTestCtx.Err() != nil {
				break
			}

			kasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
			framework.ExpectNoError(err)
			previousLatestRevision = kasStatus.Status.LatestAvailableRevision // our next command will increment it.

			framework.Logf("Forcing API rollout %d", i)
			ginkgo.By(fmt.Sprintf("Forcing API rollout %d", i))
			forceKubeAPIServerRollout(ctx, operatorClient, fmt.Sprintf("rollout %d-", i))
		}

		// Ensure we wait for kube-apiserver to stabilize before exiting the test, in case we were
		// still rolling out when we hit the timeout above
		shouldEndWaitCtx, shouldEndWaitCancelFn := context.WithTimeout(context.Background(), 20*time.Minute)
		defer shouldEndWaitCancelFn()
		err = exutil.WaitForOperatorProgressingFalse(shouldEndWaitCtx, oc.AdminConfigClient(), "kube-apiserver")
		if err != nil {
			errs = append(errs, fmt.Errorf("error waiting for kube-apiserver to stabilize: %w", err))
		}

		if len(errs) > 0 {
			framework.ExpectNoError(errors.Join(errs...))
		}
		if len(flakes) > 0 {
			result.Flakef("errors that will eventually be failures: %v", errors.Join(flakes...))
		}
	})

	// OCP-44738: The installer pod fall-backoff should not happen if latestAvailableRevision > targetRevision
	ginkgo.It("[OTP][OCP-44738] should handle installer pod fall-backoff correctly when latestAvailableRevision greater than targetRevision [APIServer_Disruptive][Disruptive]", ote.Informing(), func() {
		if !isSNOCluster(oc) {
			ginkgo.Skip("This is not a SNO cluster, skip.")
		}

		defer func() {
			compat_otp.By("4) Change Step 1 injection by updating unsupportedConfigOverrides to null")
			patch := `[{"op": "replace", "path": "/spec/unsupportedConfigOverrides", "value": null}]`
			rollOutError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
			framework.ExpectNoError(rollOutError)

			compat_otp.By("5) Performed apiserver force rollout to test step 4 changes.")
			patch = fmt.Sprintf(`[ {"op": "replace", "path": "/spec/forceRedeploymentReason", "value": "Force Redploy %v" } ]`, time.Now().UnixNano())
			patchForceRedploymentError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
			framework.ExpectNoError(patchForceRedploymentError)

			compat_otp.By("6) Check latestAvailableRevision > targetRevision")
			rollOutError = wait.PollUntilContextTimeout(context.Background(), 60*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
				targetRevisionOut, revisionGetErr := oc.WithoutNamespace().Run("get").Args("kubeapiserver/cluster", "-o", "jsonpath={.status.nodeStatuses[*].targetRevision}").Output()
				framework.ExpectNoError(revisionGetErr)
				targetRevision, _ := strconv.Atoi(targetRevisionOut)

				latestAvailableRevisionOut, latestrevisionGetErr := oc.WithoutNamespace().Run("get").Args("kubeapiserver/cluster", "-o", "jsonpath={.status.latestAvailableRevision}").Output()
				framework.ExpectNoError(latestrevisionGetErr)
				latestAvailableRevision, _ := strconv.Atoi(latestAvailableRevisionOut)

				if latestAvailableRevision > targetRevision {
					framework.Logf("Step 6, Test Passed: latestAvailableRevision > targetRevision")
					return true, nil
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(rollOutError, "Step 6, Test Failed: latestAvailableRevision > targetRevision and rollout is not affected")

			compat_otp.By("7) Check Kube-apiserver operator Roll Out Successfully & rollout is not affected")
			rollOutError = wait.PollUntilContextTimeout(context.Background(), 60*time.Second, 900*time.Second, false, func(cxt context.Context) (bool, error) {
				operatorOutput, operatorChkError := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
				if operatorChkError == nil {
					matched, _ := regexp.MatchString("True.*False.*False", operatorOutput)
					if matched {
						framework.Logf("Kube-apiserver operator Roll Out Successfully & rollout is not affected")
						framework.Logf("Step 7, Test Passed")
						return true, nil
					}
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(rollOutError, "Step 7, Test Failed: Kube-apiserver operator failed to Roll Out")
		}()

		compat_otp.By("1) Set the installer pods to fail and try backoff during rollout by injecting error")
		patch := `[{"op": "replace", "path": "/spec/unsupportedConfigOverrides", "value": {"installerErrorInjection":{"failPropability":1.0}}}]`
		patchConfigError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
		framework.ExpectNoError(patchConfigError)

		compat_otp.By("2) Performed apiserver force rollout to test step 1 changes.")
		patch = fmt.Sprintf(`[ {"op": "replace", "path": "/spec/forceRedeploymentReason", "value": "Force Redploy %v" } ]`, time.Now().UnixNano())
		patchForceRedploymentError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
		framework.ExpectNoError(patchForceRedploymentError)

		compat_otp.By("3) Check apiserver created retry installer pods with error and retrying backoff")
		fallbackError := wait.PollUntilContextTimeout(context.Background(), 60*time.Second, 600*time.Second, false, func(cxt context.Context) (bool, error) {
			targetRevision, revisionGetErr := oc.WithoutNamespace().Run("get").Args("kubeapiserver/cluster", "-o", "jsonpath={.status.nodeStatuses[*].targetRevision}").Output()
			framework.ExpectNoError(revisionGetErr)

			// Check apiserver installer pod is failing with retry error
			installerPod, installerPodErr := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l", "app=installer").Output()
			framework.ExpectNoError(installerPodErr)
			cmd := fmt.Sprintf("echo '%v' | grep 'Error' | grep -c 'installer-%v-retry' || true", installerPod, targetRevision)
			retryPodOutput, retryPodChkerr := exec.Command("bash", "-c", cmd).Output()
			framework.ExpectNoError(retryPodChkerr)

			retryPodCount, strConvError := strconv.Atoi(strings.Trim(string(retryPodOutput), "\n"))
			framework.ExpectNoError(strConvError)
			if retryPodCount > 0 {
				framework.Logf("Step 3, Test Passed: Got retry error installer pod")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(fallbackError, "Step 3, Test Failed: Failed to get retry error installer pod")
	})

	// OCP-56693: Make SAR traffic from oauth and openshift apiserver exempt with API Priority and Fairness feature
	ginkgo.It("[OTP][OCP-56693] should make SAR traffic from oauth and openshift apiserver exempt with API Priority and Fairness [APIServer_Disruptive][Slow][Disruptive]", ote.Informing(), func() {
		var (
			patchJSON             = `[{"op": "replace", "path": "/spec/logLevel", "value": "TraceAll"}]`
			restorePatchJSON      = `[{"op": "replace", "path": "/spec/logLevel", "value": "Normal"}]`
			expectedStatus        = map[string]string{"Progressing": "True"}
			kubeApiserverCoStatus = map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
			caseID                = "OCP-56693"
			dirname               = "/tmp/-" + caseID
		)

		defer os.RemoveAll(dirname)
		defer func() {
			compat_otp.By("4)Restoring the loglevel to the default setting ...")
			output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", restorePatchJSON).Output()
			framework.ExpectNoError(err)
			if strings.Contains(output, "patched (no change)") {
				framework.Logf("kubeapiserver/cluster logLevel is not changed to the default values")
			} else {
				framework.Logf("kubeapiserver/cluster logLevel is changed to the default values")
				compat_otp.By("4) Checking KAS operator should be in Progressing and Available after rollout and recovery")
				framework.Logf("Checking kube-apiserver operator should be in Progressing in 100 seconds")
				err = waitCoBecomes(oc, "kube-apiserver", 100, expectedStatus)
				compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator is not start progressing in 100 seconds")

				framework.Logf("Checking kube-apiserver operator should be Available in 1500 seconds")
				err = waitCoBecomes(oc, "kube-apiserver", 1500, kubeApiserverCoStatus)
				compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator is not becomes available in 1500 seconds")
				logLevel, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath={.spec.logLevel}`).Output()
				framework.ExpectNoError(err1)
				o.Expect(logLevel).Should(o.Equal(`Normal`))
			}

		}()

		err := os.MkdirAll(dirname, 0o755)
		framework.ExpectNoError(err)

		compat_otp.By("1) Checking if oauth and openshift apiserver exempt with API Priority and Fairness feature")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("FlowSchema").Output()
		framework.ExpectNoError(err)
		o.Expect(output).Should(o.MatchRegexp("openshift-apiserver-sar.*exempt"))
		o.Expect(output).Should(o.MatchRegexp("openshift-oauth-apiserver-sar.*exempt"))

		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patchJSON).Execute()
		framework.ExpectNoError(err)
		compat_otp.By("2) Checking KAS operator should be in Progressing and Available after rollout and recovery")
		framework.Logf("Checking kube-apiserver operator should be in Progressing in 100 seconds")
		err = waitCoBecomes(oc, "kube-apiserver", 100, expectedStatus)
		compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator is not start progressing in 100 seconds")
		framework.Logf("Checking kube-apiserver operator should be Available in 1500 seconds")
		err = waitCoBecomes(oc, "kube-apiserver", 1500, kubeApiserverCoStatus)
		compat_otp.AssertWaitPollNoErr(err, "kube-apiserver operator is not becomes available in 1500 seconds")
		logLevel, logLevelErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeapiserver/cluster", "-o", `jsonpath={.spec.logLevel}`).Output()
		framework.ExpectNoError(logLevelErr)
		o.Expect(logLevel).Should(o.Equal(`TraceAll`))

		compat_otp.By("3) Checking if SAR traffics from flowschema openshift-apiserver and oauth-apiserver found in KAS logs")
		kasPods, err := compat_otp.GetAllPodsWithLabel(oc, "openshift-kube-apiserver", "app=openshift-kube-apiserver")
		framework.ExpectNoError(err)
		o.Expect(kasPods).ShouldNot(o.BeEmpty())
		for _, kasPod := range kasPods {
			framework.Logf("pod name:%s", kasPod)
			_, errlog := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", "openshift-kube-apiserver", kasPod).OutputToFile(caseID + "/kas.log." + kasPod)
			framework.ExpectNoError(errlog)
		}
		cmd := fmt.Sprintf(`grep 'startRequest' %v | grep 'system:serviceaccount:openshift-apiserver:openshift-apiserver-sa' | grep -iE 'immediate|exempt' | head -1`, dirname+"/kas.log.*")
		framework.Logf("%s", cmd)
		noOASLogs, err := exec.Command("bash", "-c", cmd).Output()
		framework.ExpectNoError(err)
		cmd = fmt.Sprintf(`grep 'startRequest' %v | grep 'system:serviceaccount:openshift-oauth-apiserver:oauth-apiserver-sa' | grep -iE 'immediate|exempt' | head -1`, dirname+"/kas.log.*")
		framework.Logf("%s", cmd)
		noOAUTHLogs, err := exec.Command("bash", "-c", cmd).Output()
		framework.ExpectNoError(err)
		if len(noOASLogs) > 0 && len(noOAUTHLogs) > 0 {
			framework.Logf("Found SAR traffics from flowschema openshift-apiserver:%s", noOASLogs)
			framework.Logf("Found SAR traffics from flowschema oauth-apiserver: %s", noOAUTHLogs)
			framework.Logf("Test Passed!")
		} else {
			framework.Failf("Test Failed: No SAR traffics from flowschema openshift-apiserver and oauth-apiserver found in KAS logs")
		}
	})

	// OCP-78850: SNO kube-apiserver can fall back to last good revision well when failing to roll out
	ginkgo.It("[OTP][OCP-78850] should fall back to last good revision when SNO kube-apiserver fails to roll out [APIServer_Disruptive][Disruptive]", ote.Informing(), func() {
		if !isSNOCluster(oc) {
			ginkgo.Skip("This is not a SNO cluster, skip.")
		}

		nodes, nodeGetError := compat_otp.GetAllNodes(oc)
		framework.ExpectNoError(nodeGetError)

		framework.Logf("Check openshift-kube-apiserver pods current revision before changes")
		out, revisionChkError := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", "jsonpath={.items[*].metadata.labels.revision}").Output()
		framework.ExpectNoError(revisionChkError)
		PreRevision, _ := strconv.Atoi(out)
		framework.Logf("Current revision Count: %v", PreRevision)

		defer func() {
			compat_otp.By("Roll Out Step 1 Changes")
			patch := `[{"op": "replace", "path": "/spec/unsupportedConfigOverrides", "value": null}]`
			rollOutError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
			framework.ExpectNoError(rollOutError)

			compat_otp.By("7) Check Kube-apiserver operator Roll Out with new revision count")
			rollOutError = wait.PollUntilContextTimeout(context.Background(), 100*time.Second, 900*time.Second, false, func(cxt context.Context) (bool, error) {
				Output, operatorChkError := oc.WithoutNamespace().Run("get").Args("co/kube-apiserver").Output()
				if operatorChkError == nil {
					matched, _ := regexp.MatchString("True.*False.*False", Output)
					if matched {
						out, revisionChkErr := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", "jsonpath={.items[*].metadata.labels.revision}").Output()
						PostRevision, _ := strconv.Atoi(out)
						framework.ExpectNoError(revisionChkErr)
						o.Expect(PostRevision).Should(o.BeNumerically(">", PreRevision), "Validation failed as PostRevision value not greater than PreRevision")
						framework.Logf("Kube-apiserver operator Roll Out Successfully with new revision count")
						framework.Logf("Step 7, Test Passed")
						return true, nil
					}
				}
				return false, nil
			})
			compat_otp.AssertWaitPollNoErr(rollOutError, "Step 7, Test Failed: Kube-apiserver operator failed to Roll Out with new revision count")
		}()

		compat_otp.By("1) Add invalid configuration to kube-apiserver to make it failed")
		patch := `[{"op": "replace", "path": "/spec/unsupportedConfigOverrides", "value": {"apiServerArguments":{"foo":["bar"]}}}]`
		configError := oc.AsAdmin().WithoutNamespace().Run("patch").Args("kubeapiserver/cluster", "--type=json", "-p", patch).Execute()
		framework.ExpectNoError(configError)

		compat_otp.By("2) Check new startup-monitor pod created & running under openshift-kube-apiserver project")
		podChkError := wait.PollUntilContextTimeout(context.Background(), 3*time.Second, 180*time.Second, false, func(cxt context.Context) (bool, error) {
			out, runError := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=app=installer", "-o", `jsonpath='{.items[?(@.status.phase=="Running")].status.phase}'`).Output()
			if runError == nil {
				if matched, _ := regexp.MatchString("Running", out); matched {
					framework.Logf("Step 2, Test Passed: Startup-monitor pod created & running under openshift-kube-apiserver project")
					return true, nil
				}
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(podChkError, "Step 2, Test Failed: Failed to Create startup-monitor pod")

		compat_otp.By("3) Check kube-apiserver to fall back to previous good revision")
		fallbackError := wait.PollUntilContextTimeout(context.Background(), 100*time.Second, 900*time.Second, false, func(cxt context.Context) (bool, error) {
			annotations, fallbackErr := oc.WithoutNamespace().Run("get").Args("po", "-n", "openshift-kube-apiserver", "-l=apiserver", "-o", `jsonpath={.items[*].metadata.annotations.startup-monitor\.static-pods\.openshift\.io/fallback-for-revision}`).Output()
			if fallbackErr == nil {
				failedRevision, _ := strconv.Atoi(annotations)
				o.Expect(failedRevision - 1).Should(o.BeNumerically("==", PreRevision))
				compat_otp.By("Check created soft-link kube-apiserver-last-known-good to the last good revision")
				out, fileChkError := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, nodes[0], []string{"--to-namespace=openshift-kube-apiserver"}, "bash", "-c", "ls -l /etc/kubernetes/static-pod-resources/kube-apiserver-last-known-good")
				framework.ExpectNoError(fileChkError)
				o.Expect(out).To(o.ContainSubstring("kube-apiserver-pod.yaml"))
				framework.Logf("Step 3, Test Passed: Cluster is fall back to last good revision")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(fallbackError, "Step 3, Test Failed: Failed to start kube-apiserver with previous good revision")

		compat_otp.By("4: Check startup-monitor pod was created during fallback and currently in Stopped/Removed state")
		cmd := "journalctl -u crio --since '10min ago'| grep 'startup-monitor' | grep -E 'Stopped container|Removed container'"
		out, journalctlErr := compat_otp.DebugNodeRetryWithOptionsAndChroot(oc, nodes[0], []string{"--to-namespace=openshift-kube-apiserver"}, "bash", "-c", cmd)
		framework.ExpectNoError(journalctlErr)
		o.Expect(out).ShouldNot(o.BeEmpty())
		framework.Logf("Step 4, Test Passed : Startup-monitor pod was created and Stopped/Removed state")

		compat_otp.By("5) Check kube-apiserver operator status changed to degraded")
		expectedStatus := map[string]string{"Degraded": "True"}
		operatorChkErr := waitCoBecomes(oc, "kube-apiserver", 900, expectedStatus)
		compat_otp.AssertWaitPollNoErr(operatorChkErr, "Step 5, Test Failed: kube-apiserver operator failed to Degraded")

		compat_otp.By("6) Check kubeapiserver operator nodeStatuses show lastFallbackCount info correctly")
		out, revisionChkErr := oc.WithoutNamespace().Run("get").Args("kubeapiserver/cluster", "-o", "jsonpath='{.status.nodeStatuses[*].lastFailedRevisionErrors}'").Output()
		framework.ExpectNoError(revisionChkErr)
		o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("fallback to last-known-good revision %v took place", PreRevision)))
		framework.Logf("Step 6, Test Passed")
	})

})
