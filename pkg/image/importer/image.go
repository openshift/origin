package importer

import (
	"encoding/json"
	"fmt"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	godigest "github.com/opencontainers/go-digest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/api/image/dockerpre012"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	dockerregistry "github.com/openshift/origin/pkg/image/importer/dockerv1client"
)

func schema1ToImage(manifest *schema1.SignedManifest, d godigest.Digest) (*imageapi.Image, error) {
	if len(manifest.History) == 0 {
		return nil, fmt.Errorf("image has no v1Compatibility history and cannot be used")
	}
	dockerImage, err := unmarshalDockerImage([]byte(manifest.History[0].V1Compatibility))
	if err != nil {
		return nil, err
	}
	mediatype, payload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	if len(d) > 0 {
		dockerImage.ID = d.String()
	} else {
		dockerImage.ID = godigest.FromBytes(manifest.Canonical).String()
	}
	image := &imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: dockerImage.ID,
		},
		DockerImageMetadata:          *dockerImage,
		DockerImageManifest:          string(payload),
		DockerImageManifestMediaType: mediatype,
		DockerImageMetadataVersion:   "1.0",
	}

	return image, nil
}

func schema2ToImage(manifest *schema2.DeserializedManifest, imageConfig []byte, d godigest.Digest) (*imageapi.Image, error) {
	mediatype, payload, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	dockerImage, err := unmarshalDockerImage(imageConfig)
	if err != nil {
		return nil, err
	}
	if len(d) > 0 {
		dockerImage.ID = d.String()
	} else {
		dockerImage.ID = godigest.FromBytes(payload).String()
	}

	image := &imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: dockerImage.ID,
		},
		DockerImageMetadata:          *dockerImage,
		DockerImageManifest:          string(payload),
		DockerImageConfig:            string(imageConfig),
		DockerImageManifestMediaType: mediatype,
		DockerImageMetadataVersion:   "1.0",
	}

	return image, nil
}

func schema0ToImage(dockerImage *dockerregistry.Image) (*imageapi.Image, error) {
	var baseImage imageapi.DockerImage
	if err := legacyscheme.Scheme.Convert(&dockerImage.Image, &baseImage, nil); err != nil {
		return nil, fmt.Errorf("could not convert image: %#v", err)
	}

	image := &imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: dockerImage.Image.ID,
		},
		DockerImageMetadata:        baseImage,
		DockerImageMetadataVersion: "1.0",
	}

	return image, nil
}

func unmarshalDockerImage(body []byte) (*imageapi.DockerImage, error) {
	var image dockerpre012.DockerImage
	if err := json.Unmarshal(body, &image); err != nil {
		return nil, err
	}
	dockerImage := &imageapi.DockerImage{}
	if err := legacyscheme.Scheme.Convert(&image, dockerImage, nil); err != nil {
		return nil, err
	}
	return dockerImage, nil
}

func isDockerError(err error, code errcode.ErrorCode) bool {
	switch t := err.(type) {
	case errcode.Errors:
		for _, err := range t {
			if isDockerError(err, code) {
				return true
			}
		}
	case errcode.ErrorCode:
		if code == t {
			return true
		}
	case errcode.Error:
		if t.ErrorCode() == code {
			return true
		}
	}
	return false
}
