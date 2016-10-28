package api_test

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"

	"github.com/openshift/origin/pkg/quota/api"
	_ "github.com/openshift/origin/pkg/quota/api/install"
)

func TestDeepCopy(t *testing.T) {
	make := func() *api.ClusterResourceQuota {
		q := resource.Quantity{}
		q.Set(100)
		crq := &api.ClusterResourceQuota{}
		crq.Status.Namespaces.Insert("ns1", kapi.ResourceQuotaStatus{Hard: kapi.ResourceList{"a": q.DeepCopy()}, Used: kapi.ResourceList{"a": q.DeepCopy()}})
		crq.Status.Namespaces.Insert("ns2", kapi.ResourceQuotaStatus{Hard: kapi.ResourceList{"b": q.DeepCopy()}, Used: kapi.ResourceList{"b": q.DeepCopy()}})
		return crq
	}

	check := make()

	original := make()
	if !reflect.DeepEqual(check, original) {
		t.Error("before mutation of copy, check and original should be identical but are not, likely failure in deepequal")
	}
	if !kapi.Semantic.DeepEqual(check, original) {
		t.Error("before mutation of copy, check and original should be identical but are not, likely failure in deepequal")
	}

	copiedObj, err := kapi.Scheme.Copy(original)
	if err != nil {
		t.Fatal(err)
	}
	copied := copiedObj.(*api.ClusterResourceQuota)
	if !reflect.DeepEqual(copied, original) {
		t.Error("before mutation of copy, copied and original should be identical but are not, likely failure in deepequal")
	}
	if !kapi.Semantic.DeepEqual(copied, original) {
		t.Error("before mutation of copy, copied and original should be identical but are not, likely failure in deepequal")
	}

	// Mutate the copy
	for e := copied.Status.Namespaces.OrderedKeys().Front(); e != nil; e = e.Next() {
		k := e.Value.(string)
		ns, _ := copied.Status.Namespaces.Get(k)
		for k2, v2 := range ns.Hard {
			v2.Set(v2.Value() + 2)
			ns.Hard[k2] = v2
		}
		for k2, v2 := range ns.Used {
			v2.Set(v2.Value() + 1)
			ns.Used[k2] = v2
		}
		copied.Status.Namespaces.Insert(k, ns)
	}

	if !reflect.DeepEqual(check, original) {
		t.Error("after mutation of copy, check and original should be identical but are not, likely failure in deep copy (ensure custom DeepCopy is being used)")
	}
	if !kapi.Semantic.DeepEqual(check, original) {
		t.Error("after mutation of copy, check and original should be identical but are not, likely failure in deep copy (ensure custom DeepCopy is being used)")
	}

	if reflect.DeepEqual(original, copied) {
		t.Error("after mutation of copy, original and copied should be different but are not, likely failure in deep copy (ensure custom DeepCopy is being used)")
	}
	if kapi.Semantic.DeepEqual(original, copied) {
		t.Error("after mutation of copy, original and copied should be different but are not, likely failure in deep copy (ensure custom DeepCopy is being used)")
	}
}
