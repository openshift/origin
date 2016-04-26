package authorizer

import (
	"net/http"

	kapi "k8s.io/kubernetes/pkg/api"
	kapiserver "k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/util/sets"
)

type browserSafeRequestInfoResolver struct {
	// infoResolver is used to determine info for the request
	infoResolver RequestInfoResolver

	// contextMapper is used to look up the context corresponding to a request
	// to obtain the user associated with the request
	contextMapper kapi.RequestContextMapper

	// list of groups, any of which indicate the request is authenticated
	authenticatedGroups sets.String
}

func NewBrowserSafeRequestInfoResolver(contextMapper kapi.RequestContextMapper, authenticatedGroups sets.String, infoResolver RequestInfoResolver) RequestInfoResolver {
	return &browserSafeRequestInfoResolver{
		contextMapper:       contextMapper,
		authenticatedGroups: authenticatedGroups,
		infoResolver:        infoResolver,
	}
}

func (a *browserSafeRequestInfoResolver) GetRequestInfo(req *http.Request) (kapiserver.RequestInfo, error) {
	requestInfo, err := a.infoResolver.GetRequestInfo(req)
	if err != nil {
		return requestInfo, err
	}

	if !requestInfo.IsResourceRequest {
		return requestInfo, nil
	}

	isProxyVerb := requestInfo.Verb == "proxy"
	isProxySubresource := requestInfo.Subresource == "proxy"

	if !isProxyVerb && !isProxySubresource {
		// Requests to non-proxy resources don't expose HTML or HTTP-handling user content to browsers
		return requestInfo, nil
	}

	if len(req.Header.Get("X-CSRF-Token")) > 0 {
		// Browsers cannot set custom headers on direct requests
		return requestInfo, nil
	}

	if ctx, hasContext := a.contextMapper.Get(req); hasContext {
		user, hasUser := kapi.UserFrom(ctx)
		if hasUser && a.authenticatedGroups.HasAny(user.GetGroups()...) {
			// An authenticated request indicates this isn't a browser page load.
			// Browsers cannot make direct authenticated requests.
			// This depends on the API not enabling basic or cookie-based auth.
			return requestInfo, nil
		}

	}

	// TODO: compare request.Host to a list of hosts allowed for the requestInfo.Namespace (e.g. <namespace>.proxy.example.com)

	if isProxyVerb {
		requestInfo.Verb = "unsafeproxy"
	}
	if isProxySubresource {
		requestInfo.Subresource = "unsafeproxy"
	}

	return requestInfo, nil
}
