package dockerhelper

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	stitar "github.com/openshift/source-to-image/pkg/tar"
)

// removeLeadingDirectoryAdapter wraps a tar.Reader and strips the first leading
// directory name of all of the files in the archive.  An error is returned in
// the case that files with differing first leading directory names are
// encountered.
type removeLeadingDirectoryAdapter struct {
	stitar.Reader
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

func newContainerDownloader(client *docker.Client, container, path string) io.ReadCloser {
	r, w := io.Pipe()

	go func() {
		opts := docker.DownloadFromContainerOptions{
			Path:         path,
			OutputStream: w,
		}
		w.CloseWithError(client.DownloadFromContainer(container, opts))
	}()

	return r
}

func newContainerUploader(client *docker.Client, container, path string) (io.WriteCloser, <-chan error) {
	r, w := io.Pipe()
	errch := make(chan error, 1)

	go func() {
		opts := docker.UploadToContainerOptions{
			Path:        path,
			InputStream: r,
		}
		errch <- client.UploadToContainer(container, opts)
	}()

	return w, errch
}

type readCloser struct {
	io.Reader
	io.Closer
}

// StreamFileFromContainer returns an io.ReadCloser from which the contents of a
// file in a remote container can be read.
func StreamFileFromContainer(client *docker.Client, container, src string) (io.ReadCloser, error) {
	downloader := newContainerDownloader(client, container, src)
	tarReader := tar.NewReader(downloader)

	header, err := tarReader.Next()
	if err != nil {
		return nil, err
	}
	if header.Name != filepath.Base(src) || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) {
		return nil, fmt.Errorf("unexpected tar file content %s, type %c", header.Name, header.Typeflag)
	}
	return readCloser{Reader: tarReader, Closer: downloader}, nil
}

// DownloadDirFromContainer downloads an entire directory of files from a remote
// container.
func DownloadDirFromContainer(client *docker.Client, container, src, dst string) error {
	downloader := newContainerDownloader(client, container, src)
	defer downloader.Close()
	tarReader := &removeLeadingDirectoryAdapter{Reader: tar.NewReader(downloader)}

	t := stitar.New()
	return t.ExtractTarStreamFromTarReader(dst, tarReader, nil)
}

// UploadFileToContainer uploads a file to a remote container.
func UploadFileToContainer(client *docker.Client, container, src, dest string) error {
	uploader, errch := newContainerUploader(client, container, filepath.Dir(dest))

	nullWalkFunc := func(path string, info os.FileInfo, err error) error { return err }

	t := stitar.New()
	err := t.StreamFileAsTarWithCallback(src, filepath.Base(dest), uploader, nullWalkFunc, false)
	uploader.Close()
	if err != nil {
		return err
	}

	return <-errch
}
