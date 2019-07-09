package cli

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	oauthv1 "github.com/openshift/api/oauth/v1"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[cli] oc adm must-gather", func() {
	defer g.GinkgoRecover()
	oc := util.NewCLI("oc-adm-must-gather", util.KubeConfigPath()).AsAdmin()
	g.It("runs successfully", func() {
		// makes some tokens that should not show in the audit logs
		const tokenName = "must-gather-audit-logs-token-plus-some-padding-here-to-make-the-limit"
		oauthClient := oauthv1client.NewForConfigOrDie(oc.AdminConfig())
		_, err1 := oauthClient.OAuthAccessTokens().Create(&oauthv1.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			ClientName:  "openshift-challenging-client",
			ExpiresIn:   30,
			Scopes:      []string{"user:info"},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
			UserName:    "a",
			UserUID:     "1",
		})
		o.Expect(err1).ToNot(o.HaveOccurred())
		_, err2 := oauthClient.OAuthAuthorizeTokens().Create(&oauthv1.OAuthAuthorizeToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			ClientName:  "openshift-challenging-client",
			ExpiresIn:   30,
			Scopes:      []string{"user:info"},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
			UserName:    "a",
			UserUID:     "1",
		})
		o.Expect(err2).ToNot(o.HaveOccurred())
		// let audit log writes occurs to disk (best effort, should be enough to make the test fail most of the time)
		time.Sleep(10 * time.Second)

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).ToNot(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		o.Expect(oc.Run("adm", "must-gather").Args("--dest-dir", tempDir).Execute()).To(o.Succeed())

		auditDirectories := [][]string{
			{tempDir, "audit_logs", "kube-apiserver"},
			{tempDir, "audit_logs", "openshift-apiserver"},
		}

		expectedDirectories := append([][]string{
			{tempDir, "cluster-scoped-resources", "config.openshift.io"},
			{tempDir, "cluster-scoped-resources", "operator.openshift.io"},
			{tempDir, "cluster-scoped-resources", "core"},
			{tempDir, "cluster-scoped-resources", "apiregistration.k8s.io"},
			{tempDir, "namespaces", "openshift"},
			{tempDir, "namespaces", "openshift-kube-apiserver-operator"},
		}, auditDirectories...)

		expectedFiles := [][]string{
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "apiservers.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "authentications.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "builds.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "clusteroperators.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "clusterversions.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "consoles.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "dnses.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "featuregates.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "images.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "infrastructures.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "ingresses.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "networks.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "oauths.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "projects.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "schedulers.yaml"},
			{tempDir, "namespaces", "openshift-kube-apiserver", "core", "configmaps.yaml"},
			{tempDir, "namespaces", "openshift-kube-apiserver", "core", "secrets.yaml"},
			{tempDir, "audit_logs", "kube-apiserver.audit_logs_listing"},
			{tempDir, "audit_logs", "openshift-apiserver.audit_logs_listing"},
			{tempDir, "host_service_logs", "masters", "crio_service.log"},
			{tempDir, "host_service_logs", "masters", "kubelet_service.log"},
		}

		for _, expectedDirectory := range expectedDirectories {
			o.Expect(path.Join(expectedDirectory...)).To(o.BeADirectory())
		}

		emptyFiles := []string{}
		for _, expectedFile := range expectedFiles {
			expectedFilePath := path.Join(expectedFile...)
			o.Expect(expectedFilePath).To(o.BeAnExistingFile())
			stat, err := os.Stat(expectedFilePath)
			o.Expect(err).ToNot(o.HaveOccurred())
			if size := stat.Size(); size < 50 {
				emptyFiles = append(emptyFiles, expectedFilePath)
			}
		}
		if len(emptyFiles) > 0 {
			o.Expect(fmt.Errorf("expected files should not be empty: %s", strings.Join(emptyFiles, ","))).NotTo(o.HaveOccurred())
		}

		// make sure we do not log OAuth tokens
		for _, auditDirectory := range auditDirectories {
			eventsChecked := 0
			err := filepath.Walk(path.Join(auditDirectory...), func(path string, info os.FileInfo, err error) error {
				g.By(path)
				o.Expect(err).ToNot(o.HaveOccurred())
				if info.IsDir() {
					return nil
				}

				readFile := false

				file, err := os.Open(path)
				o.Expect(err).ToNot(o.HaveOccurred())
				defer file.Close()

				gzipReader, err := gzip.NewReader(file)
				o.Expect(err).ToNot(o.HaveOccurred())

				scanner := bufio.NewScanner(gzipReader)
				for scanner.Scan() {
					text := scanner.Text()
					if !strings.HasSuffix(text, "}") {
						continue // ignore truncated data
					}
					o.Expect(text).To(o.HavePrefix(`{"kind":"Event",`))
					for _, token := range []string{"oauthaccesstokens", "oauthauthorizetokens", tokenName} {
						o.Expect(text).ToNot(o.ContainSubstring(token))
					}
					readFile = true
					eventsChecked++
				}
				// ignore this error as we usually fail to read the whole GZ file
				// o.Expect(scanner.Err()).ToNot(o.HaveOccurred())
				o.Expect(readFile).To(o.BeTrue())

				return nil
			})
			o.Expect(err).ToNot(o.HaveOccurred())
			o.Expect(eventsChecked).To(o.BeNumerically(">", 10000))
		}
	})

	g.It("runs successfully with options", func() {
		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).ToNot(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		args := []string{
			"--dest-dir", tempDir,
			"--source-dir", "/artifacts",
			"--",
			"/bin/bash", "-c",
			"ls -l > /artifacts/ls.log",
		}
		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())
		expectedFilePath := path.Join(tempDir, "ls.log")
		o.Expect(expectedFilePath).To(o.BeAnExistingFile())
		stat, err := os.Stat(expectedFilePath)
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(stat.Size()).To(o.BeNumerically(">", 0))
	})
})
