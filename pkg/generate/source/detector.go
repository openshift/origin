package source

import (
	"os"
	"path/filepath"
)

type Info struct {
	Platform string
	Version  string
}

type DetectorFunc func(dir string) (*Info, bool)

type Detectors []DetectorFunc

var DefaultDetectors = Detectors{
	DetectRuby,
	DetectJava,
	DetectNodeJS,
}

type sourceDetector struct {
	detectors []DetectorFunc
}

func (s Detectors) DetectSource(dir string) (*Info, bool) {
	for _, d := range s {
		if info, found := d(dir); found {
			return info, true
		}
	}
	return nil, false
}

func DetectRuby(dir string) (*Info, bool) {
	if filesPresent(dir, []string{"Gemfile", "Rakefile"}) {
		return &Info{
			Platform: "Ruby",
		}, true
	}
	return nil, false
}

func DetectJava(dir string) (*Info, bool) {
	if filesPresent(dir, []string{"pom.xml"}) {
		return &Info{
			Platform: "JEE",
		}, true
	}
	return nil, false
}

func DetectNodeJS(dir string) (*Info, bool) {
	if filesPresent(dir, []string{"config.json"}) {
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
