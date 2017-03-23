package servicebroker

import (
	"reflect"
	"testing"

	schema "github.com/lestrrat/go-jsschema"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	kapi "k8s.io/kubernetes/pkg/api"
)

func TestServiceFromTemplate(t *testing.T) {
	template := &templateapi.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "name",
			UID:  "ee33151d-a34d-442d-a0ca-6353b73a58fd",
			Annotations: map[string]string{
				"description": "description",
				"tags":        "tag1,tag2",
				"openshift.io/display-name":                   "displayName",
				"iconClass":                                   "iconClass",
				"template.openshift.io/long-description":      "longDescription",
				"template.openshift.io/provider-display-name": "providerDisplayName",
				"template.openshift.io/documentation-url":     "documentationURL",
				"template.openshift.io/support-url":           "supportURL",
			},
		},
		Parameters: []templateapi.Parameter{
			{
				Name:     "param1",
				Required: true,
			},
			{
				Name: "param2",
			},
		},
	}

	expectedService := &api.Service{
		Name:        "name",
		ID:          "ee33151d-a34d-442d-a0ca-6353b73a58fd",
		Description: "description",
		Tags:        []string{"tag1", "tag2"},
		Bindable:    true,
		Metadata: map[string]interface{}{
			"providerDisplayName":            "providerDisplayName",
			"documentationUrl":               "documentationURL",
			"supportUrl":                     "supportURL",
			"displayName":                    "displayName",
			"console.openshift.io/iconClass": "iconClass",
			"longDescription":                "longDescription",
		},
		Plans: []api.Plan{
			{
				ID:          "ee33151d-a34d-442d-a0ca-6353b73a58fd",
				Name:        "default",
				Description: "Default plan",
				Free:        true,
				Bindable:    true,
				Schemas: api.Schema{
					ServiceInstances: api.ServiceInstances{
						Create: map[string]*schema.Schema{
							"parameters": {
								Type:      schema.PrimitiveTypes{schema.ObjectType},
								SchemaRef: "http://json-schema.org/draft-04/schema",
								Required: []string{
									"template.openshift.io/namespace",
									"template.openshift.io/requester-username",
									"param1",
								},
								Properties: map[string]*schema.Schema{
									"template.openshift.io/namespace": {
										Title:       "Template service broker: namespace",
										Description: "OpenShift namespace in which to provision service",
										Type:        schema.PrimitiveTypes{schema.StringType},
									},
									"template.openshift.io/requester-username": {
										Title:       "Template service broker: requester username",
										Description: "OpenShift user requesting provision/bind",
										Type:        schema.PrimitiveTypes{schema.StringType},
									},
									"param1": {
										Default: "",
										Type:    schema.PrimitiveTypes{schema.StringType},
									},
									"param2": {
										Default: "",
										Type:    schema.PrimitiveTypes{schema.StringType},
									},
								},
							},
						},
					},
					ServiceBindings: api.ServiceBindings{
						Create: map[string]*schema.Schema{
							"parameters": {
								Type:      schema.PrimitiveTypes{schema.ObjectType},
								SchemaRef: "http://json-schema.org/draft-04/schema",
								Required:  []string{"template.openshift.io/requester-username"},
								Properties: map[string]*schema.Schema{
									"template.openshift.io/requester-username": {
										Title:       "Template service broker: requester username",
										Description: "OpenShift user requesting provision/bind",
										Type:        schema.PrimitiveTypes{schema.StringType},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	service := serviceFromTemplate(template)

	if !reflect.DeepEqual(service, expectedService) {
		t.Error("service did not match expectedService")
	}
}
