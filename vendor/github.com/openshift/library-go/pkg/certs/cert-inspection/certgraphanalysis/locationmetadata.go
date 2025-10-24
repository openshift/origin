//go:build linux
// +build linux

package certgraphanalysis

import (
	"os"
	"syscall"

	"github.com/moby/sys/user"
	"github.com/opencontainers/selinux/go-selinux"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

func getOnDiskLocationMetadata(path string) *certgraphapi.OnDiskLocationWithMetadata {
	ret := &certgraphapi.OnDiskLocationWithMetadata{
		OnDiskLocation: certgraphapi.OnDiskLocation{
			Path: path,
		},
	}

	// Get permissions and uid/gid (omit if error occured)
	if info, err := os.Stat(path); err == nil {
		ret.Permissions = info.Mode().Perm().String()
		if statt, ok := info.Sys().(*syscall.Stat_t); ok {
			if u, err := user.LookupUid(int(statt.Uid)); err == nil {
				ret.User = u.Name
			}
			if g, err := user.LookupGid(int(statt.Gid)); err == nil {
				ret.Group = g.Name
			}
		}
	}

	// Get selinux label (omit if error occured)
	if label, err := selinux.FileLabel(path); err == nil {
		ret.SELinuxOptions = label
	}

	return ret
}
