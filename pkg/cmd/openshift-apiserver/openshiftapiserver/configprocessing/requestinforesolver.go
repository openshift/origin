package configprocessing

import (
	oauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func OpenshiftRequestInfoResolver() apirequest.RequestInfoResolver {
	// Default API request info factory
	requestInfoFactory := &apirequest.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
	personalSARRequestInfoResolver := oauthorizer.NewPersonalSARRequestInfoResolver(requestInfoFactory)
	projectRequestInfoResolver := oauthorizer.NewProjectRequestInfoResolver(personalSARRequestInfoResolver)
	return projectRequestInfoResolver
}
