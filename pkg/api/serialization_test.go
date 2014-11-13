package api_test

import (
	"flag"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/fsouza/go-dockerclient"
	"github.com/google/gofuzz"

	osapi "github.com/openshift/origin/pkg/api"
	_ "github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
)

var fuzzIters = flag.Int("fuzz_iters", 30, "How many fuzzing iterations to do.")

// apiObjectFuzzer can randomly populate api objects.
var apiObjectFuzzer = fuzz.New().NilChance(.5).NumElements(1, 1).Funcs(
	func(j *runtime.PluginBase, c fuzz.Continue) {
		// Do nothing; this struct has only a Kind field and it must stay blank in memory.
	},
	func(j *runtime.TypeMeta, c fuzz.Continue) {
		// We have to customize the randomization of TypeMetas because their
		// APIVersion and Kind must remain blank in memory.
		j.APIVersion = ""
		j.Kind = ""
		j.Name = c.RandString()
		// TODO: Fix JSON/YAML packages and/or write custom encoding
		// for uint64's. Somehow the LS *byte* of this is lost, but
		// only when all 8 bytes are set.
		j.ResourceVersion = strconv.FormatUint(c.RandUint64()>>8, 10)
		j.SelfLink = c.RandString()

		var sec, nsec int64
		c.Fuzz(&sec)
		c.Fuzz(&nsec)
		j.CreationTimestamp = util.Unix(sec, nsec).Rfc3339Copy()
	},
	func(j *api.ObjectMeta, c fuzz.Continue) {
		// We have to customize the randomization of TypeMetas because their
		// APIVersion and Kind must remain blank in memory.
		j.Name = c.RandString()
		// TODO: Fix JSON/YAML packages and/or write custom encoding
		// for uint64's. Somehow the LS *byte* of this is lost, but
		// only when all 8 bytes are set.
		j.ResourceVersion = strconv.FormatUint(c.RandUint64()>>8, 10)
		j.SelfLink = c.RandString()

		var sec, nsec int64
		c.Fuzz(&sec)
		c.Fuzz(&nsec)
		j.CreationTimestamp = util.Unix(sec, nsec).Rfc3339Copy()
	},
	func(intstr *util.IntOrString, c fuzz.Continue) {
		// util.IntOrString will panic if its kind is set wrong.
		if c.RandBool() {
			intstr.Kind = util.IntstrInt
			intstr.IntVal = int(c.RandUint64())
			intstr.StrVal = ""
		} else {
			intstr.Kind = util.IntstrString
			intstr.IntVal = 0
			intstr.StrVal = c.RandString()
		}
	},
	func(u64 *uint64, c fuzz.Continue) {
		// TODO: uint64's are NOT handled right.
		*u64 = c.RandUint64() >> 8
	},
	func(pb map[docker.Port][]docker.PortBinding, c fuzz.Continue) {
		// This is necessary because keys with nil values get omitted.
		// TODO: Is this a bug?
		pb[docker.Port(c.RandString())] = []docker.PortBinding{
			{c.RandString(), c.RandString()},
			{c.RandString(), c.RandString()},
		}
	},
	func(pm map[string]docker.PortMapping, c fuzz.Continue) {
		// This is necessary because keys with nil values get omitted.
		// TODO: Is this a bug?
		pm[c.RandString()] = docker.PortMapping{
			c.RandString(): c.RandString(),
		}
	},
)

func runTest(t *testing.T, codec runtime.Codec, source runtime.Object) {
	name := reflect.TypeOf(source).Elem().Name()
	apiObjectFuzzer.Fuzz(source)
	j, err := meta.Accessor(source)
	if err != nil {
		t.Fatalf("Unexpected error %v for %#v", err, source)
	}
	j.SetKind("")
	j.SetAPIVersion("")

	data, err := codec.Encode(source)
	if err != nil {
		t.Errorf("%v: %v (%#v)", name, err, source)
		return
	}

	obj2, err := codec.Decode(data)
	if err != nil {
		t.Errorf("%v: %v", name, err)
		return
	} else {
		if !reflect.DeepEqual(source, obj2) {
			t.Errorf("1: %v: diff: %v", name, util.ObjectDiff(source, obj2))
			return
		}
	}
	obj3 := reflect.New(reflect.TypeOf(source).Elem()).Interface().(runtime.Object)
	err = codec.DecodeInto(data, obj3)
	if err != nil {
		t.Errorf("2: %v: %v", name, err)
		return
	} else {
		if !reflect.DeepEqual(source, obj3) {
			t.Errorf("3: %v: diff: %v", name, util.ObjectDiff(source, obj3))
			return
		}
	}
}

func TestTypes(t *testing.T) {
	for kind, reflectType := range api.Scheme.KnownTypes("") {
		if !strings.Contains(reflectType.PkgPath(), "/origin/") {
			continue
		}
		// TODO: gofuzz chokes on the round trip of RawExtension
		if kind == "Config" || kind == "Template" {
			continue
		}
		t.Logf("About to test %v", reflectType)
		// Try a few times, since runTest uses random values.
		for i := 0; i < *fuzzIters; i++ {
			item, err := api.Scheme.New("", kind)
			if err != nil {
				t.Errorf("Couldn't make a %v? %v", kind, err)
				continue
			}
			if _, err := meta.Accessor(item); err != nil {
				t.Logf("%s is not a TypeMeta and cannot be round tripped: %v", kind, err)
				continue
			}
			runTest(t, v1beta1.Codec, item)
			runTest(t, osapi.Codec, item)
		}
	}
}
