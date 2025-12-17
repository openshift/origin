package cluster

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	g "github.com/onsi/ginkgo/v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"gopkg.in/yaml.v2"
)

var _ = g.Describe("[sig-scalability][Feature:Performance][Serial][Slow] Mirror cluster", func() {
	defer g.GinkgoRecover()
	const filename string = "cm.yml"
	var oc = exutil.NewCLI("cl")
	var c kclientset.Interface

	g.BeforeEach(func() {
		c = oc.AdminKubeClient()
	})

	g.It("it should read the node info", g.Label("Size:S"), func() {
		nodeinfo := map[string]map[string]int{}

		nodes, err := c.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil || len(nodes.Items) == 0 {
			e2e.Failf("Error listing nodes: %v\n", err)
		}

		for _, node := range nodes.Items {
			if _, ok := nodeinfo[node.Labels["type"]]; !ok {
				nodeinfo[node.Labels["type"]] = make(map[string]int)
			}
			nodeinfo[node.Labels["type"]][node.Labels["beta.kubernetes.io/instance-type"]] += 1
		}
		e2e.Logf("We have %v\n", nodeinfo)
	})

	g.It("it should read the cluster apps [apigroup:apps.openshift.io]", g.Label("Size:S"), func() {
		var pods *v1.PodList
		config := ContextType{}
		config.ClusterLoader.Cleanup = true
		nsPod := make(map[string][]v1.Pod)
		nsTemplate := map[string]map[string]struct{}{}
		excludedNS := map[string]struct{}{
			"default":     {},
			"kube-system": {}}

		// Get all namespaces
		nsList, err := c.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			e2e.Failf("Error listing namespaces: %v\n", err)
		}

		// Crawl namespace slice
		for _, ns := range nsList.Items {
			var goodIndex int

			if strings.Contains(ns.Name, "openshift") {
				continue
			}
			// List objects in current namespace
			e2e.Logf("Listing objects in namespace %v", ns.Name)

			// Check for DeploymentConfigs
			dcs, err := oc.AdminAppsClient().AppsV1().DeploymentConfigs(ns.Name).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				e2e.Failf("Error listing DeploymentConfigs: %v\n", err)
			}

			for _, dc := range dcs.Items {
				dc, err := oc.AdminAppsClient().AppsV1().DeploymentConfigs(ns.Name).Get(context.Background(), dc.Name, metav1.GetOptions{})
				if err != nil {
					e2e.Failf("Error DC not found: %v\n", err)
				}
				if templateName, ok := dc.Labels["template"]; ok {
					// Get DC that has template label
					nsTemplate[ns.Name] = make(map[string]struct{})
					nsTemplate[ns.Name][templateName] = struct{}{}
					e2e.Logf("DC found, with template label: %+v", templateName)
				} else {
					e2e.Logf("No template associated with this DC: %s", dc.Name)
				}
			}

			// List pods in namespace
			pods, err = c.CoreV1().Pods(ns.Name).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				e2e.Failf("Error listing pods: %v\n", err)
			}

			for i, pod := range pods.Items {

				// Only consider running pods
				if pod.Status.Phase == v1.PodRunning {
					// If the pod is part of a deployment we will take the template name instead
					if value, ok := pod.Labels["deployment"]; ok {
						// Get RC that matches pod label
						rc, err := c.CoreV1().ReplicationControllers(ns.Name).Get(context.Background(), value, metav1.GetOptions{})
						if err != nil {
							e2e.Failf("Error RC not found: %v\n", err)
						}
						// Find template name from RC labels
						if templateName, ok := rc.Labels["template"]; ok {
							nsTemplate[ns.Name] = make(map[string]struct{})
							nsTemplate[ns.Name][templateName] = struct{}{}
							e2e.Logf("RC found, with template label: %+v", templateName)
						} else {
							e2e.Logf("No template associated with this RC: %s", rc.Name)
						}
					} else {
						// Save standalone pods only
						pods.Items[goodIndex] = pods.Items[i]
						goodIndex++
					}
				}
			}
			nsPod[ns.Name] = pods.Items[:goodIndex]
		}

		// Crawl of namespace-pod map
		for k, v := range nsPod {
			if _, ok := excludedNS[k]; !ok {
				ns := newNS(k)
				for i := range v {
					ns.Pods = append(ns.Pods, newPod(nsPod[k][i]))
				}
				for i := range nsTemplate[k] {
					ns.Templates = append(ns.Templates, newTemplate(i))
				}
				if len(ns.Pods) > 0 || len(ns.Templates) > 0 {
					config.ClusterLoader.Projects = append(config.ClusterLoader.Projects, ns)
				}
			}
		}

		// Marshal CL config struct to yaml
		d, err := yaml.Marshal(&config)
		if err != nil {
			log.Fatalf("error: %v", err)
		}

		// Write to file
		err = ioutil.WriteFile(filename, d, 0644)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
	})
})

func newNS(ns string) ClusterLoaderType {
	return ClusterLoaderType{
		Number:   1,
		Tuning:   "default",
		Basename: ns,
	}
}

func newPod(pod v1.Pod) ClusterLoaderObjectType {
	return ClusterLoaderObjectType{
		Number:   1,
		Image:    pod.Spec.Containers[0].Image,
		Basename: pod.Name,
	}
}

func newTemplate(template string) ClusterLoaderObjectType {
	return ClusterLoaderObjectType{
		Number: 1,
		File:   fmt.Sprint("./examples/quickstarts/", template, ".json"),
	}
}
