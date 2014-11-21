package tar

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fileDesc struct {
	name         string
	modifiedDate time.Time
	mode         os.FileMode
	content      string
	shouldSkip   bool
}

func createTestFiles(baseDir string, files []fileDesc) error {
	for _, fd := range files {
		filename := filepath.Join(baseDir, fd.name)
		err := os.MkdirAll(filepath.Dir(filename), 0700)
		if err != nil {
			return err
		}
		file, err := os.Create(filename)
		file.WriteString(fd.content)
		file.Chmod(fd.mode)
		file.Close()
		os.Chtimes(filename, fd.modifiedDate, fd.modifiedDate)
	}
	return nil
}

func verifyTarFile(t *testing.T, filename string, files []fileDesc) {
	filesToVerify := make(map[string]fileDesc)
	for _, fd := range files {
		if !fd.shouldSkip {
			filesToVerify[fd.name] = fd
		}
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
			if finfo.Mode() != fd.mode {
				t.Errorf("File %s from tar %s does not match expected mode. Expected: %v, actual: %v\n",
					hdr.Name, filename, fd.mode, finfo.Mode())
			}
			if finfo.ModTime().UTC() != fd.modifiedDate {
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

		} else {
			t.Errorf("Cannot find file %s from tar in files to verify.\n", hdr.Name)
		}
	}

	if len(filesToVerify) > 0 {
		t.Errorf("Did not find all expected files in tar: %v", filesToVerify)
	}
}

func TestCreateTar(t *testing.T) {
	th := NewTar()
	tempDir, err := ioutil.TempDir("", "testtar")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false},
		{"dir01/.git/hello.txt", modificationDate, 0600, "Ignore file content", true},
	}
	err = createTestFiles(tempDir, testFiles)
	if err != nil {
		t.Fatalf("Cannot create test files: %v", err)
	}

	tarFile, err := th.CreateTarFile("", tempDir)
	if err != nil {
		t.Fatalf("Unable to create new tar upload file: %v", err)
	}
	verifyTarFile(t, tarFile, testFiles)
}

func createTestTar(files []fileDesc, writer io.Writer) {
	tw := tar.NewWriter(writer)
	for _, fd := range files {
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
		tw.WriteHeader(hdr)
		tw.Write(contentBytes)
	}
	tw.Close()
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
					t.Errorf("File mode is not equal for %s. Expected: %v, Actual: %v\n",
						relpath, fd.mode, info.Mode())
				}
				if info.ModTime().UTC() != fd.modifiedDate {
					t.Errorf("File modified date is not equal for %s. Expected: %v, Actual: %v\n",
						relpath, fd.modifiedDate, info.ModTime())
				}
				contentBytes, err := ioutil.ReadFile(path)
				if err != nil {
					t.Errorf("Error reading file %s: %v", path, err)
					return err
				}
				content := string(contentBytes)
				if content != fd.content {
					t.Errorf("File content is not equal for %s. Expected: %s, Actual: %s\n",
						relpath, fd.content, content)
				}
			} else {
				t.Errorf("Unexpected file found: %s", relpath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Error walking directory %s: %v", dir, err)
	}
}

func TestExtractTarStream(t *testing.T) {
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false},
	}
	reader, writer := io.Pipe()
	destDir, err := ioutil.TempDir("", "testExtract")
	if err != nil {
		t.Fatalf("Cannot create temp directory: %v\n", err)
	}
	defer os.RemoveAll(destDir)
	wg := sync.WaitGroup{}
	wg.Add(2)
	th := NewTar()

	go func() {
		defer wg.Done()
		createTestTar(testFiles, writer)
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
	th := NewTar()
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
	if err == nil || strings.Index(err.Error(), "Timeout") != 0 {
		t.Errorf("Did not get the expected timeout error. err = %v\n", err)
	}
}
