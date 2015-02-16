package git

import (
	"path/filepath"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/util"
)

type Clone struct {
	Git
	util.FileSystem
}

// Download downloads the application source code from the GIT repository
// and checkout the Ref specified in the request.
func (c *Clone) Download(request *api.Request) error {
	targetSourceDir := filepath.Join(request.WorkingDir, "upload", "src")

	if c.ValidCloneSpec(request.Source) {

		if len(request.ContextDir) > 0 {
			targetSourceDir = filepath.Join(request.WorkingDir, "upload", "tmp")
		}
		glog.V(2).Infof("Cloning into %s", targetSourceDir)
		if err := c.Clone(request.Source, targetSourceDir); err != nil {
			glog.Errorf("Git clone failed: %+v", err)
			return err
		}

		if request.Ref != "" {
			glog.V(1).Infof("Checking out ref %s", request.Ref)

			if err := c.Checkout(targetSourceDir, request.Ref); err != nil {
				return err
			}
		}

		if len(request.ContextDir) > 0 {
			originalTargetDir := filepath.Join(request.WorkingDir, "upload", "src")
			c.RemoveDirectory(originalTargetDir)
			err := c.Copy(filepath.Join(targetSourceDir, request.ContextDir), originalTargetDir)
			if err != nil {
				return err
			}
			c.RemoveDirectory(targetSourceDir)
		}

		return nil
	}

	return c.Copy(filepath.Join(request.Source, request.ContextDir), targetSourceDir)
}
