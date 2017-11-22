package images

import (
	"io"
	"os"
	"strings"
	"syscall"

	cp "github.com/containers/image/copy"
	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/docker/tarfile"
	"github.com/containers/image/manifest"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/signature"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/kubernetes-incubator/cri-o/libpod/common"
	"github.com/pkg/errors"
)

const (
	// DefaultRegistry is a prefix that we apply to an image name
	// to check docker hub first for the image
	DefaultRegistry = "docker://"
)

var (
	// DockerArchive is the transport we prepend to an image name
	// when saving to docker-archive
	DockerArchive = dockerarchive.Transport.Name()
	// OCIArchive is the transport we prepend to an image name
	// when saving to oci-archive
	OCIArchive = ociarchive.Transport.Name()
)

// CopyOptions contains the options given when pushing or pulling images
type CopyOptions struct {
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// ReportWriter is an io.Writer which will be used to log the writing
	// of the new image.
	ReportWriter io.Writer
	// Store is the local storage store which holds the source image.
	Store storage.Store
	// DockerRegistryOptions encapsulates settings that affect how we
	// connect or authenticate to a remote registry to which we want to
	// push the image.
	common.DockerRegistryOptions
	// SigningOptions encapsulates settings that control whether or not we
	// strip or add signatures to the image when pushing (uploading) the
	// image to a registry.
	common.SigningOptions
	// Quiet suppresses the output when a push or pull happens
	Quiet bool
}

// PushImage pushes the src image to the destination
func PushImage(srcName, destName string, options CopyOptions) error {
	if srcName == "" || destName == "" {
		return errors.Wrapf(syscall.EINVAL, "source and destination image names must be specified")
	}

	// Get the destination Image Reference
	dest, err := alltransports.ParseImageName(destName)
	if err != nil {
		return errors.Wrapf(err, "error getting destination imageReference for %q", destName)
	}

	policyContext, err := common.GetPolicyContext(options.SignaturePolicyPath)
	if err != nil {
		return errors.Wrapf(err, "Could not get default policy context for signature policy path %q", options.SignaturePolicyPath)
	}
	defer policyContext.Destroy()
	// Look up the image name and its layer, then build the imagePushData from
	// the image
	img, err := FindImage(options.Store, srcName)
	if err != nil {
		return errors.Wrapf(err, "error locating image %q for importing settings", srcName)
	}
	systemContext := common.GetSystemContext(options.SignaturePolicyPath)
	cd, err := ImportCopyDataFromImage(options.Store, systemContext, img.ID, "", "")
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

	copyOptions := common.GetCopyOptions(options.ReportWriter, options.SignaturePolicyPath, nil, &options.DockerRegistryOptions, options.SigningOptions)

	// Copy the image to the remote destination
	err = cp.Image(policyContext, dest, src, copyOptions)
	if err != nil {
		return errors.Wrapf(err, "Error copying image to the remote destination")
	}
	return nil
}

// PullImage copies the image from the source to the destination
func PullImage(imgName string, allTags bool, options CopyOptions) error {
	var (
		images []string
		output io.Writer
	)
	store := options.Store
	sc := common.GetSystemContext(options.SignaturePolicyPath)

	if options.Quiet {
		output = nil
	} else {
		output = os.Stdout
	}

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
	archFile := splitArr[len(splitArr)-1]

	// supports pulling from docker-archive, oci, and registries
	if srcRef.Transport().Name() == DockerArchive {
		tarSource := tarfile.NewSource(archFile)
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
				newImg, err := srcRef.NewImage(sc)
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
	} else if srcRef.Transport().Name() == OCIArchive {
		// retrieve the manifest from index.json to access the image name
		manifest, err := ociarchive.LoadManifestDescriptor(srcRef)
		if err != nil {
			return errors.Wrapf(err, "error loading manifest for %q", srcRef)
		}

		if manifest.Annotations == nil || manifest.Annotations["org.opencontainers.image.ref.name"] == "" {
			return errors.Errorf("error, archive doesn't have a name annotation. Cannot store image with no name")
		}
		images = append(images, manifest.Annotations["org.opencontainers.image.ref.name"])
	} else {
		images = append(images, imgName)
	}

	policy, err := signature.DefaultPolicy(sc)
	if err != nil {
		return err
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	copyOptions := common.GetCopyOptions(output, "", nil, nil, common.SigningOptions{})

	for _, image := range images {
		destRef, err := is.Transport.ParseStoreReference(store, image)
		if err != nil {
			return errors.Errorf("error parsing dest reference name: %v", err)
		}
		if err = cp.Image(policyContext, destRef, srcRef, copyOptions); err != nil {
			return errors.Errorf("error loading image %q: %v", image, err)
		}
	}
	return nil
}
