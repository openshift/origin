package app

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/errors"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	imagehelpers "github.com/openshift/oc/pkg/helpers/image"
)

var s2iEnvironmentNames = []string{"STI_LOCATION", "STI_SCRIPTS_URL", "STI_BUILDER"}

const s2iScriptsLabel = "io.openshift.s2i.scripts-url"

// IsBuilderImage checks whether the provided container image is
// a builder image or not
func IsBuilderImage(image *dockerv10.DockerImage) bool {
	if image == nil || image.Config == nil {
		return false
	}
	// Has the scripts annotation
	if _, ok := image.Config.Labels[s2iScriptsLabel]; ok {
		return true
	}
	// Has the legacy environment variables
	for _, env := range image.Config.Env {
		for _, name := range s2iEnvironmentNames {
			if strings.HasPrefix(env, name+"=") {
				return true
			}
		}
	}
	return false
}

// IsBuilderStreamTag checks whether the provided image stream tag is
// a builder image or not
func IsBuilderStreamTag(stream *imagev1.ImageStream, tag string) bool {
	if stream == nil {
		return false
	}

	// Has the tag annotation
	t, hasTag := imageutil.SpecHasTag(stream, tag)
	if !hasTag {
		return false
	}

	return imagehelpers.HasAnnotationTag(&t, "builder")
}

func IsBuilderMatch(match *ComponentMatch) bool {
	if match.DockerImage != nil && IsBuilderImage(match.DockerImage) {
		return true
	}
	if match.ImageStream != nil && IsBuilderStreamTag(match.ImageStream, match.ImageTag) {
		return true
	}
	return false
}

// isGeneratorJobImage checks whether the provided container image is
// installable
func isGeneratorJobImage(image *dockerv10.DockerImage) bool {
	if image == nil || image.Config == nil {
		return false
	}
	// Has the job annotation
	if image.Config.Labels[labelGenerateJob] == "true" {
		return true
	}
	return false
}

// isGeneratorJobImageStreamTag checks whether the provided image stream tag is
// installable
func isGeneratorJobImageStreamTag(stream *imagev1.ImageStream, tag string) bool {
	if stream == nil {
		return false
	}
	// Has the job annotation
	t, hasTag := imageutil.SpecHasTag(stream, tag)
	if !hasTag {
		return false
	}

	return t.Annotations[labelGenerateJob] == "true"
}

func parseGenerateTokenAs(value string) (*TokenInput, error) {
	parts := strings.SplitN(value, ":", 2)
	switch parts[0] {
	case "env":
		if len(parts) != 2 {
			return nil, fmt.Errorf("label %s=%s; expected 'env:<NAME>' or not set", labelGenerateTokenAs, value)
		}
		name := strings.TrimSpace(parts[1])
		if len(name) == 0 {
			return nil, fmt.Errorf("label %s=%s; expected 'env:<NAME>' but name was empty", labelGenerateTokenAs, value)
		}
		return &TokenInput{Env: &name}, nil
	case "file":
		if len(parts) != 2 {
			return nil, fmt.Errorf("label %s=%s; expected 'file:<PATH>' or not set", labelGenerateTokenAs, value)
		}
		name := strings.TrimSpace(parts[1])
		if len(name) == 0 {
			return nil, fmt.Errorf("label %s=%s; expected 'file:<PATH>' but path was empty", labelGenerateTokenAs, value)
		}
		return &TokenInput{File: &name}, nil
	case "serviceaccount":
		return &TokenInput{ServiceAccount: true}, nil
	default:
		return nil, fmt.Errorf("unrecognized value for label %s=%s; expected 'env:<NAME>', 'file:<PATH>', or 'serviceaccount'", labelGenerateTokenAs, value)
	}
}

const (
	labelGenerateJob     = "io.openshift.generate.job"
	labelGenerateTokenAs = "io.openshift.generate.token.as"
)

type TokenInput struct {
	Env            *string
	File           *string
	ServiceAccount bool
}

type GeneratorInput struct {
	Job   bool
	Token *TokenInput
}

// GeneratorInputFromMatch attempts to extract a GeneratorInput struct from the provided match.
// If errors occur, a partial GeneratorInput may be returned along an error.
func GeneratorInputFromMatch(match *ComponentMatch) (GeneratorInput, error) {
	input := GeneratorInput{}
	errs := []error{}

	if match.DockerImage != nil && match.DockerImage.Config != nil {
		input.Job = isGeneratorJobImage(match.DockerImage)

		if value, ok := match.DockerImage.Config.Labels[labelGenerateTokenAs]; ok {
			if token, err := parseGenerateTokenAs(value); err != nil {
				errs = append(errs, err)
			} else {
				input.Token = token
			}
		}
	}

	if match.ImageStream != nil {
		input.Job = isGeneratorJobImageStreamTag(match.ImageStream, match.ImageTag)

		if t, hasTag := imageutil.SpecHasTag(match.ImageStream, match.ImageTag); hasTag {
			if value, ok := t.Annotations[labelGenerateTokenAs]; ok {
				if token, err := parseGenerateTokenAs(value); err != nil {
					errs = append(errs, err)
				} else {
					input.Token = token
				}
			}
		}
	}
	return input, errors.NewAggregate(errs)
}
