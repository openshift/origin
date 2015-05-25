package app

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/origin/pkg/generate/source"
)

var (
	argumentGit         = regexp.MustCompile("^(http://|https://|git@|git://).*(?:#([a-zA-Z0-9]*))?$")
	argumentGitProtocol = regexp.MustCompile("^(git@|git://)")
	argumentPath        = regexp.MustCompile("^\\.|^\\/[^/]+")
)

// IsPossibleSourceRepository checks whether the provided string is a source repository or not
func IsPossibleSourceRepository(s string) bool {
	return IsRemoteRepository(s) || argumentPath.MatchString(s)
}

// IsRemoteRepository checks whether the provided string is a remote repository or not
func IsRemoteRepository(s string) bool {
	return argumentGit.MatchString(s) || argumentGitProtocol.MatchString(s)
}

// SourceRepository represents a code repository that may be the target of a build.
type SourceRepository struct {
	location   string
	url        url.URL
	localDir   string
	remoteURL  *url.URL
	contextDir string

	usedBy          []ComponentReference
	buildWithDocker bool
}

// NewSourceRepository creates a reference to a local or remote source code repository from
// a URL or path.
func NewSourceRepository(s string) (*SourceRepository, error) {
	location, err := git.ParseRepository(s)
	if err != nil {
		return nil, err
	}

	return &SourceRepository{
		location: s,
		url:      *location,
	}, nil
}

// UsedBy sets up which component uses the source repository
func (r *SourceRepository) UsedBy(ref ComponentReference) {
	r.usedBy = append(r.usedBy, ref)
}

// Remote checks whether the source repository is remote
func (r *SourceRepository) Remote() bool {
	return r.url.Scheme != "file"
}

// InUse checks if the source repository is in use
func (r *SourceRepository) InUse() bool {
	return len(r.usedBy) > 0
}

// BuildWithDocker specifies that the source repository was built with Docker
func (r *SourceRepository) BuildWithDocker() {
	r.buildWithDocker = true
}

// IsDockerBuild checks if the source repository was built with Docker
func (r *SourceRepository) IsDockerBuild() bool {
	return r.buildWithDocker
}

func (r *SourceRepository) String() string {
	return r.location
}

// LocalPath returns the local path of the source repository
func (r *SourceRepository) LocalPath() (string, error) {
	if len(r.localDir) > 0 {
		return r.localDir, nil
	}
	switch {
	case r.url.Scheme == "file":
		r.localDir = filepath.Join(r.url.Path, r.contextDir)
	default:
		gitRepo := git.NewRepository()
		var err error
		if r.localDir, err = ioutil.TempDir("", "gen"); err != nil {
			return "", err
		}
		localUrl := r.url
		ref := localUrl.Fragment
		localUrl.Fragment = ""
		if err = gitRepo.Clone(r.localDir, localUrl.String()); err != nil {
			return "", fmt.Errorf("cannot clone repository %s: %v", localUrl.String(), err)
		}
		if len(ref) > 0 {
			if err = gitRepo.Checkout(r.localDir, ref); err != nil {
				return "", fmt.Errorf("cannot checkout ref %s of repository %s: %v", ref, localUrl.String(), err)
			}
		}
		r.localDir = filepath.Join(r.localDir, r.contextDir)
	}
	return r.localDir, nil
}

// RemoteURL returns the remote URL of the source repository
func (r *SourceRepository) RemoteURL() (*url.URL, error) {
	if r.remoteURL != nil {
		return r.remoteURL, nil
	}
	switch r.url.Scheme {
	case "file":
		gitRepo := git.NewRepository()
		remote, _, err := gitRepo.GetOriginURL(r.url.Path)
		if err != nil {
			return nil, err
		}
		ref := gitRepo.GetRef(r.url.Path)
		if len(ref) > 0 {
			remote = fmt.Sprintf("%s#%s", remote, ref)
		}
		if r.remoteURL, err = url.Parse(remote); err != nil {
			return nil, err
		}
	default:
		r.remoteURL = &r.url
	}
	return r.remoteURL, nil
}

// SetContextDir sets the context directory to use for the source repository
func (r *SourceRepository) SetContextDir(dir string) {
	r.contextDir = dir
}

// ContextDir returns the context directory of the source repository
func (r *SourceRepository) ContextDir() string {
	return r.contextDir
}

// SourceRepositoryInfo contains info about a source repository
type SourceRepositoryInfo struct {
	Path       string
	Types      []SourceLanguageType
	Dockerfile dockerfile.Dockerfile
}

// Terms returns which languages the source repository was
// built with
func (info *SourceRepositoryInfo) Terms() []string {
	terms := []string{}
	for i := range info.Types {
		terms = append(terms, info.Types[i].Platform)
	}
	return terms
}

// SourceLanguageType contains info about the type of the language
// a source repository is built in
type SourceLanguageType struct {
	Platform string
	Version  string
}

// Detector is an interface for detecting information about a
// source repository
type Detector interface {
	Detect(dir string) (*SourceRepositoryInfo, error)
}

// SourceRepositoryEnumerator implements the Detector interface
type SourceRepositoryEnumerator struct {
	Detectors source.Detectors
	Tester    dockerfile.Tester
}

// Detect extracts source code information about the provided source repository
func (e SourceRepositoryEnumerator) Detect(dir string) (*SourceRepositoryInfo, error) {
	info := &SourceRepositoryInfo{
		Path: dir,
	}
	for _, d := range e.Detectors {
		if detected, ok := d(dir); ok {
			info.Types = append(info.Types, SourceLanguageType{
				Platform: detected.Platform,
				Version:  detected.Version,
			})
		}
	}
	if path, ok, err := e.Tester.Has(dir); err == nil && ok {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		dockerfile, err := dockerfile.NewParser().Parse(file)
		if err != nil {
			return nil, err
		}
		info.Dockerfile = dockerfile
	}
	return info, nil
}

// StrategyAndSourceForRepository returns the build strategy and source code reference
// of the provided source repository
// TODO: user should be able to choose whether to download a remote source ref for
// more info
func StrategyAndSourceForRepository(repo *SourceRepository, image *ImageRef) (*BuildStrategyRef, *SourceRef, error) {
	if image != nil {
		remoteUrl, err := repo.RemoteURL()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot obtain remote URL for repository at %s", repo.location)
		}
		strategy := &BuildStrategyRef{
			Base:          image,
			IsDockerBuild: repo.IsDockerBuild(),
		}
		source := &SourceRef{
			URL:        remoteUrl,
			Ref:        remoteUrl.Fragment,
			ContextDir: repo.ContextDir(),
		}
		return strategy, source, nil
	}

	srcRef, err := NewSourceRefGenerator().FromGitURL(repo.location, repo.ContextDir())
	if err != nil {
		return nil, nil, fmt.Errorf("cannot obtain remote URL for repository at %s: %v", repo.location, err)
	}
	strategy, err := NewBuildStrategyRefGenerator(source.DefaultDetectors).FromSourceRef(srcRef)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot generate strategy for repository at %s: %v", repo.location, err)
	}
	return strategy, srcRef, nil
}

// MockSourceRepositories is a set of mocked source repositories
// used for testing
func MockSourceRepositories() []*SourceRepository {
	return []*SourceRepository{
		{
			location: "some/location.git",
		},
		{
			location: "https://github.com/openshift/ruby-hello-world.git",
			url: url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/openshift/ruby-hello-world.git",
			},
		},
		{
			location: "another/location.git",
		},
	}
}
