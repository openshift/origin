package app

import (
	"fmt"
	"net/url"

	"github.com/openshift/origin/pkg/generate/git"
)

// SourceRefGenerator generates new SourceRefs either from a URL or a Directory
//
// Generators for SourceRef
// - Git URL        -> SourceRef
// - Directory      -> SourceRef
type SourceRefGenerator struct {
	repository git.Repository
}

// NewSourceRefGenerator creates a new SourceRefGenerator
func NewSourceRefGenerator() *SourceRefGenerator {
	return &SourceRefGenerator{
		repository: git.NewRepository(),
	}
}

// FromGitURL creates a SourceRef from a Git URL.
// If the URL includes a hash, it is used for the SourceRef's branch
// reference. Otherwise, 'master' is assumed
func (g *SourceRefGenerator) FromGitURL(location, contextDir string) (*SourceRef, error) {
	url, err := url.Parse(location)
	if err != nil {
		return nil, err
	}

	ref := url.Fragment
	url.Fragment = ""
	if len(ref) == 0 {
		ref = "master"
	}
	return &SourceRef{URL: url, Ref: ref, ContextDir: contextDir}, nil
}

// FromDirectory creates a SourceRef from a directory that contains
// a git repository. The URL is obtained from the origin remote branch, and
// the reference is taken from the currently checked out branch.
func (g *SourceRefGenerator) FromDirectory(directory string) (*SourceRef, error) {
	// Make sure that this is a git directory
	gitRoot, err := g.repository.GetRootDir(directory)
	if err != nil {
		return nil, fmt.Errorf("could not obtain git repository root for %s, the directory may not be part of a valid source repository", directory)
	}

	// Get URL
	location, ok, err := g.repository.GetOriginURL(gitRoot)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no origin remote defined for the provided Git repository")
	}

	// Get Branch Ref
	ref := g.repository.GetRef(gitRoot)

	srcRef, err := g.FromGitURL(fmt.Sprintf("%s#%s", location, ref), directory)
	if err != nil {
		return nil, err
	}
	srcRef.Dir = gitRoot
	return srcRef, nil
}
