package jenkins

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/template/templateprocessing"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	serverapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateinternal "github.com/openshift/origin/pkg/template/client/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

// PipelineTemplate stores the configuration of the
// PipelineStrategy template, used to instantiate the Jenkins service in
// given namespace.
type PipelineTemplate struct {
	Config         serverapi.JenkinsPipelineConfig
	Namespace      string
	templateClient templateclient.Interface
}

// NewPipelineTemplate returns a new PipelineTemplate.
func NewPipelineTemplate(ns string, conf serverapi.JenkinsPipelineConfig, templateClient templateclient.Interface) *PipelineTemplate {
	return &PipelineTemplate{
		Config:         conf,
		Namespace:      ns,
		templateClient: templateClient,
	}
}

// Process processes the Jenkins template. If an error occurs
func (t *PipelineTemplate) Process() (*unstructured.UnstructuredList, []error) {
	var errors []error
	jenkinsTemplate, err := t.templateClient.Template().Templates(t.Config.TemplateNamespace).Get(t.Config.TemplateName, metav1.GetOptions{})
	if err != nil {
		if kerrs.IsNotFound(err) {
			errors = append(errors, fmt.Errorf("Jenkins pipeline template %s/%s not found", t.Config.TemplateNamespace, t.Config.TemplateName))
		} else {
			errors = append(errors, err)
		}
		return nil, errors
	}
	errors = append(errors, substituteTemplateParameters(t.Config.Parameters, jenkinsTemplate)...)
	processorClient := templateinternal.NewTemplateProcessorClient(t.templateClient.Template().RESTClient(), t.Namespace)
	pTemplate, err := processorClient.Process(jenkinsTemplate)
	if err != nil {
		errors = append(errors, fmt.Errorf("processing Jenkins template %s/%s failed: %v", t.Config.TemplateNamespace, t.Config.TemplateName, err))
		return nil, errors
	}

	objectsToCreate := &kapi.List{}
	for i := range pTemplate.Objects {
		// use .Objects[i] in append to avoid range memory address reuse
		objectsToCreate.Items = append(objectsToCreate.Items, pTemplate.Objects[i])
	}

	// TODO, stop doing this crazy thing, but for now it's a very simple way to get the unstructured objects we need
	jsonBytes, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(legacyscheme.Scheme.PrioritizedVersionsAllGroups()...), objectsToCreate)
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}
	uncastList, err := runtime.Decode(unstructured.UnstructuredJSONScheme, jsonBytes)
	if err != nil {
		errors = append(errors, err)
		return nil, errors
	}

	glog.V(4).Infof("Processed Jenkins pipeline jenkinsTemplate %s/%s", pTemplate.Namespace, pTemplate.Namespace)
	return uncastList.(*unstructured.UnstructuredList), errors
}

// HasJenkinsService searches the template items and return true if the expected
// Jenkins service is contained in template.
func (t *PipelineTemplate) HasJenkinsService(items *unstructured.UnstructuredList) bool {
	for _, item := range items.Items {
		glog.Infof("Jenkins Pipeline template object %q with name %q", item.GetName(), item.GetObjectKind().GroupVersionKind())

		if item.GetName() == t.Config.ServiceName && item.GetObjectKind().GroupVersionKind().GroupKind() == kapi.Kind("Service") {
			return true
		}
	}
	return false
}

// substituteTemplateParameters injects user specified parameter values into the Template
func substituteTemplateParameters(params map[string]string, t *templateapi.Template) []error {
	var errors []error
	for name, value := range params {
		if len(name) == 0 {
			errors = append(errors, fmt.Errorf("template parameter name cannot be empty (%q)", value))
			continue
		}
		if v := templateprocessing.GetParameterByName(t, name); v != nil {
			v.Value = value
			v.Generate = ""
		} else {
			errors = append(errors, fmt.Errorf("unknown parameter %q specified for template", name))
		}
	}
	return errors
}
