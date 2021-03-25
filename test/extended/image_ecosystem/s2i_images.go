package image_ecosystem

import (
	"fmt"
)

type ImageBaseType string

type tc struct {
	// The image version string (eg. '27' or '34')
	Version string
	// Command to execute
	Cmd string
	// Expected output from the command
	Expected string

	// Tag is the image tag to correlates to the Version string
	Tag string

	// Internal: We resolve this in JustBeforeEach
	DockerImageReference string

	// whether this image is supported on s390x or ppc64le
	NonAMD bool
}

// This is a complete list of supported S2I images
var s2iImages = map[string][]tc{
	"ruby": {
		{
			Version:  "27",
			Cmd:      "ruby --version",
			Expected: "ruby 2.7",
			Tag:      "2.7",
			NonAMD:   true,
		},
		{
			Version:  "26",
			Cmd:      "ruby --version",
			Expected: "ruby 2.6",
			Tag:      "2.6",
			NonAMD:   true,
		},
	},
	"python": {
		{
			Version:  "27",
			Cmd:      "python --version",
			Expected: "Python 2.7",
			Tag:      "2.7",
			NonAMD:   true,
		},
		{
			Version:  "36",
			Cmd:      "python --version",
			Expected: "Python 3.6",
			Tag:      "3.6-ubi8",
			NonAMD:   true,
		},
	},
	"nodejs": {
		{
			Version:  "12",
			Cmd:      "node --version",
			Expected: "v12",
			Tag:      "12",
			NonAMD:   true,
		},
	},
	"perl": {
		{
			Version:  "530",
			Cmd:      "perl --version",
			Expected: "v5.30",
			Tag:      "5.30",
			NonAMD:   true,
		},
	},
	"php": {
		{
			Version:  "72",
			Cmd:      "php --version",
			Expected: "7.2",
			Tag:      "7.2-ubi8",
			NonAMD:   true,
		},
		{
			Version:  "73",
			Cmd:      "php --version",
			Expected: "7.3",
			Tag:      "7.3",
			NonAMD:   true,
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
	t.DockerImageReference = fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/openshift/%s:%s", name, t.Tag)
}
