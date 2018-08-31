package copy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderedSet(t *testing.T) {
	for _, c := range []struct{ input, expected []string }{
		{[]string{}, []string{}},
		{[]string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]string{"a", "b", "a", "c"}, []string{"a", "b", "c"}},
	} {
		os := newOrderedSet()
		for _, s := range c.input {
			os.append(s)
		}
		assert.Equal(t, c.expected, os.list, fmt.Sprintf("%#v", c.input))
	}
}

// fakeImageSource is an implementation of types.Image which only returns itself as a MIME type in Manifest
// except that "" means “reading the manifest should fail”
type fakeImageSource string

func (f fakeImageSource) Reference() types.ImageReference {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) Manifest(ctx context.Context) ([]byte, string, error) {
	if string(f) == "" {
		return nil, "", errors.New("Manifest() directed to fail")
	}
	return nil, string(f), nil
}
func (f fakeImageSource) Signatures(context.Context) ([][]byte, error) {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) ConfigInfo() types.BlobInfo {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) ConfigBlob(context.Context) ([]byte, error) {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) OCIConfig(context.Context) (*v1.Image, error) {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) LayerInfos() []types.BlobInfo {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) LayerInfosForCopy(ctx context.Context) ([]types.BlobInfo, error) {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) EmbeddedDockerReferenceConflicts(ref reference.Named) bool {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) Inspect(context.Context) (*types.ImageInspectInfo, error) {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) UpdatedImageNeedsLayerDiffIDs(options types.ManifestUpdateOptions) bool {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) UpdatedImage(ctx context.Context, options types.ManifestUpdateOptions) (types.Image, error) {
	panic("Unexpected call to a mock function")
}
func (f fakeImageSource) Size() (int64, error) {
	panic("Unexpected call to a mock function")
}

func TestDetermineManifestConversion(t *testing.T) {
	supportS1S2OCI := []string{
		v1.MediaTypeImageManifest,
		manifest.DockerV2Schema2MediaType,
		manifest.DockerV2Schema1SignedMediaType,
		manifest.DockerV2Schema1MediaType,
	}
	supportS1OCI := []string{
		v1.MediaTypeImageManifest,
		manifest.DockerV2Schema1SignedMediaType,
		manifest.DockerV2Schema1MediaType,
	}
	supportS1S2 := []string{
		manifest.DockerV2Schema2MediaType,
		manifest.DockerV2Schema1SignedMediaType,
		manifest.DockerV2Schema1MediaType,
	}
	supportOnlyS1 := []string{
		manifest.DockerV2Schema1SignedMediaType,
		manifest.DockerV2Schema1MediaType,
	}

	cases := []struct {
		description             string
		sourceType              string
		destTypes               []string
		expectedUpdate          string
		expectedOtherCandidates []string
	}{
		// Destination accepts anything — no conversion necessary
		{"s1→anything", manifest.DockerV2Schema1SignedMediaType, nil, "", []string{}},
		{"s2→anything", manifest.DockerV2Schema2MediaType, nil, "", []string{}},
		// Destination accepts the unmodified original
		{"s1→s1s2", manifest.DockerV2Schema1SignedMediaType, supportS1S2, "", []string{manifest.DockerV2Schema2MediaType, manifest.DockerV2Schema1MediaType}},
		{"s2→s1s2", manifest.DockerV2Schema2MediaType, supportS1S2, "", supportOnlyS1},
		{"s1→s1", manifest.DockerV2Schema1SignedMediaType, supportOnlyS1, "", []string{manifest.DockerV2Schema1MediaType}},
		// text/plain is normalized to s1, and if the destination accepts s1, no conversion happens.
		{"text→s1s2", "text/plain", supportS1S2, "", []string{manifest.DockerV2Schema2MediaType, manifest.DockerV2Schema1MediaType}},
		{"text→s1", "text/plain", supportOnlyS1, "", []string{manifest.DockerV2Schema1MediaType}},
		// Conversion necessary, a preferred format is acceptable
		{"s2→s1", manifest.DockerV2Schema2MediaType, supportOnlyS1, manifest.DockerV2Schema1SignedMediaType, []string{manifest.DockerV2Schema1MediaType}},
		// Conversion necessary, a preferred format is not acceptable
		{"s2→OCI", manifest.DockerV2Schema2MediaType, []string{v1.MediaTypeImageManifest}, v1.MediaTypeImageManifest, []string{}},
		// text/plain is converted if the destination does not accept s1
		{"text→s2", "text/plain", []string{manifest.DockerV2Schema2MediaType}, manifest.DockerV2Schema2MediaType, []string{}},
		// Conversion necessary, try the preferred formats in order.
		// We abuse manifest.DockerV2ListMediaType here as a MIME type which is not in supportS1S2OCI,
		// but is still recognized by manifest.NormalizedMIMEType and not normalized to s1
		{
			"special→s2", manifest.DockerV2ListMediaType, supportS1S2OCI, manifest.DockerV2Schema2MediaType,
			[]string{manifest.DockerV2Schema1SignedMediaType, v1.MediaTypeImageManifest, manifest.DockerV2Schema1MediaType},
		},
		{
			"special→s1", manifest.DockerV2ListMediaType, supportS1OCI, manifest.DockerV2Schema1SignedMediaType,
			[]string{v1.MediaTypeImageManifest, manifest.DockerV2Schema1MediaType},
		},
		{
			"special→OCI", manifest.DockerV2ListMediaType, []string{v1.MediaTypeImageManifest, "other options", "with lower priority"}, v1.MediaTypeImageManifest,
			[]string{"other options", "with lower priority"},
		},
	}

	for _, c := range cases {
		src := fakeImageSource(c.sourceType)
		ic := &imageCopier{
			manifestUpdates:   &types.ManifestUpdateOptions{},
			src:               src,
			canModifyManifest: true,
		}
		preferredMIMEType, otherCandidates, err := ic.determineManifestConversion(context.Background(), c.destTypes, "")
		require.NoError(t, err, c.description)
		assert.Equal(t, c.expectedUpdate, ic.manifestUpdates.ManifestMIMEType, c.description)
		if c.expectedUpdate == "" {
			assert.Equal(t, manifest.NormalizedMIMEType(c.sourceType), preferredMIMEType, c.description)
		} else {
			assert.Equal(t, c.expectedUpdate, preferredMIMEType, c.description)
		}
		assert.Equal(t, c.expectedOtherCandidates, otherCandidates, c.description)
	}

	// Whatever the input is, with !canModifyManifest we return "keep the original as is"
	for _, c := range cases {
		src := fakeImageSource(c.sourceType)
		ic := &imageCopier{
			manifestUpdates:   &types.ManifestUpdateOptions{},
			src:               src,
			canModifyManifest: false,
		}
		preferredMIMEType, otherCandidates, err := ic.determineManifestConversion(context.Background(), c.destTypes, "")
		require.NoError(t, err, c.description)
		assert.Equal(t, "", ic.manifestUpdates.ManifestMIMEType, c.description)
		assert.Equal(t, manifest.NormalizedMIMEType(c.sourceType), preferredMIMEType, c.description)
		assert.Equal(t, []string{}, otherCandidates, c.description)
	}

	// With forceManifestMIMEType, the output is always the forced manifest type (in this case oci manifest)
	for _, c := range cases {
		src := fakeImageSource(c.sourceType)
		ic := &imageCopier{
			manifestUpdates:   &types.ManifestUpdateOptions{},
			src:               src,
			canModifyManifest: true,
		}
		preferredMIMEType, otherCandidates, err := ic.determineManifestConversion(context.Background(), c.destTypes, v1.MediaTypeImageManifest)
		require.NoError(t, err, c.description)
		assert.Equal(t, v1.MediaTypeImageManifest, ic.manifestUpdates.ManifestMIMEType, c.description)
		assert.Equal(t, v1.MediaTypeImageManifest, preferredMIMEType, c.description)
		assert.Equal(t, []string{}, otherCandidates, c.description)
	}

	// Error reading the manifest — smoke test only.
	ic := imageCopier{
		manifestUpdates:   &types.ManifestUpdateOptions{},
		src:               fakeImageSource(""),
		canModifyManifest: true,
	}
	_, _, err := ic.determineManifestConversion(context.Background(), supportS1S2, "")
	assert.Error(t, err)
}

func TestIsMultiImage(t *testing.T) {
	// MIME type is available; more or less a smoke test, other cases are handled in manifest.MIMETypeIsMultiImage
	for _, c := range []struct {
		mt       string
		expected bool
	}{
		{manifest.DockerV2ListMediaType, true},
		{manifest.DockerV2Schema2MediaType, false},
	} {
		src := fakeImageSource(c.mt)
		res, err := isMultiImage(context.Background(), src)
		require.NoError(t, err)
		assert.Equal(t, c.expected, res, c.mt)
	}

	// Error getting manifest MIME type
	src := fakeImageSource("")
	_, err := isMultiImage(context.Background(), src)
	assert.Error(t, err)
}
