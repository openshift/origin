package v1

import (
	"github.com/openshift/origin/pkg/cmd/server/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&ProxyConfig{},
	)
}

func (*ProxyConfig) IsAnAPIObject() {}
