package check

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/containers/image/v5/manifest"

	specsgov1 "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

// AllImageChecks returns a list of image checks to be performed on the image.
func AllImageChecks() []ImageCheck {
	return []ImageCheck{
		ImageIsValidManifest(),
		ImageHasExpectedLabelsWith(map[string]string{
			"operators.operatorframework.io.index.configs.v1": "/configs",
		}),
	}
}

// ImageIsValidManifest checks if the image is a valid manifest.
func ImageIsValidManifest() ImageCheck {
	return ImageCheck{
		Name: "ImageIsValidManifest",
		Fn: func(ctx context.Context, root specsgov1.Descriptor, target oras.ReadOnlyTarget) error {
			manifestReader, err := target.Fetch(ctx, root)
			if err != nil {
				return err
			}
			defer manifestReader.Close()
			var imgManifest specsgov1.Manifest
			if err := json.NewDecoder(manifestReader).Decode(&imgManifest); err != nil {
				return err
			}
			switch imgManifest.MediaType {
			case specsgov1.MediaTypeImageManifest, manifest.DockerV2Schema2MediaType:
			default:
				return fmt.Errorf("unrecognized manifest type: %s", imgManifest.MediaType)
			}
			return nil
		},
	}
}

// ImageHasExpectedLabelsWith checks if the image has the expected labels.
func ImageHasExpectedLabelsWith(expectedLabels map[string]string) ImageCheck {
	name := "ImageHasExpectedLabels"
	if len(expectedLabels) > 0 {
		var pairs []string
		for k, v := range expectedLabels {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(pairs)
		name = fmt.Sprintf("%s[%s]", name, strings.Join(pairs, ","))
	}

	return ImageCheck{
		Name: name,
		Fn: func(ctx context.Context, root specsgov1.Descriptor, target oras.ReadOnlyTarget) error {
			fetch, err := target.Fetch(ctx, root)
			if err != nil {
				return err
			}
			defer fetch.Close()
			var m specsgov1.Manifest
			if err := json.NewDecoder(fetch).Decode(&m); err != nil {
				return err
			}
			switch m.MediaType {
			case specsgov1.MediaTypeImageManifest, manifest.DockerV2Schema2MediaType:
			default:
				return fmt.Errorf("invalid media type: %s", m.MediaType)
			}

			reader, err := target.Fetch(ctx, m.Config)
			if err != nil {
				return err
			}
			defer reader.Close()

			var img specsgov1.Image
			if err := json.NewDecoder(reader).Decode(&img); err != nil {
				return err
			}

			var errs []error
			for expectedLabel, expectedValue := range expectedLabels {
				actualValue, ok := img.Config.Labels[expectedLabel]
				if !ok {
					errs = append(errs, fmt.Errorf("missing label: %s", expectedLabel))
					continue
				}
				if actualValue != expectedValue {
					errs = append(errs,
						fmt.Errorf("label %q: test expected %q, but image has %q",
							expectedLabel,
							expectedValue,
							actualValue))
				}
			}
			return errors.Join(errs...)
		},
	}
}
