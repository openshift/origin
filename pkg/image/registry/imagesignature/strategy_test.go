package imagesignature

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	apitesting "k8s.io/kubernetes/pkg/api/testing"

	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/image/api"
)

func fuzzImageSignature(t *testing.T, signature *api.ImageSignature, seed int64) *api.ImageSignature {
	f := apitesting.FuzzerFor(t, v1.SchemeGroupVersion, rand.NewSource(seed))
	f.Funcs(
		func(j *api.ImageSignature, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.Annotations = make(map[string]string)
			j.Labels = make(map[string]string)
			j.Conditions = []api.SignatureCondition{}
			j.SignedClaims = make(map[string]string)

			j.Content = []byte(c.RandString())
			for i := 0; i < c.Rand.Intn(3)+2; i++ {
				j.Labels[c.RandString()] = c.RandString()
				j.Annotations[c.RandString()] = c.RandString()
				j.SignedClaims[c.RandString()] = c.RandString()
			}
			for i := 0; i < c.Rand.Intn(3)+2; i++ {
				cond := api.SignatureCondition{}
				c.Fuzz(&cond)
				j.Conditions = append(j.Conditions, cond)
			}
		},
	)

	updated := api.ImageSignature{}
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
	ctx := kapi.NewDefaultContext()
	signature := &api.ImageSignature{
		ObjectMeta: kapi.ObjectMeta{
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
