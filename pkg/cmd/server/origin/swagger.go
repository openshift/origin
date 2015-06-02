package origin

import (
	"github.com/emicklei/go-restful/swagger"
	"github.com/golang/glog"
)

var apiInfo = map[string]swagger.Info{
	OpenShiftAPIPrefixV1: {
		Title:       "OpenShift v1 REST API",
		Description: `The OpenShift API exposes operations for managing an OpenShift cluster, including security and user management, application deployments, image and source builds, HTTP(s) routing, and project management.`,
	},
	KubernetesAPIPrefix + "/v1": {
		Title:       "Kubernetes v1 REST API",
		Description: `The Kubernetes API allows you to run containerized applications, bind persistent storage, link those applications through service discovery, and manage the cluster infrastructure.`,
	},
}

// customizeSwaggerDefinition applies selective patches to the swagger API docs
// TODO: move most of these upstream or to go-restful
func customizeSwaggerDefinition(api map[string]swagger.ApiDeclaration) {
	for path, info := range apiInfo {
		if dec, ok := api[path]; ok {
			if len(info.Title) > 0 {
				dec.Info.Title = info.Title
			}
			if len(info.Description) > 0 {
				dec.Info.Description = info.Description
			}
			api[path] = dec
		} else {
			glog.Warningf("No API exists for predefined swagger description %s", path)
		}
	}
	for _, version := range []string{OpenShiftAPIPrefixV1} {
		model := api[version].Models["runtime.RawExtension"]
		model.Required = []string{}
		model.Properties = map[string]swagger.ModelProperty{}
		model.Description = "this may be any JSON object with a 'kind' and 'apiVersion' field; and is preserved unmodified by processing"
		api[version].Models["runtime.RawExtension"] = model

		model = api[version].Models["patch.Object"]
		model.Description = "represents an object patch, which may be any of: JSON patch (RFC 6902), JSON merge patch (RFC 7396), or the Kubernetes strategic merge patch"
		api[version].Models["patch.Object"] = model
	}
}
