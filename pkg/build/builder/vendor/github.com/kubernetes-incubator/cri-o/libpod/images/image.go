package images

import (
	"fmt"
	"strings"
	"time"

	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/kubernetes-incubator/cri-o/libpod/common"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// FilterParams contains the filter options that may be given when outputting images
type FilterParams struct {
	dangling         string
	label            string
	beforeImage      time.Time
	sinceImage       time.Time
	referencePattern string
}

// ParseFilter takes a set of images and a filter string as input, and returns the
func ParseFilter(store storage.Store, filter string) (*FilterParams, error) {
	images, err := store.Images()
	if err != nil {
		return nil, err
	}
	params := new(FilterParams)
	filterStrings := strings.Split(filter, ",")
	for _, param := range filterStrings {
		pair := strings.SplitN(param, "=", 2)
		switch strings.TrimSpace(pair[0]) {
		case "dangling":
			if common.IsValidBool(pair[1]) {
				params.dangling = pair[1]
			} else {
				return nil, fmt.Errorf("invalid filter: '%s=[%s]'", pair[0], pair[1])
			}
		case "label":
			params.label = pair[1]
		case "before":
			if img, err := findImageInSlice(images, pair[1]); err == nil {
				info, err := getImageInspectInfo(store, img)
				if err != nil {
					return nil, err
				}
				params.beforeImage = info.Created
			} else {
				return nil, fmt.Errorf("no such id: %s", pair[0])
			}
		case "since":
			if img, err := findImageInSlice(images, pair[1]); err == nil {
				info, err := getImageInspectInfo(store, img)
				if err != nil {
					return nil, err
				}
				params.sinceImage = info.Created
			} else {
				return nil, fmt.Errorf("no such id: %s``", pair[0])
			}
		case "reference":
			params.referencePattern = pair[1]
		default:
			return nil, fmt.Errorf("invalid filter: '%s'", pair[0])
		}
	}
	return params, nil
}

func matchesFilter(store storage.Store, image storage.Image, name string, params *FilterParams) bool {
	if params == nil {
		return true
	}

	info, err := getImageInspectInfo(store, image)
	if err != nil {
		return false
	}
	if params.dangling != "" && !matchesDangling(name, params.dangling) {
		return false
	} else if params.label != "" && !matchesLabel(info, store, params.label) {
		return false
	} else if !params.beforeImage.IsZero() && !matchesBeforeImage(info, name, params) {
		return false
	} else if !params.sinceImage.IsZero() && !matchesSinceImage(info, name, params) {
		return false
	} else if params.referencePattern != "" && !MatchesReference(name, params.referencePattern) {
		return false
	}
	return true
}

func matchesDangling(name string, dangling string) bool {
	if common.IsFalse(dangling) && name != "<none>" {
		return true
	} else if common.IsTrue(dangling) && name == "<none>" {
		return true
	}
	return false
}

func matchesLabel(info *types.ImageInspectInfo, store storage.Store, label string) bool {
	pair := strings.SplitN(label, "=", 2)
	for key, value := range info.Labels {
		if key == pair[0] {
			if len(pair) == 2 {
				if value == pair[1] {
					return true
				}
			} else {
				return false
			}
		}
	}
	return false
}

// Returns true if the image was created since the filter image.  Returns
// false otherwise
func matchesBeforeImage(info *types.ImageInspectInfo, name string, params *FilterParams) bool {
	return info.Created.Before(params.beforeImage)
}

// Returns true if the image was created since the filter image.  Returns
// false otherwise
func matchesSinceImage(info *types.ImageInspectInfo, name string, params *FilterParams) bool {
	return info.Created.After(params.sinceImage)
}

// MatchesID returns true if argID is a full or partial match for id
func MatchesID(id, argID string) bool {
	return strings.HasPrefix(argID, id)
}

// MatchesReference returns true if argName is a full or partial match for name
// Partial matches will register only if they match the most specific part of the name available
// For example, take the image docker.io/library/redis:latest
// redis, library,redis, docker.io/library/redis, redis:latest, etc. will match
// But redis:alpine, ry/redis, library, and io/library/redis will not
func MatchesReference(name, argName string) bool {
	if argName == "" {
		return false
	}
	splitName := strings.Split(name, ":")
	// If the arg contains a tag, we handle it differently than if it does not
	if strings.Contains(argName, ":") {
		splitArg := strings.Split(argName, ":")
		return strings.HasSuffix(splitName[0], splitArg[0]) && (splitName[1] == splitArg[1])
	}
	return strings.HasSuffix(splitName[0], argName)
}

// FormattedSize returns a human-readable formatted size for the image
func FormattedSize(size float64) string {
	suffixes := [5]string{"B", "KB", "MB", "GB", "TB"}

	count := 0
	for size >= 1024 && count < 4 {
		size /= 1024
		count++
	}
	return fmt.Sprintf("%.4g %s", size, suffixes[count])
}

// FindImage searches for a *storage.Image with a matching the given name or ID in the given store.
func FindImage(store storage.Store, image string) (*storage.Image, error) {
	var img *storage.Image
	ref, err := is.Transport.ParseStoreReference(store, image)
	if err == nil {
		img, err = is.Transport.GetStoreImage(store, ref)
	}
	if err != nil {
		img2, err2 := store.Image(image)
		if err2 != nil {
			if ref == nil {
				return nil, errors.Wrapf(err, "error parsing reference to image %q", image)
			}
			return nil, errors.Wrapf(err, "unable to locate image %q", image)
		}
		img = img2
	}
	return img, nil
}

// FindImageRef searches for and returns a new types.Image matching the given name or ID in the given store.
func FindImageRef(store storage.Store, image string) (types.Image, error) {
	img, err := FindImage(store, image)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to locate image %q", image)
	}
	ref, err := is.Transport.ParseStoreReference(store, "@"+img.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing reference to image %q", img.ID)
	}
	imgRef, err := ref.NewImage(nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading image %q", img.ID)
	}
	return imgRef, nil
}

func findImageInSlice(images []storage.Image, ref string) (storage.Image, error) {
	for _, image := range images {
		if MatchesID(image.ID, ref) {
			return image, nil
		}
		for _, name := range image.Names {
			if MatchesReference(name, ref) {
				return image, nil
			}
		}
	}
	return storage.Image{}, errors.New("could not find image")
}

// InfoAndDigestAndSize returns the inspection info and size of the image in the given
// store and the digest of its manifest, if it has one, or "" if it doesn't.
func InfoAndDigestAndSize(store storage.Store, img storage.Image) (*types.ImageInspectInfo, digest.Digest, int64, error) {
	imgRef, err := FindImageRef(store, "@"+img.ID)
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error reading image %q", img.ID)
	}
	defer imgRef.Close()
	return infoAndDigestAndSize(imgRef)
}

func infoAndDigestAndSize(imgRef types.Image) (*types.ImageInspectInfo, digest.Digest, int64, error) {
	imgSize, err := imgRef.Size()
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error reading size of image %q", transports.ImageName(imgRef.Reference()))
	}
	manifest, _, err := imgRef.Manifest()
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error reading manifest for image %q", transports.ImageName(imgRef.Reference()))
	}
	manifestDigest := digest.Digest("")
	if len(manifest) > 0 {
		manifestDigest = digest.Canonical.FromBytes(manifest)
	}
	info, err := imgRef.Inspect()
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error inspecting image %q", transports.ImageName(imgRef.Reference()))
	}
	return info, manifestDigest, imgSize, nil
}

// GetImagesMatchingFilter returns a slice of all images in the store that match the provided FilterParams.
// Images with more than one name matching the filter will be in the slice once for each name
func GetImagesMatchingFilter(store storage.Store, filter *FilterParams, argName string) ([]storage.Image, error) {
	images, err := store.Images()
	filteredImages := []storage.Image{}
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		names := []string{}
		if len(image.Names) > 0 {
			names = image.Names
		} else {
			names = append(names, "<none>")
		}
		for _, name := range names {
			if (filter == nil && argName == "") || (filter != nil && matchesFilter(store, image, name, filter)) || MatchesReference(name, argName) {
				newImage := image
				newImage.Names = []string{name}
				filteredImages = append(filteredImages, newImage)
			}
		}
	}
	return filteredImages, nil
}

func getImageInspectInfo(store storage.Store, image storage.Image) (*types.ImageInspectInfo, error) {
	storeRef, err := is.Transport.ParseStoreReference(store, "@"+image.ID)
	if err != nil {
		return nil, err
	}
	img, err := storeRef.NewImage(nil)
	if err != nil {
		return nil, err
	}
	defer img.Close()
	return img.Inspect()
}
