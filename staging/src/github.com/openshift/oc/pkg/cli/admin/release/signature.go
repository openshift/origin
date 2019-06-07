package release

import (
	"encoding/json"
	"fmt"
	"time"
)

// createReleaseSignatureMessage creates the core message to sign the release payload.
func createReleaseSignatureMessage(signer string, now time.Time, releaseDigest, pullSpec string) ([]byte, error) {
	if len(signer) == 0 || now.IsZero() || len(releaseDigest) == 0 || len(pullSpec) == 0 {
		return nil, fmt.Errorf("you must specify a signer, current timestamp, release digest, and pull spec to sign")
	}

	sig := &signature{
		Critical: criticalSignature{
			Type: "atomic container signature",
			Image: criticalImage{
				DockerManifestDigest: releaseDigest,
			},
			Identity: criticalIdentity{
				DockerReference: pullSpec,
			},
		},
		Optional: optionalSignature{
			Creator:   signer,
			Timestamp: now.Unix(),
		},
	}
	return json.MarshalIndent(sig, "", "  ")
}

// An atomic container signature has the following schema:
//
// {
// 	"critical": {
// 			"type": "atomic container signature",
// 			"image": {
// 					"docker-manifest-digest": "sha256:817a12c32a39bbe394944ba49de563e085f1d3c5266eb8e9723256bc4448680e"
// 			},
// 			"identity": {
// 					"docker-reference": "docker.io/library/busybox:latest"
// 			}
// 	},
// 	"optional": {
// 			"creator": "some software package v1.0.1-35",
// 			"timestamp": 1483228800,
// 	}
// }
type signature struct {
	Critical criticalSignature `json:"critical"`
	Optional optionalSignature `json:"optional"`
}

type criticalSignature struct {
	Type     string           `json:"type"`
	Image    criticalImage    `json:"image"`
	Identity criticalIdentity `json:"identity"`
}

type criticalImage struct {
	DockerManifestDigest string `json:"docker-manifest-digest"`
}

type criticalIdentity struct {
	DockerReference string `json:"docker-reference"`
}

type optionalSignature struct {
	Creator   string `json:"creator"`
	Timestamp int64  `json:"timestamp"`
}
