package api

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/conversion"
)

func DeepCopy_api_ResourceQuotasStatusByNamespace(in1 interface{}, out1 interface{}, c *conversion.Cloner) error {
	in := in1.(*ResourceQuotasStatusByNamespace)
	out := out1.(*ResourceQuotasStatusByNamespace)
	for e := in.OrderedKeys().Front(); e != nil; e = e.Next() {
		namespace := e.Value.(string)
		status, _ := in.Get(namespace)

		outstatus := &kapi.ResourceQuotaStatus{}
		kapi.DeepCopy_api_ResourceQuotaStatus(&status, outstatus, c)

		if out == nil {
			out = &ResourceQuotasStatusByNamespace{}
		}
		out.Insert(namespace, *outstatus)
	}
	return nil
}
