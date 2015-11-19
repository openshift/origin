package scripts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
)

// Environment represents a single environment variable definition
type Environment struct {
	Name  string
	Value string
}

// GetEnvironment gets the .sti/environment file located in the sources and
// parse it into []environment
func GetEnvironment(config *api.Config) ([]Environment, error) {
	envPath := filepath.Join(config.WorkingDir, api.Source, ".sti", api.Environment)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil, errors.New("no environment file found in application sources")
	}

	f, err := os.Open(envPath)
	if err != nil {
		return nil, errors.New("unable to read .sti/environment file")
	}
	defer f.Close()

	result := []Environment{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		// Allow for comments in environment file
		if strings.HasPrefix(s, "#") {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			continue
		}
		e := Environment{
			Name:  strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
		}
		glog.V(1).Infof("Setting '%s' to '%s'", e.Name, e.Value)
		result = append(result, e)
	}

	return result, scanner.Err()
}

// ConvertEnvironment converts the []Environment to "key=val" strings
func ConvertEnvironment(env []Environment) (result []string) {
	for _, e := range env {
		result = append(result, fmt.Sprintf("%s=%s", e.Name, e.Value))
	}
	return
}

// ConvertEnvironmentToDocker converts the []Environment into Dockerfile format
func ConvertEnvironmentToDocker(env []Environment) (result string) {
	for i, e := range env {
		if i == 0 {
			result += fmt.Sprintf("ENV %s=\"%s\"", e.Name, e.Value)
		} else {
			result += fmt.Sprintf(" \\\n\t%s=\"%s\"", e.Name, e.Value)
		}
	}
	result += "\n"
	return
}
