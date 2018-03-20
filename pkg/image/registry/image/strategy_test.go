package image

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/testing/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapitesting "k8s.io/kubernetes/pkg/api/testing"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func fuzzImage(t *testing.T, image *imageapi.Image, seed int64) *imageapi.Image {
	f := fuzzer.FuzzerFor(kapitesting.FuzzerFuncs, rand.NewSource(seed), legacyscheme.Codecs)
	f.Funcs(
		func(j *imageapi.Image, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.Annotations = make(map[string]string)
			j.Labels = make(map[string]string)
			j.Signatures = make([]imageapi.ImageSignature, c.Rand.Intn(3)+2)
			for i := range j.Signatures {
				sign := &j.Signatures[i]
				c.Fuzz(sign)
				sign.Conditions = make([]imageapi.SignatureCondition, c.Rand.Intn(3)+2)
				for ci := range sign.Conditions {
					cond := &sign.Conditions[ci]
					c.Fuzz(cond)
				}
			}
			for i := 0; i < c.Rand.Intn(3)+2; i++ {
				j.Labels[c.RandString()] = c.RandString()
				j.Annotations[c.RandString()] = c.RandString()
			}
		},
	)

	updated := imageapi.Image{}
	f.Fuzz(&updated)
	updated.Namespace = image.Namespace
	updated.Name = image.Name

	j, err := meta.TypeAccessor(image)
	if err != nil {
		t.Fatalf("Unexpected error %v for %#v", err, image)
	}
	j.SetKind("")
	j.SetAPIVersion("")

	return &updated
}

func TestStrategyPrepareForCreate(t *testing.T) {
	ctx := apirequest.NewDefaultContext()

	original := imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image",
		},
	}

	seed := int64(2703387474910584091) //rand.Int63()
	fuzzed := fuzzImage(t, &original, seed)
	image := fuzzed.DeepCopy()

	if len(image.Signatures) == 0 {
		t.Fatalf("fuzzifier failed to generate signatures")
	}

	Strategy.PrepareForCreate(ctx, image)

	testVerifySignatures(t, fuzzed, image)
}

func testVerifySignatures(t *testing.T, orig, new *imageapi.Image) {
	if len(new.Signatures) != len(orig.Signatures) {
		t.Errorf("unexpected number of signatures: %d != %d", len(new.Signatures), len(orig.Signatures))
	}

	for i, sig := range new.Signatures {
		// expect annotations to be cleared
		delete(orig.Signatures[i].Annotations, managedSignatureAnnotation)

		vi := reflect.ValueOf(&sig).Elem()
		vf := reflect.ValueOf(&orig.Signatures[i]).Elem()
		typeOfT := vf.Type()

		for j := 0; j < vf.NumField(); j++ {
			iField := vi.Field(j)
			fField := vf.Field(j)

			switch typeOfT.Field(j).Name {
			case "Content", "Type", "TypeMeta", "ObjectMeta":
				if !reflect.DeepEqual(iField.Interface(), fField.Interface()) {
					t.Errorf("%s field should not differ: %s", typeOfT.Field(j).Name, diff.ObjectGoPrintDiff(iField.Interface(), fField.Interface()))
				}
			}
		}
	}
}

func TestStrategyPrepareForCreateSignature(t *testing.T) {
	ctx := apirequest.NewDefaultContext()

	original := imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image",
		},
	}

	seed := int64(2703387474910584091) //rand.Int63()
	fuzzed := fuzzImage(t, &original, seed)

	if len(fuzzed.Signatures) == 0 {
		t.Fatalf("fuzzifier failed to generate signatures")
	}

	for _, tc := range []struct {
		name        string
		annotations map[string]string
		expected    map[string]string
	}{
		{
			name:        "unset annotations",
			annotations: nil,
			expected:    nil,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    map[string]string{},
		},
		{
			name:        "managed annotation shall be removed",
			annotations: map[string]string{managedSignatureAnnotation: "value"},
			expected:    map[string]string{},
		},
		{
			name:        "other annotations shall stay",
			annotations: map[string]string{"key": "value"},
			expected:    map[string]string{"key": "value"},
		},
		{
			name:        "remove and keep",
			annotations: map[string]string{"key": "value", managedSignatureAnnotation: "true"},
			expected:    map[string]string{"key": "value"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fuzzed.Signatures[0].Annotations = tc.annotations
			image := fuzzed.DeepCopy()

			Strategy.PrepareForCreate(ctx, image)

			testVerifySignatures(t, fuzzed, image)

			if !reflect.DeepEqual(image.Signatures[0].Annotations, tc.expected) {
				t.Errorf("unexpected signature annotations: %s", diff.ObjectGoPrintDiff(image.Annotations, tc.expected))
			}
		})
	}
}
