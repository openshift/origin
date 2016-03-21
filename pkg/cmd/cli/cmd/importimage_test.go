package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestCreateImageImport(t *testing.T) {
	testCases := []struct {
		name   string
		stream *imageapi.ImageStream
		all    bool
		err    string
		from   []kapi.ObjectReference
	}{
		{
			// 0: checking import's from when only .spec.dockerImageRepository is set, no status
			name: "testis",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			from: []kapi.ObjectReference{
				{
					Kind: "DockerImage",
					Name: "repo.com/somens/someimage",
				},
			},
		},
		{
			// 1: checking import's from when only .spec.dockerImageRepository is set, no status (with all flag set)
			name: "testis",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			all: true,
			from: []kapi.ObjectReference{
				{
					Kind: "DockerImage",
					Name: "repo.com/somens/someimage",
				},
			},
		},
		{
			// 2: with --all flag only .spec.dockerImageRepository is handled
			name: "testis",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
						},
					},
				},
			},
			all: true,
			err: "all is applicable only to images with spec.dockerImageRepository",
		},
		{
			// 3: empty image stream
			name: "testis",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
			},
			err: "image stream has not defined anything to import",
		},
		{
			// 4: correct import of latest tag with tags specified in .spec.Tags
			name: "testis:latest",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
						},
					},
				},
			},
			from: []kapi.ObjectReference{
				{
					Kind: "DockerImage",
					Name: "repo.com/somens/someimage:latest",
				},
			},
		},
		{
			// 5: import latest from image stream which has only tags specified and no latest
			name: "testis:latest",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"nonlatest": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
						},
					},
				},
			},
			err: "does not exist on the image stream",
		},
	}

	for idx, test := range testCases {
		fake := testclient.NewSimpleFake(test.stream)
		o := ImportImageOptions{
			Target:    test.stream.Name,
			All:       test.all,
			Namespace: test.stream.Namespace,
			isClient:  fake.ImageStreams(test.stream.Namespace),
		}
		// we need to run Validate, because it sets appropriate Name and Tag
		if err := o.Validate(&cobra.Command{}); err != nil {
			t.Errorf("(%d) unexpected error: %v", idx, err)
		}

		_, isi, err := o.createImageImport()
		// check errors
		if len(test.err) > 0 {
			if err == nil || !strings.Contains(err.Error(), test.err) {
				t.Errorf("(%d) unexpected error: expected %s, got %v", idx, test.err, err)
			}
			if isi != nil {
				t.Errorf("(%d) unexpected import spec: expected nil, got %#v", idx, isi)
			}
			continue
		}
		if len(test.err) == 0 && err != nil {
			t.Errorf("(%d) unexpected error: %v", idx, err)
			continue
		}
		// check values
		if test.all {
			if !kapi.Semantic.DeepEqual(isi.Spec.Repository.From, test.from[0]) {
				t.Errorf("(%d) unexpected import spec, expected %#v, got %#v", idx, test.from[0], isi.Spec.Repository.From)
			}
		} else {
			if len(isi.Spec.Images) != len(test.from) {
				t.Errorf("(%d) unexpected number of images, expected %d, got %d", idx, len(test.from), len(isi.Spec.Images))
			}
			for i := 0; i < len(test.from); i++ {
				if !kapi.Semantic.DeepEqual(isi.Spec.Images[i].From, test.from[i]) {
					t.Errorf("(%d) unexpected import spec[%d], expected %#v, got %#v", idx, i, test.from[i], isi.Spec.Images[i].From)
				}
			}
		}
	}
}
