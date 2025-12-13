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

	oc := exutil.NewCLI("apiserver")

	g.It("validates APIServer in config.openshift.io/v1", g.Label("Size:S"), func() {
		client, err := dynamic.NewForConfig(oc.AdminConfig())
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}
		gvr := schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "apiservers"}

		invalid := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(invalidAPIServer), &invalid.Object); err != nil {
			g.Fail(err.Error())
		}
		_, err = client.Resource(gvr).Create(context.Background(), invalid, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())

		if err != nil {
			o.Expect(err.Error()).To(o.ContainSubstring("may not be set"))
			o.Expect(err.Error()).To(o.ContainSubstring("spec.servingCerts.defaultServingCertificate.name"))
		}
	})
})

const invalidAPIServer = `
kind: APIServer
apiVersion: config.openshift.io/v1
metadata:
  name: invalid
spec:
  servingCerts:
    defaultServingCertificate:
      name: foo # this must be empty
`
