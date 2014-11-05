package sti

// STIRequest contains essential fields for any request: a base image, source directory,
// and tag
type STIRequest struct {
	BaseImage           string
	DockerSocket        string
	Verbose             bool
	PreserveWorkingDir  bool
	Source              string
	Ref                 string
	Tag                 string
	Clean               bool
	RemovePreviousImage bool
	Environment         map[string]string
	CallbackUrl         string
	ScriptsUrl          string

	incremental bool
	workingDir  string
}

// STIResult includes a flag that indicates whether the build was successful
// and if an image was created, the image ID
type STIResult struct {
	Success    bool
	Messages   []string
	WorkingDir string
	ImageID    string
}
