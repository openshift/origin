package project

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	log "github.com/golang/glog"
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

// relativePath returns the proper relative path for the given file path. If
// the relativeTo string equals "-", then it means that it's from the stdin,
// and the returned path will be the current working directory. Otherwise, if
// file is really an absolute path, then it will be returned without any
// changes. Otherwise, the returned path will be a combination of relativeTo
// and file.
func relativePath(file, relativeTo string) string {
	// stdin: return the current working directory if possible.
	if relativeTo == "-" {
		if cwd, err := os.Getwd(); err == nil {
			return filepath.Join(cwd, file)
		}
	}

	// If the given file is already an absolute path, just return it.
	// Otherwise, the returned path will be relative to the given relativeTo
	// path.
	if filepath.IsAbs(file) {
		return file
	}

	abs, err := filepath.Abs(filepath.Join(path.Dir(relativeTo), file))
	if err != nil {
		log.V(4).Infof("Failed to get absolute directory: %s", err)
		return file
	}
	return abs
}

// FileResourceLookup is a "bare" structure that implements the project.ResourceLookup interface
type FileResourceLookup struct {
}

// Lookup returns the content and the actual filename of the file that is "built" using the
// specified file and relativeTo string. file and relativeTo are supposed to be file path.
// If file starts with a slash ('/'), it tries to load it, otherwise it will build a
// filename using the folder part of relativeTo joined with file.
func (f *FileResourceLookup) Lookup(file, relativeTo string) ([]byte, string, error) {
	file = relativePath(file, relativeTo)
	log.V(4).Infof("Reading file %s", file)
	bytes, err := ioutil.ReadFile(file)
	return bytes, file, err
}

// ResolvePath returns the path to be used for the given path volume. This
// function already takes care of relative paths.
func (f *FileResourceLookup) ResolvePath(path, relativeTo string) string {
	vs := strings.SplitN(path, ":", 2)
	if len(vs) != 2 || filepath.IsAbs(vs[0]) {
		return path
	}
	vs[0] = relativePath(vs[0], relativeTo)
	return strings.Join(vs, ":")
}
