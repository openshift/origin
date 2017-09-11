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

package defaultserviceplan

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/internalversion"
	internalversion "github.com/kubernetes-incubator/service-catalog/pkg/client/listers_generated/servicecatalog/internalversion"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scadmission "github.com/kubernetes-incubator/service-catalog/pkg/apiserver/admission"
)

const (
	// PluginName is name of admission plug-in
	PluginName = "DefaultServicePlan"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(io.Reader) (admission.Interface, error) {
		return NewDefaultServicePlan()
	})
}

// exists is an implementation of admission.Interface.
// It checks to see if Service Instance is being created without
// a Service Plan if there is only one Service Plan for the
// specified Service and defaults to that value.
// that the cluster actually has support for it.
type defaultServicePlan struct {
	*admission.Handler
	scLister internalversion.ServiceClassLister
}

var _ = scadmission.WantsInternalServiceCatalogInformerFactory(&defaultServicePlan{})

func (d *defaultServicePlan) Admit(a admission.Attributes) error {
	// we need to wait for our caches to warm
	if !d.WaitForReady() {
		return admission.NewForbidden(a, fmt.Errorf("not yet ready to handle request"))
	}

	// We only care about service Instances
	if a.GetResource().Group != servicecatalog.GroupName || a.GetResource().GroupResource() != servicecatalog.Resource("serviceinstances") {
		return nil
	}
	instance, ok := a.GetObject().(*servicecatalog.ServiceInstance)
	if !ok {
		return apierrors.NewBadRequest("Resource was marked with kind Instance but was unable to be converted")
	}
	// If the plan is specified, let it through and have the controller
	// deal with finding the right plan, etc.
	if len(instance.Spec.PlanName) > 0 {
		return nil
	}

	sc, err := d.scLister.Get(instance.Spec.ServiceClassName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return admission.NewForbidden(a, err)
		}
		msg := fmt.Sprintf("ServiceClass %q does not exist, can not figure out the default Service Plan.", instance.Spec.ServiceClassName)
		glog.V(4).Info(msg)
		return admission.NewForbidden(a, errors.New(msg))
	}
	if len(sc.Plans) > 1 {
		msg := fmt.Sprintf("ServiceClass %q has more than one plan, PlanName must be specified", instance.Spec.ServiceClassName)
		glog.V(4).Info(msg)
		return admission.NewForbidden(a, errors.New(msg))
	}

	p := sc.Plans[0]
	glog.V(4).Infof("Using default plan %q for Service Class %q for instance %s",
		p.Name, sc.Name, instance.Name)
	instance.Spec.PlanName = p.Name
	return nil
}

// NewDefaultServicePlan creates a new admission control handler that
// fills in a default Service Plan if omitted from Service Instance
// creation request and if there exists only one plan in the
// specified Service Class
func NewDefaultServicePlan() (admission.Interface, error) {
	return &defaultServicePlan{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

func (d *defaultServicePlan) SetInternalServiceCatalogInformerFactory(f informers.SharedInformerFactory) {
	scInformer := f.Servicecatalog().InternalVersion().ServiceClasses()
	d.scLister = scInformer.Lister()
	d.SetReadyFunc(scInformer.Informer().HasSynced)
}

func (d *defaultServicePlan) Validate() error {
	if d.scLister == nil {
		return errors.New("missing service class lister")
	}
	return nil
}
