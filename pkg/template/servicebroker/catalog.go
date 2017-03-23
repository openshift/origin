package servicebroker

import (
	"net/http"
	"strings"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

var annotationMap = map[string]string{
	oapi.OpenShiftDisplayName:                 api.ServiceMetadataDisplayName,
	templateapi.IconClassAnnotation:           templateapi.ServiceMetadataIconClass,
	templateapi.LongDescriptionAnnotation:     api.ServiceMetadataLongDescription,
	templateapi.ProviderDisplayNameAnnotation: api.ServiceMetadataProviderDisplayName,
	templateapi.DocumentationURLAnnotation:    api.ServiceMetadataDocumentationURL,
	templateapi.SupportURLAnnotation:          api.ServiceMetadataSupportURL,
}

func serviceFromTemplate(template *templateapi.Template) *api.Service {
	metadata := make(map[string]interface{})
	for srcname, dstname := range annotationMap {
		if value, ok := template.Annotations[srcname]; ok {
			metadata[dstname] = value
		}
	}

	// TODO: list template parameters (https://github.com/openservicebrokerapi/servicebroker/issues/59)

	return &api.Service{
		Name:        template.Name,
		ID:          string(template.UID),
		Description: template.Annotations["description"],
		Tags:        strings.Split(template.Annotations["tags"], ","),
		Bindable:    true,
		Metadata:    metadata,
		Plans:       plans,
	}
}

func (b *Broker) Catalog() *api.Response {
	templates, err := b.lister.List()
	if err != nil {
		return api.InternalServerError(err)
	}

	services := make([]*api.Service, len(templates))
	for i, template := range templates {
		services[i] = serviceFromTemplate(template)
	}

	return api.NewResponse(http.StatusOK, &api.CatalogResponse{Services: services}, nil)
}
