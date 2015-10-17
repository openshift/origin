package storage

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/storage/driver"
	"github.com/docker/distribution/uuid"
)

type byDigestString []digest.Digest

func (bd byDigestString) Len() int           { return len(bd) }
func (bd byDigestString) Less(i, j int) bool { return bd[i].String() < bd[j].String() }
func (bd byDigestString) Swap(i, j int)      { bd[i], bd[j] = bd[j], bd[i] }

// linkPathFunc describes a function that can resolve a link based on the
// repository name and digest.
type linkPathFunc func(pm *pathMapper, name string, dgst digest.Digest) (string, error)

// blobsRootPathFunc describes a function that can resolve a root directory of
// blob links based on the repository name.
type blobsRootPathFunc func(pm *pathMapper, name string) (string, error)

// linkedBlobStore provides a full BlobService that namespaces the blobs to a
// given repository. Effectively, it manages the links in a given repository
// that grant access to the global blob store.
type linkedBlobStore struct {
	*blobStore
	blobServer             distribution.BlobServer
	blobAccessController   distribution.BlobDescriptorService
	repository             distribution.Repository
	ctx                    context.Context // only to be used where context can't come through method args
	deleteEnabled          bool
	resumableDigestEnabled bool

	// linkPathFns specifies one or more path functions allowing one to
	// control the repository blob link set to which the blob store
	// dispatches. This is required because manifest and layer blobs have not
	// yet been fully merged. At some point, this functionality should be
	// removed an the blob links folder should be merged. The first entry is
	// treated as the "canonical" link location and will be used for writes.
	linkPathFns []linkPathFunc

	// blobsRootPathFns functions the same way for blob root directories as
	// linkPathFns for blob links.
	blobsRootPathFns []blobsRootPathFunc
}

var _ distribution.BlobStore = &linkedBlobStore{}

func (lbs *linkedBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	return lbs.blobAccessController.Stat(ctx, dgst)
}

func (lbs *linkedBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	canonical, err := lbs.Stat(ctx, dgst) // access check
	if err != nil {
		return nil, err
	}

	return lbs.blobStore.Get(ctx, canonical.Digest)
}

func (lbs *linkedBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	canonical, err := lbs.Stat(ctx, dgst) // access check
	if err != nil {
		return nil, err
	}

	return lbs.blobStore.Open(ctx, canonical.Digest)
}

func (lbs *linkedBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	canonical, err := lbs.Stat(ctx, dgst) // access check
	if err != nil {
		return err
	}

	if canonical.MediaType != "" {
		// Set the repository local content type.
		w.Header().Set("Content-Type", canonical.MediaType)
	}

	return lbs.blobServer.ServeBlob(ctx, w, r, canonical.Digest)
}

func (lbs *linkedBlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	dgst, err := digest.FromBytes(p)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	// Place the data in the blob store first.
	desc, err := lbs.blobStore.Put(ctx, mediaType, p)
	if err != nil {
		context.GetLogger(ctx).Errorf("error putting into main store: %v", err)
		return distribution.Descriptor{}, err
	}

	if err := lbs.blobAccessController.SetDescriptor(ctx, dgst, desc); err != nil {
		return distribution.Descriptor{}, err
	}

	// TODO(stevvooe): Write out mediatype if incoming differs from what is
	// returned by Put above. Note that we should allow updates for a given
	// repository.

	return desc, lbs.linkBlob(ctx, desc)
}

// Writer begins a blob write session, returning a handle.
func (lbs *linkedBlobStore) Create(ctx context.Context) (distribution.BlobWriter, error) {
	context.GetLogger(ctx).Debug("(*linkedBlobStore).Create")

	uuid := uuid.Generate().String()
	startedAt := time.Now().UTC()

	path, err := lbs.blobStore.pm.path(uploadDataPathSpec{
		name: lbs.repository.Name(),
		id:   uuid,
	})

	if err != nil {
		return nil, err
	}

	startedAtPath, err := lbs.blobStore.pm.path(uploadStartedAtPathSpec{
		name: lbs.repository.Name(),
		id:   uuid,
	})

	if err != nil {
		return nil, err
	}

	// Write a startedat file for this upload
	if err := lbs.blobStore.driver.PutContent(ctx, startedAtPath, []byte(startedAt.Format(time.RFC3339))); err != nil {
		return nil, err
	}

	return lbs.newBlobUpload(ctx, uuid, path, startedAt)
}

func (lbs *linkedBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	context.GetLogger(ctx).Debug("(*linkedBlobStore).Resume")

	startedAtPath, err := lbs.blobStore.pm.path(uploadStartedAtPathSpec{
		name: lbs.repository.Name(),
		id:   id,
	})

	if err != nil {
		return nil, err
	}

	startedAtBytes, err := lbs.blobStore.driver.GetContent(ctx, startedAtPath)
	if err != nil {
		switch err := err.(type) {
		case driver.PathNotFoundError:
			return nil, distribution.ErrBlobUploadUnknown
		default:
			return nil, err
		}
	}

	startedAt, err := time.Parse(time.RFC3339, string(startedAtBytes))
	if err != nil {
		return nil, err
	}

	path, err := lbs.pm.path(uploadDataPathSpec{
		name: lbs.repository.Name(),
		id:   id,
	})

	if err != nil {
		return nil, err
	}

	return lbs.newBlobUpload(ctx, id, path, startedAt)
}

func (lbs *linkedBlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	context.GetLogger(ctx).Debug("(*linkedBlobStore).Delete")
	if !lbs.deleteEnabled {
		return distribution.ErrUnsupported
	}

	return lbs.blobAccessController.Clear(ctx, dgst)
}

// newBlobUpload allocates a new upload controller with the given state.
func (lbs *linkedBlobStore) newBlobUpload(ctx context.Context, uuid, path string, startedAt time.Time) (distribution.BlobWriter, error) {
	fw, err := newFileWriter(ctx, lbs.driver, path)
	if err != nil {
		return nil, err
	}

	bw := &blobWriter{
		blobStore:              lbs,
		id:                     uuid,
		startedAt:              startedAt,
		digester:               digest.Canonical.New(),
		bufferedFileWriter:     *fw,
		resumableDigestEnabled: lbs.resumableDigestEnabled,
	}

	return bw, nil
}

// linkBlob links a valid, written blob into the registry under the named
// repository for the upload controller.
func (lbs *linkedBlobStore) linkBlob(ctx context.Context, canonical distribution.Descriptor, aliases ...digest.Digest) error {
	dgsts := append([]digest.Digest{canonical.Digest}, aliases...)

	// TODO(stevvooe): Need to write out mediatype for only canonical hash
	// since we don't care about the aliases. They are generally unused except
	// for tarsum but those versions don't care about mediatype.

	// Don't make duplicate links.
	seenDigests := make(map[digest.Digest]struct{}, len(dgsts))

	// only use the first link
	linkPathFn := lbs.linkPathFns[0]

	for _, dgst := range dgsts {
		if _, seen := seenDigests[dgst]; seen {
			continue
		}
		seenDigests[dgst] = struct{}{}

		blobLinkPath, err := linkPathFn(lbs.pm, lbs.repository.Name(), dgst)
		if err != nil {
			return err
		}

		if err := lbs.blobStore.link(ctx, blobLinkPath, canonical.Digest); err != nil {
			return err
		}
	}

	return nil
}

func (lbs *linkedBlobStore) Enumerate(ctx context.Context, digests []digest.Digest, last string) (n int, err error) {
	context.GetLogger(ctx).Debug("(*linkedBlobStore).Enumerate")
	bufs := make([][]digest.Digest, len(lbs.blobsRootPathFns))

	for i := range lbs.blobsRootPathFns {
		found, err := lbs.enumerateBlobsRootPathFn(ctx, i, last)
		if err != nil {
			return 0, err
		}
		bufs[i] = found
		n += len(found)
	}

	digestsFound := make([]digest.Digest, n)
	copied := 0
	for _, buf := range bufs {
		copied += copy(digestsFound[copied:], buf)
	}
	digestsFound = digestsFound[:copied]
	sort.Sort(byDigestString(digestsFound))
	n = copy(digests, digestsFound)

	if n < len(digests) {
		err = io.EOF
	}

	return
}

// enumerateBlobsRootPathFn returns an array of blobs found at particular blobs
// root returned by path function identified by index. Blobs are walked in lexicographical
// order. All the blobs found before the 'last' offset will be skipped.
func (lbs *linkedBlobStore) enumerateBlobsRootPathFn(ctx context.Context, index int, last string) (results []digest.Digest, err error) {
	var lastPathComponents []string

	if last != "" {
		lastPathComponents, err = digestPathComponents(digest.Digest(last), false)
		if err != nil {
			return nil, fmt.Errorf("invalid last digest %q", last)
		}
	}

	if index < 0 || index >= len(lbs.blobsRootPathFns) {
		return nil, fmt.Errorf("No blobs root path function at index %d", index)
	}

	rootPath, err := lbs.blobsRootPathFns[index](lbs.pm, lbs.repository.Name())
	if err != nil {
		return nil, err
	}

	pastLastOffset := func(pthSepCount int, pth string, lastPathComponents []string) bool {
		maxComponents := pthSepCount + 1
		if maxComponents >= len(lastPathComponents) {
			return pth > path.Join(lastPathComponents...)
		}
		return pth >= path.Join(lastPathComponents[:maxComponents]...)
	}

	err = WalkSorted(ctx, lbs.driver, rootPath, func(fi driver.FileInfo) error {
		if !fi.IsDir() {
			// ignore files
			return nil
		}

		// trim <from>/ prefix
		pth := strings.TrimPrefix(strings.TrimPrefix(fi.Path(), rootPath), "/")
		sepCount := strings.Count(pth, "/")

		if !pastLastOffset(sepCount, pth, lastPathComponents) {
			// we haven't reached the 'last' offset yet
			return ErrSkipDir
		}

		if sepCount == 0 {
			// continue walking
			return nil
		}

		alg := ""
		tarsumParts := reTarsumPrefix.FindStringSubmatch(pth)
		isTarsum := len(tarsumParts) > 0
		if sepCount > 3 || (!isTarsum && sepCount > 1) {
			// too many path components
			return ErrSkipDir
		}

		if len(tarsumParts) > 0 {
			alg = tarsumPrefix + "." + tarsumParts[1] + "+"
			// trim "tarsum/<version>/" prefix from path
			pth = strings.TrimPrefix(pth[len(tarsumParts[0]):], "/")
		}

		digestParts := reDigestPath.FindStringSubmatch(pth)
		if len(digestParts) > 0 {
			alg += digestParts[1]
			dgstHex := digestParts[2]
			dgst := digest.NewDigestFromHex(alg, dgstHex)
			// append only valid digests
			if err := dgst.Validate(); err == nil {
				results = append(results, dgst)
			} else {
				context.GetLogger(ctx).Warnf("skipping invalid digest %q: %v", dgst.String(), err)
			}
			return ErrSkipDir
		}

		return nil
	})

	if err != nil {
		switch err.(type) {
		case driver.PathNotFoundError:
			// ignore
			err = nil
		default:
			if err == ErrStopWalking {
				err = nil
			}
			return nil, err
		}
	}

	return results, nil
}

type linkedBlobStatter struct {
	*blobStore
	repository distribution.Repository

	// linkPathFns specifies one or more path functions allowing one to
	// control the repository blob link set to which the blob store
	// dispatches. This is required because manifest and layer blobs have not
	// yet been fully merged. At some point, this functionality should be
	// removed an the blob links folder should be merged. The first entry is
	// treated as the "canonical" link location and will be used for writes.
	linkPathFns []linkPathFunc

	// Causes directory containing blob's data to be removed recursively upon
	// Clear.
	removeParentsOnDelete bool
}

var _ distribution.BlobDescriptorService = &linkedBlobStatter{}

func (lbs *linkedBlobStatter) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	var (
		resolveErr error
		target     digest.Digest
	)

	// try the many link path functions until we get success or an error that
	// is not PathNotFoundError.
	for _, linkPathFn := range lbs.linkPathFns {
		var err error
		target, err = lbs.resolveWithLinkFunc(ctx, dgst, linkPathFn)

		if err == nil {
			resolveErr = nil
			break // success!
		}

		switch err := err.(type) {
		case driver.PathNotFoundError:
			resolveErr = distribution.ErrBlobUnknown // move to the next linkPathFn, saving the error
		default:
			return distribution.Descriptor{}, err
		}
	}

	if resolveErr != nil {
		return distribution.Descriptor{}, resolveErr
	}

	if target != dgst {
		// Track when we are doing cross-digest domain lookups. ie, tarsum to sha256.
		context.GetLogger(ctx).Warnf("looking up blob with canonical target: %v -> %v", dgst, target)
	}

	// TODO(stevvooe): Look up repository local mediatype and replace that on
	// the returned descriptor.

	return lbs.blobStore.statter.Stat(ctx, target)
}

func (lbs *linkedBlobStatter) Clear(ctx context.Context, dgst digest.Digest) (err error) {
	resolveErr := distribution.ErrBlobUnknown

	// clear any possible existence of a link described in linkPathFns
	for _, linkPathFn := range lbs.linkPathFns {
		blobLinkPath, err := linkPathFn(lbs.pm, lbs.repository.Name(), dgst)
		if err != nil {
			return err
		}

		pth := blobLinkPath
		if lbs.removeParentsOnDelete {
			pth = path.Dir(blobLinkPath)
		}
		err = lbs.blobStore.driver.Delete(ctx, pth)
		if err != nil {
			switch err := err.(type) {
			case driver.PathNotFoundError:
				continue // just ignore this error and continue
			default:
				return err
			}
		}
		resolveErr = nil
	}

	return resolveErr
}

// resolveTargetWithFunc allows us to read a link to a resource with different
// linkPathFuncs to let us try a few different paths before returning not
// found.
func (lbs *linkedBlobStatter) resolveWithLinkFunc(ctx context.Context, dgst digest.Digest, linkPathFn linkPathFunc) (digest.Digest, error) {
	blobLinkPath, err := linkPathFn(lbs.pm, lbs.repository.Name(), dgst)
	if err != nil {
		return "", err
	}

	return lbs.blobStore.readlink(ctx, blobLinkPath)
}

func (lbs *linkedBlobStatter) SetDescriptor(ctx context.Context, dgst digest.Digest, desc distribution.Descriptor) error {
	// The canonical descriptor for a blob is set at the commit phase of upload
	return nil
}

// blobLinkPath provides the path to the blob link, also known as layers.
func blobLinkPath(pm *pathMapper, name string, dgst digest.Digest) (string, error) {
	return pm.path(layerLinkPathSpec{name: name, digest: dgst})
}

// blobsRootPath provides the path to the root of blob links, also known as
// layers.
func blobsRootPath(pm *pathMapper, name string) (string, error) {
	return pm.path(layersPathSpec{name: name})
}

// manifestRevisionLinkPath provides the path to the manifest revision link.
func manifestRevisionLinkPath(pm *pathMapper, name string, dgst digest.Digest) (string, error) {
	return pm.path(manifestRevisionLinkPathSpec{name: name, revision: dgst})
}

// manifestRevisionsPath provides the path to the manifest revisions directory.
func manifestRevisionsPath(pm *pathMapper, name string) (string, error) {
	return pm.path(manifestRevisionsPathSpec{name: name})
}
