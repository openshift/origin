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

	// Architectures on which this image is available
	Arches []string
}

// This is a complete list of supported S2I images
var s2iImages = map[string][]tc{
	"ruby": {
		{
			Version:  "30",
			Cmd:      "ruby --version",
			Expected: "ruby 3.0",
			Tag:      "3.0-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "30",
			Cmd:      "ruby --version",
			Expected: "ruby 3.0",
			Tag:      "3.0-ubi7",
			Arches:   []string{"amd64", "ppc64le", "s390x"},
		},
		{
			Version:  "27",
			Cmd:      "ruby --version",
			Expected: "ruby 2.7",
			Tag:      "2.7-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "27",
			Cmd:      "ruby --version",
			Expected: "ruby 2.7",
			Tag:      "2.7-ubi7",
			Arches:   []string{"amd64", "ppc64le", "s390x"},
		},
		{
			Version:  "25",
			Cmd:      "ruby --version",
			Expected: "ruby 2.5",
			Tag:      "2.5-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
	},
	"python": {
		{
			Version:  "27",
			Cmd:      "python --version",
			Expected: "Python 2.7",
			Tag:      "2.7-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "36",
			Cmd:      "python --version",
			Expected: "Python 3.6",
			Tag:      "3.6-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "38",
			Cmd:      "python --version",
			Expected: "Python 3.8",
			Tag:      "3.8-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "38",
			Cmd:      "python --version",
			Expected: "Python 3.8",
			Tag:      "3.8-ubi7",
			Arches:   []string{"amd64", "ppc64le", "s390x"},
		},
		{
			Version:  "39",
			Cmd:      "python --version",
			Expected: "Python 3.9",
			Tag:      "3.9-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
	},
	"nodejs": {
		{
			Version:  "16",
			Cmd:      "node --version",
			Expected: "v16",
			Tag:      "16-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "14",
			Cmd:      "node --version",
			Expected: "v14",
			Tag:      "14-ubi7",
			Arches:   []string{"amd64", "ppc64le", "s390x"},
		},
	},
	"perl": {
		{
			Version:  "532",
			Cmd:      "perl --version",
			Expected: "v5.32",
			Tag:      "5.32-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "530",
			Cmd:      "perl --version",
			Expected: "v5.30",
			Tag:      "5.30-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "530",
			Cmd:      "perl --version",
			Expected: "v5.30",
			Tag:      "5.30-el7",
			Arches:   []string{"amd64", "ppc64le", "s390x"},
		},
		{
			Version:  "526",
			Cmd:      "perl --version",
			Expected: "v5.26",
			Tag:      "5.26-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
	},
	"php": {
		{
			Version:  "80",
			Cmd:      "php --version",
			Expected: "8.0",
			Tag:      "8.0-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "74",
			Cmd:      "php --version",
			Expected: "7.4",
			Tag:      "7.4-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "73",
			Cmd:      "php --version",
			Expected: "7.3",
			Tag:      "7.3-ubi7",
			Arches:   []string{"amd64", "ppc64le", "s390x"},
		},
	},
	"nginx": {
		{
			Version:  "120",
			Cmd:      "nginx -V",
			Expected: "nginx/1.20",
			Tag:      "1.20-ubi8",
			Arches:   []string{"amd64", "arm64", "ppc64le", "s390x"},
		},
		{
			Version:  "120",
			Cmd:      "nginx -V",
			Expected: "nginx/1.20",
			Tag:      "1.20-ubi7",
			Arches:   []string{"amd64", "ppc64le", "s390x"},
		},
	},
	"dotnet": {
		{
			Version:  "60",
			Cmd:      "dotnet --version",
			Expected: "6.0",
			Tag:      "6.0-ubi8",
			Arches:   []string{"amd64", "arm64", "s390x"},
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
