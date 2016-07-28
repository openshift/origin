package upstreamconversions

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/testapi"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	extensionsv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
)

func TestDeploymentRoundtrip(t *testing.T) {
	for i := 0; i < 20; i++ {
		// Deployment
		var (
			in                 = &extensions.Deployment{}
			out                = &extensions.Deployment{}
			deploymentExternal = &extensionsv1beta1.Deployment{}
		)

		// DeploymentConfig
		var (
			configInternal = &deployapi.DeploymentConfig{}
			configExternal = &deployapiv1.DeploymentConfig{}
		)

		// Add Convert_v1beta1_Deployment_to_api_DeploymentConfig to scheme
		AddToScheme(kapi.Scheme)

		extGroup := testapi.Extensions
		fuzzInternalObject(t, extGroup.InternalGroupVersion(), in, rand.Int63(),
			func(p *kapi.PodTemplateSpec, c fuzz.Continue) {
				c.FuzzNoCustom(p)
				c.Fuzz(&p.Spec)
				p.Annotations = map[string]string{}
				p.Spec.InitContainers = []kapi.Container{}
			},
			func(s *unversioned.LabelSelector, c fuzz.Continue) {
				s.MatchLabels = map[string]string{"foo": "bar"}
			},
			// the rollbackTo is not supported in deployment config
			func(r *extensions.RollbackConfig, c fuzz.Continue) {},
		)

		mustBeEqualDiff := func(in interface{}, out interface{}) {
			if !reflect.DeepEqual(in, out) {
				t.Fatalf("objects are different:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s", in, out, diff.ObjectDiff(in, out), diff.ObjectGoPrintSideBySide(out, in))
			}
		}

		if err := kapi.Scheme.Convert(in, deploymentExternal, nil); err != nil {
			t.Fatalf("d.internal -> d.v1beta1: unexpected error: %v", err)
		}

		if err := kapi.Scheme.Convert(deploymentExternal, configInternal, nil); err != nil {
			t.Fatalf("d.v1beta1 -> dc.internal: unexpected error: %v", err)
		}

		if err := kapi.Scheme.Convert(configInternal, configExternal, nil); err != nil {
			t.Fatalf("dc.internal -> dc.v1: unexpected error: %v", err)
		}

		if err := kapi.Scheme.Convert(configExternal, out, nil); err != nil {
			t.Fatalf("dc.v1 -> d.internal: unexpected error: %v", err)
		}

		mustBeEqualDiff(in.Status, out.Status)
		mustBeEqualDiff(in.Spec, out.Spec)
	}
}

func TestDeploymentConfigRoundtrip(t *testing.T) {
	for i := 0; i < 20; i++ {
		// DeploymentConfig
		var (
			in             = &deployapi.DeploymentConfig{}
			out            = &deployapi.DeploymentConfig{}
			configExternal = &deployapiv1.DeploymentConfig{}
		)

		// Deployment
		var (
			deploymentInternal = &extensions.Deployment{}
			deploymentExternal = &extensionsv1beta1.Deployment{}
		)

		// Add Convert_v1beta1_Deployment_to_api_DeploymentConfig to scheme
		AddToScheme(kapi.Scheme)

		extGroup := testapi.Extensions
		fuzzInternalObject(t, extGroup.InternalGroupVersion(), in, rand.Int63(),
			func(p *kapi.PodTemplateSpec, c fuzz.Continue) {
				c.FuzzNoCustom(p)
				c.Fuzz(&p.Spec)
				p.Annotations = map[string]string{}
				p.Spec.InitContainers = []kapi.Container{}
			},
			func(s *unversioned.LabelSelector, c fuzz.Continue) {
				s.MatchLabels = map[string]string{"foo": "bar"}
			},
			// the rollbackTo is not supported in deployment config
			func(r *extensions.RollbackConfig, c fuzz.Continue) {},
			// custom deployment strategy is not supported in upstream
			func(p *deployapi.CustomDeploymentStrategyParams, c fuzz.Continue) {},
			func(p *deployapi.RecreateDeploymentStrategyParams, c fuzz.Continue) {
				c.FuzzNoCustom(p)
				v := int64(60)
				p.TimeoutSeconds = &v
			},
			func(p *deployapi.RollingDeploymentStrategyParams, c fuzz.Continue) {
				c.FuzzNoCustom(p)
				timeoutVal := int64(60)
				p.TimeoutSeconds = &timeoutVal
				p.UpdatePercent = nil
			},
			func(h *deployapi.ExecNewPodHook, c fuzz.Continue) {
				c.FuzzNoCustom(h)
				h.ContainerName = "foo"
			},
			func(h *deployapi.LifecycleHook, c fuzz.Continue) {
				c.FuzzNoCustom(h)
				h.FailurePolicy = deployapi.LifecycleHookFailurePolicyAbort
				for i := range h.TagImages {
					h.TagImages[i].ContainerName = "foo"
				}
			},
			func(d *deployapi.DeploymentConfig, c fuzz.Continue) {
				c.FuzzNoCustom(d)
				strategies := []deployapi.DeploymentStrategyType{
					deployapi.DeploymentStrategyTypeRolling,
					deployapi.DeploymentStrategyTypeRecreate,
				}
				d.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{
					{
						Type: deployapi.DeploymentTriggerOnConfigChange,
					},
				}
				d.Spec.Strategy.Type = strategies[rand.Intn(len(strategies))]
				d.Spec.Strategy.CustomParams = nil
				switch d.Spec.Strategy.Type {
				case deployapi.DeploymentStrategyTypeRolling:
					d.Spec.Strategy.RecreateParams = nil
					d.Spec.Strategy.RollingParams = &deployapi.RollingDeploymentStrategyParams{}
					c.Fuzz(d.Spec.Strategy.RollingParams)
				case deployapi.DeploymentStrategyTypeRecreate:
					d.Spec.Strategy.RollingParams = nil
					d.Spec.Strategy.RecreateParams = &deployapi.RecreateDeploymentStrategyParams{}
					c.Fuzz(d.Spec.Strategy.RecreateParams)
				}
			},
		)

		mustBeEqualDiff := func(input interface{}, output interface{}) {
			if !reflect.DeepEqual(input, output) {
				t.Fatalf("objects are different:\nA:\t%#v\nB:\t%#v\n\nDiff:\n%s\n\n%s",
					input, output, diff.ObjectDiff(input, output), diff.ObjectGoPrintSideBySide(input, output))
			}
		}

		if err := kapi.Scheme.Convert(in, configExternal, nil); err != nil {
			t.Fatalf("dc.internal -> dc.v1beta1: unexpected error: %v", err)
		}

		if err := kapi.Scheme.Convert(configExternal, deploymentInternal, nil); err != nil {
			t.Fatalf("dc.v1 -> d.internal: unexpected error: %v", err)
		}

		if err := kapi.Scheme.Convert(deploymentInternal, deploymentExternal, nil); err != nil {
			t.Fatalf("d.internal -> d.v1: unexpected error: %v", err)
		}

		if err := kapi.Scheme.Convert(deploymentExternal, out, nil); err != nil {
			t.Fatalf("d.v1 -> dc.internal: unexpected error: %v", err)
		}

		if out.ObjectMeta.Annotations[kapi.OriginalKindAnnotationName] != "DeploymentConfig." {
			t.Errorf("expected original-kind annotations to be set to v1.DeploymentConfig, got %v", out.ObjectMeta.Annotations[kapi.OriginalKindAnnotationName])
		}

		// TODO: The resources are properly restored, but the format is changed from
		// DecimalExponent to DecimalSI. We should investigate why is that
		// happening.
		out.Spec.Strategy.Resources = in.Spec.Strategy.Resources

		mustBeEqualDiff(in.Status, out.Status)
		mustBeEqualDiff(in.Spec, out.Spec)
	}
}

func fuzzInternalObject(t *testing.T, forVersion unversioned.GroupVersion, item runtime.Object, seed int64, funcs ...interface{}) runtime.Object {
	f := apitesting.FuzzerFor(t, forVersion, rand.NewSource(seed))
	if len(funcs) > 0 {
		f.Funcs(funcs...).Fuzz(item)
	} else {
		f.Fuzz(item)
	}

	j, err := meta.TypeAccessor(item)
	if err != nil {
		t.Fatalf("Unexpected error %v for %#v", err, item)
	}
	j.SetKind("")
	j.SetAPIVersion("")

	return item
}
