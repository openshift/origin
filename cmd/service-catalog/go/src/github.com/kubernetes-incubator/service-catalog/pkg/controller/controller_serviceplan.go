/*
Copyright 2017 The Kubernetes Authors.

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
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

// Service plan handlers and control-loop

func (c *controller) servicePlanAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("ClusterServicePlan: Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.servicePlanQueue.Add(key)
}

func (c *controller) servicePlanUpdate(oldObj, newObj interface{}) {
	c.servicePlanAdd(newObj)
}

func (c *controller) servicePlanDelete(obj interface{}) {
	servicePlan, ok := obj.(*v1beta1.ClusterServicePlan)
	if servicePlan == nil || !ok {
		return
	}

	glog.V(4).Infof("ClusterServicePlan: Received delete event for %v; no further processing will occur", servicePlan.Name)
}

// reconcileClusterServicePlanKey reconciles a ClusterServicePlan due to resync
//  or an event on the ClusterServicePlan.  Note that this is NOT the main
// reconciliation loop for ClusterServicePlans. ClusterServicePlans are
// primarily reconciled in a separate flow when a ClusterServiceBroker is
// reconciled.
func (c *controller) reconcileClusterServicePlanKey(key string) error {
	plan, err := c.servicePlanLister.Get(key)
	if errors.IsNotFound(err) {
		glog.Infof("ClusterServicePlan %q: Not doing work because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("ClusterServicePlan %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterServicePlan(plan)
}

func (c *controller) reconcileClusterServicePlan(servicePlan *v1beta1.ClusterServicePlan) error {
	glog.Infof("ClusterServicePlan %q (ExternalName: %q): processing", servicePlan.Name, servicePlan.Spec.ExternalName)

	if !servicePlan.Status.RemovedFromBrokerCatalog {
		return nil
	}

	glog.Infof("ClusterServicePlan %q (ExternalName: %q): has been removed from broker catalog; determining whether there are instances remaining", servicePlan.Name, servicePlan.Spec.ExternalName)

	serviceInstances, err := c.findServiceInstancesOnClusterServicePlan(servicePlan)
	if err != nil {
		return err
	}

	if len(serviceInstances.Items) != 0 {
		return nil
	}

	glog.Infof("ClusterServicePlan %q (ExternalName: %q): has been removed from broker catalog and has zero instances remaining; deleting", servicePlan.Name, servicePlan.Spec.ExternalName)
	return c.serviceCatalogClient.ClusterServicePlans().Delete(servicePlan.Name, &metav1.DeleteOptions{})
}

func (c *controller) findServiceInstancesOnClusterServicePlan(servicePlan *v1beta1.ClusterServicePlan) (*v1beta1.ServiceInstanceList, error) {
	fieldSet := fields.Set{
		"spec.clusterServicePlanRef.name": servicePlan.Name,
	}
	fieldSelector := fields.SelectorFromSet(fieldSet).String()
	listOpts := metav1.ListOptions{FieldSelector: fieldSelector}

	return c.serviceCatalogClient.ServiceInstances(metav1.NamespaceAll).List(listOpts)
}
