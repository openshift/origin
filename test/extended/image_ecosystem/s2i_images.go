package image_ecosystem

import "fmt"

type ImageBaseType string

type tc struct {
	// The image version string (eg. '27' or '34')
	Version string
	// Command to execute
	Cmd string
	// Expected output from the command
	Expected string

	// Repository is either openshift/ or rhcsl/
	// The default is 'openshift'
	Repository string

	// Internal: We resolve this in JustBeforeEach
	DockerImageReference string
}

// This is a complete list of supported S2I images
var s2iImages = map[string][]tc{
	"ruby": {
		{
			Version:    "22",
			Cmd:        "ruby --version",
			Expected:   "ruby 2.2.2",
			Repository: "centos",
		},
		{
			Version:    "23",
			Cmd:        "ruby --version",
			Expected:   "ruby 2.3",
			Repository: "centos",
		},
		{
			Version:    "24",
			Cmd:        "ruby --version",
			Expected:   "ruby 2.4",
			Repository: "centos",
		},
	},
	"python": {
		{
			Version:    "27",
			Cmd:        "python --version",
			Expected:   "Python 2.7",
			Repository: "centos",
		},
		{
			Version:    "34",
			Cmd:        "python --version",
			Expected:   "Python 3.4",
			Repository: "centos",
		},
		{
			Version:    "35",
			Cmd:        "python --version",
			Expected:   "Python 3.5",
			Repository: "centos",
		},
		{
			Version:    "36",
			Cmd:        "python --version",
			Expected:   "Python 3.6",
			Repository: "centos",
		},
	},
	"nodejs": {
		{
			Version:    "4",
			Cmd:        "node --version",
			Expected:   "v4",
			Repository: "centos",
		},
		{
			Version:    "6",
			Cmd:        "node --version",
			Expected:   "v6",
			Repository: "centos",
		},
	},
	"perl": {
		{
			Version:    "520",
			Cmd:        "perl --version",
			Expected:   "v5.20",
			Repository: "centos",
		},
		{
			Version:    "524",
			Cmd:        "perl --version",
			Expected:   "v5.24",
			Repository: "centos",
		},
	},
	"php": {
		{
			Version:    "56",
			Cmd:        "php --version",
			Expected:   "5.6",
			Repository: "centos",
		},
		{
			Version:    "70",
			Cmd:        "php --version",
			Expected:   "7.0",
			Repository: "centos",
		},
	},
}

func GetTestCaseForImages() map[string][]tc {
	result := make(map[string][]tc)
	for name, variants := range s2iImages {
		for i := range variants {
			resolveDockerImageReference(name, &variants[i])
			result[name] = append(result[name], variants[i])
		}
	}
	return result
}

// resolveDockerImageReferences resolves the pull specs for all images
func resolveDockerImageReference(name string, t *tc) {
	if len(t.Repository) == 0 {
		t.Repository = "openshift"
	}
	t.DockerImageReference = fmt.Sprintf("%s/%s-%s-centos7", t.Repository, name, t.Version)
}
