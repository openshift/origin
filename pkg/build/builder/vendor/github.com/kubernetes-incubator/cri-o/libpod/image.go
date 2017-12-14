package libpod

import (
	"io"
	"strings"
	"syscall"

	cp "github.com/containers/image/copy"
	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/manifest"
	"github.com/containers/image/signature"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/kubernetes-incubator/cri-o/libpod/common"
	"github.com/kubernetes-incubator/cri-o/libpod/ctr"
	"github.com/kubernetes-incubator/cri-o/libpod/images"
	"github.com/pkg/errors"
)

// Runtime API

const (
	// DefaultRegistry is a prefix that we apply to an image name
	// to check docker hub first for the image
	DefaultRegistry = "docker://"
)

// CopyOptions contains the options given when pushing or pulling images
type CopyOptions struct {
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// DockerRegistryOptions encapsulates settings that affect how we
	// connect or authenticate to a remote registry to which we want to
	// push the image.
	common.DockerRegistryOptions
	// SigningOptions encapsulates settings that control whether or not we
	// strip or add signatures to the image when pushing (uploading) the
	// image to a registry.
	common.SigningOptions
}

// Image API

// ImageFilter is a function to determine whether an image is included in
// command output. Images to be outputted are tested using the function. A true
// return will include the image, a false return will exclude it.
type ImageFilter func(*storage.Image) bool

// PullImage pulls an image from configured registries
// By default, only the latest tag (or a specific tag if requested) will be
// pulled. If allTags is true, all tags for the requested image will be pulled.
// Signature validation will be performed if the Runtime has been appropriately
// configured
func (r *Runtime) PullImage(imgName string, allTags bool, reportWriter io.Writer) error {
	// PullImage copies the image from the source to the destination
	var (
		images []string
	)

	srcRef, err := alltransports.ParseImageName(imgName)
	if err != nil {
		defaultName := DefaultRegistry + imgName
		srcRef2, err2 := alltransports.ParseImageName(defaultName)
		if err2 != nil {
			return errors.Errorf("error parsing image name %q: %v", defaultName, err2)
		}
		srcRef = srcRef2
	}

	splitArr := strings.Split(imgName, ":")

	// supports pulling from docker-archive, oci, and registries
	if splitArr[0] == "docker-archive" {
		tarSource := tarfile.NewSource(splitArr[len(splitArr)-1])
		manifest, err := tarSource.LoadTarManifest()
		if err != nil {
			return errors.Errorf("error retrieving manifest.json: %v", err)
		}
		// to pull all the images stored in one tar file
		for i := range manifest {
			if manifest[i].RepoTags != nil {
				images = append(images, manifest[i].RepoTags[0])
			} else {
				// create an image object and use the hex value of the digest as the image ID
				// for parsing the store reference
				newImg, err := srcRef.NewImage(r.imageContext)
				if err != nil {
					return err
				}
				defer newImg.Close()
				digest := newImg.ConfigInfo().Digest
				if err := digest.Validate(); err == nil {
					images = append(images, "@"+digest.Hex())
				} else {
					return errors.Wrapf(err, "error getting config info")
				}
			}
		}
	} else if splitArr[0] == "oci" {
		// needs to be implemented in future
		return errors.Errorf("oci not supported")
	} else {
		images = append(images, imgName)
	}

	policy, err := signature.DefaultPolicy(r.imageContext)
	if err != nil {
		return err
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	copyOptions := common.GetCopyOptions(reportWriter, "", nil, nil, common.SigningOptions{})

	for _, image := range images {
		destRef, err := is.Transport.ParseStoreReference(r.store, srcRef.DockerReference().String())
		if err != nil {
			return errors.Errorf("error parsing dest reference name: %v", err)
		}
		if err = cp.Image(policyContext, destRef, srcRef, copyOptions); err != nil {
			return errors.Errorf("error loading image %q: %v", image, err)
		}
	}
	return nil
}

// PushImage pushes the given image to a location described by the given path
func (r *Runtime) PushImage(source string, destination string, options CopyOptions, reportWriter io.Writer) error {
	// PushImage pushes the src image to the destination
	//func PushImage(source, destination string, options CopyOptions) error {
	if source == "" || destination == "" {
		return errors.Wrapf(syscall.EINVAL, "source and destination image names must be specified")
	}

	// Get the destination Image Reference
	dest, err := alltransports.ParseImageName(destination)
	if err != nil {
		return errors.Wrapf(err, "error getting destination imageReference for %q", destination)
	}

	policyContext, err := common.GetPolicyContext(r.GetConfig().SignaturePolicyPath)
	if err != nil {
		return errors.Wrapf(err, "Could not get default policy context for signature policy path %q", r.GetConfig().SignaturePolicyPath)
	}
	defer policyContext.Destroy()
	// Look up the image name and its layer, then build the imagePushData from
	// the image
	img, err := images.FindImage(r.store, source)
	if err != nil {
		return errors.Wrapf(err, "error locating image %q for importing settings", source)
	}
	cd, err := images.ImportCopyDataFromImage(r.store, r.imageContext, img.ID, "", "")
	if err != nil {
		return err
	}
	// Give the image we're producing the same ancestors as its source image
	cd.FromImage = cd.Docker.ContainerConfig.Image
	cd.FromImageID = string(cd.Docker.Parent)

	// Prep the layers and manifest for export
	src, err := cd.MakeImageRef(manifest.GuessMIMEType(cd.Manifest), options.Compression, img.Names, img.TopLayer, nil)
	if err != nil {
		return errors.Wrapf(err, "error copying layers and metadata")
	}

	copyOptions := common.GetCopyOptions(reportWriter, r.GetConfig().SignaturePolicyPath, nil, &options.DockerRegistryOptions, options.SigningOptions)

	// Copy the image to the remote destination
	err = cp.Image(policyContext, dest, src, copyOptions)
	if err != nil {
		return errors.Wrapf(err, "Error copying image to the remote destination")
	}
	return nil
}

// TagImage adds a tag to the given image
func (r *Runtime) TagImage(image *storage.Image, tag string) error {
	return ctr.ErrNotImplemented
}

// UntagImage removes a tag from the given image
func (r *Runtime) UntagImage(image *storage.Image, tag string) error {
	return ctr.ErrNotImplemented
}

// RemoveImage deletes an image from local storage
// Images being used by running containers cannot be removed
func (r *Runtime) RemoveImage(image *storage.Image) error {
	return ctr.ErrNotImplemented
}

// GetImage retrieves an image matching the given name or hash from system
// storage
// If no matching image can be found, an error is returned
func (r *Runtime) GetImage(image string) (*storage.Image, error) {
	return nil, ctr.ErrNotImplemented
}

// GetImages retrieves all images present in storage
// Filters can be provided which will determine which images are included in the
// output. Multiple filters are handled by ANDing their output, so only images
// matching all filters are included
func (r *Runtime) GetImages(filter ...ImageFilter) ([]*storage.Image, error) {
	return nil, ctr.ErrNotImplemented
}

// ImportImage imports an OCI format image archive into storage as an image
func (r *Runtime) ImportImage(path string) (*storage.Image, error) {
	return nil, ctr.ErrNotImplemented
}
