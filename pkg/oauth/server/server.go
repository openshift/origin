package server

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/api/v1beta1"
	"github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	"github.com/openshift/origin/pkg/oauth/registry/authorizetoken"
	"github.com/openshift/origin/pkg/oauth/registry/client"
	"github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	"github.com/openshift/origin/pkg/oauth/registry/etcd"
)

type Server struct {
	storage map[string]apiserver.RESTStorage
}

func NewServer(helper tools.EtcdHelper) *Server {
	registry := etcd.New(helper)
	s := &Server{
		storage: map[string]apiserver.RESTStorage{
			"accessTokens":         accesstoken.NewREST(registry),
			"authorizeTokens":      authorizetoken.NewREST(registry),
			"clients":              client.NewREST(registry),
			"clientAuthorizations": clientauthorization.NewREST(registry),
		},
	}
	return s
}

// API_v1beta1 returns the resources and codec for API version v1beta1.
func (s *Server) API_v1beta1() (map[string]apiserver.RESTStorage, runtime.Codec) {
	storage := make(map[string]apiserver.RESTStorage)
	for k, v := range s.storage {
		storage[k] = v
	}
	return storage, v1beta1.Codec
}
