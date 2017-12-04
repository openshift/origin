package imagesignature

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/testing/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapitesting "k8s.io/kubernetes/pkg/api/testing"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func fuzzImageSignature(t *testing.T, signature *imageapi.ImageSignature, seed int64) *imageapi.ImageSignature {
	f := fuzzer.FuzzerFor(kapitesting.FuzzerFuncs, rand.NewSource(seed), legacyscheme.Codecs)
	f.Funcs(
		func(j *imageapi.ImageSignature, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.Annotations = make(map[string]string)
			j.Labels = make(map[string]string)
			j.Conditions = []imageapi.SignatureCondition{}
			j.SignedClaims = make(map[string]string)

			j.Content = []byte(c.RandString())
			for i := 0; i < c.Rand.Intn(3)+2; i++ {
				j.Labels[c.RandString()] = c.RandString()
				j.Annotations[c.RandString()] = c.RandString()
				j.SignedClaims[c.RandString()] = c.RandString()
			}
			for i := 0; i < c.Rand.Intn(3)+2; i++ {
				cond := imageapi.SignatureCondition{}
				c.Fuzz(&cond)
				j.Conditions = append(j.Conditions, cond)
			}
		},
	)

	updated := imageapi.ImageSignature{}
	f.Fuzz(&updated)
	updated.Namespace = signature.Namespace
	updated.Name = signature.Name

	j, err := meta.TypeAccessor(signature)
	if err != nil {
		t.Fatalf("Unexpected error %v for %#v", err, signature)
	}
	j.SetKind("")
	j.SetAPIVersion("")

	return &updated
}

func TestStrategyPrepareForCreate(t *testing.T) {
	ctx := apirequest.NewDefaultContext()
	signature := &imageapi.ImageSignature{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image",
		},
	}

	seed := int64(2703387474910584091) //rand.Int63()
	signature = fuzzImageSignature(t, signature, seed)

	Strategy.PrepareForCreate(ctx, signature)

	sValue := reflect.ValueOf(signature).Elem()
	typeOfT := sValue.Type()
	for i := 0; i < sValue.NumField(); i++ {
		f := sValue.Field(i)
		switch typeOfT.Field(i).Name {
		case "Type":
			if len(f.Interface().(string)) == 0 {
				t.Errorf("type must not be empty")
			}
		case "Content":
			if len(f.Interface().([]byte)) == 0 {
				t.Errorf("content must not be empty")
			}
		case "ObjectMeta":
		default:
			value := f.Interface()
			vType := f.Type()
			if !reflect.DeepEqual(value, reflect.Zero(vType).Interface()) {
				t.Errorf("field %q expected unset, got instead: %v", typeOfT.Field(i).Name, value)
			}
		}
	}
}
