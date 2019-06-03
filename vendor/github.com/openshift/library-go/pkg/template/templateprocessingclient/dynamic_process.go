package templateprocessingclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"

	templatev1 "github.com/openshift/api/template/v1"
)

type DynamicTemplateProcessor interface {
	ProcessToList(template *templatev1.Template) (*unstructured.UnstructuredList, error)
	ProcessToListFromUnstructured(unstructuredTemplate *unstructured.Unstructured) (*unstructured.UnstructuredList, error)
}

type dynamicTemplateProcessor struct {
	client dynamic.Interface
}

func NewDynamicTemplateProcessor(client dynamic.Interface) DynamicTemplateProcessor {
	return &dynamicTemplateProcessor{client: client}
}

func (c *dynamicTemplateProcessor) ProcessToList(template *templatev1.Template) (*unstructured.UnstructuredList, error) {
	versionedTemplate, err := scheme.ConvertToVersion(template, templatev1.GroupVersion)
	if err != nil {
		return nil, err
	}
	unstructuredTemplate, err := runtime.DefaultUnstructuredConverter.ToUnstructured(versionedTemplate)
	if err != nil {
		return nil, err
	}

	return c.ProcessToListFromUnstructured(&unstructured.Unstructured{Object: unstructuredTemplate})
}

func (c *dynamicTemplateProcessor) ProcessToListFromUnstructured(unstructuredTemplate *unstructured.Unstructured) (*unstructured.UnstructuredList, error) {
	processedTemplate, err := c.client.Resource(templatev1.GroupVersion.WithResource("processedtemplates")).
		Namespace("default").Create(unstructuredTemplate, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// convert the template into something we iterate over as a list
	if err := unstructured.SetNestedField(processedTemplate.Object, processedTemplate.Object["objects"], "items"); err != nil {
		return nil, err
	}
	return processedTemplate.ToList()
}

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(templatev1.Install(scheme))
}
