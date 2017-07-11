package app

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/golang/glog"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2igit "github.com/openshift/source-to-image/pkg/scm/git"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/generate"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/origin/pkg/generate/source"
)

type Dockerfile interface {
	AST() *parser.Node
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
	return NewDockerfile(string(data))
}

func NewDockerfile(contents string) (Dockerfile, error) {
	if len(contents) == 0 {
		return nil, errors.New("Dockerfile is empty")
	}
	node, err := parser.Parse(strings.NewReader(contents))
	if err != nil {
		return nil, err
	}
	return dockerfileContents{node, contents}, nil
}

type dockerfileContents struct {
	ast      *parser.Node
	contents string
}

func (d dockerfileContents) AST() *parser.Node {
	return d.ast
}

func (d dockerfileContents) Contents() string {
	return d.contents
}

// IsRemoteRepository checks whether the provided string is a remote repository or not
func IsRemoteRepository(s string) (bool, error) {
	url, err := s2igit.Parse(s)
	if err != nil {
		glog.V(5).Infof("%s is not a valid url: %v", s, err)
		return false, err
	}
	if url.IsLocal() {
		glog.V(5).Infof("%s is not a valid remote git clone spec", s)
		return false, nil
	}
	gitRepo := git.NewRepository()

	// try up to 3 times to reach the remote git repo
	for i := 0; i < 3; i++ {
		_, _, err = gitRepo.ListRemote(url.StringNoFragment())
		if err == nil {
			break
		}
	}
	if err != nil {
		glog.V(5).Infof("could not list git remotes for %s: %v", s, err)
		return false, err
	}
	glog.V(5).Infof("%s is a valid remote git repository", s)
	return true, nil
}

// SourceRepository represents a code repository that may be the target of a build.
type SourceRepository struct {
	location        string
	url             s2igit.URL
	localDir        string
	remoteURL       *s2igit.URL
	contextDir      string
	secrets         []buildapi.SecretBuildSource
	info            *SourceRepositoryInfo
	sourceImage     ComponentReference
	sourceImageFrom string
	sourceImageTo   string

	usedBy           []ComponentReference
	strategy         generate.Strategy
	ignoreRepository bool
	binary           bool

	forceAddDockerfile bool

	requiresAuth bool
}

// NewSourceRepository creates a reference to a local or remote source code repository from
// a URL or path.
func NewSourceRepository(s string, strategy generate.Strategy) (*SourceRepository, error) {
	location, err := s2igit.Parse(s)
	if err != nil {
		return nil, err
	}

	return &SourceRepository{
		location: s,
		url:      *location,
		strategy: strategy,
	}, nil
}

// NewSourceRepositoryWithDockerfile creates a reference to a local source code repository with
// the provided relative Dockerfile path (defaults to "Dockerfile").
func NewSourceRepositoryWithDockerfile(s, dockerfilePath string) (*SourceRepository, error) {
	r, err := NewSourceRepository(s, generate.StrategyDocker)
	if err != nil {
		return nil, err
	}
	if len(dockerfilePath) == 0 {
		dockerfilePath = "Dockerfile"
	}
	f, err := NewDockerfileFromFile(filepath.Join(s, dockerfilePath))
	if err != nil {
		return nil, err
	}
	if r.info == nil {
		r.info = &SourceRepositoryInfo{}
	}
	r.info.Dockerfile = f
	return r, nil
}

// NewSourceRepositoryForDockerfile creates a source repository that is set up to use
// the contents of a Dockerfile as the input of the build.
func NewSourceRepositoryForDockerfile(contents string) (*SourceRepository, error) {
	s := &SourceRepository{
		ignoreRepository: true,
		strategy:         generate.StrategyDocker,
	}
	err := s.AddDockerfile(contents)
	return s, err
}

// NewBinarySourceRepository creates a source repository that is configured for binary
// input.
func NewBinarySourceRepository(strategy generate.Strategy) *SourceRepository {
	return &SourceRepository{
		binary:           true,
		ignoreRepository: true,
		strategy:         strategy,
	}
}

// TODO: this doesn't really match the others - this should likely be a different type of
// object that is associated with a build or component.
func NewImageSourceRepository(compRef ComponentReference, from, to string) *SourceRepository {
	return &SourceRepository{
		sourceImage:      compRef,
		sourceImageFrom:  from,
		sourceImageTo:    to,
		ignoreRepository: true,
		location:         compRef.Input().From,
		strategy:         generate.StrategySource,
	}
}

// UsedBy sets up which component uses the source repository
func (r *SourceRepository) UsedBy(ref ComponentReference) {
	r.usedBy = append(r.usedBy, ref)
}

// Remote checks whether the source repository is remote
func (r *SourceRepository) Remote() bool {
	return !r.url.IsLocal()
}

// InUse checks if the source repository is in use
func (r *SourceRepository) InUse() bool {
	return len(r.usedBy) > 0
}

// SetStrategy sets the source repository strategy
func (r *SourceRepository) SetStrategy(strategy generate.Strategy) {
	r.strategy = strategy
}

// GetStrategy returns the source repository strategy
func (r *SourceRepository) GetStrategy() generate.Strategy {
	return r.strategy
}

func (r *SourceRepository) String() string {
	return r.location
}

// Detect clones source locally if not already local and runs code detection
// with the given detector.
func (r *SourceRepository) Detect(d Detector, dockerStrategy bool) error {
	if r.info != nil {
		return nil
	}
	path, err := r.LocalPath()
	if err != nil {
		return err
	}
	r.info, err = d.Detect(path, dockerStrategy)
	if err != nil {
		return err
	}
	if err = r.DetectAuth(); err != nil {
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
	if r.url.IsLocal() {
		r.localDir = filepath.Join(r.url.LocalPath(), r.contextDir)
	} else {
		gitRepo := git.NewRepository()
		var err error
		if r.localDir, err = ioutil.TempDir("", "gen"); err != nil {
			return "", err
		}
		r.localDir, err = CloneAndCheckoutSources(gitRepo, r.url.StringNoFragment(), r.url.URL.Fragment, r.localDir, r.contextDir)
		if err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(r.localDir); os.IsNotExist(err) {
		return "", fmt.Errorf("supplied context directory '%s' does not exist in '%s'", r.contextDir, r.url.String())
	}
	return r.localDir, nil
}

// DetectAuth returns an error if the source repository cannot be cloned
// without the current user's environment. The following changes are made to the
// environment:
// 1) The HOME directory is set to a temporary dir to avoid loading any settings in .gitconfig
// 2) The GIT_SSH variable is set to /dev/null so the regular SSH keys are not used
//    (changing the HOME directory is not enough).
// 3) GIT_CONFIG_NOSYSTEM prevents git from loading system-wide config
// 4) GIT_ASKPASS to prevent git from prompting for a user/password
func (r *SourceRepository) DetectAuth() error {
	url, ok, err := r.RemoteURL()
	if err != nil {
		return err
	}
	if !ok {
		return nil // No auth needed, we can't find a remote URL
	}
	tempHome, err := ioutil.TempDir("", "githome")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempHome)
	tempSrc, err := ioutil.TempDir("", "gen")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempSrc)
	env := []string{
		fmt.Sprintf("HOME=%s", tempHome),
		"GIT_SSH=/dev/null",
		"GIT_CONFIG_NOSYSTEM=true",
		"GIT_ASKPASS=true",
	}
	if runtime.GOOS == "windows" {
		env = append(env,
			fmt.Sprintf("ProgramData=%s", os.Getenv("ProgramData")),
			fmt.Sprintf("SystemRoot=%s", os.Getenv("SystemRoot")),
		)
	}
	gitRepo := git.NewRepositoryWithEnv(env)
	_, err = CloneAndCheckoutSources(gitRepo, url.StringNoFragment(), url.URL.Fragment, tempSrc, "")
	if err != nil {
		r.requiresAuth = true
	}
	return nil
}

// RemoteURL returns the remote URL of the source repository
func (r *SourceRepository) RemoteURL() (*s2igit.URL, bool, error) {
	if r.remoteURL != nil {
		return r.remoteURL, true, nil
	}
	if r.url.IsLocal() {
		gitRepo := git.NewRepository()
		remote, ok, err := gitRepo.GetOriginURL(r.url.LocalPath())
		if err != nil && err != git.ErrGitNotAvailable {
			return nil, false, err
		}
		if !ok {
			return nil, ok, nil
		}
		ref := gitRepo.GetRef(r.url.LocalPath())
		if len(ref) > 0 {
			remote = fmt.Sprintf("%s#%s", remote, ref)
		}

		if r.remoteURL, err = s2igit.Parse(remote); err != nil {
			return nil, false, err
		}
	} else {
		r.remoteURL = &r.url
	}
	return r.remoteURL, true, nil
}

// SetContextDir sets the context directory to use for the source repository
func (r *SourceRepository) SetContextDir(dir string) {
	r.contextDir = dir
}

// ContextDir returns the context directory of the source repository
func (r *SourceRepository) ContextDir() string {
	return r.contextDir
}

// Secrets returns the secrets
func (r *SourceRepository) Secrets() []buildapi.SecretBuildSource {
	return r.secrets
}

// SetSourceImage sets the source(input) image for a repository
func (r *SourceRepository) SetSourceImage(c ComponentReference) {
	r.sourceImage = c
}

// SetSourceImagePath sets the source/destination to use when copying from the SourceImage
func (r *SourceRepository) SetSourceImagePath(source, dest string) {
	r.sourceImageFrom = source
	r.sourceImageTo = dest
}

// AddDockerfile adds the Dockerfile contents to the SourceRepository and
// configure it to build with Docker strategy. Returns an error if the contents
// are invalid.
func (r *SourceRepository) AddDockerfile(contents string) error {
	dockerfile, err := NewDockerfile(contents)
	if err != nil {
		return err
	}
	if r.info == nil {
		r.info = &SourceRepositoryInfo{}
	}
	r.info.Dockerfile = dockerfile
	r.SetStrategy(generate.StrategyDocker)
	r.forceAddDockerfile = true
	return nil
}

// AddBuildSecrets adds the defined secrets into a build. The input format for
// the secrets is "<secretName>:<destinationDir>". The destinationDir is
// optional and when not specified the default is the current working directory.
func (r *SourceRepository) AddBuildSecrets(secrets []string) error {
	injections := s2iapi.VolumeList{}
	r.secrets = []buildapi.SecretBuildSource{}
	for _, in := range secrets {
		if err := injections.Set(in); err != nil {
			return err
		}
	}
	secretExists := func(name string) bool {
		for _, s := range r.secrets {
			if s.Secret.Name == name {
				return true
			}
		}
		return false
	}
	for _, in := range injections {
		if r.GetStrategy() == generate.StrategyDocker && filepath.IsAbs(in.Destination) {
			return fmt.Errorf("for the docker strategy, the secret destination directory %q must be a relative path", in.Destination)
		}
		if len(validation.ValidateSecretName(in.Source, false)) != 0 {
			return fmt.Errorf("the %q must be valid secret name", in.Source)
		}
		if secretExists(in.Source) {
			return fmt.Errorf("the %q secret can be used just once", in.Source)
		}
		r.secrets = append(r.secrets, buildapi.SecretBuildSource{
			Secret:         kapi.LocalObjectReference{Name: in.Source},
			DestinationDir: in.Destination,
		})
	}
	return nil
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
	Path        string
	Types       []SourceLanguageType
	Dockerfile  Dockerfile
	Jenkinsfile bool
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
	Detect(dir string, dockerStrategy bool) (*SourceRepositoryInfo, error)
}

// SourceRepositoryEnumerator implements the Detector interface
type SourceRepositoryEnumerator struct {
	Detectors         source.Detectors
	DockerfileTester  generate.Tester
	JenkinsfileTester generate.Tester
}

// Detect extracts source code information about the provided source repository
func (e SourceRepositoryEnumerator) Detect(dir string, noSourceDetection bool) (*SourceRepositoryInfo, error) {
	info := &SourceRepositoryInfo{
		Path: dir,
	}

	// no point in doing source-type detection if the requested build strategy
	// is docker or pipeline
	if !noSourceDetection {
		for _, d := range e.Detectors {
			if detected := d(dir); detected != nil {
				info.Types = append(info.Types, SourceLanguageType{
					Platform: detected.Platform,
					Version:  detected.Version,
				})
			}
		}
	}
	if path, ok, err := e.DockerfileTester.Has(dir); err == nil && ok {
		dockerfile, err := NewDockerfileFromFile(path)
		if err != nil {
			return nil, err
		}
		info.Dockerfile = dockerfile
	}
	if _, ok, err := e.JenkinsfileTester.Has(dir); err == nil && ok {
		info.Jenkinsfile = true
	}

	return info, nil
}

// StrategyAndSourceForRepository returns the build strategy and source code reference
// of the provided source repository
// TODO: user should be able to choose whether to download a remote source ref for
// more info
func StrategyAndSourceForRepository(repo *SourceRepository, image *ImageRef) (*BuildStrategyRef, *SourceRef, error) {
	strategy := &BuildStrategyRef{
		Base:     image,
		Strategy: repo.strategy,
	}
	source := &SourceRef{
		Binary:       repo.binary,
		Secrets:      repo.secrets,
		RequiresAuth: repo.requiresAuth,
	}

	if repo.sourceImage != nil {
		srcImageRef, err := InputImageFromMatch(repo.sourceImage.Input().ResolvedMatch)
		if err != nil {
			return nil, nil, err
		}
		source.SourceImage = srcImageRef
		source.ImageSourcePath = repo.sourceImageFrom
		source.ImageDestPath = repo.sourceImageTo
	}

	if (repo.ignoreRepository || repo.forceAddDockerfile) && repo.Info() != nil && repo.Info().Dockerfile != nil {
		source.DockerfileContents = repo.Info().Dockerfile.Contents()
	}
	if !repo.ignoreRepository {
		remoteURL, ok, err := repo.RemoteURL()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot obtain remote URL for repository at %s", repo.location)
		}
		if ok {
			source.URL = remoteURL
		} else {
			source.Binary = true
		}
		source.ContextDir = repo.ContextDir()
	}

	return strategy, source, nil
}

// CloneAndCheckoutSources clones the remote repository using either regular
// git clone operation or shallow git clone, based on the "ref" provided (you
// cannot shallow clone using the 'ref').
// This function will return the full path to the buildable sources, including
// the context directory if specified.
func CloneAndCheckoutSources(repo git.Repository, remote, ref, localDir, contextDir string) (string, error) {
	if len(ref) == 0 {
		glog.V(5).Infof("No source ref specified, using shallow git clone")
		if err := repo.CloneWithOptions(localDir, remote, git.Shallow, "--recursive"); err != nil {
			return "", fmt.Errorf("shallow cloning repository %q to %q failed: %v", remote, localDir, err)
		}
	} else {
		glog.V(5).Infof("Requested ref %q, performing full git clone and git checkout", ref)
		if err := repo.Clone(localDir, remote); err != nil {
			return "", fmt.Errorf("cloning repository %q to %q failed: %v", remote, localDir, err)
		}
	}
	if len(ref) > 0 {
		if err := repo.Checkout(localDir, ref); err != nil {
			err = repo.PotentialPRRetryAsFetch(localDir, remote, ref, err)
			if err != nil {
				return "", fmt.Errorf("unable to checkout ref %q in %q repository: %v", ref, remote, err)
			}
		}
	}
	if len(contextDir) > 0 {
		glog.V(5).Infof("Using context directory %q. The full source path is %q", contextDir, filepath.Join(localDir, contextDir))
	}
	return filepath.Join(localDir, contextDir), nil
}
