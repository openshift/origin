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
			Version:  "20",
			Cmd:      "ruby --version",
			Expected: "ruby 2.0.0",
		},
		{
			Version:  "22",
			Cmd:      "ruby --version",
			Expected: "ruby 2.2.2",
		},
	},
	"python": {
		{
			Version:  "27",
			Cmd:      "python --version",
			Expected: "Python 2.7.8",
		},
		{
			Version:  "33",
			Cmd:      "python --version",
			Expected: "Python 3.3.2",
		},
	},
	"nodejs": {
		{
			Version:  "010",
			Cmd:      "node --version",
			Expected: "v0.10",
		},
	},
	"perl": {
		{
			Version:  "516",
			Cmd:      "perl --version",
			Expected: "v5.16.3",
		},
		{
			Version:  "520",
			Cmd:      "perl --version",
			Expected: "v5.20.1",
		},
	},
	"php": {
		{
			Version:  "55",
			Cmd:      "php --version",
			Expected: "5.5",
		},
		{
			Version:  "56",
			Cmd:      "php --version",
			Expected: "5.6",
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
