package node_tuning

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node-tuning] NTO should", func() {
	defer g.GinkgoRecover()

	var (
		ntoNamespace        = "openshift-cluster-node-tuning-operator"
		oc                  = exutil.NewCLIWithoutNamespace("nto").AsAdmin()
		buildPruningBaseDir = exutil.FixturePath("testdata", "node_tuning")
		ntoStalldFile       = filepath.Join(buildPruningBaseDir, "nto-stalld.yaml")
		stalldCurrentPID    string
	)

	// OCP-66086 - [OCPBUGS-11150] Node Tuning Operator - NTO Prevent from stalld continually restarting
	// author: liqcui@redhat.com
	// OCP Bugs: https://issues.redhat.com/browse/OCPBUGS-11150

	g.It("OCP-66086 NTO Prevent from stalld continually restarting [Slow]", g.Label("Size:L"), func() {
		e2e.Logf("get the first rhcos worker nodes as label node")
		firstCoreOSWorkerNodes, err := exutil.GetFirstCoreOsWorkerNode(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(firstCoreOSWorkerNodes) == 0 {
			g.Skip("no rhcos worker node was found - skipping test ...")
		}
		e2e.Logf("the firstCoreOSWorkerNodes is:%v", firstCoreOSWorkerNodes)

		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("node", firstCoreOSWorkerNodes, "node-role.kubernetes.io/worker-stalld-", "--overwrite").Execute()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("tuned", "openshift-stalld", "-n", ntoNamespace, "--ignore-not-found").Execute()

		e2e.Logf("label the first rhcos node with node-role.kubernetes.io/worker-stalld=")
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("node", firstCoreOSWorkerNodes, "node-role.kubernetes.io/worker-stalld=", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("create custom profile openshift-stalld")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", ntoStalldFile, "-n", ntoNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("assert if the tuned openshift-stalld created successfully")
		tunedStdOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("tuned", "-n", ntoNamespace).Output()
		e2e.Logf("current tuned status is:\n%s,", tunedStdOut)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tunedStdOut).NotTo(o.BeEmpty())
		o.Expect(tunedStdOut).To(o.ContainSubstring("openshift-stalld"))

		// Assert if profile applied to label node with re-try
		o.Eventually(func() bool {
			appliedStatus, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "profile", firstCoreOSWorkerNodes, `-ojsonpath='{.status.conditions[?(@.type=="Applied")].status}'`).Output()
			tunedProfile, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "profile", firstCoreOSWorkerNodes, "-ojsonpath={.status.tunedProfile}").Output()
			if err1 != nil || err2 != nil || strings.Contains(appliedStatus, "False") || strings.Contains(appliedStatus, "Unknown") || tunedProfile != "openshift-stalld" {
				e2e.Logf("failed to apply custom profile to nodes, the status is %s and profile is %s, check again", appliedStatus, tunedProfile)
			}
			return strings.Contains(appliedStatus, "True") && tunedProfile == "openshift-stalld"
		}, 5*time.Second, time.Second).Should(o.BeTrue())

		e2e.Logf("assert if the custom profile openshift-stalld applied to label node")
		profileStdOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ntoNamespace, "profile", firstCoreOSWorkerNodes, "-ojsonpath={.status.tunedProfile}").Output()
		e2e.Logf("current profile status is [ %s ] on [ %s ]", profileStdOut, firstCoreOSWorkerNodes)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(profileStdOut).NotTo(o.BeEmpty())
		o.Expect(profileStdOut).To(o.ContainSubstring("openshift-stalld"))

		e2e.Logf("check if stalld service is running ...")
		stalldStatus, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, firstCoreOSWorkerNodes, ntoNamespace, "systemctl", "status", "stalld")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stalldStatus).To(o.ContainSubstring("active (running)"))

		e2e.Logf("assert if stalld service restart ...")
		stalldPreviousPID, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, firstCoreOSWorkerNodes, ntoNamespace, "pidof", "stalld")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stalldPreviousPID).NotTo(o.BeEmpty())
		e2e.Logf("record the previous stalld PID is <stalldPreviousPID: %v>", stalldPreviousPID)

		e2e.Logf("start to periodically check stalld PID and compare if stalld pid change")
		// Wait for 10 minutes and check stalld pid in the meantime and exit if we found stalld restarted
		errWait := wait.Poll(2*time.Minute, 10*time.Minute, func() (bool, error) {
			stalldCurrentPID, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, firstCoreOSWorkerNodes, ntoNamespace, "pidof", "stalld")
			e2e.Logf("the current PID of stalld is < %v >", stalldCurrentPID)
			// the wait poll will exit if stalld restart or anbonrmal
			if err != nil || stalldCurrentPID != stalldPreviousPID {
				e2e.Logf("[ NOTE ] <stalldPreviousPID: %v stalldCurrentPID: %v > The PID of stalld has been changed due to stalld service restarted.", stalldPreviousPID, stalldCurrentPID)
				return true, nil
			}
			e2e.Logf("no restart of stalld process as expected, the stalld PID still is %v", stalldCurrentPID)
			return false, nil
		})

		e2e.Logf("get how many minutes stalld process keep up and running ...")
		stalldRuntimeDuration, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, firstCoreOSWorkerNodes, ntoNamespace, "/bin/bash", "-c", "ps -o etime= -p "+stalldCurrentPID)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stalldRuntimeDuration).NotTo(o.BeEmpty())
		e2e.Logf("the the stalld process keep running for %v", stalldRuntimeDuration)

		if errWait != nil {
			e2e.Logf("%v", errWait)
			return
		}

		err = fmt.Errorf("case: %v\nexpected error got because of %v", g.CurrentSpecReport().FullText(), fmt.Sprintf("stalld service restarted : %v", errWait))
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
