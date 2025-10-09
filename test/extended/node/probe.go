package node

import (
	"path/filepath"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node][NodeQE] NODE Probe feature", func() {
	defer g.GinkgoRecover()
	var (
		oc                           = compat_otp.NewCLI("node-"+getRandomString(), compat_otp.KubeConfigPath())
		buildPruningBaseDir          string
		livenessProbeTemp            string
		startupProbeTemp             string
		readinessProbeTemp           string
		livenessProbeNoTerminateTemp string
	)

	g.BeforeEach(func() {
		buildPruningBaseDir = compat_otp.FixturePath("testdata", "node")
		livenessProbeTemp = filepath.Join(buildPruningBaseDir, "livenessProbe-terminationPeriod.yaml")
		startupProbeTemp = filepath.Join(buildPruningBaseDir, "startupProbe-terminationPeriod.yaml")
		readinessProbeTemp = filepath.Join(buildPruningBaseDir, "readinessProbe-terminationPeriod.yaml")
		livenessProbeNoTerminateTemp = filepath.Join(buildPruningBaseDir, "livenessProbe-without-terminationPeriod.yaml")
	})

	// author: minmli@redhat.com
	g.It("Author:minmli-High-41579-Liveness probe failures should terminate the pod immediately", func() {
		buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
		podProbeT := filepath.Join(buildPruningBaseDir, "pod-liveness-probe.yaml")
		g.By("Test for case OCP-41579")

		g.By("create new namespace")
		oc.SetupProject()

		pod41579 := podLivenessProbe{
			name:                  "probe-pod-41579",
			namespace:             oc.Namespace(),
			overridelivenessgrace: "10",
			terminationgrace:      300,
			failurethreshold:      1,
			periodseconds:         60,
			template:              podProbeT,
		}

		g.By("Create a pod with liveness probe")
		pod41579.createPodLivenessProbe(oc)
		defer pod41579.deletePodLivenessProbe(oc)

		g.By("check pod status")
		err := podStatus(oc, pod41579.namespace, pod41579.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		g.By("check pod events") // create function
		timeout := 90
		keyword := "Container test failed liveness probe, will be restarted"
		err = podEvent(oc, timeout, keyword)
		compat_otp.AssertWaitPollNoErr(err, "event check failed: "+keyword)

		g.By("check pod restart in override termination grace period")
		err = podStatus(oc, pod41579.namespace, pod41579.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
	})

	// author: minmli@redhat.com
	g.It("Author:minmli-High-44493-add configurable terminationGracePeriod to liveness and startup probes", func() {
		var (
			testNs              = oc.Namespace()
			liveProbeTermP44493 = liveProbeTermPeriod{
				name:                  "liveness-probe",
				namespace:             testNs,
				terminationgrace:      60,
				probeterminationgrace: 10,
				template:              livenessProbeTemp,
			}

			startProbeTermP44493 = startProbeTermPeriod{
				name:                  "startup-probe",
				namespace:             testNs,
				terminationgrace:      60,
				probeterminationgrace: 10,
				template:              startupProbeTemp,
			}

			readProbeTermP44493 = readProbeTermPeriod{
				name:                  "readiness-probe",
				namespace:             testNs,
				terminationgrace:      60,
				probeterminationgrace: 10,
				template:              readinessProbeTemp,
			}

			liveProbeNoTermP44493 = liveProbeNoTermPeriod{
				name:             "liveness-probe-no",
				namespace:        testNs,
				terminationgrace: 60,
				template:         livenessProbeNoTerminateTemp,
			}
		)
		g.By("Check if exist any featureSet in featuregate cluster")
		featureSet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("featuregate", "cluster", "-o=jsonpath={.spec.featureSet}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("featureSet is: %s", featureSet)
		if featureSet != "" {
			g.Skip("featureSet is not empty,skip it!")
		}

		g.By("Create a pod with liveness probe with featuregate ProbeTerminationGracePeriod enabled")
		oc.SetupProject()

		liveProbeTermP44493.create(oc)
		ProbeTerminatePeriod(oc, liveProbeTermP44493.terminationgrace, liveProbeTermP44493.probeterminationgrace, liveProbeTermP44493.name, liveProbeTermP44493.namespace, true)
		liveProbeTermP44493.delete(oc)

		g.By("Create a pod with startup probe with featuregate ProbeTerminationGracePeriod enabled")
		startProbeTermP44493.create(oc)
		ProbeTerminatePeriod(oc, startProbeTermP44493.terminationgrace, startProbeTermP44493.probeterminationgrace, startProbeTermP44493.name, startProbeTermP44493.namespace, true)
		startProbeTermP44493.delete(oc)

		g.By("Create a pod with liveness probe but unset terminationGracePeriodSeconds in probe spec")
		liveProbeNoTermP44493.create(oc)
		ProbeTerminatePeriod(oc, liveProbeNoTermP44493.terminationgrace, 0, liveProbeNoTermP44493.name, liveProbeNoTermP44493.namespace, false)
		liveProbeNoTermP44493.delete(oc)

		/*
			// A bug is pending for probe-level terminationGracePeriod, so comment the code temporarily
			//revert featuregate afterwards
			defer func() {
				err = checkMachineConfigPoolStatus(oc, "master")
				compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
				err = checkMachineConfigPoolStatus(oc, "worker")
				compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
			}()

			defer oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate/cluster", "-p", `{"spec":{"featureSet": "CustomNoUpgrade","customNoUpgrade": {"enabled": ["ProbeTerminationGracePeriod"]}}}`, "--type=merge").Execute()

			g.By("Disable ProbeTerminationGracePeriod in featuregate")
			err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate/cluster", "-p", `{"spec":{"featureSet": "CustomNoUpgrade","customNoUpgrade": {"disabled": ["ProbeTerminationGracePeriod"]}}}`, "--type=merge").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

			g.By("Check featuregate take effect")
			featureConfig := []string{"\"ProbeTerminationGracePeriod\": false"}
			err = crioConfigExist(oc, featureConfig, "/etc/kubernetes/kubelet.conf")
			compat_otp.AssertWaitPollNoErr(err, "featureGate is not set as expected")

			g.By("Create a pod with liveness probe with featuregate ProbeTerminationGracePeriod disabled")
			liveProbeTermP44493.name = "liveness-probe"
			liveProbeTermP44493.namespace = oc.Namespace()
			liveProbeTermP44493.terminationgrace = 60
			liveProbeTermP44493.probeterminationgrace = 10
			liveProbeTermP44493.create(oc)
			ProbeTerminatePeriod(oc, liveProbeTermP44493.terminationgrace, liveProbeTermP44493.probeterminationgrace, liveProbeTermP44493.name, liveProbeTermP44493.namespace, false)
			liveProbeTermP44493.delete(oc)

			g.By("Create a pod with startup probe with featuregate ProbeTerminationGracePeriod disabled")
			startProbeTermP44493.name = "startup-probe"
			startProbeTermP44493.namespace = oc.Namespace()
			startProbeTermP44493.terminationgrace = 60
			startProbeTermP44493.probeterminationgrace = 10
			startProbeTermP44493.create(oc)
			ProbeTerminatePeriod(oc, startProbeTermP44493.terminationgrace, startProbeTermP44493.probeterminationgrace, startProbeTermP44493.name, startProbeTermP44493.namespace, false)
			startProbeTermP44493.delete(oc)
		*/

		g.By("Can not create a pod with readiness probe with ProbeTerminationGracePeriodSeconds")
		jsonCfg, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", readProbeTermP44493.template, "-p", "NAME="+readProbeTermP44493.name, "NAMESPACE="+readProbeTermP44493.namespace, "TERMINATIONGRACE="+strconv.Itoa(readProbeTermP44493.terminationgrace), "PROBETERMINATIONGRACE="+strconv.Itoa(readProbeTermP44493.probeterminationgrace)).OutputToFile("node-config-44493.json")
		o.Expect(err).NotTo(o.HaveOccurred())
		out, _ := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", jsonCfg).Output()
		o.Expect(strings.Contains(out, "spec.containers[0].readinessProbe.terminationGracePeriodSeconds: Invalid value")).To(o.BeTrue())
	})
})
