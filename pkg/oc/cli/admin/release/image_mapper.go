package release

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	imageapi "github.com/openshift/api/image/v1"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
)

type Payload struct {
	path string

	references *imageapi.ImageStream
}

func NewPayload(path string) *Payload {
	return &Payload{path: path}
}

func (p *Payload) Path() string {
	return p.path
}

// Rewrite updates the image stream to point to the locations described by the provided function.
// If a new ID appears in the returned reference, it will be used instead of the existing digest.
// All references in manifest files will be updated and then the image stream will be written to
// the correct location with any updated metadata.
func (p *Payload) Rewrite(allowTags bool, fn func(component string) imagereference.DockerImageReference) (map[string]string, error) {
	is, err := p.References()
	if err != nil {
		return nil, err
	}

	replacements := make(map[string]string)
	for i := range is.Spec.Tags {
		tag := &is.Spec.Tags[i]
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		oldImage := tag.From.Name
		oldRef, err := imagereference.Parse(oldImage)
		if err != nil {
			return nil, fmt.Errorf("unable to parse image reference for tag %q from payload: %v", tag.Name, err)
		}
		if len(oldRef.Tag) > 0 || len(oldRef.ID) == 0 {
			if !allowTags {
				return nil, fmt.Errorf("image reference tag %q in payload does not point to an image digest - unable to rewrite payload", tag.Name)
			}
		}
		ref := fn(tag.Name)
		if !allowTags {
			if len(ref.ID) == 0 {
				ref.Tag = ""
				ref.ID = oldRef.ID
			}
		}
		newImage := ref.Exact()
		replacements[oldImage] = newImage
		tag.From.Name = newImage
	}

	if glog.V(5) {
		for k, v := range replacements {
			glog.Infof("Mapping %s -> %s", k, v)
		}
	}
	mapper, err := NewExactMapper(replacements)
	if err != nil {
		return nil, err
	}

	files, err := ioutil.ReadDir(p.path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Base(file.Name()) == "image-references" {
			continue
		}
		path := filepath.Join(p.path, file.Name())
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out, err := mapper(data)
		if err != nil {
			return nil, fmt.Errorf("unable to rewrite the contents of %s: %v", path, err)
		}
		if bytes.Equal(data, out) {
			continue
		}
		glog.V(6).Infof("Rewrote\n%s\n\nto\n\n%s\n", string(data), string(out))
		if err := ioutil.WriteFile(path, out, file.Mode()); err != nil {
			return nil, err
		}
	}

	return replacements, nil
}

func (p *Payload) References() (*imageapi.ImageStream, error) {
	if p.references != nil {
		return p.references, nil
	}
	is, err := parseImageStream(filepath.Join(p.path, "image-references"))
	if err != nil {
		return nil, err
	}
	p.references = is
	return is, nil
}

func parseImageStream(path string) (*imageapi.ImageStream, error) {
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("unable to read release image info from release contents: %v", err)
	}
	return readReleaseImageReferences(data)
}

func readReleaseImageReferences(data []byte) (*imageapi.ImageStream, error) {
	is := &imageapi.ImageStream{}
	if err := yaml.Unmarshal(data, &is); err != nil {
		return nil, fmt.Errorf("unable to load release image-references: %v", err)
	}
	if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
		return nil, fmt.Errorf("unrecognized image-references in release payload")
	}
	return is, nil
}

type ManifestMapper func(data []byte) ([]byte, error)

func NewImageMapperFromImageStreamFile(path string, input *imageapi.ImageStream, allowMissingImages bool) (ManifestMapper, error) {
	is, err := parseImageStream(path)
	if err != nil {
		return nil, err
	}

	references := make(map[string]ImageReference)
	for _, tag := range is.Spec.Tags {
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		if len(tag.From.Name) == 0 {
			return nil, fmt.Errorf("Image file %q did not specify a valid target location for tag %q - no from.name for the tag", path, tag.Name)
		}
		ref := ImageReference{SourceRepository: tag.From.Name}
		for _, inputTag := range input.Spec.Tags {
			if inputTag.Name == tag.Name {
				ref.TargetPullSpec = inputTag.From.Name
				break
			}
		}
		if len(ref.TargetPullSpec) == 0 {
			if allowMissingImages {
				glog.V(2).Infof("Image file %q referenced an image %q that is not part of the input images, skipping", path, tag.From.Name)
				continue
			}
			return nil, fmt.Errorf("requested mapping for %q, but no input image could be located", tag.From.Name)
		}
		references[tag.Name] = ref
	}
	return NewImageMapper(references)
}

type ImageReference struct {
	SourceRepository string
	TargetPullSpec   string
}

func NopManifestMapper(data []byte) ([]byte, error) {
	return data, nil
}

// patternImageFormat attempts to match a docker pull spec by prefix (%s) and capture the
// prefix and either a tag or digest. It requires leading and trailing whitespace, quotes, or
// end of file.
const patternImageFormat = `([\s\"\']|^)(%s)(:[\w][\w.-]{0,127}|@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{2,})?([\s"']|$)`

func NewImageMapper(images map[string]ImageReference) (ManifestMapper, error) {
	repositories := make([]string, 0, len(images))
	bySource := make(map[string]string)
	for name, ref := range images {
		if len(ref.SourceRepository) == 0 {
			return nil, fmt.Errorf("an empty source repository is not allowed for name %q", name)
		}
		if existing, ok := bySource[ref.SourceRepository]; ok {
			return nil, fmt.Errorf("the source repository %q was defined more than once (for %q and %q)", ref.SourceRepository, existing, name)
		}
		bySource[ref.SourceRepository] = name
		repositories = append(repositories, regexp.QuoteMeta(ref.SourceRepository))
	}
	if len(repositories) == 0 {
		glog.V(5).Infof("No images are mapped, will not replace any contents")
		return NopManifestMapper, nil
	}
	pattern := fmt.Sprintf(patternImageFormat, strings.Join(repositories, "|"))
	re := regexp.MustCompile(pattern)

	return func(data []byte) ([]byte, error) {
		out := re.ReplaceAllFunc(data, func(in []byte) []byte {
			parts := re.FindSubmatch(in)
			repository := string(parts[2])
			name, ok := bySource[repository]
			if !ok {
				glog.V(4).Infof("found potential image %q, but no matching definition", repository)
				return in
			}
			ref := images[name]

			suffix := parts[3]
			glog.V(2).Infof("found repository %q with locator %q in the input, switching to %q (from pattern %s)", string(repository), string(suffix), ref.TargetPullSpec, pattern)
			switch {
			case len(suffix) == 0:
				// we found a repository, but no tag or digest (implied latest), or we got an exact match
				return []byte(string(parts[1]) + ref.TargetPullSpec + string(parts[4]))
			case suffix[0] == '@':
				// we got a digest
				return []byte(string(parts[1]) + ref.TargetPullSpec + string(parts[4]))
			default:
				// TODO: we didn't get a digest, so we have to decide what to replace
				return []byte(string(parts[1]) + ref.TargetPullSpec + string(parts[4]))
			}
		})
		return out, nil
	}, nil
}

// exactImageFormat attempts to match a string on word boundaries
const exactImageFormat = `\b%s\b`

func NewExactMapper(mappings map[string]string) (ManifestMapper, error) {
	patterns := make(map[string]*regexp.Regexp)
	for from, to := range mappings {
		pattern := fmt.Sprintf(exactImageFormat, regexp.QuoteMeta(from))
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		patterns[to] = re
	}

	return func(data []byte) ([]byte, error) {
		for to, pattern := range patterns {
			data = pattern.ReplaceAll(data, []byte(to))
		}
		return data, nil
	}, nil
}

func ComponentReferencesForImageStream(is *imageapi.ImageStream) (func(string) imagereference.DockerImageReference, error) {
	components := make(map[string]imagereference.DockerImageReference)
	for _, tag := range is.Spec.Tags {
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		ref, err := imagereference.Parse(tag.From.Name)
		if err != nil {
			return nil, fmt.Errorf("reference for %q is invalid: %v", tag.Name, err)
		}
		components[tag.Name] = ref
	}
	return func(component string) imagereference.DockerImageReference {
		ref, ok := components[component]
		if !ok {
			panic(fmt.Errorf("unknown component %s", component))
		}
		return ref
	}, nil
}
