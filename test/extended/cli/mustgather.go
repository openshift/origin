package cli

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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

	g.It("runs successfully [apigroup:config.openshift.io]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("oc must-gather doesn't currently support external controlPlaneTopology")
		}

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		o.Expect(oc.Run("adm", "must-gather").Args("--dest-dir", tempDir, "--volume-percentage=100").Execute()).To(o.Succeed())

		pluginOutputDir := GetPluginOutputDir(tempDir)

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

	g.It("runs successfully with options [apigroup:config.openshift.io]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("oc must-gather doesn't currently support external controlPlaneTopology")
		}

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)
		args := []string{
			"--dest-dir", tempDir,
			"--volume-percentage=100",
			"--source-dir", "/artifacts",
			"--",
			"/bin/bash", "-c",
			"ls -l > /artifacts/ls.log",
		}
		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())
		expectedFilePath := path.Join(GetPluginOutputDir(tempDir), "ls.log")
		o.Expect(expectedFilePath).To(o.BeAnExistingFile())
		stat, err := os.Stat(expectedFilePath)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(stat.Size()).To(o.BeNumerically(">", 0))
	})

	g.It("runs successfully for audit logs [apigroup:config.openshift.io][apigroup:oauth.openshift.io]", func() {
		// On External clusters, events will not be part of the output, since audit logs do not include control plane logs.
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("External clusters don't have control plane audit logs")
		}

		// makes some tokens that should not show in the audit logs
		_, sha256TokenName := exutil.GenerateOAuthTokenPair()
		oauthClient := oauthv1client.NewForConfigOrDie(oc.AdminConfig())
		_, err1 := oauthClient.OAuthAccessTokens().Create(context.Background(), &oauthv1.OAuthAccessToken{
			ObjectMeta: metav1.ObjectMeta{
				Name: sha256TokenName,
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
				Name: sha256TokenName,
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
		defer os.RemoveAll(tempDir)

		args := []string{
			"--dest-dir", tempDir,
			"--volume-percentage=100",
			"--",
			"/usr/bin/gather_audit_logs",
		}

		o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())
		// wait for the contents to show up in the plugin output directory, avoiding EOF errors
		time.Sleep(10 * time.Second)

		pluginOutputDir := GetPluginOutputDir(tempDir)

		expectedDirectoriesToExpectedCount := map[string]int{
			path.Join(pluginOutputDir, "audit_logs", "kube-apiserver"):      1000,
			path.Join(pluginOutputDir, "audit_logs", "openshift-apiserver"): 10, // openshift apiservers don't necessarily get much traffic.  Especially early in a run
			path.Join(pluginOutputDir, "audit_logs", "oauth-apiserver"):     10, // oauth apiservers don't necessarily get much traffic.  Especially early in a run
		}

		expectedFiles := [][]string{
			{pluginOutputDir, "audit_logs", "kube-apiserver.audit_logs_listing"},
			{pluginOutputDir, "audit_logs", "openshift-apiserver.audit_logs_listing"},
			{pluginOutputDir, "audit_logs", "oauth-apiserver.audit_logs_listing"},
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

					fileName := filepath.Base(path)
					if !IsAuditFile(fileName) {
						return nil
					}

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
			if size := stat.Size(); size < 20 {
				emptyFiles = append(emptyFiles, expectedFilePath)
			}
		}
		if len(emptyFiles) > 0 {
			o.Expect(fmt.Errorf("expected files should not be empty: %s", strings.Join(emptyFiles, ","))).NotTo(o.HaveOccurred())
		}
	})

	g.When("looking at the audit logs [apigroup:config.openshift.io]", func() {
		g.Describe("[sig-node] kubelet", func() {
			g.It("runs apiserver processes strictly sequentially in order to not risk audit log corruption", func() {
				controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				// On External clusters, events will not be part of the output, since audit logs do not include control plane logs.
				if *controlPlaneTopology == configv1.ExternalTopologyMode {
					g.Skip("External clusters don't have control plane audit logs")
				}

				tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
				o.Expect(err).NotTo(o.HaveOccurred())
				defer os.RemoveAll(tempDir)

				args := []string{
					"--dest-dir", tempDir,
					"--volume-percentage=100",
					"--",
					"/usr/bin/gather_audit_logs",
				}

				o.Expect(oc.Run("adm", "must-gather").Args(args...).Execute()).To(o.Succeed())

				pluginOutputDir := GetPluginOutputDir(tempDir)
				expectedAuditSubDirs := []string{"kube-apiserver", "openshift-apiserver", "oauth-apiserver"}

				seen := sets.String{}
				for _, apiserver := range expectedAuditSubDirs {
					err := filepath.Walk(filepath.Join(pluginOutputDir, "audit_logs", apiserver), func(path string, info os.FileInfo, err error) error {
						g.By(path)
						o.Expect(err).NotTo(o.HaveOccurred())
						if info.IsDir() {
							return nil
						}

						seen.Insert(apiserver)

						if filepath.Base(path) != "lock.log" {
							return nil
						}

						lockLog, err := ioutil.ReadFile(path)
						o.Expect(err).NotTo(o.HaveOccurred())

						// TODO: turn this into a failure as soon as kubelet is fixed
						result.Flakef("kubelet launched %s without waiting for the old process to terminate (lock was still hold): \n\n%s", apiserver, string(lockLog))
						return nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				o.Expect(seen.HasAll(expectedAuditSubDirs...), o.BeTrue())
			})
		})
	})

	// author: yinzhou@redhat.com
	g.It("runs successfully with node name option on hypershift hosted cluster timeout [apigroup:config.openshift.io][Timeout:20m]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology != configv1.ExternalTopologyMode {
			g.Skip("Non hypershift hosted cluster, skip test run")
		}

		output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("node", "-l", "hypershift.openshift.io/managed=true", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodes := strings.Fields(output)
		o.Expect(len(nodes)).To(o.BeNumerically(">", 0), "Expected at least one hypershift managed node")
		nodeName := nodes[0]

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		o.Expect(oc.Run("adm", "must-gather").Args("--node-name", nodeName, "--dest-dir", tempDir).Execute()).To(o.Succeed())
	})

	// author: yinzhou@redhat.com
	g.It("run the must-gather command with own name space [apigroup:config.openshift.io][Timeout:20m]", func() {
		g.By("Set namespace as privileged namespace")
		err := oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "pod-security.kubernetes.io/warn=privileged", "security.openshift.io/scc.podSecurityLabelSync=false", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "cluster-admin", "system:serviceaccount:"+oc.Namespace()+":default").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer exec.Command("bash", "-c", "rm -rf /tmp/must-gather-56929").Output()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer g.GinkgoRecover()
			defer wg.Done()
			_, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("--run-namespace", oc.Namespace(), "must-gather", "--source-dir=/must-gather/static-pods/", "--dest-dir=/tmp/must-gather-56929").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
		err = wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
			output, err1 := oc.AsAdmin().Run("get").Args("pod", "-n", oc.Namespace(), "-l", "app=must-gather", "-o=jsonpath={.items[0].status.phase}").Output()
			if err1 != nil {
				e2e.Logf("the err:%v, and try next round", err1)
				return false, nil
			}
			if matched, _ := regexp.MatchString("Running", output); matched {
				e2e.Logf("Check the must-gather pod running in own namespace\n")
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			oc.Run("get").Args("events", "-n", oc.Namespace()).Execute()
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "Cannot find the must-gather pod in own namespace")
		wg.Wait()
		e2e.Logf("Must-gather command completed, starting pod cleanup check")
		err = wait.Poll(10*time.Second, 600*time.Second, func() (bool, error) {
			output, err1 := oc.AsAdmin().Run("get").Args("pod", "-n", oc.Namespace(), "-l", "app=must-gather").Output()
			if err1 != nil {
				e2e.Logf("the err:%v, and try next round", err1)
				return false, nil
			}
			if matched, _ := regexp.MatchString("No resources found", output); matched {
				e2e.Logf("Check the must-gather pod disappeared in own namespace\n")
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Still find the must-gather pod in own namespace even wait for 10 mins")
	})

	// author: yinzhou@redhat.com
	g.It("Fetch audit logs of login attempts via oc commands timeout [apigroup:config.openshift.io][Timeout:20m]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Skip on hypershift (External topology)
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Hypershift clusters don't have oauth-server audit logs in must-gather")
		}

		g.By("run the must-gather")
		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather-51697.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir="+tempDir, "--", "/usr/bin/gather_audit_logs").Execute()).To(o.Succeed())

		g.By("check the must-gather result")
		oauth_audit_files := getOauthAudit(tempDir)
		for _, file := range oauth_audit_files {
			headContent, err := exec.Command("bash", "-c", fmt.Sprintf("zcat %v | head -n 1", file)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(headContent).To(o.ContainSubstring("auditID"), "Failed to read the oauth audit logs")
		}
	})

	// author: yinzhou@redhat.com
	g.It("must-gather support since and since-time flags timeout [apigroup:config.openshift.io][Timeout:20m]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Skip on hypershift (External topology)
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Hypershift clusters have different must-gather behavior")
		}

		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather-70982.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		g.By("Set namespace as privileged namespace")
		err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "pod-security.kubernetes.io/warn=privileged", "security.openshift.io/scc.podSecurityLabelSync=false", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("1. Test must-gather with correct since format should succeed.\n")
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--since=1m", "--dest-dir="+tempDir).Execute()).To(o.Succeed())

		g.By("2. Test must-gather with correct since format and special logs should succeed.\n")
		workerNodeList, err := exutil.GetClusterNodesByRole(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred())

		timeNow := getTimeFromNode(oc, workerNodeList[0], oc.Namespace())
		e2e.Logf("The time now is  %v", timeNow)
		timeStampOne := timeNow.Add(time.Minute * -5).Format("15:04:05")
		e2e.Logf("The time stamp is  %v", timeStampOne)
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--since=2m", "--dest-dir="+tempDir+"/mustgather2", "--", "/usr/bin/gather_network_logs").Execute()).To(o.Succeed())

		checkMustgatherLogTime(tempDir+"/mustgather2", workerNodeList[0], timeStampOne)

		g.By("3. Test must-gather with correct since-time format should succeed.\n")
		now := getTimeFromNode(oc, workerNodeList[0], oc.Namespace())
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--since-time="+now.Add(time.Minute*-2).Format("2006-01-02T15:04:05Z"), "--dest-dir="+tempDir).Execute()).To(o.Succeed())

		g.By("4. Test must-gather with correct since-time format and special logs should succeed.\n")
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--since-time="+now.Add(time.Minute*-1).Format("2006-01-02T15:04:05Z"), "--dest-dir="+tempDir, "--", "/usr/bin/gather_network_logs").Execute()).To(o.Succeed())

		g.By("5. Test must-gather with wrong since-time format should failed.\n")
		_, warningErr, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--since-time="+now.Format("2006-01-02"), "--dest-dir="+tempDir, "--", "/usr/bin/gather_network_logs").Outputs()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(strings.Contains(warningErr, "since-time only accepts times matching RFC3339")).To(o.BeTrue())

		g.By("6. Test must-gather with wrong since-time format should failed.\n")
		_, warningErr, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--since-time="+now.Format("2006-01-02T15:04:05"), "--dest-dir="+tempDir, "--", "/usr/bin/gather_network_logs").Outputs()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(strings.Contains(warningErr, "since-time only accepts times matching RFC3339")).To(o.BeTrue())
	})

	// author: yinzhou@redhat.com
	g.It("oc adm inspect should support since and sincetime [apigroup:config.openshift.io]", func() {
		tempDir, err := ioutil.TempDir("", "test.oc-adm-inspect-71212.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		g.By("Set namespace as privileged namespace")
		err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "pod-security.kubernetes.io/warn=privileged", "security.openshift.io/scc.podSecurityLabelSync=false", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("1. Test inspect with correct since-time format should succeed and gather correct logs.\n")
		workerNodeList, err := exutil.GetClusterNodesByRole(oc, "worker")
		o.Expect(err).NotTo(o.HaveOccurred())

		now := getTimeFromNode(oc, workerNodeList[0], oc.Namespace())
		timeStamp := now.Add(time.Minute * -5).Format("2006-01-02T15:04:05Z")

		podname, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", "openshift-multus", "-l", "app=multus", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("inspect", "ns", "openshift-multus", "--since-time="+now.Add(time.Minute*-2).Format("2006-01-02T15:04:05Z"), "--dest-dir="+tempDir).Execute()).To(o.Succeed())

		checkInspectLogTime(tempDir, podname, timeStamp)

		g.By("2. Test inspect with wrong since-time format should failed.\n")
		_, warningErr, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("inspect", "ns", "openshift-multus", "--since-time="+now.Format("2006-01-02T15:04:05"), "--dest-dir="+tempDir).Outputs()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(strings.Contains(warningErr, "--since-time only accepts times matching RFC3339")).To(o.BeTrue())

		g.By("3. Test inspect with wrong since-time format should failed.\n")
		_, warningErr, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("inspect", "ns", "openshift-multus", "--since-time="+now.Format("2006-01-02"), "--dest-dir="+tempDir).Outputs()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(strings.Contains(warningErr, "--since-time only accepts times matching RFC3339")).To(o.BeTrue())

		g.By("4. Test inspect with correct since format should succeed.\n")
		o.Expect(oc.AsAdmin().WithoutNamespace().Run("adm").Args("inspect", "ns", "openshift-multus", "--since=1m", "--dest-dir="+tempDir).Execute()).To(o.Succeed())

		g.By("5. Test inspect with wrong since format should failed.\n")
		_, warningErr, err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("inspect", "ns", "openshift-multus", "--since=1", "--dest-dir="+tempDir).Outputs()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(strings.Contains(warningErr, "time: missing unit")).To(o.BeTrue())
	})

	// author: knarra@redhat.com
	g.It("Verify version of oc binary is included into the must-gather directory when running oc adm must-gather command [apigroup:config.openshift.io]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Skip on hypershift (External topology)
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Hypershift clusters have different must-gather behavior")
		}

		g.By("Get oc client version")
		clientVersion, clientVersionErr := oc.Run("version").Args("-o", "json").Output()
		o.Expect(clientVersionErr).NotTo(o.HaveOccurred())
		versionInfo := &VersionInfo{}
		if err := json.Unmarshal([]byte(clientVersion), &versionInfo); err != nil {
			e2e.Failf("unable to decode version with error: %v", err)
		}
		e2e.Logf("Version output is %s", versionInfo.ClientInfo.GitVersion)

		g.By("Run the must-gather")
		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather-73054.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir="+tempDir, "--", "/usr/bin/gather_audit_logs").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check the must-gather and verify that oc binary version is included")
		headContent, err := exec.Command("bash", "-c", fmt.Sprintf("cat %s/must-gather.logs| head -n 5", tempDir)).Output()
		e2e.Logf("headContent is %s", headContent)
		if err != nil {
			e2e.Logf("Error is %s", err.Error())
		}
		e2e.Logf("ReleaseClientInfo is %s", versionInfo.ReleaseClientVersion)
		if versionInfo.ReleaseClientVersion != "" {
			o.Expect(headContent).To(o.ContainSubstring(versionInfo.ReleaseClientVersion))
		} else {
			o.Expect(headContent).To(o.ContainSubstring(versionInfo.ClientInfo.GitVersion))
		}
	})

	// author: knarra@redhat.com
	g.It("Verify logs generated are included in the must-gather directory when running the oc adm must-gather command timeout [apigroup:config.openshift.io][Timeout:30m]", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Skip on hypershift (External topology)
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Hypershift clusters have different must-gather behavior")
		}

		g.By("Run the must-gather")
		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather-73055.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir)

		tempDir1, err := ioutil.TempDir("", "test.oc-adm-must-gather-73055-1.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir1)

		tempDir2, err := ioutil.TempDir("", "test.oc-adm-must-gather-73055-2.")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tempDir2)

		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir="+tempDir, "--", "/usr/bin/gather_audit_logs").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check logs generated are included in the must-gather directory when must-gather is run")
		fileContent, err := exec.Command("bash", "-c", fmt.Sprintf("cat %s/must-gather.logs", tempDir)).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "must-gather.logs file should exist and be readable")
		fileContentStr := string(fileContent)
		// Verify the file contains expected must-gather output markers instead of exact string matching
		// (timestamps may have different precision between stdout and file)
		o.Expect(fileContentStr).To(o.ContainSubstring("Using must-gather plug-in image"), "must-gather.logs should contain initial output")
		o.Expect(fileContentStr).To(o.ContainSubstring("ClusterID"), "must-gather.logs should contain cluster information")
		o.Expect(len(fileContentStr)).To(o.BeNumerically(">", 100), "must-gather.logs should contain substantial output")

		// Check if gather.logs exists in the directory for default must-gather image
		checkGatherLogsForImage(oc, tempDir)

		// Check if gather.logs exists in the directory for CNV image
		// Note: registry.redhat.io may not be accessible in CI environments, so we skip if the image pull fails
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--image=registry.redhat.io/container-native-virtualization/cnv-must-gather-rhel9:v4.15.0", "--dest-dir="+tempDir1).Execute()
		if err != nil {
			e2e.Logf("Skipping CNV must-gather image test - image pull failed (likely not accessible in CI environment): %v", err)
		} else {
			checkGatherLogsForImage(oc, tempDir1)
		}

		// Check if gather.logs exists for both the images when passed to must-gather
		// Note: registry.redhat.io may not be accessible in CI environments, so we skip if the image pull fails
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--image-stream=openshift/must-gather", "--image=registry.redhat.io/container-native-virtualization/cnv-must-gather-rhel9:v4.15.0", "--dest-dir="+tempDir2).Execute()
		if err != nil {
			e2e.Logf("Skipping multi-image must-gather test with CNV - image pull failed (likely not accessible in CI environment): %v", err)
		} else {
			checkGatherLogsForImage(oc, tempDir2)
		}
	})
})

// GetPluginOutputDir returns the directory containing must-gather assets.
// Before [1], the assets were placed directly in tempDir.  Since [1],
// they have been placed in a subdirectory named after the must-gather
// image.
//
// [1]: https://github.com/openshift/oc/pull/84
func GetPluginOutputDir(tempDir string) string {
	files, err := os.ReadDir(tempDir)
	o.Expect(err).NotTo(o.HaveOccurred())
	dir := ""
	for _, file := range files {
		if file.IsDir() {
			if dir != "" {
				e2e.Logf("found multiple directories in %q, so assuming it is an old-style must-gather", tempDir)
				return tempDir
			}
			dir = path.Join(tempDir, file.Name())
		}
	}
	if dir == "" {
		e2e.Logf("found no directories in %q, so assuming it is an old-style must-gather", tempDir)
		return tempDir
	}
	e2e.Logf("found a single subdirectory %q, so assuming it is a new-style must-gather", dir)
	return dir
}

func IsAuditFile(fileName string) bool {
	if (strings.Contains(fileName, "-termination-") && strings.HasSuffix(fileName, ".log.gz")) ||
		strings.HasSuffix(fileName, "termination.log.gz") ||
		(strings.Contains(fileName, "-startup-") && strings.HasSuffix(fileName, ".log.gz")) ||
		strings.HasSuffix(fileName, "startup.log.gz") ||
		fileName == ".lock" ||
		fileName == "lock.log" {
		// these are expected, but have unstructured log format
		return false
	}

	return (strings.Contains(fileName, "-audit-") && strings.HasSuffix(fileName, ".log.gz")) || strings.HasSuffix(fileName, "audit.log.gz")
}

func getOauthAudit(mustgatherDir string) []string {
	var files []string
	filesUnderGather, err := ioutil.ReadDir(mustgatherDir)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to read the must-gather dir")
	dataDir := ""
	for _, fileD := range filesUnderGather {
		if fileD.IsDir() {
			dataDir = fileD.Name()
		}
	}
	e2e.Logf("The data dir is %v", dataDir)
	destDir := mustgatherDir + "/" + dataDir + "/audit_logs/oauth-server/"
	err = filepath.Walk(destDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			file_size := info.Size()
			// When the file_size is too little, the file maybe empty or too little records, so filter more than 1024
			if !info.IsDir() && file_size > 1024 {
				files = append(files, path)
			}
			return nil
		})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to read the destDir")
	return files
}

func getTimeFromNode(oc *exutil.CLI, nodeName string, ns string) time.Time {
	timeStr, _, dErr := oc.AsAdmin().WithoutNamespace().Run("debug").Args("node/"+nodeName, "-n", ns, "--", "chroot", "/host", "date", "+%Y-%m-%dT%H:%M:%SZ").Outputs()
	o.Expect(dErr).NotTo(o.HaveOccurred(), "Error getting date in node %s", nodeName)
	layout := "2006-01-02T15:04:05Z"
	returnTime, perr := time.Parse(layout, timeStr)
	o.Expect(perr).NotTo(o.HaveOccurred())
	return returnTime
}

func checkMustgatherLogTime(mustgatherDir string, nodeName string, timestamp string) {
	filesUnderGather, err := ioutil.ReadDir(mustgatherDir)
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to read the must-gather dir")
	dataDir := ""
	for _, fileD := range filesUnderGather {
		if fileD.IsDir() {
			dataDir = fileD.Name()
		}
	}
	e2e.Logf("The data dir is %v", dataDir)
	nodeLogsFile := mustgatherDir + "/" + dataDir + "/nodes/" + nodeName + "/" + nodeName + "_logs_kubelet.gz"
	e2e.Logf("The node log file is %v", nodeLogsFile)
	nodeLogsData, err := exec.Command("bash", "-c", fmt.Sprintf("zcat %v ", nodeLogsFile)).Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	if strings.Contains(string(nodeLogsData), timestamp) {
		e2e.Failf("Got unexpected time %v, must-gather wrong", timestamp)
	} else {
		e2e.Logf("Only able to successfully retreieve logs after timestamp %v", timestamp)
	}
}

func checkInspectLogTime(inspectDir string, podName string, timestamp string) {
	podLogsDir := inspectDir + "/namespaces/openshift-multus/pods/" + podName + "/kube-multus/kube-multus/logs"
	var fileList []string
	err := filepath.Walk(podLogsDir, func(path string, info os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return nil
	})
	if err != nil {
		e2e.Failf("Failed to check inspect directory")
	}
	for i := 1; i < len(fileList); i++ {
		podLogData, err := ioutil.ReadFile(fileList[i])
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(string(podLogData), timestamp) {
			e2e.Failf("Got unexpected time, inspect wrong")
		} else {
			e2e.Logf("Only able to successfully retreive the inspect logs after timestamp %v which is expected", timestamp)
		}
	}
}

func checkGatherLogsForImage(oc *exutil.CLI, filePath string) {
	imageDir, err := os.Open(filePath)
	if err != nil {
		e2e.Logf("Error opening directory: %v", err)
	}
	defer imageDir.Close()

	// Read the contents of the directory
	gatherlogInfos, err := imageDir.Readdir(-1)
	if err != nil {
		e2e.Logf("Error reading directory contents: %v", err)
	}

	// Check if gather.logs exist for each image
	for _, gatherlogInfo := range gatherlogInfos {
		if gatherlogInfo.IsDir() {
			filesList, err := exec.Command("bash", "-c", fmt.Sprintf("ls -l %v/%v", filePath, gatherlogInfo.Name())).Output()
			if err != nil {
				e2e.Failf("Error listing directory: %v", err)
			}
			o.Expect(strings.Contains(string(filesList), "gather.logs")).To(o.BeTrue())
		} else {
			e2e.Logf("Not a directory, continuing to the next")
		}
	}
}

// VersionInfo ...
type VersionInfo struct {
	ClientInfo           ClientVersion `json:"clientVersion"`
	ReleaseClientVersion string        `json:"releaseClientVersion"`
}

// ClientVersion ...
type ClientVersion struct {
	GitVersion string `json:"gitVersion"`
}
