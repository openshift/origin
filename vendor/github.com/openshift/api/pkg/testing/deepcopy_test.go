package testing

import (
	"math/rand"
	"testing"

	"k8s.io/apimachinery/pkg/api/testing/fuzzer"
	"k8s.io/apimachinery/pkg/api/testing/roundtrip"
	genericfuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"

	apps "github.com/openshift/api/apps/v1"
	authorization "github.com/openshift/api/authorization/v1"
	build "github.com/openshift/api/build/v1"
	image "github.com/openshift/api/image/v1"
	network "github.com/openshift/api/network/v1"
	oauth "github.com/openshift/api/oauth/v1"
	project "github.com/openshift/api/project/v1"
	quota "github.com/openshift/api/quota/v1"
	route "github.com/openshift/api/route/v1"
	security "github.com/openshift/api/security/v1"
	template "github.com/openshift/api/template/v1"
	user "github.com/openshift/api/user/v1"
)

var groups = map[schema.GroupVersion]func(*runtime.Scheme) error{
	apps.GroupVersion:          apps.Install,
	authorization.GroupVersion: authorization.Install,
	build.GroupVersion:         build.Install,
	image.GroupVersion:         image.Install,
	network.GroupVersion:       network.Install,
	oauth.GroupVersion:         oauth.Install,
	project.GroupVersion:       project.Install,
	quota.GroupVersion:         quota.Install,
	route.GroupVersion:         route.Install,
	security.GroupVersion:      security.Install,
	template.GroupVersion:      template.Install,
	user.GroupVersion:          user.Install,
}

// TestRoundTripTypesWithoutProtobuf applies the round-trip test to all round-trippable Kinds
// in the scheme.
func TestRoundTripTypesWithoutProtobuf(t *testing.T) {
	// TODO upstream this loop
	for gv, install := range groups {
		t.Logf("starting group %q", gv)
		scheme := runtime.NewScheme()
		codecs := serializer.NewCodecFactory(scheme)

		install(scheme)
		seed := rand.Int63()
		// I'm only using the generic fuzzer funcs, but at some point in time we might need to
		// switch to specialized. For now we're happy with the current serialization test.
		fuzzer := fuzzer.FuzzerFor(genericfuzzer.Funcs, rand.NewSource(seed), codecs)
		kinds := scheme.KnownTypes(gv)

		for kind := range kinds {
			if globalNonRoundTrippableTypes.Has(kind) {
				continue
			}
			gvk := gv.WithKind(kind)
			// FIXME: this is explicitly testing w/o protobuf which was failing
			// The RoundTripSpecificKindWithoutProtobuf performs the following sequence of actions:
			// 1. object := original.DeepCopyObject()
			// 2. obj3 := Decode(Encode(object)
			// 3. Fuzz(object)
			// 4. check semantic.DeepEqual(original, obj3)
			roundtrip.RoundTripSpecificKindWithoutProtobuf(t, gvk, scheme, codecs, fuzzer, nil)
		}
		t.Logf("finished group %q", gv)
	}
}

func TestFailRoundTrip(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	groupVersion := schema.GroupVersion{Group: "broken", Version: "v1"}
	builder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(groupVersion, &BrokenType{})
		metav1.AddToGroupVersion(scheme, groupVersion)
		return nil
	})
	builder.AddToScheme(scheme)
	seed := rand.Int63()
	fuzzer := fuzzer.FuzzerFor(genericfuzzer.Funcs, rand.NewSource(seed), codecs)
	gvk := groupVersion.WithKind("BrokenType")
	tmpT := new(testing.T)
	roundtrip.RoundTripSpecificKindWithoutProtobuf(tmpT, gvk, scheme, codecs, fuzzer, nil)
	// It's very hacky way of making sure the DeepCopy is actually invoked inside RoundTripSpecificKindWithoutProtobuf
	// used in the other test. If for some reason this tests starts passing we need to fail b/c we're not testing
	// the DeepCopy in the other method which we care so much about.
	if !tmpT.Failed() {
		t.Log("RoundTrip should've failed on DeepCopy but it did not!")
		t.FailNow()
	}
}

// TODO: externalize this upstream
var globalNonRoundTrippableTypes = sets.NewString(
	"ExportOptions",
	"GetOptions",
	// WatchEvent does not include kind and version and can only be deserialized
	// implicitly (if the caller expects the specific object). The watch call defines
	// the schema by content type, rather than via kind/version included in each
	// object.
	"WatchEvent",
	// ListOptions is now part of the meta group
	"ListOptions",
	// Delete options is only read in metav1
	"DeleteOptions",
)

type BrokenType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Field1 string `json:"field1,omitempty"`
	Field2 string `json:"field2,omitempty"`
}

func (in *BrokenType) DeepCopy() *BrokenType {
	return new(BrokenType)
}

func (in *BrokenType) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	} else {
		return nil
	}
}
