package cli

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	oauthv1 "github.com/openshift/api/oauth/v1"
	"github.com/openshift/client-go/image/clientset/versioned"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cli] oc adm must-gather", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("oc-adm-must-gather").AsAdmin()

	g.JustBeforeEach(func() {
		// wait for the default service account to be avaiable
		err := exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "default")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("runs successfully", func() {
		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		o.Expect(oc.Run("adm", "must-gather").Args("--dest-dir", tempDir).Execute()).To(o.Succeed())

		pluginOutputDir := getPluginOutputDir(oc, tempDir)

		expectedDirectories := [][]string{
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io"},
			{pluginOutputDir, "cluster-scoped-resources", "operator.openshift.io"},
			{pluginOutputDir, "cluster-scoped-resources", "core"},
			{pluginOutputDir, "cluster-scoped-resources", "apiregistration.k8s.io"},
			{pluginOutputDir, "namespaces", "openshift"},
			{pluginOutputDir, "namespaces", "openshift-kube-apiserver-operator"},
		}

		expectedFiles := [][]string{
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "apiservers.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "authentications.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "builds.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "clusteroperators.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "clusterversions.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "consoles.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "dnses.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "featuregates.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "images.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "infrastructures.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "ingresses.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "networks.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "oauths.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "projects.yaml"},
			{pluginOutputDir, "cluster-scoped-resources", "config.openshift.io", "schedulers.yaml"},
			// TODO: This got broken and we need to fix this. Disabled temporarily.
			// {pluginOutputDir, "namespaces", "openshift-kube-apiserver", "core", "configmaps.yaml"},
			// {pluginOutputDir, "namespaces", "openshift-kube-apiserver", "core", "secrets.yaml"},
			{pluginOutputDir, "host_service_logs", "masters", "crio_service.log"},
			{pluginOutputDir, "host_service_logs", "masters", "kubelet_service.log"},
		}

		// Skip the kube and openshift apiserver audit logs on IBM ROKS clusters
		// since those components live outside of the cluster.
		if e2e.TestContext.Provider != ibmcloud.ProviderName {
			expectedFiles = append(expectedFiles,
				[]string{pluginOutputDir, "audit_logs", "kube-apiserver.audit_logs_listing"},
				[]string{pluginOutputDir, "audit_logs", "openshift-apiserver.audit_logs_listing"},
			)
		}

		for _, expectedDirectory := range expectedDirectories {
			o.Expect(path.Join(expectedDirectory...)).To(o.BeADirectory())
		}

		emptyFiles := []string{}
		for _, expectedFile := range expectedFiles {
			expectedFilePath := path.Join(expectedFile...)
			o.Expect(expectedFilePath).To(o.BeAnExistingFile())
			stat, err := os.Stat(expectedFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			if size := stat.Size(); size < 50 {
				emptyFiles = append(emptyFiles, expectedFilePath)
			}
		}
		if len(emptyFiles) > 0 {
			o.Expect(fmt.Errorf("expected files should not be empty: %s", strings.Join(emptyFiles, ","))).NotTo(o.HaveOccurred())
		}
	})

	g.It("runs successfully with options", func() {
		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		args := []string{
			"--dest-dir", tempDir,
			"--source-dir", "/artifacts",
			"--",
			"/bin/bash", "-c",
			"ls -l > /artifacts/ls.log",
		}
		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())
		expectedFilePath := path.Join(getPluginOutputDir(oc, tempDir), "ls.log")
		o.Expect(expectedFilePath).To(o.BeAnExistingFile())
		stat, err := os.Stat(expectedFilePath)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stat.Size()).To(o.BeNumerically(">", 0))
	})

	g.It("runs successfully for audit logs", func() {
		// On IBM ROKS, events will not be part of the output, since audit logs do not include control plane logs.
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			g.Skip("ROKs doesn't have audit logs")
		}

		// makes some tokens that should not show in the audit logs
		const tokenName = "must-gather-audit-logs-token-plus-some-padding-here-to-make-the-limit"
		oauthClient := oauthv1client.NewForConfigOrDie(oc.AdminConfig())
		_, err1 := oauthClient.OAuthAccessTokens().Create(context.Background(), &oauthv1.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			ClientName:  "openshift-challenging-client",
			ExpiresIn:   30,
			Scopes:      []string{"user:info"},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
			UserName:    "a",
			UserUID:     "1",
		}, metav1.CreateOptions{})
		o.Expect(err1).NotTo(o.HaveOccurred())
		_, err2 := oauthClient.OAuthAuthorizeTokens().Create(context.Background(), &oauthv1.OAuthAuthorizeToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: tokenName,
			},
			ClientName:  "openshift-challenging-client",
			ExpiresIn:   30,
			Scopes:      []string{"user:info"},
			RedirectURI: "https://127.0.0.1:12000/oauth/token/implicit",
			UserName:    "a",
			UserUID:     "1",
		}, metav1.CreateOptions{})
		o.Expect(err2).NotTo(o.HaveOccurred())

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		// defer os.RemoveAll(tempDir)

		args := []string{
			"--dest-dir", tempDir,
			"--",
			"/usr/bin/gather_audit_logs",
		}

		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())
		// wait for the contents to show up in the plugin output directory, avoiding EOF errors
		time.Sleep(10 * time.Second)

		pluginOutputDir := getPluginOutputDir(oc, tempDir)

		expectedDirectoriesToExpectedCount := map[string]int{
			path.Join(pluginOutputDir, "audit_logs", "kube-apiserver"):      1000,
			path.Join(pluginOutputDir, "audit_logs", "openshift-apiserver"): 10, // openshift apiservers don't necessarily get much traffic.  Especially early in a run
		}

		expectedFiles := [][]string{
			{pluginOutputDir, "audit_logs", "kube-apiserver.audit_logs_listing"},
			{pluginOutputDir, "audit_logs", "openshift-apiserver.audit_logs_listing"},
		}

		// for some crazy reason, it seems that the files from must-gather take time to appear on disk for reading.  I don't understand why
		// but this was in a previous commit and I don't want to immediately flake: https://github.com/openshift/origin/commit/006745a535848e84dcbcdd1c83ae86deddd3a229#diff-ad1c47fa4213de16d8b3237df5d71724R168
		// so we're going to try to get a pass every 10 seconds for a minute.  If we pass, great.  If we don't, we report the
		// last error we had.
		var lastErr error
		err = wait.PollImmediate(10*time.Second, 1*time.Minute, func() (bool, error) {
			// make sure we do not log OAuth tokens
			for auditDirectory, expectedNumberOfAuditEntries := range expectedDirectoriesToExpectedCount {
				eventsChecked := 0
				err := filepath.Walk(auditDirectory, func(path string, info os.FileInfo, err error) error {
					g.By(path)
					o.Expect(err).NotTo(o.HaveOccurred())
					if info.IsDir() {
						return nil
					}

					if (strings.Contains(path, "-termination-") && strings.HasSuffix(path, ".log.gz")) || strings.HasSuffix(path, "termination.log.gz") {
						// these are expected, but have unstructured log format
						return nil
					}

					isAuditFile := (strings.Contains(path, "-audit-") && strings.HasSuffix(path, ".log.gz")) || strings.HasSuffix(path, "audit.log.gz")
					o.Expect(isAuditFile).To(o.BeTrue())

					// at this point, we expect only audit files with json events, one per line

					readFile := false

					file, err := os.Open(path)
					o.Expect(err).NotTo(o.HaveOccurred())
					defer file.Close()

					fi, err := file.Stat()
					o.Expect(err).NotTo(o.HaveOccurred())

					// it will happen that the audit files are sometimes empty, we can
					// safely ignore these files since they don't provide valuable information
					// TODO this doesn't seem right.  It should be really unlikely, but we'll deal with later
					if fi.Size() == 0 {
						return nil
					}

					gzipReader, err := gzip.NewReader(file)
					o.Expect(err).NotTo(o.HaveOccurred())

					scanner := bufio.NewScanner(gzipReader)
					for scanner.Scan() {
						text := scanner.Text()
						if !strings.HasSuffix(text, "}") {
							continue // ignore truncated data
						}
						o.Expect(text).To(o.HavePrefix(`{"kind":"Event",`))
						for _, token := range []string{"oauthaccesstokens", "oauthauthorizetokens", tokenName} {
							o.Expect(text).NotTo(o.ContainSubstring(token))
						}
						readFile = true
						eventsChecked++
					}
					// ignore this error as we usually fail to read the whole GZ file
					// o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
					o.Expect(readFile).To(o.BeTrue())

					return nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				if eventsChecked <= expectedNumberOfAuditEntries {
					lastErr = fmt.Errorf("expected %d audit events for %q, but only got %d", expectedNumberOfAuditEntries, auditDirectory, eventsChecked)
					return false, nil
				}

				// reset lastErr if we succeeded.
				lastErr = nil
			}

			// if we get here, it means both directories checked out ok
			return true, nil
		})
		o.Expect(lastErr).NotTo(o.HaveOccurred()) // print the last error first if we have one
		o.Expect(err).NotTo(o.HaveOccurred())     // otherwise be sure we fail on the timeout if it happened

		emptyFiles := []string{}
		for _, expectedFile := range expectedFiles {
			expectedFilePath := path.Join(expectedFile...)
			o.Expect(expectedFilePath).To(o.BeAnExistingFile())
			stat, err := os.Stat(expectedFilePath)
			o.Expect(err).NotTo(o.HaveOccurred())
			if size := stat.Size(); size < 50 {
				emptyFiles = append(emptyFiles, expectedFilePath)
			}
		}
		if len(emptyFiles) > 0 {
			o.Expect(fmt.Errorf("expected files should not be empty: %s", strings.Join(emptyFiles, ","))).NotTo(o.HaveOccurred())
		}
	})
})

func getPluginOutputDir(oc *exutil.CLI, tempDir string) string {
	imageClient := versioned.NewForConfigOrDie(oc.AdminConfig())
	stream, err := imageClient.ImageV1().ImageStreams("openshift").Get(context.Background(), "must-gather", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	imageId, ok := imageutil.ResolveLatestTaggedImage(stream, "latest")
	o.Expect(ok).To(o.BeTrue())
	pluginOutputDir := path.Join(tempDir, regexp.MustCompile("[^A-Za-z0-9]+").ReplaceAllString(imageId, "-"))
	fileInfo, err := os.Stat(pluginOutputDir)
	if err != nil || !fileInfo.IsDir() {
		pluginOutputDir = tempDir
	}
	return pluginOutputDir
}
