package proxy

import (
	"fmt"
	"math/rand"
	"net/http"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	gorillacontext "github.com/gorilla/context"

	"github.com/openshift/origin/pkg/build/proxy/dockerproxy"
	"github.com/openshift/origin/pkg/build/proxy/imagebuilder"
	"github.com/openshift/origin/pkg/build/proxy/passthrough"
	"github.com/openshift/origin/pkg/version"
)

type Server struct {
	ListenAddrs []string
	Mode        string
	Client      *docker.Client

	AllowHost string
}

func (s *Server) Start() error {
	if len(s.ListenAddrs) == 0 {
		return fmt.Errorf("must specify one or more addresses to listen on")
	}

	client := s.Client
	if client == nil {
		c, err := docker.NewClientFromEnv()
		if err != nil {
			return err
		}
		client = c
	}

	allowHost := s.AllowHost
	if len(allowHost) == 0 {
		allowHost = fmt.Sprintf("%d.openshift-build-proxy.local:1000", rand.Int31())
	}

	authorizer := &passthrough.DefaultAuthorizer{
		Client: client,
		File:   "/tmp/build-proxy-openshift-token",
	}

	p, err := dockerproxy.NewProxy(dockerproxy.Config{
		Client:      client,
		ListenAddrs: s.ListenAddrs,
	})
	if err != nil {
		return err
	}

	var server http.Handler
	switch s.Mode {
	case "passthrough":
		server = passthrough.Server{
			Proxy: p,
		}
	case "imagebuilder":
		server = imagebuilder.Server{
			Handler: p,
			Client:  client,
		}
	default:
		return fmt.Errorf("unrecognized proxy mode %q", s.Mode)
	}
	server = NewAuthorizingDockerAPIFilter(server, allowHost, authorizer)
	server = gorillacontext.ClearHandler(server)

	dockerproxy.ServeWithReady(server, p.Listen(), func() {
		glog.Infof("Proxy (version: %s) is accepting requests for %s", version.Get().String(), allowHost)
	})
	return nil
}
