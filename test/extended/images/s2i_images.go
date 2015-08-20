package images

import "fmt"

type tc struct {
	// The image version string (eg. '27' or '34')
	Version string
	// The base OS ('rhel7' or 'centos7')
	BaseOS string
	// Command to execute
	Cmd string
	// Expected output from the command
	Expected string

	// Internal: We resolve this in JustBeforeEach
	DockerImageReference string
}

// This is a complete list of supported S2I images
var s2iImages = map[string][]*tc{
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
		{
			Version:  "34",
			Cmd:      "python --version",
			Expected: "Python 3.4.2",
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

// S2ICentosImages returns a map of all supported S2I images based on Centos
func S2ICentosImages() map[string][]*tc {
	result := s2iImages
	for _, tcs := range result {
		for _, t := range tcs {
			t.BaseOS = "centos7"
		}
	}
	return result
}

// S2IRhelImages returns a map of all supported S2I images based on RHEL7
func S2IRhelImages() map[string][]*tc {
	result := s2iImages
	for _, tcs := range result {
		for _, t := range tcs {
			t.BaseOS = "rhel7"
		}
	}
	return result
}

// S2IAllImages returns a map of all supported S2I images
func S2IAllImages() map[string][]*tc {
	centos := S2ICentosImages()
	rhel := S2IRhelImages()
	for imageName, tcs := range centos {
		centos[imageName] = append(tcs, rhel[imageName]...)
	}
	return centos
}

// resolveDockerImageReferences resolves the pull specs for all images
func resolveDockerImageReferences() {
	for imageName, tcs := range s2iImages {
		for _, t := range tcs {
			t.DockerImageReference = fmt.Sprintf("openshift/%s-%s-%s", imageName, t.Version, t.BaseOS)
		}
	}
}
