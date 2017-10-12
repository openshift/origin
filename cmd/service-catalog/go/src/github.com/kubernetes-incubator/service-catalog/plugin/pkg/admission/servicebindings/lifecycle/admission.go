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
	"fmt"
	"io"

	"github.com/golang/glog"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/internalversion"
	internalversion "github.com/kubernetes-incubator/service-catalog/pkg/client/listers_generated/servicecatalog/internalversion"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
)

const (
	// PluginName is name of admission plug-in
	PluginName = "ServiceBindingsLifecycle"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(io.Reader) (admission.Interface, error) {
		return NewCredentialsBlocker()
	})
}

// enforceNoNewCredentialsForDeletedInstance is an implementation of admission.Interface.
// If creating new ServiceBindings or updating an existing
// set of credentials, fail the operation if the ServiceInstance is
// marked for deletion
type enforceNoNewCredentialsForDeletedInstance struct {
	*admission.Handler
	instanceLister internalversion.ServiceInstanceLister
}

var _ = scadmission.WantsInternalServiceCatalogInformerFactory(&enforceNoNewCredentialsForDeletedInstance{})

func (b *enforceNoNewCredentialsForDeletedInstance) Admit(a admission.Attributes) error {

	// we need to wait for our caches to warm
	if !b.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}

	// We only care about credentials
	if a.GetResource().Group != servicecatalog.GroupName || a.GetResource().GroupResource() != servicecatalog.Resource("servicebindings") {
		return nil
	}

	// We don't want to deal with any sub resources
	if a.GetSubresource() != "" {
		return nil
	}

	credentials, ok := a.GetObject().(*servicecatalog.ServiceBinding)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind ServiceBindings but was unable to be converted")
	}

	instanceRef := credentials.Spec.ServiceInstanceRef
	instance, err := b.instanceLister.ServiceInstances(credentials.Namespace).Get(instanceRef.Name)

	// block the credentials operation if the ServiceInstance is being deleted
	if err == nil && instance.DeletionTimestamp != nil {
		warning := fmt.Sprintf("ServiceBindings %s/%s references an instance that is being deleted: %s/%s",
			credentials.Namespace,
			credentials.Name,
			credentials.Namespace,
			instanceRef.Name)
		glog.Info(warning, err)
		return admission.NewForbidden(a, fmt.Errorf(warning))
	}

	return nil
}

func (b *enforceNoNewCredentialsForDeletedInstance) SetInternalServiceCatalogInformerFactory(f informers.SharedInformerFactory) {
	instanceInformer := f.Servicecatalog().InternalVersion().ServiceInstances()
	b.instanceLister = instanceInformer.Lister()
	b.SetReadyFunc(instanceInformer.Informer().HasSynced)
}

func (b *enforceNoNewCredentialsForDeletedInstance) Validate() error {
	if b.instanceLister == nil {
		return fmt.Errorf("missing instanceLister")
	}
	return nil
}

// NewCredentialsBlocker creates a new admission control handler that
// blocks creation of a ServiceBinding if the instance
// is being deleted
func NewCredentialsBlocker() (admission.Interface, error) {
	return &enforceNoNewCredentialsForDeletedInstance{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}
