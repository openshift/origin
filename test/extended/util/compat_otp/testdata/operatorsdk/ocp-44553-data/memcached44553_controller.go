/*
Copyright 2020.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"

	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	testv1 "github.com/example-inc/memcached-operator/api/v1"
	proxy "github.com/operator-framework/operator-lib/proxy"
)

// Memcached44553Reconciler reconciles a Memcached44553 object
type Memcached44553Reconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cache.httpproxy.com,resources=memcached44553s,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.httpproxy.com,resources=memcached44553s/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.httpproxy.com,resources=memcached44553s/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Memcached44553 object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *Memcached44553Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Memcached44553 instance
	log := ctrllog.FromContext(ctx)
	memcached44553 := &testv1.Memcached44553{}
	err := r.Get(ctx, req.NamespacedName, memcached44553)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Memcached44553 resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get Memcached44553")
		return ctrl.Result{}, err
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	//dep := r.deploymentForMemcached44553(memcached44553)
	//proxyVars := proxy.ReadProxyVarsFromEnv()
	//log.Info("Creating the deploymentfor memcached44553", "Get proxyVars", proxyVars)
	//for _, container := range dep.Spec.Template.Spec.Containers {
	//	log.Info("appedn the proxyVars to container.env")
	//	container.Env = append(container.Env, proxyVars...)
	//}
	err = r.Get(ctx, types.NamespacedName{Name: memcached44553.Name, Namespace: memcached44553.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForMemcached44553(memcached44553)
		//proxyVars := proxy.ReadProxyVarsFromEnv()
		//log.Info("Creating a new dep and get proxyvars", "Get proxyVars", proxyVars)
		//for _, container := range dep.Spec.Template.Spec.Containers {
		//	container.Env = append(container.Env, proxyVars...)
		//	log.Info("append the container.env proxy")
		//}
		for i, container := range dep.Spec.Template.Spec.Containers {
			dep.Spec.Template.Spec.Containers[i].Env = append(container.Env, proxy.ReadProxyVarsFromEnv()...)
		}
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	size := memcached44553.Spec.Size
	if *found.Spec.Replicas != size {
		found.Spec.Replicas = &size
		err = r.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the Memcached44553 status with the pod names
	// List the pods for this memcached44553's deployment
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(memcached44553.Namespace),
		client.MatchingLabels(labelsForMemcached44553(memcached44553.Name)),
	}
	if err = r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods", "Memcached44553.Namespace", memcached44553.Namespace, "Memcached44553.Name", memcached44553.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, memcached44553.Status.Nodes) {
		memcached44553.Status.Nodes = podNames
		err := r.Status().Update(ctx, memcached44553)
		if err != nil {
			log.Error(err, "Failed to update Memcached44553 status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// deploymentForMemcached44553 returns a memcached44553 Deployment object
func (r *Memcached44553Reconciler) deploymentForMemcached44553(m *testv1.Memcached44553) *appsv1.Deployment {
	ls := labelsForMemcached44553(m.Name)
	replicas := m.Spec.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "quay.io/olmqe/memcached:1.4",
						Name:    "memcached44553",
						Command: []string{"memcached", "-m=64", "-o", "modern", "-v"},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 11211,
							Name:          "memcached44553",
						}},
					}},
				},
			},
		},
	}
	// Set Memcached44553 instance as the owner and controller
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

// labelsForMemcached44553 returns the labels for selecting the resources
// belonging to the given memcached44553 CR name.
func labelsForMemcached44553(name string) map[string]string {
	return map[string]string{"app": "memcached44553", "memcached44553_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *Memcached44553Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testv1.Memcached44553{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
