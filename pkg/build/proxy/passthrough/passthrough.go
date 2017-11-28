package passthrough

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/build/proxy/interceptor"
)

type Server struct {
	Proxy interceptor.Proxy
}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var i interceptor.Interface = interceptor.Allow

	switch path := r.URL.Path; {
	case r.Method == "POST" && interceptor.IsBuildImageEndpoint(path):
		i = &buildInterceptor{}
	}

	s.Proxy.Intercept(i, w, r)
}

type buildInterceptor struct {
}

func (i *buildInterceptor) InterceptRequest(req *http.Request) error {
	options := &interceptor.BuildImageOptions{}
	if err := interceptor.StrictDecodeFromQuery(options, req.URL.Query()); err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("build request rejected because of an unrecogized query param: %v", err))
	}
	glog.V(4).Infof("Build request found: %#v", options)

	return nil
}

func (i *buildInterceptor) InterceptResponse(r *http.Response) error {
	return nil
}
