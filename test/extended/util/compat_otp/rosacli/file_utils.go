package rosacli

import (
	"os"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

func CreateTempFileWithContent(fileContent string) (string, error) {
	return CreateTempFileWithPrefixAndContent("tmpfile", fileContent)
}

func CreateTempFileWithPrefixAndContent(prefix string, fileContent string) (string, error) {
	f, err := os.CreateTemp("", prefix+"-")
	if err != nil {
		return "", err
	}
	return CreateFileWithContent(f.Name(), fileContent)
}

// Write string to a file
func CreateFileWithContent(fileAbsPath string, content string) (string, error) {
	err := os.WriteFile(fileAbsPath, []byte(content), 0644)
	if err != nil {
		logger.Errorf("Failed to write to file: %s", err)
		return "", err
	}
	return fileAbsPath, err
}
