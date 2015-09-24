package app

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/origin/pkg/generate/source"
)

var (
	argumentGit         = regexp.MustCompile("^(http://|https://|git@|git://).*(?:#([a-zA-Z0-9]*))?$")
	argumentGitProtocol = regexp.MustCompile("^(git@|git://)")
)

type Dockerfile interface {
	Dockerfile() (dockerfile.Dockerfile, error)
	Contents() string
}

func NewDockerfileFromFile(path string) (Dockerfile, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("Dockerfile %q is empty", path)
	}
	df := dockerfileContents{
		contents: string(data),
	}
	df.parsed, df.err = dockerfile.NewParser().Parse(bytes.NewBuffer(data))
	return df, nil
}

func NewDockerfile(contents string) (Dockerfile, error) {
	if len(contents) == 0 {
		return nil, fmt.Errorf("Dockerfile is empty")
	}
	df := dockerfileContents{
		contents: contents,
	}
	if parsed, err := dockerfile.NewParser().Parse(bytes.NewBufferString(contents)); err == nil {
		df.parsed = parsed
	}
	return df, nil
}

type dockerfileContents struct {
	parsed   dockerfile.Dockerfile
	err      error
	contents string
}

func (d dockerfileContents) Contents() string {
	return d.contents
}

func (d dockerfileContents) Dockerfile() (dockerfile.Dockerfile, error) {
	return d.parsed, d.err
}

// IsPossibleSourceRepository checks whether the provided string is a source repository or not
func IsPossibleSourceRepository(s string) bool {
	return IsRemoteRepository(s) || isDirectory(s)
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
	info       *SourceRepositoryInfo

	usedBy           []ComponentReference
	buildWithDocker  bool
	ignoreRepository bool
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

// NewSourceRepositoryForDockerfile creates a source repository that is set up to use
// the contents of a Dockerfile as the input of the build.
func NewSourceRepositoryForDockerfile(contents string) (*SourceRepository, error) {
	dockerfile, err := NewDockerfile(contents)
	if err != nil {
		return nil, err
	}
	return &SourceRepository{
		buildWithDocker:  true,
		ignoreRepository: true,
		info: &SourceRepositoryInfo{
			Dockerfile: dockerfile,
		},
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

// Detect clones source locally if not already local and runs code detection
// with the given detector.
func (r *SourceRepository) Detect(d Detector) error {
	if r.info != nil {
		return nil
	}
	path, err := r.LocalPath()
	if err != nil {
		return err
	}
	r.info, err = d.Detect(path)
	if err != nil {
		return err
	}
	return nil
}

// SetInfo sets the source repository info. This is to facilitate certain tests.
func (r *SourceRepository) SetInfo(info *SourceRepositoryInfo) {
	r.info = info
}

// Info returns the source repository info generated on code detection
func (r *SourceRepository) Info() *SourceRepositoryInfo {
	return r.info
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
		localURL := r.url
		ref := localURL.Fragment
		localURL.Fragment = ""
		if err = gitRepo.Clone(r.localDir, localURL.String()); err != nil {
			return "", fmt.Errorf("cannot clone repository %s: %v", localURL.String(), err)
		}
		if len(ref) > 0 {
			if err = gitRepo.Checkout(r.localDir, ref); err != nil {
				return "", fmt.Errorf("cannot checkout ref %s of repository %s: %v", ref, localURL.String(), err)
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

// SourceRepositories is a list of SourceRepository objects
type SourceRepositories []*SourceRepository

func (rr SourceRepositories) String() string {
	repos := []string{}
	for _, r := range rr {
		repos = append(repos, r.String())
	}
	return strings.Join(repos, ",")
}

// NotUsed returns the list of SourceRepositories that are not used
func (rr SourceRepositories) NotUsed() SourceRepositories {
	notUsed := SourceRepositories{}
	for _, r := range rr {
		if !r.InUse() {
			notUsed = append(notUsed, r)
		}
	}
	return notUsed
}

// SourceRepositoryInfo contains info about a source repository
type SourceRepositoryInfo struct {
	Path       string
	Types      []SourceLanguageType
	Dockerfile Dockerfile
}

// Terms returns which languages the source repository was
// built with
func (info *SourceRepositoryInfo) Terms() []string {
	terms := []string{}
	for i := range info.Types {
		terms = append(terms, info.Types[i].Term())
	}
	return terms
}

// SourceLanguageType contains info about the type of the language
// a source repository is built in
type SourceLanguageType struct {
	Platform string
	Version  string
}

// Term returns a search term for the given source language type
// the term will be in the form of language:version
func (t *SourceLanguageType) Term() string {
	if len(t.Version) == 0 {
		return t.Platform
	}
	return fmt.Sprintf("%s:%s", t.Platform, t.Version)
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

// ErrNoLanguageDetected is the error returned when no language can be detected by all
// source code detectors.
var ErrNoLanguageDetected = fmt.Errorf("No language matched the source repository")

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
		dockerfile, err := NewDockerfileFromFile(path)
		if err != nil {
			return nil, err
		}
		info.Dockerfile = dockerfile
	}
	if info.Dockerfile == nil && len(info.Types) == 0 {
		return nil, ErrNoLanguageDetected
	}
	return info, nil
}

// StrategyAndSourceForRepository returns the build strategy and source code reference
// of the provided source repository
// TODO: user should be able to choose whether to download a remote source ref for
// more info
func StrategyAndSourceForRepository(repo *SourceRepository, image *ImageRef) (*BuildStrategyRef, *SourceRef, error) {
	if image == nil {
		return nil, nil, fmt.Errorf("an image ref is required to generate a strategy and sourceref")
	}

	strategy := &BuildStrategyRef{
		Base:          image,
		IsDockerBuild: repo.IsDockerBuild(),
	}
	var source *SourceRef
	switch {
	case repo.ignoreRepository && repo.Info() != nil && repo.Info().Dockerfile != nil:
		source = &SourceRef{
			DockerfileContents: repo.Info().Dockerfile.Contents(),
		}
	default:
		remoteURL, err := repo.RemoteURL()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot obtain remote URL for repository at %s", repo.location)
		}
		source = &SourceRef{
			URL:        remoteURL,
			Ref:        remoteURL.Fragment,
			ContextDir: repo.ContextDir(),
		}
	}
	return strategy, source, nil
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
