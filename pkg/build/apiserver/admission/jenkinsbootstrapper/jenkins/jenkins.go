package jenkins

import (
	"fmt"

	"github.com/golang/glog"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	templatev1 "github.com/openshift/api/template/v1"
	templateclient "github.com/openshift/client-go/template/clientset/versioned"
	"github.com/openshift/origin/pkg/client/templateprocessing"
	serverapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	templatelib "github.com/openshift/origin/pkg/template/templateprocessing"
)

// PipelineTemplate stores the configuration of the
// PipelineStrategy template, used to instantiate the Jenkins service in
// given namespace.
type PipelineTemplate struct {
	Config         serverapi.JenkinsPipelineConfig
	Namespace      string
	templateClient templateclient.Interface
	dynamicClient  dynamic.Interface
}

// NewPipelineTemplate returns a new PipelineTemplate.
func NewPipelineTemplate(ns string, conf serverapi.JenkinsPipelineConfig, templateClient templateclient.Interface, dynamicClient dynamic.Interface) *PipelineTemplate {
	return &PipelineTemplate{
		Config:         conf,
		Namespace:      ns,
		templateClient: templateClient,
		dynamicClient:  dynamicClient,
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

	templateProcessor := templateprocessing.NewDynamicTemplateProcessor(t.dynamicClient)
	processedList, err := templateProcessor.ProcessToList(jenkinsTemplate)
	if err != nil {
		errors = append(errors, fmt.Errorf("processing Jenkins template %s/%s failed: %v", t.Config.TemplateNamespace, t.Config.TemplateName, err))
		return nil, errors
	}

	glog.V(4).Infof("Processed Jenkins pipeline jenkinsTemplate %s/%s", jenkinsTemplate.Namespace, jenkinsTemplate.Name)
	return processedList, errors
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
func substituteTemplateParameters(params map[string]string, t *templatev1.Template) []error {
	var errors []error
	for name, value := range params {
		if len(name) == 0 {
			errors = append(errors, fmt.Errorf("template parameter name cannot be empty (%q)", value))
			continue
		}
		if v := templatelib.GetParameterByName(t, name); v != nil {
			v.Value = value
			v.Generate = ""
		} else {
			errors = append(errors, fmt.Errorf("unknown parameter %q specified for template", name))
		}
	}
	return errors
}
