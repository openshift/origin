package trigger

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/ghodss/yaml"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	appsv1clientset "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func readDeploymentFixture(path string) (*appsv1.Deployment, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	deployment := new(appsv1.Deployment)
	err = yaml.Unmarshal(data, deployment)
	if err != nil {
		return nil, err
	}

	return deployment, err
}

func waitForDeploymentModification(appsClient appsv1clientset.AppsV1Interface, objMeta metav1.ObjectMeta, timeout time.Duration, condition func(deployment *appsv1.Deployment) (bool, error)) (*appsv1.Deployment, error) {
	watcher, err := appsClient.Deployments(objMeta.Namespace).Watch(metav1.SingleObject(objMeta))
	if err != nil {
		return nil, err
	}

	event, err := watch.Until(timeout, watcher, func(event watch.Event) (bool, error) {
		if event.Type != watch.Modified && (objMeta.ResourceVersion == "" && event.Type != watch.Added) {
			return true, fmt.Errorf("different kind of event appeared while waiting for Deployment modification: event: %#v", event)
		}
		return condition(event.Object.(*appsv1.Deployment))
	})
	if err != nil {
		return nil, err
	}
	return event.Object.(*appsv1.Deployment), nil
}
