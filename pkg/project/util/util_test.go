package util

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/google/gofuzz"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
)

// TestProjectFidelity makes sure that the project to namespace round trip does not lose any data
func TestProjectFidelity(t *testing.T) {
	f := fuzz.New().NilChance(0)
	p := &projectapi.Project{}
	for i := 0; i < 100; i++ {
		f.Fuzz(p)
		p.TypeMeta = metav1.TypeMeta{} // Ignore TypeMeta
		namespace := ConvertProject(p)
		p2 := ConvertNamespace(namespace)
		if !reflect.DeepEqual(p, p2) {
			t.Errorf("project data not preserved; the diff is %s", diff.ObjectDiff(p, p2))
		}
	}
}

// TestNamespaceFidelity makes sure that the namespace to project round trip does not lose any data
func TestNamespaceFidelity(t *testing.T) {
	f := fuzz.New().NilChance(0)
	n := &kapi.Namespace{}
	for i := 0; i < 100; i++ {
		f.Fuzz(n)
		n.TypeMeta = metav1.TypeMeta{} // Ignore TypeMeta
		project := ConvertNamespace(n)
		n2 := ConvertProject(project)
		if !reflect.DeepEqual(n, n2) {
			t.Errorf("namespace data not preserved; the diff is %s", diff.ObjectDiff(n, n2))
		}
	}
}
