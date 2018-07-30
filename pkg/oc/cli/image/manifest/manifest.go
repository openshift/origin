package manifest

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime"
	"sync"

	"github.com/spf13/pflag"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"

	"github.com/docker/libtrust"
	"github.com/golang/glog"
	digest "github.com/opencontainers/go-digest"

	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/image/dockerlayer/add"
)

// FilterOptions assist in filtering out unneeded manifests from ManifestList objects.
type FilterOptions struct {
	FilterByOS      string
	DefaultOSFilter bool
	OSFilter        *regexp.Regexp
}

// Bind adds the options to the flag set.
func (o *FilterOptions) Bind(flags *pflag.FlagSet) {
	flags.StringVar(&o.FilterByOS, "filter-by-os", o.FilterByOS, "A regular expression to control which images are mirrored. Images will be passed as '<platform>/<architecture>[/<variant>]'.")
}

// Complete checks whether the flags are ready for use.
func (o *FilterOptions) Complete(flags *pflag.FlagSet) error {
	pattern := o.FilterByOS
	if len(pattern) == 0 && !flags.Changed("filter-by-os") {
		o.DefaultOSFilter = true
		pattern = regexp.QuoteMeta(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
	}
	if len(pattern) > 0 {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("--filter-by-os was not a valid regular expression: %v", err)
		}
		o.OSFilter = re
	}
	return nil
}

// Include returns true if the provided manifest should be included, or the first image if the user didn't alter the
// default selection and there is only one image.
func (o *FilterOptions) Include(d *manifestlist.ManifestDescriptor, hasMultiple bool) bool {
	if o.OSFilter == nil {
		return true
	}
	if o.DefaultOSFilter && !hasMultiple {
		return true
	}
	if len(d.Platform.Variant) > 0 {
		return o.OSFilter.MatchString(fmt.Sprintf("%s/%s/%s", d.Platform.OS, d.Platform.Architecture, d.Platform.Variant))
	}
	return o.OSFilter.MatchString(fmt.Sprintf("%s/%s", d.Platform.OS, d.Platform.Architecture))
}

// IncludeAll returns true if the provided manifest matches the filter, or all if there was no filter.
func (o *FilterOptions) IncludeAll(d *manifestlist.ManifestDescriptor, hasMultiple bool) bool {
	if o.OSFilter == nil {
		return true
	}
	if len(d.Platform.Variant) > 0 {
		return o.OSFilter.MatchString(fmt.Sprintf("%s/%s/%s", d.Platform.OS, d.Platform.Architecture, d.Platform.Variant))
	}
	return o.OSFilter.MatchString(fmt.Sprintf("%s/%s", d.Platform.OS, d.Platform.Architecture))
}

type FilterFunc func(*manifestlist.ManifestDescriptor, bool) bool

// PreferManifestList specifically requests a manifest list first
var PreferManifestList = distribution.WithManifestMediaTypes([]string{
	manifestlist.MediaTypeManifestList,
	schema2.MediaTypeManifest,
})

// FirstManifest returns the first manifest at the request location that matches the filter function.
func FirstManifest(ctx context.Context, from imagereference.DockerImageReference, repo distribution.Repository, filterFn FilterFunc) (distribution.Manifest, digest.Digest, string, error) {
	var srcDigest digest.Digest
	if len(from.Tag) > 0 {
		desc, err := repo.Tags(ctx).Get(ctx, from.Tag)
		if err != nil {
			return nil, "", "", err
		}
		srcDigest = desc.Digest
	} else {
		srcDigest = digest.Digest(from.ID)
	}
	manifests, err := repo.Manifests(ctx)
	if err != nil {
		return nil, "", "", err
	}
	srcManifest, err := manifests.Get(ctx, srcDigest, PreferManifestList)
	if err != nil {
		return nil, "", "", err
	}

	originalSrcDigest := srcDigest
	srcManifests, srcManifest, srcDigest, err := ProcessManifestList(ctx, srcDigest, srcManifest, manifests, from, filterFn)
	if err != nil {
		return nil, "", "", err
	}
	if len(srcManifests) == 0 {
		return nil, "", "", fmt.Errorf("filtered all images from %s", from)
	}

	var location string
	if srcDigest == originalSrcDigest {
		location = fmt.Sprintf("manifest %s", srcDigest)
	} else {
		location = fmt.Sprintf("manifest %s in manifest list %s", srcDigest, originalSrcDigest)
	}
	return srcManifest, srcDigest, location, nil
}

// ManifestToImageConfig takes an image manifest and converts it into a structured object.
func ManifestToImageConfig(ctx context.Context, srcManifest distribution.Manifest, blobs distribution.BlobService, location string) (*docker10.DockerImageConfig, []distribution.Descriptor, error) {
	switch t := srcManifest.(type) {
	case *schema2.DeserializedManifest:
		if t.Config.MediaType != schema2.MediaTypeImageConfig {
			return nil, nil, fmt.Errorf("%s does not have the expected image configuration media type: %s", location, t.Config.MediaType)
		}
		configJSON, err := blobs.Get(ctx, t.Config.Digest)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot retrieve image configuration for %s: %v", location, err)
		}
		glog.V(4).Infof("Raw image config json:\n%s", string(configJSON))
		config := &docker10.DockerImageConfig{}
		if err := json.Unmarshal(configJSON, &config); err != nil {
			return nil, nil, fmt.Errorf("unable to parse image configuration: %v", err)
		}

		base := config
		layers := t.Layers
		base.Size = 0
		for _, layer := range t.Layers {
			base.Size += layer.Size
		}

		return base, layers, nil

	case *schema1.SignedManifest:
		if glog.V(4) {
			_, configJSON, _ := srcManifest.Payload()
			glog.Infof("Raw image config json:\n%s", string(configJSON))
		}
		if len(t.History) == 0 {
			return nil, nil, fmt.Errorf("input image is in an unknown format: no v1Compatibility history")
		}
		config := &docker10.DockerV1CompatibilityImage{}
		if err := json.Unmarshal([]byte(t.History[0].V1Compatibility), &config); err != nil {
			return nil, nil, err
		}

		base := &docker10.DockerImageConfig{}
		if err := docker10.Convert_DockerV1CompatibilityImage_to_DockerImageConfig(config, base); err != nil {
			return nil, nil, err
		}

		// schema1 layers are in reverse order
		layers := make([]distribution.Descriptor, 0, len(t.FSLayers))
		for i := len(t.FSLayers) - 1; i >= 0; i-- {
			layer := distribution.Descriptor{
				MediaType: schema2.MediaTypeLayer,
				Digest:    t.FSLayers[i].BlobSum,
				// size must be reconstructed from the blobs
			}
			// we must reconstruct the tar sum from the blobs
			add.AddLayerToConfig(base, layer, "")
			layers = append(layers, layer)
		}

		return base, layers, nil

	default:
		return nil, nil, fmt.Errorf("unknown image manifest of type %T from %s", srcManifest, location)
	}
}

func ProcessManifestList(ctx context.Context, srcDigest digest.Digest, srcManifest distribution.Manifest, manifests distribution.ManifestService, ref imagereference.DockerImageReference, filterFn FilterFunc) ([]distribution.Manifest, distribution.Manifest, digest.Digest, error) {
	var srcManifests []distribution.Manifest
	switch t := srcManifest.(type) {
	case *manifestlist.DeserializedManifestList:
		manifestDigest := srcDigest
		manifestList := t

		filtered := make([]manifestlist.ManifestDescriptor, 0, len(t.Manifests))
		for _, manifest := range t.Manifests {
			if !filterFn(&manifest, len(t.Manifests) > 1) {
				glog.V(5).Infof("Skipping image for %#v from %s", manifest.Platform, ref)
				continue
			}
			glog.V(5).Infof("Including image for %#v from %s", manifest.Platform, ref)
			filtered = append(filtered, manifest)
		}

		if len(filtered) == 0 {
			return nil, nil, "", nil
		}

		// if we're filtering the manifest list, update the source manifest and digest
		if len(filtered) != len(t.Manifests) {
			var err error
			t, err = manifestlist.FromDescriptors(filtered)
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to filter source image %s manifest list: %v", ref, err)
			}
			_, body, err := t.Payload()
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to filter source image %s manifest list (bad payload): %v", ref, err)
			}
			manifestList = t
			manifestDigest = srcDigest.Algorithm().FromBytes(body)
			glog.V(5).Infof("Filtered manifest list to new digest %s:\n%s", manifestDigest, body)
		}

		for i, manifest := range t.Manifests {
			childManifest, err := manifests.Get(ctx, manifest.Digest, distribution.WithManifestMediaTypes([]string{manifestlist.MediaTypeManifestList, schema2.MediaTypeManifest}))
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to retrieve source image %s manifest #%d from manifest list: %v", ref, i+1, err)
			}
			srcManifests = append(srcManifests, childManifest)
		}

		switch {
		case len(srcManifests) == 1:
			_, body, err := srcManifests[0].Payload()
			if err != nil {
				return nil, nil, "", fmt.Errorf("unable to convert source image %s manifest list to single manifest: %v", ref, err)
			}
			manifestDigest := srcDigest.Algorithm().FromBytes(body)
			glog.V(5).Infof("Used only one manifest from the list %s", manifestDigest)
			return srcManifests, srcManifests[0], manifestDigest, nil
		default:
			return append(srcManifests, manifestList), manifestList, manifestDigest, nil
		}

	default:
		return []distribution.Manifest{srcManifest}, srcManifest, srcDigest, nil
	}
}

// TDOO: remove when quay.io switches to v2 schema
func PutManifestInCompatibleSchema(
	ctx context.Context,
	srcManifest distribution.Manifest,
	tag string,
	toManifests distribution.ManifestService,
	// supports schema2 -> schema1 downconversion
	blobs distribution.BlobService,
	ref reference.Named,
) (digest.Digest, error) {
	var options []distribution.ManifestServiceOption
	if len(tag) > 0 {
		glog.V(5).Infof("Put manifest %s:%s", ref, tag)
		options = []distribution.ManifestServiceOption{distribution.WithTag(tag)}
	} else {
		glog.V(5).Infof("Put manifest %s", ref)
	}
	toDigest, err := toManifests.Put(ctx, srcManifest, options...)
	if err == nil {
		return toDigest, nil
	}
	errs, ok := err.(errcode.Errors)
	if !ok || len(errs) == 0 {
		return toDigest, err
	}
	errcode, ok := errs[0].(errcode.Error)
	if !ok || errcode.ErrorCode() != v2.ErrorCodeManifestInvalid {
		return toDigest, err
	}
	// try downconverting to v2-schema1
	schema2Manifest, ok := srcManifest.(*schema2.DeserializedManifest)
	if !ok {
		return toDigest, err
	}
	tagRef, tagErr := reference.WithTag(ref, tag)
	if tagErr != nil {
		return toDigest, err
	}
	glog.V(5).Infof("Registry reported invalid manifest error, attempting to convert to v2schema1 as ref %s", tagRef)
	schema1Manifest, convertErr := convertToSchema1(ctx, blobs, schema2Manifest, tagRef)
	if convertErr != nil {
		return toDigest, err
	}
	if glog.V(6) {
		_, data, _ := schema1Manifest.Payload()
		glog.Infof("Converted to v2schema1\n%s", string(data))
	}
	return toManifests.Put(ctx, schema1Manifest, distribution.WithTag(tag))
}

// TDOO: remove when quay.io switches to v2 schema
func convertToSchema1(ctx context.Context, blobs distribution.BlobService, schema2Manifest *schema2.DeserializedManifest, ref reference.Named) (distribution.Manifest, error) {
	targetDescriptor := schema2Manifest.Target()
	configJSON, err := blobs.Get(ctx, targetDescriptor.Digest)
	if err != nil {
		return nil, err
	}
	trustKey, err := loadPrivateKey()
	if err != nil {
		return nil, err
	}
	builder := schema1.NewConfigManifestBuilder(blobs, trustKey, ref, configJSON)
	for _, d := range schema2Manifest.Layers {
		if err := builder.AppendReference(d); err != nil {
			return nil, err
		}
	}
	manifest, err := builder.Build(ctx)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

var (
	privateKeyLock sync.Mutex
	privateKey     libtrust.PrivateKey
)

// TDOO: remove when quay.io switches to v2 schema
func loadPrivateKey() (libtrust.PrivateKey, error) {
	privateKeyLock.Lock()
	defer privateKeyLock.Unlock()
	if privateKey != nil {
		return privateKey, nil
	}
	trustKey, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return nil, err
	}
	privateKey = trustKey
	return privateKey, nil
}
