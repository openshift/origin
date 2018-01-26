package app

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	utilenv "github.com/openshift/origin/pkg/oc/util/env"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

// Environment holds environment variables for new-app
type Environment map[string]string

// ParseEnvironmentAllowEmpty converts the provided strings in key=value form
// into environment entries. In case there's no equals sign in a string, it's
// considered as a key with empty value.
func ParseEnvironmentAllowEmpty(vals ...string) Environment {
	env := make(Environment)
	for _, s := range vals {
		if i := strings.Index(s, "="); i == -1 {
			env[s] = ""
		} else {
			env[s[:i]] = s[i+1:]
		}
	}
	return env
}

// ParseEnvironment takes a slice of strings in key=value format and transforms
// them into a map. List of duplicate keys is returned in the second return
// value.
func ParseEnvironment(vals ...string) (Environment, []string, []error) {
	errs := []error{}
	duplicates := []string{}
	env := make(Environment)
	for _, s := range vals {
		valid := utilenv.IsValidEnvironmentArgument(s)
		p := strings.SplitN(s, "=", 2)
		if !valid || len(p) != 2 {
			errs = append(errs, fmt.Errorf("invalid parameter assignment in %q", s))
			continue
		}
		key, val := p[0], p[1]
		if _, exists := env[key]; exists {
			duplicates = append(duplicates, key)
			continue
		}
		env[key] = val
	}
	return env, duplicates, errs
}

// NewEnvironment returns a new set of environment variables based on all
// the provided environment variables
func NewEnvironment(envs ...map[string]string) Environment {
	if len(envs) == 1 {
		return envs[0]
	}
	out := make(Environment)
	out.Add(envs...)
	return out
}

// Add adds the environment variables to the current environment
func (e Environment) Add(envs ...map[string]string) {
	for _, env := range envs {
		for k, v := range env {
			e[k] = v
		}
	}
}

// AddIfNotPresent adds the environment variables to the current environment.
// In case of key conflict the old value is kept. Conflicting keys are returned
// as a slice.
func (e Environment) AddIfNotPresent(more Environment) []string {
	duplicates := []string{}
	for k, v := range more {
		if _, exists := e[k]; exists {
			duplicates = append(duplicates, k)
		} else {
			e[k] = v
		}
	}

	return duplicates
}

// List sorts and returns all the environment variables
func (e Environment) List() []kapi.EnvVar {
	env := []kapi.EnvVar{}
	for k, v := range e {
		env = append(env, kapi.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	sort.Sort(sortedEnvVar(env))
	return env
}

type sortedEnvVar []kapi.EnvVar

func (m sortedEnvVar) Len() int           { return len(m) }
func (m sortedEnvVar) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m sortedEnvVar) Less(i, j int) bool { return m[i].Name < m[j].Name }

// JoinEnvironment joins two different sets of environment variables
// into one, leaving out all the duplicates
func JoinEnvironment(a, b []kapi.EnvVar) (out []kapi.EnvVar) {
	out = a
	for i := range b {
		exists := false
		for j := range a {
			if a[j].Name == b[i].Name {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		out = append(out, b[i])
	}
	return out
}

// LoadEnvironmentFile accepts filename of a file containing key=value pairs
// and puts these pairs into a map. If filename is "-" the file contents are
// read from the stdin argument, provided it is not nil.
func LoadEnvironmentFile(filename string, stdin io.Reader) (Environment, error) {
	errorFilename := filename

	if filename == "-" && stdin != nil {
		//once https://github.com/joho/godotenv/pull/20 is merged we can get rid of using tempfile
		temp, err := ioutil.TempFile("", "origin-env-stdin")
		if err != nil {
			return nil, fmt.Errorf("Cannot create temporary file: %s", err)
		}

		filename = temp.Name()
		errorFilename = "stdin"
		defer os.Remove(filename)

		if _, err = io.Copy(temp, stdin); err != nil {
			return nil, fmt.Errorf("Cannot write to temporary file %q: %s", filename, err)
		}
		temp.Close()
	}

	// godotenv successfuly returns empty map when given path to a directory,
	// remove this once https://github.com/joho/godotenv/pull/22 is merged
	if info, err := os.Stat(filename); err == nil && info.IsDir() {
		return nil, fmt.Errorf("Cannot read variables from %q: is a directory", filename)
	} else if err != nil {
		return nil, fmt.Errorf("Cannot stat %q: %s", filename, err)
	}

	env, err := godotenv.Read(filename)
	if err != nil {
		return nil, fmt.Errorf("Cannot read variables from file %q: %s", errorFilename, err)
	}
	for k, v := range env {
		if !utilenv.IsValidEnvironmentArgument(fmt.Sprintf("%s=%s", k, v)) {
			return nil, fmt.Errorf("invalid parameter assignment in %s=%s", k, v)
		}
	}
	return env, nil
}

// ParseAndCombineEnvironment parses key=value records from slice of strings
// (typically obtained from the command line) and from given files and combines
// them into single map. Key=value pairs from the envs slice have precedence
// over those read from file.
//
// The dupfn function is called for all duplicate keys that encountered. If the
// function returns an error this error is returned by
// ParseAndCombineEnvironment.
//
// If a file is "-" the file contents will be read from argument stdin (unless
// it's nil).
func ParseAndCombineEnvironment(envs []string, filenames []string, stdin io.Reader, dupfn func(string, string) error) (Environment, error) {
	vars, duplicates, errs := ParseEnvironment(envs...)
	if len(errs) > 0 {
		return nil, errs[0]
	}
	for _, s := range duplicates {
		if err := dupfn(s, ""); err != nil {
			return nil, err
		}
	}

	for _, fname := range filenames {
		fileVars, err := LoadEnvironmentFile(fname, stdin)
		if err != nil {
			return nil, err
		}

		duplicates = vars.AddIfNotPresent(fileVars)
		for _, s := range duplicates {
			if err := dupfn(s, fname); err != nil {
				return nil, err
			}
		}
	}

	return vars, nil
}
