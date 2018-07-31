package release

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/golang/glog"
	imageapi "github.com/openshift/api/image/v1"
	imagescheme "github.com/openshift/client-go/image/clientset/versioned/scheme"
)

type ManifestMapper func(data []byte) ([]byte, error)

// patternImageFormat attempts to match a docker pull spec by prefix (%s) and capture the
// prefix and either a tag or digest. It requires leading and trailing whitespace, quotes, or
// end of file.
const patternImageFormat = `([\s\"\']|^)(%s)(:[a-zA-Z\d][\w\-_]*[a-zA-Z\d]|@\w+:\w+)?([\s"']|$)`

func NewImageMapperFromImageStreamFile(path string, input *imageapi.ImageStream, allowMissingImages bool) (ManifestMapper, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	is := &imageapi.ImageStream{}
	if _, _, err := imagescheme.Codecs.UniversalDeserializer().Decode(data, nil, is); err != nil {
		return nil, err
	}
	if is.TypeMeta.Kind != "ImageStream" || is.TypeMeta.APIVersion != "image.openshift.io/v1" {
		return nil, fmt.Errorf("%q was not a valid image stream - kind and apiVersion must be ImageStream and image.openshift.io/v1", path)
	}
	references := make(map[string]ImageReference)
	for _, tag := range is.Spec.Tags {
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
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

func NewImageMapper(images map[string]ImageReference) (ManifestMapper, error) {
	repositories := make([]string, 0, len(images))
	bySource := make(map[string]string)
	for name, ref := range images {
		if existing, ok := bySource[ref.SourceRepository]; ok {
			return nil, fmt.Errorf("the source repository %q was defined more than once (for %q and %q)", ref.SourceRepository, existing, name)
		}
		bySource[ref.SourceRepository] = name
		repositories = append(repositories, regexp.QuoteMeta(ref.SourceRepository))
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
			glog.V(2).Infof("found repository %q with locator %q in the input, switching to %q", string(repository), string(suffix), ref.TargetPullSpec)
			switch {
			case len(suffix) == 0:
				// TODO: we found a repository, but no tag or digest - leave it alone for now
				return in
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
