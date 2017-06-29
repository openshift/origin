package image

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	"k8s.io/apimachinery/pkg/api/meta"
	apitesting "k8s.io/apimachinery/pkg/api/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func fuzzImage(t *testing.T, image *imageapi.Image, seed int64) *imageapi.Image {
	f := apitesting.FuzzerFor(apitesting.GenericFuzzerFuncs(t, kapi.Codecs), rand.NewSource(seed))
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
	obj, err := kapi.Scheme.DeepCopy(fuzzed)
	if err != nil {
		t.Fatalf("faild to deep copy fuzzed image: %v", err)
	}
	image := obj.(*imageapi.Image)

	if len(image.Signatures) == 0 {
		t.Fatalf("fuzzifier failed to generate signatures")
	}

	Strategy.PrepareForCreate(ctx, image)

	if len(image.Signatures) != len(fuzzed.Signatures) {
		t.Errorf("unexpected number of signatures: %d != %d", len(image.Signatures), len(fuzzed.Signatures))
	}

	for i, sig := range image.Signatures {
		vi := reflect.ValueOf(&sig).Elem()
		vf := reflect.ValueOf(&fuzzed.Signatures[i]).Elem()
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
