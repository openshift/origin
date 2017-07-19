package dockerhelper

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/engine-api/types"
	s2itar "github.com/openshift/source-to-image/pkg/tar"
	s2ifs "github.com/openshift/source-to-image/pkg/util/fs"
)

// removeLeadingDirectoryAdapter wraps a tar.Reader and strips the first leading
// directory name of all of the files in the archive.  An error is returned in
// the case that files with differing first leading directory names are
// encountered.
type removeLeadingDirectoryAdapter struct {
	s2itar.Reader
	leadingDir    string
	setLeadingDir bool
}

func (adapter removeLeadingDirectoryAdapter) Next() (*tar.Header, error) {
	for {
		header, err := adapter.Reader.Next()
		if err != nil {
			return nil, err
		}

		trimmedName := strings.Trim(header.Name, "/")
		paths := strings.SplitN(trimmedName, "/", 2)

		// ensure leading directory is consistent throughout the tar file
		if !adapter.setLeadingDir {
			adapter.setLeadingDir = true
			adapter.leadingDir = paths[0]
		}
		if adapter.leadingDir != paths[0] {
			return nil, fmt.Errorf("inconsistent leading directory at %s, type %c", header.Name, header.Typeflag)
		}

		if len(paths) == 1 {
			if header.Typeflag == tar.TypeDir {
				// this is the leading directory itself: drop it
				_, err = io.Copy(ioutil.Discard, adapter)
				if err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("unexpected non-directory %s, type %c", header.Name, header.Typeflag)
		}

		header.Name = paths[1]
		return header, err
	}
}

func newContainerUploader(client Interface, container, path string) (io.WriteCloser, <-chan error) {
	r, w := io.Pipe()
	errch := make(chan error, 1)

	go func() {
		errch <- client.CopyToContainer(container, path, r, types.CopyToContainerOptions{})
	}()

	return w, errch
}

type readCloser struct {
	io.Reader
	io.Closer
}

// StreamFileFromContainer returns an io.ReadCloser from which the contents of a
// file in a remote container can be read.
func StreamFileFromContainer(client Interface, container, src string) (io.ReadCloser, error) {
	response, err := client.CopyFromContainer(container, src)
	if err != nil {
		return nil, err
	}
	tarReader := tar.NewReader(response)
	header, err := tarReader.Next()
	if err != nil {
		response.Close()
		return nil, err
	}
	if header.Name != filepath.Base(src) || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) {
		response.Close()
		return nil, fmt.Errorf("unexpected tar file content %s, type %c", header.Name, header.Typeflag)
	}
	return readCloser{Reader: tarReader, Closer: response}, nil
}

// DownloadDirFromContainer downloads an entire directory of files from a remote
// container.
func DownloadDirFromContainer(client Interface, container, src, dst string) error {
	response, err := client.CopyFromContainer(container, src)
	if err != nil {
		return err
	}
	defer response.Close()
	tarReader := &removeLeadingDirectoryAdapter{Reader: tar.NewReader(response)}

	t := s2itar.New(s2ifs.NewFileSystem())
	return t.ExtractTarStreamFromTarReader(dst, tarReader, nil)
}

// UploadFileToContainer uploads a file to a remote container.
func UploadFileToContainer(client Interface, container, src, dest string) error {
	uploader, errch := newContainerUploader(client, container, path.Dir(dest))

	t := s2itar.New(s2ifs.NewFileSystem())
	tarWriter := s2itar.RenameAdapter{Writer: tar.NewWriter(uploader), Old: filepath.Base(src), New: path.Base(dest)}

	err := t.CreateTarStreamToTarWriter(src, true, tarWriter, nil)
	if err == nil {
		err = tarWriter.Close()
	}
	uploader.Close()
	if err != nil {
		return err
	}

	return <-errch
}
