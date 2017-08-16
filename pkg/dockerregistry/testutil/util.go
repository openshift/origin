package testutil

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	mrand "math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
	distclient "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

// UploadBlob uploads a blob with payload to the registry server located at
// serverURL.
func UploadBlob(
	payload []byte,
	serverURL *url.URL,
	creds auth.CredentialStore,
	repoName string,
) (distribution.Descriptor, error) {
	// TODO(dmage): get the context from the caller
	ctx := context.Background()

	ref, err := reference.ParseNamed(repoName)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	var rt http.RoundTripper
	if creds != nil {
		challengeManager := auth.NewSimpleChallengeManager()
		_, err := ping(challengeManager, serverURL.String()+"/v2/", "")
		if err != nil {
			return distribution.Descriptor{}, err
		}
		rt = transport.NewTransport(
			nil,
			auth.NewAuthorizer(
				challengeManager,
				auth.NewTokenHandler(nil, creds, repoName, "pull", "push"),
				auth.NewBasicHandler(creds)))
	}

	repo, err := distclient.NewRepository(ctx, ref, serverURL.String(), rt)
	if err != nil {
		return distribution.Descriptor{}, fmt.Errorf("failed to get repository %q: %v", repoName, err)
	}

	wr, err := repo.Blobs(ctx).Create(ctx)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	_, err = io.Copy(wr, bytes.NewReader(payload))
	if err != nil {
		return distribution.Descriptor{}, fmt.Errorf("unexpected error copying to upload: %v", err)
	}

	return wr.Commit(ctx, distribution.Descriptor{
		Digest: digest.FromBytes(payload),
	})
}

// UploadRandomTestBlob generates a random tar file and uploads it to the given repository.
func UploadRandomTestBlob(serverURL *url.URL, creds auth.CredentialStore, repoName string) (distribution.Descriptor, []byte, error) {
	payload, err := CreateRandomTarFile()
	if err != nil {
		return distribution.Descriptor{}, nil, fmt.Errorf("unexpected error generating test layer file: %v", err)
	}

	desc, err := UploadBlob(payload, serverURL, creds, repoName)
	if err != nil {
		return distribution.Descriptor{}, nil, fmt.Errorf("upload random test blob: %s", err)
	}

	return desc, payload, nil
}

// CreateRandomTarFile creates a random tarfile and returns its content.
// An error is returned if there is a problem generating valid content.
// Inspired by github.com/vendor/docker/distribution/testutil/tarfile.go.
func CreateRandomTarFile() ([]byte, error) {
	nFiles := 2

	var target bytes.Buffer
	wr := tar.NewWriter(&target)

	// Perturb this on each iteration of the loop below.
	header := &tar.Header{
		Mode:       0644,
		ModTime:    time.Now(),
		Typeflag:   tar.TypeReg,
		Uname:      "randocalrissian",
		Gname:      "cloudcity",
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	}

	for fileNumber := 0; fileNumber < nFiles; fileNumber++ {
		header.Name = fmt.Sprint(fileNumber)
		header.Size = mrand.Int63n(1<<9) + 1<<9

		err := wr.WriteHeader(header)
		if err != nil {
			return nil, err
		}

		randomData := make([]byte, header.Size)
		_, err = rand.Read(randomData)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(wr, bytes.NewReader(randomData))
		if err != nil {
			return nil, err
		}
	}

	if err := wr.Close(); err != nil {
		return nil, err
	}

	return target.Bytes(), nil
}

// CreateRandomImage creates an image with a random content.
func CreateRandomImage(namespace, name string) (*imageapiv1.Image, error) {
	_, manifest, _, err := CreateRandomManifest(ManifestSchema1, 3)
	if err != nil {
		return nil, err
	}

	_, manifestSchema1, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	image, err := NewImageForManifest(
		fmt.Sprintf("%s/%s", namespace, name),
		string(manifestSchema1),
		"",
		false,
	)
	if err != nil {
		return nil, err
	}

	return image, nil
}

const SampleImageManifestSchema1 = `{
   "schemaVersion": 1,
   "name": "nm/is",
   "tag": "latest",
   "architecture": "",
   "fsLayers": [
      {
         "blobSum": "sha256:b2c5513bd934a7efb412c0dd965600b8cb00575b585eaff1cb980b69037fe6cd"
      },
      {
         "blobSum": "sha256:2dde6f11a89463bf20dba3b47d8b3b6de7cdcc19e50634e95a18dd95c278768d"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"size\":18407936}"
      },
      {
         "v1Compatibility": "{\"size\":19387392}"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "5HTY:A24B:L6PG:TQ3G:GMAK:QGKZ:ICD4:S7ZJ:P5JX:UTMP:XZLK:ZXVH",
               "kty": "EC",
               "x": "j5YnDSyrVIt3NquUKvcZIpbfeD8HLZ7BVBFL4WutRBM",
               "y": "PBgFAZ3nNakYN3H9enhrdUrQ_HPYzb8oX5rtJxJo1Y8"
            },
            "alg": "ES256"
         },
         "signature": "1rXiEmWnf9eL7m7Wy3K4l25-Zv2XXl5GgqhM_yjT0ujPmTn0uwfHcCWlweHa9gput3sECj507eQyGpBOF5rD6Q",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjQ4NSwiZm9ybWF0VGFpbCI6IkNuMCIsInRpbWUiOiIyMDE2LTA3LTI2VDExOjQ2OjQ2WiJ9"
      }
   ]
}`

type testCredentialStore struct {
	username      string
	password      string
	refreshTokens map[string]string
}

var _ auth.CredentialStore = &testCredentialStore{}

// NewBasicCredentialStore returns a test credential store for use with registry token handler and/or basic
// handler.
func NewBasicCredentialStore(username, password string) auth.CredentialStore {
	return &testCredentialStore{
		username: username,
		password: password,
	}
}

func (tcs *testCredentialStore) Basic(*url.URL) (string, string) {
	return tcs.username, tcs.password
}

func (tcs *testCredentialStore) RefreshToken(u *url.URL, service string) string {
	return tcs.refreshTokens[service]
}

func (tcs *testCredentialStore) SetRefreshToken(u *url.URL, service string, token string) {
	if tcs.refreshTokens != nil {
		tcs.refreshTokens[service] = token
	}
}

// ping pings the provided endpoint to determine its required authorization challenges.
// If a version header is provided, the versions will be returned.
func ping(manager auth.ChallengeManager, endpoint, versionHeader string) ([]auth.APIVersion, error) {
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := manager.AddResponse(resp); err != nil {
		return nil, err
	}

	return auth.APIVersions(resp, versionHeader), nil
}
