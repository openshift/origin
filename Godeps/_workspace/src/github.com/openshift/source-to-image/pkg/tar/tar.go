package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/errors"
)

// defaultTimeout is the amount of time that the untar will wait for a tar
// stream to extract a single file. A timeout is needed to guard against broken
// connections in which it would wait for a long time to untar and nothing would happen
const defaultTimeout = 5 * time.Second

// defaultExclusionPattern is the pattern of files that will not be included in a tar
// file when creating one. By default it is any file inside a .git metadata directory
var defaultExclusionPattern = regexp.MustCompile("((^\\.git\\/)|(\\/.git\\/)|(\\/.git$))")

// Tar can create and extract tar files used in an STI build
type Tar interface {
	// SetExclusionPattern sets the exclusion pattern for tar
	// creation
	SetExclusionPattern(*regexp.Regexp)

	// CreateTarFile creates a tar file in the base directory
	// using the contents of dir directory
	// The name of the new tar file is returned if successful
	CreateTarFile(base, dir string) (string, error)

	// CreateTarStreamWithLogging creates a tar from the given directory
	// and streams it to the given writer.
	// An error is returned if an error occurs during streaming.
	// Archived file names are written to the logger if provided
	CreateTarStreamWithLogging(dir string, includeDirInPath bool, writer io.Writer, logger io.Writer) error

	// CreateTarStream creates a tar from the given directory
	// and streams it to the given writer.
	// An error is returned if an error occurs during streaming.
	CreateTarStream(dir string, includeDirInPath bool, writer io.Writer) error

	// ExtractTarStream extracts files from a given tar stream.
	// Times out if reading from the stream for any given file
	// exceeds the value of timeout
	ExtractTarStream(dir string, reader io.Reader) error

	// ExtractTarStreamWithLogging extracts files from a given tar stream.
	// Times out if reading from the stream for any given file
	// exceeds the value of timeout.
	// Extracted file names are written to the logger if provided.
	ExtractTarStreamWithLogging(dir string, reader io.Reader, logger io.Writer) error
}

// New creates a new Tar
func New() Tar {
	return &stiTar{
		exclude: defaultExclusionPattern,
		timeout: defaultTimeout,
	}
}

// stiTar is an implementation of the Tar interface
type stiTar struct {
	timeout          time.Duration
	exclude          *regexp.Regexp
	includeDirInPath bool
}

// SetExclusionPattern sets the exclusion pattern for tar creation
func (t *stiTar) SetExclusionPattern(p *regexp.Regexp) {
	t.exclude = p
}

// CreateTarFile creates a tar file from the given directory
// while excluding files that match the given exclusion pattern
// It returns the name of the created file
func (t *stiTar) CreateTarFile(base, dir string) (string, error) {
	tarFile, err := ioutil.TempFile(base, "tar")
	defer tarFile.Close()
	if err != nil {
		return "", err
	}
	if err = t.CreateTarStream(dir, false, tarFile); err != nil {
		return "", err
	}
	return tarFile.Name(), nil
}

func (t *stiTar) shouldExclude(path string) bool {
	return t.exclude != nil && t.exclude.MatchString(path)
}

// CreateTarStream calls CreateTarStreamWithLogging with a nil logger
func (t *stiTar) CreateTarStream(dir string, includeDirInPath bool, writer io.Writer) error {
	return t.CreateTarStreamWithLogging(dir, includeDirInPath, writer, nil)
}

// CreateTarStreamWithLogging creates a tar stream on the given writer from
// the given directory while excluding files that match the given
// exclusion pattern.
// TODO: this should encapsulate the goroutine that generates the stream.
func (t *stiTar) CreateTarStreamWithLogging(dir string, includeDirInPath bool, writer io.Writer, logger io.Writer) error {
	dir = filepath.Clean(dir) // remove relative paths and extraneous slashes
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && !t.shouldExclude(path) {
			// if file is a link just writing header info is enough
			if info.Mode()&os.ModeSymlink != 0 {
				if err := t.writeTarHeader(tarWriter, dir, path, info, includeDirInPath, logger); err != nil {
					glog.Errorf("	Error writing header for %s: %v", info.Name(), err)
				}
				return nil
			}

			// regular files are copied into tar, if accessible
			file, err := os.Open(path)
			if err != nil {
				glog.Errorf("Ignoring file %s: %v", path, err)
				return nil
			}
			defer file.Close()
			if err := t.writeTarHeader(tarWriter, dir, path, info, includeDirInPath, logger); err != nil {
				glog.Errorf("Error writing header for %s: %v", info.Name(), err)
				return nil
			}
			if _, err = io.Copy(tarWriter, file); err != nil {
				glog.Errorf("Error copying file %s to tar: %v", path, err)
				return err
			}
		}
		return nil
	})

	if err != nil {
		glog.Errorf("Error writing tar: %v", err)
		return err
	}

	return nil
}

// writeTarHeader writes tar header for given file, returns error if operation fails
func (t *stiTar) writeTarHeader(tarWriter *tar.Writer, dir string, path string, info os.FileInfo, includeDirInPath bool, logger io.Writer) error {
	var (
		link string
		err  error
	)
	if info.Mode()&os.ModeSymlink != 0 {
		link, err = os.Readlink(path)
		if err != nil {
			return err
		}
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	prefix := dir
	if includeDirInPath {
		prefix = filepath.Dir(prefix)
	}
	fileName := path
	if prefix != "." {
		fileName = path[1+len(prefix):]
	}
	header.Name = filepath.ToSlash(fileName)
	logFile(logger, header.Name)
	glog.V(5).Infof("Adding to tar: %s as %s", path, header.Name)
	if err = tarWriter.WriteHeader(header); err != nil {
		return err
	}

	return nil
}

// ExtractTarStream calls ExtractTarStreamWithLogging with a nil logger
func (t *stiTar) ExtractTarStream(dir string, reader io.Reader) error {
	return t.ExtractTarStreamWithLogging(dir, reader, nil)
}

// ExtractTarStreamWithLogging extracts files from a given tar stream.
// Times out if reading from the stream for any given file
// exceeds the value of timeout
func (t *stiTar) ExtractTarStreamWithLogging(dir string, reader io.Reader, logger io.Writer) error {
	tarReader := tar.NewReader(reader)
	errorChannel := make(chan error)
	timeout := t.timeout
	timeoutTimer := time.NewTimer(timeout)
	go func() {
		for {
			header, err := tarReader.Next()
			timeoutTimer.Reset(timeout)
			if err == io.EOF {
				errorChannel <- nil
				break
			}
			if err != nil {
				glog.Errorf("Error reading next tar header: %v", err)
				errorChannel <- err
				break
			}
			if header.FileInfo().IsDir() {
				dirPath := filepath.Join(dir, header.Name)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %q: %v", dirPath, err)
					errorChannel <- err
					break
				}
			} else {
				fileDir := filepath.Dir(header.Name)
				dirPath := filepath.Join(dir, fileDir)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %q: %v", dirPath, err)
					errorChannel <- err
					break
				}
				if header.Mode&tar.TypeSymlink == tar.TypeSymlink {
					if err := extractLink(dir, header, tarReader); err != nil {
						glog.Errorf("Error extracting link %q: %v", header.Name, err)
						errorChannel <- err
						break
					}
					continue
				}
				logFile(logger, header.Name)
				if err := extractFile(dir, header, tarReader); err != nil {
					glog.Errorf("Error extracting file %q: %v", header.Name, err)
					errorChannel <- err
					break
				}
			}
		}
	}()

	for {
		select {
		case err := <-errorChannel:
			if err != nil {
				glog.Errorf("Error extracting tar stream")
			} else {
				glog.V(2).Infof("Done extracting tar stream")
			}
			return err
		case <-timeoutTimer.C:
			return errors.NewTarTimeoutError()
		}
	}
}

func extractLink(dir string, header *tar.Header, tarReader io.Reader) error {
	dest := filepath.Join(dir, header.Name)
	source := header.Linkname

	glog.V(3).Infof("Creating symbolic link from %q to %q", dest, source)

	// TODO: set mtime for symlink (unfortunately we can't use os.Chtimes() and probably should use syscall)
	return os.Symlink(source, dest)
}

func extractFile(dir string, header *tar.Header, tarReader io.Reader) error {
	path := filepath.Join(dir, header.Name)
	glog.V(3).Infof("Creating %s", path)

	file, err := os.Create(path)
	// The file times need to be modified after it's been closed thus this function
	// is deferred after the file close (LIFO order for defer)
	defer os.Chtimes(path, time.Now(), header.FileInfo().ModTime())
	defer file.Close()
	if err != nil {
		return err
	}
	glog.V(3).Infof("Extracting/writing %s", path)
	written, err := io.Copy(file, tarReader)
	if err != nil {
		return err
	}
	if written != header.Size {
		return fmt.Errorf("Wrote %d bytes, expected to write %d", written, header.Size)
	}
	if runtime.GOOS != "windows" { // Skip chmod if on windows OS
		return file.Chmod(header.FileInfo().Mode())
	}
	return nil
}

func logFile(logger io.Writer, name string) {
	if logger == nil {
		return
	}
	fmt.Fprintf(logger, "%s\n", name)
}
