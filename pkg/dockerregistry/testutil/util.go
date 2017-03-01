package testutil

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
	distclient "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// UploadTestBlobFromReader uploads a testing blob read from the given reader to the registry located at the
// given URL.
func UploadTestBlobFromReader(
	dgst digest.Digest,
	reader io.ReadSeeker,
	serverURL *url.URL,
	creds auth.CredentialStore,
	repoName string,
) (distribution.Descriptor, []byte, error) {
	ctx := context.Background()
	ref, err := reference.ParseNamed(repoName)
	if err != nil {
		return distribution.Descriptor{}, nil, err
	}

	var rt http.RoundTripper
	if creds != nil {
		challengeManager := auth.NewSimpleChallengeManager()
		_, err := ping(challengeManager, serverURL.String()+"/v2/", "")
		if err != nil {
			return distribution.Descriptor{}, nil, err
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
		return distribution.Descriptor{}, nil, fmt.Errorf("failed to get repository %q: %v", repoName, err)
	}

	wr, err := repo.Blobs(ctx).Create(ctx)
	if err != nil {
		return distribution.Descriptor{}, nil, err
	}
	if _, err := io.Copy(wr, reader); err != nil {
		return distribution.Descriptor{}, nil, fmt.Errorf("unexpected error copying to upload: %v", err)
	}
	desc, err := wr.Commit(ctx, distribution.Descriptor{Digest: dgst})
	if err != nil {
		return distribution.Descriptor{}, nil, err
	}

	if _, err := reader.Seek(0, 0); err != nil {
		return distribution.Descriptor{}, nil, fmt.Errorf("failed to seak blob reader: %v", err)
	}
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return distribution.Descriptor{}, nil, fmt.Errorf("failed to read blob content: %v", err)
	}

	return desc, content, nil
}

// UploadPayloadAsBlob uploads a given payload to the registry serving at the given URL.
func UploadPayloadAsBlob(
	payload []byte,
	serverURL *url.URL,
	creds auth.CredentialStore,
	repoName string,
) (distribution.Descriptor, error) {
	reader := bytes.NewReader(payload)
	dgst := digest.FromBytes(payload)
	desc, _, err := UploadTestBlobFromReader(dgst, reader, serverURL, creds, repoName)
	return desc, err
}

// UploadRandomTestBlob generates a random tar file and uploads it to the given repository.
func UploadRandomTestBlob(
	serverURL *url.URL,
	creds auth.CredentialStore,
	repoName string,
) (distribution.Descriptor, []byte, error) {
	rs, ds, err := CreateRandomTarFile()
	if err != nil {
		return distribution.Descriptor{}, nil, fmt.Errorf("unexpected error generating test layer file: %v", err)
	}
	dgst := digest.Digest(ds)
	return UploadTestBlobFromReader(dgst, rs, serverURL, creds, repoName)
}

// createRandomTarFile creates a random tarfile, returning it as an io.ReadSeeker along with its digest. An
// error is returned if there is a problem generating valid content. Inspired by
// github.com/vendor/docker/distribution/testutil/tarfile.go.
func CreateRandomTarFile() (rs io.ReadSeeker, dgst digest.Digest, err error) {
	nFiles := 2
	target := &bytes.Buffer{}
	wr := tar.NewWriter(target)

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
		fileSize := mrand.Int63n(1<<9) + 1<<9

		header.Name = fmt.Sprint(fileNumber)
		header.Size = fileSize

		if err := wr.WriteHeader(header); err != nil {
			return nil, "", err
		}

		randomData := make([]byte, fileSize)

		// Fill up the buffer with some random data.
		n, err := rand.Read(randomData)

		if n != len(randomData) {
			return nil, "", fmt.Errorf("short read creating random reader: %v bytes != %v bytes", n, len(randomData))
		}

		if err != nil {
			return nil, "", err
		}

		nn, err := io.Copy(wr, bytes.NewReader(randomData))
		if nn != fileSize {
			return nil, "", fmt.Errorf("short copy writing random file to tar")
		}

		if err != nil {
			return nil, "", err
		}

		if err := wr.Flush(); err != nil {
			return nil, "", err
		}
	}

	if err := wr.Close(); err != nil {
		return nil, "", err
	}

	dgst = digest.FromBytes(target.Bytes())

	return bytes.NewReader(target.Bytes()), dgst, nil
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

// GetFakeImageGetHandler returns a reaction function for use with fake os client returning one of given image
// objects if found.
func GetFakeImageGetHandler(t *testing.T, imgs ...imageapi.Image) core.ReactionFunc {
	return func(action core.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case core.GetAction:
			for _, is := range imgs {
				if a.GetName() == is.Name {
					t.Logf("images get handler: returning image %s", is.Name)
					return true, &is, nil
				}
			}

			err := kerrors.NewNotFound(kapi.Resource("images"), a.GetName())
			t.Logf("image get handler: %v", err)
			return true, nil, err
		}
		return false, nil, nil
	}
}

// GetFakeImageStreamGetHandler creates a test handler to be used as a reactor with core.Fake client
// that handles Get request on image stream resource. Matching is from given image stream list will be
// returned if found. Additionally, a shared image stream may be requested.
func GetFakeImageStreamGetHandler(t *testing.T, iss ...imageapi.ImageStream) core.ReactionFunc {
	return func(action core.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case core.GetAction:
			for _, is := range iss {
				if is.Namespace == a.GetNamespace() && a.GetName() == is.Name {
					t.Logf("imagestream get handler: returning image stream %s/%s", is.Namespace, is.Name)
					return true, &is, nil
				}
			}

			err := kerrors.NewNotFound(kapi.Resource("imageStreams"), a.GetName())
			t.Logf("imagestream get handler: %v", err)
			return true, nil, err
		}
		return false, nil, nil
	}
}

// TestNewImageStreamObject returns a new image stream object filled with given values.
func TestNewImageStreamObject(namespace, name, tag, imageName, dockerImageReference string) *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				tag: {
					Items: []imageapi.TagEvent{
						{
							Image:                imageName,
							DockerImageReference: dockerImageReference,
						},
					},
				},
			},
		},
	}
}

// GetFakeImageStreamImageGetHandler returns a reaction function for use
// with fake os client returning one of given imagestream image objects if found.
func GetFakeImageStreamImageGetHandler(t *testing.T, iss *imageapi.ImageStream, imgs ...imageapi.Image) core.ReactionFunc {
	return func(action core.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case core.GetAction:
			for _, is := range imgs {
				name, imageID, err := imageapi.ParseImageStreamImageName(a.GetName())
				if err != nil {
					return true, nil, err
				}

				if imageID != is.Name {
					continue
				}

				t.Logf("imagestreamimage get handler: returning image %s", is.Name)

				isi := imageapi.ImageStreamImage{
					ObjectMeta: kapi.ObjectMeta{
						Namespace:         is.Namespace,
						Name:              imageapi.MakeImageStreamImageName(name, imageID),
						CreationTimestamp: is.ObjectMeta.CreationTimestamp,
						Annotations:       iss.Annotations,
					},
					Image: is,
				}

				return true, &isi, nil
			}

			err := kerrors.NewNotFound(kapi.Resource("imagestreamimages"), a.GetName())
			t.Logf("imagestreamimage get handler: %v", err)
			return true, nil, err
		}
		return false, nil, nil
	}
}

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

	return auth.APIVersions(resp, versionHeader), err
}
