package workloads

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type rodoOperatorgroup struct {
	name      string
	namespace string
	template  string
}

type rodoSubscription struct {
	name        string
	namespace   string
	channelName string
	opsrcName   string
	sourceName  string
	startingCSV string
	template    string
}

type runOnceDurationOverride struct {
	namespace             string
	activeDeadlineSeconds int
	template              string
}

func (sub *rodoSubscription) createSubscription(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "NAME="+sub.name, "NAMESPACE="+sub.namespace,
			"CHANNELNAME="+sub.channelName, "OPSRCNAME="+sub.opsrcName, "SOURCENAME="+sub.sourceName, "STARTINGCSV="+sub.startingCSV)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "sub "+sub.name+" is not created successfully")
}

func (sub *rodoSubscription) deleteSubscription(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := oc.AsAdmin().WithoutNamespace().Run("delete").Args("subscription", sub.name, "-n", sub.namespace).Execute()
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "sub "+sub.name+" is not deleted successfully")
}

func (og *rodoOperatorgroup) createOperatorGroup(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", og.template, "-p", "NAME="+og.name, "NAMESPACE="+og.namespace)
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "og "+og.name+" is not created successfully")
}

func (og *rodoOperatorgroup) deleteOperatorGroup(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := oc.AsAdmin().WithoutNamespace().Run("delete").Args("operatorgroup", og.name, "-n", og.namespace).Execute()
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "og "+og.name+" is not deleted successfully")
}

func (rodods *runOnceDurationOverride) createrunOnceDurationOverride(oc *exutil.CLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", rodods.template, "-p", "NAMESPACE="+rodods.namespace,
			"ACTIVEDEADLINESECONDS="+strconv.Itoa(rodods.activeDeadlineSeconds))
		if err1 != nil {
			e2e.Logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "RunOnceDurationOverrideOperator has not been created successfully")
}

func (sub *rodoSubscription) skipMissingCatalogsources(oc *exutil.CLI) {
	output, errRed := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-marketplace", "catalogsource", "redhat-operators").Output()
	if errRed != nil && strings.Contains(output, "NotFound") {
		g.Skip("Skip since redhat-operators catalogsource is not available")
	} else if errRed != nil && strings.Contains(output, "doesn't have a resource type \"catalogsource\"") {
		g.Skip("Skip since catalogsource is not available")
	} else {
		o.Expect(errRed).NotTo(o.HaveOccurred())
	}
}

func getRandomNum(m int64, n int64) int64 {
	rand.Seed(time.Now().UnixNano())
	return rand.Int63n(n-m+1) + m
}

func createSubscription(oc *exutil.CLI, sub rodoSubscription) {
	err := wait.Poll(5*time.Second, 15*time.Second, func() (bool, error) {
		output, errGet := oc.AsAdmin().WithoutNamespace().Run("get").Args("sub", sub.name, "-n", sub.namespace).Output()
		if errGet == nil {
			return true, nil
		}
		if errGet != nil && (strings.Contains(output, "NotFound")) {
			randomExpandInt64 := getRandomNum(3, 8)
			time.Sleep(time.Duration(randomExpandInt64) * time.Second)
			return false, nil
		}
		return false, nil
	})
	if err != nil {
		err1 := applyResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", sub.template, "-p", "NAME="+sub.name, "NAMESPACE="+sub.namespace,
			"CHANNELNAME="+sub.channelName, "OPSRCNAME="+sub.opsrcName, "SOURCENAME="+sub.sourceName, "STARTINGCSV="+sub.startingCSV)
		e2e.Logf("err %v", err1)
	}
}

func createOperator(oc *exutil.CLI, subD rodoSubscription, ogD rodoOperatorgroup) {
	g.By("Create namespace !!!")
	msg, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("ns", subD.namespace).Output()
	e2e.Logf("err %v, msg %v", err, msg)
	msg, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("namespace", subD.namespace, "openshift.io/cluster-monitoring=true", "--overwrite").Output()
	e2e.Logf("err %v, msg %v", err, msg)

	g.By("Create operatorGroup !!!")
	ogFile, err := oc.AsAdmin().Run("process").Args("--ignore-unknown-parameters=true", "-f", ogD.template, "-p", "NAME="+ogD.name, "NAMESPACE="+ogD.namespace, "-n", ogD.namespace).OutputToFile(getRandomString() + "og.json")
	e2e.Logf("Created the operator-group yaml %s, %v", ogFile, err)
	msg, err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", ogFile).Output()
	e2e.Logf("err %v, msg %v", err, msg)

	g.By("Create subscription for above catalogsource !!!")
	createSubscription(oc, subD)
}

// getPackageManifestValues dynamically fetches all required values from packagemanifest
// instead of hardcoding them in the test
func getPackageManifestValues(oc *exutil.CLI, packageName string, namespace string) rodoSubscription {
	g.By("Fetching packagemanifest values dynamically")

	// Get catalogSource (e.g., "redhat-operators")
	catalogSource, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", packageName, "-n", namespace, "-o=jsonpath={.status.catalogSource}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Catalog Source (opsrcName): %v", catalogSource)

	// Get catalogSourceNamespace (e.g., "openshift-marketplace")
	catalogSourceNamespace, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", packageName, "-n", namespace, "-o=jsonpath={.status.catalogSourceNamespace}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Catalog Source Namespace (sourceName): %v", catalogSourceNamespace)

	// Get defaultChannel (e.g., "stable")
	defaultChannel, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", packageName, "-n", namespace, "-o=jsonpath={.status.defaultChannel}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Default Channel (channelName): %v", defaultChannel)

	// Get currentCSV from the default channel
	currentCSV, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("packagemanifest", packageName, "-n", namespace, "-o=jsonpath={.status.channels[?(@.name==\""+defaultChannel+"\")].currentCSV}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Current CSV (startingCSV): %v", currentCSV)

	// Return populated rodoSubscription struct with dynamically fetched values
	return rodoSubscription{
		name:        packageName,
		namespace:   "openshift-run-once-duration-override-operator",
		channelName: defaultChannel,
		opsrcName:   catalogSource,
		sourceName:  catalogSourceNamespace,
		startingCSV: currentCSV,
	}
}

func applyResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "workload-config.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		configFile = output
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "fail to process parameters")

	e2e.Logf("the file of resource is %s", configFile)

	return oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", configFile).Execute()
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

func checkPodStatus(oc *exutil.CLI, podLabel string, namespace string, expected string) {
	err := wait.Poll(20*time.Second, 300*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", podLabel, "-o=jsonpath={.items[*].status.phase}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the result of pod:%v", output)
		if strings.Contains(output, expected) && (!(strings.Contains(strings.ToLower(output), "error"))) && (!(strings.Contains(strings.ToLower(output), "crashLoopbackOff"))) {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", podLabel, "-o", "yaml").Execute()
	}
	o.Expect(err).NotTo(o.HaveOccurred(), "the state of pod with "+podLabel+" is not expected "+expected)
}

func isSNOCluster(oc *exutil.CLI) bool {
	//Only 1 master, 1 worker node and with the same hostname.
	masterNodes, err1 := exutil.GetClusterNodesByRole(oc, "master")
	workerNodes, err2 := exutil.GetClusterNodesByRole(oc, "worker")
	if err1 == nil && err2 == nil && len(masterNodes) == 1 && len(workerNodes) == 1 && masterNodes[0] == workerNodes[0] {
		return true
	}
	return false
}

func waitForAvailableRsRunning(oc *exutil.CLI, rsKind string, rsName string, namespace string, expected string) bool {
	err := wait.Poll(20*time.Second, 180*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(rsKind, rsName, "-n", namespace, "-o=jsonpath={.status.availableReplicas}").Output()
		if err != nil {
			e2e.Logf("object is still inprogress, error: %s. Trying again", err)
			return false, nil
		}
		if matched, _ := regexp.MatchString(expected, output); matched {
			e2e.Logf("object is up:\n%s", output)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return false
	}
	return true
}
