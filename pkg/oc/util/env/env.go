package env

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

var argumentEnvironment = regexp.MustCompile(`(?ms)^(.+)\=(.*)$`)
var validArgumentEnvironment = regexp.MustCompile(`(?ms)^(\w+)\=(.*)$`)

func IsEnvironmentArgument(s string) bool {
	return argumentEnvironment.MatchString(s)
}

func IsValidEnvironmentArgument(s string) bool {
	return validArgumentEnvironment.MatchString(s)
}

func SplitEnvironmentFromResources(args []string) (resources, envArgs []string, ok bool) {
	first := true
	for _, s := range args {
		// this method also has to understand env removal syntax, i.e. KEY-
		isEnv := IsEnvironmentArgument(s) || strings.HasSuffix(s, "-")
		switch {
		case first && isEnv:
			first = false
			fallthrough
		case !first && isEnv:
			envArgs = append(envArgs, s)
		case first && !isEnv:
			resources = append(resources, s)
		case !first && !isEnv:
			return nil, nil, false
		}
	}
	return resources, envArgs, true
}

// parseIntoEnvVar parses the list of key-value pairs into kubernetes EnvVar.
// envVarType is for making errors more specific to user intentions.
func parseIntoEnvVar(spec []string, defaultReader io.Reader, envVarType string) ([]kapi.EnvVar, []string, error) {
	env := []kapi.EnvVar{}
	exists := sets.NewString()
	var remove []string
	for _, envSpec := range spec {
		switch {
		case !IsValidEnvironmentArgument(envSpec) && !strings.HasSuffix(envSpec, "-"):
			return nil, nil, fmt.Errorf("%ss must be of the form key=value and can only contain letters, numbers, and underscores", envVarType)
		case envSpec == "-":
			if defaultReader == nil {
				return nil, nil, fmt.Errorf("when '-' is used, STDIN must be open")
			}
			fileEnv, err := readEnv(defaultReader, envVarType)
			if err != nil {
				return nil, nil, err
			}
			env = append(env, fileEnv...)
		case strings.Contains(envSpec, "="):
			parts := strings.SplitN(envSpec, "=", 2)
			if len(parts) != 2 {
				return nil, nil, fmt.Errorf("invalid %s: %v", envVarType, envSpec)
			}
			exists.Insert(parts[0])
			env = append(env, kapi.EnvVar{
				Name:  parts[0],
				Value: parts[1],
			})
		case strings.HasSuffix(envSpec, "-"):
			remove = append(remove, envSpec[:len(envSpec)-1])
		default:
			return nil, nil, fmt.Errorf("unknown %s: %v", envVarType, envSpec)
		}
	}
	for _, removeLabel := range remove {
		if _, found := exists[removeLabel]; found {
			return nil, nil, fmt.Errorf("can not both modify and remove the same %s in the same command", envVarType)
		}
	}
	return env, remove, nil
}

func ParseBuildArg(spec []string, defaultReader io.Reader) ([]kapi.EnvVar, error) {
	env, _, err := parseIntoEnvVar(spec, defaultReader, "build-arg")
	return env, err
}

func ParseEnv(spec []string, defaultReader io.Reader) ([]kapi.EnvVar, []string, error) {
	return parseIntoEnvVar(spec, defaultReader, "environment variable")
}

func ParseAnnotation(spec []string, defaultReader io.Reader) (map[string]string, []string, error) {
	vars, remove, err := parseIntoEnvVar(spec, defaultReader, "annotation")
	annotations := make(map[string]string)
	for _, v := range vars {
		annotations[v.Name] = v.Value
	}
	return annotations, remove, err
}

func readEnv(r io.Reader, envVarType string) ([]kapi.EnvVar, error) {
	env := []kapi.EnvVar{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		envSpec := scanner.Text()
		if pos := strings.Index(envSpec, "#"); pos != -1 {
			envSpec = envSpec[:pos]
		}
		if strings.Contains(envSpec, "=") {
			parts := strings.SplitN(envSpec, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid %s: %v", envVarType, envSpec)
			}
			env = append(env, kapi.EnvVar{
				Name:  parts[0],
				Value: parts[1],
			})
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return env, nil
}
