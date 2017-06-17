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
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func (c *controller) reconcileServiceClassKey(key string) error {
	serviceClass, err := c.serviceClassLister.Get(key)
	if errors.IsNotFound(err) {
		glog.Infof("Not doing work for ServiceClass %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Errorf("Unable to retrieve ServiceClass %v from store: %v", key, err)
		return err
	}

	return c.reconcileServiceClass(serviceClass)
}

func (c *controller) reconcileServiceClass(serviceClass *v1alpha1.ServiceClass) error {
	glog.V(4).Infof("Processing ServiceClass %v", serviceClass.Name)
	return nil
}

func (c *controller) serviceClassUpdate(oldObj, newObj interface{}) {
	c.serviceClassAdd(newObj)
}

func (c *controller) serviceClassDelete(obj interface{}) {
	serviceClass, ok := obj.(*v1alpha1.ServiceClass)
	if serviceClass == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for ServiceClass %v", serviceClass.Name)
}
