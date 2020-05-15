package etcd

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

var _ = g.Describe("[Serial] API data in etcd", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("etcd-storage-path", exutil.KubeConfigPath())

	_ = g.It("should be stored at the correct location and version for all resources", func() {
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			e2e.Skipf("IBM ROKS clusters run etcd outside of the cluster. Etcd cannot be accessed directly from within the cluster")
		}

		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, "oc", "port-forward", "service/etcd", ":2379", "-n", "openshift-etcd", "--config", exutil.KubeConfigPath())

		defer func() {
			cancel()
			_ = cmd.Wait() // wait to clean up resources but ignore returned error since cancel kills the process
		}()

		stdOut, err := cmd.StdoutPipe()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(cmd.Start()).NotTo(o.HaveOccurred())

		scanner := bufio.NewScanner(stdOut)
		o.Expect(scanner.Scan()).To(o.BeTrue())
		o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
		output := scanner.Text()

		port := strings.TrimSuffix(strings.TrimPrefix(output, "Forwarding from 127.0.0.1:"), " -> 2379")
		_, err = strconv.Atoi(port)
		o.Expect(err).NotTo(o.HaveOccurred(), "port forward output not in expected format: %s", output)

		coreV1 := oc.AdminKubeClient().CoreV1()
		etcdConfigMap, err := coreV1.ConfigMaps("openshift-config").Get("etcd-ca-bundle", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		etcdSecret, err := coreV1.Secrets("openshift-config").Get("etcd-client", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		tlsConfig, err := restclient.TLSConfigFor(&restclient.Config{
			TLSClientConfig: restclient.TLSClientConfig{
				CertData: etcdSecret.Data[corev1.TLSCertKey],
				KeyData:  etcdSecret.Data[corev1.TLSPrivateKeyKey],
				CAData:   []byte(etcdConfigMap.Data["ca-bundle.crt"]),
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		etcdClient3, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{"https://127.0.0.1:" + port},
			DialTimeout: 30 * time.Second,
			TLS:         tlsConfig,
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		testEtcd3StoragePath(g.GinkgoT(), oc.AdminConfig(), etcdClient3.KV)
	})
})
