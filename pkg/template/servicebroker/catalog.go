package servicebroker

import (
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/golang/glog"
	jsschema "github.com/lestrrat/go-jsschema"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

const (
	namespaceTitle       = "Template service broker: namespace"
	namespaceDescription = "OpenShift namespace in which to provision service"

	// the following should go away with catalog<->broker support for passing
	// identity information.
	requesterUsernameTitle       = "Template service broker: requester username"
	requesterUsernameDescription = "OpenShift user requesting provision/bind"
)

// Map OpenShift template annotations to open service broker metadata field
// community standards.
var annotationMap = map[string]string{
	oapi.OpenShiftDisplayName:                 api.ServiceMetadataDisplayName,
	templateapi.IconClassAnnotation:           templateapi.ServiceMetadataIconClass,
	templateapi.LongDescriptionAnnotation:     api.ServiceMetadataLongDescription,
	templateapi.ProviderDisplayNameAnnotation: api.ServiceMetadataProviderDisplayName,
	templateapi.DocumentationURLAnnotation:    api.ServiceMetadataDocumentationURL,
	templateapi.SupportURLAnnotation:          api.ServiceMetadataSupportURL,
}

// serviceFromTemplate populates an open service broker service response from
// an OpenShift template.
func serviceFromTemplate(template *templateapi.Template) *api.Service {
	metadata := make(map[string]interface{})
	for srcname, dstname := range annotationMap {
		if value, ok := template.Annotations[srcname]; ok {
			metadata[dstname] = value
		}
	}

	properties := map[string]*jsschema.Schema{
		templateapi.NamespaceParameterKey: {
			Title:       namespaceTitle,
			Description: namespaceDescription,
			Type:        []jsschema.PrimitiveType{jsschema.StringType},
		},
		templateapi.RequesterUsernameParameterKey: {
			Title:       requesterUsernameTitle,
			Description: requesterUsernameDescription,
			Type:        []jsschema.PrimitiveType{jsschema.StringType},
		},
	}
	required := []string{templateapi.NamespaceParameterKey, templateapi.RequesterUsernameParameterKey}
	for _, param := range template.Parameters {
		properties[param.Name] = &jsschema.Schema{
			Title:       param.DisplayName,
			Description: param.Description,
			Default:     param.Value,
			Type:        []jsschema.PrimitiveType{jsschema.StringType},
		}
		if param.Required {
			required = append(required, param.Name)
		}
	}

	plan := api.Plan{
		ID:          string(template.UID), // TODO: this should be a unique value
		Name:        "default",
		Description: "Default plan",
		Free:        true,
		Bindable:    true,
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
						SchemaRef: jsschema.SchemaURL,
						Type:      []jsschema.PrimitiveType{jsschema.ObjectType},
						Properties: map[string]*jsschema.Schema{
							templateapi.RequesterUsernameParameterKey: {
								Title:       requesterUsernameTitle,
								Description: requesterUsernameDescription,
								Type:        []jsschema.PrimitiveType{jsschema.StringType},
							},
						},
						Required: []string{templateapi.RequesterUsernameParameterKey},
					},
				},
			},
		},
	}

	return &api.Service{
		Name:        template.Name,
		ID:          string(template.UID),
		Description: template.Annotations["description"],
		Tags:        strings.Split(template.Annotations["tags"], ","),
		Bindable:    true,
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
