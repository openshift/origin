package api_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/gofuzz"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"

	osapi "github.com/openshift/origin/pkg/api"
	_ "github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/api/v1beta3"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	build "github.com/openshift/origin/pkg/build/api"
	deploy "github.com/openshift/origin/pkg/deploy/api"
	image "github.com/openshift/origin/pkg/image/api"
	route "github.com/openshift/origin/pkg/route/api"
	template "github.com/openshift/origin/pkg/template/api"
	uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

func fuzzInternalObject(t *testing.T, forVersion string, item runtime.Object, seed int64) runtime.Object {
	f := apitesting.FuzzerFor(t, forVersion, rand.NewSource(seed))
	f.Funcs(
		// Roles and RoleBindings maps are never nil
		func(j *authorizationapi.Policy, c fuzz.Continue) {
			j.Roles = make(map[string]*authorizationapi.Role)
		},
		func(j *authorizationapi.PolicyBinding, c fuzz.Continue) {
			j.RoleBindings = make(map[string]*authorizationapi.RoleBinding)
		},
		func(j *authorizationapi.ClusterPolicy, c fuzz.Continue) {
			j.Roles = make(map[string]*authorizationapi.ClusterRole)
		},
		func(j *authorizationapi.ClusterPolicyBinding, c fuzz.Continue) {
			j.RoleBindings = make(map[string]*authorizationapi.ClusterRoleBinding)
		},
		func(j *authorizationapi.RoleBinding, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			for i := range j.Subjects {
				kinds := []string{authorizationapi.UserKind, authorizationapi.SystemUserKind, authorizationapi.GroupKind, authorizationapi.SystemGroupKind, authorizationapi.ServiceAccountKind}
				j.Subjects[i].Kind = kinds[c.Intn(len(kinds))]
				switch j.Subjects[i].Kind {
				case authorizationapi.UserKind:
					j.Subjects[i].Namespace = ""
					if valid, _ := uservalidation.ValidateUserName(j.Subjects[i].Name, false); !valid {
						j.Subjects[i].Name = fmt.Sprintf("validusername%d", i)
					}

				case authorizationapi.GroupKind:
					j.Subjects[i].Namespace = ""
					if valid, _ := uservalidation.ValidateGroupName(j.Subjects[i].Name, false); !valid {
						j.Subjects[i].Name = fmt.Sprintf("validgroupname%d", i)
					}

				case authorizationapi.ServiceAccountKind:
					if valid, _ := validation.ValidateNamespaceName(j.Subjects[i].Namespace, false); !valid {
						j.Subjects[i].Namespace = fmt.Sprintf("sanamespacehere%d", i)
					}
					if valid, _ := validation.ValidateServiceAccountName(j.Subjects[i].Name, false); !valid {
						j.Subjects[i].Name = fmt.Sprintf("sanamehere%d", i)
					}

				case authorizationapi.SystemUserKind, authorizationapi.SystemGroupKind:
					j.Subjects[i].Namespace = ""
					j.Subjects[i].Name = ":" + j.Subjects[i].Name

				}

				j.Subjects[i].UID = types.UID("")
				j.Subjects[i].APIVersion = ""
				j.Subjects[i].ResourceVersion = ""
				j.Subjects[i].FieldPath = ""
			}
		},
		func(j *authorizationapi.ClusterRoleBinding, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			for i := range j.Subjects {
				kinds := []string{authorizationapi.UserKind, authorizationapi.SystemUserKind, authorizationapi.GroupKind, authorizationapi.SystemGroupKind, authorizationapi.ServiceAccountKind}
				j.Subjects[i].Kind = kinds[c.Intn(len(kinds))]
				switch j.Subjects[i].Kind {
				case authorizationapi.UserKind:
					j.Subjects[i].Namespace = ""
					if valid, _ := uservalidation.ValidateUserName(j.Subjects[i].Name, false); !valid {
						j.Subjects[i].Name = fmt.Sprintf("validusername%d", i)
					}

				case authorizationapi.GroupKind:
					j.Subjects[i].Namespace = ""
					if valid, _ := uservalidation.ValidateGroupName(j.Subjects[i].Name, false); !valid {
						j.Subjects[i].Name = fmt.Sprintf("validgroupname%d", i)
					}

				case authorizationapi.ServiceAccountKind:
					if valid, _ := validation.ValidateNamespaceName(j.Subjects[i].Namespace, false); !valid {
						j.Subjects[i].Namespace = fmt.Sprintf("sanamespacehere%d", i)
					}
					if valid, _ := validation.ValidateServiceAccountName(j.Subjects[i].Name, false); !valid {
						j.Subjects[i].Name = fmt.Sprintf("sanamehere%d", i)
					}

				case authorizationapi.SystemUserKind, authorizationapi.SystemGroupKind:
					j.Subjects[i].Namespace = ""
					j.Subjects[i].Name = ":" + j.Subjects[i].Name

				}

				j.Subjects[i].UID = types.UID("")
				j.Subjects[i].APIVersion = ""
				j.Subjects[i].ResourceVersion = ""
				j.Subjects[i].FieldPath = ""
			}
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
		func(j *image.ImageStreamMapping, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.DockerImageRepository = ""
		},
		func(j *image.ImageStreamImage, c fuzz.Continue) {
			c.Fuzz(&j.Image)
			// because we de-embedded Image from ImageStreamImage, in order to round trip
			// successfully, the ImageStreamImage's ObjectMeta must match the Image's.
			j.ObjectMeta = j.Image.ObjectMeta
		},
		func(j *image.ImageStreamTag, c fuzz.Continue) {
			c.Fuzz(&j.Image)
			// because we de-embedded Image from ImageStreamTag, in order to round trip
			// successfully, the ImageStreamTag's ObjectMeta must match the Image's.
			j.ObjectMeta = j.Image.ObjectMeta
		},
		func(j *image.TagReference, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			if j.From != nil {
				specs := []string{"", "ImageStreamTag", "ImageStreamImage"}
				j.From.Kind = specs[c.Intn(len(specs))]
			}
		},
		func(j *build.SourceBuildStrategy, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.From.Kind = "ImageStreamTag"
			j.From.Name = "image:tag"
			j.From.APIVersion = ""
			j.From.ResourceVersion = ""
			j.From.FieldPath = ""
		},
		func(j *build.CustomBuildStrategy, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.From.Kind = "ImageStreamTag"
			j.From.Name = "image:tag"
			j.From.APIVersion = ""
			j.From.ResourceVersion = ""
			j.From.FieldPath = ""
		},
		func(j *build.DockerBuildStrategy, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.From.Kind = "ImageStreamTag"
			j.From.Name = "image:tag"
			j.From.APIVersion = ""
			j.From.ResourceVersion = ""
			j.From.FieldPath = ""
		},
		func(j *build.BuildOutput, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			if j.To != nil && (len(j.To.Kind) == 0 || j.To.Kind == "ImageStream") {
				j.To.Kind = "ImageStreamTag"
			}
			if j.To != nil && strings.Contains(j.To.Name, ":") {
				j.To.Name = strings.Replace(j.To.Name, ":", "-", -1)
			}
		},
		func(j *route.RouteSpec, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.To = api.ObjectReference{
				Kind: "Service",
				Name: j.To.Name,
			}
		},
		func(j *deploy.DeploymentConfig, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			j.Triggers = []deploy.DeploymentTriggerPolicy{{Type: deploy.DeploymentTriggerOnConfigChange}}
		},
		func(j *deploy.DeploymentStrategy, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			strategyTypes := []deploy.DeploymentStrategyType{deploy.DeploymentStrategyTypeRecreate, deploy.DeploymentStrategyTypeRolling, deploy.DeploymentStrategyTypeCustom}
			j.Type = strategyTypes[c.Rand.Intn(len(strategyTypes))]
			switch j.Type {
			case deploy.DeploymentStrategyTypeRolling:
				params := &deploy.RollingDeploymentStrategyParams{}
				randInt64 := func() *int64 {
					p := int64(c.RandUint64())
					return &p
				}
				params.TimeoutSeconds = randInt64()
				params.IntervalSeconds = randInt64()
				params.UpdatePeriodSeconds = randInt64()
				if c.RandBool() {
					params.MaxUnavailable = util.NewIntOrStringFromInt(int(c.RandUint64()))
					params.MaxSurge = util.NewIntOrStringFromInt(int(c.RandUint64()))
				} else {
					params.MaxSurge = util.NewIntOrStringFromString(fmt.Sprintf("%d%%", c.RandUint64()))
					params.MaxUnavailable = util.NewIntOrStringFromString(fmt.Sprintf("%d%%", c.RandUint64()))
				}
				j.RollingParams = params
			default:
				j.RollingParams = nil
			}
		},
		func(j *deploy.DeploymentCauseImageTrigger, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			specs := []string{"", "a/b", "a/b/c", "a:5000/b/c", "a/b", "a/b"}
			tags := []string{"", "stuff", "other"}
			j.RepositoryName = specs[c.Intn(len(specs))]
			if len(j.RepositoryName) > 0 {
				j.Tag = tags[c.Intn(len(tags))]
			} else {
				j.Tag = ""
			}
		},
		func(j *deploy.DeploymentTriggerImageChangeParams, c fuzz.Continue) {
			c.FuzzNoCustom(j)
			specs := []string{"a/b", "a/b/c", "a:5000/b/c", "a/b:latest", "a/b@test"}
			j.From.Kind = "DockerImage"
			j.From.Name = specs[c.Intn(len(specs))]
			if ref, err := image.ParseDockerImageReference(j.From.Name); err == nil {
				j.Tag = ref.Tag
				ref.Tag, ref.ID = "", ""
				j.RepositoryName = ref.String()
			}
		},
		func(j *runtime.EmbeddedObject, c fuzz.Continue) {
			// runtime.EmbeddedObject causes a panic inside of fuzz because runtime.Object isn't handled.
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

func roundTrip(t *testing.T, codec runtime.Codec, originalItem runtime.Object) {
	// Make a copy of the originalItem to give to conversion functions
	// This lets us know if conversion messed with the input object
	deepCopy, err := api.Scheme.DeepCopy(originalItem)
	if err != nil {
		t.Errorf("Could not copy object: %v", err)
		return
	}
	item := deepCopy.(runtime.Object)

	name := reflect.TypeOf(item).Elem().Name()
	data, err := codec.Encode(item)
	if err != nil {
		if conversion.IsNotRegisteredError(err) {
			t.Logf("%v is not registered", name)
			return
		}
		t.Errorf("%v: %v (%#v)", name, err, item)
		return
	}

	obj2, err := codec.Decode(data)
	if err != nil {
		t.Errorf("0: %v: %v\nCodec: %v\nData: %s\nSource: %#v", name, err, codec, string(data), originalItem)
		return
	}
	if reflect.TypeOf(item) != reflect.TypeOf(obj2) {
		obj2conv := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
		if err := api.Scheme.Convert(obj2, obj2conv); err != nil {
			t.Errorf("0X: no conversion from %v to %v: %v", reflect.TypeOf(item), reflect.TypeOf(obj2), err)
			return
		}
		obj2 = obj2conv
	}
	if !api.Semantic.DeepEqual(originalItem, obj2) {
		t.Errorf("1: %v: diff: %v\nCodec: %v\nData: %s\nSource: %s", name, util.ObjectDiff(originalItem, obj2), codec, string(data), util.ObjectGoPrintSideBySide(originalItem, obj2))
		return
	}

	obj3 := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
	err = codec.DecodeInto(data, obj3)
	if err != nil {
		t.Errorf("2: %v: %v", name, err)
		return
	}
	if !api.Semantic.DeepEqual(originalItem, obj3) {
		t.Errorf("3: %v: diff: %v\nCodec: %v", name, util.ObjectDiff(originalItem, obj3), codec)
		return
	}
}

var skipStandardVersions = map[string][]string{
	"DockerImage": {"pre012", "1.0"},
}
var skipV1beta1 = map[string]struct{}{}
var skipV1beta3 = map[string]struct{}{}
var skipV1 = map[string]struct{}{}

const fuzzIters = 20

// For debugging problems
func TestSpecificKind(t *testing.T) {
	api.Scheme.Log(t)
	defer api.Scheme.Log(nil)

	kind := "ImageStreamTag"
	item, err := api.Scheme.New("", kind)
	if err != nil {
		t.Errorf("Couldn't make a %v? %v", kind, err)
		return
	}
	seed := int64(2703387474910584091) //rand.Int63()
	fuzzInternalObject(t, "", item, seed)
	roundTrip(t, osapi.Codec, item)
	roundTrip(t, v1beta3.Codec, item)
	roundTrip(t, v1.Codec, item)
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
			if _, ok := skipV1beta3[kind]; !ok {
				fuzzInternalObject(t, "v1beta3", item, seed)
				roundTrip(t, v1beta3.Codec, item)
			}
			if _, ok := skipV1[kind]; !ok {
				fuzzInternalObject(t, "v1", item, seed)
				roundTrip(t, v1.Codec, item)
			}
		}
	}
}
