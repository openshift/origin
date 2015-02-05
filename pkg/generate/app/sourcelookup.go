package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/source"
)

var (
	argumentGit         = regexp.MustCompile("^(http://|https://|git@|git://).*\\.git(?:#([a-zA-Z0-9]*))?$")
	argumentGitProtocol = regexp.MustCompile("^(git@|git://)")
	argumentPath        = regexp.MustCompile("^\\.|^\\/[^/]+")
)

func IsPossibleSourceRepository(s string) bool {
	return argumentGit.MatchString(s) || argumentGitProtocol.MatchString(s) || argumentPath.MatchString(s)
}

func IsRemoteRepository(s string) bool {
	return argumentGit.MatchString(s) || argumentGitProtocol.MatchString(s)
}

// SourceRepository represents an code repository that may be the target of a build.
type SourceRepository struct {
	location string
	url      url.URL

	usedBy          []ComponentReference
	buildWithDocker bool
}

// NewSourceRepository creates a reference to a local or remote source code repository from
// a URL or path.
func NewSourceRepository(s string) (*SourceRepository, error) {
	var location *url.URL
	switch {
	case strings.HasPrefix(s, "git@"):
		base := "git://" + strings.TrimPrefix(s, "git@")
		url, err := url.Parse(base)
		if err != nil {
			return nil, err
		}
		location = url

	default:
		uri, err := url.Parse(s)
		if err != nil {
			return nil, err
		}

		if uri.Scheme == "" {
			path := s
			ref := ""
			segments := strings.SplitN(path, "#", 2)
			if len(segments) == 2 {
				path, ref = segments[0], segments[1]
			}
			path, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			uri = &url.URL{
				Scheme:   "file",
				Path:     path,
				Fragment: ref,
			}
		}

		location = uri
	}
	return &SourceRepository{
		location: s,
		url:      *location,
	}, nil
}

func (r *SourceRepository) UsedBy(ref ComponentReference) {
	r.usedBy = append(r.usedBy, ref)
}

func (r *SourceRepository) Remote() bool {
	return r.url.Scheme != "file"
}

func (r *SourceRepository) InUse() bool {
	return len(r.usedBy) > 0
}

func (r *SourceRepository) BuildWithDocker() {
	r.buildWithDocker = true
}

func (r *SourceRepository) IsDockerBuild() bool {
	return r.buildWithDocker
}

func (r *SourceRepository) String() string {
	return r.location
}

func (r *SourceRepository) LocalPath() (string, error) {
	switch {
	case r.url.Scheme == "file":
		return r.url.Path, nil
		// TODO: implement other types
		// TODO: lazy cache (predictably?)
	default:

		return "", fmt.Errorf("reading local repositories is not implemented: %q", r.location)
	}
}

type SourceRepositoryInfo struct {
	Path       string
	Types      []SourceLanguageType
	Dockerfile dockerfile.Dockerfile
}

func (info *SourceRepositoryInfo) Terms() []string {
	terms := []string{}
	for i := range info.Types {
		terms = append(terms, info.Types[i].Platform)
	}
	return terms
}

type SourceLanguageType struct {
	Platform string
	Version  string
}

type Detector interface {
	Detect(dir string) (*SourceRepositoryInfo, error)
}

type SourceRepositoryEnumerator struct {
	Detectors source.Detectors
	Tester    dockerfile.Tester
}

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

func StrategyAndSourceForRepository(repo *SourceRepository) (*BuildStrategyRef, *SourceRef, error) {
	// TODO: replace with repository origin lookup, then in the future replace with auto push repository to server
	if !repo.Remote() {
		return nil, nil, fmt.Errorf("the repository %q can't be used, as the CLI does not yet support pushing a local repository from your filesystem to OpenShift", repo)
	}
	strategy := &BuildStrategyRef{
		IsDockerBuild: repo.IsDockerBuild(),
		DockerContext: "",
	}
	source := &SourceRef{
		URL: &repo.url,
		Ref: repo.url.Fragment,
	}
	return strategy, source, nil
}
