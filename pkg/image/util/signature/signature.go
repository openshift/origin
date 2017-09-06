package signature

import (
	"context"
	"crypto/sha256"
	"fmt"

	docker "github.com/containers/image/docker"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// GetSignatureForImage pulls down image signatures from remote or local signature store.
// See this document for more details about how to configure the signature stores:
// https://github.com/containers/image/blob/master/docs/registries.d.md
func GetSignatureForImage(ctx context.Context, registryHost, repositoryName, imageName string) ([]imageapi.ImageSignature, error) {
	reference, err := docker.ParseReference("//" + registryHost + "/" + repositoryName + "@" + imageName)
	if err != nil {
		return nil, err
	}
	source, err := reference.NewImageSource(nil, nil)
	if err != nil {
		return nil, err
	}
	defer source.Close()

	signatures, err := source.GetSignatures(ctx)
	if err != nil {
		return nil, err
	}
	ret := []imageapi.ImageSignature{}
	for _, blob := range signatures {
		sig := imageapi.ImageSignature{Type: imageapi.ImageSignatureTypeAtomicImageV1}
		// This will use the name of the image (sha256:xxxx) and the SHA256 of the signature itself as the signature
		// name has to be unique for each signature.
		sig.Name = imageName + "@" + fmt.Sprintf("%x", sha256.Sum256(blob))
		sig.Content = blob
		ret = append(ret, sig)
	}
	return ret, nil
}
