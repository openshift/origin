package api_test

import (
	"math/rand"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/gofuzz"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/runtime/serializer"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util/diff"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/api/v1"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imagepolicyapi "github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	podnodeapi "github.com/openshift/origin/pkg/scheduler/admission/podnodeconstraints/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/cmd/server/api/install"
)

func fuzzInternalObject(t *testing.T, forVersion unversioned.GroupVersion, item runtime.Object, seed int64) runtime.Object {
	f := fuzzerFor(t, forVersion, rand.NewSource(seed))
	f.Funcs(
		// these follow defaulting rules
		func(obj *configapi.MasterConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.APILevels) == 0 {
				obj.APILevels = configapi.DefaultOpenShiftAPILevels
			}
			if len(obj.Controllers) == 0 {
				obj.Controllers = configapi.ControllersAll
			}
			if obj.ServingInfo.RequestTimeoutSeconds == 0 {
				obj.ServingInfo.RequestTimeoutSeconds = 60 * 60
			}
			if obj.ServingInfo.MaxRequestsInFlight == 0 {
				obj.ServingInfo.MaxRequestsInFlight = 500
			}
			if len(obj.PolicyConfig.OpenShiftInfrastructureNamespace) == 0 {
				obj.PolicyConfig.OpenShiftInfrastructureNamespace = bootstrappolicy.DefaultOpenShiftInfraNamespace
			}
			if len(obj.RoutingConfig.Subdomain) == 0 {
				obj.RoutingConfig.Subdomain = "router.default.svc.cluster.local"
			}

			if obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides == nil {
				obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides = &configapi.ClientConnectionOverrides{
					AcceptContentTypes: "test/second",
					ContentType:        "test/first",
				}
			}
			if obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.QPS <= 0 {
				obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.QPS = 2.0
			}
			if obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.Burst <= 0 {
				obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.Burst = 2
			}
			if len(obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.AcceptContentTypes) == 0 {
				obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.AcceptContentTypes = "test/fourth"
			}
			if len(obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.ContentType) == 0 {
				obj.MasterClients.OpenShiftLoopbackClientConnectionOverrides.ContentType = "test/fifth"
			}
			if obj.MasterClients.ExternalKubernetesClientConnectionOverrides == nil {
				obj.MasterClients.ExternalKubernetesClientConnectionOverrides = &configapi.ClientConnectionOverrides{
					AcceptContentTypes: "test/other",
					ContentType:        "test/third",
				}
			}
			if obj.MasterClients.ExternalKubernetesClientConnectionOverrides.QPS <= 0 {
				obj.MasterClients.ExternalKubernetesClientConnectionOverrides.QPS = 2.0
			}
			if obj.MasterClients.ExternalKubernetesClientConnectionOverrides.Burst <= 0 {
				obj.MasterClients.ExternalKubernetesClientConnectionOverrides.Burst = 2
			}
			if len(obj.MasterClients.ExternalKubernetesClientConnectionOverrides.AcceptContentTypes) == 0 {
				obj.MasterClients.ExternalKubernetesClientConnectionOverrides.AcceptContentTypes = "test/fourth"
			}
			if len(obj.MasterClients.ExternalKubernetesClientConnectionOverrides.ContentType) == 0 {
				obj.MasterClients.ExternalKubernetesClientConnectionOverrides.ContentType = "test/fifth"
			}

			// Populate the new NetworkConfig.ServiceNetworkCIDR field from the KubernetesMasterConfig.ServicesSubnet field if needed
			if len(obj.NetworkConfig.ServiceNetworkCIDR) == 0 {
				if obj.KubernetesMasterConfig != nil && len(obj.KubernetesMasterConfig.ServicesSubnet) > 0 {
					// if a subnet is set in the kubernetes master config, use that
					obj.NetworkConfig.ServiceNetworkCIDR = obj.KubernetesMasterConfig.ServicesSubnet
				} else {
					// default ServiceClusterIPRange used by kubernetes if nothing is specified
					obj.NetworkConfig.ServiceNetworkCIDR = "10.0.0.0/24"
				}
			}

			// TODO stop duplicating the conversion in the test.
			kubeConfig := obj.KubernetesMasterConfig
			noCloudProvider := kubeConfig != nil && (len(kubeConfig.ControllerArguments["cloud-provider"]) == 0 || kubeConfig.ControllerArguments["cloud-provider"][0] == "")
			if noCloudProvider && len(obj.NetworkConfig.IngressIPNetworkCIDR) == 0 {
				cidr := "172.46.0.0/16"
				if !(configapi.CIDRsOverlap(cidr, obj.NetworkConfig.ClusterNetworkCIDR) || configapi.CIDRsOverlap(cidr, obj.NetworkConfig.ServiceNetworkCIDR)) {
					obj.NetworkConfig.IngressIPNetworkCIDR = cidr
				}
			}

			// Historically, the clientCA was incorrectly used as the master's server cert CA bundle
			// If missing from the config, migrate the ClientCA into that field
			if obj.OAuthConfig != nil && obj.OAuthConfig.MasterCA == nil {
				s := obj.ServingInfo.ClientCA
				// The final value of OAuthConfig.MasterCA should never be nil
				obj.OAuthConfig.MasterCA = &s
			}

			// test an admission plugin nested for round tripping
			if c.RandBool() {
				obj.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
					"abc": {
						Location: "test",
						Configuration: &configapi.LDAPSyncConfig{
							URL: "ldap://some:other@server:8080/test",
						},
					},
				}
			}
			// test a Kubernetes admission plugin nested for round tripping
			if obj.KubernetesMasterConfig != nil && c.RandBool() {
				obj.KubernetesMasterConfig.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
					"abc": {
						Location: "test",
						Configuration: &configapi.LDAPSyncConfig{
							URL: "ldap://some:other@server:8080/test",
						},
					},
				}
			}
			if obj.OAuthConfig != nil && c.RandBool() {
				obj.OAuthConfig.IdentityProviders = []configapi.IdentityProvider{
					{
						MappingMethod: "claim",
						Provider: &configapi.LDAPSyncConfig{
							URL: "ldap://some:other@server:8080/test",
						},
					},
				}
			}
		},
		func(obj *configapi.KubernetesMasterConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if obj.MasterCount == 0 {
				obj.MasterCount = 1
			}
			if len(obj.ServicesNodePortRange) == 0 {
				obj.ServicesNodePortRange = "30000-32767"
			}
			if len(obj.PodEvictionTimeout) == 0 {
				obj.PodEvictionTimeout = "5m"
			}
		},
		func(obj *configapi.JenkinsPipelineConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if obj.AutoProvisionEnabled == nil {
				v := c.RandBool()
				obj.AutoProvisionEnabled = &v
			}
			if len(obj.TemplateNamespace) == 0 {
				obj.TemplateNamespace = "value"
			}
			if len(obj.TemplateName) == 0 {
				obj.TemplateName = "anothervalue"
			}
			if len(obj.ServiceName) == 0 {
				obj.ServiceName = "thirdvalue"
			}
		},
		func(obj *configapi.NodeConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			// Defaults/migrations for NetworkConfig
			if len(obj.NetworkConfig.NetworkPluginName) == 0 {
				obj.NetworkConfig.NetworkPluginName = "plugin-name"
			}
			if obj.NetworkConfig.MTU == 0 {
				obj.NetworkConfig.MTU = 1450
			}
			if len(obj.IPTablesSyncPeriod) == 0 {
				obj.IPTablesSyncPeriod = "5s"
			}

			if obj.MasterClientConnectionOverrides == nil {
				obj.MasterClientConnectionOverrides = &configapi.ClientConnectionOverrides{
					QPS:                1.0,
					Burst:              1,
					AcceptContentTypes: "test/other",
					ContentType:        "test/third",
				}
			}
			if len(obj.MasterClientConnectionOverrides.AcceptContentTypes) == 0 {
				obj.MasterClientConnectionOverrides.AcceptContentTypes = "test/fourth"
			}
			if len(obj.MasterClientConnectionOverrides.ContentType) == 0 {
				obj.MasterClientConnectionOverrides.ContentType = "test/fifth"
			}

			// Auth cache defaults
			if len(obj.AuthConfig.AuthenticationCacheTTL) == 0 {
				obj.AuthConfig.AuthenticationCacheTTL = "5m"
			}
			if obj.AuthConfig.AuthenticationCacheSize == 0 {
				obj.AuthConfig.AuthenticationCacheSize = 1000
			}
			if len(obj.AuthConfig.AuthorizationCacheTTL) == 0 {
				obj.AuthConfig.AuthorizationCacheTTL = "5m"
			}
			if obj.AuthConfig.AuthorizationCacheSize == 0 {
				obj.AuthConfig.AuthorizationCacheSize = 1000
			}
		},
		func(obj *configapi.EtcdStorageConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.KubernetesStorageVersion) == 0 {
				obj.KubernetesStorageVersion = "v1"
			}
			if len(obj.KubernetesStoragePrefix) == 0 {
				obj.KubernetesStoragePrefix = "kubernetes.io"
			}
			if len(obj.OpenShiftStorageVersion) == 0 {
				obj.OpenShiftStorageVersion = configapi.DefaultOpenShiftStorageVersionLevel
			}
			if len(obj.OpenShiftStoragePrefix) == 0 {
				obj.OpenShiftStoragePrefix = "openshift.io"
			}
		},
		func(obj *configapi.DockerConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.ExecHandlerName) == 0 {
				obj.ExecHandlerName = configapi.DockerExecHandlerNative
			}
		},
		func(obj *configapi.ServingInfo, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.BindNetwork) == 0 {
				obj.BindNetwork = "tcp4"
			}
		},
		func(obj *configapi.ImagePolicyConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if obj.MaxImagesBulkImportedPerRepository == 0 {
				obj.MaxImagesBulkImportedPerRepository = 5
			}
			if obj.MaxScheduledImageImportsPerMinute == 0 {
				obj.MaxScheduledImageImportsPerMinute = 60
			}
			if obj.ScheduledImageImportMinimumIntervalSeconds == 0 {
				obj.ScheduledImageImportMinimumIntervalSeconds = 15 * 60
			}
		},
		func(obj *configapi.DNSConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.BindNetwork) == 0 {
				obj.BindNetwork = "tcp4"
			}
		},
		func(obj *configapi.SecurityAllocator, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.UIDAllocatorRange) == 0 {
				obj.UIDAllocatorRange = "1000000000-1999999999/10000"
			}
			if len(obj.MCSAllocatorRange) == 0 {
				obj.MCSAllocatorRange = "s0:/2"
			}
			if obj.MCSLabelsPerProject == 0 {
				obj.MCSLabelsPerProject = 5
			}
		},
		func(obj *configapi.IdentityProvider, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.MappingMethod) == 0 {
				// By default, only let one identity provider authenticate a particular user
				// If multiple identity providers collide, the second one in will fail to auth
				// The admin can set this to "add" if they want to allow new identities to join existing users
				obj.MappingMethod = "claim"
			}
		},
		func(s *configapi.StringSource, c fuzz.Continue) {
			if c.RandBool() {
				c.Fuzz(&s.Value)
			} else {
				c.Fuzz(&s.StringSourceSpec)
			}
		},
		func(obj *podnodeapi.PodNodeConstraintsConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if obj.NodeSelectorLabelBlacklist == nil {
				obj.NodeSelectorLabelBlacklist = []string{"kubernetes.io/hostname"}
			}
		},
		func(obj *labels.Selector, c fuzz.Continue) {
		},
		func(obj *imagepolicyapi.ImagePolicyConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			for i := range obj.ExecutionRules {
				if len(obj.ExecutionRules[i].OnResources) == 0 {
					obj.ExecutionRules[i].OnResources = []unversioned.GroupResource{{Resource: "pods"}}
				}
				obj.ExecutionRules[i].MatchImageLabelSelectors = nil
			}
			if len(obj.ResolveImages) == 0 {
				obj.ResolveImages = imagepolicyapi.Attempt
			}
		},
		func(obj *configapi.GrantConfig, c fuzz.Continue) {
			c.FuzzNoCustom(obj)
			if len(obj.ServiceAccountMethod) == 0 {
				obj.ServiceAccountMethod = "prompt"
			}
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
	deepCopy, err := configapi.Scheme.DeepCopy(originalItem)
	if err != nil {
		t.Errorf("Could not copy object: %v", err)
		return
	}
	item := deepCopy.(runtime.Object)

	name := reflect.TypeOf(item).Elem().Name()
	data, err := runtime.Encode(codec, item)
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			t.Logf("%v is not registered", name)
			return
		}
		t.Errorf("%v: %v (%#v)", name, err, item)
		return
	}

	obj2, err := runtime.Decode(codec, data)
	if err != nil {
		t.Errorf("0: %v: %v\nCodec: %v\nData: %s\nSource: %#v", name, err, codec, string(data), originalItem)
		return
	}
	if reflect.TypeOf(item) != reflect.TypeOf(obj2) {
		obj2conv := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
		if err := configapi.Scheme.Convert(obj2, obj2conv, nil); err != nil {
			t.Errorf("0X: no conversion from %v to %v: %v", reflect.TypeOf(item), reflect.TypeOf(obj2), err)
			return
		}
		obj2 = obj2conv
	}

	if !kapi.Semantic.DeepEqual(originalItem, obj2) {
		t.Errorf("1: %v: diff: %v\nCodec: %v\nData: %s", name, diff.ObjectReflectDiff(originalItem, obj2), codec, string(data))
		return
	}

	obj3 := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
	if err := runtime.DecodeInto(codec, data, obj3); err != nil {
		t.Errorf("2: %v: %v", name, err)
		return
	}
	if !kapi.Semantic.DeepEqual(originalItem, obj3) {
		t.Errorf("3: %v: diff: %v\nCodec: %v", name, diff.ObjectReflectDiff(originalItem, obj3), codec)
		return
	}
}

const fuzzIters = 20

// For debugging problems
func TestSpecificKind(t *testing.T) {
	configapi.Scheme.Log(t)
	defer configapi.Scheme.Log(nil)

	kind := "MasterConfig"
	item, err := configapi.Scheme.New(configapi.SchemeGroupVersion.WithKind(kind))
	if err != nil {
		t.Errorf("Couldn't make a %v? %v", kind, err)
		return
	}
	seed := int64(2703387474910584091) //rand.Int63()
	for i := 0; i < fuzzIters; i++ {
		fuzzInternalObject(t, configapiv1.SchemeGroupVersion, item, seed)
		roundTrip(t, serializer.NewCodecFactory(configapi.Scheme).LegacyCodec(configapiv1.SchemeGroupVersion), item)
	}
}

// TestTypes will try to roundtrip all OpenShift and Kubernetes stable api types
func TestTypes(t *testing.T) {
	for kind := range configapi.Scheme.KnownTypes(configapi.SchemeGroupVersion) {
		// Try a few times, since runTest uses random values.
		for i := 0; i < fuzzIters; i++ {
			item, err := configapi.Scheme.New(configapi.SchemeGroupVersion.WithKind(kind))
			if err != nil {
				t.Errorf("Couldn't make a %v? %v", kind, err)
				continue
			}
			if _, err := meta.TypeAccessor(item); err != nil {
				t.Fatalf("%q is not a TypeMeta and cannot be tested - add it to nonRoundTrippableTypes: %v", kind, err)
			}
			seed := rand.Int63()

			fuzzInternalObject(t, configapiv1.SchemeGroupVersion, item, seed)
			roundTrip(t, serializer.NewCodecFactory(configapi.Scheme).LegacyCodec(configapiv1.SchemeGroupVersion), item)
		}
	}
}

func TestSpecificRoundTrips(t *testing.T) {
	boolFalse := false
	testCases := []struct {
		mediaType string
		in, out   runtime.Object
		to, from  unversioned.GroupVersion
	}{
		{
			in: &configapi.MasterConfig{
				AdmissionConfig: configapi.AdmissionConfig{
					PluginConfig: map[string]configapi.AdmissionPluginConfig{
						"test1": {Configuration: &configapi.LDAPSyncConfig{BindDN: "first"}},
						"test2": {Configuration: &runtime.Unknown{Raw: []byte(`{"kind":"LDAPSyncConfig","apiVersion":"v1","bindDN":"second"}`)}},
						"test3": {Configuration: &runtime.Unknown{Raw: []byte(`{"kind":"Unknown","apiVersion":"some/version"}`)}},
						"test4": {Configuration: nil},
					},
				},
			},
			to: configapiv1.SchemeGroupVersion,
			out: &configapiv1.MasterConfig{
				TypeMeta: unversioned.TypeMeta{Kind: "MasterConfig", APIVersion: "v1"},
				AdmissionConfig: configapiv1.AdmissionConfig{
					PluginConfig: map[string]configapiv1.AdmissionPluginConfig{
						"test1": {Configuration: runtime.RawExtension{
							Object: &configapiv1.LDAPSyncConfig{BindDN: "first"},
							Raw:    []byte(`{"kind":"LDAPSyncConfig","apiVersion":"v1","url":"","bindDN":"first","bindPassword":"","insecure":false,"ca":"","groupUIDNameMapping":null}`),
						}},
						"test2": {Configuration: runtime.RawExtension{
							Object: &configapiv1.LDAPSyncConfig{BindDN: "second"},
							Raw:    []byte(`{"kind":"LDAPSyncConfig","apiVersion":"v1","bindDN":"second"}`),
						}},
						"test3": {Configuration: runtime.RawExtension{
							Object: &runtime.Unknown{TypeMeta: runtime.TypeMeta{Kind: "Unknown", APIVersion: "some/version"}, ContentType: "application/json", Raw: []byte(`{"kind":"Unknown","apiVersion":"some/version"}`)},
							Raw:    []byte(`{"kind":"Unknown","apiVersion":"some/version"}`),
						}},
						"test4": {},
					},
				},
				VolumeConfig: configapiv1.MasterVolumeConfig{DynamicProvisioningEnabled: &boolFalse},
			},
			from: configapiv1.SchemeGroupVersion,
		},
	}

	f := serializer.NewCodecFactory(configapi.Scheme)
	for i, test := range testCases {
		var s runtime.Serializer
		if len(test.mediaType) != 0 {
			info, _ := f.SerializerForMediaType(test.mediaType, nil)
			s = info.Serializer
		} else {
			info, _ := f.SerializerForMediaType(f.SupportedMediaTypes()[0], nil)
			s = info.Serializer
		}
		data, err := runtime.Encode(f.LegacyCodec(test.to), test.in)
		if err != nil {
			t.Errorf("%d: unable to encode: %v", i, err)
			continue
		}
		result, err := runtime.Decode(f.DecoderToVersion(s, test.from), data)
		if err != nil {
			t.Errorf("%d: unable to decode: %v", i, err)
			continue
		}
		if !reflect.DeepEqual(test.out, result) {
			t.Errorf("%d: result did not match: %s", i, diff.ObjectReflectDiff(test.out, result))
			continue
		}
	}
}

func fuzzerFor(t *testing.T, version unversioned.GroupVersion, src rand.Source) *fuzz.Fuzzer {
	f := fuzz.New().NilChance(.5).NumElements(1, 1)
	if src != nil {
		f.RandSource(src)
	}
	f.Funcs(
		func(j *runtime.TypeMeta, c fuzz.Continue) {
			// We have to customize the randomization of TypeMetas because their
			// APIVersion and Kind must remain blank in memory.
			j.APIVersion = ""
			j.Kind = ""
		},
		func(j *runtime.Object, c fuzz.Continue) {
			*j = &runtime.Unknown{
				TypeMeta: runtime.TypeMeta{
					APIVersion: "unknown.group/unknown",
					Kind:       "Something",
				},
				ContentType: "application/json",
				Raw:         []byte(`{"apiVersion":"unknown.group/unknown","kind":"Something","someKey":"someValue"}`),
			}
		},
		func(j *unversioned.TypeMeta, c fuzz.Continue) {
			// We have to customize the randomization of TypeMetas because their
			// APIVersion and Kind must remain blank in memory.
			j.APIVersion = ""
			j.Kind = ""
		},
		func(j *kapi.ObjectMeta, c fuzz.Continue) {
			j.Name = c.RandString()
			j.ResourceVersion = strconv.FormatUint(c.RandUint64(), 10)
			j.SelfLink = c.RandString()
			j.UID = types.UID(c.RandString())
			j.GenerateName = c.RandString()

			var sec, nsec int64
			c.Fuzz(&sec)
			c.Fuzz(&nsec)
			j.CreationTimestamp = unversioned.Unix(sec, nsec).Rfc3339Copy()
		},
		func(j *kapi.ObjectReference, c fuzz.Continue) {
			// We have to customize the randomization of TypeMetas because their
			// APIVersion and Kind must remain blank in memory.
			j.APIVersion = c.RandString()
			j.Kind = c.RandString()
			j.Namespace = c.RandString()
			j.Name = c.RandString()
			j.ResourceVersion = strconv.FormatUint(c.RandUint64(), 10)
			j.FieldPath = c.RandString()
		},
	)
	return f
}
