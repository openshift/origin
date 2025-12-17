package coreos

import (
	"context"
	"encoding/json"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mcoNamespace = "openshift-machine-config-operator"
	cmName       = "coreos-bootimages"
	cmKey        = "stream"
)

// stream is a subset of https://github.com/coreos/stream-metadata-go
// in the future we could vendor that, but it doesn't matter much
// right now, we're just sanity checking the config exists.
type stream struct {
	Stream string `json:"stream"`
}

// This test validates https://github.com/openshift/enhancements/pull/679
var _ = g.Describe("[sig-coreos] [Conformance] CoreOS bootimages", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("coreos")

	g.It("TestBootimagesPresent [apigroup:machineconfiguration.openshift.io]", g.Label("Size:S"), func() {
		client := oc.AdminKubeClient()
		cm, err := client.CoreV1().ConfigMaps(mcoNamespace).Get(context.Background(), cmName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		d, ok := cm.Data[cmKey]
		o.Expect(ok).To(o.BeTrue())
		var st stream
		err = json.Unmarshal([]byte(d), &st)
		o.Expect(err).To(o.BeNil())
		o.Expect(len(st.Stream)).ToNot(o.BeZero())
	})
})
