package mirror

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/docker/distribution/registry/client/auth"
	digest "github.com/opencontainers/go-digest"

	"github.com/openshift/origin/pkg/image/apis/image/reference"
)

// ErrAlreadyExists may be returned by the blob Create function to indicate that the blob already exists.
var ErrAlreadyExists = fmt.Errorf("blob already exists in the target location")

type Mapping struct {
	Source      reference.DockerImageReference
	Destination reference.DockerImageReference
	Type        DestinationType
}

func parseSource(ref string) (reference.DockerImageReference, error) {
	src, err := reference.Parse(ref)
	if err != nil {
		return src, fmt.Errorf("%q is not a valid image reference: %v", ref, err)
	}
	if len(src.Tag) == 0 && len(src.ID) == 0 {
		return src, fmt.Errorf("you must specify a tag or digest for SRC")
	}
	return src, nil
}

func parseDestination(ref string) (reference.DockerImageReference, DestinationType, error) {
	dstType := DestinationRegistry
	switch {
	case strings.HasPrefix(ref, "s3://"):
		dstType = DestinationS3
		ref = strings.TrimPrefix(ref, "s3://")
	}
	dst, err := reference.Parse(ref)
	if err != nil {
		return dst, dstType, fmt.Errorf("%q is not a valid image reference: %v", ref, err)
	}
	if len(dst.ID) != 0 {
		return dst, dstType, fmt.Errorf("you must specify a tag for DST or leave it blank to only push by digest")
	}
	return dst, dstType, nil
}

func parseArgs(args []string, overlap map[string]string) ([]Mapping, error) {
	var remainingArgs []string
	var mappings []Mapping
	for _, s := range args {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			remainingArgs = append(remainingArgs, s)
			continue
		}
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return nil, fmt.Errorf("all arguments must be valid SRC=DST mappings")
		}
		src, err := parseSource(parts[0])
		if err != nil {
			return nil, err
		}
		dst, dstType, err := parseDestination(parts[1])
		if err != nil {
			return nil, err
		}
		if _, ok := overlap[dst.String()]; ok {
			return nil, fmt.Errorf("each destination tag may only be specified once: %s", dst.String())
		}
		overlap[dst.String()] = src.String()

		mappings = append(mappings, Mapping{Source: src, Destination: dst, Type: dstType})
	}

	switch {
	case len(remainingArgs) > 1 && len(mappings) == 0:
		src, err := parseSource(remainingArgs[0])
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(remainingArgs); i++ {
			if len(remainingArgs[i]) == 0 {
				continue
			}
			dst, dstType, err := parseDestination(remainingArgs[i])
			if err != nil {
				return nil, err
			}
			if _, ok := overlap[dst.String()]; ok {
				return nil, fmt.Errorf("each destination tag may only be specified once: %s", dst.String())
			}
			overlap[dst.String()] = src.String()
			mappings = append(mappings, Mapping{Source: src, Destination: dst, Type: dstType})
		}
	case len(remainingArgs) == 1 && len(mappings) == 0:
		return nil, fmt.Errorf("all arguments must be valid SRC=DST mappings, or you must specify one SRC argument and one or more DST arguments")
	}
	return mappings, nil
}

func parseFile(filename string, overlap map[string]string) ([]Mapping, error) {
	var fileMappings []Mapping
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	lineNumber := 0
	for s.Scan() {
		line := s.Text()
		lineNumber++

		// remove comments and whitespace
		if i := strings.Index(line, "#"); i != -1 {
			line = line[0:i]
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		args := strings.Split(line, " ")
		mappings, err := parseArgs(args, overlap)
		if err != nil {
			return nil, fmt.Errorf("file %s, line %d: %v", filename, lineNumber, err)
		}
		fileMappings = append(fileMappings, mappings...)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return fileMappings, nil
}

type key struct {
	registry   string
	repository string
}

type DestinationType string

var (
	DestinationRegistry DestinationType = "docker"
	DestinationS3       DestinationType = "s3"
)

type destination struct {
	t    DestinationType
	ref  reference.DockerImageReference
	tags []string
}

type pushTargets map[key]destination

type destinations struct {
	ref reference.DockerImageReference

	lock    sync.Mutex
	tags    map[string]pushTargets
	digests map[string]pushTargets
}

func (d *destinations) mergeIntoDigests(srcDigest digest.Digest, target pushTargets) {
	d.lock.Lock()
	defer d.lock.Unlock()
	srcKey := srcDigest.String()
	current, ok := d.digests[srcKey]
	if !ok {
		d.digests[srcKey] = target
		return
	}
	for repo, dst := range target {
		existing, ok := current[repo]
		if !ok {
			current[repo] = dst
			continue
		}
		existing.tags = append(existing.tags, dst.tags...)
	}
}

type targetTree map[key]*destinations

func buildTargetTree(mappings []Mapping) targetTree {
	tree := make(targetTree)
	for _, m := range mappings {
		srcKey := key{registry: m.Source.Registry, repository: m.Source.RepositoryName()}
		dstKey := key{registry: m.Destination.Registry, repository: m.Destination.RepositoryName()}

		src, ok := tree[srcKey]
		if !ok {
			src = &destinations{}
			src.ref = m.Source.AsRepository()
			src.digests = make(map[string]pushTargets)
			src.tags = make(map[string]pushTargets)
			tree[srcKey] = src
		}

		var current pushTargets
		if tag := m.Source.Tag; len(tag) != 0 {
			current = src.tags[tag]
			if current == nil {
				current = make(pushTargets)
				src.tags[tag] = current
			}
		} else {
			current = src.digests[m.Source.ID]
			if current == nil {
				current = make(pushTargets)
				src.digests[m.Source.ID] = current
			}
		}

		dst, ok := current[dstKey]
		if !ok {
			dst.ref = m.Destination.AsRepository()
			dst.t = m.Type
		}
		if len(m.Destination.Tag) > 0 {
			dst.tags = append(dst.tags, m.Destination.Tag)
		}
		current[dstKey] = dst
	}
	return tree
}

func addDockerRegistryScopes(scopes map[string]map[string]bool, targets map[string]pushTargets, srcKey key) {
	for _, target := range targets {
		for dstKey, t := range target {
			m := scopes[dstKey.registry]
			if m == nil {
				m = make(map[string]bool)
				scopes[dstKey.registry] = m
			}
			m[dstKey.repository] = true
			if t.t != DestinationRegistry || dstKey.registry != srcKey.registry || dstKey.repository == srcKey.repository {
				continue
			}
			m = scopes[srcKey.registry]
			if m == nil {
				m = make(map[string]bool)
				scopes[srcKey.registry] = m
			}
			if _, ok := m[srcKey.repository]; !ok {
				m[srcKey.repository] = false
			}
		}
	}
}

func calculateDockerRegistryScopes(tree targetTree) map[string][]auth.Scope {
	scopes := make(map[string]map[string]bool)
	for srcKey, dst := range tree {
		addDockerRegistryScopes(scopes, dst.tags, srcKey)
		addDockerRegistryScopes(scopes, dst.digests, srcKey)
	}
	uniqueScopes := make(map[string][]auth.Scope)
	for registry, repos := range scopes {
		var repoScopes []auth.Scope
		for name, push := range repos {
			if push {
				repoScopes = append(repoScopes, auth.RepositoryScope{Repository: name, Actions: []string{"pull", "push"}})
			} else {
				repoScopes = append(repoScopes, auth.RepositoryScope{Repository: name, Actions: []string{"pull"}})
			}
		}
		uniqueScopes[registry] = repoScopes
	}
	return uniqueScopes
}
