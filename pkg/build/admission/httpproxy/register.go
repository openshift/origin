package httpproxy

import (
	_ "github.com/openshift/origin/pkg/build/admission/httpproxy/latest"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&ProxyConfig{},
	)
}

func (*ProxyConfig) IsAnAPIObject() {}
