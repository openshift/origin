package scripts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
)

// GetEnvironment gets the .s2i/environment file located in the sources and
// parse it into EnvironmentList.
func GetEnvironment(config *api.Config) (api.EnvironmentList, error) {
	envPath := filepath.Join(config.WorkingDir, api.Source, ".s2i", api.Environment)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// TODO: Remove this when the '.sti/environment' is deprecated.
		envPath = filepath.Join(config.WorkingDir, api.Source, ".sti", api.Environment)
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			return nil, errors.New("no environment file found in application sources")
		}
		glog.Info("DEPRECATED: Use .s2i/environment instead of .sti/environment")
	}

	f, err := os.Open(envPath)
	if err != nil {
		return nil, errors.New("unable to read custom environment file")
	}
	defer f.Close()

	result := api.EnvironmentList{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		// Allow for comments in environment file
		if strings.HasPrefix(s, "#") {
			continue
		}
		result.Set(s)
	}

	glog.V(1).Infof("Setting %d environment variables provided by environment file in sources", len(result))
	return result, scanner.Err()
}

// ConvertEnvironmentList converts the EnvironmentList to "key=val" strings.
func ConvertEnvironmentList(env api.EnvironmentList) (result []string) {
	for _, e := range env {
		result = append(result, fmt.Sprintf("%s=%s", e.Name, e.Value))
	}
	return
}

// ConvertEnvironmentToDocker converts the EnvironmentList into Dockerfile format.
func ConvertEnvironmentToDocker(env api.EnvironmentList) (result string) {
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
