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

package lifecycle

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/pkg/api/v1"
	core "k8s.io/client-go/testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/internalclientset/fake"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/internalversion"
)

// newHandlerForTest returns a configured handler for testing.
func newHandlerForTest(internalClient internalclientset.Interface) (admission.Interface, informers.SharedInformerFactory, error) {
	f := informers.NewSharedInformerFactory(internalClient, 5*time.Minute)
	handler, err := NewCredentialsBlocker()
	if err != nil {
		return nil, f, err
	}
	pluginInitializer := scadmission.NewPluginInitializer(internalClient, f, nil, nil)
	pluginInitializer.Initialize(handler)
	err = admission.Validate(handler)
	return handler, f, err
}

// newServiceInstance returns a new Service Instance for unit tests
func newServiceInstance() servicecatalog.ServiceInstance {
	return servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-instance", Namespace: "test-ns"},
	}
}

// newServiceInstanceCredential returns a new Service Instance Credential that
// references the "test-instance" service instance.
func newServiceInstanceCredential() servicecatalog.ServiceInstanceCredential {
	return servicecatalog.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cred",
			Namespace: "test-ns",
		},
		Spec: servicecatalog.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{
				Name: "test-instance",
			},
			SecretName: "test-secret",
		},
	}
}

// TestBlockNewCredentialsForDeletedInstance validates the admission controller will
// block creation of a Service Instance Credential that is referencing a
// Service Instance which is marked for deletion
func TestBlockNewCredentialsForDeletedInstance(t *testing.T) {
	fakeClient := &fake.Clientset{}
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}

	instance := newServiceInstance()
	instance.DeletionTimestamp = &metav1.Time{}
	scList := &servicecatalog.ServiceInstanceList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1",
		}}
	scList.Items = append(scList.Items, instance)
	fakeClient.AddReactor("list", "serviceinstances", func(action core.Action) (bool, runtime.Object, error) {
		return true, scList, nil
	})

	credential := newServiceInstanceCredential()

	informerFactory.Start(wait.NeverStop)

	err = handler.Admit(admission.NewAttributesRecord(&credential, nil, servicecatalog.Kind("ServiceInstanceCredentials").WithVersion("version"),
		"test-ns", "test-cred", servicecatalog.Resource("serviceinstancecredentials").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Errorf("Unexpected error: %v", err.Error())
	} else {
		if err.Error() != "serviceinstancecredentials.servicecatalog.k8s.io \"test-cred\" is forbidden: ServiceInstanceCredentials test-ns/test-cred references an instance that is being deleted: test-ns/test-instance" {
			t.Fatalf("admission controller blocked the request but not with expected error, expected a forbidden error, got %q", err.Error())
		}
	}
}

// TestAllowNewCredentialsForNonDeletedInstance validates the admission controller will not block
// creation of a Service Instance Credential if the instance is not
// marked for deletion
func TestAllowNewCredentialsForNonDeletedInstance(t *testing.T) {
	fakeClient := &fake.Clientset{}
	handler, informerFactory, err := newHandlerForTest(fakeClient)
	if err != nil {
		t.Errorf("unexpected error initializing handler: %v", err)
	}
	informerFactory.Start(wait.NeverStop)

	credential := newServiceInstanceCredential()
	err = handler.Admit(admission.NewAttributesRecord(&credential, nil, servicecatalog.Kind("ServiceInstanceCredentials").WithVersion("version"),
		"test-ns", "test-cred", servicecatalog.Resource("serviceinstancecredentials").WithVersion("version"), "", admission.Create, nil))
	if err != nil {
		t.Errorf("Error, admission controller should not block this test")
	}
}
