package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
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
	// CreateTarFile creates a tar file in the base directory
	// using the contents of dir directory
	// The name of the new tar file is returned if successful
	CreateTarFile(base, dir string) (string, error)

	// ExtractTarStream extracts files from a given tar stream.
	// Times out if reading from the stream for any given file
	// exceeds the value of timeout
	ExtractTarStream(dir string, reader io.Reader) error
}

// NewTar creates a new Tar
func NewTar(verbose bool) Tar {
	return &stiTar{
		verbose: verbose,
		exclude: defaultExclusionPattern,
		timeout: defaultTimeout,
	}
}

// stiTar is an implementation of the Tar interface
type stiTar struct {
	timeout time.Duration
	verbose bool
	exclude *regexp.Regexp
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
	if err = t.CreateTarStream(dir, tarFile); err != nil {
		return "", err
	}
	return tarFile.Name(), nil
}

func (t *stiTar) shouldExclude(path string) bool {
	return t.exclude != nil && t.exclude.MatchString(path)
}

// CreateTarStream creates a tar stream on the given writer from
// the given directory while excluding files that match the given
// exclusion pattern.
func (t *stiTar) CreateTarStream(dir string, writer io.Writer) error {
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && !t.shouldExclude(path) {
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			header.Name = path[1+len(dir):]

			if t.verbose {
				log.Printf("Adding to tar: %s as %s\n", path, header.Name)
			}

			if err = tarWriter.WriteHeader(header); err != nil {
				return err
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err = io.Copy(tarWriter, file); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Error writing tar: %v\n", err)
		return err
	}

	return nil
}

// ExtractTarStream extracts files from a given tar stream.
// Times out if reading from the stream for any given file
// exceeds the value of timeout
func (t *stiTar) ExtractTarStream(dir string, reader io.Reader) error {
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
				log.Printf("Error reading next tar header: %s", err.Error())
				errorChannel <- err
				break
			}
			if header.FileInfo().IsDir() {
				dirPath := filepath.Join(dir, header.Name)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					log.Printf("Error creating dir %s: %s", dirPath, err.Error())
					errorChannel <- err
					break
				}
			} else {
				fileDir := filepath.Dir(header.Name)
				dirPath := filepath.Join(dir, fileDir)
				if err = os.MkdirAll(dirPath, 0700); err != nil {
					log.Printf("Error creating dir %s: %s", dirPath, err.Error())
					errorChannel <- err
					break
				}
				path := filepath.Join(dir, header.Name)
				if t.verbose {
					log.Printf("Creating %s", path)
				}
				success := false
				// The file times need to be modified after it's been closed
				// thus this function is deferred before the file close
				defer func() {
					if success && os.Chtimes(path, time.Now(),
						header.FileInfo().ModTime()) != nil {
						log.Printf("Error setting file dates: %v", err)
						errorChannel <- err
					}
				}()
				file, err := os.Create(path)
				defer file.Close()
				if err != nil {
					log.Printf("Error creating file %s: %s", path, err.Error())
					errorChannel <- err
					break
				}
				if t.verbose {
					log.Printf("Extracting/writing %s", path)
				}
				written, err := io.Copy(file, tarReader)
				if err != nil {
					log.Printf("Error writing file: %s", err.Error())
					errorChannel <- err
					break
				}
				if written != header.Size {
					message := fmt.Sprintf("Wrote %d bytes, expected to write %d\n",
						written, header.Size)
					log.Println(message)
					errorChannel <- fmt.Errorf(message)
					break
				}
				if err = file.Chmod(header.FileInfo().Mode()); err != nil {
					log.Printf("Error setting file mode: %v", err)
					errorChannel <- err
					break
				}
				if t.verbose {
					log.Printf("Done with %s", path)
				}
				success = true
			}
		}
	}()

	for {
		select {
		case err := <-errorChannel:
			if err != nil {
				log.Printf("Error extracting tar stream")
			}
			if t.verbose {
				log.Printf("Done extracting tar stream")
			}
			return err
		case <-timeoutTimer.C:
			return fmt.Errorf("Timeout waiting for tar stream")
		}
	}
}
