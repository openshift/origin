package imagesignature

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapitesting "k8s.io/kubernetes/pkg/api/testing"

	imagev1 "github.com/openshift/api/image/v1"
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

func TestIndexOfImageSignature(t *testing.T) {
	for _, tc := range []struct {
		name          string
		signatures    []imagev1.ImageSignature
		matchType     string
		matchContent  []byte
		expectedIndex int
	}{
		{
			name:          "empty",
			matchType:     imagev1.ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: -1,
		},

		{
			name: "not present",
			signatures: []imagev1.ImageSignature{
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
			},
			matchType:     imagev1.ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: -1,
		},

		{
			name: "first and only",
			signatures: []imagev1.ImageSignature{
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
			},
			matchType:     imagev1.ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("binary"),
			expectedIndex: 0,
		},

		{
			name: "last",
			signatures: []imagev1.ImageSignature{
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
			},
			matchType:     imagev1.ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: 2,
		},

		{
			name: "many matches",
			signatures: []imagev1.ImageSignature{
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob2"),
				},
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    imagev1.ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
			},
			matchType:     imagev1.ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: 1,
		},
	} {

		im := imagev1.Image{
			Signatures: make([]imagev1.ImageSignature, len(tc.signatures)),
		}
		for i, signature := range tc.signatures {
			signature.Name = fmt.Sprintf("%s:%s", signature.Type, signature.Content)
			im.Signatures[i] = signature
		}

		matchName := fmt.Sprintf("%s:%s", tc.matchType, tc.matchContent)

		index := indexOfImageSignatureByName(im.Signatures, matchName)
		if index != tc.expectedIndex {
			t.Errorf("[%s] got unexpected index: %d != %d", tc.name, index, tc.expectedIndex)
		}

		index = indexOfImageSignature(im.Signatures, tc.matchType, tc.matchContent)
		if index != tc.expectedIndex {
			t.Errorf("[%s] got unexpected index: %d != %d", tc.name, index, tc.expectedIndex)
		}
	}
}
