package image

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

var fixedTestImageRefs = []string{
	"quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83",
	"quay.io/openshifttest/hello-openshift@sha256:56c354e7885051b6bb4263f9faa58b2c292d44790599b7dde0e49e7c466cf339",
	"docker.io/openpolicyagent/opa:0.69.0",
	"docker.io/openpolicyagent/kube-mgmt:0.8",
	"quay.io/openshifttest/nginx-alpine@sha256:04f316442d48ba60e3ea0b5a67eb89b0b667abf1c198a3d0056ca748736336a0",
	"quay.io/openshifttest/hello-sdn@sha256:c89445416459e7adea9a5a416b3365ed3d74f2491beb904d61dc8d1eb89a72a4",
	"quay.io/redhatproductsecurity/rapidast:2.9.1",
	"quay.io/openshift-release-dev/ocp-release:4.13.0-rc.0-x86_64",
	"registry.redhat.io/ocp-tools-4/jenkins-rhel8:v4.13.0-1686682222",
}

const privateImageRepository = "mirror.example.com:5000/ci/test-images"

func TestMain(m *testing.M) {
	InitializeImages(privateImageRepository)
	os.Exit(m.Run())
}

func TestFixedImageMappings(t *testing.T) {
	original := OriginalImages()
	for _, ref := range fixedTestImageRefs {
		if id, ok := original[ref]; !ok || id != -1 {
			t.Fatalf("OriginalImages()[%q] = (%d, %t), want (-1, true)", ref, id, ok)
		}
	}

	for _, mode := range []struct {
		name string
		repo string
	}{
		{name: "upstream"},
		{name: "private-mirror", repo: privateImageRepository},
	} {
		t.Run(mode.name, func(t *testing.T) {
			mapped := GetMappedImages(original, mode.repo)
			if len(mapped) != len(original) {
				t.Fatalf("mapped %d images, want %d", len(mapped), len(original))
			}
			validMirrorRef := regexp.MustCompile(`^` + regexp.QuoteMeta(mode.repo) + `:e2e-[A-Za-z0-9_][A-Za-z0-9_.-]{0,123}$`)
			seen := make(map[string]string)
			for _, ref := range fixedTestImageRefs {
				t.Run(ref, func(t *testing.T) {
					want, ok := mapped[ref]
					if !ok {
						t.Fatalf("GetMappedImages omitted %q", ref)
					}
					if mode.repo == "" {
						if want != ref {
							t.Errorf("upstream mapping = %q, want %q", want, ref)
						}
					} else if !validMirrorRef.MatchString(want) {
						t.Errorf("mapping = %q, want mirror repository with a single valid tag and no digest", want)
					}
					if previous, exists := seen[want]; exists {
						t.Errorf("%q and %q map to the same image %q", previous, ref, want)
					}
					seen[want] = ref
				})
			}
		})
	}
}

func TestLocationForFixedImages(t *testing.T) {
	mapped := GetMappedImages(OriginalImages(), privateImageRepository)
	for _, ref := range fixedTestImageRefs {
		t.Run(ref, func(t *testing.T) {
			if got, want := LocationFor(ref), mapped[ref]; got != want {
				t.Errorf("LocationFor(%q) = %q, want %q", ref, got, want)
			}
		})
	}
}

func TestReplaceContentsFixedImageFixtures(t *testing.T) {
	fixtures := []struct {
		path       string
		references int
	}{
		{"../compat_otp/testdata/node/pod-with-two-containers.yaml", 2},
		{"../../testdata/apiserverauth/ocp55494-admission-controller.yaml", 2},
		{"../../testdata/apiserver/ocp10873-dc.yaml", 1},
		{"../../testdata/apiserver/aosqe-pod-for-ping.json", 1},
		{"../../testdata/apiserverauth/rapidast-job.yaml", 1},
		{"../../testdata/builds/jenkins-pipeline/jenkins-rhel.yaml", 3},
	}
	// Read source fixtures rather than generated bindata, including ImageStream DockerImage names.
	imageField := regexp.MustCompile(`(?m)^\s*(?:-\s*)?(?:"image"|image|name):\s*['"]?((?:quay\.io|docker\.io|registry\.redhat\.io)/[^'"\s,]+)`)
	original := OriginalImages()
	mapped := GetMappedImages(original, privateImageRepository)
	for _, fixture := range fixtures {
		t.Run(fixture.path, func(t *testing.T) {
			data, err := os.ReadFile(fixture.path)
			if err != nil {
				t.Fatal(err)
			}
			matches := imageField.FindAllSubmatch(data, -1)
			if len(matches) != fixture.references {
				t.Fatalf("found %d image references, want %d", len(matches), fixture.references)
			}
			want := string(data)
			for _, match := range matches {
				ref := string(match[1])
				if _, ok := original[ref]; !ok {
					t.Fatalf("fixture image %q is not in OriginalImages", ref)
				}
				want = strings.ReplaceAll(want, ref, mapped[ref])
			}

			got, err := ReplaceContents(data)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Errorf("ReplaceContents output differs from expected fixture:\ngot:\n%s\nwant:\n%s", got, want)
			}
			for _, match := range matches {
				if bytes.Contains(got, match[1]) {
					t.Errorf("source image %q remains after rewriting", match[1])
				}
			}
		})
	}
}
