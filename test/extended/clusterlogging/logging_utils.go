package clusterlogging

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	//"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	//apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 600
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

// OperatorObjects objects for creating operators via OLM
type OperatorObjects struct {
	Namepsace     string
	Operatorgroup string
	Sub           string
}

func createLoggingResources(oc *exutil.CLI, oo OperatorObjects) error {
	filenames := reflect.ValueOf(oo)
	t := reflect.TypeOf(oo)
	num := filenames.NumField()
	for i := 0; i < num; i++ {
		filename := filenames.Field(i).Interface()
		name := fmt.Sprint(filename)
		fmt.Printf("Creating %s ...\n", t.Field(i).Name)
		err := oc.AsAdmin().Run("create").Args("-f", name).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create %s \n", t.Field(i).Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func waitForDeployPodsToBeReady(oc *exutil.CLI, namespace string, name string) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of %s deployment\n", name)
				return false, nil
			}
			return false, err
		}
		if int(deployment.Status.AvailableReplicas) == int(deployment.Status.Replicas) {
			replicas := int(deployment.Status.Replicas)
			e2e.Logf("Deployment %s available (%d/%d)\n", name, replicas, replicas)
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s deployment (%d/%d)\n", name, deployment.Status.AvailableReplicas, deployment.Status.Replicas)
		return false, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func waitForDaemonsetPodsToBeReady(oc *exutil.CLI, name string, ns string) error {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	nodeCount := len(nodes.Items)
	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of %s daemonset\n", name)
				return false, nil
			}
			return false, err
		}
		if int(daemonset.Status.NumberReady) == nodeCount {
			return true, nil
		}
		e2e.Logf("Waiting for full availability of %s daemonset (%d/%d)\n", name, int(daemonset.Status.NumberReady), nodeCount)
		return false, nil
	})
	if err != nil {
		return err
	}
	e2e.Logf("Daemonset %s available (%d/%d)\n", name, nodeCount, nodeCount)
	return nil
}

func getDeploymentsNameViaLabel(oc *exutil.CLI, ns string, label string, count int) ([]string, error) {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of deployment\n")
				return false, nil
			}
			return false, err
		}
		if len(deployList.Items) == count {
			return true, nil
		}
		return false, nil
	})
	if err == nil {
		deployList, err := oc.AdminKubeClient().AppsV1().Deployments(ns).List(metav1.ListOptions{LabelSelector: label})
		if err != nil {
			return nil, err
		}
		expectedDeployments := make([]string, 0, len(deployList.Items))
		for _, deploy := range deployList.Items {
			expectedDeployments = append(expectedDeployments, deploy.Name)
		}
		return expectedDeployments, nil
	}
	return nil, err
}

// delete objects in the cluster
func clearResources(oc *exutil.CLI, resourcetype string, name string, ns string) error {
	msg, err := oc.AsAdmin().Run("delete").Args("-n", ns, resourcetype, name).Output()
	if err != nil {
		errstring := fmt.Sprintf("%v", msg)
		if strings.Contains(errstring, "NotFound") || strings.Contains(errstring, "the server doesn't have a resource type") {
			return nil
		}
		return err
	}
	return nil
}

func deleteNamespace(oc *exutil.CLI, ns string) error {
	// err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ns, nil)
	err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	//err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ns, metav1.NewDeleteOptions(0))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func checkResourcesCreatedByOperators(oc *exutil.CLI, ns string, resourcetype string, name string) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		output, err := oc.AsAdmin().Run("get").Args("-n", ns, resourcetype, name).Output()
		if err != nil {
			msg := fmt.Sprintf(output)
			if strings.Contains(msg, "NotFound") {
				return false, nil
			}
			return false, err
		}
		e2e.Logf("Find %s %s", resourcetype, name)
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}

func checkCronJob(oc *exutil.CLI, name string, ns string) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		_, err = oc.AdminKubeClient().BatchV1beta1().CronJobs(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				e2e.Logf("Waiting for availability of cronjob %s \n", name)
				return false, nil
			}
			return false, err
		}
		e2e.Logf("Find cronjob %s \n", name)
		return true, nil
	})
	if err != nil {
		return err
	}
	return nil
}
