package compat_otp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

//This will check if operator deployment/daemonset is created sucessfully
//will update sro test case to use this common utils later.
//example:
//WaitOprResourceReady(oc, deployment, deployment-name, namespace, true, true)
//WaitOprResourceReady(oc, statefulset, statefulset-name, namespace, false, false)
//WaitOprResourceReady(oc, daemonset, daemonset-name, namespace, false, false)
//If islongduration is true, it will sleep 720s, otherwise 180s
//If excludewinnode is true, skip checking windows nodes daemonset status
//For daemonset or deployment have random name, getting name before use this function

// WaitOprResourceReady used for checking if deployment/daemonset/statefulset is ready
func WaitOprResourceReady(oc *exutil.CLI, kind, name, namespace string, islongduration bool, excludewinnode bool) {
	//If islongduration is true, it will sleep 720s, otherwise 180s
	var timeDurationSec int
	if islongduration {
		timeDurationSec = 720
	} else {
		timeDurationSec = 360
	}

	waitErr := wait.Poll(20*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {
		var (
			kindNames  string
			err        error
			isCreated  bool
			desiredNum string
			readyNum   string
		)

		//Check if deployment/daemonset/statefulset is created.
		switch kind {
		case "deployment", "statefulset":
			kindNames, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, name, "-n", namespace, "-oname").Output()
			if strings.Contains(kindNames, "NotFound") || strings.Contains(kindNames, "No resources") || len(kindNames) == 0 || err != nil {
				isCreated = false
			} else {
				//deployment/statefulset has been created, but not running, need to compare .status.readyReplicas and  in .status.replicas
				isCreated = true
				desiredNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.readyReplicas}").Output()
				readyNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.replicas}").Output()
			}
		case "daemonset":
			kindNames, err = oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, name, "-n", namespace, "-oname").Output()
			e2e.Logf("daemonset name is:" + kindNames)
			if len(kindNames) == 0 || err != nil {
				isCreated = false
			} else {
				//daemonset/statefulset has been created, but not running, need to compare .status.desiredNumberScheduled and .status.numberReady}
				//if the two value is equal, set output="has successfully progressed"
				isCreated = true
				desiredNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.desiredNumberScheduled}").Output()
				//If there are windows worker nodes, the desired daemonset should be linux node's num
				_, WindowsNodeNum := CountNodeNumByOS(oc)
				if WindowsNodeNum > 0 && excludewinnode {

					//Exclude windows nodes
					e2e.Logf("%v desiredNum is: %v", kindNames, desiredNum)
					desiredLinuxWorkerNum, _ := strconv.Atoi(desiredNum)
					e2e.Logf("desiredlinuxworkerNum is:%v", desiredLinuxWorkerNum)
					desiredNum = strconv.Itoa(desiredLinuxWorkerNum - WindowsNodeNum)
				}
				readyNum, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(kindNames, "-n", namespace, "-o=jsonpath={.status.numberReady}").Output()
			}
		default:
			e2e.Logf("Invalid Resource Type")
		}

		e2e.Logf("desiredNum is: " + desiredNum + " readyNum is: " + readyNum)
		//daemonset/deloyment has been created, but not running, need to compare desiredNum and readynum
		//if isCreate is true and the two value is equal, the pod is ready
		if isCreated && len(kindNames) != 0 && desiredNum == readyNum {
			e2e.Logf("The %v is successfully progressed and running normally", kindNames)
			return true, nil
		}
		e2e.Logf("The %v is not ready or running normally", kindNames)
		return false, nil

	})
	AssertWaitPollNoErr(waitErr, fmt.Sprintf("the pod of %v is not running", name))
}

// IsNodeLabeledByNFD Check if NFD Installed base on the cluster labels
func IsNodeLabeledByNFD(oc *exutil.CLI) bool {
	workNode, _ := GetFirstWorkerNode(oc)
	Output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", workNode, "-o", "jsonpath='{.metadata.annotations}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(Output, "nfd.node.kubernetes.io/feature-labels") {
		e2e.Logf("NFD installed on openshift container platform and labeled nodes")
		return true
	}
	return false
}

// CountNodeNumByOS used for count how many worker node by windows or linux
func CountNodeNumByOS(oc *exutil.CLI) (linuxNum int, windowsNum int) {
	//Count how many windows node and linux node
	linuxNodeNames, err := GetAllNodesbyOSType(oc, "linux")
	o.Expect(err).NotTo(o.HaveOccurred())
	windowsNodeNames, err := GetAllNodesbyOSType(oc, "windows")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("linuxNodeNames is:%v", linuxNodeNames[:])
	e2e.Logf("windowsNodeNames is:%v", windowsNodeNames[:])
	linuxNum = len(linuxNodeNames)
	windowsNum = len(windowsNodeNames)
	e2e.Logf("Linux node is:%v, windows node is %v", linuxNum, windowsNum)
	return linuxNum, windowsNum
}

// GetFirstLinuxMachineSets used for getting first linux worker nodes name
func GetFirstLinuxMachineSets(oc *exutil.CLI) string {
	machinesets, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, "-o=jsonpath={.items[*].metadata.name}", "-n", "openshift-machine-api").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	var regularMachineset []string
	machinesetsArray := strings.Split(machinesets, " ")
	//Remove windows machineset
	for _, machineset := range machinesetsArray {

		if strings.Contains(machineset, "windows") || strings.Contains(machineset, "edge") {
			continue
		}
		regularMachineset = append(regularMachineset, machineset)
		e2e.Logf("None windows or edge is %v\n", regularMachineset)

	}
	e2e.Logf("regularMachineset is %v\n", regularMachineset)
	return regularMachineset[0]
}

// InstallNFD attempts to install the Node Feature Discovery operator and verify that it is running
func InstallNFD(oc *exutil.CLI, nfdNamespace string) {
	var (
		nfdNamespaceFile     = FixturePath("testdata", "psap", "nfd", "nfd-namespace.yaml")
		nfdOperatorgroupFile = FixturePath("testdata", "psap", "nfd", "nfd-operatorgroup.yaml")
		nfdSubFile           = FixturePath("testdata", "psap", "nfd", "nfd-sub.yaml")
	)
	// check if NFD namespace already exists
	nsName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespace", nfdNamespace).Output()
	// if namespace exists, check if NFD is installed - exit if it is, continue with installation otherwise
	// if an error is thrown, namespace does not exist, create and continue with installation
	if strings.Contains(nsName, "NotFound") || strings.Contains(nsName, "No resources") || err != nil {
		e2e.Logf("NFD namespace not found - creating namespace and installing NFD ...")
		CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", nfdNamespaceFile)
	} else {
		e2e.Logf("NFD namespace found - checking if NFD is installed ...")
	}

	ogName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("OperatorGroup", "openshift-nfd", "-n", nfdNamespace).Output()
	if strings.Contains(ogName, "NotFound") || strings.Contains(ogName, "No resources") || err != nil {
		// create NFD operator group from template
		ApplyNsResourceFromTemplate(oc, nfdNamespace, "--ignore-unknown-parameters=true", "-f", nfdOperatorgroupFile)
	} else {
		e2e.Logf("NFD operatorgroup found - continue to check subscription ...")
	}

	// get default channel and create subscription from template
	channel, err := GetOperatorPKGManifestDefaultChannel(oc, "nfd", "openshift-marketplace")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Channel: %v", channel)
	// get default channel and create subscription from template
	source, err := GetOperatorPKGManifestSource(oc, "nfd", "openshift-marketplace")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Source: %v", source)

	subName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("Subscription", "-n", nfdNamespace).Output()
	if strings.Contains(subName, "NotFound") || strings.Contains(subName, "No resources") || !strings.Contains(subName, "nfd") || err != nil {
		// create NFD operator group from template
		ApplyNsResourceFromTemplate(oc, nfdNamespace, "--ignore-unknown-parameters=true", "-f", nfdSubFile, "-p", "CHANNEL="+channel, "SOURCE="+source)
	} else {
		e2e.Logf("NFD subscription found - continue to check pod status ...")
	}

	//Wait for NFD controller manager is ready
	WaitOprResourceReady(oc, "deployment", "nfd-controller-manager", nfdNamespace, false, false)

}

// CreateNFDInstance used for create NFD Instance in different namespace
func CreateNFDInstance(oc *exutil.CLI, namespace string) {

	var (
		nfdInstanceFile = FixturePath("testdata", "psap", "nfd", "nfd-instance.yaml")
	)
	// get cluster version and create NFD instance from template
	clusterVersion, _, err := GetClusterVersion(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Cluster Version: %v", clusterVersion)

	nfdinstanceName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("NodeFeatureDiscovery", "nfd-instance", "-n", namespace).Output()
	e2e.Logf("NFD Instance is: %v", nfdinstanceName)
	if strings.Contains(nfdinstanceName, "NotFound") || strings.Contains(nfdinstanceName, "No resources") || err != nil {
		// create NFD operator group from template
		nfdInstanceImage := GetNFDInstanceImage(oc, namespace)
		e2e.Logf("NFD instance image name: %v", nfdInstanceImage)
		o.Expect(nfdInstanceImage).NotTo(o.BeEmpty())
		ApplyNsResourceFromTemplate(oc, namespace, "--ignore-unknown-parameters=true", "-f", nfdInstanceFile, "-p", "IMAGE="+nfdInstanceImage, "NAMESPACE="+namespace)
	} else {
		e2e.Logf("NFD instance found - continue to check pod status ...")
	}

	//wait for NFD master and worker is ready
	WaitOprResourceReady(oc, "deployment", "nfd-master", namespace, false, false)
	WaitOprResourceReady(oc, "daemonset", "nfd-worker", namespace, false, true)
}

// GetNFDVersionbyPackageManifest return NFD version
func GetNFDVersionbyPackageManifest(oc *exutil.CLI, namespace string) string {
	nfdVersionOrigin, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "nfd", "-n", namespace, "-ojsonpath={.status.channels[*].currentCSVDesc.version}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nfdVersionOrigin).NotTo(o.BeEmpty())
	nfdVersionArr := strings.Split(nfdVersionOrigin, ".")
	nfdVersion := nfdVersionArr[0] + "." + nfdVersionArr[1]
	return nfdVersion
}

// GetNFDInstanceImage return correct image name in manifest channel
func GetNFDInstanceImage(oc *exutil.CLI, namespace string) string {
	var nfdInstanceImage string
	nfdInstanceImageStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "nfd", "-n", namespace, "-ojsonpath={.status.channels[*].currentCSVDesc.relatedImages}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nfdInstanceImageStr).NotTo(o.BeEmpty())

	strTmp1 := strings.ReplaceAll(nfdInstanceImageStr, "[", ",")
	strTmp2 := strings.ReplaceAll(strTmp1, "]", ",")
	strTmp3 := strings.ReplaceAll(strTmp2, `"`, "")

	nfdInstanceImageArr := strings.Split(strTmp3, ",")

	//using the last one image if mulitiple image was found
	for i := 0; i < len(nfdInstanceImageArr); i++ {
		if strings.Contains(nfdInstanceImageArr[i], "node-feature-discovery") {
			nfdInstanceImage = nfdInstanceImageArr[i]
		}
	}
	e2e.Logf("NFD instance image name: %v", nfdInstanceImage)
	return nfdInstanceImage
}

// GetOperatorPKGManifestSource used for getting operator Packagemanifest source name
func GetOperatorPKGManifestSource(oc *exutil.CLI, pkgManifestName, namespace string) (string, error) {
	catalogSourceNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("catalogsource", "-n", namespace, "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(catalogSourceNames, "qe-app-registry") || err != nil {
		//If the catalogsource qe-app-registry exist, prefer to use qe-app-registry, not use redhat-operators or certificate-operator ...
		return "qe-app-registry", nil
	}
	soureName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", pkgManifestName, "-n", namespace, "-o=jsonpath={.status.catalogSource}").Output()
	return soureName, err
}

// GetOperatorPKGManifestDefaultChannel to getting operator Packagemanifest default channel
func GetOperatorPKGManifestDefaultChannel(oc *exutil.CLI, pkgManifestName, namespace string) (string, error) {
	channel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", pkgManifestName, "-n", namespace, "-o", "jsonpath={.status.defaultChannel}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return channel, err
}

// ApplyOperatorResourceByYaml - It's not a template yaml file, the yaml shouldn't include namespace, we specify namespace by parameter.
func ApplyOperatorResourceByYaml(oc *exutil.CLI, namespace string, yamlfile string) {
	if len(namespace) == 0 {
		//Create cluster-wide resource
		err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", yamlfile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		//Create namespace-wide resource
		err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", yamlfile, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// CleanupOperatorResourceByYaml - It's not a template yaml file, the yaml shouldn't include namespace, we specify namespace by parameter.
func CleanupOperatorResourceByYaml(oc *exutil.CLI, namespace string, yamlfile string) {
	if len(namespace) == 0 {
		//Delete cluster-wide resource
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", yamlfile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		//Delete namespace-wide resource
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", yamlfile, "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// AssertOprPodLogsbyFilterWithDuration used for truncting pods logs by filter
func AssertOprPodLogsbyFilterWithDuration(oc *exutil.CLI, podName string, namespace string, filter string, timeDurationSec int, minimalMatch int) {
	podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace, "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(podList).To(o.ContainSubstring(podName))

	e2e.Logf("Got pods list as below: \n" + podList)
	//Filter pod name base on deployment name
	regexpoprname, _ := regexp.Compile(".*" + podName + ".*")
	podListArry := regexpoprname.FindAllString(podList, -1)

	podListSize := len(podListArry)
	for i := 0; i < podListSize; i++ {
		//Check the log files until finding the keywords by filter
		waitErr := wait.Poll(15*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {
			e2e.Logf("Verify the logs on %v", podListArry[i])
			output, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podListArry[i], "-n", namespace).Output()
			regexpstr, _ := regexp.Compile(".*" + filter + ".*")
			loglines := regexpstr.FindAllString(output, -1)
			matchNumber := len(loglines)
			if strings.Contains(output, filter) && matchNumber >= minimalMatch {
				//Print the last entry log
				matchNumber = matchNumber - 1
				e2e.Logf("The result is: %v", loglines[matchNumber])
				return true, nil
			}
			e2e.Logf("Can not find the key words in pod logs by: %v", filter)
			return false, nil
		})
		AssertWaitPollNoErr(waitErr, fmt.Sprintf("the pod of %v is not running", podName))
	}
}

// AssertOprPodLogsbyFilter trunct pods logs by filter
func AssertOprPodLogsbyFilter(oc *exutil.CLI, podName string, namespace string, filter string, minimalMatch int) bool {
	podList, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", namespace, "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(podList).To(o.ContainSubstring(podName))

	e2e.Logf("Got pods list as below: \n" + podList)
	//Filter pod name base on deployment name
	regexpoprname, _ := regexp.Compile(".*" + podName + ".*")
	podListArry := regexpoprname.FindAllString(podList, -1)

	podListSize := len(podListArry)
	var isMatch bool
	for i := 0; i < podListSize; i++ {
		e2e.Logf("Verify the logs on %v", podListArry[i])
		output, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args(podListArry[i], "-n", namespace).Output()
		regexpstr, _ := regexp.Compile(".*" + filter + ".*")
		loglines := regexpstr.FindAllString(output, -1)
		matchNumber := len(loglines)
		if strings.Contains(output, filter) && matchNumber >= minimalMatch {
			//Print the last entry log
			matchNumber = matchNumber - 1
			e2e.Logf("The result is: %v", loglines[matchNumber])
			isMatch = true
		} else {
			e2e.Logf("Can not find the key words in pod logs by: %v", filter)
			isMatch = false
		}
	}
	return isMatch
}

// WaitForNoPodsAvailableByKind used for checking no pods in a certain namespace
func WaitForNoPodsAvailableByKind(oc *exutil.CLI, kind string, name string, namespace string) {
	err := wait.Poll(10*time.Second, 180*time.Second, func() (bool, error) {
		kindNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(kind, name, "-n", namespace, "-oname").Output()
		if strings.Contains(kindNames, "NotFound") || strings.Contains(kindNames, "No resources") || len(kindNames) == 0 || err != nil {
			//Check if the new profiles name applied on a node
			e2e.Logf("All the pod has been terminated:\n %v", kindNames)
			return true, nil
		}
		e2e.Logf("The pod is still terminating, waiting for a while: \n%v", kindNames)
		return false, nil
	})
	AssertWaitPollNoErr(err, "No pod was found ...")
}

// InstallPAO attempts to install the Performance Add-On operator and verify that it is running
func InstallPAO(oc *exutil.CLI, paoNamespace string) {
	var (
		paoNamespaceFile     = FixturePath("testdata", "psap", "pao", "pao-namespace.yaml")
		paoOperatorgroupFile = FixturePath("testdata", "psap", "pao", "pao-operatorgroup.yaml")
		paoSubFile           = FixturePath("testdata", "psap", "pao", "pao-subscription.yaml")
	)
	// check if PAO namespace already exists
	nsName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("namespace", paoNamespace).Output()
	// if namespace exists, check if PAO is installed - exit if it is, continue with installation otherwise
	// if an error is thrown, namespace does not exist, create and continue with installation
	if strings.Contains(nsName, "NotFound") || strings.Contains(nsName, "No resources") || err != nil {
		e2e.Logf("PAO namespace not found - creating namespace and installing PAO ...")
		CreateClusterResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", paoNamespaceFile)
	} else {
		e2e.Logf("PAO namespace found - checking if PAO is installed ...")
	}

	ogName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("OperatorGroup", "openshift-performance-addon-operator", "-n", paoNamespace).Output()
	if strings.Contains(ogName, "NotFound") || strings.Contains(ogName, "No resources") || err != nil {
		// create PAO operator group from template
		ApplyNsResourceFromTemplate(oc, paoNamespace, "--ignore-unknown-parameters=true", "-f", paoOperatorgroupFile)
	} else {
		e2e.Logf("PAO operatorgroup found - continue to check subscription ...")
	}

	// get default channel and create subscription from template
	channel, err := GetOperatorPKGManifestDefaultChannel(oc, "performance-addon-operator", "openshift-marketplace")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Channel: %v", channel)
	// get default channel and create subscription from template
	source, err := GetOperatorPKGManifestSource(oc, "performance-addon-operator", "openshift-marketplace")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Source: %v", source)

	subName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("Subscription", "-n", paoNamespace).Output()
	if strings.Contains(subName, "NotFound") || strings.Contains(subName, "No resources") || !strings.Contains(subName, "performance-operator") || err != nil {
		// create PAO operator group from template
		ApplyNsResourceFromTemplate(oc, paoNamespace, "--ignore-unknown-parameters=true", "-f", paoSubFile, "-p", "CHANNEL="+channel, "SOURCE="+source)
	} else {
		e2e.Logf("PAO subscription found - continue to check pod status ...")
	}

	//Wait for PAO controller manager is ready
	WaitOprResourceReady(oc, "deployment", "performance-operator", paoNamespace, false, false)
}

// IsPAOInstalled used for deploying Performance Add-on Operator
func IsPAOInstalled(oc *exutil.CLI) bool {
	var isInstalled bool
	deployments, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("deployment", "-A").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(deployments, "performance-operator") {
		isInstalled = true
	} else {
		e2e.Logf("PAO doesn't installed - will install pao ...")
		isInstalled = false
	}
	return isInstalled
}

// IsPAOInOperatorHub used for checking if PAO exist in OperatorHub
func IsPAOInOperatorHub(oc *exutil.CLI) bool {
	var havePAO bool
	packagemanifest, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", "-A").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(packagemanifest, "performance-addon-operator") {
		havePAO = true
	} else {
		e2e.Logf("No PAO packagemanifet detect in operatorhub - skip ...")
		havePAO = false
	}
	return havePAO
}

// StringToBASE64 Base64 Encode
func StringToBASE64(src string) string {
	// plaintext, err := base64.StdEncoding.DecodeString(src)
	stdEnc := base64.StdEncoding.EncodeToString([]byte(src))
	return string(stdEnc)
}

// BASE64DecodeStr Base64 Decode
func BASE64DecodeStr(src string) string {
	plaintext, err := base64.StdEncoding.DecodeString(src)
	if err != nil {
		return ""
	}
	return string(plaintext)
}

// CreateMachinesetbyInstanceType used to create a machineset with specified machineset name and instance type
func CreateMachinesetbyInstanceType(oc *exutil.CLI, machinesetName string, instanceType string) {
	// Get existing machinesets in cluster
	ocGetMachineset, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, "-n", "openshift-machine-api", "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(ocGetMachineset).NotTo(o.BeEmpty())
	e2e.Logf("Existing machinesets:\n%v", ocGetMachineset)

	// Get name of first machineset in existing machineset list
	firstMachinesetName := GetFirstLinuxMachineSets(oc)
	o.Expect(firstMachinesetName).NotTo(o.BeEmpty())
	e2e.Logf("Got %v from machineset list", firstMachinesetName)

	machinesetYamlOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, firstMachinesetName, "-n", "openshift-machine-api", "-oyaml").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(machinesetYamlOutput).NotTo(o.BeEmpty())

	//Create machinset by specifying a machineset name
	regMachineSet := regexp.MustCompile(firstMachinesetName)
	newMachinesetYaml := regMachineSet.ReplaceAllString(machinesetYamlOutput, machinesetName)

	//Change instanceType to g4dn.xlarge
	iaasPlatform := CheckPlatform(oc)
	if iaasPlatform == "aws" || iaasPlatform == "alibabacloud" {
		regInstanceType := regexp.MustCompile(`instanceType:.*`)
		e2e.Logf("instanceType is %v inside CreateMachinesetbyInstanceType", instanceType)
		newInstanceType := "instanceType: " + instanceType
		newMachinesetYaml = regInstanceType.ReplaceAllString(newMachinesetYaml, newInstanceType)
	} else if iaasPlatform == "gcp" {
		regInstanceType := regexp.MustCompile(`machineType:.*`)
		e2e.Logf("machineType is %v inside CreateMachinesetbyInstanceType", instanceType)
		newInstanceType := "machineType: " + instanceType
		newMachinesetYaml = regInstanceType.ReplaceAllString(newMachinesetYaml, newInstanceType)
	} else if iaasPlatform == "azure" {
		regInstanceType := regexp.MustCompile(`vmSize:.*`)
		e2e.Logf("vmSize is %v inside CreateMachinesetbyInstanceType", instanceType)
		newInstanceType := "vmSize: " + instanceType
		newMachinesetYaml = regInstanceType.ReplaceAllString(newMachinesetYaml, newInstanceType)
	} else if iaasPlatform == "ibmcloud" {
		regInstanceType := regexp.MustCompile(`profile:.*`)
		e2e.Logf("profile is %v inside CreateMachinesetbyInstanceType", instanceType)
		newInstanceType := "profile: " + instanceType
		newMachinesetYaml = regInstanceType.ReplaceAllString(newMachinesetYaml, newInstanceType)
	} else {
		e2e.Logf("unsupported instance: %v", instanceType)
	}

	//Make sure the replicas is 1
	regReplicas := regexp.MustCompile(`replicas:.*`)
	replicasNum := "replicas: 1"
	newMachinesetYaml = regReplicas.ReplaceAllString(newMachinesetYaml, replicasNum)

	machinesetNewB := []byte(newMachinesetYaml)

	newMachinesetFileName := filepath.Join(e2e.TestContext.OutputDir, oc.Namespace()+"-"+machinesetName+"-new.yaml")
	defer os.RemoveAll(newMachinesetFileName)
	err = ioutil.WriteFile(newMachinesetFileName, machinesetNewB, 0o644)
	o.Expect(err).NotTo(o.HaveOccurred())
	ApplyOperatorResourceByYaml(oc, "openshift-machine-api", newMachinesetFileName)
}

// IsMachineSetExist check if machineset exist in OCP
func IsMachineSetExist(oc *exutil.CLI) bool {

	haveMachineSet := true
	Output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", "-n", "openshift-machine-api").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(Output).NotTo(o.BeEmpty())

	if strings.Contains(Output, "No resources found") {
		haveMachineSet = false
	}
	return haveMachineSet
}

// GetMachineSetInstanceType used to get first machineset instance type
func GetMachineSetInstanceType(oc *exutil.CLI) string {
	var (
		instanceType string
		err          error
	)
	firstMachinesetName := GetFirstLinuxMachineSets(oc)
	e2e.Logf("Got %v from machineset list", firstMachinesetName)
	iaasPlatform := CheckPlatform(oc)
	if iaasPlatform == "aws" {
		instanceType, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", firstMachinesetName, "-n", "openshift-machine-api", "-ojsonpath={.spec.template.spec.providerSpec.value.instanceType}").Output()
	} else if iaasPlatform == "azure" {
		instanceType, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", firstMachinesetName, "-n", "openshift-machine-api", "-ojsonpath={.spec.template.spec.providerSpec.value.vmSize}").Output()
	} else if iaasPlatform == "gcp" {
		instanceType, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", firstMachinesetName, "-n", "openshift-machine-api", "-ojsonpath={.spec.template.spec.providerSpec.value.machineType}").Output()
	} else if iaasPlatform == "ibmcloud" {
		instanceType, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", firstMachinesetName, "-n", "openshift-machine-api", "-ojsonpath={.spec.template.spec.providerSpec.value.profile}").Output()
	} else if iaasPlatform == "alibabacloud" {
		instanceType, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", firstMachinesetName, "-n", "openshift-machine-api", "-ojsonpath={.spec.template.spec.providerSpec.value.instanceType}").Output()
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(instanceType).NotTo(o.BeEmpty())
	return instanceType
}

// GetNodeNameByMachineset used for get node name by machineset name
func GetNodeNameByMachineset(oc *exutil.CLI, machinesetName string) string {

	var machineName string
	machinesetLabels, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachineset, machinesetName, "-n", "openshift-machine-api", "-ojsonpath={.spec.selector.matchLabels.machine\\.openshift\\.io/cluster-api-machineset}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(machinesetLabels).NotTo(o.BeEmpty())
	machineNameStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(MapiMachine, "-l", "machine.openshift.io/cluster-api-machineset="+machinesetLabels, "-n", "openshift-machine-api", "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(machineNameStr).NotTo(o.BeEmpty())
	machineNames := strings.Split(machineNameStr, "\n")
	if len(machineNames) > 0 {
		machineName = machineNames[0]
	}

	e2e.Logf("machineName is %v in GetNodeNameByMachineset", machineName)

	nodeName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(machineName, "-n", "openshift-machine-api", "-ojsonpath={.status.nodeRef.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodeName).NotTo(o.BeEmpty())
	return nodeName
}

// AssertIfMCPChangesAppliedByName checks the MCP of a given oc client and determines if the machine counts are as expected
func AssertIfMCPChangesAppliedByName(oc *exutil.CLI, mcpName string, timeDurationSec int) {
	err := wait.Poll(time.Duration(timeDurationSec/10)*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {
		var (
			mcpMachineCount         string
			mcpReadyMachineCount    string
			mcpUpdatedMachineCount  string
			mcpDegradedMachineCount string
			mcpUpdatingStatus       string
			mcpUpdatedStatus        string
			err                     error
		)

		mcpUpdatingStatus, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, `-ojsonpath='{.status.conditions[?(@.type=="Updating")].status}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mcpUpdatingStatus).NotTo(o.BeEmpty())
		mcpUpdatedStatus, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, `-ojsonpath='{.status.conditions[?(@.type=="Updated")].status}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mcpUpdatedStatus).NotTo(o.BeEmpty())

		//Do not check master err due to sometimes SNO can not accesss api server when server rebooted
		mcpMachineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.machineCount}").Output()
		mcpReadyMachineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.readyMachineCount}").Output()
		mcpUpdatedMachineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.updatedMachineCount}").Output()
		mcpDegradedMachineCount, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.degradedMachineCount}").Output()
		if strings.Contains(mcpUpdatingStatus, "False") && strings.Contains(mcpUpdatedStatus, "True") && mcpMachineCount == mcpReadyMachineCount && mcpMachineCount == mcpUpdatedMachineCount && mcpDegradedMachineCount == "0" {
			e2e.Logf("MachineConfigPool [%v] checks succeeded!", mcpName)
			return true, nil
		}

		e2e.Logf("MachineConfigPool [%v] checks failed, the following values were found (all should be '%v'):\nmachineCount: %v\nmcpUpdatingStatus: %v\nmcpUpdatedStatus: %v\nreadyMachineCount: %v\nupdatedMachineCount: %v\nmcpDegradedMachine:%v\nRetrying...", mcpName, mcpMachineCount, mcpMachineCount, mcpUpdatingStatus, mcpUpdatedStatus, mcpReadyMachineCount, mcpUpdatedMachineCount, mcpDegradedMachineCount)
		return false, nil
	})
	AssertWaitPollNoErr(err, "MachineConfigPool checks were not successful within timeout limit")
}

// DeleteMCAndMCPByName used for checking if node return to worker machine config pool and the specified mcp is zero, then delete mc and mcp
func DeleteMCAndMCPByName(oc *exutil.CLI, mcName string, mcpName string, timeDurationSec int) {

	//Check if labeled node return back to worker mcp, then delete mc and mcp after worker mcp is ready
	e2e.Logf("Check if labeled node return back to worker mcp")
	AssertIfMCPChangesAppliedByName(oc, "worker", timeDurationSec)

	mcpNameList, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp").Output()

	if strings.Contains(mcpNameList, mcpName) {
		//Confirm if the custom machine count is 0
		mcpMachineCount, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("mcp", mcpName, "-o=jsonpath={..status.machineCount}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mcpMachineCount).NotTo(o.BeEmpty())
		if mcpMachineCount == "0" {
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("mcp", mcpName, "--ignore-not-found").Execute()
			oc.AsAdmin().WithoutNamespace().Run("delete").Args("mc", mcName, "--ignore-not-found").Execute()
		}
	} else {
		e2e.Logf("The mcp [%v] has been deleted ...", mcpName)
	}
}

// CreateCustomNodePoolInHypershift retrun custom nodepool yaml
func CreateCustomNodePoolInHypershift(oc *exutil.CLI, cloudProvider, guestClusterName, nodePoolName, nodeCount, instanceType, upgradeType, clustersNS, defaultNodePoolName string) {

	if cloudProvider == "aws" {
		cmdString := fmt.Sprintf("hypershift create nodepool %s --cluster-name %s --name %s --node-count %s --instance-type %s --node-upgrade-type %s --namespace %s", cloudProvider, guestClusterName, nodePoolName, nodeCount, instanceType, upgradeType, clustersNS)
		e2e.Logf("cmdString is %v )", cmdString)
		_, err := exec.Command("bash", "-c", cmdString).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else if cloudProvider == "azure" {
		subnetID, err := oc.AsAdmin().Run("get").Args("-n", clustersNS, "nodepool", defaultNodePoolName, "-ojsonpath={.spec.platform.azure.subnetID}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmdString := fmt.Sprintf("hypershift create nodepool %s --cluster-name %s --name %s --node-count %s --instance-type %s --node-upgrade-type %s --nodepool-subnet-id %s --namespace %s", cloudProvider, guestClusterName, nodePoolName, nodeCount, instanceType, upgradeType, subnetID, clustersNS)
		e2e.Logf("cmdString is %v )", cmdString)
		_, err = exec.Command("bash", "-c", cmdString).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else if cloudProvider == "aks" {
		subnetID, err := oc.AsAdmin().Run("get").Args("-n", clustersNS, "nodepool", defaultNodePoolName, "-ojsonpath={.spec.platform.azure.subnetID}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		azMKPSKU, err := oc.AsAdmin().Run("get").Args("-n", clustersNS, "nodepool", defaultNodePoolName, "-ojsonpath={.spec.platform.azure.image.azureMarketplace.sku}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		azMKPVersion, err := oc.AsAdmin().Run("get").Args("-n", clustersNS, "nodepool", defaultNodePoolName, "-ojsonpath={.spec.platform.azure.image.azureMarketplace.version}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		azMKPOffer, err := oc.AsAdmin().Run("get").Args("-n", clustersNS, "nodepool", defaultNodePoolName, "-ojsonpath={.spec.platform.azure.image.azureMarketplace.offer}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		azMKPPublisher, err := oc.AsAdmin().Run("get").Args("-n", clustersNS, "nodepool", defaultNodePoolName, "-ojsonpath={.spec.platform.azure.image.azureMarketplace.publisher}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		cmdString := fmt.Sprintf("hypershift create nodepool azure --cluster-name %s --name %s --node-count %s --instance-type %s --node-upgrade-type %s --nodepool-subnet-id %s --namespace %s --marketplace-offer %s --marketplace-publisher %s --marketplace-sku %s --marketplace-version %s", guestClusterName, nodePoolName, nodeCount, instanceType, upgradeType, subnetID, clustersNS, azMKPOffer, azMKPPublisher, azMKPSKU, azMKPVersion)
		e2e.Logf("cmdString is %v )", cmdString)
		_, err = exec.Command("bash", "-c", cmdString).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		e2e.Logf("Unsupported cloud provider is %v )", cloudProvider)
	}
}

// AssertIfNodePoolIsReadyByName checks if the Nodepool is ready
func AssertIfNodePoolIsReadyByName(oc *exutil.CLI, nodePoolName string, timeDurationSec int, clustersNS string) {

	o.Expect(timeDurationSec).Should(o.BeNumerically(">=", 10), "Disaster error: specify the value of timeDurationSec great than 10.")

	err := wait.Poll(time.Duration(timeDurationSec/10)*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {

		var (
			isNodePoolReady   string
			isAllNodesHealthy string
			err               error
		)
		isAllNodesHealthy, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodepool", nodePoolName, "-n", clustersNS, `-ojsonpath='{.status.conditions[?(@.type=="AllNodesHealthy")].status}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isAllNodesHealthy).NotTo(o.BeEmpty())

		isNodePoolReady, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodepool", nodePoolName, "-n", clustersNS, `-ojsonpath='{.status.conditions[?(@.type=="Ready")].status}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodePoolReady).NotTo(o.BeEmpty())

		//For master node, only make sure one of master is ready.
		if strings.Contains(isNodePoolReady, "True") && strings.Contains(isAllNodesHealthy, "True") {
			return true, nil
		}
		e2e.Logf("Node Pool [%v] checks failed, the following values were found (read type should be true '%v')", nodePoolName, isNodePoolReady)
		return false, nil
	})
	AssertWaitPollNoErr(err, "Nodepool checks were not successful within timeout limit")
}

// AssertIfNodePoolUpdatingConfigByName checks if the Nodepool is ready
func AssertIfNodePoolUpdatingConfigByName(oc *exutil.CLI, nodePoolName string, timeDurationSec int, clustersNS string) {

	o.Expect(timeDurationSec).Should(o.BeNumerically(">=", 10), "Disaster error: specify the value of timeDurationSec great than 10.")

	err := wait.Poll(time.Duration(timeDurationSec/10)*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {

		var (
			isNodePoolUpdatingConfig  string
			isNodePoolAllNodesHealthy string
			isNodePoolReady           string
			err                       error
		)
		isNodePoolUpdatingConfig, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodepool", nodePoolName, "-n", clustersNS, `-ojsonpath='{.status.conditions[?(@.type=="UpdatingConfig")].status}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodePoolUpdatingConfig).NotTo(o.BeEmpty())

		isNodePoolAllNodesHealthy, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodepool", nodePoolName, "-n", clustersNS, `-ojsonpath='{.status.conditions[?(@.type=="AllNodesHealthy")].status}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodePoolAllNodesHealthy).NotTo(o.BeEmpty())

		isNodePoolReady, err = oc.AsAdmin().WithoutNamespace().Run("get").Args("nodepool", nodePoolName, "-n", clustersNS, `-ojsonpath='{.status.conditions[?(@.type=="Ready")].status}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isNodePoolAllNodesHealthy).NotTo(o.BeEmpty())

		if !strings.Contains(isNodePoolUpdatingConfig, "True") && strings.Contains(isNodePoolAllNodesHealthy, "True") && strings.Contains(isNodePoolReady, "True") {
			e2e.Logf("Node Pool [%v] status isNodePoolUpdatingConfig: %v isNodePoolAllNodesHealthy: %v isNodePoolReady: %v')", nodePoolName, isNodePoolUpdatingConfig, isNodePoolAllNodesHealthy, isNodePoolReady)
			return true, nil
		}
		e2e.Logf("Node Pool [%v] checks failed, the following values were found (ready type should be empty '%v')", nodePoolName, isNodePoolUpdatingConfig)
		return false, nil
	})
	AssertWaitPollNoErr(err, "Nodepool checks were not successful within timeout limit")
}

// IsSNOCluster will check if OCP is a single node cluster
func IsSNOCluster(oc *exutil.CLI) bool {
	//Only 1 master, 1 worker node and with the same hostname.
	masterNodes, _ := GetClusterNodesBy(oc, "master")
	workerNodes, _ := GetClusterNodesBy(oc, "worker")
	if len(masterNodes) == 1 && len(workerNodes) == 1 && masterNodes[0] == workerNodes[0] {
		return true
	}
	return false
}

func IsOneMasterWithNWorkerNodes(oc *exutil.CLI) bool {

	//Skip one master with 1-N worker nodes senario
	topologyTypeStdOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("infrastructure", "-ojsonpath={.items[*].status.infrastructureTopology}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(topologyTypeStdOut).NotTo(o.BeEmpty())
	topologyType := strings.ToLower(topologyTypeStdOut)

	masterNodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(masterNodes).NotTo(o.BeEmpty())
	masterNodesArr := strings.Split(masterNodes, "\n")

	workerNodes, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-oname").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(workerNodes).NotTo(o.BeEmpty())
	workerNodesArr := strings.Split(workerNodes, "\n")
	workerNums := len(workerNodesArr)

	if workerNodesArr[0] == masterNodesArr[0] {
		return topologyType == "singlereplica" && workerNums > 1
	} else {
		return topologyType == "singlereplica" && workerNums >= 1
	}
}

// CheckAllNodepoolReadyByHostedClusterName used for checking if all nodepool is ready
// eg. CheckAllNodepoolReadyByHostedClusterName(oc, psap-qe-hcluster01,clusters,3600)
func CheckAllNodepoolReadyByHostedClusterName(oc *exutil.CLI, nodePoolName, hostedClusterNS string, timeDurationSec int) bool {

	var (
		isMatch bool
	)

	err := wait.Poll(90*time.Second, time.Duration(timeDurationSec)*time.Second, func() (bool, error) {
		nodesStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("--ignore-not-found", "np", nodePoolName, `-ojsonpath='{.status.conditions[?(@.type=="Ready")].status}'`, "--namespace", hostedClusterNS).Output()
		o.Expect(err).ShouldNot(o.HaveOccurred())
		e2e.Logf("The nodepool ready status is %v ...", nodesStatus)
		if len(nodesStatus) <= 0 {
			isMatch = true
			return true, nil
		}
		return false, nil
	})
	AssertWaitPollNoErr(err, "The status of nodepool isn't ready")
	return isMatch
}

// getLastWorkerNodeByOsID returns the cluster node by OS type, linux or windows
func getLastWorkerNodeByOsType(oc *exutil.CLI, ostype string) (string, error) {
	nodes, err := GetClusterNodesBy(oc, "worker")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodes).NotTo(o.BeEmpty())

	totalNodeNum := len(nodes)

	for i := totalNodeNum - 1; i >= 0; i-- {
		//Skip the node that is work node and also is master node in the OCP with one master + [1-N] worker node
		nodeLabels, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node/"+nodes[i], "-o", "jsonpath={.metadata.labels}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeLabels).NotTo(o.BeEmpty())

		regNodeLabls := regexp.MustCompile("control-plane|master")
		isMaster := regNodeLabls.MatchString(nodeLabels)

		stdout, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node/"+nodes[i], `-ojsonpath='{.metadata.labels.kubernetes\.io/os}'`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stdout).NotTo(o.BeEmpty())

		if strings.Trim(stdout, "'") == ostype && !isMaster {
			return nodes[i], err
		}
	}
	return "", err
}

// GetLastLinuxWorkerNode return last worker node
func GetLastLinuxWorkerNode(oc *exutil.CLI) (string, error) {
	return getLastWorkerNodeByOsType(oc, "linux")
}

// ValidHypershiftAndGetGuestKubeConf4SecondHostedCluster check if it is hypershift env and get kubeconf of the hosted cluster
// the first return is hosted cluster name
// the second return is the file of kubeconfig of the hosted cluster
// the third return is the hostedcluster namespace in mgmt cluster which contains the generated resources
// if it is not hypershift env, it will skip test.
func ValidHypershiftAndGetGuestKubeConf4SecondHostedCluster(oc *exutil.CLI) (string, string, string) {
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

	clusterNamesStr, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"-n", hostedclusterNS, "hostedclusters", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	clusterNames := strings.Split(clusterNamesStr, " ")
	e2e.Logf(fmt.Sprintf("clusterNames is: %v", clusterNames))
	if len(clusterNames) < 2 {
		g.Skip("there is no second hosted cluster, skip test run")
	}

	hypersfhitPodStatus, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		"-n", operatorNS, "pod", "-o=jsonpath={.items[0].status.phase}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(hypersfhitPodStatus).To(o.ContainSubstring("Running"))

	//get second hosted cluster to run test
	e2e.Logf("the hosted cluster names: %s, and will select the second", clusterNames)
	clusterName := clusterNames[1]

	var hostedClusterKubeconfigFile string
	hostedClusterKubeconfigFile = "/tmp/guestcluster-kubeconfig-" + clusterName + "-" + GetRandomString()
	output, err := exec.Command("bash", "-c", fmt.Sprintf("hypershift create kubeconfig --name %s --namespace %s > %s",
		clusterName, hostedclusterNS, hostedClusterKubeconfigFile)).Output()
	e2e.Logf("the cmd output: %s", string(output))
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf(fmt.Sprintf("create a new hosted cluster kubeconfig: %v", hostedClusterKubeconfigFile))
	e2e.Logf("if you want hostedcluster controlplane namespace, you could get it by combining %s and %s with -", hostedclusterNS, clusterName)
	return clusterName, hostedClusterKubeconfigFile, hostedclusterNS
}

// Is3MasterNoDedicatedWorkerNode reture if the OCP have three master/worker node, but no dedicated worker node.
func Is3MasterNoDedicatedWorkerNode(oc *exutil.CLI) bool {
	// Only 1 master, 1 worker node and with the same hostname.
	masterNodes, err := GetClusterNodesBy(oc, "master")
	o.Expect(err).NotTo(o.HaveOccurred())
	workerNodes, err := GetClusterNodesBy(oc, "worker")
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(masterNodes) != 3 || len(workerNodes) != 3 {
		return false
	}

	matchCount := 0
	for i := 0; i < len(workerNodes); i++ {
		for j := 0; j < len(masterNodes); j++ {
			if workerNodes[i] == masterNodes[j] {
				matchCount++
			}
		}
	}
	return matchCount == 3
}

func converseInstanceType(currentInstanceType string, sSubString string, tSubString string) string {
	var expectedInstanceType string
	if strings.Contains(currentInstanceType, sSubString) {
		expectedInstanceType = strings.ReplaceAll(currentInstanceType, sSubString, tSubString)

	} else if strings.Contains(currentInstanceType, tSubString) {
		expectedInstanceType = strings.ReplaceAll(currentInstanceType, tSubString, sSubString)
	}
	return expectedInstanceType
}

// SpecifyMachinesetWithDifferentInstanceType used for specify cpu type that different from default one
func SpecifyMachinesetWithDifferentInstanceType(oc *exutil.CLI) string {

	var expectedInstanceType string
	//Check cloud provider name
	iaasPlatform := CheckPlatform(oc)

	//Get instance type of the first machineset
	currentInstanceType := GetMachineSetInstanceType(oc)

	switch iaasPlatform {
	case "aws":
		//we use m6i.2xlarge as default instance type, if current machineset instanceType is "m6i.2xlarge", we use "m6i.xlarge"
		expectedInstanceType = converseInstanceType(currentInstanceType, "2xlarge", "xlarge")
		if len(expectedInstanceType) == 0 {
			expectedInstanceType = "m6i.xlarge"
		}
	case "azure":
		//we use Standard_DS3_v2 as default instance type, if current machineset instanceType is "Standard_DS3_v2", we use "Standard_DS2_v2"
		expectedInstanceType = converseInstanceType(currentInstanceType, "DS3_v2", "DS2_v2")
		if len(expectedInstanceType) == 0 {
			expectedInstanceType = "Standard_DS2_v2"
		}
	case "gcp":
		// we use n1-standard-4 as default instance type, if current machineset instanceType is "n1-standard-4", we use "n1-standard-2"
		expectedInstanceType = converseInstanceType(currentInstanceType, "standard-4", "standard-2")
		if len(expectedInstanceType) == 0 {
			expectedInstanceType = "n1-standard-2"
		}
		e2e.Logf("1 currentInstanceType is %v, expectedInstanceType is %v, ", currentInstanceType, expectedInstanceType)

	case "ibmcloud":
		//we use bx2-4x16 as default instance type, if current machineset instanceType is "bx2-4x16", we use "bx2d-2x8"
		expectedInstanceType = converseInstanceType(currentInstanceType, "4x16", "2x8")
		if len(expectedInstanceType) == 0 {
			expectedInstanceType = "bx2d-2x8"
		}
	case "alibabacloud":
		//we use ecs.g6.xlarge as default instance type, if current machineset instanceType is "ecs.g6.xlarge", we use "ecs.g6.large"
		expectedInstanceType = converseInstanceType(currentInstanceType, "sxlarge", "large")
		if len(expectedInstanceType) == 0 {
			expectedInstanceType = "ecs.g6.large"
		}
	default:
		e2e.Logf("Unsupported cloud provider specified, please check")
		expectedInstanceType = ""
	}
	e2e.Logf("3 currentInstanceType is %v, expectedInstanceType is %v, ", currentInstanceType, expectedInstanceType)
	return expectedInstanceType
}

// GetImagestreamImageName Return an imagestream's image repository name
func GetImagestreamImageName(oc *exutil.CLI, imagestreamName string) string {
	var imageName string

	//Ignore NotFound error, it will return a empty string, then use another image if the image doesn't exit
	imageRepos, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("is", imagestreamName, "-n", "openshift", "-ojsonpath={.status.dockerImageRepository}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	if !strings.Contains(imageRepos, "NotFound") {
		imageTags, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("is", imagestreamName, "-n", "openshift", "-ojsonpath={.status.tags[*].tag}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		imageTagList := strings.Split(imageTags, " ")
		//Because some image stream tag is broken, we need to find which image is available in disconnected cluster.
		for i := 0; i < len(imageTagList); i++ {
			jsonathStr := fmt.Sprintf(`-ojsonpath='{.status.tags[%v].conditions[?(@.status=="False")]}{.status.tags[%v].tag}'`, i, i)
			stdOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("is", imagestreamName, "-n", "openshift", jsonathStr).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(stdOut).NotTo(o.BeEmpty())
			e2e.Logf("stdOut is: %v", stdOut)
			if !strings.Contains(stdOut, "NotFound") {
				imageTag := strings.ReplaceAll(stdOut, "'", "")
				imageName = imageRepos + ":" + imageTag
				break
			}

		}

	}
	return imageName
}

// GetRelicasByMachinesetName used for get replicas number by machineset name
func GetRelicasByMachinesetName(oc *exutil.CLI, machinesetName string) string {

	machineseReplicas, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineset", machinesetName, "-n", "openshift-machine-api", `-ojsonpath="{.spec.replicas}"`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(machineseReplicas).NotTo(o.BeEmpty())

	e2e.Logf("machineseReplicas is %v in GetRelicasByMachinesetName", machineseReplicas)
	return machineseReplicas
}

// CountNodeNumByOS used for count how many worker node by windows or linux
func CountLinuxWorkerNodeNumByOS(oc *exutil.CLI) (linuxNum int) {
	//Count how many windows node and linux node
	rhcosWorkerNodes, err := GetAllWorkerNodesByOSID(oc, "rhcos")
	o.Expect(err).NotTo(o.HaveOccurred())
	rhelWorkerNodes, err := GetAllWorkerNodesByOSID(oc, "rhel")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("rhcosWorkerNodes is:%v", rhcosWorkerNodes[:])
	e2e.Logf("rhelWorkerNodes is:%v", rhelWorkerNodes[:])
	rhcosNum := len(rhcosWorkerNodes)
	rhelNum := len(rhelWorkerNodes)
	e2e.Logf("rhcos node is:%v, rhel node is %v", rhcosNum, rhelNum)
	return rhcosNum + rhelNum
}

func ShowSystemctlPropertyValueOfServiceUnitByName(oc *exutil.CLI, tunedNodeName string, ntoNamespace string, serviceUnit string, propertyName string) string {

	var (
		propertyValue string
		err           error
	)
	// Example:
	// Show all properties by systemctl show kubelet
	// ExecMainStartTimestamp=Fri 2024-09-06 11:16:00 UTC
	// ExecMainStartTimestampMonotonic=27894650

	allProperties, err := DebugNodeWithOptionsAndChroot(oc, tunedNodeName, []string{"-q", "--to-namespace=" + ntoNamespace}, "systemctl", "show", serviceUnit)
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Contains(allProperties, propertyName) {
		propertyValue, err = DebugNodeWithOptionsAndChroot(oc, tunedNodeName, []string{"-q", "--to-namespace=" + ntoNamespace}, "systemctl", "show", "-p", propertyName, serviceUnit)
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		o.Expect(strings.Contains(allProperties, propertyName)).To(o.BeTrue())
	}
	//It will return the string like as ExecMainStartTimestampMonotonic=27894650
	return strings.Trim(propertyValue, "\n")
}

func GetSystemctlServiceUnitTimestampByPropertyNameWithMonotonic(propertyValue string) int {

	var (
		serviceUnitTimestamp    int
		serviceUnitTimestampArr []string
		err                     error
	)

	// Extract 27871378 from AssertTimestampMonotonic=27871378
	serviceUnitTimestampArr = strings.Split(propertyValue, "=")
	if len(serviceUnitTimestampArr) > 1 && strings.Contains(propertyValue, "Monotonic") {
		serviceUnitTimestamp, err = strconv.Atoi(serviceUnitTimestampArr[1])
		e2e.Logf("the serviceUnitTimestamp is [ %v ]", serviceUnitTimestamp)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	return serviceUnitTimestamp
}

func CPUManagerStatebyNode(oc *exutil.CLI, namespace string, nodeName string, ContainerName string) (string, string) {

	var (
		PODCUPs string
		CPUNums string
	)

	cpuManagerStateStdOut, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("node/"+nodeName, "-n", namespace, "-q", "--", "chroot", "host", "cat", "/var/lib/kubelet/cpu_manager_state").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(cpuManagerStateStdOut).NotTo(o.BeEmpty())

	var cpuManagerStateInfo map[string]interface{}
	json.Unmarshal([]byte(cpuManagerStateStdOut), &cpuManagerStateInfo)

	defaultCpuSet := fmt.Sprint(cpuManagerStateInfo["defaultCpuSet"])
	o.Expect(defaultCpuSet).NotTo(o.BeEmpty())

	Entries := fmt.Sprint(cpuManagerStateInfo["entries"])
	o.Expect(Entries).NotTo(o.BeEmpty())

	PODUUIDMapCPUs := strings.Split(Entries, " ")
	Len := len(PODUUIDMapCPUs)
	for i := 0; i < Len; i++ {
		if strings.Contains(PODUUIDMapCPUs[i], ContainerName) {
			PODUUIDMapCPU := strings.Split(PODUUIDMapCPUs[i], ":")
			CPUNums = strings.Trim(PODUUIDMapCPU[len(PODUUIDMapCPU)-1], "]")
		}
		PODCUPs += CPUNums + " "
	}
	return defaultCpuSet, PODCUPs
}

func GetContainerIDByPODName(oc *exutil.CLI, podName string, namespace string) string {
	var containerID string
	containerIDStdOut, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, `-ojsonpath='{.status.containerStatuses[?(@.name=="etcd")].containerID}`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(containerIDStdOut).NotTo(o.BeEmpty())

	containerIDArr := strings.Split(containerIDStdOut, "/")
	Len := len(containerIDArr)
	if Len > 0 {
		containerID = containerIDArr[Len-1]
	}
	return containerID
}

func GetPODCPUSet(oc *exutil.CLI, namespace string, nodeName string, containerID string) string {

	podCPUSetStdDir, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("node/"+nodeName, "-n", namespace, "-q", "--", "chroot", "host", "find", "/sys/fs/cgroup/", "-name", "*crio-"+containerID+"*").Output()
	e2e.Logf("The podCPUSetStdDir is [ %v ]", podCPUSetStdDir)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(podCPUSetStdDir).NotTo(o.BeEmpty())
	podCPUSet, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("node/"+nodeName, "-n", namespace, "-q", "--", "chroot", "host", "cat", podCPUSetStdDir+"/cpuset.cpus").Output()
	e2e.Logf("The podCPUSet is [ %v ]", podCPUSet)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(podCPUSet).NotTo(o.BeEmpty())
	return podCPUSet
}

// fuction to check given string is in array or not
func ImplStringArrayContains(stringArray []string, name string) bool {
	// iterate over the array and compare given string to each element
	for _, value := range stringArray {
		if value == name {
			return true
		}
	}
	return false
}
