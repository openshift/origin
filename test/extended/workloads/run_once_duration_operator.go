package workloads

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/compat_otp/architecture"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-scheduling][Serial] Workloads Set activeDeadLineseconds using the run-once-duration-override-operator", func() {
	defer g.GinkgoRecover()
	var (
		oc                       = exutil.NewCLI("rodo-operator")
		kubeNamespace            = "openshift-run-once-duration-override-operator"
		buildPruningBaseDir      string
		rodoOperatorGroupT       string
		rodoSubscriptionT        string
		runOnceDurationOverrideT string
		sub                      rodoSubscription
		og                       rodoOperatorgroup
		rodods                   runOnceDurationOverride
		dsPodString              string
	)

	g.BeforeEach(func() {
		buildPruningBaseDir = exutil.FixturePath("testdata", "workloads")
		rodoOperatorGroupT = filepath.Join(buildPruningBaseDir, "rodo_operatorgroup.yaml")
		rodoSubscriptionT = filepath.Join(buildPruningBaseDir, "rodo_subscription.yaml")
		runOnceDurationOverrideT = filepath.Join(buildPruningBaseDir, "rodo_ds.yaml")

		// Dynamically fetch all packagemanifest values instead of hardcoding
		sub = getPackageManifestValues(oc, "run-once-duration-override-operator", "openshift-marketplace")
		sub.template = rodoSubscriptionT
		sub.namespace = kubeNamespace

		og = rodoOperatorgroup{
			name:      "openshift-run-once-duration-override-operator",
			namespace: kubeNamespace,
			template:  rodoOperatorGroupT,
		}

		rodods = runOnceDurationOverride{
			namespace:             kubeNamespace,
			activeDeadlineSeconds: 60,
			template:              runOnceDurationOverrideT,
		}
		// Set expected number of Daemon set pods if cluster is SNO vs normal
		dsPodString = "Running Running Running"

		if isSNOCluster(oc) {
			dsPodString = "Running"
		}

		// Skip tests on arm64 single-arch and multi-arch clusters
		architecture.SkipNonAmd64SingleArch(oc)
		architecture.SkipArchitectures(oc, architecture.MULTI)
		sub.skipMissingCatalogsources(oc)
	})

	// author: knarra@redhat.com
	// OCP-60351, OCP-60352
	g.It("should install Run Once Duration Override operator and verify it works [apigroup:config.openshift.io]", func() {

		podWithRestartPolicy := filepath.Join(buildPruningBaseDir, "pod_with_restart_policy.yaml")
		podWithOnFailurePolicy := filepath.Join(buildPruningBaseDir, "pod_with_on_failure_policy.yaml")

		g.By("Create the run once duration override  namespace")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()
		msg, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Output()
		e2e.Logf("err %v, msg %v", err, msg)
		msg, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("namespace", kubeNamespace, "openshift.io/cluster-monitoring=true", "--overwrite").Output()
		e2e.Logf("err %v, msg %v", err, msg)

		g.By("Create the operatorgroup")
		defer og.deleteOperatorGroup(oc)
		og.createOperatorGroup(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the subscription")
		defer sub.deleteSubscription(oc)
		sub.createSubscription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for the runOnceDurationOverride operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "run-once-duration-override-operator", kubeNamespace, "1"); ok {
			e2e.Logf("RunOnceDurationOverride operator runnnig now\n")
		} else {
			checkOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "run-once-duration-override-operator", "-n", kubeNamespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Output of runoncedurationoverrideoperator after waiting for 5 mins: %s", checkOutput)
			e2e.Failf("Runoncedurationoverrideoperator is not running even after waiting for 5 mins")
		}

		g.By("Create runoncedurationoverride cluster")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("runoncedurationoverride", "--all", "-n", kubeNamespace).Execute()
		rodods.createrunOnceDurationOverride(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			outputReady, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ds", "runoncedurationoverride", "-n", kubeNamespace, "-o=jsonpath={.status.numberReady}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			outputDesired, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ds", "runoncedurationoverride", "-n", kubeNamespace, "-o=jsonpath={.status.desiredNumberScheduled}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if outputReady == outputDesired {
				e2e.Logf("daemonset pods are up:\n%s %s", outputReady, outputDesired)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Expected number of daemonset pods are not ready")

		g.By("Validate that right version of RODO  is running")
		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			rodoDSStatus, rodoPodError := oc.WithoutNamespace().AsAdmin().Run("get").Args("po", "-n", "openshift-run-once-duration-override-operator", "-l=runoncedurationoverride=true", "-ojsonpath='{.items[*].status.phase}'").Output()
			if rodoPodError != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString(dsPodString, rodoDSStatus); !matched {
				e2e.Logf("All the ds pods are not still in running state, retrying  %s", rodoDSStatus)
				return false, nil
			}
			e2e.Logf("All the ds pods are running %s", rodoDSStatus)
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "All rodo pods are not still running after 3 minutes")

		rodoCsvOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", kubeNamespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(rodoCsvOutput, sub.startingCSV)).To(o.BeTrue())

		//Add the k8 dependencies checkpoint for RODO
		g.By("Get the latest version of Kubernetes")
		ocVersion, versionErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-o=jsonpath={.items[0].status.nodeInfo.kubeletVersion}").Output()
		o.Expect(versionErr).NotTo(o.HaveOccurred())
		kubenetesVersion := strings.Split(strings.Split(ocVersion, "+")[0], "v")[1]
		kuberVersion := strings.Split(kubenetesVersion, ".")[0] + "." + strings.Split(kubenetesVersion, ".")[1]
		e2e.Logf("kuberVersion is %s", kuberVersion)

		g.By("Get rebased version of kubernetes from runoncedurationoverride operator")
		minkuberversion, rodoErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-l=operators.coreos.com/run-once-duration-override-operator.openshift-run-once-duration=", "-n", kubeNamespace, "-o=jsonpath={.items[0].spec.minKubeVersion}").Output()
		o.Expect(rodoErr).NotTo(o.HaveOccurred())
		rebasedVersion := strings.Split(minkuberversion, ".")[0] + "." + strings.Split(minkuberversion, ".")[1]
		e2e.Logf("rebasedVersion is %s", rebasedVersion)

		if strings.Contains(rebasedVersion, kuberVersion) || strings.Contains(rebasedVersion, "1.32") {
			e2e.Logf("RODO operator has been rebased with latest kubernetes")
		} else {
			// Mark as e2e.Failf after 4.20 GA
			e2e.Logf("RODO operator not rebased with latest kubernetes")
		}

		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		// Label the test project
		patch := `[{"op":"add", "path":"/metadata/labels/runoncedurationoverrides.admission.runoncedurationoverride.openshift.io~1enabled", "value":"true"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("ns", oc.Namespace(), "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait and check to see if the project has got the label applied
		err = wait.Poll(5*time.Second, 110*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", oc.Namespace(), "-o=jsonpath={.metadata.labels}").Output()
			if err != nil {
				e2e.Logf("err: %v, and try next round...", err.Error())
				return false, nil
			}
			if strings.Contains(output, "runoncedurationoverrides.admission.runoncedurationoverride.openshift.io") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Admission label has not been applied correctly")

		// Debug code adding sleep for sometime to determine if the issue is with timing or RODO
		time.Sleep(30 * time.Second)

		// Create pods with Restart & OnFailure Policy
		podFileList := []string{podWithRestartPolicy, podWithOnFailurePolicy}
		for _, podFile := range podFileList {
			createPodErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", podFile, "-n", oc.Namespace()).Execute()
			o.Expect(createPodErr).NotTo(o.HaveOccurred())
		}

		// Retrieve the pod list and make sure they are running
		podList := []string{"restartpod60352", "onfailurepod60352"}
		for _, pod := range podList {
			checkPodStatus(oc, "app="+pod, oc.Namespace(), "Running")
			// Make sure activeDeadLineSeconds field is set to 60
			err = wait.Poll(15*time.Second, 180*time.Second, func() (bool, error) {
				activeDeadLineSeconds, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", pod, "-n", oc.Namespace(), "-o=jsonpath={.spec.activeDeadlineSeconds}").Output()
				if err != nil {
					e2e.Logf("err: %v, and try next round...", err.Error())
					return false, nil
				}
				if matched, _ := regexp.MatchString("60", activeDeadLineSeconds); !matched {
					e2e.Logf("ActiveDeadLineSeconds on pod %s was not set correctly:%s\n, retrying", pod, activeDeadLineSeconds)
					return false, nil
				}
				e2e.Logf("ActiveDeadLineSeconds on pod %s was set correctly:\n%s", pod, activeDeadLineSeconds)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("ActiveDeadLineSeconds on pod was not set correctly even after waiting for 3 minutes"))
		}
		// Verify that pod no longer runs actively after the activeDeadLineSeconds have reached
		for _, pod := range podList {
			checkPodStatus(oc, "app="+pod, oc.Namespace(), "Failed")
			checkMessage, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", pod, "-n", oc.Namespace()).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if matched, _ := regexp.MatchString("DeadlineExceeded", checkMessage); !matched {
				e2e.Failf("Pod %s has not gone into error state even after reaching activeDeadLineSeconds set in the operator\n", pod)
			}
		}
	})

	// author: knarra@redhat.com
	// OCP-62690
	g.It("should set activeDeadlineSeconds to minimum of pod and RODO values [apigroup:config.openshift.io]", func() {
		podWithActiveDeadLineSeconds := filepath.Join(buildPruningBaseDir, "pod_with_active_dead_line_seconds.yaml")
		podWithAdsGreaterThanOperator := filepath.Join(buildPruningBaseDir, "pod_with_ads_greater_than_operator.yaml")

		g.By("Create the run once duration override  namespace")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()
		msg, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", kubeNamespace).Output()
		e2e.Logf("err %v, msg %v", err, msg)
		msg, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("namespace", kubeNamespace, "openshift.io/cluster-monitoring=true", "--overwrite").Output()
		e2e.Logf("err %v, msg %v", err, msg)

		g.By("Create the operatorgroup")
		defer og.deleteOperatorGroup(oc)
		og.createOperatorGroup(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Create the subscription")
		defer sub.deleteSubscription(oc)
		sub.createSubscription(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for the runOnceDurationOverride operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "run-once-duration-override-operator", kubeNamespace, "1"); ok {
			e2e.Logf("RunOnceDurationOverride operator runnnig now\n")
		} else {
			checkOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "run-once-duration-override-operator", "-n", kubeNamespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Output of runoncedurationoverrideoperator after waiting for 5 mins: %s", checkOutput)
			e2e.Failf("Runoncedurationoverrideoperator is not running even after waiting for 5 mins")
		}

		g.By("Create runoncedurationoverride cluster")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("runoncedurationoverride", "--all", "-n", kubeNamespace).Execute()
		rodods.createrunOnceDurationOverride(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			outputReady, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ds", "runoncedurationoverride", "-n", kubeNamespace, "-o=jsonpath={.status.numberReady}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			outputDesired, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ds", "runoncedurationoverride", "-n", kubeNamespace, "-o=jsonpath={.status.desiredNumberScheduled}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if outputReady == outputDesired {
				e2e.Logf("daemonset pods are up:\n%s %s", outputReady, outputDesired)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Expected number of daemonset pods are not ready")

		g.By("Check the daemonset pods are running well inside openshift-run-once-duration-override-operator")
		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			rodoDSStatus, rodoPodError := oc.WithoutNamespace().AsAdmin().Run("get").Args("po", "-n", "openshift-run-once-duration-override-operator", "-l=runoncedurationoverride=true", "-ojsonpath='{.items[*].status.phase}'").Output()
			if rodoPodError != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if matched, _ := regexp.MatchString(dsPodString, rodoDSStatus); !matched {
				e2e.Logf("All the ds pods are not still in running state, retrying  %s", rodoDSStatus)
				return false, nil
			}
			e2e.Logf("All the ds pods are running %s", rodoDSStatus)
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "All rodo pods are not still running after 3 minutes")

		// Create test project
		g.By("Create test project")
		oc.SetupProject()

		// Label the test project
		patch := `[{"op":"add", "path":"/metadata/labels/runoncedurationoverrides.admission.runoncedurationoverride.openshift.io~1enabled", "value":"true"}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("ns", oc.Namespace(), "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Wait and check to see if the project has got the label applied
		err = wait.Poll(5*time.Second, 110*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ns", oc.Namespace(), "-o=jsonpath={.metadata.labels}").Output()
			if err != nil {
				e2e.Logf("err: %v, and try next round...", err.Error())
				return false, nil
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Namespace label output is :%v", output)
			if strings.Contains(output, "runoncedurationoverrides.admission.runoncedurationoverride.openshift.io") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Admission label has not been applied correctly")

		// Debug code adding sleep for sometime to determine if the issue is with timing or RODO
		time.Sleep(30 * time.Second)

		// Verify that activeDeadLineSeconds value is set as the min value of pod.spec.ActiveDeadLineSeconds & RODO activeDeadLineSeconds
		createPodADSErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", podWithActiveDeadLineSeconds, "-n", oc.Namespace()).Execute()
		o.Expect(createPodADSErr).NotTo(o.HaveOccurred())

		// Make sure activeDeadLineSeconds field is set to 60
		checkPodStatus(oc, "app=podwithactivedeadlineseconds62690", oc.Namespace(), "Running")
		err = wait.Poll(15*time.Second, 180*time.Second, func() (bool, error) {
			activeDeadLineSeconds, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "podwithactivedeadlineseconds62690", "-n", oc.Namespace(), "-o=jsonpath={.spec.activeDeadlineSeconds}").Output()
			if err != nil {
				e2e.Logf("err: %v, and try next round...", err.Error())
				return false, nil
			}
			if matched, _ := regexp.MatchString("60", activeDeadLineSeconds); !matched {
				e2e.Logf("ActiveDeadLineSeconds was not set as the min value between pod & RODO, retrying\n")
				return false, nil
			}
			e2e.Logf("ActiveDeadLineSeconds was set as %s which is the min value between pod & RODO", activeDeadLineSeconds)
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("ActiveDeadLineSeconds on pod was not set as the min value between pod&rodo even after waiting for 3 minutes"))

		// Verify that pod no longer runs actively after the activeDeadLineSeconds have reached
		err = wait.Poll(5*time.Second, 110*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace(), "-l", "app=podwithactivedeadlineseconds62690", "-o=jsonpath={.items[*].status.phase}").Output()
			if err != nil {
				e2e.Logf("err: %v, and try next round...", err.Error())
				return false, nil
			}
			if strings.Contains(output, "Failed") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "The current state of pod podwithactivedeadlineseconds62690 is not expected")

		checkMessage, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", "podwithactivedeadlineseconds62690", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if matched, _ := regexp.MatchString("DeadlineExceeded", checkMessage); !matched {
			e2e.Failf("Pod podwithactivedeadlineseconds62690 has not gone into error state even after reaching activeDeadLineSeconds\n")
		}

		// Update runoncedurationoverride to have activeDeadLineseconds set to the value lesser than the pod
		g.By("Patch the runoncedurationoverride object to set the activeDeadLineSeconds")
		patch = `[{"op":"replace", "path":"/spec/runOnceDurationOverride/spec/activeDeadlineSeconds", "value":80}]`
		err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("runoncedurationoverride", "cluster", "-n", kubeNamespace, "--type=json", "-p", patch).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify ds pods are running fine after patching the runoncedurationoverride operator
		err = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			outputReady, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ds", "runoncedurationoverride", "-n", kubeNamespace, "-o=jsonpath={.status.numberReady}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			outputDesired, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ds", "runoncedurationoverride", "-n", kubeNamespace, "-o=jsonpath={.status.desiredNumberScheduled}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if err != nil {
				e2e.Logf("deploy is still inprogress, error: %s. Trying again", err)
				return false, nil
			}
			if outputReady == outputDesired {
				e2e.Logf("daemonset pods are up:\n%s %s", outputReady, outputDesired)
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Expected number of daemonset pods are not ready")

		// Debug code adding sleep for sometime to determine if the issue is with timing or RODO
		time.Sleep(30 * time.Second)

		createPodADSGOErr := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", podWithAdsGreaterThanOperator, "-n", oc.Namespace()).Execute()
		o.Expect(createPodADSGOErr).NotTo(o.HaveOccurred())

		// Make sure activeDeadLineSeconds field is set to 80
		checkPodStatus(oc, "app=podwithadsgo62690", oc.Namespace(), "Running")
		err = wait.Poll(15*time.Second, 180*time.Second, func() (bool, error) {
			activeDeadLineSeconds, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "podwithadsgo62690", "-n", oc.Namespace(), "-o=jsonpath={.spec.activeDeadlineSeconds}").Output()
			if err != nil {
				e2e.Logf("err: %v, and try next round...", err.Error())
				return false, nil
			}
			if matched, _ := regexp.MatchString("80", activeDeadLineSeconds); !matched {
				e2e.Logf("ActiveDeadLineSeconds was not set as the min value between pod & RODO, value is %s retrying\n", activeDeadLineSeconds)
				return false, nil
			}
			e2e.Logf("ActiveDeadLineSeconds was set as %s which is the min value between pod & RODO", activeDeadLineSeconds)
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("ActiveDeadLineSeconds on pod was not set correctly even after waiting for 3 minutes"))

		// Verify that pod no longer runs actively after the activeDeadLineSeconds have reached
		err = wait.Poll(5*time.Second, 120*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", oc.Namespace(), "-l", "app=podwithadsgo62690", "-o=jsonpath={.items[*].status.phase}").Output()
			if err != nil {
				e2e.Logf("err: %v, and try next round...", err.Error())
				return false, nil
			}
			e2e.Logf("the result of pod:%v", output)
			if strings.Contains(output, "Failed") {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "The current state of pod podwithadsgo62690 is not expected")

		checkMessage, err = oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", "podwithadsgo62690", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if matched, _ := regexp.MatchString("DeadlineExceeded", checkMessage); !matched {
			e2e.Failf("Pod podwithadsgo62690 has not gone into error state even after reaching activeDeadLineSeconds\n")
		}

	})

	// author: yinzhou@redhat.com
	// OCP-83033
	g.It("should define .spec.relatedImages to support disconnected cluster [apigroup:config.openshift.io]", func() {
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ns", kubeNamespace).Execute()
		defer og.deleteOperatorGroup(oc)
		defer sub.deleteSubscription(oc)

		createOperator(oc, sub, og)

		g.By("Wait for the runOnceDurationOverride operator pod running")
		if ok := waitForAvailableRsRunning(oc, "deploy", "run-once-duration-override-operator", kubeNamespace, "1"); ok {
			e2e.Logf("RunOnceDurationOverride operator runnnig now\n")
		} else {
			checkOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deploy", "run-once-duration-override-operator", "-n", kubeNamespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Output of runoncedurationoverrideoperator after waiting for 5 mins: %s", checkOutput)
			e2e.Failf("Runoncedurationoverrideoperator is not running even after waiting for 5 mins")
		}

		g.By("Check the .spec.relatedImages define correctly")
		relatedImages, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", "-n", kubeNamespace, "-o=jsonpath={.items[0].spec.relatedImages}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		if sub.startingCSV != "runoncedurationoverrideoperator.v1.3.0" && strings.Contains(relatedImages, "run-once-duration-override-operator") {
			e2e.Logf("Found the correct related images")
		} else if strings.Contains(relatedImages, "run-once-duration-override-operand") && strings.Contains(relatedImages, "run-once-duration-override-operator") {
			e2e.Logf("Found the correct related images")
		} else {
			e2e.Failf("run-once-duration-override-operator related images are not defined correctly")
		}
	})
})
