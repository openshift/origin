package compat_otp

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// The string pairs to update in a file
type StringPairs struct {
	OldString string
	NewString string
}

// DuplicateFileToPath copies the file at srcPath to destPath.
func DuplicateFileToPath(srcPath string, destPath string) {
	var destFile, srcFile *os.File
	var err error

	srcFile, err = os.Open(srcPath)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() {
		o.Expect(srcFile.Close()).NotTo(o.HaveOccurred())
	}()

	// If the file already exists, it is truncated. If the file does not exist, it is created with mode 0666.
	destFile, err = os.Create(destPath)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() {
		o.Expect(destFile.Close()).NotTo(o.HaveOccurred())
	}()

	_, err = io.Copy(destFile, srcFile)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(destFile.Sync()).NotTo(o.HaveOccurred())
}

// DuplicateFileToTemp creates a temporary duplicate of the file at srcPath using destPattern for naming,
// returning the path of the duplicate.
func DuplicateFileToTemp(srcPath string, destPrefix string) string {
	destFile, err := os.CreateTemp(os.TempDir(), destPrefix)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create temporary file")
	o.Expect(destFile.Close()).NotTo(o.HaveOccurred(), "Failed to close temporary file")

	destPath := destFile.Name()
	DuplicateFileToPath(srcPath, destPath)
	return destPath
}

// MoveFileToPath attempts to move a file from srcPath to destPath.
func MoveFileToPath(srcPath string, destPath string) {
	switch err := os.Rename(srcPath, destPath); {
	case err == nil:
		return
	case strings.Contains(err.Error(), "invalid cross-device link"):
		e2e.Logf("Failed to rename file from %s to %s: %v, attempting an alternative", srcPath, destPath, err)
		DuplicateFileToPath(srcPath, destPath)
		o.Expect(os.Remove(srcPath)).NotTo(o.HaveOccurred(), "Failed to remove source file")
	default:
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to rename source file")
	}
}

// PathExists
// @Description	Check if a file exists
// @Create 		jianl Jan 22 2025
// @Param 		path	string	the file path
// @Return		(bool, error)
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// updateFile
// @Description	Replace oldString in a given file by newString
// @Create 		jianl Jan 22 2025
// @Param 		filePath 	string 			the file path
// @Param 		stringPairs StringPairs 	a list of struct of old string and new string
// @Return 		nil
func UpdateFile(filePath string, stringPairs []StringPairs) (err error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Panicf("failed to read file content: %s", err)
		return err
	}
	fileContent := string(content)
	for _, pair := range stringPairs {
		fileContent = strings.Replace(fileContent, pair.OldString, pair.NewString, -1)
	}
	err = os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		log.Panicf("failed to write file: %s", err)
		return err
	}
	return nil
}
