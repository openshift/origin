// Package server wraps storage driver of docker/distribution. Module changes
// repository name in read requests for objects in the repository.
package server

import (
	"fmt"
	"strings"

	context "github.com/docker/distribution/context"
	storagedriver "github.com/docker/distribution/registry/storage/driver"
	registrystorage "github.com/docker/distribution/registry/storage/driver/middleware"
)

func init() {
	registrystorage.Register("openshift", registrystorage.InitFunc(newLocalStorageDriver))
}

type localStorageDriver struct {
	storagedriver.StorageDriver
}

var _ storagedriver.StorageDriver = &localStorageDriver{}

func newLocalStorageDriver(storageDriver storagedriver.StorageDriver, options map[string]interface{}) (storagedriver.StorageDriver, error) {
	return &localStorageDriver{storageDriver}, nil
}

func (s *localStorageDriver) GetContent(ctx context.Context, path string) ([]byte, error) {
	reponame, forceReponame := ctx.Value(StorageGetContentName).(string)
	namespace, forceNamespace := ctx.Value(StorageGetContentNamespace).(string)

	if forceReponame || forceNamespace {
		// Layout description is a private information inside the docker/distribution. So we support
		// the layout of a particular version and hardcode it.
		// Supported path: /docker/registry/v2/repositories/<name>/...
		if !strings.HasPrefix(path, "/docker/registry/v2/") {
			return nil, fmt.Errorf("Unsupported filesystem layout: %q", path)
		}

		pathParts := strings.Split(path, "/")

		if pathParts[4] == "repositories" {
			if forceNamespace {
				pathParts[5] = namespace
			}
			if forceReponame {
				pathParts[6] = reponame
			}
			path = strings.Join(pathParts, "/")
		}
	}

	return s.StorageDriver.GetContent(ctx, path)
}
