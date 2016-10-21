package empty

import (
	"github.com/openshift/source-to-image/pkg/api"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog

// Noop is for build configs with an empty Source definition, where
// the assemble script is responsible for retrieving source
type Noop struct {
}

// Download is a no-op downloader so that Noop satisfies build.Downloader
func (n *Noop) Download(config *api.Config) (*api.SourceInfo, error) {
	glog.V(1).Info("No source location defined (the assemble script is responsible for obtaining the source)")

	return &api.SourceInfo{}, nil
}
