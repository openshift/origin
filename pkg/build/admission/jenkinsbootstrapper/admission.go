package jenkinsbootstrapper

import (
	"fmt"
	"io"
	"net/http"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/api/latest"
	authenticationclient "github.com/openshift/origin/pkg/auth/client"
	buildapi "github.com/openshift/origin/pkg/build/api"
	jenkinscontroller "github.com/openshift/origin/pkg/build/controller/jenkins"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/config/cmd"
)

func init() {
	admission.RegisterPlugin("openshift.io/JenkinsBootstrapper", func(c clientset.Interface, config io.Reader) (admission.Interface, error) {
		return NewJenkinsBootstrapper(c.Core()), nil
	})
}

type jenkinsBootstrapper struct {
	*admission.Handler

	privilegedRESTClientConfig restclient.Config
	serviceClient              coreclient.ServicesGetter
	openshiftClient            client.Interface

	jenkinsConfig configapi.JenkinsPipelineConfig
}

// NewJenkinsBootstrapper returns an admission plugin that will create required jenkins resources as the user if they are needed.
func NewJenkinsBootstrapper(serviceClient coreclient.ServicesGetter) admission.Interface {
	return &jenkinsBootstrapper{
		Handler:       admission.NewHandler(admission.Create),
		serviceClient: serviceClient,
	}
}

func (a *jenkinsBootstrapper) Admit(attributes admission.Attributes) error {
	if a.jenkinsConfig.AutoProvisionEnabled != nil && !*a.jenkinsConfig.AutoProvisionEnabled {
		return nil
	}
	if len(attributes.GetSubresource()) != 0 {
		return nil
	}
	if attributes.GetResource().GroupResource() != buildapi.Resource("buildconfigs") && attributes.GetResource().GroupResource() != buildapi.Resource("builds") {
		return nil
	}
	if !needsJenkinsTemplate(attributes.GetObject()) {
		return nil
	}

	namespace := attributes.GetNamespace()

	svcName := a.jenkinsConfig.ServiceName
	if len(svcName) == 0 {
		return nil
	}

	// TODO pull this from a cache.
	if _, err := a.serviceClient.Services(namespace).Get(svcName); !kapierrors.IsNotFound(err) {
		// if it isn't a "not found" error, return the error.  Either its nil and there's nothing to do or something went really wrong
		return err
	}

	glog.V(3).Infof("Adding new jenkins service %q to the project %q", svcName, namespace)
	jenkinsTemplate := jenkinscontroller.NewPipelineTemplate(namespace, a.jenkinsConfig, a.openshiftClient)
	objects, errs := jenkinsTemplate.Process()
	if len(errs) > 0 {
		return kutilerrors.NewAggregate(errs)
	}
	if !jenkinsTemplate.HasJenkinsService(objects) {
		return fmt.Errorf("template %s/%s does not contain required service %q", a.jenkinsConfig.TemplateNamespace, a.jenkinsConfig.TemplateName, a.jenkinsConfig.ServiceName)
	}

	impersonatingConfig := a.privilegedRESTClientConfig
	oldWrapTransport := impersonatingConfig.WrapTransport
	impersonatingConfig.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return authenticationclient.NewImpersonatingRoundTripper(attributes.GetUserInfo(), oldWrapTransport(rt))
	}

	var bulkErr error

	bulk := &cmd.Bulk{
		Mapper: &resource.Mapper{
			RESTMapper:  registered.RESTMapper(),
			ObjectTyper: kapi.Scheme,
			ClientMapper: resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
				if latest.OriginKind(mapping.GroupVersionKind) {
					return client.New(&impersonatingConfig)
				}
				return kclient.New(&impersonatingConfig)
			}),
		},
		Op: cmd.Create,
		After: func(info *resource.Info, err error) bool {
			if kapierrors.IsAlreadyExists(err) {
				return false
			}
			if err != nil {
				bulkErr = err
				return true
			}
			return false
		},
	}
	// we're intercepting the error we care about using After
	bulk.Run(objects, namespace)
	if bulkErr != nil {
		return bulkErr
	}

	glog.V(1).Infof("Jenkins Pipeline service %q created", svcName)

	return nil

}

func needsJenkinsTemplate(obj runtime.Object) bool {
	switch t := obj.(type) {
	case *buildapi.Build:
		return t.Spec.Strategy.JenkinsPipelineStrategy != nil
	case *buildapi.BuildConfig:
		return t.Spec.Strategy.JenkinsPipelineStrategy != nil
	default:
		return false
	}
}

func (a *jenkinsBootstrapper) SetJenkinsPipelineConfig(jenkinsConfig configapi.JenkinsPipelineConfig) {
	a.jenkinsConfig = jenkinsConfig
}

func (a *jenkinsBootstrapper) SetRESTClientConfig(restClientConfig restclient.Config) {
	a.privilegedRESTClientConfig = restClientConfig
}

func (a *jenkinsBootstrapper) SetOpenshiftClient(oclient client.Interface) {
	a.openshiftClient = oclient
}
