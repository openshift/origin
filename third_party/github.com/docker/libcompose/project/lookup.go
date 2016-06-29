package project

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Lookup creates a string slice of string containing a "docker-friendly" environment string
// in the form of 'key=value'. It loop through the lookups and returns the latest value if
// more than one lookup return a result.
func (l *ComposableEnvLookup) Lookup(key, serviceName string, config *ServiceConfig) []string {
	result := []string{}
	for _, lookup := range l.Lookups {
		env := lookup.Lookup(key, serviceName, config)
		if len(env) == 1 {
			result = env
		}
	}
	return result
}

// Lookup creates a string slice of string containing a "docker-friendly" environment string
// in the form of 'key=value'. It gets environment values using a '.env' file in the specified
// path.
func (l *EnvfileLookup) Lookup(key, serviceName string, config *ServiceConfig) []string {
	envs, err := ParseEnvFile(l.Path)
	if err != nil {
		return []string{}
	}
	for _, env := range envs {
		e := strings.Split(env, "=")
		if e[0] == key {
			return []string{env}
		}
	}
	return []string{}
}

// Lookup creates a string slice of string containing a "docker-friendly" environment string
// in the form of 'key=value'. It gets environment values using os.Getenv.
// If the os environment variable does not exists, the slice is empty. serviceName and config
// are not used at all in this implementation.
func (o *OsEnvLookup) Lookup(key, serviceName string, config *ServiceConfig) []string {
	ret := os.Getenv(key)
	if ret == "" {
		return []string{}
	}
	return []string{fmt.Sprintf("%s=%s", key, ret)}
}

var whiteSpaces = " \t"

// ParseEnvFile reads a file with environment variables enumerated by lines
//
// ``Environment variable names used by the utilities in the Shell and
// Utilities volume of IEEE Std 1003.1-2001 consist solely of uppercase
// letters, digits, and the '_' (underscore) from the characters defined in
// Portable Character Set and do not begin with a digit. *But*, other
// characters may be permitted by an implementation; applications shall
// tolerate the presence of such names.''
// -- http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html
//
// As of #16585, it's up to application inside docker to validate or not
// environment variables, that's why we just strip leading whitespace and
// nothing more.
func ParseEnvFile(filename string) ([]string, error) {
	fh, err := os.Open(filename)
	if err != nil {
		return []string{}, err
	}
	defer fh.Close()

	lines := []string{}
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		// trim the line from all leading whitespace first
		line := strings.TrimLeft(scanner.Text(), whiteSpaces)
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			data := strings.SplitN(line, "=", 2)

			// trim the front of a variable, but nothing else
			variable := strings.TrimLeft(data[0], whiteSpaces)
			if strings.ContainsAny(variable, whiteSpaces) {
				return []string{}, fmt.Errorf("variable '%s' has white spaces", variable)
			}

			if len(data) > 1 {

				// pass the value through, no trimming
				lines = append(lines, fmt.Sprintf("%s=%s", variable, data[1]))
			} else {
				// if only a pass-through variable is given, clean it up.
				lines = append(lines, fmt.Sprintf("%s=%s", strings.TrimSpace(line), os.Getenv(line)))
			}
		}
	}
	return lines, scanner.Err()
}
