package apiserver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/test/library"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift/library-go/pkg/crypto"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Serial][Slow]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver")

	g.It("TestTLSModernProfile", func() {
		ctx := context.TODO()

		t := g.GinkgoT()

		configClient := oc.AdminConfigClient()
		apiServerPodClient := oc.AdminKubeClient().CoreV1().Pods("openshift-kube-apiserver")
		etcdPodClient := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd")

		initialConfigState, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var applyPatch, removePatch string

		if initialConfigState.Spec.TLSSecurityProfile != nil {
			jsonStr, err := json.Marshal(initialConfigState.Spec.TLSSecurityProfile)
			o.Expect(err).NotTo(o.HaveOccurred())

			// If TLSSecurityProfile is already set, preserve and replace it

			applyPatch = `[{"op":"replace","path":"/spec/tlsSecurityProfile","value":{"type":"Modern","modern":{}}}]`
			removePatch = fmt.Sprintf(`[{"op":"replace","path":"/spec/tlsSecurityProfile",value:%s}]`, jsonStr)
		} else {
			applyPatch = `[{"op":"add","path":"/spec/tlsSecurityProfile","value":{"type":"Modern","modern":{}}}]`
			removePatch = `[{"op":"remove","path":"/spec/tlsSecurityProfile"}]`
		}

		t.DeferCleanup(func(ctx context.Context) {
			g.By("Cleanup - removing TLS profile")

			_, err := configClient.ConfigV1().APIServers().Patch(ctx, "cluster", types.JSONPatchType, []byte(removePatch), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = library.WaitForPodsToStabilizeOnTheSameRevision(t, apiServerPodClient, "app=openshift-kube-apiserver", 5, 24*time.Second, 5*time.Second, 30*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("Checking if TLS 1.2 is usable before the modern TLS profile is applied")

		// We're going to be dialing TCP directly, not connecting over HTTP as usual, so we don't want the protocol on the host.
		tlsHost := strings.TrimPrefix(oc.AdminConfig().Host, "https://")

		config := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

		conn, err := tls.Dial("tcp4", tlsHost, config)
		o.Expect(err).NotTo(o.HaveOccurred())

		conn.Close()

		g.By("Applying a JSON Patch to use the modern TLS profile")

		_, err = configClient.ConfigV1().APIServers().Patch(ctx, "cluster", types.JSONPatchType, []byte(applyPatch), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for etcd to stabilize")

		err = library.WaitForPodsToStabilizeOnTheSameRevision(t, etcdPodClient, "app=etcd", 5, 24*time.Second, 5*time.Second, 30*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the API server to stabilize")

		err = library.WaitForPodsToStabilizeOnTheSameRevision(t, apiServerPodClient, "app=openshift-kube-apiserver", 5, 24*time.Second, 5*time.Second, 30*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Dialing the API with a minimum TLS version of 1.3 and expecting success")

		config = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}

		conn, err = tls.Dial("tcp4", tlsHost, config)
		o.Expect(err).NotTo(o.HaveOccurred())

		conn.Close()

		g.By("Dialing the API with a minimum TLS version of 1.2 and expecting failure")

		config = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

		_, err = tls.Dial("tcp4", tlsHost, config)
		o.Expect(err).To(o.HaveOccurred())
	})
})

var _ = g.Describe("[sig-api-machinery][Feature:APIServer]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("apiserver")

	g.It("TestTLSDefaults", func() {
		t := g.GinkgoT()
		// Verify we fail with TLS versions less than the default, and work with TLS versions >= the default
		for _, tlsVersionName := range crypto.ValidTLSVersions() {
			tlsVersion := crypto.TLSVersionOrDie(tlsVersionName)
			expectSuccess := tlsVersion >= crypto.DefaultTLSVersion()
			config := &tls.Config{MinVersion: tlsVersion, MaxVersion: tlsVersion, InsecureSkipVerify: true}

			// We're going to be dialing TCP directly, not connecting over HTTP as usual, so we don't want the protocol on the host.
			host := strings.TrimPrefix(oc.AdminConfig().Host, "https://")

			{
				conn, err := tls.Dial("tcp4", host, config)
				if err == nil {
					conn.Close()
				}
				if success := err == nil; success != expectSuccess {
					t.Errorf("Expected success %v, got %v with TLS version %s dialing master", expectSuccess, success, tlsVersionName)
				}
			}
		}

		// Verify the only ciphers we work with are in the default set.
		// Not all default ciphers will succeed because they depend on the serving cert type.
		defaultCiphers := map[uint16]bool{}
		for _, defaultCipher := range crypto.DefaultCiphers() {
			defaultCiphers[defaultCipher] = true
		}
		for _, cipherName := range crypto.ValidCipherSuites() {
			cipher, err := crypto.CipherSuite(cipherName)
			if err != nil {
				t.Fatal(err)
			}
			expectFailure := !defaultCiphers[cipher]
			config := &tls.Config{CipherSuites: []uint16{cipher}, InsecureSkipVerify: true}

			{
				conn, err := tls.Dial("tcp4", oc.AdminConfig().Host, config)
				if err == nil {
					conn.Close()
					if expectFailure {
						t.Errorf("Expected failure on cipher %s, got success dialing master", cipherName)
					}
				}
			}
		}

	})
})
