package configprocessing

import (
	"github.com/openshift/origin/pkg/api/legacy"
	oauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

var LegacyAPIGroupPrefixes = sets.NewString(genericapiserver.DefaultLegacyAPIPrefix, legacy.RESTPrefix)

func OpenshiftRequestInfoResolver() apirequest.RequestInfoResolver {
	// Default API request info factory
	requestInfoFactory := &apirequest.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "osapi", "oapi", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi"),
	}
	personalSARRequestInfoResolver := oauthorizer.NewPersonalSARRequestInfoResolver(requestInfoFactory)
	projectRequestInfoResolver := oauthorizer.NewProjectRequestInfoResolver(personalSARRequestInfoResolver)
	return projectRequestInfoResolver
}
