package keepalived

import (
	"fmt"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type ReporterFunction func(string, error) 


//  Delete matching replication controllers.
func deleteReplicationControllers(name string, namespace string, selector map[string]string, kClient kclient.Interface, after ReporterFunction) {
	labels := labels.SelectorFromSet(selector)
	rcInterface := kClient.ReplicationControllers(namespace)

	rcList, err := rcInterface.List(labels)
	if err != nil {
		after(name, err)
		return
	}

	for _, rc := range rcList.Items {
		if false == strings.HasPrefix(rc.Name, name) {
			continue
		}

		rcName := fmt.Sprintf("replicationController/%v", rc.Name)
		err := rcInterface.Delete(rc.Name)
		after(rcName, err)
	}
}

//  Delete matching Pods for the named deployment.
func deletePods(name string, namespace string, selector map[string]string, kClient kclient.Interface, after ReporterFunction) {
	labels := labels.SelectorFromSet(labels.Set{"deploymentconfig": name})
	podInterface := kClient.Pods(namespace)
	podList, err := podInterface.List(labels)
	if err != nil {
		after(fmt.Sprintf("pods/%v", name), err)
		return
	}

	for _, pod := range podList.Items {
		podName := fmt.Sprintf("pod/%v", pod.Name)
		err := podInterface.Delete(pod.Name)
		after(podName, err)
	}
}

//  Cleanup all the deployment artificats.
func CleanupDeployment(name string, ns string, selector map[string]string, f *clientcmd.Factory, after ReporterFunction) {
	//  First up get the OS and kube clients.
	osClient, kClient, err := f.Clients()
	if err != nil {
		after(name, fmt.Errorf("Error getting client: %v", err))
		return
	}

	//  Delete the matching replication controllers.
	deleteReplicationControllers(name, ns, selector, kClient, after) 
	//  Delete the matching pods.
	deletePods(name, ns, selector, kClient, after)

	//  Delete the matching service.
	serviceName := fmt.Sprintf("service/%v", name)
	serviceInterface := kClient.Services(ns)
        err = serviceInterface.Delete(name)
        after(serviceName, err)

	//  And finally delete the deployment config.
	deploymentConfigName := fmt.Sprintf("deploymentConfig/%v", name)
	err = osClient.DeploymentConfigs(ns).Delete(name)
	after(deploymentConfigName, err)
}
