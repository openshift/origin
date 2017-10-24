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

// Service class handlers and control-loop

func (c *controller) serviceClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.serviceClassQueue.Add(key)
}

func (c *controller) serviceClassUpdate(oldObj, newObj interface{}) {
	c.serviceClassAdd(newObj)
}

func (c *controller) serviceClassDelete(obj interface{}) {
	serviceClass, ok := obj.(*v1beta1.ClusterServiceClass)
	if serviceClass == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for ServiceClass %v; no further processing will occur", serviceClass.Name)
}

// reconcileServiceClassKey reconciles a ServiceClass due to controller resync
// or an event on the ServiceClass.  Note that this is NOT the main
// reconciliation loop for ServiceClass. ServiceClasses are primarily
// reconciled in a separate flow when a ClusterServiceBroker is reconciled.
func (c *controller) reconcileClusterServiceClassKey(key string) error {
	plan, err := c.serviceClassLister.Get(key)
	if errors.IsNotFound(err) {
		glog.Infof("ClusterServiceClass %q: Not doing work because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("ClusterServiceClass %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterServiceClass(plan)
}

func (c *controller) reconcileClusterServiceClass(serviceClass *v1beta1.ClusterServiceClass) error {
	glog.Infof("ClusterServiceClass %q (ExternalName: %q): processing", serviceClass.Name, serviceClass.Spec.ExternalName)

	if !serviceClass.Status.RemovedFromBrokerCatalog {
		return nil
	}

	glog.Infof("ClusterServiceClass %q (ExternalName: %q): has been removed from broker catalog; determining whether there are instances remaining", serviceClass.Name, serviceClass.Spec.ExternalName)

	serviceInstances, err := c.findServiceInstancesOnClusterServiceClass(serviceClass)
	if err != nil {
		return err
	}

	if len(serviceInstances.Items) != 0 {
		return nil
	}

	glog.Infof("ClusterServiceClass %q (ExternalName: %q): has been removed from broker catalog and has zero instances remaining; deleting", serviceClass.Name, serviceClass.Spec.ExternalName)
	return c.serviceCatalogClient.ClusterServiceClasses().Delete(serviceClass.Name, &metav1.DeleteOptions{})
}

func (c *controller) findServiceInstancesOnClusterServiceClass(serviceClass *v1beta1.ClusterServiceClass) (*v1beta1.ServiceInstanceList, error) {
	fieldSet := fields.Set{
		"spec.clusterServiceClassRef.name": serviceClass.Name,
	}
	fieldSelector := fields.SelectorFromSet(fieldSet).String()
	listOpts := metav1.ListOptions{FieldSelector: fieldSelector}

	return c.serviceCatalogClient.ServiceInstances(metav1.NamespaceAll).List(listOpts)
}
