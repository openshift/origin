package apiserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/test/library"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned"
	"github.com/openshift/library-go/pkg/crypto"
	exutil "github.com/openshift/origin/test/extended/util"
)

const tlsTestDuration = 45 * time.Minute
const tlsWaitForCleanupDuration = 10 * time.Minute

var _ = g.Describe("[sig-api-machinery][Feature:APIServer][Serial]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("apiserver")

	g.It("TestTLSModernProfile", func() {
		ctx, ctxCancelFn := context.WithTimeout(context.Background(), tlsTestDuration)
		defer ctxCancelFn()

		t := g.GinkgoT()

		configClient := oc.AdminConfigClient()

		operatorClient := oc.AdminOperatorClient()

		defer func() {
			g.By("Cleanup - removing TLS profile")

			cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), tlsWaitForCleanupDuration)
			defer cancelCleanup()

			kasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(cleanupCtx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			currentRevision := kasStatus.Status.LatestAvailableRevision

			removeModernProfilePatch := `[{"op":"remove","path":"/spec/tlsSecurityProfile"}]`

			_, err = configClient.ConfigV1().APIServers().Patch(cleanupCtx, "cluster", types.JSONPatchType, []byte(removeModernProfilePatch), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = waitForKASIncrementedRevision(cleanupCtx, operatorClient, "cluster", currentRevision)
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		g.By("Checking if TLS 1.2 is usable before the modern TLS profile is applied")

		// We're going to be dialing TCP directly, not connecting over HTTP as usual, so we don't want the protocol on the host.
		tlsHost := strings.TrimPrefix(oc.AdminConfig().Host, "https://")

		config := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

		conn, err := tls.Dial("tcp4", tlsHost, config)
		if err != nil {
			t.Fatalf("Expected success with TLS 1.2 using default profile, got %v", err)
		} else {
			t.Log("TLS 1.2 is usable")
		}

		conn.Close()

		g.By("Applying a JSON Patch to use the modern TLS profile")

		kasStatus, err := operatorClient.OperatorV1().KubeAPIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		currentKASRevision := kasStatus.Status.LatestAvailableRevision

		addModernProfilePatch := `[{"op":"add","path":"/spec/tlsSecurityProfile","value":{"type":"Modern","modern":{}}}]`

		_, err = configClient.ConfigV1().APIServers().Patch(ctx, "cluster", types.JSONPatchType, []byte(addModernProfilePatch), metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for etcd to stabilize")

		podClient := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd")
		err = library.WaitForPodsToStabilizeOnTheSameRevision(t, podClient, "app=etcd", 5, 24*time.Second, 5*time.Second, 30*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the API server to stabilize")

		_, err = waitForKASIncrementedRevision(ctx, operatorClient, "cluster", currentKASRevision)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Dialing the API with a minimum TLS version of 1.3 and expecting success")

		config = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}

		conn, err = tls.Dial("tcp4", tlsHost, config)
		if err != nil {
			t.Fatalf("Expected success with TLS 1.3, got %v", err)
		} else {
			t.Log("TLS 1.3 is usable")
		}

		conn.Close()

		g.By("Dialing the API with a minimum TLS version of 1.2 and expecting failure")

		config = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

		conn, err = tls.Dial("tcp4", tlsHost, config)
		if err == nil {
			t.Fatalf("Expected failure with TLS 1.2, got success")
			conn.Close()
		} else {
			t.Log("TLS 1.2 is not usable")
		}

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

func waitForKASIncrementedRevision(ctx context.Context, operatorClient operatorv1client.Interface, name string, currentRevision int32) (int32, error) {
	for {
		kasStatus, _ := operatorClient.OperatorV1().KubeAPIServers().Get(context.Background(), name, metav1.GetOptions{})
		// Intentionally don't return if this errors, as the API server is most likely still coming online, which is what we're waiting for.

		if ctx.Err() != nil {
			return 0, fmt.Errorf("timed out waiting for KubeAPIServer to increment revision")
		}

		if kasStatus.Status.LatestAvailableRevision > currentRevision {
			return kasStatus.Status.LatestAvailableRevision, nil
		}

		time.Sleep(10 * time.Second)
	}
}
