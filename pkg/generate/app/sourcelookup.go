package app

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
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
	location  string
	url       url.URL
	localDir  string
	remoteURL *url.URL

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
	if len(r.localDir) > 0 {
		return r.localDir, nil
	}
	switch {
	case r.url.Scheme == "file":
		r.localDir = r.url.Path
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
	}
	return r.localDir, nil
}

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
	remoteUrl, err := repo.RemoteURL()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot obtain remote URL for repository at %s", repo.location)
	}
	strategy := &BuildStrategyRef{
		IsDockerBuild: repo.IsDockerBuild(),
	}
	source := &SourceRef{
		URL:        remoteUrl,
		Ref:        remoteUrl.Fragment,
		ContextDir: "",
	}
	return strategy, source, nil
}
