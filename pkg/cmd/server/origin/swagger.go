package origin

import (
	"github.com/emicklei/go-restful/swagger"
	"github.com/golang/glog"
)

var apiInfo = map[string]swagger.Info{
	OpenShiftAPIPrefixV1: {
		Title:       "OpenShift v1 REST API",
		Description: `The OpenShift API exposes operations for managing an enterprise Kubernetes cluster, including security and user management, application deployments, image and source builds, HTTP(s) routing, and project management.`,
	},
	KubernetesAPIPrefix + "/v1": {
		Title:       "Kubernetes v1 REST API",
		Description: `The Kubernetes API allows you to run containerized applications, bind persistent storage, link those applications through service discovery, and manage the cluster infrastructure.`,
	},
}

// customizeSwaggerDefinition applies selective patches to the swagger API docs
// TODO: move most of these upstream or to go-restful
func customizeSwaggerDefinition(api *swagger.ApiDeclarationList) {
	for path, info := range apiInfo {
		if dec, ok := api.At(path); ok {
			if len(info.Title) > 0 {
				dec.Info.Title = info.Title
			}
			if len(info.Description) > 0 {
				dec.Info.Description = info.Description
			}
			api.Put(path, dec)
		} else {
			glog.Warningf("No API exists for predefined swagger description %s", path)
		}
	}
	for _, version := range []string{OpenShiftAPIPrefixV1} {
		apiDeclaration, _ := api.At(version)
		models := &apiDeclaration.Models

		model, _ := models.At("runtime.RawExtension")
		model.Required = []string{}
		model.Properties = swagger.ModelPropertyList{}
		model.Description = "this may be any JSON object with a 'kind' and 'apiVersion' field; and is preserved unmodified by processing"
		models.Put("runtime.RawExtension", model)

		model, _ = models.At("patch.Object")
		model.Description = "represents an object patch, which may be any of: JSON patch (RFC 6902), JSON merge patch (RFC 7396), or the Kubernetes strategic merge patch"
		models.Put("patch.Object", model)

		apiDeclaration.Models = *models
		api.Put(version, apiDeclaration)
	}
}
