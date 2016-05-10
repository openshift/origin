package build

import (
	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/util"
)

// DefaultCleaner provides a cleaner for most STI build use-cases. It cleans the
// temporary directories created by STI build and it also cleans the temporary
// Docker images produced by LayeredBuild
type DefaultCleaner struct {
	util.FileSystem
	docker.Docker
}

// Cleanup removes the temporary directories where the sources were stored for build.
func (c *DefaultCleaner) Cleanup(config *api.Config) {
	if config.PreserveWorkingDir {
		glog.Infof("Temporary directory '%s' will be saved, not deleted", config.WorkingDir)
	} else {
		glog.V(2).Infof("Removing temporary directory %s", config.WorkingDir)
		c.RemoveDirectory(config.WorkingDir)
	}
	if config.LayeredBuild {
		glog.V(2).Infof("Removing temporary image %s", config.BuilderImage)
		c.RemoveImage(config.BuilderImage)
	}
}
