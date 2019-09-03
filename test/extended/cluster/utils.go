package cluster

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ghodss/yaml"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"

	"github.com/openshift/library-go/pkg/template/templateprocessingclient"
	"github.com/openshift/origin/test/extended/cluster/metrics"
	exutil "github.com/openshift/origin/test/extended/util"
)

// The number of times we re-try to create a pod
const maxRetries = 4

// ParsePods unmarshalls the pod spec file defined in the CL config into a struct
func ParsePods(file string) (kapiv1.Pod, error) {
	var configStruct kapiv1.Pod
	if file == "" {
		return configStruct, nil
	}

	configFile, err := ioutil.ReadFile(file)
	if err != nil {
		return configStruct, err
	}

	switch filepath.Ext(file) {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(configFile, &configStruct)
		if err != nil {
			return configStruct, err
		}
	case ".json":
		err = json.Unmarshal(configFile, &configStruct)
		if err != nil {
			return configStruct, err
		}
	default:
		return configStruct, fmt.Errorf("unknown config file extension")
	}

	return configStruct, nil
}

func WaitForRCReady(oc *exutil.CLI, ns, name string, timeout time.Duration) error {
	err := wait.Poll(2*time.Second, timeout,
		func() (bool, error) {
			rc, err := oc.AdminKubeClient().CoreV1().ReplicationControllers(ns).Get(name, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Failed getting RCs: %v", err)
				return false, nil // Ignore this error (nil) and try again in "Poll" time
			}

			if rc.Status.Replicas == rc.Status.AvailableReplicas &&
				rc.Status.Replicas == rc.Status.ReadyReplicas {
				return true, nil
			}
			return false, nil
		})
	return err
}

// SyncPods waits for pods to enter a state
func SyncPods(c kclientset.Interface, ns string, selectors map[string]string, timeout time.Duration, state kapiv1.PodPhase) (err error) {
	label := labels.SelectorFromSet(selectors)

	err = wait.Poll(2*time.Second, timeout,
		func() (bool, error) {
			podList, err := pod.WaitForPodsWithLabel(c, ns, label)
			if err != nil {
				framework.Failf("Failed getting pods: %v", err)
				return false, nil // Ignore this error (nil) and try again in "Poll" time
			}
			pods := podList.Items

			if pods == nil || len(pods) == 0 {
				return true, nil
			}
			for _, p := range pods {
				if p.Status.Phase != state {
					return false, nil
				}
			}
			return true, nil
		})
	return err
}

// SyncRunningPods waits for pods to enter Running state
func SyncRunningPods(c kclientset.Interface, ns string, selectors map[string]string, timeout time.Duration) (err error) {
	err = SyncPods(c, ns, selectors, timeout, kapiv1.PodRunning)
	if err == nil {
		// There wasn't a timeout
		e2e.Logf("All pods running in %s with labels: %v", ns, selectors)
	}
	return err
}

// SyncSucceededPods waits for pods to enter Completed state
func SyncSucceededPods(c kclientset.Interface, ns string, selectors map[string]string, timeout time.Duration) (err error) {
	err = SyncPods(c, ns, selectors, timeout, kapiv1.PodSucceeded)
	if err == nil {
		// There wasn't a timeout
		e2e.Logf("All pods succeeded in %s with labels: %v", ns, selectors)
	}
	return err
}

func (p *ClusterLoaderObjectType) createPodStruct(ns string, labels map[string]string, spec kapiv1.PodSpec, number int) *kapiv1.Pod {
	if len(spec.Containers) == 0 {
		return &kapiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(p.Basename+"-pod-%v", number),
				Namespace: ns,
				Labels:    labels,
			},
			Spec: kapiv1.PodSpec{
				RestartPolicy: kapiv1.RestartPolicyNever,
				Containers: []kapiv1.Container{
					{
						Name:  "test-container",
						Image: p.Image,
					},
				},
			},
		}
	}

	return &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf(p.Basename+"-pod-%v", number),
			Namespace: ns,
			Labels:    labels,
		},
		Spec: spec,
	}
}

// CreatePods creates pods in user defined namespaces with user configurable tuning sets
func (p *ClusterLoaderObjectType) CreatePods(c kclientset.Interface, ns string, labels map[string]string, spec kapiv1.PodSpec, tuning *TuningSetType, step *metrics.PodStepDuration) error {
	if len(spec.Containers) == 0 && p.Image == "" {
		return fmt.Errorf("pod definition missing both spec and image (at least one is required)")
	}
	for i := 0; i < p.Number; i++ {
		framework.Logf("%v/%v : Creating pod", i+1, p.Number)
		// Retry on pod creation failure
		for retryCount := 0; retryCount <= maxRetries; retryCount++ {
			pod := p.createPodStruct(ns, labels, spec, i)
			_, err := c.CoreV1().Pods(ns).Create(pod)
			if err == nil {
				break
			}
			framework.ExpectNoError(err)
		}
		if tuning != nil {
			// If a rate limit has been defined we wait for N ms between creation
			if tuning.Pods.RateLimit.Delay != 0 {
				framework.Logf("Sleeping %d ms between podcreation.", tuning.Pods.RateLimit.Delay)
				time.Sleep(tuning.Pods.RateLimit.Delay * time.Millisecond)
				(*step).RateDelayCount++
			}
			// If a stepping tuningset has been defined in the config, we wait for the step of pods to be created, and pause
			if tuning.Pods.Stepping.StepSize != 0 && (i+1)%tuning.Pods.Stepping.StepSize == 0 {
				framework.Logf("Waiting for pods created this step to be running")
				waitStartTime := time.Now()
				pods, err := exutil.WaitForPods(c.CoreV1().Pods(ns), exutil.ParseLabelsOrDie(mapToString(labels)), exutil.CheckPodIsRunning, i+1, tuning.Pods.Stepping.Timeout*time.Second)
				(*step).WaitPodsDurations = append((*step).WaitPodsDurations, time.Since(waitStartTime))
				if err != nil {
					framework.Failf("Error in pod wait... %v", err)
				} else if len(pods) < i+1 {
					framework.Failf("Only got %v out of %v pods", len(pods), i+1)
				}

				framework.Logf("We have created %d pods and are now sleeping for %d seconds", i+1, tuning.Pods.Stepping.Pause)
				time.Sleep(tuning.Pods.Stepping.Pause * time.Second)
				(*step).StepPauseCount++
			}
		}
	}

	syncStartTime := time.Now()
	if p.Sync.Running {
		timeout, err := time.ParseDuration(p.Sync.Timeout)
		if err != nil {
			return err
		}
		return SyncRunningPods(c, ns, p.Sync.Selectors, timeout)
	}

	if p.Sync.Server.Enabled {
		var podCount PodCount
		return Server(&podCount, p.Sync.Server.Port, false)
	}

	if p.Sync.Succeeded {
		timeout, err := time.ParseDuration(p.Sync.Timeout)
		if err != nil {
			return err
		}
		return SyncSucceededPods(c, ns, p.Sync.Selectors, timeout)
	}
	endTime := time.Now()
	(*step).SyncTime = endTime.Sub(syncStartTime)
	(*step).TotalTime = endTime.Sub((*step).StartTime)
	return nil
}

func mapToString(m map[string]string) (s string) {
	for k, v := range m {
		s = fmt.Sprintf("%s=%s", k, v)
	}
	return
}

// GetTuningSet matches the name of the tuning set defined in the project and returns a pointer to the set
func GetTuningSet(tuningSets []TuningSetType, podTuning string) (tuning *TuningSetType) {
	if podTuning != "" {
		// Iterate through defined tuningSets
		for _, ts := range tuningSets {
			// If we have a matching tuningSet keep it
			if ts.Name == podTuning {
				tuning = &ts
				return
			}
		}
		framework.Failf("No pod tuning found for: %s", podTuning)
	}
	return nil
}

// Server is the webservice that will synchronize the start and stop of Pods
func Server(c *PodCount, port int, awaitShutdown bool) error {
	const serverPort = 9090

	http.HandleFunc("/start", handleStart(startHandler, c))
	http.HandleFunc("/stop", handleStop(stopHandler, c))
	if port <= 0 || port > 65535 {
		e2e.Logf("Invalid server port %v, using %v", port, serverPort)
		port = serverPort
	}

	server := &http.Server{Addr: fmt.Sprintf((":%d"), port), Handler: nil}

	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}

	go server.Serve(ln)
	fmt.Println("Listening on port", server.Addr)
	if awaitShutdown {
		select {
		case <-c.Shutdown:
			fmt.Println("Shutdown server")
			ln.Close()
			return err
		}
	}

	return nil
}

func handleStart(fn http.HandlerFunc, c *PodCount) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.Started++
		fn(w, r)
		fmt.Printf("Start requests: %d, Stop requests: %d\n", c.Started, c.Stopped)
	}
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello")
}

func handleStop(fn http.HandlerFunc, c *PodCount) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.Stopped++
		fn(w, r)
		fmt.Printf("Start requests: %d, Stop requests: %d\n", c.Started, c.Stopped)
		if c.Stopped == c.Started && c.Stopped > 0 {
			c.Shutdown <- true
		}
	}
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Goodbye")
}

// firstLowercase conversts the first letter of a string to lowercase.
func firstLowercase(s string) string {
	a := []rune(s)
	a[0] = unicode.ToLower(a[0])
	return string(a)
}

// convertVariables takes our loaded struct and converts it into a map[string]string.
func convertVariablesToMap(params map[string]interface{}) map[string]string {
	values := make(map[string]string)
	for k, v := range params {
		k = firstLowercase(k)
		if v != 0 && v != "" {
			if _, ok := v.(int); ok {
				values[k] = strconv.Itoa(v.(int))
			} else {
				values[k] = v.(string)
			}
		}
	}
	return values
}

func getFromFileArg(k string, v interface{}) (arg string) {
	return fmt.Sprintf("--from-file=%s=%v", k, v)
}

// CreateConfigmaps creates config maps from files in user defined namespaces.
func CreateConfigmaps(oc *exutil.CLI, c kclientset.Interface, nsName string, configmaps map[string]interface{}) error {
	var args []string
	var err error

	for k, v := range configmaps {
		if v != nil && v != "" {
			args = append(args, "configmap")
			args = append(args, k)
			args = append(args, getFromFileArg(k, v))
		} else {
			return fmt.Errorf("no or empty value provided for configmap filename")
		}

		err = oc.SetNamespace(nsName).Run("create").Args(args...).Execute()
	}
	return err
}

// CreateSecrets creates secrets from files in user defined namespaces.
func CreateSecrets(oc *exutil.CLI, c kclientset.Interface, nsName string, secrets map[string]interface{}) error {
	var args []string
	var err error

	for k, v := range secrets {
		if v != nil && v != "" {
			args = append(args, "secret")
			args = append(args, "generic")
			args = append(args, k)
			args = append(args, getFromFileArg(k, v))
		} else {
			return fmt.Errorf("no or empty value provided for secret filename")
		}

		err = oc.SetNamespace(nsName).Run("create").Args(args...).Execute()
	}
	return err
}

func convertVariablesToString(params map[string]interface{}) (args []string) {
	for k, v := range params {
		k = strings.ToUpper(k)
		if v == nil {
			// Parameter not defined, see if it is defined in the environment.
			var found bool
			v, found = os.LookupEnv(fmt.Sprintf("%s", k))
			if !found {
				// Parameter not defined in the environment, do not define it
				continue
			}
			// Parameter defined in the environment, use the value
		}
		args = append(args, "-p")
		args = append(args, fmt.Sprintf("%s=%v", k, v))
	}
	return
}

// InjectConfigMap modifies the pod struct and replaces the environment variables.
func InjectConfigMap(c kclientset.Interface, ns string, vars map[string]interface{}, config kapiv1.Pod) string {
	configMapName := ns + "-configmap"
	freshConfigVars := convertVariablesToMap(vars)
	dirtyConfigVars := getClusterData(c, freshConfigVars)
	configMap := newConfigMap(ns, configMapName, dirtyConfigVars)
	framework.Logf("Creating configMap %v in namespace %v", configMap.Name, ns)
	var err error
	if configMap, err = c.CoreV1().ConfigMaps(ns).Create(configMap); err != nil {
		framework.Failf("Unable to create test configMap %s: %v", configMap.Name, err)
	}

	for i, envVar := range config.Spec.Containers[0].Env {
		if _, ok := dirtyConfigVars[envVar.Name]; ok {
			framework.Logf("Found match to replace: %+v", envVar)
			config.Spec.Containers[0].Env[i] = kapiv1.EnvVar{
				Name: envVar.Name,
				ValueFrom: &kapiv1.EnvVarSource{
					ConfigMapKeyRef: &kapiv1.ConfigMapKeySelector{
						LocalObjectReference: kapiv1.LocalObjectReference{
							Name: configMapName,
						},
						Key: envVar.Name,
					},
				},
			}
		} else {
			framework.Logf("Environment variable %v is not defined in Pod file, skipping.", envVar.Name)
		}
	}
	return configMapName
}

// getClusterData will return map containing updated strings based on custer data
func getClusterData(c kclientset.Interface, config map[string]string) map[string]string {
	newConfig := make(map[string]string)
	for k, v := range config {
		if k == "routerIP" {
			// TODO sjug: make localhost func
			//v = localhost(f)
			v = "127.0.0.1"
		} else if k == "targetHost" {
			// getEndpointsWithLabel will not return single string
			v = concatenateIP(getEndpointsWithLabel(c, config["match"]))
		}
		newConfig[k] = v
	}
	return newConfig
}

func concatenateIP(endpointInfo []ServiceInfo) (ip string) {
	for i := range endpointInfo {
		ip += endpointInfo[i].IP + ","
	}
	return
}

func getEndpointsWithLabel(c kclientset.Interface, label string) (endpointInfo []ServiceInfo) {
	selector, _ := labels.Parse(label)
	endpoints, err := c.CoreV1().Endpoints("").List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		panic(err.Error())
	}
	for _, v := range endpoints.Items {
		if len(v.Subsets) > 0 {
			for _, ep := range v.Subsets[0].Addresses {
				end := ServiceInfo{v.ObjectMeta.Name, ep.IP, v.Subsets[0].Ports[0].Port}
				fmt.Printf("For endpoint \"%s\", the IP is %v, the port is %d\n", end.Name, end.IP, end.Port)
				endpointInfo = append(endpointInfo, end)
			}
		}
	}

	return
}

//func getPodDetailsWithLabel(f *framework.Framework, label string) (podInfo []ServiceInfo) {
//	selector := v1.ListOptions{LabelSelector: label}
//	pods, err := f.ClientSet.Core().Pods("").List(selector)
//	if err != nil {
//		panic(err.Error())
//	}
//	for _, v := range pods.Items {
//		pod, err := f.ClientSet.Core().Pods(v.ObjectMeta.Namespace).Get(v.ObjectMeta.Name, metav1.GetOptions{})
//		if err != nil {
//			panic(err.Error())
//		}
//		info := ServiceInfo{pod.Name, pod.Status.PodIP, pod.Spec.Containers[0].Ports[0].ContainerPort}
//		fmt.Printf("For pod \"%s\", the IP is %v, the port is %d\n", info.Name, info.IP, info.Port)
//		podInfo = append(podInfo, info)
//	}
//
//	return
//}

func newConfigMap(ns string, name string, vars map[string]string) *kapiv1.ConfigMap {
	return &kapiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Data: vars,
	}
}

// CreateTemplates creates templates in user defined namespaces with user configurable tuning sets.
func CreateTemplates(oc *exutil.CLI, c kclientset.Interface, nsName, config string, template ClusterLoaderObjectType, tuning *TuningSetType, step *metrics.TemplateStepDuration) error {
	var allArgs []string
	templateFile, err := mkPath(template.File, config)
	if err != nil {
		return err
	}
	e2e.Logf("We're loading file %v: ", templateFile)
	allArgs = append(allArgs, "-f")
	allArgs = append(allArgs, templateFile)

	if template.Parameters == nil {
		e2e.Logf("Template environment variables will not be modified.")
	} else {
		params := convertVariablesToString(template.Parameters)
		allArgs = append(allArgs, params...)
	}

	for i := 0; i < template.Number; i++ {
		identifier := map[string]interface{}{"IDENTIFIER": i}
		identifierParams := convertVariablesToString(identifier)
		idArgs := append(allArgs, identifierParams...)
		e2e.Logf("args: %v", idArgs)
		configFile, err := oc.SetNamespace(nsName).Run("process").Args(idArgs...).OutputToFile("config.json")
		if err != nil {
			e2e.Failf("Unable to process template file. Error: %v", err)
		}

		err = oc.SetNamespace(nsName).Run("create").Args("-f", configFile).Execute()
		if err != nil {
			e2e.Failf("Unable to create template objects. Error: %v", err)
		}
		if err != nil {
			return err
		}

		// If there is a tuning set defined for this template
		if tuning != nil {
			if tuning.Templates.RateLimit.Delay != 0 {
				e2e.Logf("Sleeping %d ms between template creation.", tuning.Templates.RateLimit.Delay)
				time.Sleep(time.Duration(tuning.Templates.RateLimit.Delay) * time.Millisecond)
				(*step).RateDelayCount++
			}
			if tuning.Templates.Stepping.StepSize != 0 && (i+1)%tuning.Templates.Stepping.StepSize == 0 {
				e2e.Logf("We have created %d templates and are now sleeping for %d seconds", i+1, tuning.Templates.Stepping.Pause)
				time.Sleep(time.Duration(tuning.Templates.Stepping.Pause) * time.Second)
				(*step).StepPauseCount++
			}
		}
	}

	syncStartTime := time.Now()
	sync := template.Sync
	if sync.Running {
		timeout, err := time.ParseDuration(sync.Timeout)
		if err != nil {
			return err
		}
		err = SyncRunningPods(c, nsName, sync.Selectors, timeout)
		if err != nil {
			return err
		}
	}

	if sync.Server.Enabled {
		var podCount PodCount
		err := Server(&podCount, sync.Server.Port, false)
		if err != nil {
			return err
		}
	}

	if sync.Succeeded {
		timeout, err := time.ParseDuration(sync.Timeout)
		if err != nil {
			return err
		}
		err = SyncSucceededPods(c, nsName, sync.Selectors, timeout)
		if err != nil {
			return err
		}
	}

	endTime := time.Now()
	(*step).SyncTime = endTime.Sub(syncStartTime)
	(*step).TotalTime = endTime.Sub((*step).StartTime)
	return nil
}

// CreateSimpleTemplates creates templates in user defined namespaces without tuningsets
func CreateSimpleTemplates(oc *exutil.CLI, nsName, config string, template ClusterLoaderObjectType) error {
	clusterAdminClientConfig := oc.AdminConfig()
	restmapper := oc.AsAdmin().RESTMapper()

	templateFile, err := mkPath(template.File, config)
	if err != nil {
		return err
	}
	e2e.Logf("We're loading file %v: ", templateFile)

	data, err := ioutil.ReadFile(templateFile)
	if err != nil {
		return err
	}

	templateFileExtension := filepath.Ext(templateFile)
	if templateFileExtension == ".yaml" || templateFileExtension == ".yml" {
		data, err = yaml.YAMLToJSON(data)
		if err != nil {
			e2e.Logf("Unable to convert %s to YAML.", templateFile)
		}
	}

	templateObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, data)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(clusterAdminClientConfig)
	if err != nil {
		return err
	}

	temp, ok := templateObj.(*unstructured.Unstructured)
	if !ok {
		return err
	}

	parameters, ok := temp.Object["parameters"]
	if !ok {
		e2e.Logf("Template has no parameters")
	}
	params := parameters.([]interface{})

	for count := 0; count < template.Number; count++ {
		for _, p := range params {
			field := p.(map[string]interface{})
			if field["name"] == "IDENTIFIER" {
				field["value"] = fmt.Sprintf("%d", count)
				break
			}
		}
		e2e.Logf("New Parameters: %v", params)

		processedList, err := templateprocessingclient.NewDynamicTemplateProcessor(dynamicClient).ProcessToListFromUnstructured(templateObj.(*unstructured.Unstructured))
		if err != nil {
			return err
		}

		processedItems := len(processedList.Items)
		if processedItems == 0 {
			return err
		}
		e2e.Logf("Processed template list (items %d): %v\n", processedItems, templateObj)

		for _, v := range processedList.Items {
			var err error
			unstructuredObj := &unstructured.Unstructured{}
			unstructuredObj.Object = v.Object
			if err != nil {
				return err
			}

			mapping, err := restmapper.RESTMapping(unstructuredObj.GroupVersionKind().GroupKind(), unstructuredObj.GroupVersionKind().Version)
			createdObj, err := dynamicClient.Resource(mapping.Resource).Namespace(nsName).Create(unstructuredObj, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			e2e.Logf("Created object: %v\n", createdObj)
		}
	}

	return nil
}

func getNsCmdFlag(name string) string {
	return fmt.Sprintf("--namespace=%v", name)
}

// SetNamespaceLabels sets the labels of a namespace
func SetNamespaceLabels(c kclientset.Interface, name string, labels map[string]string) (*kapiv1.Namespace, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	ns, err := c.CoreV1().Namespaces().Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ns.Labels = labels
	return c.CoreV1().Namespaces().Update(ns)
}

// ProjectExists checks to see if a namespace exists, returns boolean
func ProjectExists(oc *exutil.CLI, name string) (bool, error) {
	p, err := oc.AdminProjectClient().ProjectV1().Projects().Get(name, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	if (*p).Name == name {
		return true, nil
	}
	return false, nil
}

// DeleteProject deletes a namespace with timeout
func DeleteProject(oc *exutil.CLI, name string, interval, timeout time.Duration) error {
	e2e.Logf("Deleting project %v ...", name)
	err := oc.AdminProjectClient().ProjectV1().Projects().Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	err = wait.Poll(interval, timeout, func() (bool, error) {
		exists, err := ProjectExists(oc, name)
		if err != nil {
			return true, err
		}
		if exists {
			e2e.Logf("The project %v is still there", name)
			return false, nil
		}
		return true, nil
	})
	return err
}
