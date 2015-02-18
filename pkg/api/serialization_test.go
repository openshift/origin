package api_test

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	apitesting "github.com/GoogleCloudPlatform/kubernetes/pkg/api/testing"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/google/gofuzz"

	osapi "github.com/openshift/origin/pkg/api"
	_ "github.com/openshift/origin/pkg/api/latest"
	//"github.com/openshift/origin/pkg/api/v1beta1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	config "github.com/openshift/origin/pkg/config/api"
	image "github.com/openshift/origin/pkg/image/api"
	template "github.com/openshift/origin/pkg/template/api"
)

func fuzzInternalObject(t *testing.T, forVersion string, item runtime.Object, seed int64) runtime.Object {
	f := apitesting.FuzzerFor(t, forVersion, rand.NewSource(seed))
	f.Funcs(
		// Roles and RoleBindings maps are never nil
		func(j *authorizationapi.Policy, c fuzz.Continue) {
			j.Roles = make(map[string]authorizationapi.Role)
		},
		func(j *authorizationapi.PolicyBinding, c fuzz.Continue) {
			j.RoleBindings = make(map[string]authorizationapi.RoleBinding)
		},
		func(j *template.Template, c fuzz.Continue) {
			c.Fuzz(&j.ObjectMeta)
			c.Fuzz(&j.Parameters)
			// TODO: replace with structured type definition
			j.Objects = []runtime.Object{}
		},
		func(j *image.Image, c fuzz.Continue) {
			c.Fuzz(&j.ObjectMeta)
			c.Fuzz(&j.DockerImageMetadata)
			j.DockerImageMetadata.APIVersion = ""
			j.DockerImageMetadata.Kind = ""
			j.DockerImageMetadataVersion = []string{"pre012", "1.0"}[c.Rand.Intn(2)]
			j.DockerImageReference = c.RandString()
		},
		func(j *runtime.EmbeddedObject, c fuzz.Continue) {
			// runtime.EmbeddedObject causes a panic inside of fuzz because runtime.Object isn't handled.
		},
		func(j *config.Config, c fuzz.Continue) {
			c.Fuzz(&j.ListMeta)
			// TODO: replace with structured type definition
			j.Items = []runtime.Object{}
		},
		func(t *time.Time, c fuzz.Continue) {
			// This is necessary because the standard fuzzed time.Time object is
			// completely nil, but when JSON unmarshals dates it fills in the
			// unexported loc field with the time.UTC object, resulting in
			// reflect.DeepEqual returning false in the round trip tests. We solve it
			// by using a date that will be identical to the one JSON unmarshals.
			*t = time.Date(2000, 1, 1, 1, 1, 1, 0, time.UTC)
		},
		func(u64 *uint64, c fuzz.Continue) {
			// TODO: uint64's are NOT handled right.
			*u64 = c.RandUint64() >> 8
		},
	)

	f.Fuzz(item)

	j, err := meta.TypeAccessor(item)
	if err != nil {
		t.Fatalf("Unexpected error %v for %#v", err, item)
	}
	j.SetKind("")
	j.SetAPIVersion("")

	return item
}

func roundTrip(t *testing.T, codec runtime.Codec, item runtime.Object) {
	name := reflect.TypeOf(item).Elem().Name()
	data, err := codec.Encode(item)
	if err != nil {
		t.Errorf("%v: %v (%#v)", name, err, item)
		return
	}

	obj2, err := codec.Decode(data)
	if err != nil {
		t.Errorf("0: %v: %v\nCodec: %v\nData: %s\nSource: %#v", name, err, codec, string(data), item)
		return
	}
	if !api.Semantic.DeepEqual(item, obj2) {
		t.Errorf("1: %v: diff: %v\nCodec: %v\nData: %s\nSource: %#v\nFinal: %#v", name, util.ObjectGoPrintDiff(item, obj2), codec, string(data), item, obj2)
		return
	}

	obj3 := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
	err = codec.DecodeInto(data, obj3)
	if err != nil {
		t.Errorf("2: %v: %v", name, err)
		return
	}
	if !api.Semantic.DeepEqual(item, obj3) {
		t.Errorf("3: %v: diff: %v\nCodec: %v", name, util.ObjectDiff(item, obj3), codec)
		return
	}
}

var skipStandardVersions = map[string][]string{
	"DockerImage": {"pre012", "1.0"},
}

const fuzzIters = 20

// For debugging problems
func TestSpecificKind(t *testing.T) {
	api.Scheme.Log(t)
	defer api.Scheme.Log(nil)

	kind := "Role"
	item, err := api.Scheme.New("", kind)
	if err != nil {
		t.Errorf("Couldn't make a %v? %v", kind, err)
		return
	}
	seed := int64(2703387474910584091) //rand.Int63()
	fuzzInternalObject(t, "", item, seed)
	roundTrip(t, osapi.Codec, item)
}

func TestTypes(t *testing.T) {
	for kind, reflectType := range api.Scheme.KnownTypes("") {
		if !strings.Contains(reflectType.PkgPath(), "/origin/") {
			continue
		}
		t.Logf("About to test %v", reflectType)
		// Try a few times, since runTest uses random values.
		for i := 0; i < fuzzIters; i++ {
			item, err := api.Scheme.New("", kind)
			if err != nil {
				t.Errorf("Couldn't make a %v? %v", kind, err)
				continue
			}
			if _, err := meta.TypeAccessor(item); err != nil {
				t.Fatalf("%q is not a TypeMeta and cannot be tested - add it to nonRoundTrippableTypes: %v", kind, err)
			}
			seed := rand.Int63()

			if versions, ok := skipStandardVersions[kind]; ok {
				for _, v := range versions {
					fuzzInternalObject(t, "", item, seed)
					roundTrip(t, runtime.CodecFor(api.Scheme, v), item)
				}
				continue
			}
			fuzzInternalObject(t, "", item, seed)
			roundTrip(t, osapi.Codec, item)
			//fuzzInternalObject(t, "", item, seed)
			//roundTrip(t, v1beta1.Codec, item)
		}
	}
}
