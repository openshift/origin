package compat_otp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/blang/semver/v4"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type HostedClusterPlatformType = string

const (
	// AWSPlatform represents Amazon Web Services infrastructure.
	AWSPlatform HostedClusterPlatformType = "AWS"

	// NonePlatform represents user supplied (e.g. bare metal) infrastructure.
	NonePlatform HostedClusterPlatformType = "None"

	// IBMCloudPlatform represents IBM Cloud infrastructure.
	IBMCloudPlatform HostedClusterPlatformType = "IBMCloud"

	// AgentPlatform represents user supplied insfrastructure booted with agents.
	AgentPlatform HostedClusterPlatformType = "Agent"

	// KubevirtPlatform represents Kubevirt infrastructure.
	KubevirtPlatform HostedClusterPlatformType = "KubeVirt"

	// AzurePlatform represents Azure infrastructure.
	AzurePlatform HostedClusterPlatformType = "Azure"

	// PowerVSPlatform represents PowerVS infrastructure.
	PowerVSPlatform HostedClusterPlatformType = "PowerVS"
)

// ValidHypershiftAndGetGuestKubeConf check if it is hypershift env and get kubeconf of the hosted cluster
// the first return is hosted cluster name
// the second return is the file of kubeconfig of the hosted cluster
// the third return is the hostedcluster namespace in mgmt cluster which contains the generated resources
// if it is not hypershift env, it will skip test.
func ValidHypershiftAndGetGuestKubeConf(oc *exutil.CLI) (string, string, string) {
	if IsROSA() {
		e2e.Logf("there is a ROSA env")
		hostedClusterName, hostedclusterKubeconfig, hostedClusterNs := ROSAValidHypershiftAndGetGuestKubeConf(oc)
		if len(hostedClusterName) == 0 || len(hostedclusterKubeconfig) == 0 || len(hostedClusterNs) == 0 {
			g.Skip("there is a ROSA env, but the env is problematic, skip test run")
		}
		return hostedClusterName, hostedclusterKubeconfig, hostedClusterNs
	}
	operatorNS := GetHyperShiftOperatorNameSpace(oc)
	if len(operatorNS) <= 0 {
		g.Skip("there is no hypershift operator on host cluster, skip test run")
	}

	hostedclusterNS := GetHyperShiftHostedClusterNameSpace(oc)
	if len(hostedclusterNS) <= 0 {
		g.Skip("there is no hosted cluster NS in mgmt cluster, skip test run")
	}

	clusterNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"-n", hostedclusterNS, "hostedclusters", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(clusterNames) <= 0 {
		g.Skip("there is no hosted cluster, skip test run")
	}

	hypersfhitPodStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"-n", operatorNS, "pod", "-l", "hypershift.openshift.io/operator-component=operator", "-l", "app=operator", "-o=jsonpath={.items[*].status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hypersfhitPodStatus).To(o.ContainSubstring("Running"))

	//get first hosted cluster to run test
	e2e.Logf("the hosted cluster names: %s, and will select the first", clusterNames)
	clusterName := strings.Split(clusterNames, " ")[0]

	var hostedClusterKubeconfigFile string
	if os.Getenv("GUEST_KUBECONFIG") != "" {
		e2e.Logf("the kubeconfig you set GUEST_KUBECONFIG must be that of the hosted cluster %s in namespace %s", clusterName, hostedclusterNS)
		hostedClusterKubeconfigFile = os.Getenv("GUEST_KUBECONFIG")
		e2e.Logf("use a known hosted cluster kubeconfig: %v", hostedClusterKubeconfigFile)
	} else {
		hostedClusterKubeconfigFile = "/tmp/guestcluster-kubeconfig-" + clusterName + "-" + GetRandomString()
		output, err := exec.Command("bash", "-c", fmt.Sprintf("hypershift create kubeconfig --name %s --namespace %s > %s",
			clusterName, hostedclusterNS, hostedClusterKubeconfigFile)).Output()
		e2e.Logf("the cmd output: %s", string(output))
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("create a new hosted cluster kubeconfig: %v", hostedClusterKubeconfigFile)
	}
	e2e.Logf("if you want hostedcluster controlplane namespace, you could get it by combining %s and %s with -", hostedclusterNS, clusterName)
	return clusterName, hostedClusterKubeconfigFile, hostedclusterNS
}

// ValidHypershiftAndGetGuestKubeConfWithNoSkip check if it is hypershift env and get kubeconf of the hosted cluster
// the first return is hosted cluster name
// the second return is the file of kubeconfig of the hosted cluster
// the third return is the hostedcluster namespace in mgmt cluster which contains the generated resources
// if it is not hypershift env, it will not skip the testcase and return null string.
func ValidHypershiftAndGetGuestKubeConfWithNoSkip(oc *exutil.CLI) (string, string, string) {
	if IsROSA() {
		e2e.Logf("there is a ROSA env")
		return ROSAValidHypershiftAndGetGuestKubeConf(oc)
	}
	operatorNS := GetHyperShiftOperatorNameSpace(oc)
	if len(operatorNS) <= 0 {
		return "", "", ""
	}

	hostedclusterNS := GetHyperShiftHostedClusterNameSpace(oc)
	if len(hostedclusterNS) <= 0 {
		return "", "", ""
	}

	clusterNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"-n", hostedclusterNS, "hostedclusters", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(clusterNames) <= 0 {
		return "", "", ""
	}

	hypersfhitPodStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"-n", operatorNS, "pod", "-l", "hypershift.openshift.io/operator-component=operator", "-l", "app=operator", "-o=jsonpath={.items[*].status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hypersfhitPodStatus).To(o.ContainSubstring("Running"))

	//get first hosted cluster to run test
	e2e.Logf("the hosted cluster names: %s, and will select the first", clusterNames)
	clusterName := strings.Split(clusterNames, " ")[0]

	var hostedClusterKubeconfigFile string
	if os.Getenv("GUEST_KUBECONFIG") != "" {
		e2e.Logf("the kubeconfig you set GUEST_KUBECONFIG must be that of the guestcluster %s in namespace %s", clusterName, hostedclusterNS)
		hostedClusterKubeconfigFile = os.Getenv("GUEST_KUBECONFIG")
		e2e.Logf("use a known hosted cluster kubeconfig: %v", hostedClusterKubeconfigFile)
	} else {
		hostedClusterKubeconfigFile = "/tmp/guestcluster-kubeconfig-" + clusterName + "-" + GetRandomString()
		output, err := exec.Command("bash", "-c", fmt.Sprintf("hypershift create kubeconfig --name %s --namespace %s > %s",
			clusterName, hostedclusterNS, hostedClusterKubeconfigFile)).Output()
		e2e.Logf("the cmd output: %s", string(output))
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("create a new hosted cluster kubeconfig: %v", hostedClusterKubeconfigFile)
	}
	e2e.Logf("if you want hostedcluster controlplane namespace, you could get it by combining %s and %s with -", hostedclusterNS, clusterName)
	return clusterName, hostedClusterKubeconfigFile, hostedclusterNS
}

// GetHyperShiftOperatorNameSpace get hypershift operator namespace
// if not exist, it will return empty string.
func GetHyperShiftOperatorNameSpace(oc *exutil.CLI) string {
	args := []string{
		"pods", "-A",
		"-l", "hypershift.openshift.io/operator-component=operator",
		"-l", "app=operator",
		"--ignore-not-found",
		"-ojsonpath={.items[0].metadata.namespace}",
	}
	namespace, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.TrimSpace(namespace)
}

// GetHyperShiftHostedClusterNameSpace get hypershift hostedcluster namespace
// if not exist, it will return empty string. If more than one exists, it will return the first one.
func GetHyperShiftHostedClusterNameSpace(oc *exutil.CLI) string {
	namespace, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"hostedcluster", "-A", "--ignore-not-found", "-ojsonpath={.items[*].metadata.namespace}").Output()

	if err != nil && !strings.Contains(namespace, "the server doesn't have a resource type") {
		o.Expect(err).NotTo(o.HaveOccurred(), "get hostedcluster fail: %v", err)
	}

	if len(namespace) <= 0 {
		return namespace
	}
	namespaces := strings.Fields(namespace)
	if len(namespaces) == 1 {
		return namespaces[0]
	}
	ns := ""
	for _, ns = range namespaces {
		if ns != "clusters" {
			break
		}
	}
	return ns
}

// ROSAValidHypershiftAndGetGuestKubeConf check if it is ROSA-hypershift env and get kubeconf of the hosted cluster, only support prow
// the first return is hosted cluster name
// the second return is the file of kubeconfig of the hosted cluster
// the third return is the hostedcluster namespace in mgmt cluster which contains the generated resources
// if it is not hypershift env, it will skip test.
func ROSAValidHypershiftAndGetGuestKubeConf(oc *exutil.CLI) (string, string, string) {
	operatorNS := GetHyperShiftOperatorNameSpace(oc)
	if len(operatorNS) <= 0 {
		e2e.Logf("there is no hypershift operator on host cluster")
		return "", "", ""
	}

	data, err := ioutil.ReadFile(os.Getenv("SHARED_DIR") + "/cluster-name")
	if err != nil {
		e2e.Logf("can't get hostedcluster name %s SHARE_DIR: %s", err.Error(), os.Getenv("SHARED_DIR"))
		return "", "", ""
	}
	clusterName := strings.ReplaceAll(string(data), "\n", "")
	hostedclusterNS, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("-A", "hostedclusters", `-o=jsonpath={.items[?(@.metadata.name=="`+clusterName+`")].metadata.namespace}`).Output()
	if len(hostedclusterNS) <= 0 {
		e2e.Logf("there is no hosted cluster NS in mgmt cluster")
	}

	hostedClusterKubeconfigFile := os.Getenv("SHARED_DIR") + "/nested_kubeconfig"
	return clusterName, hostedClusterKubeconfigFile, hostedclusterNS
}

// GetHostedClusterPlatformType returns a hosted cluster platform type
// oc is the management cluster client to query the hosted cluster platform type based on hostedcluster CR obj
func GetHostedClusterPlatformType(oc *exutil.CLI, clusterName, clusterNamespace string) (HostedClusterPlatformType, error) {
	if IsHypershiftHostedCluster(oc) {
		return "", fmt.Errorf("this is a hosted cluster env. You should use oc of the management cluster")
	}
	return oc.AsAdmin().WithoutNamespace().Run("get").Args("hostedcluster", clusterName, "-n", clusterNamespace, `-ojsonpath={.spec.platform.type}`).Output()
}

// GetNodePoolNamesbyHostedClusterName gets the nodepools names of the hosted cluster
func GetNodePoolNamesbyHostedClusterName(oc *exutil.CLI, hostedClusterName, hostedClusterNS string) []string {
	var nodePoolName []string
	nodePoolNameList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodepool", "-n", hostedClusterNS, "-ojsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodePoolNameList).NotTo(o.BeEmpty())

	nodePoolName = strings.Fields(nodePoolNameList)
	e2e.Logf("\n\nGot nodepool(s) for the hosted cluster %s: %v\n", hostedClusterName, nodePoolName)
	return nodePoolName
}

// GetHostedClusterVersion gets a HostedCluster's version from the management cluster.
func GetHostedClusterVersion(mgmtOc *exutil.CLI, hostedClusterName, hostedClusterNs string) semver.Version {
	hcVersionStr, _, err := mgmtOc.
		AsAdmin().
		WithoutNamespace().
		Run("get").
		Args("hostedcluster", hostedClusterName, "-n", hostedClusterNs, `-o=jsonpath={.status.version.history[?(@.state!="")].version}`).
		Outputs()
	o.Expect(err).NotTo(o.HaveOccurred())

	hcVersion := semver.MustParse(hcVersionStr)
	e2e.Logf("Found hosted cluster %s version = %q", hostedClusterName, hcVersion)
	return hcVersion
}

func CheckHypershiftOperatorExistence(mgmtOC *exutil.CLI) (bool, error) {
	stdout, _, err := mgmtOC.AsAdmin().WithoutNamespace().Run("get").
		Args("pods", "-n", "hypershift", "-o=jsonpath={.items[*].metadata.name}").Outputs()
	if err != nil {
		return false, fmt.Errorf("failed to get HO Pods: %v", err)
	}
	return len(stdout) > 0, nil
}

func SkipOnHypershiftOperatorExistence(mgmtOC *exutil.CLI, expectHO bool) {
	HOExist, err := CheckHypershiftOperatorExistence(mgmtOC)
	if err != nil {
		e2e.Logf("failed to check Hypershift Operator existence: %v, defaulting to not found", err)
	}

	if HOExist && !expectHO {
		g.Skip("Not expecting Hypershift Operator but it is found, skip the test")
	}
	if !HOExist && expectHO {
		g.Skip("Expecting Hypershift Operator but it is not found, skip the test")
	}
}

// WaitForHypershiftHostedClusterReady waits for the hostedCluster ready
func WaitForHypershiftHostedClusterReady(oc *exutil.CLI, hostedClusterName, hostedClusterNS string) {
	pollWaitErr := wait.PollUntilContextTimeout(context.Background(), 20*time.Second, 10*time.Minute, false, func(cxt context.Context) (bool, error) {
		hostedClusterAvailable, getStatusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("hostedclusters", "-n", hostedClusterNS, "--ignore-not-found", hostedClusterName, `-ojsonpath='{.status.conditions[?(@.type=="Available")].status}'`).Output()
		if getStatusErr != nil {
			e2e.Logf("Failed to get hosted cluster %q status: %v, try next round", hostedClusterName, getStatusErr)
			return false, nil
		}
		if !strings.Contains(hostedClusterAvailable, "True") {
			e2e.Logf("Hosted cluster %q status: Available=%s, try next round", hostedClusterName, hostedClusterAvailable)
			return false, nil
		}

		hostedClusterProgressState, getStateErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("hostedclusters", "-n", hostedClusterNS, "--ignore-not-found", hostedClusterName, `-ojsonpath={.status.version.history[?(@.state!="")].state}`).Output()
		if getStateErr != nil {
			e2e.Logf("Failed to get hosted cluster %q progress state: %v, try next round", hostedClusterName, getStateErr)
			return false, nil
		}
		if !strings.Contains(hostedClusterProgressState, "Completed") {
			e2e.Logf("Hosted cluster %q progress state: %q, try next round", hostedClusterName, hostedClusterProgressState)
			return false, nil
		}
		e2e.Logf("Hosted cluster %q is ready now", hostedClusterName)
		return true, nil
	})
	AssertWaitPollNoErr(pollWaitErr, fmt.Sprintf("Hosted cluster %q still not ready", hostedClusterName))

}
