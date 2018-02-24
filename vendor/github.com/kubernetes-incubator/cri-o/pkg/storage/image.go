package storage

import (
	"errors"
	"net"
	"path"
	"strings"
	"sync"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/manifest"
	"github.com/containers/image/signature"
	istorage "github.com/containers/image/storage"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
)

const (
	minimumTruncatedIDLength = 3
)

var (
	// ErrCannotParseImageID is returned when we try to ResolveNames for an image ID
	ErrCannotParseImageID = errors.New("cannot parse an image ID")
	// ErrImageMultiplyTagged is returned when we try to remove an image that still has multiple names
	ErrImageMultiplyTagged = errors.New("image still has multiple names applied")
)

// ImageResult wraps a subset of information about an image: its ID, its names,
// and the size, if known, or nil if it isn't.
type ImageResult struct {
	ID           string
	Name         string
	RepoTags     []string
	RepoDigests  []string
	Size         *uint64
	Digest       digest.Digest
	ConfigDigest digest.Digest
	User         string
}

type indexInfo struct {
	name   string
	secure bool
}

// A set of information that we prefer to cache about images, so that we can
// avoid having to reread them every time we need to return information about
// images.
type imageCacheItem struct {
	user         string
	size         *uint64
	configDigest digest.Digest
}

type imageService struct {
	store                 storage.Store
	defaultTransport      string
	insecureRegistryCIDRs []*net.IPNet
	indexConfigs          map[string]*indexInfo
	registries            []string
	imageCache            map[string]imageCacheItem
	imageCacheLock        sync.Mutex
}

// sizer knows its size.
type sizer interface {
	Size() (int64, error)
}

// ImageServer wraps up various CRI-related activities into a reusable
// implementation.
type ImageServer interface {
	// ListImages returns list of all images which match the filter.
	ListImages(systemContext *types.SystemContext, filter string) ([]ImageResult, error)
	// ImageStatus returns status of an image which matches the filter.
	ImageStatus(systemContext *types.SystemContext, filter string) (*ImageResult, error)
	// PrepareImage returns an Image where the config digest can be grabbed
	// for further analysis. Call Close() on the resulting image.
	PrepareImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.Image, error)
	// PullImage imports an image from the specified location.
	PullImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.ImageReference, error)
	// UntagImage removes a name from the specified image, and if it was
	// the only name the image had, removes the image.
	UntagImage(systemContext *types.SystemContext, imageName string) error
	// RemoveImage deletes the specified image.
	RemoveImage(systemContext *types.SystemContext, imageName string) error
	// GetStore returns the reference to the storage library Store which
	// the image server uses to hold images, and is the destination used
	// when it's asked to pull an image.
	GetStore() storage.Store
	// CanPull preliminary checks whether we're allowed to pull an image
	CanPull(imageName string, options *copy.Options) (bool, error)
	// ResolveNames takes an image reference and if it's unqualified (w/o hostname),
	// it uses crio's default registries to qualify it.
	ResolveNames(imageName string) ([]string, error)
}

func (svc *imageService) getRef(name string) (types.ImageReference, error) {
	ref, err := alltransports.ParseImageName(name)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+name)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, name)
			if err3 != nil {
				return nil, err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	return ref, nil
}

func sortNamesByType(names []string) (bestName string, tags, digests []string) {
	for _, name := range names {
		if len(name) > 72 && name[len(name)-72:len(name)-64] == "@sha256:" {
			digests = append(digests, name)
		} else {
			tags = append(tags, name)
		}
	}
	if len(digests) > 0 {
		bestName = digests[0]
	}
	if len(tags) > 0 {
		bestName = tags[0]
	}
	return bestName, tags, digests
}

func (svc *imageService) makeRepoDigests(knownRepoDigests, tags []string, imageID string) (imageDigest digest.Digest, repoDigests []string) {
	// Look up the image's digest.
	img, err := svc.store.Image(imageID)
	if err != nil {
		return "", knownRepoDigests
	}
	imageDigest = img.Digest
	if imageDigest == "" {
		imgDigest, err := svc.store.ImageBigDataDigest(imageID, storage.ImageDigestBigDataKey)
		if err != nil || imgDigest == "" {
			return "", knownRepoDigests
		}
		imageDigest = imgDigest
	}
	// If there are no names to convert to canonical references, we're done.
	if len(tags) == 0 {
		return imageDigest, knownRepoDigests
	}
	// We only want to supplement what's already explicitly in the list, so keep track of values
	// that we already know.
	digestMap := make(map[string]struct{})
	repoDigests = knownRepoDigests
	for _, repoDigest := range knownRepoDigests {
		digestMap[repoDigest] = struct{}{}
	}
	// For each tagged name, parse the name, and if we can extract a named reference, convert
	// it into a canonical reference using the digest and add it to the list.
	for _, tag := range tags {
		if ref, err2 := reference.ParseAnyReference(tag); err2 == nil {
			if name, ok := ref.(reference.Named); ok {
				trimmed := reference.TrimNamed(name)
				if imageRef, err3 := reference.WithDigest(trimmed, imageDigest); err3 == nil {
					if _, ok := digestMap[imageRef.String()]; !ok {
						repoDigests = append(repoDigests, imageRef.String())
						digestMap[imageRef.String()] = struct{}{}
					}
				}
			}
		}
	}
	return imageDigest, repoDigests
}

func (svc *imageService) ListImages(systemContext *types.SystemContext, filter string) ([]ImageResult, error) {
	results := []ImageResult{}
	if filter != "" {
		ref, err := svc.getRef(filter)
		if err != nil {
			return nil, err
		}
		if image, err := istorage.Transport.GetStoreImage(svc.store, ref); err == nil {
			var user string
			var size *uint64
			var configDigest digest.Digest
			if cacheItem, ok := svc.imageCache[image.ID]; ok {
				user, size, configDigest = cacheItem.user, cacheItem.size, cacheItem.configDigest
			} else {
				img, err := ref.NewImageSource(systemContext)
				if err != nil {
					return nil, err
				}
				size = imageSize(img)
				configDigest, err = imageConfigDigest(img, nil)
				img.Close()
				if err != nil {
					return nil, err
				}
				imageFull, err := ref.NewImage(systemContext)
				if err != nil {
					return nil, err
				}
				defer imageFull.Close()
				imageConfig, err := imageFull.OCIConfig()
				if err != nil {
					return nil, err
				}
				user = imageConfig.Config.User
				cacheItem := imageCacheItem{
					user:         user,
					size:         size,
					configDigest: configDigest,
				}
				svc.imageCache[image.ID] = cacheItem
			}
			name, tags, digests := sortNamesByType(image.Names)
			imageDigest, repoDigests := svc.makeRepoDigests(digests, tags, image.ID)
			results = append(results, ImageResult{
				ID:           image.ID,
				Name:         name,
				RepoTags:     tags,
				RepoDigests:  repoDigests,
				Size:         size,
				Digest:       imageDigest,
				ConfigDigest: configDigest,
				User:         user,
			})
		}
	} else {
		images, err := svc.store.Images()
		if err != nil {
			return nil, err
		}
		visited := make(map[string]struct{})
		defer func() {
			// We built a map using IDs of images that we looked
			// at, so remove any items from the cache that don't
			// correspond to any of those IDs.
			removedIDs := make([]string, 0, len(svc.imageCache))
			for imageID := range svc.imageCache {
				if _, keep := visited[imageID]; !keep {
					// We have cached data for an image
					// with this ID, but it's not in the
					// list of images now, so the image has
					// been removed.
					removedIDs = append(removedIDs, imageID)
				}
			}
			// Handle the removals.
			svc.imageCacheLock.Lock()
			for _, removedID := range removedIDs {
				delete(svc.imageCache, removedID)
			}
			svc.imageCacheLock.Unlock()
		}()
		for _, image := range images {
			visited[image.ID] = struct{}{}
			var user string
			var size *uint64
			var configDigest digest.Digest
			svc.imageCacheLock.Lock()
			cacheItem, ok := svc.imageCache[image.ID]
			svc.imageCacheLock.Unlock()
			if ok {
				user, size, configDigest = cacheItem.user, cacheItem.size, cacheItem.configDigest
			} else {
				ref, err := istorage.Transport.ParseStoreReference(svc.store, "@"+image.ID)
				if err != nil {
					return nil, err
				}
				img, err := ref.NewImageSource(systemContext)
				if err != nil {
					return nil, err
				}
				size = imageSize(img)
				configDigest, err = imageConfigDigest(img, nil)
				img.Close()
				if err != nil {
					return nil, err
				}
				imageFull, err := ref.NewImage(systemContext)
				if err != nil {
					return nil, err
				}
				defer imageFull.Close()

				imageConfig, err := imageFull.OCIConfig()
				if err != nil {
					return nil, err
				}
				user = imageConfig.Config.User
				cacheItem := imageCacheItem{
					user:         user,
					size:         size,
					configDigest: configDigest,
				}
				svc.imageCacheLock.Lock()
				svc.imageCache[image.ID] = cacheItem
				svc.imageCacheLock.Unlock()
			}
			name, tags, digests := sortNamesByType(image.Names)
			imageDigest, repoDigests := svc.makeRepoDigests(digests, tags, image.ID)
			results = append(results, ImageResult{
				ID:           image.ID,
				Name:         name,
				RepoTags:     tags,
				RepoDigests:  repoDigests,
				Size:         size,
				Digest:       imageDigest,
				ConfigDigest: configDigest,
				User:         user,
			})
		}
	}
	return results, nil
}

func (svc *imageService) ImageStatus(systemContext *types.SystemContext, nameOrID string) (*ImageResult, error) {
	ref, err := alltransports.ParseImageName(nameOrID)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+nameOrID)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, nameOrID)
			if err3 != nil {
				return nil, err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	image, err := istorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return nil, err
	}
	imageFull, err := ref.NewImage(systemContext)
	if err != nil {
		return nil, err
	}
	defer imageFull.Close()

	imageConfig, err := imageFull.OCIConfig()
	if err != nil {
		return nil, err
	}

	img, err := ref.NewImageSource(systemContext)
	if err != nil {
		return nil, err
	}
	defer img.Close()
	size := imageSize(img)
	configDigest, err := imageConfigDigest(img, nil)
	if err != nil {
		return nil, err
	}

	name, tags, digests := sortNamesByType(image.Names)
	imageDigest, repoDigests := svc.makeRepoDigests(digests, tags, image.ID)
	result := ImageResult{
		ID:           image.ID,
		Name:         name,
		RepoTags:     tags,
		RepoDigests:  repoDigests,
		Size:         size,
		Digest:       imageDigest,
		ConfigDigest: configDigest,
		User:         imageConfig.Config.User,
	}

	return &result, nil
}

func imageSize(img types.ImageSource) *uint64 {
	if s, ok := img.(sizer); ok {
		if sum, err := s.Size(); err == nil {
			usum := uint64(sum)
			return &usum
		}
	}
	return nil
}

func imageConfigDigest(img types.ImageSource, instanceDigest *digest.Digest) (digest.Digest, error) {
	manifestBytes, manifestType, err := img.GetManifest(instanceDigest)
	if err != nil {
		return "", err
	}
	imgManifest, err := manifest.FromBlob(manifestBytes, manifestType)
	if err != nil {
		return "", err
	}
	return imgManifest.ConfigInfo().Digest, nil
}

func (svc *imageService) CanPull(imageName string, options *copy.Options) (bool, error) {
	srcRef, err := svc.prepareReference(imageName, options)
	if err != nil {
		return false, err
	}
	rawSource, err := srcRef.NewImageSource(options.SourceCtx)
	if err != nil {
		return false, err
	}
	sourceCtx := &types.SystemContext{}
	if options.SourceCtx != nil {
		sourceCtx = options.SourceCtx
	}
	src, err := image.FromSource(sourceCtx, rawSource)
	if err != nil {
		rawSource.Close()
		return false, err
	}
	src.Close()
	return true, nil
}

// prepareReference creates an image reference from an image string and set options
// for the source context
func (svc *imageService) prepareReference(imageName string, options *copy.Options) (types.ImageReference, error) {
	if imageName == "" {
		return nil, storage.ErrNotAnImage
	}

	srcRef, err := alltransports.ParseImageName(imageName)
	if err != nil {
		if svc.defaultTransport == "" {
			return nil, err
		}
		srcRef2, err2 := alltransports.ParseImageName(svc.defaultTransport + imageName)
		if err2 != nil {
			return nil, err
		}
		srcRef = srcRef2
	}

	if options.SourceCtx == nil {
		options.SourceCtx = &types.SystemContext{}
	}

	hostname := reference.Domain(srcRef.DockerReference())
	if secure := svc.isSecureIndex(hostname); !secure {
		options.SourceCtx.DockerInsecureSkipTLSVerify = !secure
	}
	return srcRef, nil
}

func (svc *imageService) PrepareImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.Image, error) {
	if options == nil {
		options = &copy.Options{}
	}

	srcRef, err := svc.prepareReference(imageName, options)
	if err != nil {
		return nil, err
	}
	return srcRef.NewImage(systemContext)
}

func (svc *imageService) PullImage(systemContext *types.SystemContext, imageName string, options *copy.Options) (types.ImageReference, error) {
	policy, err := signature.DefaultPolicy(systemContext)
	if err != nil {
		return nil, err
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}
	if options == nil {
		options = &copy.Options{}
	}

	srcRef, err := svc.prepareReference(imageName, options)
	if err != nil {
		return nil, err
	}

	dest := imageName
	if srcRef.DockerReference() != nil {
		dest = srcRef.DockerReference().Name()
		if tagged, ok := srcRef.DockerReference().(reference.NamedTagged); ok {
			dest = dest + ":" + tagged.Tag()
		}
		if canonical, ok := srcRef.DockerReference().(reference.Canonical); ok {
			dest = dest + "@" + canonical.Digest().String()
		}
	}
	destRef, err := istorage.Transport.ParseStoreReference(svc.store, dest)
	if err != nil {
		return nil, err
	}
	err = copy.Image(policyContext, destRef, srcRef, options)
	if err != nil {
		return nil, err
	}
	return destRef, nil
}

func (svc *imageService) UntagImage(systemContext *types.SystemContext, nameOrID string) error {
	ref, err := alltransports.ParseImageName(nameOrID)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+nameOrID)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, nameOrID)
			if err3 != nil {
				return err
			}
			ref2 = ref3
		}
		ref = ref2
	}

	img, err := istorage.Transport.GetStoreImage(svc.store, ref)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(img.ID, nameOrID) {
		namedRef, err := svc.prepareReference(nameOrID, &copy.Options{})
		if err != nil {
			return err
		}

		name := nameOrID
		if namedRef.DockerReference() != nil {
			name = namedRef.DockerReference().Name()
			if tagged, ok := namedRef.DockerReference().(reference.NamedTagged); ok {
				name = name + ":" + tagged.Tag()
			}
			if canonical, ok := namedRef.DockerReference().(reference.Canonical); ok {
				name = name + "@" + canonical.Digest().String()
			}
		}

		prunedNames := make([]string, 0, len(img.Names))
		for _, imgName := range img.Names {
			if imgName != name && imgName != nameOrID {
				prunedNames = append(prunedNames, imgName)
			}
		}

		if len(prunedNames) > 0 {
			return svc.store.SetNames(img.ID, prunedNames)
		}
	}

	return ref.DeleteImage(systemContext)
}

func (svc *imageService) RemoveImage(systemContext *types.SystemContext, nameOrID string) error {
	ref, err := alltransports.ParseImageName(nameOrID)
	if err != nil {
		ref2, err2 := istorage.Transport.ParseStoreReference(svc.store, "@"+nameOrID)
		if err2 != nil {
			ref3, err3 := istorage.Transport.ParseStoreReference(svc.store, nameOrID)
			if err3 != nil {
				return err
			}
			ref2 = ref3
		}
		ref = ref2
	}
	return ref.DeleteImage(systemContext)
}

func (svc *imageService) GetStore() storage.Store {
	return svc.store
}

func (svc *imageService) isSecureIndex(indexName string) bool {
	if index, ok := svc.indexConfigs[indexName]; ok {
		return index.secure
	}

	host, _, err := net.SplitHostPort(indexName)
	if err != nil {
		// assume indexName is of the form `host` without the port and go on.
		host = indexName
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		ip := net.ParseIP(host)
		if ip != nil {
			addrs = []net.IP{ip}
		}

		// if ip == nil, then `host` is neither an IP nor it could be looked up,
		// either because the index is unreachable, or because the index is behind an HTTP proxy.
		// So, len(addrs) == 0 and we're not aborting.
	}

	// Try CIDR notation only if addrs has any elements, i.e. if `host`'s IP could be determined.
	for _, addr := range addrs {
		for _, ipnet := range svc.insecureRegistryCIDRs {
			// check if the addr falls in the subnet
			if ipnet.Contains(addr) {
				return false
			}
		}
	}

	return true
}

func splitDockerDomain(name string) (domain, remainder string) {
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		domain, remainder = "", name
	} else {
		domain, remainder = name[:i], name[i+1:]
	}
	return
}

func (svc *imageService) ResolveNames(imageName string) ([]string, error) {
	// _Maybe_ it's a truncated image ID.  Don't prepend a registry name, then.
	if len(imageName) >= minimumTruncatedIDLength && svc.store != nil {
		if img, err := svc.store.Image(imageName); err == nil && img != nil && strings.HasPrefix(img.ID, imageName) {
			// It's a truncated version of the ID of an image that's present in local storage;
			// we need to expand it.
			return []string{img.ID}, nil
		}
	}
	// This to prevent any image ID to go through this routine
	_, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		if strings.Contains(err.Error(), "cannot specify 64-byte hexadecimal strings") {
			return nil, ErrCannotParseImageID
		}
		return nil, err
	}
	domain, remainder := splitDockerDomain(imageName)
	if domain != "" {
		// this means the image is already fully qualified
		return []string{imageName}, nil
	}
	// we got an unqualified image here, we can't go ahead w/o registries configured
	// properly.
	if len(svc.registries) == 0 {
		return nil, errors.New("no registries configured while trying to pull an unqualified image")
	}
	// this means we got an image in the form of "busybox"
	// we need to use additional registries...
	// normalize the unqualified image to be domain/repo/image...
	images := []string{}
	for _, r := range svc.registries {
		rem := remainder
		if r == "docker.io" && !strings.ContainsRune(remainder, '/') {
			rem = "library/" + rem
		}
		images = append(images, path.Join(r, rem))
	}
	return images, nil
}

// GetImageService returns an ImageServer that uses the passed-in store, and
// which will prepend the passed-in defaultTransport value to an image name if
// a name that's passed to its PullImage() method can't be resolved to an image
// in the store and can't be resolved to a source on its own.
func GetImageService(store storage.Store, defaultTransport string, insecureRegistries []string, registries []string) (ImageServer, error) {
	if store == nil {
		var err error
		store, err = storage.GetStore(storage.DefaultStoreOptions)
		if err != nil {
			return nil, err
		}
	}

	seenRegistries := make(map[string]bool, len(registries))
	cleanRegistries := []string{}
	for _, r := range registries {
		if seenRegistries[r] {
			continue
		}
		cleanRegistries = append(cleanRegistries, r)
		seenRegistries[r] = true
	}

	is := &imageService{
		store:                 store,
		defaultTransport:      defaultTransport,
		indexConfigs:          make(map[string]*indexInfo, 0),
		insecureRegistryCIDRs: make([]*net.IPNet, 0),
		registries:            cleanRegistries,
		imageCache:            make(map[string]imageCacheItem),
	}

	insecureRegistries = append(insecureRegistries, "127.0.0.0/8")
	// Split --insecure-registry into CIDR and registry-specific settings.
	for _, r := range insecureRegistries {
		// Check if CIDR was passed to --insecure-registry
		_, ipnet, err := net.ParseCIDR(r)
		if err == nil {
			// Valid CIDR.
			is.insecureRegistryCIDRs = append(is.insecureRegistryCIDRs, ipnet)
		} else {
			// Assume `host:port` if not CIDR.
			is.indexConfigs[r] = &indexInfo{
				name:   r,
				secure: false,
			}
		}
	}

	return is, nil
}
