package images

import "fmt"

type ImageBaseType string

const (
	RHELBased   ImageBaseType = "rhel7"
	CentosBased ImageBaseType = "centos7"
	AllImages   ImageBaseType = "all"
)

type tc struct {
	// The image version string (eg. '27' or '34')
	Version string
	// The base OS ('rhel7' or 'centos7')
	BaseOS ImageBaseType
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

// Internal OpenShift registry to fetch the RHEL7 images from
const InternalRegistryAddr = "ci.dev.openshift.redhat.com:5000"

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

func GetTestCaseForImages(base ImageBaseType) map[string][]tc {
	if base == AllImages {
		result := GetTestCaseForImages(RHELBased)
		for n, t := range GetTestCaseForImages(CentosBased) {
			result[n] = append(result[n], t...)
		}
		return result
	}
	result := make(map[string][]tc)
	for name, variants := range s2iImages {
		switch base {
		case RHELBased:
			for i := range variants {
				variants[i].BaseOS = RHELBased
				resolveDockerImageReference(name, &variants[i])
				result[name] = append(result[name], variants[i])
			}
		case CentosBased:
			for i := range variants {
				variants[i].BaseOS = CentosBased
				resolveDockerImageReference(name, &variants[i])
				result[name] = append(result[name], variants[i])

			}
		}
	}
	return result
}

// resolveDockerImageReferences resolves the pull specs for all images
func resolveDockerImageReference(name string, t *tc) {
	if len(t.Repository) == 0 {
		t.Repository = "openshift"
	}
	t.DockerImageReference = fmt.Sprintf("%s/%s-%s-%s", t.Repository, name, t.Version, t.BaseOS)
	if t.BaseOS == RHELBased {
		t.DockerImageReference = fmt.Sprintf("%s/%s", InternalRegistryAddr, t.DockerImageReference)
	}
}
