package apiserver

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("apiserver-resource-quota")

	const resourceQuotaYAML = `
apiVersion: v1
kind: ResourceQuota
metadata:
  name: quota
spec:
  hard:
    cpu: "20"
    memory: 1Gi
    persistentvolumeclaims: "1"
    pods: "10"
    replicationcontrollers: "20"
    resourcequotas: "1"
    secrets: "10"
    services: "5"
`
	const pvcYAML = `
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim-1
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 3Gi
`

	g.It("APIServer The number of created persistent volume claims can not exceed the limitation", func() {
		client, err := dynamic.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}
		gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}

		resourceQuota := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(resourceQuotaYAML), &resourceQuota.Object); err != nil {
			g.Fail(err.Error())
		}

		output, err := client.Resource(gvr).Create(context.Background(), resourceQuota, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.MatchRegexp(`[Rr]esourcequota.*quota.*created`))

		client, err = dynamic.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}
		gvr = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}

		pvc := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(pvcYAML), &pvc.Object); err != nil {
			g.Fail(err.Error())
		}
		output, err = client.Resource(gvr).Create(context.Background(), pvc, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.MatchRegexp(`[Pp]ersistentvolumeclaim.*myclaim-1.*created`))

		output, err = client.Resource(gvr).Create(context.Background(), pvc, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(output).To(o.MatchRegexp(`[Ee]rror.*when creat.*myclaim-1.*forbidden.*[Ee]xceeded quota`))
	})
})
