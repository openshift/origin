package tar

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/openshift/source-to-image/pkg/errors"
)

type fileDesc struct {
	name         string
	modifiedDate time.Time
	mode         os.FileMode
	content      string
	shouldSkip   bool
	target       string
}

type linkDesc struct {
	linkName string
	fileName string
}

func createTestFiles(baseDir string, files []fileDesc) error {
	for _, fd := range files {
		fileName := filepath.Join(baseDir, fd.name)
		if err := os.MkdirAll(filepath.Dir(fileName), 0700); err != nil {
			return err
		}
		file, err := os.Create(fileName)
		if err != nil {
			return err
		}
		file.WriteString(fd.content)
		file.Chmod(fd.mode)
		file.Close()
		os.Chtimes(fileName, fd.modifiedDate, fd.modifiedDate)
	}
	return nil
}

func createTestLinks(baseDir string, links []linkDesc) error {
	for _, ld := range links {
		linkName := filepath.Join(baseDir, ld.linkName)
		if err := os.MkdirAll(filepath.Dir(linkName), 0700); err != nil {
			return err
		}
		if err := os.Symlink(ld.fileName, linkName); err != nil {
			return err
		}
	}
	return nil
}

func verifyTarFile(t *testing.T, filename string, files []fileDesc, links []linkDesc) {
	filesToVerify := make(map[string]fileDesc)
	for _, fd := range files {
		if !fd.shouldSkip {
			filesToVerify[fd.name] = fd
		}
	}
	linksToVerify := make(map[string]linkDesc)
	for _, ld := range links {
		linksToVerify[ld.linkName] = ld
	}

	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		t.Fatalf("Cannot open tar file to verify: %s, %v\n", filename, err)
	}
	tr := tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if hdr == nil {
			break
		}
		if err != nil {
			t.Fatalf("Error reading tar %s: %v\n", filename, err)
		}
		finfo := hdr.FileInfo()
		if fd, ok := filesToVerify[hdr.Name]; ok {
			delete(filesToVerify, hdr.Name)
			if finfo.Mode().Perm() != fd.mode {
				t.Errorf("File %s from tar %s does not match expected mode. Expected: %v, actual: %v\n",
					hdr.Name, filename, fd.mode, finfo.Mode().Perm())
			}
			if !fd.modifiedDate.IsZero() && finfo.ModTime().UTC() != fd.modifiedDate {
				t.Errorf("File %s from tar %s does not match expected modified date. Expected: %v, actual: %v\n",
					hdr.Name, filename, fd.modifiedDate, finfo.ModTime().UTC())
			}
			fileBytes, err := ioutil.ReadAll(tr)
			if err != nil {
				t.Fatalf("Error reading tar %s: %v\n", filename, err)
			}
			fileContent := string(fileBytes)
			if fileContent != fd.content {
				t.Errorf("Content for file %s in tar %s doesn't match expected value. Expected: %s, Actual: %s",
					finfo.Name(), filename, fd.content, fileContent)
			}
		} else if ld, ok := linksToVerify[hdr.Name]; ok {
			delete(linksToVerify, hdr.Name)
			if finfo.Mode()&os.ModeSymlink == 0 {
				t.Errorf("Incorrect link %s", finfo.Name())
			}
			if hdr.Linkname != ld.fileName {
				t.Errorf("Incorrect link location. Expected: %s, Actual %s", ld.fileName, hdr.Linkname)
			}
		} else {
			t.Errorf("Cannot find file %s from tar in files to verify.\n", hdr.Name)
		}
	}

	if len(filesToVerify) > 0 || len(linksToVerify) > 0 {
		t.Errorf("Did not find all expected files in tar: %v, %v", filesToVerify, linksToVerify)
	}
}

func TestCreateTarStreamIncludeParentDir(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "testtar")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false, ""},
		{"dir01/.git/hello.txt", modificationDate, 0600, "Ignore file content", true, ""},
	}
	if err := createTestFiles(tempDir, testFiles); err != nil {
		t.Fatalf("Cannot create test files: %v", err)
	}
	th := New()
	tarFile, err := ioutil.TempFile("", "testtarout")
	if err != nil {
		t.Fatalf("Unable to create temporary file %v", err)
	}
	defer os.Remove(tarFile.Name())
	err = th.CreateTarStream(tempDir, true, tarFile)
	if err != nil {
		t.Fatalf("Unable to create tar file %v", err)
	}
	tarFile.Close()
	for i := range testFiles {
		testFiles[i].name = filepath.Join(filepath.Base(tempDir), testFiles[i].name)
	}
	verifyTarFile(t, tarFile.Name(), testFiles, []linkDesc{})

}

func TestCreateTar(t *testing.T) {
	th := New()
	tempDir, err := ioutil.TempDir("", "testtar")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false, ""},
		{"dir01/.git/hello.txt", modificationDate, 0600, "Ignore file content", true, ""},
	}
	if err := createTestFiles(tempDir, testFiles); err != nil {
		t.Fatalf("Cannot create test files: %v", err)
	}
	testLinks := []linkDesc{
		{"link/okfilelink", "../dir01/dir02/test1.txt"},
		{"link/errfilelink", "../dir01/missing.target"},
		{"link/okdirlink", "../dir01/dir02"},
		{"link/errdirlink", "../dir01/.git"},
	}
	if err := createTestLinks(tempDir, testLinks); err != nil {
		t.Fatalf("Cannot create link files: %v", err)
	}

	tarFile, err := th.CreateTarFile("", tempDir)
	defer os.Remove(tarFile)
	if err != nil {
		t.Fatalf("Unable to create new tar upload file: %v", err)
	}
	verifyTarFile(t, tarFile, testFiles, testLinks)
}

func createTestTar(files []fileDesc, writer io.Writer) error {
	tw := tar.NewWriter(writer)
	defer tw.Close()
	for _, fd := range files {
		if isSymLink(fd.mode) {
			if err := addSymLink(tw, &fd); err != nil {
				msg := "unable to add symbolic link %q (points to %q) to archive: %v"
				return fmt.Errorf(msg, fd.name, fd.target, err)
			}
			continue
		}
		if err := addRegularFile(tw, &fd); err != nil {
			return fmt.Errorf("unable to add file %q to archive: %v", fd.name, err)
		}
	}
	return nil
}

func addRegularFile(tw *tar.Writer, fd *fileDesc) error {
	contentBytes := []byte(fd.content)
	hdr := &tar.Header{
		Name:       fd.name,
		Mode:       int64(fd.mode),
		Size:       int64(len(contentBytes)),
		Typeflag:   tar.TypeReg,
		AccessTime: time.Now(),
		ModTime:    fd.modifiedDate,
		ChangeTime: fd.modifiedDate,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(contentBytes)
	return err
}

func addSymLink(tw *tar.Writer, fd *fileDesc) error {
	if len(fd.target) == 0 {
		return fmt.Errorf("link %q must point to somewhere, but target wasn't defined", fd.name)
	}

	hdr := &tar.Header{
		Name:     fd.name,
		Linkname: fd.target,
		Mode:     int64(fd.mode & os.ModePerm),
		Typeflag: tar.TypeSymlink,
		ModTime:  fd.modifiedDate,
	}

	return tw.WriteHeader(hdr)
}

func isSymLink(mode os.FileMode) bool {
	return mode&os.ModeSymlink == os.ModeSymlink
}

func verifyDirectory(t *testing.T, dir string, files []fileDesc) {
	filesToVerify := make(map[string]fileDesc)
	for _, fd := range files {
		filesToVerify[fd.name] = fd
	}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			relpath := path[len(dir)+1:]
			if fd, ok := filesToVerify[relpath]; ok {
				if info.Mode() != fd.mode {
					t.Errorf("File mode is not equal for %q. Expected: %v, Actual: %v\n",
						relpath, fd.mode, info.Mode())
				}
				// TODO: check modification time for symlinks when extractLink() will support it
				if info.ModTime().UTC() != fd.modifiedDate && !isSymLink(fd.mode) {
					t.Errorf("File modified date is not equal for %q. Expected: %v, Actual: %v\n",
						relpath, fd.modifiedDate, info.ModTime())
				}
				contentBytes, err := ioutil.ReadFile(path)
				if err != nil {
					t.Errorf("Error reading file %q: %v", path, err)
					return err
				}
				content := string(contentBytes)
				if content != fd.content {
					t.Errorf("File content is not equal for %q. Expected: %s, Actual: %s\n",
						relpath, fd.content, content)
				}
				if isSymLink(fd.mode) {
					target, err := os.Readlink(path)
					if err != nil {
						t.Errorf("Error reading symlink %q: %v", path, err)
						return err
					}
					if target != fd.target {
						msg := "Symbolic link %q points to wrong path. Expected: %s, Actual: %s\n"
						t.Errorf(msg, fd.name, fd.target, target)
					}
				}
			} else {
				t.Errorf("Unexpected file found: %q", relpath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Error walking directory %q: %v", dir, err)
	}
}

func TestExtractTarStream(t *testing.T) {
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false, ""},
		{"dir01/symlink", modificationDate, os.ModeSymlink | 0777, "Test3 file content", false, "../dir01/dir03/test3.txt"},
	}
	reader, writer := io.Pipe()
	destDir, err := ioutil.TempDir("", "testExtract")
	if err != nil {
		t.Fatalf("Cannot create temp directory: %v\n", err)
	}
	defer os.RemoveAll(destDir)
	wg := sync.WaitGroup{}
	wg.Add(2)
	th := New()

	go func() {
		defer wg.Done()
		if err := createTestTar(testFiles, writer); err != nil {
			t.Fatal(err)
		}
		writer.Close()
	}()
	go func() {
		defer wg.Done()
		th.ExtractTarStream(destDir, reader)
	}()
	wg.Wait()
	verifyDirectory(t, destDir, testFiles)
}

func TestExtractTarStreamTimeout(t *testing.T) {
	reader, writer := io.Pipe()
	destDir, err := ioutil.TempDir("", "testExtract")
	if err != nil {
		t.Fatalf("Cannot create temp directory: %v\n", err)
	}
	defer os.RemoveAll(destDir)
	wg := sync.WaitGroup{}
	wg.Add(2)
	th := New()
	th.(*stiTar).timeout = 10 * time.Millisecond
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		writer.Close()
	}()
	extractError := make(chan error, 1)
	go func() {
		defer wg.Done()
		extractError <- th.ExtractTarStream(destDir, reader)
	}()
	wg.Wait()
	err = <-extractError
	if e, ok := err.(errors.Error); err == nil || (ok && e.ErrorCode != errors.TarTimeoutError) {
		t.Errorf("Did not get the expected timeout error. err = %v\n", err)
	}
}
