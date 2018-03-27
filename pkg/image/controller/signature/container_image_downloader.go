package signature

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/containers/image/docker"
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type containerImageSignatureDownloader struct {
	ctx     context.Context
	timeout time.Duration
}

func NewContainerImageSignatureDownloader(ctx context.Context, timeout time.Duration) SignatureDownloader {
	return &containerImageSignatureDownloader{
		ctx:     ctx,
		timeout: timeout,
	}
}

type GetSignaturesError struct {
	error
}

func (s *containerImageSignatureDownloader) DownloadImageSignatures(image *imageapi.Image) ([]imageapi.ImageSignature, error) {
	reference, err := docker.ParseReference("//" + image.DockerImageReference)
	if err != nil {
		return nil, err
	}
	source, err := reference.NewImageSource(nil, nil)
	if err != nil {
		// In case we fail to talk to registry to get the image metadata (private
		// registry, internal registry, etc...), do not fail with error to avoid
		// spamming logs.
		glog.V(4).Infof("Failed to get %q: %v", image.DockerImageReference, err)
		return []imageapi.ImageSignature{}, nil
	}
	defer source.Close()

	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()

	signatures, err := source.GetSignatures(ctx)
	if err != nil {
		glog.V(4).Infof("Failed to get signatures for %v due to: %v", source.Reference(), err)
		return []imageapi.ImageSignature{}, GetSignaturesError{err}
	}

	ret := []imageapi.ImageSignature{}
	for _, blob := range signatures {
		sig := imageapi.ImageSignature{Type: imageapi.ImageSignatureTypeAtomicImageV1}
		// This will use the name of the image (sha256:xxxx) and the SHA256 of the
		// signature itself as the signature name has to be unique for each
		// signature.
		sig.Name = imageapi.JoinImageStreamImage(image.Name, fmt.Sprintf("%x", sha256.Sum256(blob)))
		sig.Content = blob
		sig.CreationTimestamp = metav1.Now()
		ret = append(ret, sig)
	}
	return ret, nil
}
