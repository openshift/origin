package scmauth

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/util/file"
)

func createGitConfig(includePath string) error {
	tempDir, err := ioutil.TempDir("", "git")
	if err != nil {
		return err
	}
	gitconfig := filepath.Join(tempDir, ".gitconfig")
	content := fmt.Sprintf("[include]\npath = %s\n", includePath)
	if err := ioutil.WriteFile(gitconfig, []byte(content), 0600); err != nil {
		return err
	}
	// The GIT_CONFIG variable won't affect regular git operation
	// therefore the HOME variable needs to be set so git can pick up
	// .gitconfig from that location. The GIT_CONFIG variable is still used
	// to track the location of the GIT_CONFIG for multiple SCMAuth objects.
	os.Setenv("HOME", tempDir)
	os.Setenv("GIT_CONFIG", gitconfig)
	return nil
}

// ensureGitConfigIncludes ensures that the OS env var GIT_CONFIG is set and
// that it points to a file that has an include statement for the given path
func ensureGitConfigIncludes(path string) error {
	gitconfig := os.Getenv("GIT_CONFIG")
	if gitconfig == "" {
		return createGitConfig(path)
	}

	lines, err := file.ReadLines(gitconfig)
	if err != nil {
		return err
	}
	for _, line := range lines {
		// If include already exists, return with no error
		if line == fmt.Sprintf("path = %s", path) {
			return nil
		}
	}

	lines = append(lines, fmt.Sprintf("path = %s", path))
	content := []byte(strings.Join(lines, "\n"))
	return ioutil.WriteFile(gitconfig, content, 0644)
}
