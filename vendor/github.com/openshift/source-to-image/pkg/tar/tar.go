package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/source-to-image/pkg/util/fs"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"

	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/util"
)

var glog = utilglog.StderrLog

// defaultTimeout is the amount of time that the untar will wait for a tar
// stream to extract a single file. A timeout is needed to guard against broken
// connections in which it would wait for a long time to untar and nothing would happen
const defaultTimeout = 30 * time.Second

// DefaultExclusionPattern is the pattern of files that will not be included in a tar
// file when creating one. By default it is any file inside a .git metadata directory
var DefaultExclusionPattern = regexp.MustCompile(`(^|/)\.git(/|$)`)

// Tar can create and extract tar files used in an STI build
type Tar interface {
	// SetExclusionPattern sets the exclusion pattern for tar
	// creation
	SetExclusionPattern(*regexp.Regexp)

	// CreateTarFile creates a tar file in the base directory
	// using the contents of dir directory
	// The name of the new tar file is returned if successful
	CreateTarFile(base, dir string) (string, error)

	// CreateTarStreamToTarWriter creates a tar from the given directory
	// and streams it to the given writer.
	// An error is returned if an error occurs during streaming.
	// Archived file names are written to the logger if provided
	CreateTarStreamToTarWriter(dir string, includeDirInPath bool, writer Writer, logger io.Writer) error

	// CreateTarStream creates a tar from the given directory
	// and streams it to the given writer.
	// An error is returned if an error occurs during streaming.
	CreateTarStream(dir string, includeDirInPath bool, writer io.Writer) error

	// CreateTarStreamReader returns an io.ReadCloser from which a tar stream can be
	// read.  The tar stream is created using CreateTarStream.
	CreateTarStreamReader(dir string, includeDirInPath bool) io.ReadCloser

	// ExtractTarStream extracts files from a given tar stream.
	// Times out if reading from the stream for any given file
	// exceeds the value of timeout.
	ExtractTarStream(dir string, reader io.Reader) error

	// ExtractTarStreamWithLogging extracts files from a given tar stream.
	// Times out if reading from the stream for any given file
	// exceeds the value of timeout.
	// Extracted file names are written to the logger if provided.
	ExtractTarStreamWithLogging(dir string, reader io.Reader, logger io.Writer) error

	// ExtractTarStreamFromTarReader extracts files from a given tar stream.
	// Times out if reading from the stream for any given file
	// exceeds the value of timeout.
	// Extracted file names are written to the logger if provided.
	ExtractTarStreamFromTarReader(dir string, tarReader Reader, logger io.Writer) error
}

// Reader is an interface which tar.Reader implements.
type Reader interface {
	io.Reader
	Next() (*tar.Header, error)
}

// Writer is an interface which tar.Writer implements.
type Writer interface {
	io.WriteCloser
	Flush() error
	WriteHeader(hdr *tar.Header) error
}

// ChmodAdapter changes the mode of files and directories inline as a tarfile is
// being written
type ChmodAdapter struct {
	Writer
	NewFileMode     int64
	NewExecFileMode int64
	NewDirMode      int64
}

// WriteHeader changes the mode of files and directories inline as a tarfile is
// being written
func (a ChmodAdapter) WriteHeader(hdr *tar.Header) error {
	if hdr.FileInfo().Mode()&os.ModeSymlink == 0 {
		newMode := hdr.Mode &^ 0777
		if hdr.FileInfo().IsDir() {
			newMode |= a.NewDirMode
		} else if hdr.FileInfo().Mode()&0010 != 0 { // S_IXUSR
			newMode |= a.NewExecFileMode
		} else {
			newMode |= a.NewFileMode
		}
		hdr.Mode = newMode
	}
	return a.Writer.WriteHeader(hdr)
}

// RenameAdapter renames files and directories inline as a tarfile is being
// written
type RenameAdapter struct {
	Writer
	Old string
	New string
}

// WriteHeader renames files and directories inline as a tarfile is being
// written
func (a RenameAdapter) WriteHeader(hdr *tar.Header) error {
	if hdr.Name == a.Old {
		hdr.Name = a.New
	} else if strings.HasPrefix(hdr.Name, a.Old+"/") {
		hdr.Name = a.New + hdr.Name[len(a.Old):]
	}

	return a.Writer.WriteHeader(hdr)
}

// New creates a new Tar
func New(fs fs.FileSystem) Tar {
	return &stiTar{
		FileSystem: fs,
		exclude:    DefaultExclusionPattern,
		timeout:    defaultTimeout,
	}
}

// stiTar is an implementation of the Tar interface
type stiTar struct {
	fs.FileSystem
	timeout          time.Duration
	exclude          *regexp.Regexp
	includeDirInPath bool
}

// SetExclusionPattern sets the exclusion pattern for tar creation.  The
// exclusion pattern always uses UNIX-style (/) path separators, even on
// Windows.
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
	return t.exclude != nil && t.exclude.String() != "" && t.exclude.MatchString(filepath.ToSlash(path))
}

// CreateTarStream calls CreateTarStreamToTarWriter with a nil logger
func (t *stiTar) CreateTarStream(dir string, includeDirInPath bool, writer io.Writer) error {
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()

	return t.CreateTarStreamToTarWriter(dir, includeDirInPath, tarWriter, nil)
}

// CreateTarStreamReader returns an io.ReadCloser from which a tar stream can be
// read.  The tar stream is created using CreateTarStream.
func (t *stiTar) CreateTarStreamReader(dir string, includeDirInPath bool) io.ReadCloser {
	r, w := io.Pipe()
	go func() {
		w.CloseWithError(t.CreateTarStream(dir, includeDirInPath, w))
	}()
	return r
}

// CreateTarStreamToTarWriter creates a tar stream on the given writer from
// the given directory while excluding files that match the given
// exclusion pattern.
func (t *stiTar) CreateTarStreamToTarWriter(dir string, includeDirInPath bool, tarWriter Writer, logger io.Writer) error {
	dir = filepath.Clean(dir) // remove relative paths and extraneous slashes
	glog.V(5).Infof("Adding %q to tar ...", dir)
	err := t.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// on Windows, directory symlinks report as a directory and as a symlink.
		// They should be treated as symlinks.
		if !t.shouldExclude(path) {
			// if file is a link just writing header info is enough
			if info.Mode()&os.ModeSymlink != 0 {
				if dir == path {
					return nil
				}
				if err = t.writeTarHeader(tarWriter, dir, path, info, includeDirInPath, logger); err != nil {
					glog.Errorf("Error writing header for %q: %v", info.Name(), err)
				}
				// on Windows, filepath.Walk recurses into directory symlinks when it
				// shouldn't.  https://github.com/golang/go/issues/17540
				if err == nil && info.Mode()&os.ModeDir != 0 {
					return filepath.SkipDir
				}
				return err
			}
			if info.IsDir() {
				if dir == path {
					return nil
				}
				if err = t.writeTarHeader(tarWriter, dir, path, info, includeDirInPath, logger); err != nil {
					glog.Errorf("Error writing header for %q: %v", info.Name(), err)
				}
				return err
			}

			// regular files are copied into tar, if accessible
			file, err := os.Open(path)
			if err != nil {
				glog.Errorf("Ignoring file %s: %v", path, err)
				return nil
			}
			defer file.Close()
			if err = t.writeTarHeader(tarWriter, dir, path, info, includeDirInPath, logger); err != nil {
				glog.Errorf("Error writing header for %q: %v", info.Name(), err)
				return err
			}
			if _, err = io.Copy(tarWriter, file); err != nil {
				glog.Errorf("Error copying file %q to tar: %v", path, err)
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
func (t *stiTar) writeTarHeader(tarWriter Writer, dir string, path string, info os.FileInfo, includeDirInPath bool, logger io.Writer) error {
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
	// on Windows, tar.FileInfoHeader incorrectly interprets directory symlinks
	// as directories.  https://github.com/golang/go/issues/17541
	if info.Mode()&os.ModeSymlink != 0 && info.Mode()&os.ModeDir != 0 {
		header.Typeflag = tar.TypeSymlink
		header.Mode &^= 040000 // c_ISDIR
		header.Mode |= 0120000 // c_ISLNK
		header.Linkname = link
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
	header.Linkname = filepath.ToSlash(header.Linkname)
	logFile(logger, header.Name)
	glog.V(5).Infof("Adding to tar: %s as %s", path, header.Name)
	if err = tarWriter.WriteHeader(header); err != nil {
		return err
	}

	return nil
}

// ExtractTarStream calls ExtractTarStreamFromTarReader with a default reader and nil logger
func (t *stiTar) ExtractTarStream(dir string, reader io.Reader) error {
	tarReader := tar.NewReader(reader)
	return t.ExtractTarStreamFromTarReader(dir, tarReader, nil)
}

// ExtractTarStreamWithLogging calls ExtractTarStreamFromTarReader with a default reader
func (t *stiTar) ExtractTarStreamWithLogging(dir string, reader io.Reader, logger io.Writer) error {
	tarReader := tar.NewReader(reader)
	return t.ExtractTarStreamFromTarReader(dir, tarReader, logger)
}

// ExtractTarStreamFromTarReader extracts files from a given tar stream.
// Times out if reading from the stream for any given file
// exceeds the value of timeout
func (t *stiTar) ExtractTarStreamFromTarReader(dir string, tarReader Reader, logger io.Writer) error {
	err := util.TimeoutAfter(t.timeout, "", func(timeoutTimer *time.Timer) error {
		for {
			header, err := tarReader.Next()
			if !timeoutTimer.Stop() {
				return &util.TimeoutError{}
			}
			timeoutTimer.Reset(t.timeout)
			if err == io.EOF {
				return nil
			}
			if err != nil {
				glog.Errorf("Error reading next tar header: %v", err)
				return err
			}
			if header.FileInfo().IsDir() {
				dirPath := filepath.Join(dir, header.Name)
				glog.V(3).Infof("Creating directory %s", dirPath)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %q: %v", dirPath, err)
					return err
				}
			} else {
				fileDir := filepath.Dir(header.Name)
				dirPath := filepath.Join(dir, fileDir)
				glog.V(3).Infof("Creating directory %s", dirPath)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %q: %v", dirPath, err)
					return err
				}
				if header.Typeflag == tar.TypeSymlink {
					if err := t.extractLink(dir, header, tarReader); err != nil {
						glog.Errorf("Error extracting link %q: %v", header.Name, err)
						return err
					}
					continue
				}
				logFile(logger, header.Name)
				if err := t.extractFile(dir, header, tarReader); err != nil {
					glog.Errorf("Error extracting file %q: %v", header.Name, err)
					return err
				}
			}
		}
	})

	if err != nil {
		glog.Error("Error extracting tar stream")
	} else {
		glog.V(2).Info("Done extracting tar stream")
	}

	if util.IsTimeoutError(err) {
		err = s2ierr.NewTarTimeoutError()
	}

	return err
}

func (t *stiTar) extractLink(dir string, header *tar.Header, tarReader io.Reader) error {
	dest := filepath.Join(dir, header.Name)
	source := header.Linkname

	glog.V(3).Infof("Creating symbolic link from %q to %q", dest, source)

	// TODO: set mtime for symlink (unfortunately we can't use os.Chtimes() and probably should use syscall)
	return os.Symlink(source, dest)
}

func (t *stiTar) extractFile(dir string, header *tar.Header, tarReader io.Reader) error {
	path := filepath.Join(dir, header.Name)
	glog.V(3).Infof("Creating %s", path)

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	// The file times need to be modified after it's been closed thus this function
	// is deferred after the file close (LIFO order for defer)
	defer os.Chtimes(path, time.Now(), header.FileInfo().ModTime())
	defer file.Close()
	glog.V(3).Infof("Extracting/writing %s", path)
	written, err := io.Copy(file, tarReader)
	if err != nil {
		return err
	}
	if written != header.Size {
		return fmt.Errorf("Wrote %d bytes, expected to write %d", written, header.Size)
	}
	return t.Chmod(path, header.FileInfo().Mode())
}

func logFile(logger io.Writer, name string) {
	if logger == nil {
		return
	}
	fmt.Fprintf(logger, "%s\n", name)
}
