package empty

import (
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/scm/git"
	utillog "github.com/openshift/source-to-image/pkg/util/log"
)

var log = utillog.StderrLog

// Noop is for build configs with an empty Source definition, where
// the assemble script is responsible for retrieving source
type Noop struct {
}

// Download is a no-op downloader so that Noop satisfies build.Downloader
func (n *Noop) Download(config *api.Config) (*git.SourceInfo, error) {
	log.V(1).Info("No source location defined (the assemble script is responsible for obtaining the source)")

	return &git.SourceInfo{}, nil
}
