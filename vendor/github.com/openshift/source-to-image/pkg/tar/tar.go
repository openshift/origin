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

	utilglog "github.com/openshift/source-to-image/pkg/util/glog"

	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/util"
)

var glog = utilglog.StderrLog

// defaultTimeout is the amount of time that the untar will wait for a tar
// stream to extract a single file. A timeout is needed to guard against broken
// connections in which it would wait for a long time to untar and nothing would happen
const defaultTimeout = 30 * time.Second

// DefaultExclusionPattern is the pattern of files that will not be included in a tar
// file when creating one. By default it is any file inside a .git metadata directory
var DefaultExclusionPattern = regexp.MustCompile("((^\\.git\\/)|(\\/.git\\/)|(\\/.git$))")

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

	// StreamFileAsTar streams a single file as a TAR archive into specified
	// writer. The second argument is the file name in archive.
	// The file permissions in tar archive will change to 0666.
	StreamFileAsTar(string, string, io.Writer) error

	// StreamFileAsTarWithCallback streams a single file as a TAR archive into specified
	// writer. By specifying walkFn you can modify file's permissions in arbitrary way.
	// If modifyInplace is set to false, file will be copied into a temporary directory before changing its permissions.
	StreamFileAsTarWithCallback(source, name string, writer io.Writer, walkFn filepath.WalkFunc, modifyInplace bool) error

	// StreamDirAsTar streams a directory as a TAR archive into specified writer.
	// The second argument is the name of the folder in the archive.
	// All files in the source folder will have permissions changed to 0666 in the
	// tar archive.
	StreamDirAsTar(string, string, io.Writer) error

	// StreamDirAsTarWithCallback streams a directory as a TAR archive into specified writer.
	// By specifying walkFn you can modify files' permissions in arbitrary way.
	// If modifyInplace is set to false, all the files will be copied into a temporary directory before changing their permissions.
	StreamDirAsTarWithCallback(source string, writer io.Writer, walkFn filepath.WalkFunc, modifyInplace bool) error
}

// Reader is an interface which tar.Reader implements.
type Reader interface {
	io.Reader
	Next() (*tar.Header, error)
}

// New creates a new Tar
func New() Tar {
	return &stiTar{
		exclude: DefaultExclusionPattern,
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

// StreamDirAsTar streams the source directory as a tar archive.
// The permissions of the file is changed to 0666.
func (t *stiTar) StreamDirAsTar(source, dest string, writer io.Writer) error {
	makeFileWorldWritable := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip chmod if on windows OS and for symlinks
		if runtime.GOOS == "windows" || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		mode := os.FileMode(0666)
		if info.IsDir() {
			mode = 0777
		}
		return os.Chmod(path, mode)
	}
	return t.StreamDirAsTarWithCallback(source, writer, makeFileWorldWritable, false)
}

// StreamDirAsTarWithCallback streams the source directory as a tar archive.
func (t *stiTar) StreamDirAsTarWithCallback(source string, writer io.Writer, walkFn filepath.WalkFunc, modifyInplace bool) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("the source %q has to be directory, not a file", source)
	}
	destDir := source
	if !modifyInplace {
		fs := util.NewFileSystem()
		tmpDir, err := ioutil.TempDir("", "s2i-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		if err = fs.Copy(source, tmpDir); err != nil {
			return err
		}
		destDir = tmpDir
	}
	if err := filepath.Walk(destDir, walkFn); err != nil {
		return err
	}
	return t.CreateTarStream(destDir, false, writer)
}

// StreamFileAsTar streams the source file as a tar archive.
// The permissions of all files in archive is changed to 0666.
func (t *stiTar) StreamFileAsTar(source, name string, writer io.Writer) error {
	makeFileWorldWritable := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip chmod if on windows OS and for symlinks
		if runtime.GOOS == "windows" || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		return os.Chmod(path, 0666)
	}
	return t.StreamFileAsTarWithCallback(source, name, writer, makeFileWorldWritable, false)
}

// StreamFileAsTarWithCallback streams the source file as a tar archive.
func (t *stiTar) StreamFileAsTarWithCallback(source, name string, writer io.Writer, walkFn filepath.WalkFunc, modifyInplace bool) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("the source %q has to be regular file, not directory", source)
	}
	fs := util.NewFileSystem()
	tmpDir, err := ioutil.TempDir("", "s2i-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	dst := filepath.Join(tmpDir, name)
	if err := fs.Copy(source, dst); err != nil {
		return err
	}
	fileInfo, fileErr := os.Stat(dst)
	if err := walkFn(dst, fileInfo, fileErr); err != nil {
		return err
	}
	return t.CreateTarStream(tmpDir, false, writer)
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
	return t.exclude != nil && t.exclude.String() != "" && t.exclude.MatchString(path)
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
	glog.V(5).Infof("Adding %q to tar ...", dir)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !t.shouldExclude(path) {
			// if file is a link just writing header info is enough
			if info.Mode()&os.ModeSymlink != 0 {
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
	errorChannel := make(chan error, 1)
	timeoutTimer := time.NewTimer(t.timeout)
	go func() {
		for {
			header, err := tarReader.Next()
			if !timeoutTimer.Stop() {
				break
			}
			timeoutTimer.Reset(t.timeout)
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
				glog.V(3).Infof("Creating directory %s", dirPath)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %q: %v", dirPath, err)
					errorChannel <- err
					break
				}
			} else {
				fileDir := filepath.Dir(header.Name)
				dirPath := filepath.Join(dir, fileDir)
				glog.V(3).Infof("Creating directory %s", dirPath)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					glog.Errorf("Error creating dir %q: %v", dirPath, err)
					errorChannel <- err
					break
				}
				if header.Typeflag == tar.TypeSymlink {
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
				glog.Error("Error extracting tar stream")
			} else {
				glog.V(2).Info("Done extracting tar stream")
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
