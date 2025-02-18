package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	operatorv1 "github.com/openshift/api/operator/v1"
	applyoperatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	"github.com/openshift/library-go/pkg/apiserver/jsonpatch"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"

	"github.com/openshift/origin/test/extended/testdata"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery] JSON Patch [apigroup:operator.openshift.io]", func() {
	ctx := context.TODO()
	gvr := operatorv1.GroupVersion.WithResource("kubeapiservers")
	gvk := operatorv1.GroupVersion.WithKind("KubeAPIServer")
	oc := exutil.NewCLIWithoutNamespace("json-patch")

	g.BeforeEach(func() {
		isManagedServiceCluster, err := exutil.IsManagedServiceCluster(ctx, oc.AdminKubeClient())
		o.Expect(err).ToNot(o.HaveOccurred())
		if isManagedServiceCluster {
			g.Skip("skipping JSON Patch tests on managed service cluster")
		}
	})

	g.It("should delete an entry from an array with a test precondition provided", func() {
		g.By("Creating KubeAPIServerOperator CR for the test")
		resourceClient := createResourceClient(oc.AdminConfig(), gvr)
		kasOperator := createWellKnownKubeAPIServerOperatorResource(ctx, resourceClient)

		g.By("Applying a JSON Patch to remove a node status at index 1")
		jsonPatch := jsonpatch.New().WithRemove("/status/nodeStatuses/1", jsonpatch.NewTestCondition("/status/nodeStatuses/1/nodeName", "master-2"))
		kasOperator, err := applyJSONPatch(ctx, kasOperator.Name, jsonPatch, resourceClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kasOperator.Status.NodeStatuses).To(o.Equal([]operatorv1.NodeStatus{
			{NodeName: "master-1"},
		}))
	})
	g.It("should delete multiple entries from an array when multiple test precondition provided", func() {
		g.By("Creating KubeAPIServerOperator CR for the test")
		resourceClient := createResourceClient(oc.AdminConfig(), gvr)
		kasOperator := createWellKnownKubeAPIServerOperatorResource(ctx, resourceClient)

		g.By("Applying a JSON Patch to remove a node status at index 0 and 1")
		jsonPatch := jsonpatch.New().
			WithRemove("/status/nodeStatuses/0", jsonpatch.NewTestCondition("/status/nodeStatuses/0/nodeName", "master-1")).
			WithRemove("/status/nodeStatuses/0", jsonpatch.NewTestCondition("/status/nodeStatuses/0/nodeName", "master-2"))
		kasOperator, err := applyJSONPatch(ctx, kasOperator.Name, jsonPatch, resourceClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kasOperator.Status.NodeStatuses).To(o.HaveLen(0))
	})
	g.It("should error when the test precondition provided doesn't match", func() {
		g.By("Creating KubeAPIServerOperator CR for the test")
		resourceClient := createResourceClient(oc.AdminConfig(), gvr)
		kasOperator := createWellKnownKubeAPIServerOperatorResource(ctx, resourceClient)

		g.By("Applying a JSON Patch to remove a node status at index 1")
		jsonPatch := jsonpatch.New().WithRemove("/status/nodeStatuses/1", jsonpatch.NewTestCondition("/status/nodeStatuses/1/nodeName", "master-1"))
		kasOperator, err := applyJSONPatch(ctx, kasOperator.Name, jsonPatch, resourceClient)
		o.Expect(k8serrors.IsInvalid(err)).To(o.BeTrue(), fmt.Sprintf("unexpected error received = %v", err))
	})

	g.It("should delete an entry from an array with multiple field owners", func() {
		g.By("Creating KubeAPIServerOperator CR for the test")
		resourceClient := createResourceClient(oc.AdminConfig(), gvr)
		kasOperator := createWellKnownKubeAPIServerOperatorResource(ctx, resourceClient)

		g.By("Updating current revision for a node status at index 0 via Apply as manager-1")
		kasOperator.Status.NodeStatuses[0].CurrentRevision = 1
		kasOperator = applyStaticPodStatus(ctx, gvk, kasOperator, "manager-1", resourceClient)

		g.By("Updating current revision for a node status at index 0 via Apply as manager-2")
		kasOperator.Status.NodeStatuses[0].CurrentRevision = 2
		kasOperator = applyStaticPodStatus(ctx, gvk, kasOperator, "manager-2", resourceClient)

		g.By("Dropping a node status at index 0 via Apply as manager-1 (entry not removed)")
		kasOperator.Status.NodeStatuses = kasOperator.Status.NodeStatuses[1:]
		kasOperator = applyStaticPodStatus(ctx, gvk, kasOperator, "manager-1", resourceClient)
		o.Expect(kasOperator.Status.NodeStatuses).To(o.Equal([]operatorv1.NodeStatus{
			{NodeName: "master-1", CurrentRevision: 2},
			{NodeName: "master-2"},
		}))

		g.By("Applying a JSON Patch to remove a node status at index 0")
		jsonPatch := jsonpatch.New().WithRemove("/status/nodeStatuses/0", jsonpatch.NewTestCondition("/status/nodeStatuses/0/nodeName", "master-1"))
		kasOperator, err := applyJSONPatch(ctx, kasOperator.Name, jsonPatch, resourceClient)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(kasOperator.Status.NodeStatuses).To(o.Equal([]operatorv1.NodeStatus{
			{NodeName: "master-2"},
		}))
	})
})

func createWellKnownKubeAPIServerOperatorResource(ctx context.Context, resourceClient dynamic.ResourceInterface) *operatorv1.KubeAPIServer {
	kasOperatorYaml := testdata.MustAsset("test/extended/testdata/apiserver/operator-kube-apiserver-cr.yaml")
	unstructuredKasOperatorManifest := resourceread.ReadUnstructuredOrDie(kasOperatorYaml)
	manifest, _ := json.Marshal(unstructuredKasOperatorManifest)
	framework.Logf("JSONPATCH: unstructuredKasOperatorManifest %s", string(manifest))
	unstructuredKasOperator, err := resourceClient.Create(ctx, unstructuredKasOperatorManifest, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	manifest2, _ := json.Marshal(unstructuredKasOperator)
	framework.Logf("JSONPATCH: unstructuredKasOperator %s", string(manifest2))

	kasOperator := unstructuredToKubeAPIServerOperator(unstructuredKasOperator.Object)
	manifest3, _ := json.Marshal(kasOperator)
	framework.Logf("JSONPATCH: kasOperator %s", string(manifest3))
	kasOperatorFromManifest := unstructuredToKubeAPIServerOperator(unstructuredKasOperatorManifest.Object)
	manifest4, _ := json.Marshal(kasOperatorFromManifest)
	framework.Logf("JSONPATCH: kasOperatorFromManifest %s", string(manifest4))
	kasOperator.Status = kasOperatorFromManifest.Status
	manifest5, _ := json.Marshal(kasOperator)
	framework.Logf("JSONPATCH: kasOperator with status %s", string(manifest5))
	unstructuredKasOperator, err = resourceClient.UpdateStatus(ctx, kubeAPIServerOperatorToUnstructured(kasOperator), metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	kasOperator = unstructuredToKubeAPIServerOperator(unstructuredKasOperator.Object)
	o.Expect(kasOperator.Status.NodeStatuses).To(o.Equal([]operatorv1.NodeStatus{
		{NodeName: "master-1"},
		{NodeName: "master-2"},
	}))

	g.DeferCleanup(func(ctx context.Context) {
		g.By(fmt.Sprintf("Cleaning up KubeAPIServerOperator %s CR for the test", kasOperator.Name))
		err := resourceClient.Delete(ctx, kasOperator.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	return kasOperator
}

func applyJSONPatch(ctx context.Context, name string, jsonPatch *jsonpatch.PatchSet, resourceClient dynamic.ResourceInterface) (*operatorv1.KubeAPIServer, error) {
	jsonPatchBytes, err := jsonPatch.Marshal()
	if err != nil {
		return nil, err
	}
	g.By(string(jsonPatchBytes))
	unstructuredKasOperator, err := resourceClient.Patch(ctx, name, types.JSONPatchType, jsonPatchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return nil, err
	}
	return unstructuredToKubeAPIServerOperator(unstructuredKasOperator.Object), nil
}

func applyStaticPodStatus(ctx context.Context, gvk schema.GroupVersionKind, kasOperator *operatorv1.KubeAPIServer, fieldManager string, resourceClient dynamic.ResourceInterface) *operatorv1.KubeAPIServer {
	statusApplyConfiguration := toStaticPodOperatorStatusApplyConfiguraiton(kasOperator)
	unstructuredKasOperator, err := resourceClient.ApplyStatus(ctx, kasOperator.Name, toUnstructuredKubeAPIServerOperatorStatusApplyConfiguration(gvk, kasOperator, statusApplyConfiguration), metav1.ApplyOptions{Force: true, FieldManager: fieldManager})
	o.Expect(err).NotTo(o.HaveOccurred())
	kasOperator = unstructuredToKubeAPIServerOperator(unstructuredKasOperator.Object)
	return kasOperator
}

func toStaticPodOperatorStatusApplyConfiguraiton(kasOperator *operatorv1.KubeAPIServer) *applyoperatorv1.StaticPodOperatorStatusApplyConfiguration {
	nodeStatusApplyConfiguration := toNodeStatusesToNodeStatusApplyConfiguration(kasOperator)
	return applyoperatorv1.StaticPodOperatorStatus().WithNodeStatuses(nodeStatusApplyConfiguration...)
}

func toNodeStatusesToNodeStatusApplyConfiguration(kasOpeartor *operatorv1.KubeAPIServer) []*applyoperatorv1.NodeStatusApplyConfiguration {
	ret := []*applyoperatorv1.NodeStatusApplyConfiguration{}
	for _, nodeStatus := range kasOpeartor.Status.NodeStatuses {
		nodeStatusConfig := applyoperatorv1.NodeStatus().
			WithNodeName(nodeStatus.NodeName).
			WithTargetRevision(nodeStatus.TargetRevision).
			WithCurrentRevision(nodeStatus.CurrentRevision).
			WithLastFailedRevision(nodeStatus.LastFailedRevision).
			WithLastFailedReason(nodeStatus.LastFailedReason).
			WithLastFailedCount(nodeStatus.LastFailedCount).
			WithLastFallbackCount(nodeStatus.LastFallbackCount).
			WithLastFailedRevisionErrors(nodeStatus.LastFailedRevisionErrors...)
		if nodeStatus.LastFailedTime != nil {
			nodeStatusConfig.WithLastFailedTime(*nodeStatusConfig.LastFailedTime)
		}
		ret = append(ret, nodeStatusConfig)
	}
	return ret
}

func toUnstructuredKubeAPIServerOperatorStatusApplyConfiguration(gvk schema.GroupVersionKind, kasOperator *operatorv1.KubeAPIServer, desiredKasOperatorConfiguration *applyoperatorv1.StaticPodOperatorStatusApplyConfiguration) *unstructured.Unstructured {
	unstructuredStatusApplyConfiguration, err := runtime.DefaultUnstructuredConverter.ToUnstructured(desiredKasOperatorConfiguration)
	o.Expect(err).NotTo(o.HaveOccurred())

	ret := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": unstructuredStatusApplyConfiguration,
		},
	}
	ret.SetGroupVersionKind(gvk)
	ret.SetName(kasOperator.Name)

	return ret
}

func unstructuredToKubeAPIServerOperator(obj map[string]interface{}) *operatorv1.KubeAPIServer {
	ret := &operatorv1.KubeAPIServer{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj, ret)
	o.Expect(err).NotTo(o.HaveOccurred())
	return ret
}

func kubeAPIServerOperatorToUnstructured(kasOperator *operatorv1.KubeAPIServer) *unstructured.Unstructured {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(kasOperator)
	o.Expect(err).NotTo(o.HaveOccurred())
	return &unstructured.Unstructured{Object: raw}
}

func createResourceClient(cfg *rest.Config, gvr schema.GroupVersionResource) dynamic.ResourceInterface {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	o.Expect(err).NotTo(o.HaveOccurred())
	return dynamicClient.Resource(gvr)
}
