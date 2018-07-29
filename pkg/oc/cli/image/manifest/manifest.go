package manifest

import (
	"context"
	"fmt"
	"sync"

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

	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
)

func ProcessManifestList(ctx context.Context, srcDigest digest.Digest, srcManifest distribution.Manifest, manifests distribution.ManifestService, ref imagereference.DockerImageReference, filterFn func(*manifestlist.ManifestDescriptor, bool) bool) ([]distribution.Manifest, distribution.Manifest, digest.Digest, error) {
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
