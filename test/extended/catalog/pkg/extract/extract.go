package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	imgcopy "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/archive"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
)

// Result represents the extracted OCI content and related context
type Result struct {
	Store   oras.ReadOnlyTarget
	TmpDir  string
	Cleanup func()
}

// UnpackImage pulls the image, extracts it to disk, and opens it as an OCI store.
func UnpackImage(ctx context.Context, imageRef, name string) (res *Result, err error) {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("oci-%s-", name))
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	cleanup := func() { os.RemoveAll(tmpDir) }

	if err := pullImage(ctx, imageRef, tmpDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("pull image: %w", err)
	}

	if err := getLayers(ctx, tmpDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("extract filesystem: %w", err)
	}

	store, err := oci.New(tmpDir)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("open OCI store: %w", err)
	}

	return &Result{
		Store:   store,
		TmpDir:  tmpDir,
		Cleanup: cleanup,
	}, nil
}

// pullImage pulls an image and writes it to OCI layout at the specified directory
func pullImage(ctx context.Context, imageRef, tmpDir string) error {
	srcRef, err := docker.ParseReference("//" + imageRef)
	if err != nil {
		return fmt.Errorf("parse image ref: %w", err)
	}

	destRef, err := layout.ParseReference(fmt.Sprintf("%s:%s", tmpDir, "v1"))
	if err != nil {
		return fmt.Errorf("parse layout ref: %w", err)
	}

	// Force image resolution to Linux platform to avoid OS mismatch errors
	// during testing on Mac. Example error:
	// err: <*errors.errorString>{
	//     s: "no image found in manifest list for architecture \"arm64\", variant \"v8\", OS \"darwin\"",
	// }
	sysCtx := &types.SystemContext{
		OSChoice: "linux",
	}
	
	policy, err := signature.DefaultPolicy(sysCtx)
	if err != nil {
		return fmt.Errorf("load default policy: %w", err)
	}

	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return fmt.Errorf("create policy context: %w", err)
	}
	defer policyCtx.Destroy()

	if _, err := imgcopy.Image(ctx, policyCtx, destRef, srcRef, &imgcopy.Options{
		SourceCtx:      sysCtx,
		DestinationCtx: sysCtx,
	}); err != nil {
		return fmt.Errorf("copy image: %w", err)
	}

	return nil
}

// getLayers extracts the filesystem layers from the catalog image into <tmpDir>/fs
func getLayers(ctx context.Context, imagePath string) error {
	ref, err := layout.ParseReference(fmt.Sprintf("%s:%s", imagePath, "v1"))
	if err != nil {
		return fmt.Errorf("parse layout: %w", err)
	}
	src, err := ref.NewImageSource(ctx, nil)
	if err != nil {
		return fmt.Errorf("open image source: %w", err)
	}
	defer src.Close()

	manifests, _, err := src.GetManifest(ctx, nil)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	mf, err := manifest.FromBlob(manifests, manifest.GuessMIMEType(manifests))
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	dir := filepath.Join(imagePath, "fs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("make fs dir to unpack: %w", err)
	}

	for i, layer := range mf.LayerInfos() {
		rc, _, err := src.GetBlob(ctx, layer.BlobInfo, nil)
		if err != nil {
			return fmt.Errorf("get blob %d: %w", i, err)
		}

		decompressed, _, err := compression.AutoDecompress(rc)
		if err != nil {
			rc.Close()
			return fmt.Errorf("decompress blob %d: %w", i, err)
		}

		// To avoid permission errors faced when extracting the layers
		mask := os.FileMode(0755)
		opts := &archive.TarOptions{
			ForceMask: &mask,
		}

		_, err = archive.ApplyUncompressedLayer(dir, decompressed, opts)
		decompressed.Close()
		rc.Close()

		if err != nil {
			return fmt.Errorf("apply layer %d: %w", i, err)
		}
	}
	return nil
}
