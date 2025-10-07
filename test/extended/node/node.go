package node

import (
	"context"
	"fmt"

	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/openshift/origin/test/extended/util/compat_otp/architecture"
	"github.com/tidwall/sjson"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	//e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

var _ = g.Describe("[sig-node][NodeQE] NODE initContainer policy,volume,readines,quota", func() {
	defer g.GinkgoRecover()

	var (
		oc                        = exutil.NewCLI("node-" + getRandomString())
		buildPruningBaseDir       = compat_otp.FixturePath("testdata", "node")
		customTemp                = filepath.Join(buildPruningBaseDir, "pod-modify.yaml")
		podTerminationTemp        = filepath.Join(buildPruningBaseDir, "pod-termination.yaml")
		podInitConTemp            = filepath.Join(buildPruningBaseDir, "pod-initContainer.yaml")
		podSigstoreTemp           = filepath.Join(buildPruningBaseDir, "pod-sigStore.yaml")
		podSleepTemp              = filepath.Join(buildPruningBaseDir, "sleepPod46306.yaml")
		kubeletConfigTemp         = filepath.Join(buildPruningBaseDir, "kubeletconfig-hardeviction.yaml")
		memHogTemp                = filepath.Join(buildPruningBaseDir, "mem-hog-ocp11600.yaml")
		podTwoContainersTemp      = filepath.Join(buildPruningBaseDir, "pod-with-two-containers.yaml")
		podUserNSTemp             = filepath.Join(buildPruningBaseDir, "pod-user-namespace.yaml")
		ctrcfgOverlayTemp         = filepath.Join(buildPruningBaseDir, "containerRuntimeConfig-overlay.yaml")
		podHelloTemp              = filepath.Join(buildPruningBaseDir, "pod-hello.yaml")
		podWkloadCPUTemp          = filepath.Join(buildPruningBaseDir, "pod-workload-cpu.yaml")
		podWkloadCPUNoAnTemp      = filepath.Join(buildPruningBaseDir, "pod-workload-cpu-without-anotation.yaml")
		podNoWkloadCPUTemp        = filepath.Join(buildPruningBaseDir, "pod-without-workload-cpu.yaml")
		runtimeTimeoutTemp        = filepath.Join(buildPruningBaseDir, "kubeletconfig-runReqTout.yaml")
		upgradeMachineConfigTemp1 = filepath.Join(buildPruningBaseDir, "custom-kubelet-test1.yaml")
		upgradeMachineConfigTemp2 = filepath.Join(buildPruningBaseDir, "custom-kubelet-test2.yaml")
		systemreserveTemp         = filepath.Join(buildPruningBaseDir, "kubeletconfig-defaultsysres.yaml")
		podLogLinkTemp            = filepath.Join(buildPruningBaseDir, "pod-loglink.yaml")
		livenessProbeTemp         = filepath.Join(buildPruningBaseDir, "livenessProbe-terminationPeriod.yaml")
		podWASMTemp               = filepath.Join(buildPruningBaseDir, "pod-wasm.yaml")
		podDisruptionBudgetTemp   = filepath.Join(buildPruningBaseDir, "pod-disruption-budget.yaml")
		genericDeploymentTemp     = filepath.Join(buildPruningBaseDir, "generic-deployment.yaml")
		podDevFuseTemp            = filepath.Join(buildPruningBaseDir, "pod-dev-fuse.yaml")
		podCPULoadBalanceTemp     = filepath.Join(buildPruningBaseDir, "pod-cpu-load-balance.yaml")
		ImageconfigContTemp       = filepath.Join(buildPruningBaseDir, "image-config.json")

		ImgConfCont = ImgConfigContDescription{
			name:     "",
			template: ImageconfigContTemp,
		}
		podDevFuse70987 = podDevFuseDescription{
			name:      "",
			namespace: "",
			template:  podDevFuseTemp,
		}

		podLogLink65404 = podLogLinkDescription{
			name:      "",
			namespace: "",
			template:  podLogLinkTemp,
		}

		podWkloadCPU52313 = podNoWkloadCPUDescription{
			name:      "",
			namespace: "",
			template:  podNoWkloadCPUTemp,
		}

		podWkloadCPU52326 = podWkloadCPUDescription{
			name:        "",
			namespace:   "",
			workloadcpu: "",
			template:    podWkloadCPUTemp,
		}

		podWkloadCPU52328 = podWkloadCPUDescription{
			name:        "",
			namespace:   "",
			workloadcpu: "",
			template:    podWkloadCPUTemp,
		}

		podWkloadCPU52329 = podWkloadCPUNoAnotation{
			name:        "",
			namespace:   "",
			workloadcpu: "",
			template:    podWkloadCPUNoAnTemp,
		}

		podHello = podHelloDescription{
			name:      "",
			namespace: "",
			template:  podHelloTemp,
		}

		podUserNS47663 = podUserNSDescription{
			name:      "",
			namespace: "",
			template:  podUserNSTemp,
		}

		podModify = podModifyDescription{
			name:          "",
			namespace:     "",
			mountpath:     "",
			command:       "",
			args:          "",
			restartPolicy: "",
			user:          "",
			role:          "",
			level:         "",
			template:      customTemp,
		}

		podTermination = podTerminationDescription{
			name:      "",
			namespace: "",
			template:  podTerminationTemp,
		}

		podInitCon38271 = podInitConDescription{
			name:      "",
			namespace: "",
			template:  podInitConTemp,
		}

		podSigstore73667 = podSigstoreDescription{
			name:      "",
			namespace: "",
			template:  podSigstoreTemp,
		}

		podSleep = podSleepDescription{
			namespace: "",
			template:  podSleepTemp,
		}

		kubeletConfig = kubeletConfigDescription{
			name:       "",
			labelkey:   "",
			labelvalue: "",
			template:   kubeletConfigTemp,
		}

		memHog = memHogDescription{
			name:       "",
			namespace:  "",
			labelkey:   "",
			labelvalue: "",
			template:   memHogTemp,
		}

		podTwoContainers = podTwoContainersDescription{
			name:      "",
			namespace: "",
			template:  podTwoContainersTemp,
		}

		ctrcfgOverlay = ctrcfgOverlayDescription{
			name:     "",
			overlay:  "",
			template: ctrcfgOverlayTemp,
		}

		runtimeTimeout = runtimeTimeoutDescription{
			name:       "",
			labelkey:   "",
			labelvalue: "",
			template:   runtimeTimeoutTemp,
		}

		upgradeMachineconfig1 = upgradeMachineconfig1Description{
			name:     "",
			template: upgradeMachineConfigTemp1,
		}
		upgradeMachineconfig2 = upgradeMachineconfig2Description{
			name:     "",
			template: upgradeMachineConfigTemp2,
		}
		systemReserveES = systemReserveESDescription{
			name:       "",
			labelkey:   "",
			labelvalue: "",
			template:   systemreserveTemp,
		}
	)
	// author: pmali@redhat.com
	g.It("DEPRECATED-Author:pmali-High-12893-Init containers with restart policy Always", func() {
		oc.SetupProject()
		podModify.name = "init-always-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "Always"

		g.By("create FAILED init container with pod restartPolicy Always")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusReason(oc)
		compat_otp.AssertWaitPollNoErr(err, "pod status does not contain CrashLoopBackOff")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy Always")

		podModify.name = "init-always-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Always"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc, podModify.namespace, podModify.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("DEPRECATED-Author:pmali-High-12894-Init containers with restart policy OnFailure", func() {
		oc.SetupProject()
		podModify.name = "init-onfailure-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "OnFailure"

		g.By("create FAILED init container with pod restartPolicy OnFailure")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusReason(oc)
		compat_otp.AssertWaitPollNoErr(err, "pod status does not contain CrashLoopBackOff")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy OnFailure")

		podModify.name = "init-onfailure-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "OnFailure"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc, podModify.namespace, podModify.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("DEPRECATED-Author:pmali-High-12896-Init containers with restart policy Never", func() {
		oc.SetupProject()
		podModify.name = "init-never-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "exit 1"
		podModify.restartPolicy = "Never"

		g.By("create FAILED init container with pod restartPolicy Never")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusterminatedReason(oc)
		compat_otp.AssertWaitPollNoErr(err, "pod status does not contain Error")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with pod restartPolicy Never")

		podModify.name = "init-never-succ"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc, podModify.namespace, podModify.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("DEPRECATED-Author:pmali-High-12911-App container status depends on init containers exit code	", func() {
		oc.SetupProject()
		podModify.name = "init-fail"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/false"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		g.By("create FAILED init container with exit code and command /bin/false")
		podModify.create(oc)
		g.By("Check pod failure reason")
		err := podStatusterminatedReason(oc)
		compat_otp.AssertWaitPollNoErr(err, "pod status does not contain Error")
		g.By("Delete Pod ")
		podModify.delete(oc)

		g.By("create SUCCESSFUL init container with command /bin/true")
		podModify.name = "init-success"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/true"
		podModify.args = "sleep 30"
		podModify.restartPolicy = "Never"

		podModify.create(oc)
		g.By("Check pod Status")
		err = podStatus(oc, podModify.namespace, podModify.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Delete Pod ")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("DEPRECATED-Author:pmali-High-12913-Init containers with volume work fine", func() {

		oc.SetupProject()
		podModify.name = "init-volume"
		podModify.namespace = oc.Namespace()
		podModify.mountpath = "/init-test"
		podModify.command = "/bin/bash"
		podModify.args = "echo This is OCP volume test > /work-dir/volume-test"
		podModify.restartPolicy = "Never"

		g.By("Create a pod with initContainer using volume\n")
		podModify.create(oc)
		g.By("Check pod status")
		err := podStatus(oc, podModify.namespace, podModify.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Check Vol status\n")
		err = volStatus(oc)
		compat_otp.AssertWaitPollNoErr(err, "Init containers with volume do not work fine")
		g.By("Delete Pod\n")
		podModify.delete(oc)
	})

	// author: pmali@redhat.com
	g.It("Author:pmali-Medium-30521-CRIO Termination Grace Period test", func() {

		oc.SetupProject()
		podTermination.name = "pod-termination"
		podTermination.namespace = oc.Namespace()

		g.By("Create a pod with termination grace period\n")
		podTermination.create(oc)
		g.By("Check pod status\n")
		err := podStatus(oc, podTermination.namespace, podTermination.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Check container TimeoutStopUSec\n")
		err = podTermination.getTerminationGrace(oc)
		compat_otp.AssertWaitPollNoErr(err, "terminationGracePeriodSeconds is not valid")
		g.By("Delete Pod\n")
		podTermination.delete(oc)
	})

	// author: minmli@redhat.com
	g.It("Author:minmli-High-38271-Init containers should not restart when the exited init container is removed from node", func() {
		g.By("Test for case OCP-38271")
		oc.SetupProject()
		podInitCon38271.name = "initcon-pod"
		podInitCon38271.namespace = oc.Namespace()

		g.By("Create a pod with init container")
		podInitCon38271.create(oc)
		defer podInitCon38271.delete(oc)

		g.By("Check pod status")
		err := podStatus(oc, podInitCon38271.namespace, podInitCon38271.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		g.By("Check init container exit normally")
		err = podInitCon38271.containerExit(oc)
		compat_otp.AssertWaitPollNoErr(err, "conainer not exit normally")

		g.By("Delete init container")
		_, err = podInitCon38271.deleteInitContainer(oc)
		compat_otp.AssertWaitPollNoErr(err, "fail to delete container")

		g.By("Check init container not restart again")
		err = podInitCon38271.initContainerNotRestart(oc)
		compat_otp.AssertWaitPollNoErr(err, "init container restart")
	})

	// author: schoudha@redhat.com
	g.It("Author:schoudha-High-70987-Allow dev fuse by default in CRI-O", func() {
		compat_otp.By("Test for case OCP-70987")
		podDevFuse70987.name = "pod-devfuse"
		podDevFuse70987.namespace = oc.Namespace()

		defer podDevFuse70987.delete(oc)
		compat_otp.By("Create a pod with dev fuse")
		podDevFuse70987.create(oc)

		compat_otp.By("Check pod status")
		err := podStatus(oc, podDevFuse70987.namespace, podDevFuse70987.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("Check if dev fuse is mounted inside the pod")
		err = checkDevFuseMount(oc, podDevFuse70987.namespace, podDevFuse70987.name)
		compat_otp.AssertWaitPollNoErr(err, "dev fuse is not mounted inside pod")
	})

	// author: pmali@redhat.com
	g.It("DEPRECATED-NonPreRelease-Longduration-Author:pmali-High-46306-Node should not becomes NotReady with error creating container storage layer not known[Disruptive][Slow]", func() {

		oc.SetupProject()
		podSleep.namespace = oc.Namespace()

		g.By("Get Worker Node and Add label app=sleep\n")
		workerNodeName := getSingleWorkerNode(oc)
		addLabelToResource(oc, "app=sleep", workerNodeName, "nodes")
		defer removeLabelFromNode(oc, "app-", workerNodeName, "nodes")

		g.By("Create a 50 pods on the same node\n")
		for i := 0; i < 50; i++ {
			podSleep.create(oc)
		}

		g.By("Check pod status\n")
		err := podStatus(oc, podModify.namespace, podModify.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is NOT running")

		g.By("Delete project\n")
		go podSleep.deleteProject(oc)

		g.By("Reboot Worker node\n")
		go rebootNode(oc, workerNodeName)

		//g.By("****** Reboot Worker Node ****** ")
		//compat_otp.DebugNodeWithChroot(oc, workerNodeName, "reboot")

		//g.By("Check Nodes Status\n")
		//err = checkNodeStatus(oc, workerNodeName)
		//compat_otp.AssertWaitPollNoErr(err, "node is not ready")

		g.By("Get Master node\n")
		masterNode := getSingleMasterNode(oc)

		g.By("Check Master Node Logs\n")
		err = masterNodeLog(oc, masterNode)
		compat_otp.AssertWaitPollNoErr(err, "Logs Found, Test Failed")
	})

	// author: pmali@redhat.com
	g.It("DEPRECATED-Longduration-NonPreRelease-Author:pmali-Medium-11600-kubelet will evict pod immediately when met hard eviction threshold memory [Disruptive][Slow]", func() {

		oc.SetupProject()
		kubeletConfig.name = "kubeletconfig-ocp11600"
		kubeletConfig.labelkey = "custom-kubelet-ocp11600"
		kubeletConfig.labelvalue = "hard-eviction"

		memHog.name = "mem-hog-ocp11600"
		memHog.namespace = oc.Namespace()
		memHog.labelkey = kubeletConfig.labelkey
		memHog.labelvalue = kubeletConfig.labelvalue

		g.By("Get Worker Node and Add label custom-kubelet-ocp11600=hard-eviction\n")
		addLabelToResource(oc, "custom-kubelet-ocp11600=hard-eviction", "worker", "mcp")
		defer removeLabelFromNode(oc, "custom-kubelet-ocp11600-", "worker", "mcp")

		g.By("Create Kubelet config \n")
		kubeletConfig.create(oc)
		defer getmcpStatus(oc, "worker") // To check all the Nodes are in Ready State after deleteing kubeletconfig
		defer cleanupObjectsClusterScope(oc, objectTableRefcscope{"kubeletconfig", "kubeletconfig-ocp11600"})

		g.By("Make sure Worker mcp is Updated correctly\n")
		err := getmcpStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "mcp is not updated")

		g.By("Create a 10 pods on the same node\n")
		for i := 0; i < 10; i++ {
			memHog.create(oc)
		}
		defer cleanupObjectsClusterScope(oc, objectTableRefcscope{"ns", oc.Namespace()})

		g.By("Check worker Node events\n")
		workerNodeName := getSingleWorkerNode(oc)
		err = getWorkerNodeDescribe(oc, workerNodeName)
		compat_otp.AssertWaitPollNoErr(err, "Logs did not Found memory pressure, Test Failed")
	})

	// author: weinliu@redhat.com
	g.It("Author:weinliu-Critical-11055-/dev/shm can be automatically shared among all of a pod's containers", func() {
		g.By("Test for case OCP-11055")
		oc.SetupProject()
		podTwoContainers.name = "pod-twocontainers"
		podTwoContainers.namespace = oc.Namespace()
		g.By("Create a pod with two containers")
		podTwoContainers.create(oc)
		defer podTwoContainers.delete(oc)
		g.By("Check pod status")
		err := podStatus(oc, podTwoContainers.namespace, podTwoContainers.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")
		g.By("Enter container 1 and write files")
		_, err = compat_otp.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift", "echo 'written_from_container1' > /dev/shm/c1")
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Enter container 2 and check whether it can share container 1 shared files")
		containerFile1, err := compat_otp.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift-fedora", "cat /dev/shm/c1")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Container1 File Content is: %v", containerFile1)
		o.Expect(containerFile1).To(o.Equal("written_from_container1"))
		g.By("Enter container 2 and write files")
		_, err = compat_otp.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift-fedora", "echo 'written_from_container2' > /dev/shm/c2")
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Enter container 1 and check whether it can share container 2 shared files")
		containerFile2, err := compat_otp.RemoteShPodWithBashSpecifyContainer(oc, podTwoContainers.namespace, podTwoContainers.name, "hello-openshift", "cat /dev/shm/c2")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Container2 File Content is: %v", containerFile2)
		o.Expect(containerFile2).To(o.Equal("written_from_container2"))
	})

	// author: minmli@redhat.com
	g.It("DEPRECATED-Author:minmli-High-47663-run pods in user namespaces via crio workload annotation", func() {
		oc.SetupProject()
		g.By("Test for case OCP-47663")
		podUserNS47663.name = "userns-47663"
		podUserNS47663.namespace = oc.Namespace()

		g.By("Check workload of openshift-builder exist in crio config")
		err := podUserNS47663.crioWorkloadConfigExist(oc)
		compat_otp.AssertWaitPollNoErr(err, "crio workload config not exist")

		g.By("Check user containers exist in /etc/sub[ug]id")
		err = podUserNS47663.userContainersExistForNS(oc)
		compat_otp.AssertWaitPollNoErr(err, "user containers not exist for user namespace")

		g.By("Create a pod with annotation of openshift-builder workload")
		podUserNS47663.createPodUserNS(oc)
		defer podUserNS47663.deletePodUserNS(oc)

		g.By("Check pod status")
		err = podStatus(oc, podUserNS47663.namespace, podUserNS47663.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		g.By("Check pod run in user namespace")
		err = podUserNS47663.podRunInUserNS(oc)
		compat_otp.AssertWaitPollNoErr(err, "pod not run in user namespace")
	})

	// author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-52328-set workload resource usage from pod level : pod should not take effect if not defaulted or specified in workload [Disruptive][Slow]", func() {
		oc.SetupProject()
		compat_otp.By("Test for case OCP-52328")

		compat_otp.By("Create a machine config for workload setting")
		mcCPUOverride := filepath.Join(buildPruningBaseDir, "machineconfig-cpu-override-52328.yaml")
		mcpName := "worker"
		defer func() {
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + mcCPUOverride).Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + mcCPUOverride).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check mcp finish rolling out")
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check workload setting is as expected")
		wkloadConfig := []string{"crio.runtime.workloads.management", "activation_annotation = \"io.openshift.manager\"", "annotation_prefix = \"io.openshift.workload.manager\"", "crio.runtime.workloads.management.resources", "cpushares = 512"}
		configPath := "/etc/crio/crio.conf.d/01-workload.conf"
		err = configExist(oc, wkloadConfig, configPath)
		compat_otp.AssertWaitPollNoErr(err, "workload setting is not set as expected")

		compat_otp.By("Create a pod not specify cpuset in workload setting by annotation")
		defer podWkloadCPU52328.delete(oc)
		podWkloadCPU52328.name = "wkloadcpu-52328"
		podWkloadCPU52328.namespace = oc.Namespace()
		podWkloadCPU52328.workloadcpu = "{\"cpuset\": \"\", \"cpushares\": 1024}"
		podWkloadCPU52328.create(oc)

		compat_otp.By("Check pod status")
		err = podStatus(oc, podWkloadCPU52328.namespace, podWkloadCPU52328.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("Check the pod only override cpushares")
		cpuset := ""
		err = overrideWkloadCPU(oc, cpuset, podWkloadCPU52328.namespace)
		compat_otp.AssertWaitPollNoErr(err, "the pod not only override cpushares in workload setting")
	})

	// author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-52313-High-52326-High-52329-set workload resource usage from pod level : pod can get configured to defaults and override defaults and pod should not be set if annotation not specified [Disruptive][Slow]", func() {
		oc.SetupProject()
		compat_otp.By("Test for case OCP-52313, OCP-52326 and OCP-52329")

		compat_otp.By("Create a machine config for workload setting")
		mcCPUOverride := filepath.Join(buildPruningBaseDir, "machineconfig-cpu-override.yaml")
		defer func() {
			mcpName := "worker"
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + mcCPUOverride).Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + mcCPUOverride).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check mcp finish rolling out")
		mcpName := "worker"
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check workload setting is as expected")
		wkloadConfig := []string{"crio.runtime.workloads.management", "activation_annotation = \"io.openshift.manager\"", "annotation_prefix = \"io.openshift.workload.manager\"", "crio.runtime.workloads.management.resources", "cpushares = 512", "cpuset = \"0\""}
		configPath := "/etc/crio/crio.conf.d/01-workload.conf"
		err = configExist(oc, wkloadConfig, configPath)
		compat_otp.AssertWaitPollNoErr(err, "workload setting is not set as expected")

		compat_otp.By("Create a pod with default workload setting by annotation")
		podWkloadCPU52313.name = "wkloadcpu-52313"
		podWkloadCPU52313.namespace = oc.Namespace()
		podWkloadCPU52313.create(oc)

		compat_otp.By("Check pod status")
		err = podStatus(oc, podWkloadCPU52313.namespace, podWkloadCPU52313.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("Check the pod get configured to default workload setting")
		cpuset := "0"
		err = overrideWkloadCPU(oc, cpuset, podWkloadCPU52313.namespace)
		compat_otp.AssertWaitPollNoErr(err, "the pod is not configured to default workload setting")
		podWkloadCPU52313.delete(oc)

		compat_otp.By("Create a pod override the default workload setting by annotation")
		podWkloadCPU52326.name = "wkloadcpu-52326"
		podWkloadCPU52326.namespace = oc.Namespace()
		podWkloadCPU52326.workloadcpu = "{\"cpuset\": \"0-1\", \"cpushares\": 200}"
		podWkloadCPU52326.create(oc)

		compat_otp.By("Check pod status")
		err = podStatus(oc, podWkloadCPU52326.namespace, podWkloadCPU52326.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("Check the pod override the default workload setting")
		cpuset = "0-1"
		err = overrideWkloadCPU(oc, cpuset, podWkloadCPU52326.namespace)
		compat_otp.AssertWaitPollNoErr(err, "the pod not override the default workload setting")
		podWkloadCPU52326.delete(oc)

		compat_otp.By("Create a pod without annotation but with prefix")
		defer podWkloadCPU52329.delete(oc)
		podWkloadCPU52329.name = "wkloadcpu-52329"
		podWkloadCPU52329.namespace = oc.Namespace()
		podWkloadCPU52329.workloadcpu = "{\"cpuset\": \"0-1\", \"cpushares\": 1800}"
		podWkloadCPU52329.create(oc)

		compat_otp.By("Check pod status")
		err = podStatus(oc, podWkloadCPU52329.namespace, podWkloadCPU52329.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("Check the pod keep default workload setting")
		cpuset = "0-1"
		err = defaultWkloadCPU(oc, cpuset, podWkloadCPU52329.namespace)
		compat_otp.AssertWaitPollNoErr(err, "the pod not keep efault workload setting")
	})

	// author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-LEVEL0-High-46313-set overlaySize in containerRuntimeConfig should take effect in container [Disruptive][Slow]", func() {
		oc.SetupProject()
		g.By("Test for case OCP-46313")
		ctrcfgOverlay.name = "ctrcfg-46313"
		ctrcfgOverlay.overlay = "9G"

		g.By("Create a containerRuntimeConfig to set overlaySize")
		ctrcfgOverlay.create(oc)
		defer func() {
			g.By("Deleting configRuntimeConfig")
			cleanupObjectsClusterScope(oc, objectTableRefcscope{"ContainerRuntimeConfig", "ctrcfg-46313"})
			g.By("Check mcp finish rolling out")
			err := getmcpStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "mcp is not updated")
		}()

		g.By("Check mcp finish rolling out")
		err := getmcpStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "mcp is not updated")

		g.By("Check overlaySize take effect in config file")
		err = checkOverlaySize(oc, ctrcfgOverlay.overlay)
		compat_otp.AssertWaitPollNoErr(err, "overlaySize not take effect")

		g.By("Create a pod")
		podTermination.name = "pod-46313"
		podTermination.namespace = oc.Namespace()
		podTermination.create(oc)
		defer podTermination.delete(oc)

		g.By("Check pod status")
		err = podStatus(oc, podTermination.namespace, podTermination.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		g.By("Check in pod the root partition size for Overlay is correct.")
		err = checkPodOverlaySize(oc, ctrcfgOverlay.overlay)
		compat_otp.AssertWaitPollNoErr(err, "pod overlay size is not correct !!!")
	})

	g.It("Author:minmli-High-56266-kubelet/crio will delete netns when a pod is deleted", func() {
		g.By("Test for case OCP-56266")
		oc.SetupProject()

		g.By("Create a pod")
		podHello.name = "pod-56266"
		podHello.namespace = oc.Namespace()
		podHello.create(oc)

		g.By("Check pod status")
		err := podStatus(oc, podHello.namespace, podHello.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		g.By("Get Pod's Node name")
		hostname := getPodNodeName(oc, podHello.namespace)

		g.By("Get Pod's NetNS")
		netNsPath, err := getPodNetNs(oc, hostname)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Delete the pod")
		podHello.delete(oc)

		g.By("Check the NetNs file was cleaned")
		err = checkNetNs(oc, hostname, netNsPath)
		compat_otp.AssertWaitPollNoErr(err, "the NetNs file is not cleaned !!!")
	})

	g.It("Author:minmli-High-55486-check not exist error MountVolume SetUp failed for volume serviceca object openshift-image-registry serviceca not registered", func() {
		g.By("Test for case OCP-55486")
		oc.SetupProject()

		g.By("Check events of each cronjob")
		err := checkEventsForErr(oc)
		compat_otp.AssertWaitPollNoErr(err, "Found error: MountVolume.SetUp failed for volume ... not registered ")
	})
	//author: asahay@redhat.com
	g.It("Author:asahay-Medium-55033-check KUBELET_LOG_LEVEL is 2", func() {
		g.By("Test for OCP-55033")
		g.By("check Kubelet Log Level\n")
		assertKubeletLogLevel(oc)
	})

	//author: asahay@redhat.com
	g.It("Author:asahay-NonHyperShiftHOST-NonPreRelease-Longduration-LEVEL0-High-52472-update runtimeRequestTimeout parameter using KubeletConfig CR [Disruptive][Slow]", func() {

		oc.SetupProject()
		runtimeTimeout.name = "kubeletconfig-52472"
		runtimeTimeout.labelkey = "custom-kubelet"
		runtimeTimeout.labelvalue = "test-timeout"

		g.By("Label mcp worker custom-kubelet as test-timeout \n")
		addLabelToResource(oc, "custom-kubelet=test-timeout", "worker", "mcp")
		defer removeLabelFromNode(oc, "custom-kubelet-", "worker", "mcp")

		g.By("Create KubeletConfig \n")
		defer func() {
			mcpName := "worker"
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer runtimeTimeout.delete(oc)
		runtimeTimeout.create(oc)

		g.By("Check mcp finish rolling out")
		mcpName := "worker"
		err := checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		g.By("Check Runtime Request Timeout")
		runTimeTimeout(oc)
	})

	//author :asahay@redhat.com

	g.It("Author:asahay-NonHyperShiftHOST-NonPreRelease-PreChkUpgrade-High-45436-Upgrading a cluster by making sure not keep duplicate machine config when it has multiple kubeletconfig [Disruptive][Slow]", func() {

		upgradeMachineconfig1.name = "max-pod"
		upgradeMachineconfig2.name = "max-pod-1"
		g.By("Create first KubeletConfig \n")
		upgradeMachineconfig1.create(oc)

		g.By("Check mcp finish rolling out")
		mcpName := "worker"
		err := checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		g.By("Create second KubeletConfig \n")
		upgradeMachineconfig2.create(oc)

		g.By("Check mcp finish rolling out")
		mcpName1 := "worker"
		err1 := checkMachineConfigPoolStatus(oc, mcpName1)
		compat_otp.AssertWaitPollNoErr(err1, "macineconfigpool worker update failed")

	})

	g.It("Author:asahay-NonHyperShiftHOST-NonPreRelease-PstChkUpgrade-High-45436-post check Upgrading a cluster by making sure not keep duplicate machine config when it has multiple kubeletconfig [Disruptive][Slow]", func() {
		upgradeMachineconfig1.name = "max-pod"
		defer func() {
			g.By("Delete the KubeletConfig")
			cleanupObjectsClusterScope(oc, objectTableRefcscope{"KubeletConfig", upgradeMachineconfig1.name})
			g.By("Check mcp finish rolling out")
			err := checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "mcp is not updated")
		}()

		upgradeMachineconfig2.name = "max-pod-1"
		defer func() {
			g.By("Delete the KubeletConfig")
			cleanupObjectsClusterScope(oc, objectTableRefcscope{"KubeletConfig", upgradeMachineconfig2.name})
			g.By("Check mcp finish rolling out")
			err := checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "mcp is not updated")
		}()
		g.By("Checking no duplicate machine config")
		checkUpgradeMachineConfig(oc)

	})

	//author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-PreChkUpgrade-Author:minmli-High-45351-prepare to check crioConfig[Disruptive][Slow]", func() {
		rhelWorkers, err := compat_otp.GetAllWorkerNodesByOSID(oc, "rhel")
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(rhelWorkers) > 0 {
			g.Skip("ctrcfg.overlay can't be supported by rhel nodes")
		}

		if compat_otp.IsSNOCluster(oc) || compat_otp.Is3MasterNoDedicatedWorkerNode(oc) {
			g.Skip("Skipped: Skip test for SNO/Compact clusters")
		}

		g.By("1) oc debug one worker and edit /etc/crio/crio.conf")
		// we update log_level = "debug" in /etc/crio/crio.conf
		nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		_, err = compat_otp.DebugNodeWithChroot(oc, nodename, "/bin/bash", "-c", "sed -i 's/log_level = \"info\"/log_level = \"debug\"/g' /etc/crio/crio.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("2) create a ContainerRuntimeConfig to set overlaySize")
		ctrcfgOverlay.name = "ctrcfg-45351"
		ctrcfgOverlay.overlay = "35G"
		mcpName := "worker"
		ctrcfgOverlay.create(oc)

		g.By("3) check mcp finish rolling out")
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "mcp update failed")

		g.By("4) check overlaySize update as expected")
		err = checkOverlaySize(oc, ctrcfgOverlay.overlay)
		compat_otp.AssertWaitPollNoErr(err, "overlaySize not update as expected")
	})

	//author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-PstChkUpgrade-Author:minmli-High-45351-post check crioConfig[Disruptive][Slow]", func() {
		rhelWorkers, err := compat_otp.GetAllWorkerNodesByOSID(oc, "rhel")
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(rhelWorkers) > 0 {
			g.Skip("ctrcfg.overlay can't be supported by rhel nodes")
		}

		if compat_otp.IsSNOCluster(oc) || compat_otp.Is3MasterNoDedicatedWorkerNode(oc) {
			g.Skip("Skipped: Skip test for SNO/Compact clusters")
		}

		g.By("1) check overlaySize don't change after upgrade")
		ctrcfgOverlay.name = "ctrcfg-45351"
		ctrcfgOverlay.overlay = "35G"

		defer func() {
			g.By("Delete the configRuntimeConfig")
			cleanupObjectsClusterScope(oc, objectTableRefcscope{"ContainerRuntimeConfig", ctrcfgOverlay.name})
			g.By("Check mcp finish rolling out")
			err := checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "mcp is not updated")
		}()

		defer func() {
			g.By("Restore /etc/crio/crio.conf")
			nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, node := range nodeList.Items {
				nodename := node.Name
				_, err = compat_otp.DebugNodeWithChroot(oc, nodename, "/bin/bash", "-c", "sed -i 's/log_level = \"debug\"/log_level = \"info\"/g' /etc/crio/crio.conf")
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}()

		err = checkOverlaySize(oc, ctrcfgOverlay.overlay)
		compat_otp.AssertWaitPollNoErr(err, "overlaySize change after upgrade")

		g.By("2) check conmon value from crio config")
		//we need check every node for the conmon = ""
		checkConmonForAllNode(oc)
	})

	g.It("Author:asahay-Medium-57332-collecting the audit log with must gather", func() {

		defer exec.Command("bash", "-c", "rm -rf /tmp/must-gather-57332").Output()
		g.By("Running the must gather command \n")
		_, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir=/tmp/must-gather-57332", "--", "/usr/bin/gather_audit_logs").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("check the must-gather result")
		_, err = exec.Command("bash", "-c", "ls -l /tmp/must-gather-57332").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

	})
	g.It("Author:asahay-NonHyperShiftHOST-Longduration-NonPreRelease-High-44820-change container registry config [Serial][Slow]", func() {
		ImgConfCont.name = "cluster"
		expectedStatus1 := map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}
		compat_otp.By("Verifying Config Changes in Image Registry")

		compat_otp.By("#. Copy and save existing CRD configuration in JSON format")
		originImageConfigJSON, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("image.config", "cluster", "-o", "json").Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		e2e.Logf("\n Original Image Configuration  %v", originImageConfigJSON)
		defer func() {
			compat_otp.By("restore original ImageConfig")
			createImageConfigWIthExportJSON(oc, originImageConfigJSON) // restore original yaml

			compat_otp.By("Check mcp finish updating")
			err := checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "Worker MCP is not updated")
			err = checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "Master MCP is not updated")

			compat_otp.By("Check the openshift-apiserver operator status")
			err = waitCoBecomes(oc, "openshift-apiserver", 480, expectedStatus1)
			compat_otp.AssertWaitPollNoErr(err, "openshift-apiserver operator does not become available in 480 seconds")

			compat_otp.By("Check the image-registry operator status")
			err = waitCoBecomes(oc, "image-registry", 480, expectedStatus1)
			compat_otp.AssertWaitPollNoErr(err, "image-registry operator does not become available in 480 seconds")
		}()

		checkImageConfigUpdatedAsExpected(oc)

	})

	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-57401-Create ImageDigestMirrorSet successfully [Disruptive][Slow]", func() {
		//If a cluster contains any ICSP or IDMS, it will skip the case
		if checkICSP(oc) || checkIDMS(oc) {
			g.Skip("This cluster contain ICSP or IDMS, skip the test.")
		}
		compat_otp.By("Create an ImageDigestMirrorSet")
		idms := filepath.Join(buildPruningBaseDir, "ImageDigestMirrorSet.yaml")
		defer func() {
			err := checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + idms).Execute()

		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + idms).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check the mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, "master")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
		err = checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check the ImageDigestMirrorSet apply to config")
		err = checkRegistryForIdms(oc)
		compat_otp.AssertWaitPollNoErr(err, "check registry config failed")

		compat_otp.By("The ImageContentSourcePolicy can't exist wiht ImageDigestMirrorSet or ImageTagMirrorSet")
		icsp := filepath.Join(buildPruningBaseDir, "ImageContentSourcePolicy.yaml")
		out, _ := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", icsp).Output()
		o.Expect(strings.Contains(out, "Kind.ImageContentSourcePolicy: Forbidden: can't create ImageContentSourcePolicy when ImageDigestMirrorSet resources exist")).To(o.BeTrue())
	})

	//author: minmli@redhat.com
	g.It("NonHyperShiftHOST-Author:minmli-Medium-59552-Enable image signature verification for Red Hat Container Registries [Serial]", func() {
		compat_otp.By("Check if mcp worker exist in current cluster")
		machineCount, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "worker", "-o=jsonpath={.status.machineCount}").Output()
		if machineCount == "0" {
			g.Skip("Skip for non-supported platform: mcp worker not exist!")
		}

		compat_otp.By("Apply a machine config to set image signature policy for worker nodes")
		mcImgSig := filepath.Join(buildPruningBaseDir, "machineconfig-image-signature-59552.yaml")
		mcpName := "worker"
		defer func() {
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + mcImgSig).Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + mcImgSig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check the mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check the signature configuration policy.json")
		err = checkImgSignature(oc)
		compat_otp.AssertWaitPollNoErr(err, "check signature configuration failed")
	})

	g.It("Author:asahay-NonHyperShiftHOST-NonPreRelease-Longduration-Medium-62746-A default SYSTEM_RESERVED_ES value is applied if it is empty [Disruptive][Slow]", func() {

		compat_otp.By("set SYSTEM_RESERVED_ES as empty")
		nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		_, err = compat_otp.DebugNodeWithChroot(oc, nodename, "/bin/bash", "-c", "sed -i 's/SYSTEM_RESERVED_ES=1Gi/SYSTEM_RESERVED_ES=/g' /etc/crio/crio.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		systemReserveES.name = "kubeletconfig-62746"
		systemReserveES.labelkey = "custom-kubelet"
		systemReserveES.labelvalue = "reserve-space"

		compat_otp.By("Label mcp worker custom-kubelet as reserve-space \n")
		addLabelToResource(oc, "custom-kubelet=reserve-space", "worker", "mcp")
		defer removeLabelFromNode(oc, "custom-kubelet-", "worker", "mcp")

		compat_otp.By("Create KubeletConfig \n")
		defer func() {
			mcpName := "worker"
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer systemReserveES.delete(oc)
		systemReserveES.create(oc)

		compat_otp.By("Check mcp finish rolling out")
		mcpName := "worker"
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check Default value")
		parameterCheck(oc)
	})

	//author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-65404-log link inside pod via crio works well [Disruptive]", func() {
		compat_otp.By("Apply a machine config to enable log link via crio")
		mcLogLink := filepath.Join(buildPruningBaseDir, "machineconfig-log-link.yaml")
		mcpName := "worker"
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + mcLogLink).Execute()
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + mcLogLink).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check the mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check the crio config as expected")
		logLinkConfig := []string{"crio.runtime.workloads.linked", "activation_annotation = \"io.kubernetes.cri-o.LinkLogs\"", "allowed_annotations = [ \"io.kubernetes.cri-o.LinkLogs\" ]"}
		configPath := "/etc/crio/crio.conf.d/99-linked-log.conf"
		err = configExist(oc, logLinkConfig, configPath)
		compat_otp.AssertWaitPollNoErr(err, "crio config is not set as expected")

		compat_otp.By("Create a pod with LinkLogs annotation")
		podLogLink65404.name = "httpd"
		podLogLink65404.namespace = oc.Namespace()
		err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", oc.Namespace(), "security.openshift.io/scc.podSecurityLabelSync=false", "pod-security.kubernetes.io/enforce=privileged", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer podLogLink65404.delete(oc)
		podLogLink65404.create(oc)

		compat_otp.By("Check pod status")
		err = podStatus(oc, podLogLink65404.namespace, podLogLink65404.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("Check log link successfully")
		checkLogLink(oc, podLogLink65404.namespace)
	})

	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-55683-Crun on OpenShift enable [Disruptive]", func() {
		compat_otp.By("Apply a ContarinerRuntimeConfig to enable crun")
		ctrcfgCrun := filepath.Join(buildPruningBaseDir, "containerRuntimeConfig-crun.yaml")
		mcpName := "worker"
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + ctrcfgCrun).Execute()
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + ctrcfgCrun).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check the mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check crun is running")
		checkCrun(oc)
	})

	g.It("Author:minmli-DEPRECATED-High-68184-container_network metrics should keep reporting after container restart", func() {
		livenessProbeTermP68184 := liveProbeTermPeriod{
			name:                  "liveness-probe",
			namespace:             oc.Namespace(),
			terminationgrace:      60,
			probeterminationgrace: 10,
			template:              livenessProbeTemp,
		}

		compat_otp.By("Create a pod")
		defer livenessProbeTermP68184.delete(oc)
		livenessProbeTermP68184.create(oc)

		compat_otp.By("Check pod status")
		err := podStatus(oc, livenessProbeTermP68184.namespace, livenessProbeTermP68184.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("Check the container_network* metrics report well")
		podNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", livenessProbeTermP68184.name, "-o=jsonpath={.spec.nodeName}", "-n", livenessProbeTermP68184.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("podNode is :%v", podNode)
		var cmdOut1 string
		var cmdOut2 string
		waitErr := wait.Poll(10*time.Second, 70*time.Second, func() (bool, error) {
			cmd1 := fmt.Sprintf(`oc get --raw /api/v1/nodes/%v/proxy/metrics/cadvisor  | grep container_network_transmit | grep %v || true`, podNode, livenessProbeTermP68184.name)
			cmdOut1, err := exec.Command("bash", "-c", cmd1).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(cmdOut1), "container_network_transmit_bytes_total") && strings.Contains(string(cmdOut1), "container_network_transmit_errors_total") && strings.Contains(string(cmdOut1), "container_network_transmit_packets_dropped_total") && strings.Contains(string(cmdOut1), "container_network_transmit_packets_total") {
				e2e.Logf("\ncontainer_network* metrics report well after pod start")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(waitErr, fmt.Sprintf("check metrics failed after pod start! Metric result is: \n %v \n", cmdOut1))

		compat_otp.By("Check the container_network* metrics still report after container restart")
		waitErr = wait.Poll(80*time.Second, 5*time.Minute, func() (bool, error) {
			restartCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", livenessProbeTermP68184.name, "-o=jsonpath={.status.containerStatuses[0].restartCount}", "-n", livenessProbeTermP68184.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("restartCount is :%v", restartCount)
			o.Expect(strconv.Atoi(restartCount)).Should(o.BeNumerically(">=", 1), "error: the pod restart time < 1")

			cmd2 := fmt.Sprintf(`oc get --raw /api/v1/nodes/%v/proxy/metrics/cadvisor  | grep container_network_transmit | grep %v || true`, podNode, livenessProbeTermP68184.name)
			cmdOut2, err := exec.Command("bash", "-c", cmd2).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(cmdOut2), "container_network_transmit_bytes_total") && strings.Contains(string(cmdOut2), "container_network_transmit_errors_total") && strings.Contains(string(cmdOut2), "container_network_transmit_packets_dropped_total") && strings.Contains(string(cmdOut2), "container_network_transmit_packets_total") {
				e2e.Logf("\ncontainer_network* metrics report well after pod restart")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(waitErr, fmt.Sprintf("check metrics failed after pod restart! Metric result is: \n %v \n", cmdOut2))
	})

	//author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-Medium-66398-Enable WASM workloads in OCP", func() {
		podWASM66398 := podWASM{
			name:      "wasm-http",
			namespace: oc.Namespace(),
			template:  podWASMTemp,
		}

		compat_otp.By("Apply a machineconfig to configure crun-wasm as the default runtime")
		mcWASM := filepath.Join(buildPruningBaseDir, "machineconfig-wasm.yaml")
		mcpName := "worker"
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + mcWASM).Execute()
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + mcWASM).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check the machine config pool finish updating")
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Verify the crun-wasm is configured as expected")
		wasmConfig := []string{"crio.runtime", "default_runtime = \"crun-wasm\"", "crio.runtime.runtimes.crun-wasm", "runtime_path = \"/usr/bin/crun\"", "crio.runtime.runtimes.crun-wasm.platform_runtime_paths", "\"wasi/wasm32\" = \"/usr/bin/crun-wasm\""}
		configPath := "/etc/crio/crio.conf.d/99-crun-wasm.conf"
		err = configExist(oc, wasmConfig, configPath)
		compat_otp.AssertWaitPollNoErr(err, "crun-wasm is not set as expected")

		compat_otp.By("Check if wasm bits are enabled appropriately")
		compat_otp.By("1)label namespace pod-security.kubernetes.io/enforce=baseline")
		addLabelToResource(oc, "pod-security.kubernetes.io/enforce=baseline", oc.Namespace(), "namespace")
		compat_otp.By("2)Create a pod")
		defer podWASM66398.delete(oc)
		podWASM66398.create(oc)

		compat_otp.By("3)Check pod status")
		err = podStatus(oc, podWASM66398.namespace, podWASM66398.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("4)Expose the pod as a service")
		_, err = oc.AsAdmin().WithoutNamespace().Run("expose").Args("pod", podWASM66398.name, "-n", podWASM66398.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5)Expose the service as a route")
		_, err = oc.AsAdmin().WithoutNamespace().Run("expose").Args("service", podWASM66398.name, "-n", podWASM66398.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("6)Get the route name")
		routeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("route", podWASM66398.name, "-ojsonpath={.spec.host}", "-n", podWASM66398.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("7)Curl the route name")
		out, err := exec.Command("bash", "-c", "curl "+routeName+" -d \"Hello world!\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(string(out), "echo: Hello world!")).Should(o.BeTrue())
	})

	//author: jfrancoa@redhat.com
	//automates: https://issues.redhat.com/browse/OCPBUGS-15035
	g.It("NonHyperShiftHOST-NonPreRelease-Author:jfrancoa-Medium-67564-node's drain should block when PodDisruptionBudget minAvailable equals 100 percentage and selector is empty [Disruptive]", func() {
		compat_otp.By("Create a deployment with 6 replicas")
		deploy := NewDeployment("hello-openshift", oc.Namespace(), "6", genericDeploymentTemp)
		defer deploy.delete(oc)
		deploy.create(oc)
		deploy.waitForCreation(oc, 5)

		compat_otp.By("Create PodDisruptionBudget")
		pdb := NewPDB("my-pdb", oc.Namespace(), "100%", podDisruptionBudgetTemp)
		defer pdb.delete(oc)
		pdb.create(oc)

		worker := getSingleWorkerNode(oc)
		compat_otp.By(fmt.Sprintf("Obtain the pods running on node %v", worker))

		podsInWorker, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("pods", "-n", oc.Namespace(), "-o=jsonpath={.items[?(@.spec.nodeName=='"+worker+"')].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(strings.Split(podsInWorker, " "))).Should(o.BeNumerically(">", 0))

		// if the pdb's status is false and reason InsufficientPods
		// means that it's not possible to drain a node keeping the
		// required minimum availability, therefore the drain operation
		// should block.
		compat_otp.By("Make sure that PDB's status is False")
		pdbStatus, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("poddisruptionbudget", "my-pdb", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[0].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(pdbStatus, "False")).Should(o.BeTrue())

		compat_otp.By(fmt.Sprintf("Drain the node %v", worker))
		defer waitClusterOperatorAvailable(oc)
		defer oc.WithoutNamespace().AsAdmin().Run("adm").Args("uncordon", worker).Execute()
		// Try to drain the node (it should fail) due to the 100%'s PDB minAvailability
		// as the draining is impossible to happen, if we don't pass a timeout value this
		// command will wait forever, as default timeout is 0s, which means infinite.
		out, err := oc.WithoutNamespace().AsAdmin().Run("adm").Args("drain", worker, "--ignore-daemonsets", "--delete-emptydir-data", "--timeout=30s").Output()
		o.Expect(err).To(o.HaveOccurred(), "Drain operation should have been blocked but it wasn't")
		o.Expect(strings.Contains(out, "Cannot evict pod as it would violate the pod's disruption budget")).Should(o.BeTrue())
		o.Expect(strings.Contains(out, "There are pending nodes to be drained")).Should(o.BeTrue())

		compat_otp.By("Verify that the pods were not drained from the node")
		podsAfterDrain, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("pods", "-n", oc.Namespace(), "-o=jsonpath={.items[?(@.spec.nodeName=='"+worker+"')].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podsInWorker).Should(o.BeIdenticalTo(podsAfterDrain))
	})

	//author: minmli@redhat.com
	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-70203-ICSP and IDMS/ITMS can coexist in cluster[Disruptive][Slow]", func() {
		compat_otp.By("Check if any ICSP/IDMS/ITMS exist in the cluster")
		//If a cluster contains any ICSP or IDMS or ITMS, it will skip the case
		if checkICSPorIDMSorITMS(oc) {
			g.Skip("This cluster contain ICSP or IDMS or ITMS, skip the test.")
		}

		compat_otp.By("1)Create an ICSP")
		icsp := filepath.Join(buildPruningBaseDir, "ImageContentSourcePolicy-1.yaml")
		defer func() {
			err := checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + icsp).Execute()

		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + icsp).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("2)Check the mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, "master")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
		err = checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("3)Check the config file /etc/containers/registries.conf update as expected")
		registryConfig := []string{"location = \"registry.access.redhat.com/ubi8/ubi-minimal\"", "location = \"example.io/example/ubi-minimal\"", "location = \"example.com/example/ubi-minimal\"", "location = \"registry.example.com/example\"", "location = \"mirror.example.net\""}
		configPath := "/etc/containers/registries.conf"
		err = configExist(oc, registryConfig, configPath)
		compat_otp.AssertWaitPollNoErr(err, "registry config is not set as expected")

		/*
			//After OCPBUGS-27190 is fixed, will uncomment the code block
			compat_otp.By("4)Create an IDMS with the same registry/mirror config as ICSP but with conflicting policy")
			idms := filepath.Join(buildPruningBaseDir, "ImageDigestMirrorSet-conflict.yaml")
			out, _ := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", idms).Output()
			o.Expect(strings.Contains(out, "XXXXX")).To(o.BeTrue())
		*/

		compat_otp.By("5)Create an IDMS with the same registry/mirror config as ICSP")
		idms1 := filepath.Join(buildPruningBaseDir, "ImageDigestMirrorSet-1.yaml")
		defer func() {
			err := checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + idms1).Execute()

		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + idms1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("6)Check the mcp doesn't get updated after idms created")
		o.Consistently(func() bool {
			workerUpdated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "worker", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updated\")].status}").Output()
			workerUpdating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "worker", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updating\")].status}").Output()
			masterUpdated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "master", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updated\")].status}").Output()
			masterUpdating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "master", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updating\")].status}").Output()
			return workerUpdated == "True" && workerUpdating == "False" && masterUpdated == "True" && masterUpdating == "False"
		}).WithTimeout(60 * time.Second).WithPolling(5 * time.Second).Should(o.BeTrue())

		compat_otp.By("7)Delete the ICSP")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + icsp).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("8)Check the mcp doesn't get updated after icsp deleted")
		o.Consistently(func() bool {
			workerUpdated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "worker", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updated\")].status}").Output()
			workerUpdating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "worker", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updating\")].status}").Output()
			masterUpdated, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "master", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updated\")].status}").Output()
			masterUpdating, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", "master", "-n", oc.Namespace(), "-o=jsonpath={.status.conditions[?(@.type==\"Updating\")].status}").Output()
			return workerUpdated == "True" && workerUpdating == "False" && masterUpdated == "True" && masterUpdating == "False"
		}).WithTimeout(60 * time.Second).WithPolling(5 * time.Second).Should(o.BeTrue())

		compat_otp.By("9)Check the config file /etc/containers/registries.conf keep the same")
		registryConfig = []string{"location = \"registry.access.redhat.com/ubi8/ubi-minimal\"", "location = \"example.io/example/ubi-minimal\"", "location = \"example.com/example/ubi-minimal\"", "location = \"registry.example.com/example\"", "location = \"mirror.example.net\""}
		configPath = "/etc/containers/registries.conf"
		err = configExist(oc, registryConfig, configPath)
		compat_otp.AssertWaitPollNoErr(err, "registry config is not set as expected")

		compat_otp.By("10)Create an ITMS with different registry/mirror config from IDMS")
		itms := filepath.Join(buildPruningBaseDir, "ImageTagMirrorSet.yaml")
		defer func() {
			err := checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + itms).Execute()

		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + itms).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("11)Check mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, "master")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
		err = checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("12)Check the config file /etc/containers/registries.conf update as expected")
		registryConfig = []string{"location = \"registry.access.redhat.com/ubi9/ubi-minimal\"", "location = \"registry.redhat.io\"", "location = \"mirror.example.com\""}
		configPath = "/etc/containers/registries.conf"
		err = configExist(oc, registryConfig, configPath)
		compat_otp.AssertWaitPollNoErr(err, "registry config is not set as expected")
	})

	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-41897-Restricting CPUs for infra and application containers[Disruptive][Slow]", func() {
		compat_otp.By("Check cpu core num on the node")
		workerNodes := getWorkersList(oc)
		cpuNum := getCPUNum(oc, workerNodes[0])
		//This case can only run on a node with more than 4 cpu cores
		if cpuNum <= 4 {
			g.Skip("This cluster has less than 4 cpu cores, skip the test.")
		}
		compat_otp.By("Test for case OCP-41897")
		cpuPerformanceprofile := filepath.Join(buildPruningBaseDir, "cpu-performanceprofile.yaml")
		perfProfile41897 := cpuPerfProfile{
			name:     "performance-41897",
			isolated: "",
			template: cpuPerformanceprofile,
		}
		isolatedCPU := "0,5-" + strconv.Itoa(cpuNum-1)
		perfProfile41897.isolated = isolatedCPU

		compat_otp.By("1)Create a performanceProfile")
		//when delete the performanceprofile, only mcp worker will update
		defer func() {
			err := checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		defer perfProfile41897.delete(oc)
		perfProfile41897.create(oc)

		//for 4.14+, master and worker pool need update to change cgroup from v2 to v1, then worker pool update to apply performanceprofile
		compat_otp.By("2)Check the mcp finish updating")
		//if cgroup is v2, then mcp master and worker need update to change to v1 first
		cgroupV := getCgroupVersion(oc)
		if cgroupV == "cgroup2fs" {
			err := checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}

		// the kubelet get generated when the mcp worker update to apply performanceprofile
		compat_otp.By("3)Check the kubeletconfig get generated")
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeletconfig", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(output, perfProfile41897.name)).Should(o.BeTrue())
		e2e.Logf("kubeletconfig exist: [%v], then check the mcp worker finish updating\n", output)
		err = checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("4)Check the reserved cpu are as expected")
		// 1) "reservedSystemCPUs": "1-4" from /etc/kubernetes/kubelet.conf
		// 2) sh-5.1# pgrep systemd |while read i; do taskset -cp $i; done  || results: pid 1's current affinity list: 1-4
		//isolatedCPU := "0,5-" + strconv.Itoa(cpuNum-1)
		reservedCPU := "1-4"
		checkReservedCPU(oc, reservedCPU)
	})

	g.It("NonHyperShiftHOST-NonPreRelease-Longduration-Author:minmli-High-62985-Support disable cpu load balancing and cpu quota on RHEL 9 [Disruptive][Slow]", func() {
		// in 4.16, it support cgroupv2; in 4.15-, it only support cgroupv1
		compat_otp.By("Check cpu core num on the node")
		workerNodes := getWorkersList(oc)
		cpuNum := getCPUNum(oc, workerNodes[0])
		//This case can only run on a node with more than 4 cpu cores
		if cpuNum <= 4 {
			g.Skip("This cluster has less than 4 cpu cores, skip the test.")
		}

		cpuPerformanceprofile := filepath.Join(buildPruningBaseDir, "cpu-performanceprofile.yaml")
		perfProfile62985 := cpuPerfProfile{
			name:     "performance-62985",
			isolated: "",
			template: cpuPerformanceprofile,
		}
		isolatedCPU := "0,5-" + strconv.Itoa(cpuNum-1)
		perfProfile62985.isolated = isolatedCPU

		compat_otp.By("1)Create a performanceProfile")
		defer func() {
			perfProfile62985.delete(oc)
			err := checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		perfProfile62985.create(oc)

		err := checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("2)Check the reserved cpu are as expected")
		// 1) "reservedSystemCPUs": "1-4" from /etc/kubernetes/kubelet.conf
		// 2) sh-5.1# pgrep systemd |while read i; do taskset -cp $i; done  || results: pid 1's current affinity list: 1-4
		reservedCPU := "1-4"
		checkReservedCPU(oc, reservedCPU)

		compat_otp.By("3)Turn on cpu info in dmesg log")
		defer dmesgTurnOnCPU(oc, "1")
		dmesgTurnOnCPU(oc, "0")

		compat_otp.By("4)Create a pod with Guaranteed QoS, using at least a full CPU and load balance/cpu-quota disable annotation")
		podCPULoadBalance62985 := podCPULoadBalance{
			name:         "cpu-load-balce-62985",
			namespace:    oc.Namespace(),
			runtimeclass: "performance-performance-62985", //"performance-" + perfProfile62985.name
			template:     podCPULoadBalanceTemp,
		}
		defer podCPULoadBalance62985.delete(oc)
		podCPULoadBalance62985.create(oc)

		compat_otp.By("5)Check pod Status")
		err = podStatus(oc, podCPULoadBalance62985.namespace, podCPULoadBalance62985.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("6)Check the cpus are properly having load balance disabled")
		checkCPULoadBalanceDisabled(oc, podCPULoadBalance62985.namespace, podCPULoadBalance62985.name)

		compat_otp.By("7)Check cpu-quota is disabled from container scope and pod cgroup correctly")
		cgroupV := getCgroupVersion(oc)
		checkCPUQuotaDisabled(oc, podCPULoadBalance62985.namespace, podCPULoadBalance62985.name, cgroupV)
	})

	//author: minmli@redhat.com
	g.It("Author:minmli-NonHyperShiftHOST-NonPreRelease-Longduration-High-73667-High-73412-Crio verify the sigstore signature using default policy when pulling images [Disruptive][Slow]", func() {
		compat_otp.By("1)Enable featureGate of TechPreviewNoUpgrade")

		compat_otp.By("Check if exist any featureSet in featuregate cluster")
		featureSet, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("featuregate", "cluster", "-o=jsonpath={.spec.featureSet}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("featureSet is: %s", featureSet)

		if featureSet == "TechPreviewNoUpgrade" {
			e2e.Logf("featureSet is TechPreviewNoUpgrade already, no need setting again!")
			/*
				//comment the part of [featureSet == ""] to abserve the execution of tp profile in CI
				} else if featureSet == "" {
					_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("featuregate", "cluster", "-p", "{\"spec\": {\"featureSet\": \"TechPreviewNoUpgrade\"}}", "--type=merge").Output()
					if err != nil {
						e2e.Failf("Fail to enable TechPreviewNoUpgrade, error:%v", err)
					}

					compat_otp.By("check mcp master and worker finish updating")
					err = checkMachineConfigPoolStatus(oc, "master")
					compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
					err = checkMachineConfigPoolStatus(oc, "worker")
					compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
				} else {
					g.Skip("featureSet is neither empty nor TechPreviewNoUpgrade,skip it!")
				}
			*/
		} else {
			g.Skip("featureSet is not TechPreviewNoUpgrade,skip it!")
		}

		compat_otp.By("2)Check the featureGate take effect")
		//featureConfig := []string{"SignatureStores: true", "SigstoreImageVerification: true"} //4.17 be so
		featureConfig := []string{"\"SignatureStores\": true", "\"SigstoreImageVerification\": true"} //4.16 be so
		kubeletPath := "/etc/kubernetes/kubelet.conf"
		err = configExist(oc, featureConfig, kubeletPath)
		compat_otp.AssertWaitPollNoErr(err, "featureGate config check failed")

		compat_otp.By("3)Set the crio loglevel [debug]")
		ctrcfgLog := filepath.Join(buildPruningBaseDir, "containerRuntimeConfig_log_level.yaml")
		mcpName := "worker"
		defer func() {
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + ctrcfgLog).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + ctrcfgLog).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("Check the crio loglevel")
		nodeName := getSingleWorkerNode(oc)
		out, _ := compat_otp.DebugNodeWithChroot(oc, nodeName, "/bin/bash", "-c", "crio config | grep log_level")
		o.Expect(strings.Contains(string(out), "log_level = \"debug\"")).Should(o.BeTrue())

		compat_otp.By("4)Apply the ClusterImagePolicy manifest")
		imgPolicy := filepath.Join(buildPruningBaseDir, "imagePolicy.yaml")
		defer func() {
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + imgPolicy).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = checkMachineConfigPoolStatus(oc, "master")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + imgPolicy).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("Check mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, "master")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool master update failed")
		err = checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")

		compat_otp.By("5)Create a pod with an image containing sigstore signature")
		podSigstore73667.name = "pod-73667-sig"
		podSigstore73667.namespace = oc.Namespace()

		defer podSigstore73667.delete(oc)
		podSigstore73667.create(oc)

		compat_otp.By("6)Check the pod status")
		err = podStatus(oc, podSigstore73667.namespace, podSigstore73667.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("7)check the crio log about sigstore signature verification")
		dockerNs := "docker.io"
		image := "docker.io/lyman9966/rhel8"
		checkSigstoreVerified(oc, podSigstore73667.namespace, podSigstore73667.name, image, dockerNs)

		compat_otp.By("8)validate pulling an image not containing sigstore signature will fail")
		nodeName = getSingleWorkerNode(oc)
		out, _ = compat_otp.DebugNodeWithChroot(oc, nodeName, "/bin/bash", "-c", "crictl pull docker.io/ocpqe/hello-pod:latest")
		o.Expect(strings.Contains(string(out), "Source image rejected: A signature was required, but no signature exists")).Should(o.BeTrue())
	})

	//author: minmli@redhat.com
	g.It("Author:minmli-NonHyperShiftHOST-NonPreRelease-Longduration-Critical-72080-Verify cpu affinity of container process matches with cpuset cgroup controller interface file cpuset.cpus [Disruptive][Slow]", func() {
		//this case verify 3 scenarios:
		//1)Verify burstable pods affinity contains all online cpus
		//2)when guaranteed pods are created (with integral cpus) , the affinity of burstable pods are modified accordingly to remove any cpus that was used by guaranteed pod
		//3)After node reboot, burstable pods affinity should contain all cpus excluding the cpus used by guranteed pods
		compat_otp.By("1)Label a specific worker node")
		workerNodes := getWorkersList(oc)
		var worker string
		for i := 0; i < len(workerNodes); i++ {
			readyStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", workerNodes[i], "-o=jsonpath={.status.conditions[?(@.reason=='KubeletReady')].status}").Output()
			if readyStatus == "True" {
				worker = workerNodes[i]
				break
			}
		}

		defer oc.AsAdmin().WithoutNamespace().Run("label").Args("nodes", worker, "node-role.kubernetes.io/worker-affinity-tests-").Output()
		_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args("nodes", worker, "node-role.kubernetes.io/worker-affinity-tests=", "--overwrite").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("2)Create a machine config pool for the specific worker")
		mcpAffinity := filepath.Join(buildPruningBaseDir, "machineconfigpool-affinity.yaml")
		defer func() {
			err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + mcpAffinity).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = checkMachineConfigPoolStatus(oc, "worker")
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + mcpAffinity).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("2.1)Check the mcp finish updating")
		mcpName := "worker-affinity-tests"
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("macineconfigpool %v update failed!", mcpName))
		//the mcp worker also need updating after mcp worker-affinity-tests finish updating
		err = checkMachineConfigPoolStatus(oc, "worker")
		compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("macineconfigpool worker update failed!"))

		compat_otp.By("3)Create a kubeletconfig to enable cpumanager")
		kubeconfigCpumager := filepath.Join(buildPruningBaseDir, "kubeletconfig-cpumanager.yaml")
		defer func() {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + kubeconfigCpumager).Execute()
			err := checkMachineConfigPoolStatus(oc, mcpName)
			compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("macineconfigpool %v update failed!", mcpName))
		}()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + kubeconfigCpumager).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3.1)Check the mcp finish updating")
		err = checkMachineConfigPoolStatus(oc, mcpName)
		compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("macineconfigpool %v update failed!", mcpName))

		compat_otp.By("4)Check one running burstable pod that its cpu affinity include all online cpus")
		//select one pod of ns openshift-cluster-node-tuning-operator which is running on the $worker node
		burstPodName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-cluster-node-tuning-operator", "--field-selector=spec.nodeName="+worker, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		coreNum := getCPUNum(oc, worker)
		burstNs := "openshift-cluster-node-tuning-operator"
		checkCPUAffinityBurst(oc, burstPodName, burstNs, worker, coreNum, "false")

		compat_otp.By("5)Create a guranteed pod with integral cpus")
		podGuTemp := filepath.Join(buildPruningBaseDir, "pod-guaranteed.yaml")
		podGu72080 := podGuDescription{
			name:      "gurantee-72080",
			namespace: oc.Namespace(),
			nodename:  worker,
			template:  podGuTemp,
		}
		defer podGu72080.delete(oc)
		podGu72080.create(oc)

		compat_otp.By("5.1)Check the pod status")
		err = podStatus(oc, podGu72080.namespace, podGu72080.name)
		compat_otp.AssertWaitPollNoErr(err, "pod is not running")

		compat_otp.By("5.2)Get cpu affinity of the guranteed pod")
		guAffinity := getCPUAffinityFromPod(oc, podGu72080.namespace, podGu72080.name)

		compat_otp.By("6)Check the cpu affinity of burstable pod changed after creating the guranteed pod")
		checkCPUAffinityBurst(oc, burstPodName, burstNs, worker, coreNum, guAffinity)

		compat_otp.By("7)Delete the guranteed pod")
		podGu72080.delete(oc)

		compat_otp.By("8)Check the cpu affinity of burstable pod revert after deleting the guranteed pod")
		// there exist a bug currently, when deleting the pod, the cpu affinity of burstable pod can't revert in a short time
		//checkCPUAffinityBurst(oc, burstPodName, burstNs, worker, coreNum, "false")

		compat_otp.By("9)Create a deployment with guranteed pod with integral cpus")
		deployGuTemp := filepath.Join(buildPruningBaseDir, "guaranteed-deployment.yaml")
		deploy := NewDeploymentWithNode("guarantee-72080", oc.Namespace(), "1", worker, deployGuTemp)
		defer deploy.delete(oc)
		deploy.create(oc)
		deploy.waitForCreation(oc, 5)

		compat_otp.By("9.1)Get cpu affinity of the guranteed pod owned by the deployment")
		guPodName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace(), "--field-selector", "spec.nodeName="+worker, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		guAffinity = getCPUAffinityFromPod(oc, oc.Namespace(), guPodName)

		compat_otp.By("10)Check the cpu affinity of burstable pod changed after creating the deployment")
		checkCPUAffinityBurst(oc, burstPodName, burstNs, worker, coreNum, guAffinity)

		compat_otp.By("11)Reboot the node")
		defer checkNodeStatus(oc, worker, "Ready")
		rebootNode(oc, worker)
		checkNodeStatus(oc, worker, "NotReady")
		checkNodeStatus(oc, worker, "Ready")

		compat_otp.By("12)Check the cpu affinity of burstable pod contain all cpus excluding the cpus used by guranteed pods")
		deploy.waitForCreation(oc, 5)
		guPodName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", oc.Namespace(), "--field-selector", "spec.nodeName="+worker, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		guAffinity = getCPUAffinityFromPod(oc, oc.Namespace(), guPodName)
		burstPodName, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", "openshift-cluster-node-tuning-operator", "--field-selector=spec.nodeName="+worker, "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		checkCPUAffinityBurst(oc, burstPodName, burstNs, worker, coreNum, guAffinity)
	})

	//author: asahay@redhat.com

	g.It("Author:asahay-High-78394-Make CRUN as Default Runtime for 4.18", func() {

		compat_otp.By("1) Check Cluster Version")
		clusterVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Cluster version: %s\n", clusterVersion)

		var expectedRuntime string
		if strings.Contains(clusterVersion, "4.18") {
			expectedRuntime = "crun"
		} else {
			expectedRuntime = "runc"
		}

		compat_otp.By("2) Check all Nodes are Up and Default Runtime is crun")
		defaultRuntimeCheck(oc, expectedRuntime)

	})

	//author: bgudi@redhat.com
	g.It("Author:bgudi-Medium-80983-Changing cgroup from v2 to v1 result in error from 4.19 versions", func() {
		compat_otp.By("1)Check cgroup version")
		cgroupV := getCgroupVersion(oc)
		o.Expect(strings.Contains(cgroupV, "cgroup2fs")).Should(o.BeTrue())

		compat_otp.By("2)Changing cgroup from v2 to v1 should result in error")
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("nodes.config.openshift.io", "cluster",
			"-p", "{\"spec\": {\"cgroupMode\": \"v1\"}}", "--type=merge").Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(strings.Contains(output, "The Node \"cluster\" is invalid: spec.cgroupMode: Unsupported value: \"v1\": supported values: \"v2\", \"\"")).Should(o.BeTrue())
	})

	g.It("Author:asahay-NonPreRelease-Longduration-High-78610-Default Runtime can be Updated to runc in 4.18[Serial]", func() {

		compat_otp.By("1) Check Cluster Version")
		clusterVersion, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterversion").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Cluster version: %s\n", clusterVersion)

		compat_otp.By("2.1) Apply ContainerRuntimeConfig install manifest on Worker node to request defaultRuntime to runc ")
		ContainerRuntimeConfigTemp1 := filepath.Join(buildPruningBaseDir, "ContainerRuntimeConfigWorker-78610.yaml")
		defer func() {
			err := oc.AsAdmin().Run("delete").Args("-f=" + ContainerRuntimeConfigTemp1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			mcpname1 := "worker"
			err = checkMachineConfigPoolStatus(oc, mcpname1)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err1 := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f=" + ContainerRuntimeConfigTemp1).Execute()
		o.Expect(err1).NotTo(o.HaveOccurred())

		compat_otp.By("2.2) Apply ContainerRuntimeConfig install manifest on Master node to request defaultRuntime to runc ")
		ContainerRuntimeConfigTemp2 := filepath.Join(buildPruningBaseDir, "ContainerRuntimeConfigMaster-78610.yaml")
		defer func() {
			err := oc.AsAdmin().Run("delete").Args("-f=" + ContainerRuntimeConfigTemp2).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			mcpname2 := "master"
			err = checkMachineConfigPoolStatus(oc, mcpname2)
			compat_otp.AssertWaitPollNoErr(err, "macineconfigpool worker update failed")
		}()
		err2 := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f=" + ContainerRuntimeConfigTemp2).Execute()
		o.Expect(err2).NotTo(o.HaveOccurred())

		compat_otp.By("3) Wait for MCP to Finish Update")
		compat_otp.By("Check mcp finish rolling out")
		oc.NotShowInfo()
		mcpName1 := "worker"
		mcpName2 := "master"
		err3 := checkMachineConfigPoolStatus(oc, mcpName1)
		compat_otp.AssertWaitPollNoErr(err3, "macineconfigpool worker update failed")
		err4 := checkMachineConfigPoolStatus(oc, mcpName2)
		compat_otp.AssertWaitPollNoErr(err4, "macineconfigpool master update failed")

		//for checking machine config pool

		mcp, err5 := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp").Output()
		o.Expect(err5).NotTo(o.HaveOccurred())
		e2e.Logf("\n Machine config pools are:\n %s", mcp)

		compat_otp.By("4) Check the Default Runtime Value")
		UpdatedRuntimeCheck(oc, "runc")

	})

})

var _ = g.Describe("[sig-node][NodeQE] NODE keda", func() {

	defer g.GinkgoRecover()
	var (
		oc                        = compat_otp.NewCLI("keda-operator", compat_otp.KubeConfigPath())
		cmaKedaControllerTemplate string
		buildPruningBaseDir       = compat_otp.FixturePath("testdata", "node")
		sub                       subscriptionDescription
	)
	g.BeforeEach(func() {
		// skip ARM64 arch
		architecture.SkipNonAmd64SingleArch(oc)
		buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
		cmaKedaControllerTemplate = filepath.Join(buildPruningBaseDir, "cma-keda-controller-template.yaml")
		sub.skipMissingCatalogsources(oc)
		createKedaOperator(oc)
	})
	// author: weinliu@redhat.com
	g.It("Author:weinliu-LEVEL0-High-52383-Keda Install", func() {
		g.By("CMA (Keda) operator has been installed successfully")
	})

	// author: weinliu@redhat.com
	g.It("Author:weinliu-High-62570-Verify must-gather tool works with CMA", func() {
		var (
			mustgatherName = "mustgather" + getRandomString()
			mustgatherDir  = "/tmp/" + mustgatherName
			mustgatherLog  = mustgatherName + ".log"
			logFile        string
		)
		g.By("Get the mustGatherImage")
		mustGatherImage, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("packagemanifest", "-n=openshift-marketplace", "openshift-custom-metrics-autoscaler-operator", "-o=jsonpath={.status.channels[?(.name=='stable')].currentCSVDesc.annotations.containerImage}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By("Running the must gather command \n")
		defer os.RemoveAll(mustgatherDir)
		logFile, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir="+mustgatherDir, "--image="+mustGatherImage).Output()
		if err != nil {
			e2e.Logf("mustgather created from image %v in %v logged to %v,%v %v", mustGatherImage, mustgatherDir, mustgatherLog, logFile, err)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
	// author: weinliu@redhat.com
	g.It("Author:weinliu-High-60961-Audit logging test - stdout Metadata[Serial]", func() {
		g.By("Create KedaController with log level Metadata")
		g.By("Create CMA Keda Controller ")
		cmaKedaController := cmaKedaControllerDescription{
			level:     "Metadata",
			template:  cmaKedaControllerTemplate,
			name:      "keda",
			namespace: "openshift-keda",
		}
		defer cmaKedaController.delete(oc)
		cmaKedaController.create(oc)
		metricsApiserverPodName := getPodNameByLabel(oc, "openshift-keda", "app=keda-metrics-apiserver")
		waitPodReady(oc, "openshift-keda", "app=keda-metrics-apiserver")
		g.By("Check the Audit Logged as configed")
		log, err := compat_otp.GetSpecificPodLogs(oc, "openshift-keda", "", metricsApiserverPodName[0], "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(log, "\"level\":\"Metadata\"")).Should(o.BeTrue())
	})

	g.It("Author:asahay-High-60962-Audit logging test - stdout Request[Serial]", func() {
		g.By("Create KedaController with log level Request")
		g.By("Create CMA Keda Controller ")
		cmaKedaController := cmaKedaControllerDescription{
			level:     "Request",
			template:  cmaKedaControllerTemplate,
			name:      "keda",
			namespace: "openshift-keda",
		}
		defer cmaKedaController.delete(oc)
		cmaKedaController.create(oc)
		metricsApiserverPodName := getPodNameByLabel(oc, "openshift-keda", "app=keda-metrics-apiserver")
		waitPodReady(oc, "openshift-keda", "app=keda-metrics-apiserver")
		g.By("Check the Audit Logged as configed")
		log, err := compat_otp.GetSpecificPodLogs(oc, "openshift-keda", "", metricsApiserverPodName[0], "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(log, "\"level\":\"Request\"")).Should(o.BeTrue())
	})

	g.It("Author:asahay-High-60963-Audit logging test - stdout RequestResponse[Serial]", func() {
		g.By("Create KedaController with log level RequestResponse")
		g.By("Create CMA Keda Controller ")
		cmaKedaController := cmaKedaControllerDescription{
			level:     "RequestResponse",
			template:  cmaKedaControllerTemplate,
			name:      "keda",
			namespace: "openshift-keda",
		}
		defer cmaKedaController.delete(oc)
		cmaKedaController.create(oc)
		metricsApiserverPodName := getPodNameByLabel(oc, "openshift-keda", "app=keda-metrics-apiserver")
		waitPodReady(oc, "openshift-keda", "app=keda-metrics-apiserver")
		g.By("Check the Audit Logged as configed")
		log, err := compat_otp.GetSpecificPodLogs(oc, "openshift-keda", "", metricsApiserverPodName[0], "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(log, "\"level\":\"RequestResponse\"")).Should(o.BeTrue())
	})
	//Author: asahay@redhat.com
	g.It("Author:asahay-High-60964-Audit logging test - Writing to PVC [Serial]", func() {

		compat_otp.By("1) Create a PVC")
		pvc := filepath.Join(buildPruningBaseDir, "pvc-60964.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+pvc, "-n", "openshift-keda").Execute()
		err := oc.AsAdmin().Run("create").Args("-f="+pvc, "-n", "openshift-keda").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("2) Create KedaController with log level Metdata")
		compat_otp.By("Create CMA Keda Controller ")
		pvcKedaControllerTemp := filepath.Join(buildPruningBaseDir, "pvcKedaControllerTemp-60964.yaml")
		pvcKedaController := pvcKedaControllerDescription{
			level:          "Metadata",
			template:       pvcKedaControllerTemp,
			name:           "keda",
			namespace:      "openshift-keda",
			watchNamespace: "openshift-keda",
		}

		defer pvcKedaController.delete(oc)
		pvcKedaController.create(oc)
		metricsApiserverPodName := getPodNameByLabel(oc, "openshift-keda", "app=keda-metrics-apiserver")
		waitPodReady(oc, "openshift-keda", "app=keda-metrics-apiserver")

		var output string

		compat_otp.By("3) Checking PVC creation")
		output, err = oc.AsAdmin().Run("get").Args("pvc", "-n", "openshift-keda").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("PVC is %v", output)

		compat_otp.By("4) Checking KEDA Controller")
		errCheck := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("KedaController", "-n", "openshift-keda").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(output, "keda") {
				e2e.Logf("Keda Controller has been created Successfully!")
				return true, nil
			}
			return false, nil
		})
		e2e.Logf("Output is %s", output)
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("KedaController has not been created"))

		compat_otp.By("5) Checking status of pods")
		waitPodReady(oc, "openshift-keda", "app=keda-metrics-apiserver")

		compat_otp.By("6) Verifying audit logs for 'Metadata'")
		errCheck = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
			auditOutput := ExecCommandOnPod(oc, metricsApiserverPodName[0], "openshift-keda", "tail $(ls -t /var/audit-policy/log*/log-out-pvc | head -1)")
			if strings.Contains(auditOutput, "Metadata") {
				e2e.Logf("Audit log contains 'Metadata ")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("Audit Log does not contain Metadata"))

	})

	// author: weinliu@redhat.com
	g.It("Author:weinliu-Critical-52384-Automatically scaling pods based on Kafka Metrics[Serial][Slow]", func() {
		var (
			scaledObjectStatus string
		)
		compat_otp.By("Create a kedacontroller with default template")
		kedaControllerDefault := filepath.Join(buildPruningBaseDir, "keda-controller-default.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-keda", "KedaController", "keda").Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + kedaControllerDefault).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		kafaksNs := "kafka-52384"
		defer deleteProject(oc, kafaksNs)
		createProject(oc, kafaksNs)
		//Create kafak
		compat_otp.By("Subscribe to AMQ operator")
		defer removeAmqOperator(oc)
		createAmqOperator(oc)
		compat_otp.By("Test for case OCP-52384")
		compat_otp.By(" 1) Create a Kafka instance")
		kafka := filepath.Join(buildPruningBaseDir, "kafka-52384.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + kafka).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + kafka).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("2) Create a Kafka topic")
		kafkaTopic := filepath.Join(buildPruningBaseDir, "kafka-topic-52384.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f=" + kafkaTopic).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + kafkaTopic).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3) Check if Kafka and Kafka topic are ready")
		// Wait for Kafka and KafkaTopic to be ready
		waitForKafkaReady(oc, "my-cluster", kafaksNs)
		namespace := oc.Namespace()

		compat_otp.By("4) Create a Kafka Consumer")
		kafkaConsumerDeployment := filepath.Join(buildPruningBaseDir, "kafka-consumer-deployment-52384.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+kafkaConsumerDeployment, "-n", namespace).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+kafkaConsumerDeployment, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5) Create a scaledobjectc")
		kafkaScaledobject := filepath.Join(buildPruningBaseDir, "kafka-scaledobject-52384.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+kafkaScaledobject, "-n", namespace).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+kafkaScaledobject, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5.1) Check ScaledObject is up")
		err = wait.Poll(3*time.Second, 300*time.Second, func() (bool, error) {
			scaledObjectStatus, _ = oc.AsAdmin().Run("get").Args("ScaledObject", "kafka-amqstreams-consumer-scaledobject", "-o=jsonpath={.status.health.s0-kafka-my-topic.status}", "-n", namespace).Output()
			if scaledObjectStatus == "Happy" {
				e2e.Logf("ScaledObject is up and working")
				return true, nil
			}
			e2e.Logf("ScaledObject is not in working status, current status: %v", scaledObjectStatus)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "scaling failed")
		compat_otp.By("Kafka scaling is up and ready")

		compat_otp.By("6)Create a Kafka load")
		kafkaLoad := filepath.Join(buildPruningBaseDir, "kafka-load-52384.yaml")
		defer oc.AsAdmin().Run("delete").Args("jobs", "--field-selector", "status.successful=1", "-n", namespace).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+kafkaLoad, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("6.1) Check ScaledObject is up")
		err = wait.Poll(3*time.Second, 300*time.Second, func() (bool, error) {
			scaledObjectStatus, _ = oc.AsAdmin().Run("get").Args("ScaledObject", "kafka-amqstreams-consumer-scaledobject", "-o=jsonpath={.status.health.s0-kafka-my-topic.status}", "-n", namespace).Output()
			if scaledObjectStatus == "Happy" {
				e2e.Logf("ScaledObject is up and working")
				return true, nil
			}
			e2e.Logf("ScaledObject is not in working status, current status: %v", scaledObjectStatus)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "scaling failed")
		compat_otp.By("Kafka scaling is up and ready")
	})

	// author: weinliu@redhat.com
	g.It("Author:weinliu-ConnectedOnly-Critical-52385-Automatically scaling pods based on Prometheus metrics[Serial]", func() {
		compat_otp.By("Create a kedacontroller with default template")
		kedaControllerDefault := filepath.Join(buildPruningBaseDir, "keda-controller-default.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-keda", "KedaController", "keda").Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + kedaControllerDefault).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var scaledObjectStatus string
		triggerAuthenticationTempl := filepath.Join(buildPruningBaseDir, "triggerauthentication-52385.yaml")
		triggerAuthentication52385 := triggerAuthenticationDescription{
			secretname: "",
			namespace:  "",
			template:   triggerAuthenticationTempl,
		}

		cmaNs := "cma-52385"
		defer deleteProject(oc, cmaNs)
		createProject(oc, cmaNs)

		compat_otp.By("1) Create OpenShift monitoring for user-defined projects")
		// Look for cluster-level monitoring configuration
		getOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ConfigMap", "cluster-monitoring-config", "-n", "openshift-monitoring", "--ignore-not-found").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Enable user workload monitoring
		if len(getOutput) > 0 {
			compat_otp.By("ConfigMap cluster-monitoring-config exists, extracting cluster-monitoring-config ...")
			extractOutput, _, _ := oc.AsAdmin().WithoutNamespace().Run("extract").Args("ConfigMap/cluster-monitoring-config", "-n", "openshift-monitoring", "--to=-").Outputs()
			//if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(extractOutput, "'", ""), "\"", ""), " ", ""), "enableUserWorkload:true") {
			cleanedOutput := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(extractOutput, "'", ""), "\"", ""), " ", "")
			e2e.Logf("cleanedOutput is  %s", cleanedOutput)
			if matched, _ := regexp.MatchString("enableUserWorkload:\\s*true", cleanedOutput); matched {
				compat_otp.By("User workload is enabled, doing nothing ... ")
			} else {
				compat_otp.By("User workload is not enabled, enabling ...")
				compat_otp.By("Get current monitoring configuration to recover")
				originclusterMonitoringConfig, getContentError := oc.AsAdmin().Run("get").Args("ConfigMap/cluster-monitoring-config", "-ojson", "-n", "openshift-monitoring").Output()
				o.Expect(getContentError).NotTo(o.HaveOccurred())
				originclusterMonitoringConfig, getContentError = sjson.Delete(originclusterMonitoringConfig, `metadata.resourceVersion`)
				o.Expect(getContentError).NotTo(o.HaveOccurred())
				originclusterMonitoringConfig, getContentError = sjson.Delete(originclusterMonitoringConfig, `metadata.uid`)
				o.Expect(getContentError).NotTo(o.HaveOccurred())
				originclusterMonitoringConfigFilePath := filepath.Join(e2e.TestContext.OutputDir, oc.Namespace()+"-52385.json")
				o.Expect(os.WriteFile(originclusterMonitoringConfigFilePath, []byte(originclusterMonitoringConfig), 0644)).NotTo(o.HaveOccurred())
				defer func() {
					errReplace := oc.AsAdmin().WithoutNamespace().Run("replace").Args("-f", originclusterMonitoringConfigFilePath).Execute()
					o.Expect(errReplace).NotTo(o.HaveOccurred())
				}()
				compat_otp.By("Deleting current monitoring configuration")
				oc.WithoutNamespace().AsAdmin().Run("delete").Args("ConfigMap/cluster-monitoring-config", "-n", "openshift-monitoring").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				compat_otp.By("Create my monitoring configuration")
				prometheusConfigmap := filepath.Join(buildPruningBaseDir, "prometheus-configmap.yaml")
				_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f=" + prometheusConfigmap).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		} else {
			e2e.Logf("ConfigMap cluster-monitoring-config does not exist, creating ...")
			prometheusConfigmap := filepath.Join(buildPruningBaseDir, "prometheus-configmap.yaml")
			defer func() {
				errDelete := oc.WithoutNamespace().AsAdmin().Run("delete").Args("-f=" + prometheusConfigmap).Execute()
				o.Expect(errDelete).NotTo(o.HaveOccurred())
			}()
			_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f=" + prometheusConfigmap).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		compat_otp.By("2) Deploy application that exposes Prometheus metrics")
		prometheusComsumer := filepath.Join(buildPruningBaseDir, "prometheus-comsumer-deployment.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+prometheusComsumer, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+prometheusComsumer, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("2.1) Verify the deployment is available")
		errCheck := wait.Poll(20*time.Second, 280*time.Second, func() (bool, error) {
			output, err1 := oc.AsAdmin().Run("get").Args("deployment", "-n", cmaNs).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			if strings.Contains(output, "test-app") && strings.Contains(output, "1/1") {
				e2e.Logf("Deployment has been created Sucessfully!")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("Depolyment has not been created"))

		compat_otp.By("3) Create a Service Account")
		defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("sa", "thanos-52385", "-n", cmaNs).Execute()
		err = oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", "thanos-52385", "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3.1) Create Service Account Token")
		servicetokenTemp := filepath.Join(buildPruningBaseDir, "servicetoken-52385.yaml")
		token, err := oc.AsAdmin().SetNamespace(cmaNs).Run("apply").Args("-f", servicetokenTemp).Output()
		e2e.Logf("err %v, token %v", err, token)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3.2) Make sure the token is available")
		serviceToken, err := oc.AsAdmin().Run("get").Args("secret", "thanos-token", "-n", cmaNs).Output()
		e2e.Logf("err %v, token %v", err, serviceToken)
		o.Expect(err).NotTo(o.HaveOccurred())

		saTokenName := "thanos-token"

		compat_otp.By("4) Define TriggerAuthentication with the Service Account's token")
		triggerAuthentication52385.secretname = string(saTokenName[:])
		triggerAuthentication52385.namespace = cmaNs
		defer oc.AsAdmin().Run("delete").Args("-n", cmaNs, "TriggerAuthentication", "keda-trigger-auth-prometheus").Execute()
		triggerAuthentication52385.create(oc)

		compat_otp.By("4.1) Check TriggerAuthentication is Available")
		triggerauth, err := oc.AsAdmin().Run("get").Args("TriggerAuthentication", "-n", cmaNs).Output()
		e2e.Logf("Triggerauthentication is %v", triggerauth)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5) Create a role for reading metric from Thanos")
		role := filepath.Join(buildPruningBaseDir, "role.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+role, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+role, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5.1) Check Role is Available")
		rolecheck, err := oc.AsAdmin().Run("get").Args("Role", "-n", cmaNs).Output()
		e2e.Logf("Role %v", rolecheck)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5.2) Add the role for reading metrics from Thanos to the Service Account")
		rolebinding := filepath.Join(buildPruningBaseDir, "rolebinding-52385.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f="+rolebinding, "-n", cmaNs).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f="+rolebinding, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("6) Deploy ScaledObject to enable application autoscaling")
		scaledobject := filepath.Join(buildPruningBaseDir, "scaledobject-52385.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f="+scaledobject, "-n", cmaNs).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f="+scaledobject, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("6.1) Check ScaledObject is up")
		err = wait.Poll(3*time.Second, 100*time.Second, func() (bool, error) {
			scaledObjectStatus, _ = oc.AsAdmin().Run("get").Args("ScaledObject", "prometheus-scaledobject", "-o=jsonpath={.status.health.s0-prometheus.status}", "-n", cmaNs).Output()
			if scaledObjectStatus == "Happy" {
				e2e.Logf("ScaledObject is up and working")
				return true, nil
			}
			e2e.Logf("ScaledObject is not in working status, current status: %v", scaledObjectStatus)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "scaling failed")
		compat_otp.By("prometheus scaling is up and ready")

		compat_otp.By("7) Generate requests to test the application autoscaling")
		load := filepath.Join(buildPruningBaseDir, "load-52385.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f="+load, "-n", cmaNs).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f="+load, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("7.1) Check ScaledObject is up")
		err = wait.Poll(3*time.Second, 100*time.Second, func() (bool, error) {
			scaledObjectStatus, _ = oc.AsAdmin().Run("get").Args("ScaledObject", "prometheus-scaledobject", "-o=jsonpath={.status.health.s0-prometheus.status}", "-n", cmaNs).Output()
			if scaledObjectStatus == "Happy" {
				e2e.Logf("ScaledObject is up and working")
				return true, nil
			}
			e2e.Logf("ScaledObject is not in working status, current status: %v", scaledObjectStatus)
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(err, "scaling failed")
		compat_otp.By("prometheus scaling is up and ready")
	})

	//author: asahay@redhat.com

	g.It("Author:asahay-ConnectedOnly-Critical-73296-KEDA-Operator is missing files causing cron triggers with Timezone Failure [Serial]", func() {
		compat_otp.By("Create a kedacontroller with default template")
		kedaControllerDefault := filepath.Join(buildPruningBaseDir, "keda-controller-default.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-keda", "KedaController", "keda").Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + kedaControllerDefault).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		triggerAuthenticationTempl := filepath.Join(buildPruningBaseDir, "triggerauthentication-73296.yaml")
		triggerAuthentication73296 := triggerAuthenticationDescription{
			secretname: "",
			namespace:  "",
			template:   triggerAuthenticationTempl,
		}
		cmaNs := "cma-73296"
		defer deleteProject(oc, cmaNs)
		createProject(oc, cmaNs)

		compat_otp.By("1) Create OpenShift monitoring for user-defined projects")
		// Look for cluster-level monitoring configuration
		getOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ConfigMap", "cluster-monitoring-config", "-n", "openshift-monitoring", "--ignore-not-found").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Enable user workload monitoring
		if len(getOutput) > 0 {
			compat_otp.By("ConfigMap cluster-monitoring-config exists, extracting cluster-monitoring-config ...")
			extractOutput, _, _ := oc.AsAdmin().WithoutNamespace().Run("extract").Args("ConfigMap/cluster-monitoring-config", "-n", "openshift-monitoring", "--to=-").Outputs()
			//if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(extractOutput, "'", ""), "\"", ""), " ", ""), "enableUserWorkload:true") {
			cleanedOutput := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(extractOutput, "'", ""), "\"", ""), " ", "")
			e2e.Logf("cleanedOutput is  %s", cleanedOutput)
			if matched, _ := regexp.MatchString("enableUserWorkload:\\s*true", cleanedOutput); matched {
				compat_otp.By("User workload is enabled, doing nothing ... ")
			} else {
				compat_otp.By("User workload is not enabled, enabling ...")
				compat_otp.By("Get current monitoring configuration to recover")
				originclusterMonitoringConfig, getContentError := oc.AsAdmin().Run("get").Args("ConfigMap/cluster-monitoring-config", "-ojson", "-n", "openshift-monitoring").Output()
				o.Expect(getContentError).NotTo(o.HaveOccurred())
				originclusterMonitoringConfig, getContentError = sjson.Delete(originclusterMonitoringConfig, `metadata.resourceVersion`)
				o.Expect(getContentError).NotTo(o.HaveOccurred())
				originclusterMonitoringConfig, getContentError = sjson.Delete(originclusterMonitoringConfig, `metadata.uid`)
				o.Expect(getContentError).NotTo(o.HaveOccurred())
				originclusterMonitoringConfigFilePath := filepath.Join(e2e.TestContext.OutputDir, oc.Namespace()+"-73296.json")
				o.Expect(os.WriteFile(originclusterMonitoringConfigFilePath, []byte(originclusterMonitoringConfig), 0644)).NotTo(o.HaveOccurred())
				defer func() {
					errReplace := oc.AsAdmin().WithoutNamespace().Run("replace").Args("-f", originclusterMonitoringConfigFilePath).Execute()
					o.Expect(errReplace).NotTo(o.HaveOccurred())
				}()
				compat_otp.By("Deleting current monitoring configuration")
				oc.WithoutNamespace().AsAdmin().Run("delete").Args("ConfigMap/cluster-monitoring-config", "-n", "openshift-monitoring").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				compat_otp.By("Create my monitoring configuration")
				prometheusConfigmap := filepath.Join(buildPruningBaseDir, "prometheus-configmap.yaml")
				_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f=" + prometheusConfigmap).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		} else {
			e2e.Logf("ConfigMap cluster-monitoring-config does not exist, creating ...")
			prometheusConfigmap := filepath.Join(buildPruningBaseDir, "prometheus-configmap.yaml")
			defer func() {
				errDelete := oc.WithoutNamespace().AsAdmin().Run("delete").Args("-f=" + prometheusConfigmap).Execute()
				o.Expect(errDelete).NotTo(o.HaveOccurred())
			}()
			_, err = oc.WithoutNamespace().AsAdmin().Run("create").Args("-f=" + prometheusConfigmap).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		compat_otp.By("2) Deploy application that exposes Prometheus metrics")
		prometheusComsumer := filepath.Join(buildPruningBaseDir, "prometheus-comsumer-deployment.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+prometheusComsumer, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+prometheusComsumer, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3) Create a Service Account")
		defer oc.WithoutNamespace().AsAdmin().Run("delete").Args("sa", "thanos-73296", "-n", cmaNs).Execute()
		err = oc.WithoutNamespace().AsAdmin().Run("create").Args("sa", "thanos-73296", "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3.1) Create Service Account Token")
		servicetokenTemp := filepath.Join(buildPruningBaseDir, "servicetoken-73296.yaml")
		token, err := oc.AsAdmin().SetNamespace(cmaNs).Run("apply").Args("-f", servicetokenTemp).Output()
		e2e.Logf("err %v, token %v", err, token)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3.2) Make sure the token is still there and didn't get deleted")
		serviceToken, err := oc.AsAdmin().Run("get").Args("secret", "thanos-token", "-n", cmaNs).Output()
		e2e.Logf("err %v, token %v", err, serviceToken)
		o.Expect(err).NotTo(o.HaveOccurred())

		saTokenName := "thanos-token"

		compat_otp.By("3.3) Define TriggerAuthentication with the Service Account's token")
		triggerAuthentication73296.secretname = string(saTokenName[:])
		triggerAuthentication73296.namespace = cmaNs
		defer oc.AsAdmin().Run("delete").Args("-n", cmaNs, "TriggerAuthentication", "keda-trigger-auth-prometheus").Execute()
		triggerAuthentication73296.create(oc)

		compat_otp.By("4) Create a role for reading metric from Thanos")
		role := filepath.Join(buildPruningBaseDir, "role.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+role, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+role, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5) Add the role for reading metrics from Thanos to the Service Account")
		rolebinding := filepath.Join(buildPruningBaseDir, "rolebinding-73296.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f="+rolebinding, "-n", cmaNs).Execute()
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("-f="+rolebinding, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("6) Create a Test Deployment")
		testDeploymentTemp := filepath.Join(buildPruningBaseDir, "testdeployment-73296.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+testDeploymentTemp, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+testDeploymentTemp, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("6.1) Verify the deployment is available")
		errCheck := wait.Poll(20*time.Second, 280*time.Second, func() (bool, error) {
			output, err1 := oc.AsAdmin().Run("get").Args("deployment", "-n", cmaNs).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			if strings.Contains(output, "busybox") && strings.Contains(output, "1/1") {
				e2e.Logf("Deployment has been created Sucessfully!")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("Depolyment has not been created"))

		compat_otp.By("7) Create a ScaledObject with a cron trigger with timezone applied.")
		timezoneScaledObjectTemp := filepath.Join(buildPruningBaseDir, "timezonescaledobject-73296.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+timezoneScaledObjectTemp, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+timezoneScaledObjectTemp, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("7.1) Verifying the scaledobject readiness")
		errCheck = wait.Poll(20*time.Second, 380*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("scaledobject", "cron-scaledobject", "-n", cmaNs, "-o", "jsonpath={.status.conditions[?(@.status=='True')].status} {.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if output == "True True True" {
				e2e.Logf("ScaledObject is Active and Running.")
				return true, nil
			}
			return false, nil
		})
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("ScaledObject is not ready"))

		PodName := getPodNameByLabel(oc, "openshift-keda", "app=keda-operator")
		waitPodReady(oc, "openshift-keda", "app=keda-operator")
		compat_otp.By(" 8) Check the Logs Containig INFO Reconciling ScaledObject")
		log, err := compat_otp.GetSpecificPodLogs(oc, "openshift-keda", "", PodName[0], "")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(log, "INFO\tReconciling ScaledObject")).Should(o.BeTrue())

	})
	// author: asahay@redhat.com
	g.It("Author:asahay-High-60966-CMA Scale applications based on memory metrics [Serial]", func() {
		compat_otp.By("1) Create a kedacontroller with default template")

		kedaControllerDefault := filepath.Join(buildPruningBaseDir, "keda-controller-default.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", "openshift-keda", "KedaController", "keda").Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f=" + kedaControllerDefault).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		cmaNs := "cma-60966"
		defer deleteProject(oc, cmaNs)
		createProject(oc, cmaNs)

		var output string

		compat_otp.By("2) Creating a Keda HPA deployment")
		kedaHPADemoDeploymentTemp := filepath.Join(buildPruningBaseDir, "keda-hpa-demo-deployment.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+kedaHPADemoDeploymentTemp, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+kedaHPADemoDeploymentTemp, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3) Verify the deployment is available")
		errCheck := wait.Poll(20*time.Second, 280*time.Second, func() (bool, error) {
			output, err1 := oc.AsAdmin().Run("get").Args("deployment", "-n", cmaNs).Output()
			o.Expect(err1).NotTo(o.HaveOccurred())
			if strings.Contains(output, "keda-hpa-demo-deployment") && strings.Contains(output, "1/1") {
				e2e.Logf("Deployment has been created Sucessfully!")
				return true, nil
			}
			return false, nil
		})
		e2e.Logf("Output: %v", output)
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("Depolyment has not been created"))

		compat_otp.By("4) Creating a ScaledObject")
		memScaledObjectTemp := filepath.Join(buildPruningBaseDir, "mem-scaledobject.yaml")
		defer oc.AsAdmin().Run("delete").Args("-f="+memScaledObjectTemp, "-n", cmaNs).Execute()
		err = oc.AsAdmin().Run("create").Args("-f="+memScaledObjectTemp, "-n", cmaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("5) Verifying the scaledobject readiness")
		errCheck = wait.Poll(20*time.Second, 380*time.Second, func() (bool, error) {
			output, err := oc.AsAdmin().Run("get").Args("scaledobject", "mem-scaledobject", "-n", cmaNs, "-o", "jsonpath={.status.conditions[?(@.status=='True')].status} {.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if output == "True True True" {
				e2e.Logf("ScaledObject is Active and Running.")
				return true, nil
			}
			return false, nil
		})
		e2e.Logf("Output: %v", output)
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("ScaledObject is not ready"))

		compat_otp.By("6) Checking HPA status using jsonpath")
		errCheck = wait.Poll(20*time.Second, 380*time.Second, func() (bool, error) {
			output, err = oc.AsAdmin().Run("get").Args("hpa", "keda-hpa-mem-scaledobject", "-n", cmaNs, "-o", "jsonpath={.spec.minReplicas} {.spec.maxReplicas}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			// The lower limit for the number of replicas to which the autoscaler can scale down is 1 and the upper limit for the number of replicas to which the autoscaler can scale up is 10
			if strings.Contains(output, "1 10") {
				e2e.Logf("HPA is configured correctly as expected!")
				return true, nil
			}

			return false, nil
		})
		e2e.Logf("Output: %v", output)
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("HPA status check failed"))

		compat_otp.By("7) Describing HPA to verify conditions")

		errCheck = wait.Poll(20*time.Second, 380*time.Second, func() (bool, error) {
			output, err = oc.AsAdmin().Run("get").Args("hpa", "keda-hpa-mem-scaledobject", "-n", cmaNs, "-o", "jsonpath={.status.conditions[?(@.type=='AbleToScale')].status} {.status.conditions[?(@.type=='ScalingActive')].status}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if output == "True True" {
				e2e.Logf("HPA conditions are as expected: AbleToScale is True, ScalingActive is True.")
				return true, nil
			}
			return false, nil
		})
		e2e.Logf("Output: %v", output)
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("HPA conditions are not met"))

	})
})

var _ = g.Describe("[sig-node][NodeQE] NODE VPA Vertical Pod Autoscaler", func() {

	defer g.GinkgoRecover()
	var (
		oc                  = exutil.NewCLI("vpa-operator")
		buildPruningBaseDir = compat_otp.FixturePath("testdata", "node")
	)
	g.BeforeEach(func() {
		compat_otp.SkipMissingQECatalogsource(oc)
		createVpaOperator(oc)
	})
	// author: weinliu@redhat.com
	g.It("Author:weinliu-DEPRECATED-High-60991-VPA Install", func() {
		g.By("VPA operator is installed successfully")
	})
	// author: weinliu@redhat.com
	g.It("Author:weinliu-High-70961-Allow cluster admins to specify VPA API client rates and memory-saver [Serial]", func() {
		g.By("VPA operator is installed successfully")
		compat_otp.By("Create a new VerticalPodAutoscalerController ")
		vpaNs := "openshift-vertical-pod-autoscaler"
		vpacontroller := filepath.Join(buildPruningBaseDir, "vpacontroller-70961.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f="+vpacontroller, "-n", vpaNs).Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f="+vpacontroller, "-n", vpaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		compat_otp.By("Check VPA operator's args")
		recommenderArgs, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("VerticalPodAutoscalerController", "vpa-70961", "-n", "openshift-vertical-pod-autoscaler", "-o=jsonpath={.spec.deploymentOverrides.recommender.container.args}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect("[\"--kube-api-qps=20.0\",\"--kube-api-burst=60.0\",\"--memory-saver=true\"]").Should(o.Equal(recommenderArgs))
		admissioinArgs, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("VerticalPodAutoscalerController", "vpa-70961", "-n", "openshift-vertical-pod-autoscaler", "-o=jsonpath={.spec.deploymentOverrides.admission.container.args}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect("[\"--kube-api-qps=30.0\",\"--kube-api-burst=40.0\"]").Should(o.Equal(admissioinArgs))
		updaterArgs, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("VerticalPodAutoscalerController", "vpa-70961", "-n", "openshift-vertical-pod-autoscaler", "-o=jsonpath={.spec.deploymentOverrides.updater.container.args}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect("[\"--kube-api-qps=20.0\",\"--kube-api-burst=80.0\"]").Should(o.Equal(updaterArgs))
	})
	// author: weinliu@redhat.com
	g.It("Author:weinliu-High-70962-Allow cluster admins to specify CPU & Memory requests and limits of VPA controllers [Serial]", func() {
		compat_otp.By("VPA operator is installed successfully")
		compat_otp.By("Create a new VerticalPodAutoscalerController ")
		vpaNs := "openshift-vertical-pod-autoscaler"
		vpacontroller := filepath.Join(buildPruningBaseDir, "vpacontroller-70962.yaml")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f="+vpacontroller, "-n", vpaNs).Execute()
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("-f="+vpacontroller, "-n", vpaNs).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		compat_otp.By("Check VPA operator's args")
		recommenderArgs, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("VerticalPodAutoscalerController", "vpa-70962", "-n", "openshift-vertical-pod-autoscaler", "-o=jsonpath={.spec.deploymentOverrides.recommender.container.resources.requests}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect("{\"cpu\":\"60m\",\"memory\":\"60Mi\"}").Should(o.Equal(recommenderArgs))
		admissioinArgs, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("VerticalPodAutoscalerController", "vpa-70962", "-n", "openshift-vertical-pod-autoscaler", "-o=jsonpath={.spec.deploymentOverrides.admission.container.resources.requests}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect("{\"cpu\":\"40m\",\"memory\":\"40Mi\"}").Should(o.Equal(admissioinArgs))
		updaterArgs, err := oc.WithoutNamespace().AsAdmin().Run("get").Args("VerticalPodAutoscalerController", "vpa-70962", "-n", "openshift-vertical-pod-autoscaler", "-o=jsonpath={.spec.deploymentOverrides.updater.container.resources.requests}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect("{\"cpu\":\"80m\",\"memory\":\"80Mi\"}").Should(o.Equal(updaterArgs))
	})
})

var _ = g.Describe("[sig-node][NodeQE] NODE Install and verify Cluster Resource Override Admission Webhook", func() {
	defer g.GinkgoRecover()
	var (
		oc = compat_otp.NewCLI("clusterresourceoverride-operator", compat_otp.KubeConfigPath())
	)
	g.BeforeEach(func() {

		g.By("Skip test when precondition not meet !!!")
		compat_otp.SkipMissingQECatalogsource(oc)
		installOperatorClusterresourceoverride(oc)

	})
	// author: asahay@redhat.com

	g.It("Author:asahay-High-27070-Cluster Resource Override Operator. [Serial]", func() {
		defer deleteAPIService(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ClusterResourceOverride", "cluster", "-n", "clusterresourceoverride-operator").Execute()
		createCRClusterresourceoverride(oc)
		var err error
		var croCR string
		errCheck := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			croCR, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ClusterResourceOverride", "cluster", "-n", "clusterresourceoverride-operator").Output()
			if err != nil {
				e2e.Logf("error  %v, please try next round", err)
				return false, nil
			}
			if !strings.Contains(croCR, "cluster") {
				return false, nil
			}
			return true, nil

		})
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("can not get cluster with output %v, the error is %v", croCR, err))
		e2e.Logf("Operator is installed successfully")
	})

	g.It("Author:asahay-Medium-27075-Testing the config changes. [Serial]", func() {

		defer deleteAPIService(oc)
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("ClusterResourceOverride", "cluster").Execute()
		createCRClusterresourceoverride(oc)
		var err error
		var croCR string
		errCheck := wait.Poll(10*time.Second, 120*time.Second, func() (bool, error) {
			croCR, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("ClusterResourceOverride", "cluster", "-n", "clusterresourceoverride-operator").Output()
			if err != nil {
				e2e.Logf("error  %v, please try next round", err)
				return false, nil
			}
			if !strings.Contains(croCR, "cluster") {
				return false, nil
			}
			return true, nil
		})
		compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("can not get cluster with output %v, the error is %v", croCR, err))
		e2e.Logf("Operator is installed successfully")

		g.By("Testing the changes\n")
		testCRClusterresourceoverride(oc)

	})

})
