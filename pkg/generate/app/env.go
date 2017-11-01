package app

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/joho/godotenv"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	kapi "k8s.io/kubernetes/pkg/api"
)

// Environment holds environment variables for new-app
type environmentEntry struct {
	key   string
	value string
}
type Environment []environmentEntry

// ParseEnvironmentAllowEmpty converts the provided strings in key=value form
// into environment entries. In case there's no equals sign in a string, it's
// considered as a key with empty value.
func ParseEnvironmentAllowEmpty(vals ...string) Environment {
	env := Environment{}
	for _, s := range vals {
		var key, value string
		if i := strings.Index(s, "="); i == -1 {
			key = s
			value = ""
		} else {
			key = s[:i]
			value = s[i+1:]
		}
		env.Add(key, value)
	}
	return env
}

// ParseEnvironment takes a slice of strings in key=value format and transforms
// them into an Environment. List of duplicate keys is returned in the second return
// value.
func ParseEnvironment(vals ...string) (Environment, []string, []error) {
	errs := []error{}
	duplicates := []string{}
	env := Environment{}
	for _, s := range vals {
		valid := cmdutil.IsValidEnvironmentArgument(s)
		p := strings.SplitN(s, "=", 2)
		if !valid || len(p) != 2 {
			errs = append(errs, fmt.Errorf("invalid parameter assignment in %q", s))
			continue
		}
		key, val := p[0], p[1]
		if env.Has(key) {
			duplicates = append(duplicates, key)
			continue
		}
		env.Add(key, val)
	}
	return env, duplicates, errs
}

// NewEnvironment returns a new Environment variable based on all
// the provided Environment variables
func NewEnvironment(envs ...Environment) Environment {
	out := Environment{}
	out.AddEnvs(envs...)
	return out
}

// NewEnvironmentFromEnvVars returns a new Environment with the
// entries from the given environment variables
func NewEnvironmentFromEnvVars(envs []kapi.EnvVar) Environment {
	out := Environment{}
	out.AddEnvVars(envs)
	return out
}

// NewEnvironmentFromMap returns a new Environment with the
// entries from the given map
func NewEnvironmentFromMap(m map[string]string) Environment {
	out := Environment{}
	out.AddMap(m)
	return out
}

// AddEnvs adds the given Environments to the current environment
func (e *Environment) AddEnvs(envs ...Environment) {
	for _, env := range envs {
		for _, entry := range env {
			e.Add(entry.key, entry.value)
		}
	}
}

func (e *Environment) AddMap(m map[string]string) {
	for k, v := range m {
		e.Add(k, v)
	}
}

// AddEnvVars adds EnvVar entries to the current environment
func (e *Environment) AddEnvVars(envs []kapi.EnvVar) {
	for _, entry := range envs {
		e.Add(entry.Name, entry.Value)
	}
}

// Add adds a single entry to the current environment
func (e *Environment) Add(key, value string) {
	(*e) = append(*e, environmentEntry{key: key, value: value})
}

// Has returns true if the passed key exists in the current environment
func (e *Environment) Has(key string) bool {
	for _, entry := range *e {
		if entry.key == key {
			return true
		}
	}
	return false
}

// AddIfNotPresent adds the environment variables to the current environment.
// In case of key conflict the old value is kept. Conflicting keys are returned
// as a slice.
func (e *Environment) AddIfNotPresent(more Environment) []string {
	duplicates := []string{}
	for _, entry := range more {
		if e.Has(entry.key) {
			duplicates = append(duplicates, entry.key)
		} else {
			e.Add(entry.key, entry.value)
		}
	}
	return duplicates
}

// Map returns a map from the current environment
func (e *Environment) Map() map[string]string {
	result := map[string]string{}
	for _, entry := range *e {
		result[entry.key] = entry.value
	}
	return result
}

// List returns all the environment variables
func (e *Environment) List() []kapi.EnvVar {
	env := []kapi.EnvVar{}
	for _, entry := range *e {
		env = append(env, kapi.EnvVar{
			Name:  entry.key,
			Value: entry.value,
		})
	}
	return env
}

// JoinEnvironment joins two different sets of environment variables
// into one, leaving out all the duplicates
func JoinEnvironment(a, b []kapi.EnvVar) []kapi.EnvVar {
	out := NewEnvironmentFromEnvVars(a)
	for _, envVar := range b {
		if !out.Has(envVar.Name) {
			out.Add(envVar.Name, envVar.Value)
		}
	}
	return out.List()
}

// LoadEnvironmentFile accepts filename of a file containing key=value pairs
// and puts these pairs into a map. If filename is "-" the file contents are
// read from the stdin argument, provided it is not nil.
func LoadEnvironmentFile(filename string, stdin io.Reader) (Environment, error) {
	errorFilename := filename

	if filename == "-" && stdin != nil {
		temp, err := ioutil.TempFile("", "origin-env-stdin")
		if err != nil {
			return nil, fmt.Errorf("Cannot create temporary file: %s", err)
		}
		defer temp.Close()

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
		return nil, fmt.Errorf("Cannot stat %q: %v", filename, err)
	}

	env, err := godotenv.Read(filename)
	if err != nil {
		return nil, fmt.Errorf("Cannot read variables from file %q: %s", errorFilename, err)
	}
	result := Environment{}
	for k, v := range env {
		if !cmdutil.IsValidEnvironmentArgument(fmt.Sprintf("%s=%s", k, v)) {
			return nil, fmt.Errorf("invalid parameter assignment in %s=%s", k, v)
		}
		result.Add(k, v)
	}
	return result, nil
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
