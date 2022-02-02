package aws

import (
	"context"

	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/objx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

func FetchResourceTags(clusterInfra objx.Map) objx.Map {
	awsPlatformSpec := clusterInfra.Get("spec.platformSpec.aws").ObjxMap()
	o.Expect(awsPlatformSpec).Should(o.HaveKey("resourceTags"))

	tagset := make(map[string]interface{})
	for _, tag := range awsPlatformSpec.Get("resourceTags").MustObjxMapSlice() {
		tagset[tag.Get("key").MustStr()] = tag.Get("value").MustStr()
	}

	return objx.Map(tagset)
}

func VerifyResourceTags(resourceTags objx.Map, tagList map[string]string) {
	for k, v := range resourceTags {
		o.Expect(tagList).Should(o.HaveKeyWithValue(k, v))
	}
}

func UpdateResourceTags(cfgClient dynamic.NamespaceableResourceInterface, infraobj *unstructured.Unstructured, tag v1.AWSResourceTag) *unstructured.Unstructured {
	// verify infrastructure values and update user tags
	var infra v1.Infrastructure
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(infraobj.UnstructuredContent(), &infra)
	o.Expect(err).NotTo(o.HaveOccurred())

	infra.Spec.PlatformSpec.AWS.ResourceTags = append(infra.Spec.PlatformSpec.AWS.ResourceTags, tag)
	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&infra)
	o.Expect(err).NotTo(o.HaveOccurred())

	var updatedCfg unstructured.Unstructured
	updatedCfg.Object = res
	updatedInfra, err := cfgClient.Update(context.Background(), &updatedCfg, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return updatedInfra
}
