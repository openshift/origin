package describe

import (
	"bytes"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

func TestFormatImageStreamTags(t *testing.T) {
	three := int64(3)
	repo := imageapi.ImageStream{
		Spec: imageapi.ImageStreamSpec{
			Tags: map[string]imageapi.TagReference{
				"spec1": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "foo",
						Name:      "bar:latest",
					},
				},
				"spec2": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "mysql",
						Name:      "latest@sha256:e52c6534db85036dabac5e71ff14e720db94def2d90f986f3548425ea27b3719",
					},
				},
				"spec3": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "mysql",
					},
					ImportPolicy: imageapi.TagImportPolicy{
						Scheduled: true,
					},
					Generation:      &three,
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.LocalTagReferencePolicy},
				},
				"spec4": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "mysql:2",
					},
					Reference: true,
				},
				"spec5": {},
			},
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				imageapi.DefaultImageTag: {
					Items: []imageapi.TagEvent{
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
				"spec1": {
					Items: []imageapi.TagEvent{
						{
							Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "registry:5000/foo/bar@sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
							Image:                "sha256:4bd26aef1ce78b4f05ede83496276f11e3343441574ca1ce89dffd146c708c16",
						},
					},
				},
				"spec2": {
					Items: []imageapi.TagEvent{
						{
							Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: "mysql:latest",
							Image:                "sha256:e52c6534db85036dabac5e71ff14e720db94def2d90f986f3548425ea27b3719",
						},
					},
				},
				"spec3": {
					Conditions: []imageapi.TagEventCondition{
						{
							Type:    imageapi.ImportSuccess,
							Status:  kapi.ConditionFalse,
							Reason:  "NotFound",
							Message: "Image not found due to error",
						},
					},
					Items: []imageapi.TagEvent{
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
