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
			Version:    "26",
			Cmd:        "ruby --version",
			Expected:   "ruby 2.6",
			Repository: "rhscl",
		},
		{
			Version:    "25",
			Cmd:        "ruby --version",
			Expected:   "ruby 2.5",
			Repository: "rhscl",
		},
		{
			Version:    "24",
			Cmd:        "ruby --version",
			Expected:   "ruby 2.4",
			Repository: "rhscl",
		},
	},
	"python": {
		{
			Version:    "27",
			Cmd:        "python --version",
			Expected:   "Python 2.7",
			Repository: "rhscl",
		},
		{
			Version:    "36",
			Cmd:        "python --version",
			Expected:   "Python 3.6",
			Repository: "rhscl",
		},
	},
	"nodejs": {
		{
			Version:    "10",
			Cmd:        "node --version",
			Expected:   "v10",
			Repository: "rhscl",
		},
		{
			Version:    "12",
			Cmd:        "node --version",
			Expected:   "v12",
			Repository: "rhscl",
		},
	},
	"perl": {
		{
			Version:    "526",
			Cmd:        "perl --version",
			Expected:   "v5.26",
			Repository: "rhscl",
		},
	},
	"php": {
		{
			Version:    "72",
			Cmd:        "php --version",
			Expected:   "7.2",
			Repository: "rhscl",
		},
		{
			Version:    "73",
			Cmd:        "php --version",
			Expected:   "7.3",
			Repository: "rhscl",
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
	t.DockerImageReference = fmt.Sprintf("registry.redhat.io/%s/%s-%s-rhel7", t.Repository, name, t.Version)
}
