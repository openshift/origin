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
	"testing"
	"time"

	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/util/fs"
)

type dirDesc struct {
	name         string
	modifiedDate time.Time
	mode         os.FileMode
}

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

func createTestFiles(baseDir string, dirs []dirDesc, files []fileDesc, links []linkDesc) error {
	for _, dd := range dirs {
		fileName := filepath.Join(baseDir, dd.name)
		err := os.Mkdir(fileName, dd.mode)
		if err != nil {
			return err
		}
		os.Chmod(fileName, dd.mode) // umask
	}
	for _, fd := range files {
		fileName := filepath.Join(baseDir, fd.name)
		err := ioutil.WriteFile(fileName, []byte(fd.content), fd.mode)
		if err != nil {
			return err
		}
		os.Chmod(fileName, fd.mode)
		os.Chtimes(fileName, fd.modifiedDate, fd.modifiedDate)
	}
	for _, ld := range links {
		linkName := filepath.Join(baseDir, ld.linkName)
		if err := os.MkdirAll(filepath.Dir(linkName), 0700); err != nil {
			return err
		}
		if err := os.Symlink(ld.fileName, linkName); err != nil {
			return err
		}
	}
	for _, dd := range dirs {
		fileName := filepath.Join(baseDir, dd.name)
		os.Chtimes(fileName, dd.modifiedDate, dd.modifiedDate)
	}
	return nil
}

func verifyTarFile(t *testing.T, filename string, dirs []dirDesc, files []fileDesc, links []linkDesc) {
	if runtime.GOOS == "windows" {
		for i := range files {
			if files[i].mode&0700 == 0400 {
				files[i].mode = 0444
			} else {
				files[i].mode = 0666
			}
		}
		for i := range dirs {
			if dirs[i].mode&0700 == 0500 {
				dirs[i].mode = 0555
			} else {
				dirs[i].mode = 0777
			}
		}
	}
	dirsToVerify := make(map[string]dirDesc)
	for _, dd := range dirs {
		dirsToVerify[dd.name] = dd
	}
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
		t.Fatalf("Cannot open tar file %q: %v", filename, err)
	}
	tr := tar.NewReader(file)
	for {
		hdr, err := tr.Next()
		if hdr == nil {
			break
		}
		if err != nil {
			t.Fatalf("Error reading tar %q: %v", filename, err)
		}
		finfo := hdr.FileInfo()
		if dd, ok := dirsToVerify[hdr.Name]; ok {
			delete(dirsToVerify, hdr.Name)
			if finfo.Mode()&os.ModeDir == 0 {
				t.Errorf("Incorrect dir %q", finfo.Name())
			}
			if finfo.Mode().Perm() != dd.mode {
				t.Errorf("Dir %q from tar %q does not match expected mode. Expected: %v, actual: %v",
					hdr.Name, filename, dd.mode, finfo.Mode().Perm())
			}
			if !dd.modifiedDate.IsZero() && finfo.ModTime().UTC() != dd.modifiedDate {
				t.Errorf("Dir %q from tar %q does not match expected modified date. Expected: %v, actual: %v",
					hdr.Name, filename, dd.modifiedDate, finfo.ModTime().UTC())
			}
		} else if fd, ok := filesToVerify[hdr.Name]; ok {
			delete(filesToVerify, hdr.Name)
			if finfo.Mode().Perm() != fd.mode {
				t.Errorf("File %q from tar %q does not match expected mode. Expected: %v, actual: %v",
					hdr.Name, filename, fd.mode, finfo.Mode().Perm())
			}
			if !fd.modifiedDate.IsZero() && finfo.ModTime().UTC() != fd.modifiedDate {
				t.Errorf("File %q from tar %q does not match expected modified date. Expected: %v, actual: %v",
					hdr.Name, filename, fd.modifiedDate, finfo.ModTime().UTC())
			}
			fileBytes, err := ioutil.ReadAll(tr)
			if err != nil {
				t.Fatalf("Error reading tar %q: %v", filename, err)
			}
			fileContent := string(fileBytes)
			if fileContent != fd.content {
				t.Errorf("Content for file %q in tar %q doesn't match expected value. Expected: %q, Actual: %q",
					finfo.Name(), filename, fd.content, fileContent)
			}
		} else if ld, ok := linksToVerify[hdr.Name]; ok {
			delete(linksToVerify, hdr.Name)
			if finfo.Mode()&os.ModeSymlink == 0 {
				t.Errorf("Incorrect link %q", finfo.Name())
			}
			if hdr.Linkname != ld.fileName {
				t.Errorf("Incorrect link location. Expected: %q, Actual %q", ld.fileName, hdr.Linkname)
			}
		} else {
			t.Errorf("Cannot find file %q from tar in files to verify.", hdr.Name)
		}
	}

	if len(filesToVerify) > 0 || len(linksToVerify) > 0 {
		t.Errorf("Did not find all expected files in tar: fileToVerify %v, linksToVerify %v", filesToVerify, linksToVerify)
	}
}

func TestCreateTarStreamIncludeParentDir(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "testtar")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testDirs := []dirDesc{
		{"dir01", modificationDate, 0700},
		{"dir01/.git", modificationDate, 0755},
		{"dir01/dir02", modificationDate, 0755},
		{"dir01/dir03", modificationDate, 0775},
	}
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false, ""},
		{"dir01/.git/hello.txt", modificationDate, 0600, "Ignore file content", true, ""},
	}
	if err = createTestFiles(tempDir, testDirs, testFiles, []linkDesc{}); err != nil {
		t.Fatalf("Cannot create test files: %v", err)
	}
	th := New(fs.NewFileSystem())
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
	for i := range testDirs {
		testDirs[i].name = filepath.ToSlash(filepath.Join(filepath.Base(tempDir), testDirs[i].name))
	}
	for i := range testFiles {
		testFiles[i].name = filepath.ToSlash(filepath.Join(filepath.Base(tempDir), testFiles[i].name))
	}
	verifyTarFile(t, tarFile.Name(), testDirs, testFiles, []linkDesc{})
}

func TestCreateTar(t *testing.T) {
	th := New(fs.NewFileSystem())
	tempDir, err := ioutil.TempDir("", "testtar")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testDirs := []dirDesc{
		{"dir01", modificationDate, 0700},
		{"dir01/.git", modificationDate, 0755},
		{"dir01/dir02", modificationDate, 0755},
		{"dir01/dir03", modificationDate, 0775},
		{"link", modificationDate, 0775},
	}
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false, ""},
		{"dir01/.git/hello.txt", modificationDate, 0600, "Ignore file content", true, ""},
	}
	testLinks := []linkDesc{
		{"link/okfilelink", "../dir01/dir02/test1.txt"},
		{"link/errfilelink", "../dir01/missing.target"},
		{"link/okdirlink", "../dir01/dir02"},
		{"link/errdirlink", "../dir01/.git"},
	}
	if err = createTestFiles(tempDir, testDirs, testFiles, testLinks); err != nil {
		t.Fatalf("Cannot create test files: %v", err)
	}

	tarFile, err := th.CreateTarFile("", tempDir)
	defer os.Remove(tarFile)
	if err != nil {
		t.Fatalf("Unable to create new tar upload file: %v", err)
	}
	verifyTarFile(t, tarFile, testDirs, testFiles, testLinks)
}

func TestCreateTarIncludeDotGit(t *testing.T) {
	th := New(fs.NewFileSystem())
	th.SetExclusionPattern(regexp.MustCompile("test3.txt"))
	tempDir, err := ioutil.TempDir("", "testtar")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testDirs := []dirDesc{
		{"dir01", modificationDate, 0700},
		{"dir01/.git", modificationDate, 0755},
		{"dir01/dir02", modificationDate, 0755},
		{"dir01/dir03", modificationDate, 0775},
		{"link", modificationDate, 0775},
	}
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", true, ""},
		{"dir01/.git/hello.txt", modificationDate, 0600, "Allow .git content", false, ""},
	}
	testLinks := []linkDesc{
		{"link/okfilelink", "../dir01/dir02/test1.txt"},
		{"link/errfilelink", "../dir01/missing.target"},
		{"link/okdirlink", "../dir01/dir02"},
		{"link/okdirlink2", "../dir01/.git"},
	}
	if err = createTestFiles(tempDir, testDirs, testFiles, testLinks); err != nil {
		t.Fatalf("Cannot create test files: %v", err)
	}

	tarFile, err := th.CreateTarFile("", tempDir)
	defer os.Remove(tarFile)
	if err != nil {
		t.Fatalf("Unable to create new tar upload file: %v", err)
	}
	verifyTarFile(t, tarFile, testDirs, testFiles, testLinks)
}

func TestCreateTarEmptyRegexp(t *testing.T) {
	th := New(fs.NewFileSystem())
	th.SetExclusionPattern(regexp.MustCompile(""))
	tempDir, err := ioutil.TempDir("", "testtar")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	testDirs := []dirDesc{
		{"dir01", modificationDate, 0700},
		{"dir01/.git", modificationDate, 0755},
		{"dir01/dir02", modificationDate, 0755},
		{"dir01/dir03", modificationDate, 0775},
		{"link", modificationDate, 0775},
	}
	testFiles := []fileDesc{
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false, ""},
		{"dir01/.git/hello.txt", modificationDate, 0600, "Allow .git content", false, ""},
	}
	testLinks := []linkDesc{
		{"link/okfilelink", "../dir01/dir02/test1.txt"},
		{"link/errfilelink", "../dir01/missing.target"},
		{"link/okdirlink", "../dir01/dir02"},
		{"link/okdirlink2", "../dir01/.git"},
	}
	if err = createTestFiles(tempDir, testDirs, testFiles, testLinks); err != nil {
		t.Fatalf("Cannot create test files: %v", err)
	}

	tarFile, err := th.CreateTarFile("", tempDir)
	defer os.Remove(tarFile)
	if err != nil {
		t.Fatalf("Unable to create new tar upload file: %v", err)
	}
	verifyTarFile(t, tarFile, testDirs, testFiles, testLinks)
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
		if fd.mode.IsDir() {
			if err := addDir(tw, &fd); err != nil {
				msg := "unable to add dir %q to archive: %v"
				return fmt.Errorf(msg, fd.name, err)
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

func addDir(tw *tar.Writer, fd *fileDesc) error {
	hdr := &tar.Header{
		Name:       fd.name,
		Mode:       int64(fd.mode & 0777),
		Typeflag:   tar.TypeDir,
		AccessTime: time.Now(),
		ModTime:    fd.modifiedDate,
		ChangeTime: fd.modifiedDate,
	}
	return tw.WriteHeader(hdr)
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
	if runtime.GOOS == "windows" {
		for i := range files {
			files[i].name = filepath.FromSlash(files[i].name)
			if files[i].mode&0200 == 0200 {
				// if the file is user writable make it writable for everyone
				files[i].mode |= 0666
			} else {
				// if the file is only readable, make it readable for everyone
				// first clear the r/w permission bits
				files[i].mode &^= 0666
				// then set r permission for all
				files[i].mode |= 0444
			}
			if files[i].mode.IsDir() {
				// if the file is a directory, make it executable for everyone
				files[i].mode |= 0111
			} else {
				// if it's not a directory, clear the executable bits as they are
				// irrelevant on windows.
				files[i].mode &^= 0111
			}
			files[i].target = filepath.FromSlash(files[i].target)
		}
	}
	pathsToVerify := make(map[string]fileDesc)
	for _, fd := range files {
		pathsToVerify[fd.name] = fd
	}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if path == dir {
			return nil
		}
		relpath := path[len(dir)+1:]
		if fd, ok := pathsToVerify[relpath]; ok {
			if info.Mode() != fd.mode {
				t.Errorf("File mode is not equal for %q. Expected: %v, Actual: %v",
					relpath, fd.mode, info.Mode())
			}
			// TODO: check modification time for symlinks when extractLink() will support it
			if info.ModTime().UTC() != fd.modifiedDate && !isSymLink(fd.mode) && !fd.mode.IsDir() {
				t.Errorf("File modified date is not equal for %q. Expected: %v, Actual: %v",
					relpath, fd.modifiedDate, info.ModTime())
			}
			if !info.IsDir() {
				contentBytes, err := ioutil.ReadFile(path)
				if err != nil {
					t.Errorf("Error reading file %q: %v", path, err)
					return err
				}
				content := string(contentBytes)
				if content != fd.content {
					t.Errorf("File content is not equal for %q. Expected: %q, Actual: %q",
						relpath, fd.content, content)
				}
			}
			if isSymLink(fd.mode) {
				target, err := os.Readlink(path)
				if err != nil {
					t.Errorf("Error reading symlink %q: %v", path, err)
					return err
				}
				if target != fd.target {
					msg := "Symbolic link %q points to wrong path. Expected: %q, Actual: %q"
					t.Errorf(msg, fd.name, fd.target, target)
				}
			}
		} else {
			t.Errorf("Unexpected file found: %q", relpath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Error walking directory %q: %v", dir, err)
	}
}

func TestExtractTarStream(t *testing.T) {
	modificationDate := time.Date(2011, time.March, 5, 23, 30, 1, 0, time.UTC)
	var symLinkMode os.FileMode = 0777
	if runtime.GOOS == "darwin" {
		// Symlinks show up as Lrwxr-xr-x on macOS
		symLinkMode = 0755
	}
	testFiles := []fileDesc{
		{"dir01", modificationDate, 0700 | os.ModeDir, "", false, ""},
		{"dir01/.git", modificationDate, 0755 | os.ModeDir, "", false, ""},
		{"dir01/dir02", modificationDate, 0755 | os.ModeDir, "", false, ""},
		{"dir01/dir03", modificationDate, 0775 | os.ModeDir, "", false, ""},
		{"dir01/dir02/test1.txt", modificationDate, 0700, "Test1 file content", false, ""},
		{"dir01/test2.git", modificationDate, 0660, "Test2 file content", false, ""},
		{"dir01/dir03/test3.txt", modificationDate, 0444, "Test3 file content", false, ""},
		{"dir01/symlink", modificationDate, os.ModeSymlink | symLinkMode, "Test3 file content", false, "../dir01/dir03/test3.txt"},
	}
	reader, writer := io.Pipe()
	destDir, err := ioutil.TempDir("", "testExtract")
	if err != nil {
		t.Fatalf("Cannot create temp directory: %v", err)
	}
	defer os.RemoveAll(destDir)
	th := New(fs.NewFileSystem())

	go func() {
		err := createTestTar(testFiles, writer)
		if err != nil {
			t.Fatalf("Error creating tar stream: %v", err)
		}
		writer.CloseWithError(err)
	}()
	th.ExtractTarStream(destDir, reader)
	verifyDirectory(t, destDir, testFiles)
}

func TestExtractTarStreamTimeout(t *testing.T) {
	reader, writer := io.Pipe()
	destDir, err := ioutil.TempDir("", "testExtract")
	if err != nil {
		t.Fatalf("Cannot create temp directory: %v", err)
	}
	defer os.RemoveAll(destDir)
	th := New(fs.NewFileSystem())
	th.(*stiTar).timeout = 10 * time.Millisecond
	time.AfterFunc(20*time.Millisecond, func() { writer.Close() })
	err = th.ExtractTarStream(destDir, reader)
	if e, ok := err.(s2ierr.Error); err == nil || (ok && e.ErrorCode != s2ierr.TarTimeoutError) {
		t.Errorf("Did not get the expected timeout error. err = %v", err)
	}
}
