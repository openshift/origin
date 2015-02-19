package api

// Script defines script names used by STI process.
type Script string

const (
	// Assemble is the name of the script responsible for build process of the resulting image.
	Assemble Script = "assemble"
	// Run is the name of the script responsible for running the final application.
	Run Script = "run"
	// SaveArtifacts is the name of the script responsible for storing dependencies etc. between builds.
	SaveArtifacts Script = "save-artifacts"
	// Usage i the name of the script responsible for printing the builder image's short info.
	Usage Script = "usage"
)

// String returns name of the script.
func (s Script) String() string {
	return string(s)
}
