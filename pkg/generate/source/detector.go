package source

import (
	"os"
	"path/filepath"
)

// Info is detected platform information from a source directory
type Info struct {
	Platform string
	Version  string
}

// DetectorFunc is a function that returns source Info from a given directory.
// It returns true if it was able to detect the code in the given directory.
type DetectorFunc func(dir string) (*Info, bool)

// Detectors is a set of DetectorFunc that is used to detect the
// language/platform for a given source directory
type Detectors []DetectorFunc

// DefafultDetectors is a default set of Detector functions
var DefaultDetectors = Detectors{
	DetectRuby,
	DetectJava,
	DetectNodeJS,
}

type sourceDetector struct {
	detectors []DetectorFunc
}

// DetectSource returns source information from a given directory using
// a set of Detectors
func (s Detectors) DetectSource(dir string) (*Info, bool) {
	for _, d := range s {
		if info, found := d(dir); found {
			return info, true
		}
	}
	return nil, false
}

// DetectRuby detects whether the source code in the given repository is Ruby
func DetectRuby(dir string) (*Info, bool) {
	if filesPresent(dir, []string{"Gemfile", "Rakefile", "config.ru"}) {
		return &Info{
			Platform: "Ruby",
		}, true
	}
	return nil, false
}

// DetectJava detects whether the source code in the given repository is Java
func DetectJava(dir string) (*Info, bool) {
	if filesPresent(dir, []string{"pom.xml"}) {
		return &Info{
			Platform: "JEE",
		}, true
	}
	return nil, false
}

// DetectNodeJS detects whether the source code in the given repository is NodeJS
func DetectNodeJS(dir string) (*Info, bool) {
	if filesPresent(dir, []string{"config.json", "package.json"}) {
		return &Info{
			Platform: "NodeJS",
		}, true
	}
	return nil, false
}

func filesPresent(dir string, files []string) bool {
	for _, f := range files {
		_, err := os.Stat(filepath.Join(dir, f))
		if err == nil {
			return true
		}
	}
	return false
}
