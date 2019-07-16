package apiserver

import (
	"fmt"
	"io"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/origin/pkg/admission/customresourcevalidation"
	"github.com/openshift/origin/pkg/cmd/server/admission"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kadmission "k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/rest"
)

const PluginName = "config.openshift.io/ValidateAPIServer"

// Register registers a plugin
func Register(plugins *kadmission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (kadmission.Interface, error) {
		return NewValidateAPIServer()
	})
}

type validateCustomResourceWithClient struct {
	kadmission.ValidationInterface

	infrastructureGetter configv1client.InfrastructuresGetter
}

func NewValidateAPIServer() (kadmission.Interface, error) {
	ret := &validateCustomResourceWithClient{}

	delegate, err := customresourcevalidation.NewValidator(
		map[schema.GroupResource]bool{
			configv1.GroupVersion.WithResource("apiservers").GroupResource(): true,
		},
		map[schema.GroupVersionKind]customresourcevalidation.ObjectValidator{
			configv1.GroupVersion.WithKind("APIServer"): apiserverV1{infrastructureGetter: ret.getInfrastructureGetter},
		})
	if err != nil {
		return nil, err
	}
	ret.ValidationInterface = delegate

	return ret, nil
}

var _ admission.WantsRESTClientConfig = &validateCustomResourceWithClient{}

func (a *validateCustomResourceWithClient) getInfrastructureGetter() configv1client.InfrastructuresGetter {
	return a.infrastructureGetter
}

func (a *validateCustomResourceWithClient) SetRESTClientConfig(restClientConfig rest.Config) {
	var err error
	a.infrastructureGetter, err = configv1client.NewForConfig(&restClientConfig)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
}

func (a *validateCustomResourceWithClient) ValidateInitialization() error {
	if a.infrastructureGetter == nil {
		return fmt.Errorf(PluginName + " needs an infrastructureGetter")
	}

	if initializationValidator, ok := a.ValidationInterface.(kadmission.InitializationValidator); ok {
		return initializationValidator.ValidateInitialization()
	}

	return nil
}
