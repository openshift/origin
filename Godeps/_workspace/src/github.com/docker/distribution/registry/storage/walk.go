package storage

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	storageDriver "github.com/docker/distribution/registry/storage/driver"
)

const (
	enumBufLength = 2048
)

// SkipDir is used as a return value from onFileFunc to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
var ErrSkipDir = errors.New("skip this directory")

// ErrStopWalking is used as a return value from onFileFunc to indicate that
// Walk should terminate without an error. If used, Walk will terminate with
// this error.
var ErrStopWalking = errors.New("stop walking")

// WalkFn is called once per file by Walk
// If the returned error is ErrSkipDir and fileInfo refers
// to a directory, the directory will not be entered and Walk
// will continue the traversal.  Otherwise Walk will return
type WalkFn func(fileInfo storageDriver.FileInfo) error

// Walk traverses a filesystem defined within driver, starting
// from the given path, calling f on each file
func Walk(ctx context.Context, driver storageDriver.StorageDriver, from string, f WalkFn) error {
	children, err := driver.List(ctx, from)
	if err != nil {
		return err
	}
	for _, child := range children {
		fileInfo, err := driver.Stat(ctx, child)
		if err != nil {
			return err
		}
		switch f(fileInfo) {
		case ErrSkipDir:
			continue
		case ErrStopWalking:
			return ErrStopWalking
		case nil:
			if fileInfo.IsDir() {
				if err := Walk(ctx, driver, child, f); err == ErrStopWalking {
					return err
				}
			}
		default:
			return err
		}
	}
	return nil
}

// WalkSorted traverses a filesystem defined within driver, starting
// from the given path, calling f on each file. Directories are walked
// in lexicographical order.
func WalkSorted(ctx context.Context, driver storageDriver.StorageDriver, from string, f WalkFn) error {
	children, err := driver.List(ctx, from)
	if err != nil {
		return err
	}
	sort.Strings(children)
	for _, child := range children {
		fileInfo, err := driver.Stat(ctx, child)
		if err != nil {
			return err
		}
		switch f(fileInfo) {
		case ErrSkipDir:
			continue
		case ErrStopWalking:
			return ErrStopWalking
		case nil:
			if fileInfo.IsDir() {
				if err := WalkSorted(ctx, driver, child, f); err == ErrStopWalking {
					return err
				}
			}
		default:
			return err
		}
	}
	return nil
}

// pushError formats an error type given a path and an error
// and pushes it to a slice of errors
func pushError(errors []error, path string, err error) []error {
	return append(errors, fmt.Errorf("%s: %s", path, err))
}

// EnumerateAllBlobs is a utility function that returns all the blob digests
// found in given blob store. It should be used with care because of memory and
// time complexity.
func EnumerateAllBlobs(be distribution.BlobEnumerator, ctx context.Context) ([]digest.Digest, error) {
	// prepare an array of equally-sized buffers to collect all the digests
	// TODO: set a limit to bufs size?
	bufs := make([][]digest.Digest, 0, 1)
	total := 0
	last := ""

	for {
		buf := make([]digest.Digest, enumBufLength)
		n, err := be.Enumerate(ctx, buf, last)
		total += n
		if n > 0 {
			bufs = append(bufs, buf)
			last = buf[n-1].String()
		}
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}
	}

	// join all buffers
	res := make([]digest.Digest, total)
	for i := range bufs {
		if i < len(bufs)-1 {
			copy(res[enumBufLength*i:], bufs[i])
		} else {
			copy(res[enumBufLength*i:], bufs[i][:total-enumBufLength*i])
		}
	}

	return res, nil
}
