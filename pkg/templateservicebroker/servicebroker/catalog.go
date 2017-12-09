package servicebroker

import (
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/golang/glog"
	jsschema "github.com/lestrrat/go-jsschema"

	templateapiv1 "github.com/openshift/api/template/v1"
	oapi "github.com/openshift/origin/pkg/api"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

const (
	noDescriptionProvided = "No description provided."
)

// Map OpenShift template annotations to open service broker metadata field
// community standards.
var annotationMap = map[string]string{
	oapi.OpenShiftDisplayName:                   api.ServiceMetadataDisplayName,
	oapi.OpenShiftLongDescriptionAnnotation:     api.ServiceMetadataLongDescription,
	oapi.OpenShiftProviderDisplayNameAnnotation: api.ServiceMetadataProviderDisplayName,
	oapi.OpenShiftDocumentationURLAnnotation:    api.ServiceMetadataDocumentationURL,
	oapi.OpenShiftSupportURLAnnotation:          api.ServiceMetadataSupportURL,
	templateapi.IconClassAnnotation:             templateapi.ServiceMetadataIconClass,
}

// serviceFromTemplate populates an open service broker service response from
// an OpenShift template.
func serviceFromTemplate(template *templateapiv1.Template) *api.Service {
	metadata := make(map[string]interface{})
	for srcname, dstname := range annotationMap {
		if value, ok := template.Annotations[srcname]; ok {
			metadata[dstname] = value
		}
	}

	properties := map[string]*jsschema.Schema{}
	paramOrdering := []string{}
	required := []string{}
	for _, param := range template.Parameters {
		properties[param.Name] = &jsschema.Schema{
			Title:       param.DisplayName,
			Description: param.Description,
			Default:     param.Value,
			Type:        []jsschema.PrimitiveType{jsschema.StringType},
		}
		if param.Required && param.Generate == "" {
			required = append(required, param.Name)
		}
		paramOrdering = append(paramOrdering, param.Name)
	}

	bindable := strings.ToLower(template.Annotations[templateapi.BindableAnnotation]) != "false"

	plan := api.Plan{
		ID:          string(template.UID), // TODO: this should be a unique value
		Name:        "default",
		Description: "Default plan",
		Free:        true,
		Bindable:    bindable,
		Schemas: api.Schema{
			ServiceInstance: api.ServiceInstances{
				Create: map[string]*jsschema.Schema{
					"parameters": {
						SchemaRef:  jsschema.SchemaURL,
						Type:       []jsschema.PrimitiveType{jsschema.ObjectType},
						Properties: properties,
						Required:   required,
					},
				},
			},
			ServiceBinding: api.ServiceBindings{
				Create: map[string]*jsschema.Schema{
					"parameters": {
						SchemaRef:  jsschema.SchemaURL,
						Type:       []jsschema.PrimitiveType{jsschema.ObjectType},
						Properties: map[string]*jsschema.Schema{},
						Required:   []string{},
					},
				},
			},
		},
	}

	// This metadata ensures the template parameters are displayed in the
	// service catalog in the same order as they are defined in the template.
	plan.Metadata = make(map[string]interface{})
	plan.Metadata["schemas"] = api.ParameterSchemas{
		ServiceInstance: api.ParameterSchema{
			Create: api.OpenShiftMetadata{
				OpenShiftFormDefinition: paramOrdering,
			},
		},
	}

	description := template.Annotations["description"]
	if description == "" {
		description = noDescriptionProvided
	}

	return &api.Service{
		Name:        template.Name,
		ID:          string(template.UID),
		Description: description,
		Tags:        strings.Split(template.Annotations["tags"], ","),
		Bindable:    bindable,
		Metadata:    metadata,
		Plans:       []api.Plan{plan},
	}
}

// Catalog returns our service catalog (one service per OpenShift template in
// configured namespace(s)).
func (b *Broker) Catalog() *api.Response {
	glog.V(4).Infof("Template service broker: Catalog")

	var services []*api.Service

	for namespace := range b.templateNamespaces {
		templates, err := b.lister.Templates(namespace).List(labels.Everything())
		if err != nil {
			return api.InternalServerError(err)
		}

		for _, template := range templates {
			services = append(services, serviceFromTemplate(template))
		}
	}

	return api.NewResponse(http.StatusOK, &api.CatalogResponse{Services: services}, nil)
}
