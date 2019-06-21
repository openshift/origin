package util

import (
	"math/rand"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	metafuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/api"
	projectapi "github.com/openshift/openshift-apiserver/pkg/project/apis/project"
)

// TestProjectFidelity makes sure that the project to namespace round trip does not lose any data
func TestProjectFidelity(t *testing.T) {
	_, codecFactory := apitesting.SchemeForOrDie(api.Install, projectapi.Install)
	f := fuzzer.FuzzerFor(
		fuzzer.MergeFuzzerFuncs(metafuzzer.Funcs),
		rand.NewSource(rand.Int63()),
		codecFactory,
	)

	p := &projectapi.Project{}
	for i := 0; i < 100; i++ {
		f.Fuzz(p)
		p.Annotations = map[string]string{}           // we mutate annotations
		p.Spec.Finalizers = []coreapi.FinalizerName{} // we mutate finalizers
		p.TypeMeta = metav1.TypeMeta{}                // Ignore TypeMeta

		namespace := ConvertProjectToExternal(p)
		p2 := ConvertNamespaceFromExternal(namespace)
		if !reflect.DeepEqual(p, p2) {
			t.Errorf("project data not preserved; the diff is %s", diff.ObjectReflectDiff(p, p2))
		}
	}
}

// TestNamespaceFidelity makes sure that the namespace to project round trip does not lose any data
func TestNamespaceFidelity(t *testing.T) {
	_, codecFactory := apitesting.SchemeForOrDie(api.Install, projectapi.Install)
	f := fuzzer.FuzzerFor(
		fuzzer.MergeFuzzerFuncs(metafuzzer.Funcs),
		rand.NewSource(rand.Int63()),
		codecFactory,
	)

	n := &corev1.Namespace{}
	for i := 0; i < 100; i++ {
		f.Fuzz(n)
		n.Annotations = map[string]string{}          // we mutate annotations
		n.Spec.Finalizers = []corev1.FinalizerName{} // we mutate finalizers
		n.TypeMeta = metav1.TypeMeta{}               // Ignore TypeMeta

		project := ConvertNamespaceFromExternal(n)
		n2 := ConvertProjectToExternal(project)
		if !reflect.DeepEqual(n, n2) {
			t.Errorf("namespace data not preserved; the diff is %s", diff.ObjectReflectDiff(n, n2))
		}
	}
}
