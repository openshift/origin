package storage

import (
	"path"
	"regexp"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/storage/driver"
)

var (
	reTarsumVersionPathSuffix = regexp.MustCompile(`/(tarsum)/(\w+)$`)
)

// blobStore implements the read side of the blob store interface over a
// driver without enforcing per-repository membership. This object is
// intentionally a leaky abstraction, providing utility methods that support
// creating and traversing backend links.
type blobStore struct {
	driver  driver.StorageDriver
	statter distribution.BlobDescriptorService
}

var _ distribution.BlobProvider = &blobStore{}
var _ distribution.BlobEnumerator = &blobStore{}

// Get implements the BlobReadService.Get call.
func (bs *blobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	bp, err := bs.path(dgst)
	if err != nil {
		return nil, err
	}

	p, err := bs.driver.GetContent(ctx, bp)
	if err != nil {
		switch err.(type) {
		case driver.PathNotFoundError:
			return nil, distribution.ErrBlobUnknown
		}

		return nil, err
	}

	return p, err
}

func (bs *blobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	desc, err := bs.statter.Stat(ctx, dgst)
	if err != nil {
		return nil, err
	}

	path, err := bs.path(desc.Digest)
	if err != nil {
		return nil, err
	}

	return newFileReader(ctx, bs.driver, path, desc.Size)
}

// Put stores the content p in the blob store, calculating the digest. If the
// content is already present, only the digest will be returned. This should
// only be used for small objects, such as manifests. This implemented as a convenience for other Put implementations
func (bs *blobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	dgst, err := digest.FromBytes(p)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	desc, err := bs.statter.Stat(ctx, dgst)
	if err == nil {
		// content already present
		return desc, nil
	} else if err != distribution.ErrBlobUnknown {
		// real error, return it
		return distribution.Descriptor{}, err
	}

	bp, err := bs.path(dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	// TODO(stevvooe): Write out mediatype here, as well.

	return distribution.Descriptor{
		Size: int64(len(p)),

		// NOTE(stevvooe): The central blob store firewalls media types from
		// other users. The caller should look this up and override the value
		// for the specific repository.
		MediaType: "application/octet-stream",
		Digest:    dgst,
	}, bs.driver.PutContent(ctx, bp, p)
}

// Enumerate returns an array of digests of all blobs in registry's blob store.
// Blob's data may or may not be present.
func (bs *blobStore) Enumerate(ctx context.Context) ([]digest.Digest, error) {
	return enumerateBlobDigests(ctx, bs.driver, path.Join(storagePathRoot, storagePathVersion, "blobs"), true)
}

// enumerateBlobDigests is a utility function for walking blob store under given rootPath.
// If multilevel is true, following layouts are expected:
//
//    <rootPath>/<algorithm>/<first two bytes of digest>/<full digest>
// 	  <rootPath>/tarsum/<version>/<digest algorithm>/<first two bytes of digest>/<full digest>
//
// Otherwise following layouts are presumed:
//
//    <rootPath>/<algorithm>/<full digest>
// 	  <rootPath>/tarsum/<version>/<digest algorithm>/<full digest>
func enumerateBlobDigests(ctx context.Context, d driver.StorageDriver, rootPath string, multilevel bool) ([]digest.Digest, error) {
	var (
		digestPrefix string
		res          []digest.Digest
	)

	algPaths, err := d.List(ctx, rootPath)
	if err != nil {
		switch err := err.(type) {
		case driver.PathNotFoundError:
			return nil, nil
		default:
			return nil, err
		}
	}

	if matches := reTarsumVersionPathSuffix.FindStringSubmatch(rootPath); len(matches) > 1 {
		digestPrefix = matches[1] + "." + matches[2] + "+"
		// Ignore nested tarsum directory
		for i, algPath := range algPaths {
			if path.Base(algPath) == "tarsum" {
				algPaths = append(algPaths[:i], algPaths[i+1:]...)
				context.GetLogger(ctx).Debugf("ignoring tarsum directory while enumerating digests in: %s", rootPath)
				break
			}
		}
	}

	dgstAppend := func(algIndex, dgstPrefCount, dgstPrefIndex, currDgstCount, currDgstIndex int, dgst digest.Digest) {
		rLen := len(res)
		if rLen >= cap(res) {
			// Estimate a number of digests read based on a number of remaining
			// prefixes and algorithms to browse.
			expectDgsts := ((rLen - currDgstIndex + currDgstCount) * dgstPrefCount / (dgstPrefIndex + 1)) * (len(algPaths) - algIndex)
			// Preallocate 125% of digests expected
			prealloc := expectDgsts + expectDgsts/4 + 1
			newArray := make([]digest.Digest, 0, prealloc)
			copy(newArray, res)
			res = newArray
		}
		res = res[:rLen+1]
		res[rLen] = dgst
	}

	enumDigestsIn := func(algorithm string, algIndex, dgstPrefCount, dgstPrefIndex int, root string) error {
		dgstPaths, err := d.List(ctx, root)
		if err != nil {
			switch err := err.(type) {
			case driver.PathNotFoundError:
				return nil
			default:
				return err
			}
		}

		for di, dgstPath := range dgstPaths {
			dgstHex := path.Base(dgstPath)
			dgst := digest.NewDigestFromHex(algorithm, dgstHex)
			if err := dgst.Validate(); err == nil {
				dgstAppend(algIndex, dgstPrefCount, dgstPrefIndex, len(dgstPaths), di, dgst)
			} else {
				context.GetLogger(ctx).Warnf("skipping invalid digest: %s:%s, due to: %v", algorithm, dgstHex, err)
			}
		}
		return err
	}

	for ai, algPath := range algPaths {
		if path.Base(algPath) == "tarsum" {
			tsDgsts, err := enumerateTarsumDigests(ctx, d, algPath, multilevel)
			if err != nil {
				switch err := err.(type) {
				case driver.PathNotFoundError:
					continue
				default:
					return nil, err
				}
			}
			res = append(res, tsDgsts...)
			continue
		}

		algorithm := digestPrefix + path.Base(algPath)
		if multilevel {
			prefPaths, err := d.List(ctx, algPath)
			if err != nil {
				if err != nil {
					switch err := err.(type) {
					case driver.PathNotFoundError:
						continue
					default:
						return nil, err
					}
				}
			}
			for pi, prefPath := range prefPaths {
				if err := enumDigestsIn(algorithm, ai, len(prefPaths), pi, prefPath); err != nil {
					return nil, err
				}
			}
		} else {
			if err := enumDigestsIn(algorithm, ai, 1, 0, algPath); err != nil {
				return nil, err
			}
		}
	}

	return res, nil
}

// enumerateTarsumDigests is a utility function for walking tarsum directory.
// It returns all the digests found.
func enumerateTarsumDigests(ctx context.Context, d driver.StorageDriver, rootPath string, multilevel bool) ([]digest.Digest, error) {
	res := []digest.Digest{}

	versions, err := d.List(ctx, rootPath)
	if err != nil {
		switch err := err.(type) {
		case driver.PathNotFoundError:
			return nil, nil
		default:
			return nil, err
		}
	}

	for _, vPath := range versions {
		dgsts, err := enumerateBlobDigests(ctx, d, vPath, multilevel)
		if err != nil {
			return nil, err
		}
		res = append(res, dgsts...)
	}

	return res, nil
}

// path returns the canonical path for the blob identified by digest. The blob
// may or may not exist.
func (bs *blobStore) path(dgst digest.Digest) (string, error) {
	bp, err := pathFor(blobDataPathSpec{
		digest: dgst,
	})

	if err != nil {
		return "", err
	}

	return bp, nil
}

// link links the path to the provided digest by writing the digest into the
// target file. Caller must ensure that the blob actually exists.
func (bs *blobStore) link(ctx context.Context, path string, dgst digest.Digest) error {
	// The contents of the "link" file are the exact string contents of the
	// digest, which is specified in that package.
	return bs.driver.PutContent(ctx, path, []byte(dgst))
}

// readlink returns the linked digest at path.
func (bs *blobStore) readlink(ctx context.Context, path string) (digest.Digest, error) {
	content, err := bs.driver.GetContent(ctx, path)
	if err != nil {
		return "", err
	}

	linked, err := digest.ParseDigest(string(content))
	if err != nil {
		return "", err
	}

	return linked, nil
}

// resolve reads the digest link at path and returns the blob store path.
func (bs *blobStore) resolve(ctx context.Context, path string) (string, error) {
	dgst, err := bs.readlink(ctx, path)
	if err != nil {
		return "", err
	}

	return bs.path(dgst)
}

type blobStatter struct {
	driver driver.StorageDriver
	// Causes directory containing blob's data to be removed recursively upon
	// Clear.
	removeParentOnDelete bool
}

var _ distribution.BlobDescriptorService = &blobStatter{}

// Stat implements BlobStatter.Stat by returning the descriptor for the blob
// in the main blob store. If this method returns successfully, there is
// strong guarantee that the blob exists and is available.
func (bs *blobStatter) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	path, err := pathFor(blobDataPathSpec{
		digest: dgst,
	})

	if err != nil {
		return distribution.Descriptor{}, err
	}

	fi, err := bs.driver.Stat(ctx, path)
	if err != nil {
		switch err := err.(type) {
		case driver.PathNotFoundError:
			return distribution.Descriptor{}, distribution.ErrBlobUnknown
		default:
			return distribution.Descriptor{}, err
		}
	}

	if fi.IsDir() {
		// NOTE(stevvooe): This represents a corruption situation. Somehow, we
		// calculated a blob path and then detected a directory. We log the
		// error and then error on the side of not knowing about the blob.
		context.GetLogger(ctx).Warnf("blob path should not be a directory: %q", path)
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}

	// TODO(stevvooe): Add method to resolve the mediatype. We can store and
	// cache a "global" media type for the blob, even if a specific repo has a
	// mediatype that overrides the main one.

	return distribution.Descriptor{
		Size: fi.Size(),

		// NOTE(stevvooe): The central blob store firewalls media types from
		// other users. The caller should look this up and override the value
		// for the specific repository.
		MediaType: "application/octet-stream",
		Digest:    dgst,
	}, nil
}

// Clear deletes blob's data with or without its parent directory depending on
// bs' configuration.
func (bs *blobStatter) Clear(ctx context.Context, dgst digest.Digest) error {
	var (
		blobPath string
		err      error
	)

	if bs.removeParentOnDelete {
		blobPath, err = pathFor(blobPathSpec{digest: dgst})
	} else {
		blobPath, err = pathFor(blobDataPathSpec{digest: dgst})
	}
	if err != nil {
		return err
	}

	context.GetLogger(ctx).Infof("Deleting blob: %s", blobPath)
	err = bs.driver.Delete(ctx, blobPath)
	if err != nil {
		return err
	}

	return nil
}

func (bs *blobStatter) SetDescriptor(ctx context.Context, dgst digest.Digest, desc distribution.Descriptor) error {
	return distribution.ErrUnsupported
}

type blobKeeper struct {
	blobStore
	deleteEnabled bool
}

var _ distribution.BlobKeeper = &blobKeeper{}

func (bk *blobKeeper) Delete(ctx context.Context, dgst digest.Digest) error {
	if !bk.deleteEnabled {
		return distribution.ErrUnsupported
	}
	return bk.statter.Clear(ctx, dgst)
}

func (bk *blobKeeper) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	return bk.statter.Stat(ctx, dgst)
}
