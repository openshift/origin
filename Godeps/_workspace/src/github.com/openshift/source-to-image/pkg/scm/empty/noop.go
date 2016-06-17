package empty

import (
	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
)

// Noop is for build configs with an empty Source definition, where
// the assemble script is responsible for retrieving source
type Noop struct {
}

func (n *Noop) Download(config *api.Config) (*api.SourceInfo, error) {
	glog.V(1).Info("No source location defined (the assemble script is responsible for obtaining the source)")

	return &api.SourceInfo{}, nil
}
