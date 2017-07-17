package jenkinsbootstrapper

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/admission"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	authenticationclient "github.com/openshift/origin/pkg/auth/client"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	jenkinscontroller "github.com/openshift/origin/pkg/build/controller/jenkins"
	"github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/config/cmd"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("openshift.io/JenkinsBootstrapper",
		func(config io.Reader) (admission.Interface, error) {
			return NewJenkinsBootstrapper(), nil
		})
}

type jenkinsBootstrapper struct {
	*admission.Handler

	privilegedRESTClientConfig restclient.Config
	serviceClient              coreclient.ServicesGetter
	openshiftClient            client.Interface

	jenkinsConfig configapi.JenkinsPipelineConfig
}

var _ = oadmission.WantsJenkinsPipelineConfig(&jenkinsBootstrapper{})
var _ = oadmission.WantsRESTClientConfig(&jenkinsBootstrapper{})
var _ = oadmission.WantsOpenshiftClient(&jenkinsBootstrapper{})
var _ = kadmission.WantsInternalKubeClientSet(&jenkinsBootstrapper{})

// NewJenkinsBootstrapper returns an admission plugin that will create required jenkins resources as the user if they are needed.
func NewJenkinsBootstrapper() admission.Interface {
	return &jenkinsBootstrapper{
		Handler: admission.NewHandler(admission.Create),
	}
}

func (a *jenkinsBootstrapper) Admit(attributes admission.Attributes) error {
	if a.jenkinsConfig.AutoProvisionEnabled != nil && !*a.jenkinsConfig.AutoProvisionEnabled {
		return nil
	}
	if len(attributes.GetSubresource()) != 0 {
		return nil
	}
	gr := attributes.GetResource().GroupResource()
	if !buildapi.IsResourceOrLegacy("buildconfigs", gr) && !buildapi.IsResourceOrLegacy("builds", gr) {
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
	if _, err := a.serviceClient.Services(namespace).Get(svcName, metav1.GetOptions{}); !kapierrors.IsNotFound(err) {
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

	impersonatingConfig := authenticationclient.NewImpersonatingConfig(attributes.GetUserInfo(), a.privilegedRESTClientConfig)

	var bulkErr error

	bulk := &cmd.Bulk{
		Mapper: &resource.Mapper{
			RESTMapper:   kapi.Registry.RESTMapper(),
			ObjectTyper:  kapi.Scheme,
			ClientMapper: cmd.ClientMapperFromConfig(&impersonatingConfig),
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

func (q *jenkinsBootstrapper) SetInternalKubeClientSet(c kclientset.Interface) {
	q.serviceClient = c.Core()
}

func (a *jenkinsBootstrapper) SetOpenshiftClient(oclient client.Interface) {
	a.openshiftClient = oclient
}

func (a *jenkinsBootstrapper) Validate() error {
	if a.openshiftClient == nil {
		return fmt.Errorf("missing openshiftClient")
	}
	return nil
}
