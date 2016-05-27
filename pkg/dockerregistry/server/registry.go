package server

import (
	log "github.com/Sirupsen/logrus"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/middleware/registry"
)

// dockerRegistry represents a collection of repositories, addressable by name.
// This variable holds the object created by the docker/distribution. We import
// it into our namespace because there are no other ways to access it. In other
// cases it is hidden from us.
var dockerRegistry distribution.Namespace

func init() {
	middleware.Register("openshift", func(ctx context.Context, registry distribution.Namespace, options map[string]interface{}) (distribution.Namespace, error) {
		log.Info("OpenShift registry middleware initializing")
		dockerRegistry = registry
		return dockerRegistry, nil
	})
}
