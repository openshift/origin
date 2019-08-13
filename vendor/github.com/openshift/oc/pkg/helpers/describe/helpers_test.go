package describe

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/openshift/api/image/v1"
	imagehelpers "github.com/openshift/oc/pkg/helpers/image"
)

func TestFormatImageStreamTags(t *testing.T) {
	three := int64(3)
	repo := imagev1.ImageStream{
		Spec: imagev1.ImageStreamSpec{
			Tags: []imagev1.TagReference{
				{
					Name: "spec1",
					From: &corev1.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "foo",
						Name:      "bar:latest",
					},
				},
				{
					Name: "spec2",
					From: &corev1.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "mysql",
						Name:      "latest@sha256:e52c6534db85036dabac5e71ff14e720db94def2d90f986f3548425ea27b3719",
					},
				},
				{
					Name: "spec3",
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "mysql",
					},
					ImportPolicy: imagev1.TagImportPolicy{
						Scheduled: true,
					},
					Generation:      &three,
					ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.LocalTagReferencePolicy},
				},
				{
					Name: "spec4",
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "mysql:2",
					},
					Reference: true,
				},
				{
					Name: "spec5",
				},
			},
		},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{
				{
					Tag: "default",
					Items: []imagev1.TagEvent{
						{
							Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "registry:5000/foo/bar@sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
							Image:                "sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
						},
						{
							Created:              metav1.Date(2015, 3, 23, 7, 15, 0, 0, time.UTC),
							DockerImageReference: "registry:5000/foo/bar@sha256:062b80555a5dd7f5d58e78b266785a399277ff8c3e402ce5fa5d8571788e6bad",
							Image:                "sha256:062b80555a5dd7f5d58e78b266785a399277ff8c3e402ce5fa5d8571788e6bad",
						},
					},
				},
				{
					Tag: "spec1",
					Items: []imagev1.TagEvent{
						{
							Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "registry:5000/foo/bar@sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
							Image:                "sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
						},
					},
				},
				{
					Tag: "spec2",
					Items: []imagev1.TagEvent{
						{
							Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "mysql:latest",
							Image:                "sha256:e52c6534db85036dabac5e71ff14e720db94def2d90f986f3548425ea27b3719",
						},
					},
				},
				{
					Tag: "spec3",
					Conditions: []imagev1.TagEventCondition{
						{
							Type:    imagev1.ImportSuccess,
							Status:  corev1.ConditionFalse,
							Reason:  "NotFound",
							Message: "Image not found due to error",
						},
					},
					Items: []imagev1.TagEvent{
						{
							Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "mysql:latest",
							Image:                "sha256:e52c6534db85036dabac5e71ff14e720db94def2d90f986f3548425ea27b3719",
							Generation:           2,
						},
					},
				},
			},
		},
	}

	out := new(tabwriter.Writer)
	b := make([]byte, 1024)
	buf := bytes.NewBuffer(b)
	out.Init(buf, 0, 8, 1, '\t', 0)

	formatImageStreamTags(out, &repo)
	out.Flush()
	actual := string(buf.String())
	t.Logf("\n%s", actual)

	for _, s := range []string{
		"no spec tag",
		"tag without source image",
		"Unique Images:\t3",
		"Tags:\t\t6",
		"* registry:5000/foo/bar@sha256:4bd26",
		"registry:5000/foo/bar@sha256:062b80",
		"tagged from foo/bar:latest",
		"tagged from mysql/latest@sha256:e52c65",
		"updates automatically from registry mysql",
		"reference to registry mysql:2",
		"prefer registry pullthrough when referencing this tag",
		"~ importing latest image ...",
		"! error: Import failed (NotFound): Image not found due to error",
	} {
		if !strings.Contains(actual, s) {
			t.Errorf("expected %s in:\n%s", s, actual)
		}
	}
}

func TestFollowTagReference(t *testing.T) {
	tests := map[string]struct {
		stream      *imagev1.ImageStream
		tag         string
		expFinalTag string
		expRef      *imagev1.TagReference
		expMultiple bool
		expErr      error
	}{
		"follow tag reference": {
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{Name: "mytag", From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "sometag"}},
						{Name: "sometag", From: &corev1.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "sometag",
			expRef: &imagev1.TagReference{
				Name: "sometag",
				From: &corev1.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"},
			},
			expMultiple: true,
			expErr:      nil,
		},
		"follow tag reference with istag:mytag format": {
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{Name: "mytag", From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "testis:sometag"}},
						{Name: "sometag", From: &corev1.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "sometag",
			expRef: &imagev1.TagReference{
				Name: "sometag",
				From: &corev1.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"},
			},
			expMultiple: true,
			expErr:      nil,
		},
		"no tag reference error": {
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{Name: "mytag", From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "correcttag"}},
						{Name: "wrongtag", From: &corev1.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "correcttag",
			expRef:      nil,
			expMultiple: true,
			expErr:      imagehelpers.ErrNotFoundReference,
		},
		"crosss image tag reference error": {
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{Name: "mytag", From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "another:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "mytag",
			expRef:      nil,
			expMultiple: false,
			expErr:      imagehelpers.ErrCrossImageStreamReference,
		},
		"crosss namespace tag reference error": {
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "thisns",
					Name:      "thisis",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{Name: "mytag", From: &corev1.ObjectReference{Kind: "ImageStreamTag", Namespace: "anotherns", Name: "thisis:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "mytag",
			expRef:      nil,
			expMultiple: false,
			expErr:      imagehelpers.ErrCrossImageStreamReference,
		},
		"circular tag reference error": {
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{Name: "mytag", From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "sometag"}},
						{Name: "sometag", From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "mytag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "mytag",
			expRef:      nil,
			expMultiple: true,
			expErr:      imagehelpers.ErrCircularReference,
		},
	}

	for name, tc := range tests {
		finalTag, ref, multiple, err := imagehelpers.FollowTagReference(tc.stream, tc.tag)
		if !reflect.DeepEqual(finalTag, tc.expFinalTag) {
			t.Errorf("%s: got %v, want %v", name, finalTag, tc.expFinalTag)
		}
		if !reflect.DeepEqual(ref, tc.expRef) {
			t.Errorf("%s: got %#v, want %#v", name, ref, tc.expRef)
		}
		if !reflect.DeepEqual(multiple, tc.expMultiple) {
			t.Errorf("%s: got %v, want %v", name, multiple, tc.expMultiple)
		}
		if err != tc.expErr {
			t.Errorf("%s: got %v, want %v", name, err, tc.expErr)
		}
	}
}
