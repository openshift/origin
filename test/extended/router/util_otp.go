package router

import (
	"fmt"
	"math/rand"
	"os/exec"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type ingressControllerDescription struct {
	name      string
	namespace string
	domain    string
	template  string
}

func (ingctrl *ingressControllerDescription) create(oc *exutil.CLI) {
	availableWorkerNode, _ := exactNodeDetails(oc)
	if availableWorkerNode < 1 {
		g.Skip("Skipping as there is no enough worker nodes")
	}
	err := createResourceFromTemplate(oc, "--ignore-unknown-parameters=true", "-f", ingctrl.template, "-p", "NAME="+ingctrl.name, "NAMESPACE="+ingctrl.namespace, "DOMAIN="+ingctrl.domain)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (ingctrl *ingressControllerDescription) delete(oc *exutil.CLI) error {
	return oc.AsAdmin().WithoutNamespace().Run("delete").Args("--ignore-not-found", "-n", ingctrl.namespace, "ingresscontroller", ingctrl.name).Execute()
}

func exactNodeDetails(oc *exutil.CLI) (int, string) {
	linuxWorkerDetails, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "--selector=node-role.kubernetes.io/worker=,kubernetes.io/os=linux").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	nodeCount := int(strings.Count(linuxWorkerDetails, "Ready")) - (int(strings.Count(linuxWorkerDetails, "SchedulingDisabled")) + int(strings.Count(linuxWorkerDetails, "NotReady")))
	e2e.Logf("Linux worker node details are:\n%v", linuxWorkerDetails)
	e2e.Logf("Available linux worker node count is: %v", nodeCount)
	return nodeCount, linuxWorkerDetails
}

func createResourceFromTemplate(oc *exutil.CLI, parameters ...string) error {
	jsonCfg := parseToJSON(oc, parameters)
	return oc.AsAdmin().WithoutNamespace().Run("create").Args("-f", jsonCfg).Execute()
}

func parseToJSON(oc *exutil.CLI, parameters []string) string {
	var jsonCfg string
	err := wait.Poll(3*time.Second, 15*time.Second, func() (bool, error) {
		output, err := oc.AsAdmin().Run("process").Args(parameters...).OutputToFile(getRandomString() + "-temp-resource.json")
		if err != nil {
			e2e.Logf("the err:%v, and try next round", err)
			return false, nil
		}
		jsonCfg = output
		return true, nil
	})
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("fail to process %v", parameters))
	e2e.Logf("the file of resource is %s", jsonCfg)
	return jsonCfg
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

func getBaseDomain(oc *exutil.CLI) string {
	basedomain, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("dns.config/cluster", "-o=jsonpath={.spec.baseDomain}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the base domain of the cluster: %v", basedomain)
	return basedomain
}

func createRouteOTP(oc *exutil.CLI, ns, routeType, routeName, serviceName string, extraParas []string) {
	if routeType == "http" {
		cmd := []string{"-n", ns, "service", serviceName, "--name=" + routeName}
		cmd = append(cmd, extraParas...)
		_, err := oc.Run("expose").Args(cmd...).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		cmd := []string{"-n", ns, "route", routeType, routeName, "--service=" + serviceName}
		cmd = append(cmd, extraParas...)
		_, err := oc.WithoutNamespace().Run("create").Args(cmd...).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

func setAnnotation(oc *exutil.CLI, ns, resource, annotation string) {
	err := oc.Run("annotate").Args("-n", ns, resource, annotation, "--overwrite").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getOneRouterPodNameByIC(oc *exutil.CLI, icname string) string {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", "ingresscontroller.operator.openshift.io/deployment-ingresscontroller="+icname, "-o=jsonpath={.items[0].metadata.name}", "-n", "openshift-ingress").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the result of router pod name: %v", podName)
	return podName
}

func getOnePodNameByLabel(oc *exutil.CLI, ns, label string) string {
	podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-l", label, "-o=jsonpath={.items[0].metadata.name}", "-n", ns).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("the one pod with label %v is %v", label, podName)
	return podName
}

func getOneNewRouterPodFromRollingUpdate(oc *exutil.CLI, icName string) string {
	ns := "openshift-ingress"
	deployName := "deployment/router-" + icName
	rsLabel := ""
	re := regexp.MustCompile(`NewReplicaSet:\s+router-.+-([a-z0-9]+)\s+`)
	waitErr := wait.PollImmediate(3*time.Second, 15*time.Second, func() (bool, error) {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("describe").Args(deployName, "-n", ns).Output()
		hash := re.FindStringSubmatch(output)
		if len(hash) > 1 {
			rsLabel = "pod-template-hash=" + hash[1]
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "reached max time allowed but NewReplicaSet not found")
	e2e.Logf("the new ReplicaSet labels is %s", rsLabel)
	err := waitForPodWithLabelReady(oc, ns, rsLabel)
	if err != nil {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pods", "-n", ns).Output()
		e2e.Logf("All current router pods are:\n%v", output)
	}
	compat_otp.AssertWaitPollNoErr(err, "the new router pod failed to be ready within allowed time!")
	return getOnePodNameByLabel(oc, ns, rsLabel)
}

func ensureRouterDeployGenerationIs(oc *exutil.CLI, icName, expectGeneration string) {
	ns := "openshift-ingress"
	deployName := "deployment/router-" + icName
	actualGeneration := "0"

	waitErr := wait.PollImmediate(3*time.Second, 30*time.Second, func() (bool, error) {
		actualGeneration, _ = oc.AsAdmin().WithoutNamespace().Run("get").Args(deployName, "-n", ns, "-o=jsonpath={.metadata.generation}").Output()
		e2e.Logf("Get the deployment generation is: %v", actualGeneration)
		if actualGeneration == expectGeneration {
			e2e.Logf("The router deployment generation is updated to %v", actualGeneration)
			return true, nil
		}
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, fmt.Sprintf("max time reached and the expected deployment generation is %v but got %v", expectGeneration, actualGeneration))
}

func waitForPodWithLabelReady(oc *exutil.CLI, ns, label string) error {
	return wait.Poll(5*time.Second, 3*time.Minute, func() (bool, error) {
		status, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", label, `-ojsonpath={.items[*].status.conditions[?(@.type=="Ready")].status}`).Output()
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

func ensurePodWithLabelReady(oc *exutil.CLI, ns, label string) {
	err := waitForPodWithLabelReady(oc, ns, label)
	if err != nil {
		output, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", ns, "-l", label).Output()
		e2e.Logf("All pods with label %v are:\n%v", label, output)
		logs, _ := oc.AsAdmin().WithoutNamespace().Run("logs").Args("-n", ns, "-l", label, "--tail=10").Output()
		e2e.Logf("The logs of all labeled pods are:\n%v", logs)
	}
	compat_otp.AssertWaitPollNoErr(err, fmt.Sprintf("max time reached but the pods with label %v are not ready", label))
}

func createResourceFromFile(oc *exutil.CLI, ns, file string) {
	err := oc.WithoutNamespace().Run("create").Args("-f", file, "-n", ns).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func createResourceFromWebServer(oc *exutil.CLI, ns, file, srvrcInfo string) []string {
	createResourceFromFile(oc, ns, file)
	err := waitForPodWithLabelReady(oc, ns, "name="+srvrcInfo)
	compat_otp.AssertWaitPollNoErr(err, "backend server pod failed to be ready state within allowed time!")
	srvPodList := getPodListByLabel(oc, ns, "name="+srvrcInfo)
	return srvPodList
}

func getPodListByLabel(oc *exutil.CLI, namespace string, label string) []string {
	podNameAll, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", namespace, "pod", "-l", label, "-ojsonpath={.items..metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	podList := strings.Split(podNameAll, " ")
	e2e.Logf("The pod list is %v", podList)
	return podList
}

func ensureRouteIsAdmittedByIngressController(oc *exutil.CLI, ns, routeName, icName string) {
	jsonPath := fmt.Sprintf(`{.status.ingress[?(@.routerName=="%s")].conditions[?(@.type=="Admitted")].status}`, icName)
	waitForOutputEquals(oc, ns, "route/"+routeName, jsonPath, "True")
}

func waitForOutputEquals(oc *exutil.CLI, ns, resourceName, jsonPath, expected string, args ...interface{}) {
	waitDuration := 180 * time.Second
	for _, arg := range args {
		duration, ok := arg.(time.Duration)
		if ok {
			waitDuration = duration
		}
	}

	waitErr := wait.PollImmediate(5*time.Second, waitDuration, func() (bool, error) {
		output := getByJsonPath(oc, ns, resourceName, jsonPath)
		if output == expected {
			return true, nil
		}
		e2e.Logf("The output of jsonpath does NOT equal the expected string: %v, retrying...", expected)
		return false, nil
	})
	compat_otp.AssertWaitPollNoErr(waitErr, "max time reached but cannot find the expected string")
}

func getByJsonPath(oc *exutil.CLI, ns, resource, jsonPath string) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", ns, resource, "-o=jsonpath="+jsonPath).Output()
	if err != nil {
		e2e.Logf("the error is: %v", err.Error())
	}
	e2e.Logf("the output filtered by jsonpath is: %v", output)
	return output
}

func ensureHaproxyBlockConfigContains(oc *exutil.CLI, routerPodName string, blockCfgStart string, searchList []string) string {
	var (
		haproxyCfg string
		j          = 0
	)

	e2e.Logf("Polling and search haproxy config file")
	waitErr := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		haproxyCfg = getBlockConfig(oc, routerPodName, blockCfgStart)
		for i := j; i < len(searchList); i++ {
			if strings.Contains(haproxyCfg, searchList[i]) {
				e2e.Logf("Found the given string %v in haproxy.config", searchList[i])
				j++
				if j == len(searchList) {
					e2e.Logf("All the given strings are found in haproxy.config")
					return true, nil
				}
			} else {
				e2e.Logf("The given string %v is still not found in haproxy.config, retrying...", searchList[i])
				return false, nil
			}
		}
		return false, nil
	})

	compat_otp.AssertWaitPollNoErr(waitErr, "Reached max time allowed but the given string was not found in haproxy.config")
	e2e.Logf("The part of haproxy.config that matching \"%s\" is:\n%v", blockCfgStart, haproxyCfg)
	return haproxyCfg
}

func ensureHaproxyBlockConfigNotMatchRegexp(oc *exutil.CLI, routerPodName string, blockCfgStart string, searchList []string) string {
	var (
		haproxyCfg string
		j          = 0
	)

	e2e.Logf("Polling and search haproxy config file")
	waitErr := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		haproxyCfg = getBlockConfig(oc, routerPodName, blockCfgStart)
		for i := j; i < len(searchList); i++ {
			searchInfo := regexp.MustCompile(searchList[i]).FindStringSubmatch(haproxyCfg)
			if len(searchInfo) == 0 {
				e2e.Logf("Could not found the given string %v in haproxy.config as expected", searchList[i])
				j++
				if j == len(searchList) {
					e2e.Logf("Could not found all given strings in haproxy.config as expected")
					return true, nil
				}
			} else {
				e2e.Logf("The given string %v is still present in haproxy.config, retrying...", searchList[i])
				return false, nil
			}
		}
		return false, nil
	})

	compat_otp.AssertWaitPollNoErr(waitErr, "Reached max time allowed but given string is still present in haproxy.config")
	e2e.Logf("The part of haproxy.config that matching \"%s\" is:\n%v", blockCfgStart, haproxyCfg)
	return haproxyCfg
}

func getBlockConfig(oc *exutil.CLI, routerPodName, searchString string) string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-ingress", routerPodName, "--", "bash", "-c", "cat haproxy.config").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "get the content of haproxy.config failed")
	result := ""
	flag := 0
	startIndex := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, searchString) {
			result = result + line + "\n"
			flag = 1
			startIndex = len(line) - len(strings.TrimLeft(line, " "))
		} else if flag == 1 {
			lineLen := len(line)
			if lineLen == 0 {
				result = result + "\n"
			} else {
				currentIndex := len(line) - len(strings.TrimLeft(line, " "))
				if currentIndex > startIndex {
					result = result + line + "\n"
				} else {
					flag = 2
				}
			}
		} else if flag == 2 {
			break
		}
	}
	e2e.Logf("The block configuration in haproxy that matching \"%s\" is:\n%v", searchString, result)
	return result
}

func getPodv4Address(oc *exutil.CLI, podName, namespace string) string {
	podIPv4, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", namespace, "-o=jsonpath={.status.podIP}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("IP of the %s pod in namespace %s is %q ", podName, namespace, podIPv4)
	return podIPv4
}

func repeatCmdOnClient(oc *exutil.CLI, cmd, expectOutput interface{}, duration time.Duration, repeatTimes int) (string, []int) {
	var (
		clientType       = "Internal"
		matchedTimesList = []int{}
		successCurlCount = 0
		matchedCount     = 0
		expectOutputList = []string{}
		output           = ""
	)

	cmdStr, ok := cmd.(string)
	if ok {
		clientType = "External"
	}
	cmdList, _ := cmd.([]string)

	expStr, ok := expectOutput.(string)
	if ok {
		expectOutputList = append(expectOutputList, expStr)
	}
	expList, ok := expectOutput.([]string)
	if ok {
		expectOutputList = expList
	}

	for i := 0; i < len(expectOutputList); i++ {
		matchedTimesList = append(matchedTimesList, 0)
	}

	e2e.Logf("Using client type: %v", clientType)
	e2e.Logf("The cmdStr (used by External client) is '%v' and cmdList (used by Internal client) is %v", cmdStr, cmdList)
	e2e.Logf("The expectOutputList is %v and initial matchedTimesList is %v", expectOutputList, matchedTimesList)

	waitErr := wait.Poll(1*time.Second, duration*time.Second, func() (bool, error) {
		isMatch := false
		if clientType == "Internal" {
			info, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(cmdList...).Output()
			if err != nil {
				e2e.Logf("The error is: %v", err.Error())
				searchInfo := regexp.MustCompile(expectOutputList[0]).FindStringSubmatch(err.Error())
				if len(searchInfo) > 0 {
					e2e.Logf("The expected string is included in err: %v", err)
					return true, nil
				}
				e2e.Logf("Failed to execute cmd and got err %v, retrying...", err.Error())
				return false, nil
			}
			output = info
		} else {
			info, err := exec.Command("bash", "-c", cmdStr).CombinedOutput()
			if err != nil {
				e2e.Logf("The error is: %v", err.Error())
				searchInfo := regexp.MustCompile(expectOutputList[0]).FindStringSubmatch(err.Error())
				if len(searchInfo) > 0 {
					e2e.Logf("The expected string is included in err: %v", err)
					return true, nil
				}
				e2e.Logf("Failed to execute cmd and got err %v, retrying...", err.Error())
				return false, nil
			}
			output = string(info)
		}

		successCurlCount++
		e2e.Logf("Executed cmd for %v times on the client and got output: %s", successCurlCount, output)

		for i := 0; i < len(expectOutputList); i++ {
			searchInfo := regexp.MustCompile(expectOutputList[i]).FindStringSubmatch(output)
			if len(searchInfo) > 0 {
				isMatch = true
				matchedCount++
				matchedTimesList[i] = matchedTimesList[i] + 1
				break
			}
		}

		if isMatch {
			e2e.Logf("Successfully executed cmd for %v times on the client, expecting %v times", matchedCount, repeatTimes)
			if matchedCount == repeatTimes {
				return true, nil
			}
			return false, nil
		}
		successCurlCount--
		e2e.Logf("Failed to find a match in the output, retrying...")
		return false, nil
	})

	e2e.Logf("The matchedTimesList is: %v", matchedTimesList)
	compat_otp.AssertWaitPollNoErr(waitErr, "max time reached but can't execute the cmd successfully for the desired times")

	return output, matchedTimesList
}

func isCanaryRouteAvailable(oc *exutil.CLI) bool {
	routehost := getByJsonPath(oc, "openshift-ingress-canary", "route/canary", "{.status.ingress[0].host}")
	curlCmd := fmt.Sprintf(`curl https://%s -skI --connect-timeout 10`, routehost)
	_, matchedTimes := repeatCmdOnClient(oc, curlCmd, "200", 60, 1)
	if matchedTimes[0] == 1 {
		return true
	}
	return false
}

func updateFilebySedCmd(file, toBeReplaced, newContent string) {
	sedCmd := fmt.Sprintf(`sed -i'' -e 's|%s|%s|g' %s`, toBeReplaced, newContent, file)
	_, err := exec.Command("bash", "-c", sedCmd).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}
