package features

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apiserver/pkg/admission"
)

const DenyDeleteFeaturesPluginName = "config.openshift.io/DenyDeleteFeatures"

func RegisterDenyDeleteFeatures(plugins *admission.Plugins) {
	plugins.Register(DenyDeleteFeaturesPluginName, func(config io.Reader) (admission.Interface, error) {
		return newDenyDeleteFeatures()
	})
}

// denyDeleteFeatures prevents anyone from deleting features.config.openshift.io
type denyDeleteFeatures struct {
	*admission.Handler
}

func newDenyDeleteFeatures() (admission.Interface, error) {
	return &denyDeleteFeatures{
		Handler: admission.NewHandler(admission.Delete),
	}, nil
}

var _ admission.ValidationInterface = &denyDeleteFeatures{}

func (a *denyDeleteFeatures) Validate(attributes admission.Attributes) error {
	if len(attributes.GetSubresource()) > 0 {
		return nil
	}
	if attributes.GetResource().GroupResource() != (schema.GroupResource{Group: "config.openshift.io", Resource: "features"}) {
		return nil
	}

	return admission.NewForbidden(attributes, fmt.Errorf("deleting features.config.openshift.io is not allowed"))
}
