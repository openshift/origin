package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	admissionapi "k8s.io/pod-security-admission/api"
)

func randomString() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

var _ = g.Describe("[sig-cli] oc fauxtests", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("fauxtests", admissionapi.LevelBaseline)

	g.It(fmt.Sprintf("should pass the %s faux test", randomString()), func() {
		out, err := oc.Run("version").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Client"))
	})

	g.It(fmt.Sprintf("should list BuildConfigs in default namespace %s", randomString()), func() {
		out, err := oc.Run("get").Args("buildconfigs", "-n", "default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("should be able to contact the foo endpoint", func() {
		host := "127.0.0.1"
		port := 8080
		url := fmt.Sprintf("http://%s:%d/foo", host, port)
		resp, err := http.Get(url)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer resp.Body.Close()
		o.Expect(resp.StatusCode).To(o.Equal(http.StatusOK))
	})

	g.It("should have at least three nodes", func() {
		out, err := oc.Run("get").Args("nodes", "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		lines := strings.Split(strings.TrimSpace(out), "\n")
		o.Expect(len(lines)).To(o.BeNumerically(">=", 3), "expected at least 3 nodes, got %d", len(lines))
	})
})
