package node

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
)

type cpuPerfProfile struct {
	name     string
	isolated string
	template string
}

type liveProbeTermPeriod struct {
	name                  string
	namespace             string
	terminationgrace      int
	probeterminationgrace int
	template              string
}

type startProbeTermPeriod struct {
	name                  string
	namespace             string
	terminationgrace      int
	probeterminationgrace int
	template              string
}

type readProbeTermPeriod struct {
	name                  string
	namespace             string
	terminationgrace      int
	probeterminationgrace int
	template              string
}

type liveProbeNoTermPeriod struct {
	name             string
	namespace        string
	terminationgrace int
	template         string
}

type podWkloadCPUNoAnotation struct {
	name        string
	namespace   string
	workloadcpu string
	template    string
}

type podWkloadCPUDescription struct {
	name        string
	namespace   string
	workloadcpu string
	template    string
}

type podNoWkloadCPUDescription struct {
	name      string
	namespace string
	template  string
}

type podGuDescription struct {
	name      string
	namespace string
	nodename  string
	template  string
}

type podHelloDescription struct {
	name      string
	namespace string
	template  string
}

type podModifyDescription struct {
	name          string
	namespace     string
	mountpath     string
	command       string
	args          string
	restartPolicy string
	user          string
	role          string
	level         string
	template      string
}

type podLivenessProbe struct {
	name                  string
	namespace             string
	overridelivenessgrace string
	terminationgrace      int
	failurethreshold      int
	periodseconds         int
	template              string
}

type kubeletCfgMaxpods struct {
	name       string
	labelkey   string
	labelvalue string
	maxpods    int
	template   string
}

type ctrcfgDescription struct {
	namespace  string
	pidlimit   int
	loglevel   string
	overlay    string
	logsizemax string
	command    string
	configFile string
	template   string
}

type objectTableRefcscope struct {
	kind string
	name string
}

type podTerminationDescription struct {
	name      string
	namespace string
	template  string
}

type podInitConDescription struct {
	name      string
	namespace string
	template  string
}

type podSigstoreDescription struct {
	name      string
	namespace string
	template  string
}

type podUserNSDescription struct {
	name      string
	namespace string
	template  string
}

type podSleepDescription struct {
	namespace string
	template  string
}

type kubeletConfigDescription struct {
	name       string
	labelkey   string
	labelvalue string
	template   string
}

type memHogDescription struct {
	name       string
	namespace  string
	labelkey   string
	labelvalue string
	template   string
}

type podTwoContainersDescription struct {
	name      string
	namespace string
	template  string
}

type ctrcfgOverlayDescription struct {
	name     string
	overlay  string
	template string
}

type podDevFuseDescription struct {
	name      string
	namespace string
	template  string
}

type podLogLinkDescription struct {
	name      string
	namespace string
	template  string
}

type podWASM struct {
	name      string
	namespace string
	template  string
}

type podCPULoadBalance struct {
	name         string
	namespace    string
	runtimeclass string
	template     string
}

type podDisruptionBudget struct {
	name         string
	namespace    string
	minAvailable string
	template     string
}

type deployment struct {
	name      string
	namespace string
	replicas  string
	image     string
	nodename  string
	template  string
}

type triggerAuthenticationDescription struct {
	secretname string
	namespace  string
	template   string
}

// ImgConfigContDescription describes an image configuration container for testing.
type ImgConfigContDescription struct {
	name     string
	template string
}

type subscriptionDescription struct {
	catalogSourceName string
}

func (cpuPerfProfile *cpuPerfProfile) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cpuPerfProfile.template, "-p", "NAME="+cpuPerfProfile.name, "ISOLATED="+cpuPerfProfile.isolated)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (cpuPerfProfile *cpuPerfProfile) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("PerformanceProfile", cpuPerfProfile.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podCPULoadBalance *podCPULoadBalance) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podCPULoadBalance.template, "-p", "NAME="+podCPULoadBalance.name, "NAMESPACE="+podCPULoadBalance.namespace, "RUNTIMECLASS="+podCPULoadBalance.runtimeclass)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podCPULoadBalance *podCPULoadBalance) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podCPULoadBalance.namespace, "pod", podCPULoadBalance.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podWASM *podWASM) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podWASM.template, "-p", "NAME="+podWASM.name, "NAMESPACE="+podWASM.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podWASM *podWASM) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podWASM.namespace, "pod", podWASM.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podDevFuse *podDevFuseDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podDevFuse.template, "-p", "NAME="+podDevFuse.name, "NAMESPACE="+podDevFuse.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podDevFuse *podDevFuseDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podDevFuse.namespace, "pod", podDevFuse.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkDevFuseMount(oc *exutil.CLI, namespace string, podname string) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", namespace, podname, "/bin/bash", "-c", "ls -al /dev | grep fuse").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(status, "fuse") {
			e2e.Logf("\ndev fuse is mounted inside the pod")
			return true, nil
		}
		return false, nil
	})
}

func (podLogLink *podLogLinkDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podLogLink.template, "-p", "NAME="+podLogLink.name, "NAMESPACE="+podLogLink.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podLogLink *podLogLinkDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podLogLink.namespace, "pod", podLogLink.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (liveProbe *liveProbeTermPeriod) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", liveProbe.template, "-p", "NAME="+liveProbe.name, "NAMESPACE="+liveProbe.namespace, "TERMINATIONGRACE="+strconv.Itoa(liveProbe.terminationgrace), "PROBETERMINATIONGRACE="+strconv.Itoa(liveProbe.probeterminationgrace))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (liveProbe *liveProbeTermPeriod) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", liveProbe.namespace, "pod", liveProbe.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (startProbe *startProbeTermPeriod) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", startProbe.template, "-p", "NAME="+startProbe.name, "NAMESPACE="+startProbe.namespace, "TERMINATIONGRACE="+strconv.Itoa(startProbe.terminationgrace), "PROBETERMINATIONGRACE="+strconv.Itoa(startProbe.probeterminationgrace))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (startProbe *startProbeTermPeriod) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", startProbe.namespace, "pod", startProbe.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (readProbe *readProbeTermPeriod) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", readProbe.template, "-p", "NAME="+readProbe.name, "NAMESPACE="+readProbe.namespace, "TERMINATIONGRACE="+strconv.Itoa(readProbe.terminationgrace), "PROBETERMINATIONGRACE="+strconv.Itoa(readProbe.probeterminationgrace))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (readProbe *readProbeTermPeriod) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", readProbe.namespace, "pod", readProbe.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (liveProbe *liveProbeNoTermPeriod) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", liveProbe.template, "-p", "NAME="+liveProbe.name, "NAMESPACE="+liveProbe.namespace, "TERMINATIONGRACE="+strconv.Itoa(liveProbe.terminationgrace))
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (liveProbe *liveProbeNoTermPeriod) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", liveProbe.namespace, "pod", liveProbe.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podNoWkloadCPU *podNoWkloadCPUDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podNoWkloadCPU.template, "-p", "NAME="+podNoWkloadCPU.name, "NAMESPACE="+podNoWkloadCPU.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podNoWkloadCPU *podNoWkloadCPUDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podNoWkloadCPU.namespace, "pod", podNoWkloadCPU.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podGu *podGuDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podGu.template, "-p", "NAME="+podGu.name, "NAMESPACE="+podGu.namespace, "NODENAME="+podGu.nodename)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podGu *podGuDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podGu.namespace, "pod", podGu.name, "--ignore-not-found").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getWorkersList(oc *exutil.CLI) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Fields(output)
}

func getCPUNum(oc *exutil.CLI, node string) int {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", node, "-o=jsonpath={.status.capacity.cpu}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	cpuNum, err := strconv.Atoi(output)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Cpu num is: [%d]\n", cpuNum)
	return cpuNum
}

func getCgroupVersion(oc *exutil.CLI) string {
	workerNodes := getWorkersList(oc)
	cgroupV, err := compat_otp.DebugNodeWithChroot(oc, workerNodes[0], "/bin/bash", "-c", "stat -fc %T /sys/fs/cgroup")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("cgroup version info is: [%v]\n", cgroupV)
	if strings.Contains(string(cgroupV), "tmpfs") {
		return "tmpfs"
	} else if strings.Contains(string(cgroupV), "cgroup2fs") {
		return "cgroup2fs"
	} else {
		return cgroupV
	}
}

func checkReservedCPU(oc *exutil.CLI, reservedCPU string) {
	workerNodes := getWorkersList(oc)
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		for _, node := range workerNodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				kubeletConf, err := compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "cat /etc/kubernetes/kubelet.conf | grep reservedSystemCPUs")
				o.Expect(err).NotTo(o.HaveOccurred())
				//need match : "reservedSystemCPUs": "0-3"
				cpuStr := `"reservedSystemCPUs": "` + reservedCPU + `"`
				if strings.Contains(string(kubeletConf), cpuStr) {
					e2e.Logf("Reserved Cpu: [%s], is expected \n", kubeletConf)
					crioOutput, err := compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "pgrep crio | while read i; do taskset -cp $i; done")
					o.Expect(err).NotTo(o.HaveOccurred())
					crioCPUStr := "current affinity list: " + reservedCPU
					if strings.Contains(crioOutput, crioCPUStr) {
						e2e.Logf("crio use CPU: [%s], is expected \n", crioOutput)
						return true, nil
					}
					e2e.Logf("crio use CPU: [%s], not expected \n", crioOutput)
					return false, nil
				}
				e2e.Logf("Reserved Cpu: [%s], not expected \n", kubeletConf)
				return false, nil
			}
			e2e.Logf("\n NODE %s IS NOT READY\n", node)
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("Check reservedCpu failed!\n"))
}

// NewDeployment creates a deployment generator function (uses default image from template).
func NewDeployment(name, namespace, replicas, template string) *deployment {
	return &deployment{name, namespace, replicas, "", "", template}
}

// NewDeploymentWithNode creates a deployment generator function (uses default image from template and deploy on specific node).
func NewDeploymentWithNode(name, namespace, replicas, hostname, template string) *deployment {
	return &deployment{name, namespace, replicas, "", hostname, template}
}

// NewDeploymentWithImage creates a deployment generator function with image override.
func NewDeploymentWithImage(name, namespace, replicas, image, template string) *deployment {
	return &deployment{name, namespace, replicas, image, "", template}
}

// this function consider parameter image and hostname can't coexist by default
func (deployment *deployment) create(oc *exutil.CLI) {
	imageArg := ""
	nodenameArg := ""
	if deployment.image != "" {
		imageArg = "IMAGE=" + deployment.image
		err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deployment.template, "-p", "NAME="+deployment.name, "NAMESPACE="+deployment.namespace, "REPLICAS="+deployment.replicas, imageArg)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else if deployment.nodename != "" {
		nodenameArg = "NODENAME=" + deployment.nodename
		err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deployment.template, "-p", "NAME="+deployment.name, "NAMESPACE="+deployment.namespace, "REPLICAS="+deployment.replicas, nodenameArg)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", deployment.template, "-p", "NAME="+deployment.name, "NAMESPACE="+deployment.namespace, "REPLICAS="+deployment.replicas)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// waits until all the pods in the created deployment are in Ready state
func (deployment *deployment) waitForCreation(oc *exutil.CLI, timeoutMin int) {
	err := wait.Poll(3*time.Second, time.Duration(timeoutMin)*time.Minute, func() (bool, error) {
		msg, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", deployment.name, "-o=jsonpath={.status.readyReplicas}", "-n", deployment.namespace).Output()
		if err != nil {
			e2e.Logf("Command failed with error: %s .... there are no ready workloads", err)
			return false, nil
		}

		if (msg == "" && deployment.replicas == "0") || msg == deployment.replicas {
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("fail to create deployment %v in namespace %v", deployment.name, deployment.namespace))
}

func (deployment *deployment) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", deployment.namespace, "deployment", deployment.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// NewPDB creates a PDB generator function.
func NewPDB(name, namespace, minAvailable, template string) *podDisruptionBudget {
	return &podDisruptionBudget{name, namespace, minAvailable, template}
}

func (pdb *podDisruptionBudget) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pdb.template, "-p", "NAME="+pdb.name, "NAMESPACE="+pdb.namespace, "MIN_AVAILABLE="+pdb.minAvailable)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pdb *podDisruptionBudget) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", pdb.namespace, "poddisruptionbudget", pdb.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

type cmaKedaControllerDescription struct {
	level     string
	template  string
	name      string
	namespace string
}

type pvcKedaControllerDescription struct {
	level          string
	template       string
	name           string
	namespace      string
	watchNamespace string
}

type runtimeTimeoutDescription struct {
	name       string
	labelkey   string
	labelvalue string
	template   string
}

type systemReserveESDescription struct {
	name       string
	labelkey   string
	labelvalue string
	template   string
}

type upgradeMachineconfig1Description struct {
	name     string
	template string
}

type upgradeMachineconfig2Description struct {
	name     string
	template string
}

func (podWkloadCPU *podWkloadCPUDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podWkloadCPU.template, "-p", "NAME="+podWkloadCPU.name, "NAMESPACE="+podWkloadCPU.namespace, "WORKLOADCPU="+podWkloadCPU.workloadcpu)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podWkloadCPU *podWkloadCPUDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podWkloadCPU.namespace, "pod", podWkloadCPU.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podWkloadCPUNoAnota *podWkloadCPUNoAnotation) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podWkloadCPUNoAnota.template, "-p", "NAME="+podWkloadCPUNoAnota.name, "NAMESPACE="+podWkloadCPUNoAnota.namespace, "WORKLOADCPU="+podWkloadCPUNoAnota.workloadcpu)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podWkloadCPUNoAnota *podWkloadCPUNoAnotation) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podWkloadCPUNoAnota.namespace, "pod", podWkloadCPUNoAnota.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podHello *podHelloDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podHello.template, "-p", "NAME="+podHello.name, "NAMESPACE="+podHello.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podHello *podHelloDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podHello.namespace, "pod", podHello.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (ctrcfg *ctrcfgOverlayDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ctrcfg.template, "-p", "NAME="+ctrcfg.name, "OVERLAY="+ctrcfg.overlay)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podUserNS *podUserNSDescription) createPodUserNS(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podUserNS.template, "-p", "NAME="+podUserNS.name, "NAMESPACE="+podUserNS.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podUserNS *podUserNSDescription) deletePodUserNS(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podUserNS.namespace, "pod", podUserNS.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (kubeletConfig *kubeletConfigDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", kubeletConfig.template, "-p", "NAME="+kubeletConfig.name, "LABELKEY="+kubeletConfig.labelkey, "LABELVALUE="+kubeletConfig.labelvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (memHog *memHogDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", memHog.template, "-p", "NAME="+memHog.name, "LABELKEY="+memHog.labelkey, "LABELVALUE="+memHog.labelvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podSleep *podSleepDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podSleep.template, "-p", "NAMESPACE="+podSleep.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (runtimeTimeout *runtimeTimeoutDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", runtimeTimeout.template, "-p", "NAME="+runtimeTimeout.name, "LABELKEY="+runtimeTimeout.labelkey, "LABELVALUE="+runtimeTimeout.labelvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (runtimeTimeout *runtimeTimeoutDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", runtimeTimeout.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (systemReserveES *systemReserveESDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", systemReserveES.template, "-p", "NAME="+systemReserveES.name, "LABELKEY="+systemReserveES.labelkey, "LABELVALUE="+systemReserveES.labelvalue)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (systemReserveES *systemReserveESDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", systemReserveES.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (upgradeMachineconfig1 *upgradeMachineconfig1Description) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", upgradeMachineconfig1.template, "-p", "NAME="+upgradeMachineconfig1.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (upgradeMachineconfig1 *upgradeMachineconfig1Description) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", upgradeMachineconfig1.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (upgradeMachineconfig2 *upgradeMachineconfig2Description) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", upgradeMachineconfig2.template, "-p", "NAME="+upgradeMachineconfig2.name)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (upgradeMachineconfig2 *upgradeMachineconfig2Description) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", upgradeMachineconfig2.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

// Delete Namespace with all resources
func (podSleep *podSleepDescription) deleteProject(oc *exutil.CLI) error {
	e2e.Logf("Deleting Project ...")
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", podSleep.namespace).Execute()
}

func (podInitCon *podInitConDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podInitCon.template, "-p", "NAME="+podInitCon.name, "NAMESPACE="+podInitCon.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podInitCon *podInitConDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podInitCon.namespace, "pod", podInitCon.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podSigstore *podSigstoreDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podSigstore.template, "-p", "NAME="+podSigstore.name, "NAMESPACE="+podSigstore.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podSigstore *podSigstoreDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podSigstore.namespace, "pod", podSigstore.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func (kubeletcfg *kubeletCfgMaxpods) createKubeletConfigMaxpods(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", kubeletcfg.template, "-p", "NAME="+kubeletcfg.name, "LABELKEY="+kubeletcfg.labelkey, "LABELVALUE="+kubeletcfg.labelvalue, "MAXPODS="+strconv.Itoa(kubeletcfg.maxpods))
	if err != nil {
		e2e.Logf("the err of createKubeletConfigMaxpods:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (kubeletcfg *kubeletCfgMaxpods) deleteKubeletConfigMaxpods(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("kubeletconfig", kubeletcfg.name).Execute()
	if err != nil {
		e2e.Logf("the err of deleteKubeletConfigMaxpods:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podLivenessProbe) createPodLivenessProbe(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "OVERRIDELIVENESSGRACE="+pod.overridelivenessgrace, "TERMINATIONGRACE="+strconv.Itoa(pod.terminationgrace), "FAILURETHRESHOLD="+strconv.Itoa(pod.failurethreshold), "PERIODSECONDS="+strconv.Itoa(pod.periodseconds))
	if err != nil {
		e2e.Logf("the err of createPodLivenessProbe:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pod *podLivenessProbe) deletePodLivenessProbe(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", pod.namespace, "pod", pod.name).Execute()
	if err != nil {
		e2e.Logf("the err of deletePodLivenessProbe:%v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podModify *podModifyDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podModify.template, "-p", "NAME="+podModify.name, "NAMESPACE="+podModify.namespace, "MOUNTPATH="+podModify.mountpath, "COMMAND="+podModify.command, "ARGS="+podModify.args, "POLICY="+podModify.restartPolicy, "USER="+podModify.user, "ROLE="+podModify.role, "LEVEL="+podModify.level)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podModify *podModifyDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podModify.namespace, "pod", podModify.name).Execute()
}

func (podTermination *podTerminationDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podTermination.template, "-p", "NAME="+podTermination.name, "NAMESPACE="+podTermination.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (podTermination *podTerminationDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podTermination.namespace, "pod", podTermination.name).Execute()
}

func createResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "node-config.json")
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))

	e2e.Logf("The resource is %s", jsonCfg)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", jsonCfg).Execute()
}

func podStatusReason(oc *exutil.CLI) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.initContainerStatuses[*].state.waiting.reason}", "-n", oc.Namespace()).Output()
		e2e.Logf("the status of pod is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "CrashLoopBackOff") {
			e2e.Logf(" Pod failed status reason is :%s", status)
			return true, nil
		}
		return false, nil
	})
}

func podStatusterminatedReason(oc *exutil.CLI) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[*].status.initContainerStatuses[*].state.terminated.reason}", "-n", oc.Namespace()).Output()
		e2e.Logf("the status of pod is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "Error") {
			e2e.Logf(" Pod failed status reason is :%s", status)
			return true, nil
		}
		return false, nil
	})
}

func podStatus(oc *exutil.CLI, namespace string, podName string) error {
	e2e.Logf("check if pod is available")
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(status, "True") {
			e2e.Logf("Pod is running and container is Ready!")
			return true, nil
		}
		return false, nil
	})
}

func podEvent(oc *exutil.CLI, timeout int, keyword string) error {
	return wait.Poll(10*time.Second, time.Duration(timeout)*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", oc.Namespace()).Output()
		if err != nil {
			e2e.Logf("Can't get events from test project, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString(keyword, output); matched {
			e2e.Logf("%s", keyword)
			return true, nil
		}
		return false, nil
	})
}

func kubeletNotPromptDupErr(oc *exutil.CLI, keyword string, name string) error {
	return wait.Poll(10*time.Second, 3*time.Minute, func() (bool, error) {
		re := regexp.MustCompile(keyword)
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("kubeletconfig", name, "-o=jsonpath={.status.conditions[*]}").Output()
		if err != nil {
			e2e.Logf("Can't get kubeletconfig status, error: %s. Trying again", err)
			return false, nil
		}
		found := re.FindAllString(output, -1)
		if lenStr := len(found); lenStr > 1 {
			e2e.Logf("[%s] appear %d times.", keyword, lenStr)
			return false, nil
		} else if lenStr == 1 {
			e2e.Logf("[%s] appear %d times.\nkubeletconfig not prompt duplicate error message", keyword, lenStr)
			return true, nil
		} else {
			e2e.Logf("error: kubelet not prompt [%s]", keyword)
			return false, nil
		}
	})
}

func volStatus(oc *exutil.CLI) error {
	e2e.Logf("check content of volume")
	return wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("init-volume", "-c", "hello-pod", "cat", "/init-test/volume-test", "-n", oc.Namespace()).Output()
		e2e.Logf("The content of the vol is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "This is OCP volume test") {
			e2e.Logf(" Init containers with volume work fine \n")
			return true, nil
		}
		return false, nil
	})
}

// ContainerSccStatus get scc status of container
func ContainerSccStatus(oc *exutil.CLI) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "hello-pod", "-o=jsonpath={.spec.securityContext.seLinuxOptions.*}", "-n", oc.Namespace()).Output()
		e2e.Logf("The Container SCC Content is %v", status)
		if err != nil {
			e2e.Failf("the result of ReadFile:%v", err)
			return false, nil
		}
		if strings.Contains(status, "unconfined_u unconfined_r s0:c25,c968") {
			e2e.Logf("SeLinuxOptions in pod applied to container Sucessfully \n")
			return true, nil
		}
		return false, nil
	})
}

func (ctrcfg *ctrcfgDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ctrcfg.template, "-p", "LOGLEVEL="+ctrcfg.loglevel, "OVERLAY="+ctrcfg.overlay, "LOGSIZEMAX="+ctrcfg.logsizemax)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (ctrcfg *ctrcfgDescription) checkCtrcfgParameters(oc *exutil.CLI) error {
	return wait.Poll(10*time.Minute, 11*time.Minute, func() (bool, error) {
		nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		node := strings.Fields(nodeName)

		for _, v := range node {
			nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", v), "-o=jsonpath={.status.conditions[3].type}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", v, nodeStatus)

			if nodeStatus == "Ready" {
				criostatus, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(`node/`+fmt.Sprintf("%s", v), "--", "chroot", "/host", "crio", "config").OutputToFile("crio.conf")
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("\nCRI-O PARAMETER ON THE WORKER NODE : %s", v)
				e2e.Logf("\ncrio config file path is  %v", criostatus)

				wait.Poll(2*time.Second, 1*time.Minute, func() (bool, error) {
					result, err1 := exec.Command("bash", "-c", "cat "+criostatus+" | egrep 'pids_limit|log_level'").Output()
					if err != nil {
						e2e.Failf("the result of ReadFile:%v", err1)
						return false, nil
					}
					e2e.Logf("\nCtrcfg Parameters is %s", result)
					if strings.Contains(string(result), "debug") && strings.Contains(string(result), "2048") {
						e2e.Logf("\nCtrcfg parameter pod limit and log_level configured successfully")
						return true, nil
					}
					return false, nil
				})
			} else {
				e2e.Logf("\n NODES ARE NOT READY\n ")
			}
		}
		return true, nil
	})
}

func (podTermination *podTerminationDescription) getTerminationGrace(oc *exutil.CLI) error {
	e2e.Logf("check terminationGracePeriodSeconds period")
	return wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
		nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", podTermination.namespace).Output()
		e2e.Logf("The nodename is %v", nodename)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeReadyBool, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", nodename), "-o=jsonpath={.status.conditions[?(@.reason=='KubeletReady')].status}").Output()
		e2e.Logf("The Node Ready status is %v", nodeReadyBool)
		o.Expect(err).NotTo(o.HaveOccurred())
		containerID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.containerStatuses[0].containerID}", "-n", podTermination.namespace).Output()
		e2e.Logf("The containerID is %v", containerID)
		o.Expect(err).NotTo(o.HaveOccurred())
		if nodeReadyBool == "True" {
			terminationGrace, err := compat_otp.DebugNodeWithChroot(oc, nodename, "systemctl", "show", containerID)
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(string(terminationGrace), "TimeoutStopUSec=1min 30s") {
				e2e.Logf("\nTERMINATION GRACE PERIOD IS SET CORRECTLY")
				return true, nil
			}
			e2e.Logf("\ntermination grace is NOT Updated")
			return false, nil
		}
		return false, nil
	})
}

func (podInitCon *podInitConDescription) containerExit(oc *exutil.CLI) error {
	return wait.Poll(2*time.Second, 2*time.Minute, func() (bool, error) {
		initConStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.initContainerStatuses[0].state.terminated.reason}", "-n", podInitCon.namespace).Output()
		e2e.Logf("The initContainer status is %v", initConStatus)
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(initConStatus), "Completed") {
			e2e.Logf("The initContainer exit normally")
			return true, nil
		}
		e2e.Logf("The initContainer not exit!")
		return false, nil
	})
}

func (podInitCon *podInitConDescription) deleteInitContainer(oc *exutil.CLI) (string, error) {
	nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", podInitCon.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	containerID, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.initContainerStatuses[0].containerID}", "-n", podInitCon.namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The containerID is %v", containerID)
	initContainerID := string(containerID)[8:]
	e2e.Logf("The initContainerID is %s", initContainerID)
	return compat_otp.DebugNodeWithChroot(oc, fmt.Sprintf("%s", nodename), "crictl", "rm", initContainerID)
}

func (podInitCon *podInitConDescription) initContainerNotRestart(oc *exutil.CLI) error {
	return wait.Poll(3*time.Minute, 6*time.Minute, func() (bool, error) {
		re := regexp.MustCompile("running")
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", podInitCon.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(string(podname), "-n", podInitCon.namespace, "--", "cat", "/mnt/data/test").Output()
		e2e.Logf("The /mnt/data/test: %s", output)
		o.Expect(err).NotTo(o.HaveOccurred())
		found := re.FindAllString(output, -1)
		if lenStr := len(found); lenStr > 1 {
			e2e.Logf("initContainer restart %d times.", (lenStr - 1))
			return false, nil
		} else if lenStr == 1 {
			e2e.Logf("initContainer not restart")
			return true, nil
		}
		return false, nil
	})
}

func checkNodeStatus(oc *exutil.CLI, nodeName string, expectedStatus string) {
	var expectedStatus1 string
	if expectedStatus == "Ready" {
		expectedStatus1 = "True"
	} else if expectedStatus == "NotReady" {
		expectedStatus1 = "Unknown"
	} else {
		err1 := fmt.Errorf("TBD supported node status")
		o.Expect(err1).NotTo(o.HaveOccurred())
	}
	err := wait.Poll(5*time.Second, 15*time.Minute, func() (bool, error) {
		statusOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodeName, "-ojsonpath={.status.conditions[-1].status}").Output()
		if err != nil {
			e2e.Logf("\nGet node status with error : %v", err)
			return false, nil
		}
		e2e.Logf("Expect Node %s in state %v, kubelet status is %s", nodeName, expectedStatus, statusOutput)
		if statusOutput != expectedStatus1 {
			return false, nil
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("Node %s is not in expected status %s", nodeName, expectedStatus))
}

func getSingleWorkerNode(oc *exutil.CLI) string {
	workerNodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[0].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nWorker Node Name is %v", workerNodeName)
	return workerNodeName
}

func getSingleMasterNode(oc *exutil.CLI) string {
	masterNodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/master=", "-o=jsonpath={.items[1].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nMaster Node Name is %v", masterNodeName)
	return masterNodeName
}

func getPodNodeName(oc *exutil.CLI, namespace string) string {
	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", namespace).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Pod Node Name is %v \n", nodeName)
	return nodeName
}

func getPodNetNs(oc *exutil.CLI, hostname string) (string, error) {
	NetNsStr, err := compat_otp.DebugNodeWithChroot(oc, hostname, "/bin/bash", "-c", "journalctl -u crio --since=\"5 minutes ago\" | grep pod-56266 | grep NetNS")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("NetNs string : %v", NetNsStr)
	keyword := "NetNS:[^\\s]*"
	re := regexp.MustCompile(keyword)
	found := re.FindAllString(NetNsStr, -1)
	if len(found) == 0 {
		e2e.Logf("can not find NetNS for pod")
		return "", fmt.Errorf("can not find NetNS for pod")
	}
	e2e.Logf("found : %v \n", found[0])
	NetNs := strings.Split(found[0], ":")
	e2e.Logf("NetNs : %v \n", NetNs[1])
	return NetNs[1], nil
}

func addLabelToResource(oc *exutil.CLI, label string, resourceName string, resource string) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args(resource, resourceName, label, "--overwrite").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nLabel Added")
}

func removeLabelFromNode(oc *exutil.CLI, label string, workerNodeName string, resource string) {
	_, err := oc.AsAdmin().WithoutNamespace().Run("label").Args(resource, workerNodeName, label).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\nLabel Removed")
}

func rebootNode(oc *exutil.CLI, workerNodeName string) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		e2e.Logf("\nRebooting node %s....", workerNodeName)
		_, err1 := compat_otp.DebugNodeWithChroot(oc, workerNodeName, "shutdown", "-r", "+1", "-t", "30")
		o.Expect(err1).NotTo(o.HaveOccurred())
		return true, nil
	})
}

func masterNodeLog(oc *exutil.CLI, masterNode string) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args(`node/`+masterNode, "--", "chroot", "/host", "journalctl", "-u", "crio").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !strings.Contains(status, "layer not known") {
			e2e.Logf("\nTest successfully executed")
		} else {
			e2e.Logf("\nTest fail executed, and try next")
			return false, nil
		}
		return true, nil
	})
}

func getmcpStatus(oc *exutil.CLI, nodeName string) error {
	return wait.Poll(60*time.Second, 15*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", nodeName, "-ojsonpath={.status.conditions[?(@.type=='Updating')].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\nCurrent mcp UPDATING Status is %s\n", status)
		if strings.Contains(status, "False") {
			e2e.Logf("\nmcp updated successfully ")
		} else {
			e2e.Logf("\nmcp is still in UPDATING state")
			return false, nil
		}
		return true, nil
	})
}

func getWorkerNodeDescribe(oc *exutil.CLI, workerNodeName string) error {
	return wait.Poll(3*time.Second, 1*time.Minute, func() (bool, error) {
		nodeStatus, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("node", workerNodeName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(nodeStatus, "EvictionThresholdMet") {
			e2e.Logf("\n WORKER NODE MET EVICTION THRESHOLD\n ")
		} else {
			e2e.Logf("\n WORKER NODE DO NOT HAVE MEMORY PRESSURE\n ")
			return false, nil
		}
		return true, nil
	})
}

func checkOverlaySize(oc *exutil.CLI, overlaySize string) error {
	return wait.Poll(3*time.Second, 1*time.Minute, func() (bool, error) {
		workerNode := getSingleWorkerNode(oc)
		//overlayString, err := compat_otp.DebugNodeWithChroot(oc, workerNode, "/bin/bash", "-c", "head -n 7 /etc/containers/storage.conf | grep size")
		overlayString, err := compat_otp.DebugNodeWithChroot(oc, workerNode, "/bin/bash", "-c", "head -n 7 /etc/containers/storage.conf | grep size || true")
		if err != nil {
			return false, err
		}
		e2e.Logf("overlaySize string : %v", overlayString)
		if strings.Contains(string(overlayString), overlaySize) {
			e2e.Logf("overlay size check successfully")
		} else {
			e2e.Logf("overlay size check failed")
			return false, nil
		}
		return true, nil
	})
}

func checkPodOverlaySize(oc *exutil.CLI, overlaySize string) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", oc.Namespace()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		overlayString, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", oc.Namespace(), podName, "/bin/bash", "-c", "df -h | grep overlay").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("overlayString is : %v", overlayString)
		overlaySizeStr := strings.Fields(string(overlayString))
		e2e.Logf("overlaySize : %s", overlaySizeStr[1])
		overlaySizeInt := strings.Split(string(overlaySizeStr[1]), ".")[0] + "G"
		e2e.Logf("overlaySizeInt : %s", overlaySizeInt)
		if overlaySizeInt == overlaySize {
			e2e.Logf("pod overlay size is correct")
		} else {
			e2e.Logf("pod overlay size is not correct !!!")
			return false, nil
		}
		return true, nil
	})
}

func checkNetNs(oc *exutil.CLI, hostname string, netNsPath string) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		result, _ := compat_otp.DebugNodeWithChroot(oc, hostname, "ls", "-l", netNsPath)
		e2e.Logf("the check result: %v", result)
		if strings.Contains(string(result), "No such file or directory") {
			e2e.Logf("the NetNS file is cleaned successfully")
		} else {
			e2e.Logf("the NetNS file still exist")
			return false, nil
		}
		return true, nil
	})
}

// this function check if crontab events include error like : MountVolume.SetUp failed for volume "serviceca" : object "openshift-image-registry"/"serviceca" not registered
func checkEventsForErr(oc *exutil.CLI) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		// get all cronjob's namespace from:
		// NAMESPACE                              NAME               SCHEDULE       SUSPEND   ACTIVE   LAST SCHEDULE   AGE
		// openshift-image-registry               image-pruner       0 0 * * *      False     0        <none>          4h36m
		// openshift-operator-lifecycle-manager   collect-profiles   */15 * * * *   False     0        9m11s           4h40m
		allcronjobs, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("cronjob", "--all-namespaces", "-o=jsonpath={range .items[*]}{@.metadata.namespace}{\"\\n\"}{end}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the cronjobs namespaces are: %v", allcronjobs)
		for _, s := range strings.Fields(allcronjobs) {
			events, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("events", "-n", s).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			keyword := "MountVolume.SetUp failed for volume.*object.*not registered"
			re := regexp.MustCompile(keyword)
			found := re.FindAllString(events, -1)
			if len(found) > 0 {
				e2e.Logf("The events of ns [%v] hit the error: %v", s, found[0])
				return false, nil
			}
		}
		e2e.Logf("all the crontab events don't hit the error: MountVolume.SetUp failed for volume ... not registered")
		return true, nil
	})
}

func cleanupObjectsClusterScope(oc *exutil.CLI, objs ...objectTableRefcscope) error {
	return wait.Poll(1*time.Second, 1*time.Second, func() (bool, error) {
		for _, v := range objs {
			e2e.Logf("\n Start to remove: %v", v)
			status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(v.kind, v.name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(status, "Error") {
				e2e.Logf("Error getting resources... Seems resources objects are already deleted. \n")
				return true, nil
			}
			_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(v.kind, v.name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		return true, nil
	})
}

func (podTwoContainers *podTwoContainersDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", podTwoContainers.template, "-p", "NAME="+podTwoContainers.name, "NAMESPACE="+podTwoContainers.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}
func (podTwoContainers *podTwoContainersDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", podTwoContainers.namespace, "pod", podTwoContainers.name).Execute()
}

func (podUserNS *podUserNSDescription) crioWorkloadConfigExist(oc *exutil.CLI) error {
	return wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		workloadString, _ := compat_otp.DebugNodeWithChroot(oc, nodename, "cat", "/etc/crio/crio.conf.d/00-default")
		//not handle err as a workaround of issue: debug container needs more time to start in 4.13&4.14
		//o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(workloadString), "crio.runtime.workloads.openshift-builder") && strings.Contains(string(workloadString), "io.kubernetes.cri-o.userns-mode") && strings.Contains(string(workloadString), "io.kubernetes.cri-o.Devices") {
			e2e.Logf("the crio workload exist in /etc/crio/crio.conf.d/00-default")
		} else {
			e2e.Logf("the crio workload not exist in /etc/crio/crio.conf.d/00-default")
			return false, nil
		}
		return true, nil
	})
}

func (podUserNS *podUserNSDescription) userContainersExistForNS(oc *exutil.CLI) error {
	return wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		xContainers, _ := compat_otp.DebugNodeWithChroot(oc, nodename, "bash", "-c", "cat /etc/subuid /etc/subgid")
		//not handle err as a workaround of issue: debug container needs more time to start in 4.13&4.14
		//o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Count(xContainers, "containers") == 2 {
			e2e.Logf("the user containers exist in /etc/subuid and /etc/subgid")
		} else {
			e2e.Logf("the user containers not exist in /etc/subuid and /etc/subgid")
			return false, nil
		}
		return true, nil
	})
}

func (podUserNS *podUserNSDescription) podRunInUserNS(oc *exutil.CLI) error {
	return wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", podUserNS.namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		idString, err := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", podUserNS.namespace, podName, "id").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(idString), "uid=0(root) gid=0(root) groups=0(root)") {
			e2e.Logf("the user id in pod is root")
			podUserNSstr, _ := oc.AsAdmin().WithoutNamespace().Run("rsh").Args("-n", podUserNS.namespace, podName, "lsns", "-o", "NS", "-t", "user").Output()
			//not handle err due to the container crash like: unable to upgrade connection: container not found
			//o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("string(podUserNS) is : %s", string(podUserNSstr))
			podNS := strings.Fields(string(podUserNSstr))
			e2e.Logf("pod user namespace : %s", podNS[1])

			nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", podUserNS.namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			nodeUserNS, _ := compat_otp.DebugNodeWithChroot(oc, string(nodename), "/bin/bash", "-c", "lsns -t user | grep /usr/lib/systemd/systemd")
			//not handle err as a workaround of issue: debug container needs more time to start in 4.13&4.14
			//o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("host user ns string : %v", nodeUserNS)
			nodeNSstr := strings.Split(string(nodeUserNS), "\n")
			nodeNS := strings.Fields(nodeNSstr[0])
			e2e.Logf("host user namespace : %s", nodeNS[0])
			if nodeNS[0] == podNS[1] {
				e2e.Logf("pod run in the same user namespace with host")
				return false, nil
			}
			e2e.Logf("pod run in different user namespace with host")
			return true, nil
		}
		e2e.Logf("the user id in pod is not root")
		return false, nil
	})
}

func configExist(oc *exutil.CLI, config []string, configPath string) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		configString, err := compat_otp.DebugNodeWithChroot(oc, nodename, "cat", configPath)
		e2e.Logf("the %s is: \n%v", configPath, configString)
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, conf := range config {
			if !strings.Contains(string(configString), conf) {
				e2e.Logf("the config: %s not exist in %s", conf, configPath)
				return false, nil
			}
		}
		e2e.Logf("all the config exist in %s", configPath)
		return true, nil
	})
}

func checkMachineConfigPoolStatus(oc *exutil.CLI, nodeSelector string) error {
	//when mcp master change cgroup from v2 to v1, it takes more than 15 minutes
	return wait.Poll(30*time.Second, 30*time.Minute, func() (bool, error) {
		mCount, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", nodeSelector, "-n", oc.Namespace(), "-o=jsonpath={.status.machineCount}").Output()
		unmCount, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", nodeSelector, "-n", oc.Namespace(), "-o=jsonpath={.status.unavailableMachineCount}").Output()
		dmCount, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", nodeSelector, "-n", oc.Namespace(), "-o=jsonpath={.status.degradedMachineCount}").Output()
		rmCount, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", nodeSelector, "-n", oc.Namespace(), "-o=jsonpath={.status.readyMachineCount}").Output()
		e2e.Logf("MachineCount:%v unavailableMachineCount:%v degradedMachineCount:%v ReadyMachineCount:%v", mCount, unmCount, dmCount, rmCount)
		if strings.Compare(mCount, rmCount) == 0 && strings.Compare(unmCount, dmCount) == 0 {
			return true, nil
		}
		return false, nil
	})
}

// this func check the pod's cpu setting override the host default
func overrideWkloadCPU(oc *exutil.CLI, cpuset string, namespace string) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cpuSet, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(string(podname), "-n", namespace, "--", "cat", "/sys/fs/cgroup/cpuset.cpus.effective").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The cpuset is : %s", cpuSet)
		if cpuset == "" {
			//if cpuset == "", the pod will keep default value as the /sys/fs/cgroup/cpuset.cpus.effective on node
			nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", namespace).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The nodename is %v", nodename)
			cpusetDeft, err := compat_otp.DebugNodeWithChroot(oc, nodename, "cat", "/sys/fs/cgroup/cpuset.cpus.effective")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("The cpuset of host is : %s", cpusetDeft)
			if strings.Contains(cpusetDeft, cpuSet) {
				e2e.Logf("the cpuset not take effect in the pod")
				return true, nil
			}
		} else if cpuSet == cpuset {
			e2e.Logf("the pod override the default workload setting")
			return true, nil
		}
		return false, nil
	})
}

// this func check the pod's cpu setting keep the same as host default
func defaultWkloadCPU(oc *exutil.CLI, cpuset string, namespace string) error {
	return wait.Poll(1*time.Second, 3*time.Second, func() (bool, error) {
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cpuSet, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(string(podname), "-n", namespace, "--", "cat", "/sys/fs/cgroup/cpuset.cpus.effective").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The cpuset of pod is : %s", cpuSet)

		nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The nodename is %v", nodename)
		cpusetDeft, err := compat_otp.DebugNodeWithChroot(oc, nodename, "cat", "/sys/fs/cgroup/cpuset.cpus.effective")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("The cpuset of host is : %s", cpusetDeft)

		if strings.Contains(cpusetDeft, cpuSet) {
			if string(cpuSet) != cpuset {
				e2e.Logf("the pod keep the default workload setting")
				return true, nil
			}
			e2e.Logf("the pod specified value is the same as default, invalid test!")
			return false, nil
		}
		return false, nil
	})
}

// this function create CMA(Keda) operator
func createKedaOperator(oc *exutil.CLI) {
	buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
	operatorGroup := filepath.Join(buildPruningBaseDir, "operatorgroup.yaml")
	subscription := filepath.Join(buildPruningBaseDir, "subscription.yaml")
	nsOperator := filepath.Join(buildPruningBaseDir, "ns-keda-operator.yaml")
	operatorNamespace := "openshift-keda"

	msg, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", nsOperator).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", operatorGroup).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", subscription).Output()
	e2e.Logf("err %v, msg %v", err, msg)

	// checking subscription status
	errCheck := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		subState, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "openshift-custom-metrics-autoscaler-operator", "-n", operatorNamespace, "-o=jsonpath={.status.state}").Output()
		//o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(subState, "AtLatestKnown") == 0 {
			msg, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "openshift-custom-metrics-autoscaler-operator", "-n", operatorNamespace, "--no-headers").Output()
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("subscription openshift-custom-metrics-autoscaler-operator is not correct status"))

	// checking csv status
	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "openshift-custom-metrics-autoscaler-operator", "-n", operatorNamespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(csvName).NotTo(o.BeEmpty())
	errCheck = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		csvState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", operatorNamespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(csvState, "Succeeded") == 0 {
			e2e.Logf("CSV check complete!!!")
			return true, nil
		}
		return false, nil

	})
	compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("csv %v is not correct status", csvName))
}
func waitForPodWithLabelReady(oc *exutil.CLI, ns, label string) error {
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", label, "-ojsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}").Output()
		e2e.Logf("the Ready status of pod is %v", status)
		if err != nil || status == "" {
			e2e.Logf("failed to get pod status: %v, retrying...", err)
			return false, nil
		}
		if strings.Contains(status, "False") {
			e2e.Logf("the pod Ready status not met; wanted True but got %v, retrying...", status)
			return false, nil
		}
		return true, nil
	})
}

// this function is for check kubelet_log_level
func assertKubeletLogLevel(oc *exutil.CLI) {
	var kubeservice string
	var kublet string
	var err error
	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(nodeErr).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		nodes := strings.Fields(nodeName)

		for _, node := range nodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				kubeservice, err = compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "systemctl show kubelet.service | grep KUBELET_LOG_LEVEL")
				o.Expect(err).NotTo(o.HaveOccurred())
				kublet, err = compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "ps aux | grep kubelet")
				o.Expect(err).NotTo(o.HaveOccurred())

				if strings.Contains(string(kubeservice), "KUBELET_LOG_LEVEL") && strings.Contains(string(kublet), "--v=2") {
					e2e.Logf(" KUBELET_LOG_LEVEL is 2. \n")
					return true, nil
				}
				e2e.Logf(" KUBELET_LOG_LEVEL is not 2. \n")
				return false, nil
			}
			e2e.Logf("\n NODES ARE NOT READY\n ")
		}
		return false, nil
	})
	if waitErr != nil {
		e2e.Logf("Kubelet Log level is:\n %v\n", kubeservice)
		e2e.Logf("Running Proccess of kubelet are:\n %v\n", kublet)
	}
	compat_otp.AssertWaitPollNoErr(waitErr, "KUBELET_LOG_LEVEL is not expected")
}

// this function create VPA(Vertical Pod Autoscaler) operator
func createVpaOperator(oc *exutil.CLI) {
	buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
	operatorGroup := filepath.Join(buildPruningBaseDir, "vpa-operatorgroup.yaml")
	subscription := filepath.Join(buildPruningBaseDir, "vpa-subscription.yaml")
	nsOperator := filepath.Join(buildPruningBaseDir, "ns-vpa-operator.yaml")
	operatorNamespace := "openshift-vertical-pod-autoscaler"

	msg, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", nsOperator).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", operatorGroup).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", subscription).Output()
	e2e.Logf("err %v, msg %v", err, msg)

	// checking subscription status
	errCheck := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		subState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "vertical-pod-autoscaler", "-n", operatorNamespace, "-o=jsonpath={.status.state}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(subState, "AtLatestKnown") == 0 {
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("subscription vertical-pod-autoscaler is not correct status"))

	// checking csv status
	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "vertical-pod-autoscaler", "-n", operatorNamespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(csvName).NotTo(o.BeEmpty())
	errCheck = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		csvState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", operatorNamespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(csvState, "Succeeded") == 0 {
			e2e.Logf("CSV check complete!!!")
			return true, nil
		}
		return false, nil

	})
	compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("csv %v is not correct status", csvName))
}

// this function is for  updating the runtimeRequestTimeout parameter using KubeletConfig CR

func runTimeTimeout(oc *exutil.CLI) {
	var kubeletConf string
	var err error
	nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(nodeErr).NotTo(o.HaveOccurred())
	e2e.Logf("\nNode Names are %v", nodeName)
	nodes := strings.Fields(nodeName)

	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {

		for _, node := range nodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				kubeletConf, err = compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "cat /etc/kubernetes/kubelet.conf | grep runtimeRequestTimeout")
				o.Expect(err).NotTo(o.HaveOccurred())

				if strings.Contains(string(kubeletConf), "runtimeRequestTimeout") && strings.Contains(string(kubeletConf), ":") && strings.Contains(string(kubeletConf), "3m0s") {
					e2e.Logf(" RunTime Request Timeout is 3 minutes. \n")
					return true, nil
				}
				e2e.Logf("Runtime Request Timeout is not 3 minutes. \n")
				return false, nil
			}
			e2e.Logf("\n NODES ARE NOT READY\n ")
		}
		return false, nil
	})
	if waitErr != nil {
		e2e.Logf("RunTime Request Timeout is:\n %v\n", kubeletConf)

	}
	compat_otp.AssertWaitPollNoErr(waitErr, "Runtime Request Timeout is not expected")
}

func checkConmonForAllNode(oc *exutil.CLI) {
	var configStr string
	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(nodeErr).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		nodes := strings.Fields(nodeName)

		for _, node := range nodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode [%s] Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				configStr, err := compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "crio config | grep 'conmon = \"\"'")
				o.Expect(err).NotTo(o.HaveOccurred())

				if strings.Contains(string(configStr), "conmon = \"\"") {
					e2e.Logf(" conmon check pass. \n")
				} else {
					e2e.Logf(" conmon check failed. \n")
					return false, nil
				}
			} else {
				e2e.Logf("\n NODES ARE NOT READY\n ")
				return false, nil
			}
		}
		return true, nil
	})

	if waitErr != nil {
		e2e.Logf("conmon string is:\n %v\n", configStr)
	}
	compat_otp.AssertWaitPollNoErr(waitErr, "the conmon is not as expected!")
}

// waitClusterOperatorAvailable waits for all the Cluster Operator resources to be
// in Available state. This generic function can be used either after draining a node
// or after an upgrade.
func waitClusterOperatorAvailable(oc *exutil.CLI) {

	timeout := 120

	waitErr := wait.Poll(10*time.Second, time.Duration(timeout)*time.Minute, func() (bool, error) {
		availableCOStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusteroperator", "-o=jsonpath={.items[*].status.conditions[?(@.type==\"Available\")].status}").Output()
		if err != nil || strings.Contains(availableCOStatus, "False") {
			e2e.Logf("Some Cluster Operator is still Unavailable")
			return false, nil
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, fmt.Sprintf("Some cluster operator is still unavailable after %v seconds ...", timeout))
}

func checkUpgradeMachineConfig(oc *exutil.CLI) {

	var machineconfig string
	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		upgradestatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", "machine-config").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("\n Upgrade status is %s\n", upgradestatus)
		machineconfig, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc").Output()
		o.Expect(err1).NotTo(o.HaveOccurred())

		if strings.Contains(string(machineconfig), "99-worker-generated-kubelet-1") {
			re := regexp.MustCompile("99-worker-generated-kubelet")
			found := re.FindAllString(machineconfig, -1)
			lenstr := len(found)
			if lenstr == 2 {
				e2e.Logf("\n Upgrade happened successfully")
				return true, nil
			}
			e2e.Logf("\nError")
			return false, nil
		}
		e2e.Logf(" Upgarde has failed \n")
		return false, nil

	})
	if waitErr != nil {
		e2e.Logf("machine config is %s\n", machineconfig)
	}
	compat_otp.AssertWaitPollNoErr(waitErr, "the machine config is not as expected.")
}

// ProbeTerminatePeriod checks the termination period for a probe.
func ProbeTerminatePeriod(oc *exutil.CLI, terminatePeriod int, probeterminatePeriod int, podName string, namespace string, flag bool) {
	var terminate = 0
	if flag {
		terminate = probeterminatePeriod
	} else {
		terminate = terminatePeriod
	}
	e2e.Logf("terminate is: %v", terminate)

	waitErr := wait.Poll(10*time.Second, 4*time.Minute, func() (bool, error) {
		podDesc, err := oc.AsAdmin().WithoutNamespace().Run("describe").Args("pod", podName, "-n", namespace).OutputToFile("podDesc.txt")
		o.Expect(err).NotTo(o.HaveOccurred())

		probeFailT, _ := exec.Command("bash", "-c", "cat "+podDesc+" | grep \"Container.*failed.*probe, will be restarted\"").Output()
		conStartT, _ := exec.Command("bash", "-c", "cat "+podDesc+" | grep \"Started container test\" ").Output()
		e2e.Logf("probeFailT is: %v", string(probeFailT))
		e2e.Logf("conStartT is: %v", string(conStartT))
		if string(probeFailT) != "" && string(conStartT) != "" {
			// count probeFailT- conStartT between  [terminate-3, terminate+3]
			var time1 = strings.Fields(string(probeFailT))[2]
			var time2 = strings.Fields(string(conStartT))[2]
			var time1Min string
			var timeTemp string
			var time1Sec string
			var time1MinInt int
			var time1SecInt int
			if strings.Contains(time1, "m") {
				time1Min = strings.Split(time1, "m")[0]
				timeTemp = strings.Split(time1, "m")[1]
				time1MinInt, err = strconv.Atoi(time1Min)
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Contains(time1, "s") {
					time1Sec = strings.Split(timeTemp, "s")[0]
					time1SecInt, err = strconv.Atoi(time1Sec)
					o.Expect(err).NotTo(o.HaveOccurred())
				} else {
					time1Sec = "0"
					time1SecInt = 0
				}
			} else {
				time1Sec = strings.Split(time1, "s")[0]
				time1SecInt, err = strconv.Atoi(time1Sec)
				o.Expect(err).NotTo(o.HaveOccurred())
				time1MinInt = 0
			}
			e2e.Logf("time1Min:%v, timeTemp:%v, time1Sec:%v, time1MinInt:%v, time1SecInt:%v", time1Min, timeTemp, time1Sec, time1MinInt, time1SecInt)
			timeSec1 := time1MinInt*60 + time1SecInt
			e2e.Logf("timeSec1: %v ", timeSec1)

			var time2Min string
			var time2Sec string
			var time2MinInt int
			var time2SecInt int
			if strings.Contains(time2, "m") {
				time2Min = strings.Split(time2, "m")[0]
				timeTemp = strings.Split(time2, "m")[1]
				time2MinInt, err = strconv.Atoi(time2Min)
				o.Expect(err).NotTo(o.HaveOccurred())
				if strings.Contains(time2, "s") {
					time2Sec = strings.Split(timeTemp, "s")[0]
					time2SecInt, err = strconv.Atoi(time2Sec)
					o.Expect(err).NotTo(o.HaveOccurred())
				} else {
					time2Sec = "0"
					time2SecInt = 0
				}
			} else {
				time2Sec = strings.Split(time2, "s")[0]
				time2SecInt, err = strconv.Atoi(time2Sec)
				o.Expect(err).NotTo(o.HaveOccurred())
				time2MinInt = 0
			}
			e2e.Logf("time2Min:%v, time2Sec:%v, time2MinInt:%v, time2SecInt:%v", time2Min, time2Sec, time2MinInt, time2SecInt)
			timeSec2 := time2MinInt*60 + time2SecInt
			e2e.Logf("timeSec2: %v ", timeSec2)

			if ((timeSec1 - timeSec2) >= (terminate - 3)) && ((timeSec1 - timeSec2) <= (terminate + 3)) {
				e2e.Logf("terminationGracePeriod check pass")
				return true, nil
			}
			e2e.Logf("terminationGracePeriod check failed")
			return false, nil

		}
		e2e.Logf("not capture data")
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "probe terminationGracePeriod is not as expected!")
}

//this functionn is for  Install and verify Cluster Resource Override Admission Webhook

func installOperatorClusterresourceoverride(oc *exutil.CLI) {
	buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
	nsclusterresourceoperatorTemp := filepath.Join(buildPruningBaseDir, "ns-clusterresource-operator.yaml")
	croperatorgroupTemp := filepath.Join(buildPruningBaseDir, "cr-operatorgroup.yaml")
	crsubscriptionTemp := filepath.Join(buildPruningBaseDir, "cr-subscription.yaml")
	operatorNamespace := "clusterresourceoverride-operator"

	ns, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", nsclusterresourceoperatorTemp).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("/n Namespace status is %v", ns)
	og, err1 := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", croperatorgroupTemp).Output()
	o.Expect(err1).NotTo(o.HaveOccurred())
	e2e.Logf("/n Operator group status is %v", og)
	subscrip, err2 := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", crsubscriptionTemp).Output()
	o.Expect(err2).NotTo(o.HaveOccurred())
	e2e.Logf("/n Subscription status is %v", subscrip)

	errCheck := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		subscription, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "clusterresourceoverride", "-n", operatorNamespace, "-o=jsonpath={.status.state}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(subscription, "AtLatestKnown") == 0 {
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("subscription clusterresourceoverride is not in correct status"))

	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "clusterresourceoverride", "-n", operatorNamespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(csvName).NotTo(o.BeEmpty())
	errCheck = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		csvState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", operatorNamespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(csvState, "Succeeded") == 0 {
			e2e.Logf("CSV check complete!!!")
			return true, nil
		}
		return false, nil

	})
	compat_otp.AssertWaitPollNoErr(errCheck, fmt.Sprintf("csv %v is not correct status", csvName))

}

func createCRClusterresourceoverride(oc *exutil.CLI) {

	buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
	clusterresourceoverrideTemp := filepath.Join(buildPruningBaseDir, "clusterresource-override.yaml")
	cro, err3 := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", clusterresourceoverrideTemp).Output()
	o.Expect(err3).NotTo(o.HaveOccurred())
	e2e.Logf("/n Cluster Resource Overrride status is %v", cro)

}
func deleteAPIService(oc *exutil.CLI) {
	e2e.Logf("Deleting apiservice v1.admission.autoscaling.openshift.io to unblock other test cases")
	_, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("apiservice", "v1.admission.autoscaling.openshift.io").Output()
	if err != nil {
		e2e.Logf("Failed to delete apiservice: %v", err)
	} else {
		e2e.Logf("Successfully deleted apiservice v1.admission.autoscaling.openshift.io")
	}
}

//this function is to test config changes to Cluster Resource Override Webhook

func testCRClusterresourceoverride(oc *exutil.CLI) {

	patch := `[{"op": "replace", "path": "/spec/podResourceOverride/spec/cpuRequestToLimitPercent", "value":40},{"op": "replace", "path": "/spec/podResourceOverride/spec/limitCPUToMemoryPercent", "value":90},{"op": "replace", "path": "/spec/podResourceOverride/spec/memoryRequestToLimitPercent", "value":50}]`

	test, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("clusterresourceoverride.operator.autoscaling.openshift.io", "cluster", "--type=json", "-p", patch).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\n Parameters edited %v", test)
	o.Expect(strings.Contains(test, "clusterresourceoverride.operator.autoscaling.openshift.io/cluster patched")).To(o.BeTrue())

	cpuRequestToLimitPercent, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("ClusterResourceOverride", "cluster", "-o=jsonpath={.spec.podResourceOverride.spec.cpuRequestToLimitPercent}").Output()
	limitCPUToMemoryPercent, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("ClusterResourceOverride", "cluster", "-o=jsonpath={.spec.podResourceOverride.spec.limitCPUToMemoryPercent}").Output()
	memoryRequestToLimitPercent, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("ClusterResourceOverride", "cluster", "-o=jsonpath={.spec.podResourceOverride.spec.memoryRequestToLimitPercent}").Output()
	if cpuRequestToLimitPercent == "40" && limitCPUToMemoryPercent == "90" && memoryRequestToLimitPercent == "50" {
		e2e.Logf("Successfully updated the file")
	} else {
		e2e.Failf("Cluster resource overrides not updated successfully")
	}
}

func checkICSP(oc *exutil.CLI) bool {
	icsp, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ImageContentSourcePolicy").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(icsp, "No resources found") {
		e2e.Logf("there is no ImageContentSourcePolicy in this cluster")
		return false
	}
	return true
}

func checkIDMS(oc *exutil.CLI) bool {
	icsp, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ImageDigestMirrorSet").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(icsp, "No resources found") {
		e2e.Logf("there is no ImageDigestMirrorSet in this cluster")
		return false
	}
	return true
}

func checkICSPorIDMSorITMS(oc *exutil.CLI) bool {
	icsp, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ImageContentSourcePolicy").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	idms, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ImageDigestMirrorSet").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	itms, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("ImageTagMirrorSet").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(icsp, "No resources found") && strings.Contains(idms, "No resources found") && strings.Contains(itms, "No resources found") {
		e2e.Logf("there is no ImageContentSourcePolicy, ImageDigestMirrorSet and ImageTagMirrorSet in this cluster")
		return false
	}
	return true
}

func checkRegistryForIdms(oc *exutil.CLI) error {
	return wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		registry, _ := compat_otp.DebugNodeWithChroot(oc, nodename, "cat", "/etc/containers/registries.conf")
		//not handle err as a workaround of issue: debug container needs more time to start in 4.13&4.14
		//o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(registry), "registry.access.redhat.com/ubi9/ubi-minimal") && strings.Contains(string(registry), "example.io/example/ubi-minimal") && strings.Contains(string(registry), "example.com/example/ubi-minimal") && strings.Contains(string(registry), "pull-from-mirror = \"digest-only\"") && strings.Contains(string(registry), "location = \"registry.example.com/example\"") && strings.Contains(string(registry), "blocked = true") {
			e2e.Logf("ImageDigestMirrorSet apply successfully!")
		} else {
			e2e.Logf("ImageDigestMirrorSet apply failed!")
			return false, nil
		}
		return true, nil
	})
}

func checkImgSignature(oc *exutil.CLI) error {
	return wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodename := nodeList.Items[0].Name
		imgSig, _ := compat_otp.DebugNodeWithChroot(oc, nodename, "cat", "/etc/containers/policy.json")
		//not handle err as a workaround of issue: debug container needs more time to start in 4.13&4.14
		//o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(imgSig), "registry.access.redhat.com") && strings.Contains(string(imgSig), "signedBy") && strings.Contains(string(imgSig), "GPGKeys") && strings.Contains(string(imgSig), "/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release") && strings.Contains(string(imgSig), "registry.redhat.io") {
			e2e.Logf("Image signature verified pass!")
			return true, nil
		}
		e2e.Logf("Image signature verified failed!")
		return false, nil
	})
}

func checkCrun(oc *exutil.CLI) {
	var crunProc string
	var libcrun string
	nodeList, err := e2enode.GetReadySchedulableNodes(context.TODO(), oc.KubeFramework().ClientSet)
	o.Expect(err).NotTo(o.HaveOccurred())
	nodename := nodeList.Items[0].Name
	waitErr := wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		crunProc, _ = compat_otp.DebugNodeWithChroot(oc, nodename, "bash", "-c", "ps -aux | grep crun")
		libcrun, _ = compat_otp.DebugNodeWithChroot(oc, nodename, "bash", "-c", "systemctl status crio-$(sudo crictl ps -aq | head -n1).scope")
		if strings.Contains(string(crunProc), "root=/run/crun") && strings.Contains(string(libcrun), "libcrun") {
			e2e.Logf("crun is running!")
			return true, nil
		}
		e2e.Logf("crun is not running!")
		return false, nil
	})
	if waitErr != nil {
		e2e.Logf("crunProc is :\n%s\n", crunProc)
		e2e.Logf("libcrun is :\n%s\n", libcrun)
	}
	compat_otp.AssertWaitPollNoErr(waitErr, "crun check failed!")
}

// this function is for upgrade test to check SYSTEM_RESERVED_ES parameter is not empty

func parameterCheck(oc *exutil.CLI) {

	nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(nodeErr).NotTo(o.HaveOccurred())
	e2e.Logf("\nNode Names are %v", nodeName)
	nodes := strings.Fields(nodeName)

	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {

		for _, node := range nodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				sysreservedes, _ := compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "cat /etc/node-sizing.env")
				if strings.Contains(sysreservedes, "SYSTEM_RESERVED_ES=1Gi") {
					e2e.Logf("SYSTEM_RESERVED_ES default value is set. \n")

				} else {
					e2e.Logf("SYSTEM_RESERVED_ES default value has not been set. \n")
					return false, nil
				}
			} else {
				e2e.Logf("\n NODES ARE NOT READY\n")
				return false, nil
			}
		}
		return true, nil
	})
	if waitErr != nil {
		e2e.Logf("check failed")
	}
	compat_otp.AssertWaitPollNoErr(waitErr, "not default value")
}

func checkLogLink(oc *exutil.CLI, namespace string) {
	waitErr := wait.Poll(10*time.Second, 30*time.Second, func() (bool, error) {
		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].metadata.name}", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		log1, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(string(podname), "-n", namespace, "--", "cat", "/acme-logs/logs/httpd/0.log").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(log1, "httpd -D FOREGROUND") {
			e2e.Logf("log link successfully")
		} else {
			e2e.Logf("log link failed!")
			return false, nil
		}
		nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].spec.nodeName}", "-n", namespace).Output()
		e2e.Logf("The nodename is %v", nodename)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeReadyBool, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", fmt.Sprintf("%s", nodename), "-o=jsonpath={.status.conditions[?(@.reason=='KubeletReady')].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		podIP, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-o=jsonpath={.items[0].status.podIP}", "-n", namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if nodeReadyBool == "True" {
			output, err := compat_otp.DebugNodeWithChroot(oc, nodename, "curl", podIP)
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.Contains(output, "It works!") {
				e2e.Logf("curl successfully")
			} else {
				e2e.Logf("curl failed!")
				return false, nil
			}

		} else {
			e2e.Logf("NODES ARE NOT READY!")
		}

		log2, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(string(podname), "-n", namespace, "--", "cat", "/acme-logs/logs/httpd/0.log").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(log2, "\"GET / HTTP/1.1\" 200 45") {
			e2e.Logf("log link update successfully")
			return true, nil
		}
		e2e.Logf("log link update failed!")
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "check log link failed!")
}

// this function check Cpu Quota Disabled from container scope and pod cgroup
func checkCPUQuotaDisabled(oc *exutil.CLI, namespace string, podName string, cgroupV string) {
	waitErr := wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
		if cgroupV == "tmpfs" {
			e2e.Logf("the cgroup version is v1, not support in 4.16+")
		} else if cgroupV == "cgroup2fs" { // it's for cgroup v2
			out1, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", namespace, podName, "--", "/bin/sh", "-c", "cat /sys/fs/cgroup/cpu.stat | grep nr_throttled").Output()
			if err != nil {
				e2e.Logf("failed to check /sys/fs/cgroup/cpu.stat, error: %s ", err)
				return false, nil
			}
			o.Expect(strings.Contains(string(out1), "nr_throttled 0")).Should(o.BeTrue())
			out2, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", namespace, podName, "--", "/bin/sh", "-c", "cat /sys/fs/cgroup/cpu.max").Output()
			if err != nil {
				e2e.Logf("failed to check /sys/fs/cgroup/cpu.max, error: %s ", err)
				return false, nil
			}
			o.Expect(strings.Contains(string(out2), "max 100000")).Should(o.BeTrue())
			return true, nil
		} else {
			e2e.Logf("the cgroup version [%s] is valid", cgroupV)
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "check Cpu Quota Disabled failed!")
}

// this function check Cpu Load Balance Disabled from the pod's host dmesg log
func checkCPULoadBalanceDisabled(oc *exutil.CLI, namespace string, podName string) {
	waitErr := wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
		nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-o=jsonpath={.spec.nodeName}", "-n", namespace).Output()
		if err != nil {
			e2e.Logf("failed to get the pod's node name, error: %s ", err)
			return false, nil
		}
		out, err := compat_otp.DebugNodeWithChroot(oc, nodename, "/bin/bash", "-c", "dmesg | grep 'CPUs do not have asymmetric capacities'")
		if err != nil {
			e2e.Logf("failed to check CPUs asymmetric capacities, error: %s ", err)
			return false, nil
		}
		//For CPU, we set reserved: 1-4, isolated: 0,5-7;
		//If cpu 0 is load balance disabled, the log show [rd 1-7: CPUs do not have asymmetric capacities]
		//If cpu 0 and cpu 5 are load balance disabled, the log show [rd 1-4,6-7: CPUs do not have asymmetric capacities]
		//As long as any cpu is load balance disabled, the log won't be [rd 0-7: CPUs do not have asymmetric capacities]
		//If the pod doesn't include annotation "cpu-load-balancing.crio.io: "disable"", the log won't appear [CPUs do not have asymmetric capacities]
		o.Expect(strings.Contains(string(out), "CPUs do not have asymmetric capacities")).Should(o.BeTrue())
		o.Expect(out).ShouldNot(o.ContainSubstring("rd 0-7: CPUs do not have asymmetric capacities"))
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "check Cpu Quota Disabled failed!")
}

// this function can show cpu schedule info in dmesg log, flag="0" : turn on / flag="1" : turn off
func dmesgTurnOnCPU(oc *exutil.CLI, flag string) {
	nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(nodeErr).NotTo(o.HaveOccurred())
	e2e.Logf("\nNode Names are %v", nodeName)
	nodeList := strings.Fields(nodeName)

	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		for _, node := range nodeList {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			if statusErr != nil {
				e2e.Logf("failed to get node status, error: %s ", statusErr)
				return false, nil
			}
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus != "True" {
				e2e.Logf("\n NODES ARE NOT READY\n")
				return false, nil
			}

			switch flag {
			case "0":
				_, err := compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "echo Y > /sys/kernel/debug/sched/verbose")
				if err != nil {
					e2e.Logf("\n failed to set Y to CPU, error: %v ", err)
					return false, nil
				}
			case "1":
				_, err := compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "echo N > /sys/kernel/debug/sched/verbose")
				if err != nil {
					e2e.Logf("\n failed to set N to CPU, error: %v ", err)
					return false, nil
				}
			default:
				e2e.Logf("\n switch flag [%s] is invalid", flag)
				return false, nil
			}
		}
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "dmesg set cpu log failed!")
}

// this function create KedaController from template for CMA
func (cmaKedaController *cmaKedaControllerDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", cmaKedaController.template, "-p", "LEVEL="+cmaKedaController.level, "NAMESPACE="+cmaKedaController.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	waitForDeploymentPodsToBeReady(oc, "openshift-keda", "keda-metrics-apiserver")
}

// this function delete KedaController for CMA
func (cmaKedaController *cmaKedaControllerDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", cmaKedaController.namespace, "KedaController", cmaKedaController.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (pvcKedaController *pvcKedaControllerDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", pvcKedaController.template, "-p", "LEVEL="+pvcKedaController.level, "NAMESPACE="+pvcKedaController.namespace, "WATCHNAMESPACE="+pvcKedaController.watchNamespace)
	o.Expect(err).NotTo(o.HaveOccurred())
	waitForDeploymentPodsToBeReady(oc, "openshift-keda", "keda-metrics-apiserver")
}

func (pvcKedaController *pvcKedaControllerDescription) delete(oc *exutil.CLI) {
	err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", pvcKedaController.namespace, "KedaController", pvcKedaController.name).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitPodReady(oc *exutil.CLI, ns string, label string) {
	podNameList := getPodNameByLabel(oc, ns, label)
	compat_otp.AssertPodToBeReady(oc, podNameList[0], ns)
}

func getPodNameByLabel(oc *exutil.CLI, namespace string, label string) []string {
	var podName []string
	podNameAll, err := oc.AsAdmin().Run("get").Args("-n", namespace, "pod", "-l", label, "-ojsonpath={.items..metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podName = strings.Split(podNameAll, " ")
	e2e.Logf("The pod(s) are  %v ", podName)
	return podName
}

// WaitForDeploymentPodsToBeReady waits for the specific deployment to be ready
func waitForDeploymentPodsToBeReady(oc *exutil.CLI, namespace string, name string) {
	var selectors map[string]string
	err := wait.Poll(5*time.Second, 180*time.Second, func() (done bool, err error) {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of deployment/%s\n", name)
				return false, nil
			}
			return false, err
		}
		selectors = deployment.Spec.Selector.MatchLabels
		if deployment.Status.AvailableReplicas == *deployment.Spec.Replicas && deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas {
			e2e.Logf("Deployment %s available (%d/%d)\n", name, deployment.Status.AvailableReplicas, *deployment.Spec.Replicas)
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s deployment (%d/%d)\n", name, deployment.Status.AvailableReplicas, *deployment.Spec.Replicas)
		return false, nil
	})
	if err != nil && len(selectors) > 0 {
		var labels []string
		for k, v := range selectors {
			labels = append(labels, k+"="+v)
		}
		label := strings.Join(labels, ",")
		podStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", label, "-ojsonpath={.items[].status.conditions}").Output()
		containerStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", label, "-ojsonpath={.items[].status.containerStatuses}").Output()
		e2e.Failf("deployment %s is not ready:\nconditions: %s\ncontainer status: %s", name, podStatus, containerStatus)
	}
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("deployment %s is not available", name))
}

// Poll to wait for kafka to be ready
func waitForKafkaReady(oc *exutil.CLI, kafkaName string, kafkaNS string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		command := []string{"kafka.kafka.strimzi.io", kafkaName, "-n", kafkaNS, `-o=jsonpath={.status.conditions[*].type}`}
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(command...).Output()
		if err != nil {
			e2e.Logf("kafka status ready error: %v", err)
			return false, err
		}
		if output == "Ready" || output == "Warning Ready" || output == "Warning Warning Warning Warning Ready" {
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("resource kafka/%s did not appear", kafkaName))
}

// Poll to wait for kafka Topic to be ready
func waitForKafkaTopicReady(oc *exutil.CLI, kafkaTopicName string, kafkaTopicNS string) {
	err := wait.Poll(3*time.Second, 180*time.Second, func() (done bool, err error) {
		command := []string{"kafkaTopic", kafkaTopicName, "-n", kafkaTopicNS, `-o=jsonpath='{.status.conditions[*].type}'`}
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(command...).Output()
		if err != nil {
			e2e.Logf("kafka Topic status ready error: %v", err)
			return false, err
		}
		status := strings.Replace(output, "'", "", 2)
		e2e.Logf("Waiting for kafka status %s", status)
		if status == "Ready" {
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("resource kafkaTopic/%s did not appear", kafkaTopicName))
}

// this function uninstall AMQ operator
func removeAmqOperator(oc *exutil.CLI) {
	operatorNamespace := "kafka-52384"
	msg, err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", operatorNamespace, "sub", "amq-streams").Output()
	if err != nil {
		e2e.Logf("%v", msg)

	}
	_ = oc.AsAdmin().WithoutNamespace().Run("delete").Args("-n", operatorNamespace, "csv", "-l", "operators.coreos.com/amq-streams.openshift-operators").Execute()
}

// this function create AMQ operator
func createAmqOperator(oc *exutil.CLI) {
	buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
	subscription := filepath.Join(buildPruningBaseDir, "amq-sub.yaml")
	operatorNamespace := "kafka-52384"
	operatorGroupFile := filepath.Join(buildPruningBaseDir, "amq-operatorgroup-52384.yaml")
	msg, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", operatorGroupFile).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", subscription).Output()
	e2e.Logf("err %v, msg %v", err, msg)

	// checking subscription status
	errCheck := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		subState, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "amq-streams", "-n", operatorNamespace, "-o=jsonpath={.status.state}").Output()
		//o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(subState, "AtLatestKnown") == 0 {
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(errCheck, "subscription amq-streams is not correct status")

	// checking csv status
	csvName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", "amq-streams", "-n", operatorNamespace, "-o=jsonpath={.status.installedCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(csvName).NotTo(o.BeEmpty())
	errCheck = wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		csvState, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("csv", csvName, "-n", operatorNamespace, "-o=jsonpath={.status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Compare(csvState, "Succeeded") == 0 {
			e2e.Logf("CSV check complete!!!")
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(errCheck, "subscription amq-streams is not correct status")
}
func createProject(oc *exutil.CLI, namespace string) {
	oc.CreateSpecifiedNamespaceAsAdmin(namespace)
	/* turn off the automatic label synchronization required for PodSecurity admission
	   set pods security profile to privileged. See
	   https://kubernetes.io/docs/concepts/security/pod-security-admission/#pod-security-levels */
	err := compat_otp.SetNamespacePrivileged(oc, namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

// this function delete a workspace, we intend to do it after each test case run
func deleteProject(oc *exutil.CLI, namespace string) {
	oc.DeleteSpecifiedNamespaceAsAdmin(namespace)
}

func (triggerAuthentication *triggerAuthenticationDescription) create(oc *exutil.CLI) {
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", triggerAuthentication.template, "-p", "SECRET_NAME="+triggerAuthentication.secretname, "NAMESPACE="+triggerAuthentication.namespace)
	o.Expect(err).NotTo(o.HaveOccurred())
}

//this function is to  [MCO] change container registry config

func checkImageConfigUpdatedAsExpected(oc *exutil.CLI) {

	buildPruningBaseDir := compat_otp.FixturePath("testdata", "node")
	ImageconfigContTemp := filepath.Join(buildPruningBaseDir, "image-config.json")
	currentResourceVersion, getRvErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("image.config", "cluster", "-ojsonpath={.metadata.resourceVersion}").Output()
	o.Expect(getRvErr).NotTo(o.HaveOccurred())

	if currentResourceVersion != "" {
		testImageConfigJSONByte, readFileErr := ioutil.ReadFile(ImageconfigContTemp)
		o.Expect(readFileErr).NotTo(o.HaveOccurred())
		testImageConfigJSON, err := sjson.Set(string(testImageConfigJSONByte), `metadata.resourceVersion`, currentResourceVersion)
		o.Expect(err).NotTo(o.HaveOccurred())
		path := filepath.Join(e2e.TestContext.OutputDir, "new-imageConfig"+"-"+getRandomString()+".json")
		o.Expect(ioutil.WriteFile(path, pretty.Pretty([]byte(testImageConfigJSON)), 0644)).NotTo(o.HaveOccurred())
		e2e.Logf("The new ImageConfig is %s", path)
		ImageconfigContTemp = path
	}

	imgfile, err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", ImageconfigContTemp).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("\n Image File status is %v", imgfile)

	//for checking machine config

	waitErr0 := wait.Poll(30*time.Second, 1*time.Minute, func() (bool, error) {
		mc, err1 := oc.AsAdmin().WithoutNamespace().Run("get").Args("mc", "--sort-by=metadata.creationTimestamp").Output()
		o.Expect(err1).NotTo(o.HaveOccurred())
		e2e.Logf("\n Machine configs are:\n %s", mc)
		oc.NotShowInfo()
		if strings.Contains(string(mc), "rendered") {
			e2e.Logf(" New render configs are generated. \n")
			return true, nil
		}
		e2e.Logf(" New render configs are not generated. \n")
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr0, "New Renders are not expected")

	//waiting for mcp to get updated

	compat_otp.By("Check mcp finish rolling out")
	oc.NotShowInfo()
	mcpName := "worker"
	mcpName2 := "master"
	err3 := checkMachineConfigPoolStatus(oc, mcpName)
	compat_otp.AssertWaitPollNoErr(err3, "macineconfigpool worker update failed")
	err4 := checkMachineConfigPoolStatus(oc, mcpName2)
	compat_otp.AssertWaitPollNoErr(err4, "macineconfigpool master update failed")

	//for checking machine config pool

	mcp, err2 := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp").Output()
	o.Expect(err2).NotTo(o.HaveOccurred())
	e2e.Logf("\n Machine config pools are:\n %s", mcp)

	nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(nodeErr).NotTo(o.HaveOccurred())
	e2e.Logf("\nNode Names are %v", nodeName)
	nodes := strings.Fields(nodeName)

	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {

		nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", nodes[0], "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
		o.Expect(statusErr).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode %s Status is %s\n", nodes[0], nodeStatus)

		if nodeStatus == "True" {
			registrieslist, err4 := compat_otp.DebugNodeWithChroot(oc, nodes[0], "cat", "/etc/containers/registries.conf.d/01-image-searchRegistries.conf")
			o.Expect(err4).NotTo(o.HaveOccurred())
			e2e.Logf("\nImage Registry list is %v", registrieslist)

			o.Expect(strings.TrimSpace(registrieslist)).NotTo(o.BeEmpty())
			if strings.Contains((registrieslist), "qe.quay.io") {
				e2e.Logf(" Configuration has been changed successfully. \n")
				return true, nil
			}
			e2e.Logf(" Changes has not been made. \n")
			return false, nil
		}
		e2e.Logf("\n NODES ARE NOT READY\n ")
		return false, nil
	})

	compat_otp.AssertWaitPollNoErr(waitErr, "Registry List is not expected")

}

func createImageConfigWIthExportJSON(oc *exutil.CLI, originImageConfigJSON string) {
	var (
		err              error
		finalJSONContent string
	)

	currentResourceVersion, getRvErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("image.config", "cluster", "-ojsonpath={.metadata.resourceVersion}").Output()
	o.Expect(getRvErr).NotTo(o.HaveOccurred())
	finalJSONContent, err = sjson.Set(originImageConfigJSON, `metadata.resourceVersion`, currentResourceVersion)
	o.Expect(err).NotTo(o.HaveOccurred())
	path := filepath.Join(e2e.TestContext.OutputDir, "restored-imageConfig"+"-"+getRandomString()+".json")
	o.Expect(ioutil.WriteFile(path, pretty.Pretty([]byte(finalJSONContent)), 0644)).NotTo(o.HaveOccurred())
	e2e.Logf("The restored ImageConfig is %s", path)
	_, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", path).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("The ImageConfig restored successfully")

}
func waitCoBecomes(oc *exutil.CLI, coName string, waitTime int, expectedStatus map[string]string) error {
	return wait.Poll(5*time.Second, time.Duration(waitTime)*time.Second, func() (bool, error) {
		gottenStatus := getCoStatus(oc, coName, expectedStatus)
		eq := reflect.DeepEqual(expectedStatus, gottenStatus)
		if eq {
			eq := reflect.DeepEqual(expectedStatus, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"})
			if eq {
				// For True False False, we want to wait some bit more time and double check, to ensure it is stably healthy
				time.Sleep(100 * time.Second)
				gottenStatus := getCoStatus(oc, coName, expectedStatus)
				eq := reflect.DeepEqual(expectedStatus, gottenStatus)
				if eq {
					e2e.Logf("Given operator %s becomes available/non-progressing/non-degraded", coName)
					return true, nil
				}
			} else {
				e2e.Logf("Given operator %s becomes %s", coName, gottenStatus)
				return true, nil
			}
		}
		return false, nil
	})
}
func getCoStatus(oc *exutil.CLI, coName string, statusToCompare map[string]string) map[string]string {
	newStatusToCompare := make(map[string]string)
	for key := range statusToCompare {
		args := fmt.Sprintf(`-o=jsonpath={.status.conditions[?(.type == '%s')].status}`, key)
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("co", args, coName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		newStatusToCompare[key] = status
	}
	return newStatusToCompare
}

// this function is to check qe-app-registry exists if not then use redhat-operators else skip the testcase
func (sub *subscriptionDescription) skipMissingCatalogsources(oc *exutil.CLI) {
	output, errQeReg := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "catalogsource", "qe-app-registry").Output()
	if errQeReg != nil && strings.Contains(output, "NotFound") {
		output, errRed := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "catalogsource", "redhat-operators").Output()
		if errRed != nil && strings.Contains(output, "NotFound") {
			g.Skip("Skip since catalogsources not available")
		} else {
			o.Expect(errRed).NotTo(o.HaveOccurred())
		}
		sub.catalogSourceName = "redhat-operators"
	} else {
		o.Expect(errQeReg).NotTo(o.HaveOccurred())
	}
}

// this function check sigstore signature verified from crio log
func checkSigstoreVerified(oc *exutil.CLI, namespace string, podName string, image string, dockerNs string) {
	waitErr := wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
		nodename, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-o=jsonpath={.spec.nodeName}", "-n", namespace).Output()
		if err != nil {
			e2e.Logf("failed to get the pod's node name, error: %s ", err)
			return false, nil
		}
		out, err := compat_otp.DebugNodeWithChroot(oc, nodename, "/bin/bash", "-c", "journalctl -u crio --since=\"5 minutes ago\"")
		if err != nil {
			e2e.Logf("failed to get crio log, error: %s ", err)
			return false, nil
		}

		o.Expect(strings.Contains(string(out), "Looking for sigstore attachments in "+image)).Should(o.BeTrue()) //need uncomment
		//for dockerNs, for example:
		//docker.io ~ docker.io/lyman9966/rhel8
		//quay.io/openshift-release-dev/ocp-release ~ quay.io/openshift-release-dev/ocp-release@sha256:c17d4489c1b283ee71c76dda559e66a546e16b208a57eb156ef38fb30098903a
		o.Expect(strings.Contains(string(out), "Sigstore attachments: using \\\"docker\\\" namespace "+dockerNs)).Should(o.BeTrue()) //need uncomment
		o.Expect(strings.Contains(string(out), "Found a sigstore attachment manifest with 1 layers")).Should(o.BeTrue())
		o.Expect(strings.Contains(string(out), "Fetching sigstore attachment")).Should(o.BeTrue())
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "check sigstore signature failed!")
}

// ExecCommandOnPod executes a command on a pod and returns the output.
func ExecCommandOnPod(oc *exutil.CLI, podname string, namespace string, command string) string {
	var podOutput string
	var execpodErr error
	errExec := wait.PollUntilContextTimeout(context.Background(), 15*time.Second, 300*time.Second, false, func(cxt context.Context) (bool, error) {
		podOutput, execpodErr = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", namespace, podname, "--", "/bin/sh", "-c", command).Output()
		podOutput = strings.TrimSpace(podOutput)
		if execpodErr != nil {
			return false, nil
		}
		return true, nil

	})
	if errExec != nil {
		e2e.Logf("Run commands %q on pod %q failed of:  %v, output is: %s", command, podname, execpodErr, podOutput)
	}
	compat_otp.AssertWaitPollNoErr(errExec, fmt.Sprintf("Run commands %q on pod %q failed", command, podname))
	return podOutput
}

// this function return the cpu affinity of a pod
func getCPUAffinityFromPod(oc *exutil.CLI, namespace string, podname string) string {
	cpuOut, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(podname, "-n", namespace, "--", "/bin/sh", "-c", "cat /proc/self/status | grep Cpus_allowed_list").Output() // Cpus_allowed_list:	1,3
	o.Expect(err).NotTo(o.HaveOccurred())
	cpustr := strings.Split(cpuOut, ":")[1]
	cpuAffinity := strings.TrimSpace(cpustr)
	e2e.Logf("The cpu affinity is: %v", cpuAffinity)
	return cpuAffinity
}

func getCPUAffinityFromCmd(oc *exutil.CLI, pid string, nodeName string) string {
	tsksetCmd := fmt.Sprintf(`taskset -pc %v`, pid)
	cpuAffinityOut, err := compat_otp.DebugNodeWithOptionsAndChroot(oc, nodeName, []string{"-q"}, "bash", "-c", tsksetCmd) //pid 2535's current affinity list: 0-3
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("command of `taskset -pc %v` return: %v", pid, cpuAffinityOut)
	cpuAffinity := strings.Split(cpuAffinityOut, ":")[1]
	cpuAffinity = strings.TrimSpace(cpuAffinity)
	return cpuAffinity
}

func getPid(oc *exutil.CLI, podName string, namespace string, nodeName string) string {
	containerIDString, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.status.containerStatuses[0].containerID}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	containerID := strings.Split(containerIDString, "//")[1] // cri-o://98d6bb3c6dbc367571d8cf4e50943184835f298b195361130cd98da4612c3b3b
	e2e.Logf("containerID is %v", containerID)
	getPidCmd := fmt.Sprintf(`crictl inspect %v | grep -E \"pid\":`, containerID) // "pid": 2535,
	pidOut, err := compat_otp.DebugNodeWithOptionsAndChroot(oc, nodeName, []string{"-q"}, "bash", "-c", getPidCmd)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("pidOut is %v", pidOut)
	pidMid := strings.Split(pidOut, ":")[1]
	pid := strings.Split(pidMid, ",")[0]
	pid = strings.TrimSpace(pid)
	return pid
}

// this function check the cpu affinity of burstable pod
// coreNum means the number of cpu cores in the node
// guCore means the cpu core sequence that consumed by guranteed pod, when assigned "false" means no cpu core consumed by gu pod
func checkCPUAffinityBurst(oc *exutil.CLI, podName string, namespace string, nodeName string, coreNum int, guCore string) {
	pid := getPid(oc, podName, namespace, nodeName)
	burstCore := getCPUAffinityFromCmd(oc, pid, nodeName)
	allCPU := "0-" + strconv.Itoa(coreNum-1)

	if guCore == "false" {
		o.Expect(burstCore == allCPU).To(o.BeTrue(), fmt.Sprintf("test failed: burstCore != allCPU // guCore is [%v], burstable core is [%v], cpu_num is [%v]", guCore, burstCore, coreNum))
		e2e.Logf("verify pass: burstCore == allCPU // guCore is [%v], burstable core is [%v], cpu_num is [%v]", guCore, burstCore, coreNum)
	} else {
		burstout := getDiffSet(oc, coreNum, guCore)
		e2e.Logf("The diff set of [allCPU - guCore] is: %v", burstout)
		o.Expect(burstCore == burstout).To(o.BeTrue(), fmt.Sprintf("test failed: burstCore != allCPU - guCore // burstable core is [%v], guCore is [%v], cpu_num is [%v]", burstout, guCore, coreNum))
		e2e.Logf("verify pass: burstCore = allCPU - guCore // burstable core is [%v], guCore is [%v], cpu_num is [%v]", burstout, guCore, coreNum)
	}
	checkCPUInterfaceFile(oc, pid, burstCore, nodeName)
}

func checkCPUInterfaceFile(oc *exutil.CLI, pid string, cpuAffinity string, nodeName string) {
	/*
	   "pid": 78162
	   cat /proc/78162/cgroup ==  0::/kubepods.slice/kubepods-pod7c259501_a249_479d_9280_621dcd56bc41.slice/crio-0f5e79c7110c8b6d767373b6b6defd4f9c284247a3ee205204f9e250be95fec1.scope/container
	   cat /sys/fs/cgroup/kubepods.slice/kubepods-pod7c259501_a249_479d_9280_621dcd56bc41.slice/crio-0f5e79c7110c8b6d767373b6b6defd4f9c284247a3ee205204f9e250be95fec1.scope/cpuset.cpus.effective == 0-3
	   cat /sys/fs/cgroup/kubepods.slice/kubepods-pod7c259501_a249_479d_9280_621dcd56bc41.slice/crio-0f5e79c7110c8b6d767373b6b6defd4f9c284247a3ee205204f9e250be95fec1.scope/cpuset.cpus ==  0-3
	*/
	getCgroupCmd := fmt.Sprintf(`cat /proc/%v/cgroup`, pid)
	cgroupOut, err := compat_otp.DebugNodeWithOptionsAndChroot(oc, nodeName, []string{"-q"}, "bash", "-c", getCgroupCmd)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the command of `cat /proc/%v/cgroup` return: %v", pid, cgroupOut)

	cgroupStr := strings.Split(cgroupOut, "::")[1]
	e2e.Logf("cgroupStr is: %v", cgroupStr)
	cgroup := strings.TrimSpace(cgroupStr)
	cgroup = strings.Trim(cgroup, "container") //4.18
	//cgroup = cgroup + "/" //4.17
	e2e.Logf("cgroup is: %v", cgroup)
	cpuEffectiveCmd := fmt.Sprintf(`cat /sys/fs/cgroup%vcpuset.cpus.effective`, cgroup)
	cpuEffective, err := compat_otp.DebugNodeWithOptionsAndChroot(oc, nodeName, []string{"-q"}, "bash", "-c", cpuEffectiveCmd)
	o.Expect(err).NotTo(o.HaveOccurred())
	cpuEffective = strings.TrimSpace(cpuEffective)
	e2e.Logf("the command of `cat /sys/fs/cgroup%vcpuset.cpus.effective` return: %v", cgroup, cpuEffective)

	/*
		// here exists a bug, comment it temporarily
		cpuCpusCmd := fmt.Sprintf(`cat /sys/fs/cgroup%vcpuset.cpus`, cgroup)
		cpuCpus, err := compat_otp.DebugNodeWithOptionsAndChroot(oc, nodeName, []string{"-q"}, "bash", "-c", cpuCpusCmd)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the command of `cat /sys/fs/cgroup%vcpuset.cpus` return: %v", cgroup, cpuCpus)
	*/

	//e2e.Logf("cpuAffinity is: %v, cpuEffective is: %v and cpuCpus is: %v", cpuAffinity, cpuEffective, cpuCpus)
	e2e.Logf("cpuAffinity is: %v, cpuEffective is: %v", cpuAffinity, cpuEffective)
	// compare cpuAffinity == cpuEffective == cpuCpus
	o.Expect(cpuAffinity == cpuEffective).To(o.BeTrue(), fmt.Sprintf("test failed! cpuAffinity != cpuEffective, cpuAffinity:%v and cpuEffective:%v", cpuAffinity, cpuEffective))
	//o.Expect(cpuCpus == cpuEffective).To(o.BeTrue(), fmt.Sprintf("test failed, cpuCpus != cpuEffective : %v", cpuCpus))
	//e2e.Logf("verify pass: cpuAffinity == cpuEffective == cpuCpus // cpuAffinity is %v, cpuEffective is %v, cpuCpus is %v", cpuAffinity, cpuEffective, cpuCpus)
	e2e.Logf("verify pass: cpuAffinity == cpuEffective // cpuAffinity is %v, cpuEffective is %v", cpuAffinity, cpuEffective)
}

// get a difference set of cpu core, e.g. cpuNum is 4 , guCore is "1", then return "0,2-3"
func getDiffSet(oc *exutil.CLI, cpuNum int, guCore string) string {
	fullSet := make([]int, cpuNum)
	for i := 0; i < cpuNum; i++ {
		fullSet[i] = i
	}
	// Parse the guCore "1" into individual numbers
	excludeParts := strings.Split(guCore, ",")
	excludeMap := make(map[int]bool)
	for _, numStr := range excludeParts {
		num, _ := strconv.Atoi(numStr)
		excludeMap[num] = true
	}
	// Create a slice for the remaining numbers
	var remaining []int
	for _, num := range fullSet {
		if !excludeMap[num] {
			remaining = append(remaining, num)
		}
	}
	return formatStr(remaining)
}

// format the numbers into a string with ranges, e.g. numbers[0,2,3], then return "0,2-3"
func formatStr(numbers []int) string {
	var formatted []string
	i := 0
	for i < len(numbers) {
		start := numbers[i]
		// Find the end of the current contiguous range
		for i+1 < len(numbers) && numbers[i+1] == numbers[i]+1 {
			i++
		}
		end := numbers[i]
		// If the range has only one element, just add it as a single number
		if start == end {
			formatted = append(formatted, strconv.Itoa(start))
		} else {
			// Otherwise, add it as a range
			formatted = append(formatted, fmt.Sprintf("%d-%d", start, end))
		}
		i++
	}
	return strings.Join(formatted, ",")
}

// clusterNodesHealthcheck check abnormal nodes
func clusterNodesHealthcheck(oc *exutil.CLI, waitTime int) error {
	errNode := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, time.Duration(waitTime)*time.Second, false, func(cxt context.Context) (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Output()
		if err == nil {
			if strings.Contains(output, "NotReady") || strings.Contains(output, "SchedulingDisabled") {
				return false, nil
			}
		} else {
			return false, nil
		}
		e2e.Logf("Nodes are normal...")
		err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		return true, nil
	})
	if errNode != nil {
		err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return errNode
}

func defaultRuntimeCheck(oc *exutil.CLI, expectedRuntime string) {
	var defaultruntime string
	var err error
	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(nodeErr).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		nodes := strings.Fields(nodeName)

		for _, node := range nodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				defaultruntime, err = compat_otp.DebugNodeWithChroot(oc, node, "cat", "/etc/crio/crio.conf.d/00-default")
				o.Expect(err).NotTo(o.HaveOccurred())

				if strings.Contains(string(defaultruntime), expectedRuntime) {
					e2e.Logf(" Success !! Default Runtime is %s. \n", expectedRuntime)
					return true, nil
				}
				e2e.Logf(" FAILED!! Default Runtime is not %s \n", expectedRuntime)
				return false, nil
			}
			e2e.Logf("\n NODES ARE NOT READY\n ")
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "Default Runtime is not Expected")
}

// UpdatedRuntimeCheck checks if the runtime has been updated to the expected value.
func UpdatedRuntimeCheck(oc *exutil.CLI, runtime string) {
	var defaultRuntime string
	var err error
	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(nodeErr).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		nodes := strings.Fields(nodeName)

		for _, node := range nodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				defaultRuntime, err = compat_otp.DebugNodeWithChroot(oc, node, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-defaultRuntime")
				o.Expect(err).NotTo(o.HaveOccurred())

				if strings.Contains(string(defaultRuntime), runtime) {
					e2e.Logf(" Success !! Default Runtime is %s. \n", runtime)
					return true, nil
				}
				e2e.Logf(" FAILED!! Default Runtime is not %s \n", runtime)
				return false, nil
			}
			e2e.Logf("\n NODES ARE NOT READY\n ")
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "Default Runtime is not Expected")
}
