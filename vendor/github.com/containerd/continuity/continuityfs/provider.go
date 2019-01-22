// +build linux darwin freebsd

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package continuityfs

import (
	"io"
	"path/filepath"

	"github.com/containerd/continuity/driver"
	"github.com/opencontainers/go-digest"
)

// FileContentProvider is an object which is used to fetch
// data and inode information about a path or digest.
// TODO(dmcgowan): Update GetContentPath to provide a
// filehandle or ReadWriteCloser.
type FileContentProvider interface {
	Path(string, digest.Digest) (string, error)
	Open(string, digest.Digest) (io.ReadCloser, error)
}

type fsContentProvider struct {
	root   string
	driver driver.Driver
}

// NewFSFileContentProvider creates a new content provider which
// gets content from a directory on an existing filesystem based
// on the resource path.
func NewFSFileContentProvider(root string, driver driver.Driver) FileContentProvider {
	return &fsContentProvider{
		root:   root,
		driver: driver,
	}
}

func (p *fsContentProvider) Path(path string, dgst digest.Digest) (string, error) {
	return filepath.Join(p.root, path), nil
}

func (p *fsContentProvider) Open(path string, dgst digest.Digest) (io.ReadCloser, error) {
	return p.driver.Open(filepath.Join(p.root, path))
}
