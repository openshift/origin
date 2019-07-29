package dr

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

const (
	machineAnnotationName   = "machine.openshift.io/machine"
	localEtcdSignerYaml     = "/tmp/kube-etcd-cert-signer.yaml"
	expectedNumberOfMasters = 3
)

var _ = g.Describe("[Feature:DisasterRecovery][Disruptive]", func() {
	f := e2e.NewDefaultFramework("disaster-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("disaster-recovery")

	g.It("[dr-quorum-restore] Cluster should restore itself after quorum loss", func() {
		config, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dynamicClient := dynamic.NewForConfigOrDie(config)
		ms := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machine.openshift.io",
			Version:  "v1beta1",
			Resource: "machines",
		}).Namespace("openshift-machine-api")
		mcps := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigpools",
		})
		coc := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "config.openshift.io",
			Version:  "v1",
			Resource: "clusteroperators",
		})

		bastionHost := setupSSHBastion(oc)
		proxy := fmt.Sprintf(proxyTemplate, sshOpts, bastionHost)
		defer removeSSHBastion(oc)

		scaleEtcdQuorum(oc, 0)

		e2e.Logf("Finding two masters to remove")
		mapiPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-machine-api").List(metav1.ListOptions{
			LabelSelector: "k8s-app=controller",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(mapiPods.Items).NotTo(o.BeEmpty())

		survivingNodeName := mapiPods.Items[0].Spec.NodeName
		mastersNodes := getAllMasters(oc)
		o.Expect(mastersNodes).NotTo(o.BeEmpty())

		survivingMachineName := getMachineNameByNodeName(oc, survivingNodeName)
		survivingMachine, err := ms.Get(survivingMachineName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Set etcd connection string before destroying masters, as ssh bastion may be unavailable
		etcdConnectionString := constructEtcdConnectionString([]string{survivingNodeName}, proxy)

		e2e.Logf("Destroy 2 masters")
		masterMachines := make([]string, len(mastersNodes))
		for i, node := range mastersNodes {
			masterMachine := getMachineNameByNodeName(oc, node)
			masterMachines[i] = masterMachine

			if node == survivingNodeName {
				continue
			}

			e2e.Logf("Destroying %s", masterMachine)
			err = ms.Delete(masterMachine, &metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		e2e.Logf("masterMachines: %v", masterMachines)

		e2e.Logf("Confirm meltdown")
		time.Sleep(30 * time.Second)
		err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
			_, err = oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{})
			return err != nil, nil
		})

		e2e.Logf("Restore single node etcd")
		runViaBastionSSH(survivingNodeName, proxy,
			fmt.Sprintf("sudo -i /bin/bash -x /usr/local/bin/etcd-snapshot-restore.sh /root/assets/backup/etcd/member/snap/db %s", etcdConnectionString))

		e2e.Logf("Wait for API server to come up")
		time.Sleep(30 * time.Second)
		err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
			nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{})
			if err != nil || nodes.Items == nil {
				e2e.Logf("return false - err %v nodes.Items %v", err, nodes.Items)
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Create new masters")
		for _, master := range masterMachines {
			if master == survivingMachineName {
				continue
			}
			e2e.Logf("Creating master %s", master)
			newMaster := survivingMachine.DeepCopy()
			newMaster.SetName(master)
			newMaster.SetResourceVersion("")
			newMaster.SetSelfLink("")
			newMaster.SetUID("")
			newMaster.SetCreationTimestamp(metav1.NewTime(time.Time{}))
			_, err := ms.Create(newMaster, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		e2e.Logf("Waiting for machines to be created")
		err = wait.Poll(30*time.Second, 10*time.Minute, func() (done bool, err error) {
			mastersList, err := ms.List(metav1.ListOptions{
				LabelSelector: "machine.openshift.io/cluster-api-machine-role=master",
			})
			if err != nil || mastersList.Items == nil {
				e2e.Logf("return false - err %v mastersList.Items %v", err, mastersList.Items)
				return false, err
			}
			return len(mastersList.Items) == expectedNumberOfMasters, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Wait for masters to join as nodes")
		err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("Recovered from panic", r)
				}
			}()
			nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master=",
			})
			if err != nil || nodes.Items == nil {
				e2e.Logf("return false - err %v nodes.Items %v", err, nodes.Items)
				return false, err
			}
			return len(nodes.Items) == expectedNumberOfMasters, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Update DNS records")
		var survivingMasterIP string
		runCommandAndRetry("easy_install --user pip && ~/.local/bin/pip install --user boto3")

		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get("cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		internalAPI, err := url.Parse(infra.Status.APIServerURL)
		o.Expect(err).NotTo(o.HaveOccurred())
		internalAPI.Host = strings.Replace(internalAPI.Host, "api.", "", 1)

		domain, _, err := net.SplitHostPort(internalAPI.Host)
		o.Expect(err).ToNot(o.HaveOccurred())
		e2e.Logf("domain: %s", domain)
		masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		for i := range masterNodes.Items {
			node := &masterNodes.Items[i]
			etcdName := fmt.Sprintf("etcd-%d.%s", i, domain)
			masterIP := ""
			for _, address := range node.Status.Addresses {
				if address.Type == "InternalIP" {
					masterIP = address.Address
					break
				}
			}
			if node.GetName() == survivingNodeName {
				survivingMasterIP = masterIP
			}
			updateDNS(domain, etcdName, masterIP)
		}

		imagePullSecretPath := getPullSecret(oc)
		defer os.Remove(imagePullSecretPath)
		runPodSigner(oc, survivingNodeName, imagePullSecretPath, proxy)

		e2e.Logf("Restore etcd on remaining masters")
		setupEtcdEnvImage := getImagePullSpecFromRelease(oc, imagePullSecretPath, "setup-etcd-environment")
		kubeClientAgent := getImagePullSpecFromRelease(oc, imagePullSecretPath, "kube-client-agent")
		for i := range masterNodes.Items {
			node := &masterNodes.Items[i]
			masterDNS := ""
			for _, address := range node.Status.Addresses {
				if address.Type == "InternalDNS" {
					masterDNS = address.Address
					break
				}
			}
			if masterDNS == survivingNodeName {
				e2e.Logf("Skipping node as its the surviving master")
				continue
			}
			runViaBastionSSH(masterDNS, proxy,
				fmt.Sprintf("sudo -i env SETUP_ETCD_ENVIRONMENT=%s KUBE_CLIENT_AGENT=%s /bin/bash -x /usr/local/bin/etcd-member-recover.sh %s \"etcd-member-%s\"",
					setupEtcdEnvImage, kubeClientAgent, survivingMasterIP, node.GetName()))
		}

		e2e.Logf("Wait for etcd pods to become available")
		_, err = exutil.WaitForPods(
			oc.AdminKubeClient().CoreV1().Pods("openshift-etcd"),
			exutil.ParseLabelsOrDie("k8s-app=etcd"),
			exutil.CheckPodIsReady,
			expectedNumberOfMasters,
			10*time.Minute,
		)
		o.Expect(err).NotTo(o.HaveOccurred())

		scaleEtcdQuorum(oc, expectedNumberOfMasters)

		e2e.Logf("Remove etcd signer")
		err = oc.AdminKubeClient().CoreV1().Pods("openshift-config").Delete("etcd-signer", &metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=1707006#
		// SDN won't switch to Degraded mode when service is down after disaster recovery
		restartSDNPods(oc)
		waitForMastersToUpdate(oc, mcps)
		waitForOperatorsToSettle(coc)
	})
})

func scaleEtcdQuorum(oc *exutil.CLI, replicas int) {
	e2e.Logf("Scale etcd-quorum-guard to %d replicas", replicas)
	etcdQGScale, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-machine-config-operator").GetScale("etcd-quorum-guard", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	etcdQGScale.Spec.Replicas = int32(replicas)
	_, err = oc.AdminKubeClient().AppsV1().Deployments("openshift-machine-config-operator").UpdateScale("etcd-quorum-guard", etcdQGScale)
	o.Expect(err).NotTo(o.HaveOccurred())

	etcdQGScale, err = oc.AdminKubeClient().AppsV1().Deployments("openshift-machine-config-operator").GetScale("etcd-quorum-guard", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(etcdQGScale.Spec.Replicas).To(o.Equal(int32(replicas)))
}

func runPodSigner(oc *exutil.CLI, survivingNodeName, imagePullSecretPath, proxy string) {
	e2e.Logf("Run etcd signer pod")
	nodeHostname := strings.Split(survivingNodeName, ".")[0]

	kubeEtcdSignerServerImage := getImagePullSpecFromRelease(oc, imagePullSecretPath, "kube-etcd-signer-server")
	runViaBastionSSH(survivingNodeName, proxy,
		fmt.Sprintf("sudo -i env KUBE_ETCD_SIGNER_SERVER=%s /bin/bash -x /usr/local/bin/tokenize-signer.sh %s && sudo -i install -o core -g core /root/assets/manifests/kube-etcd-cert-signer.yaml /tmp/kube-etcd-cert-signer.yaml",
			kubeEtcdSignerServerImage, nodeHostname))
	scpFileFromHost("/tmp/kube-etcd-cert-signer.yaml", survivingNodeName, proxy, localEtcdSignerYaml)
	err := oc.Run("apply").Args("-f", localEtcdSignerYaml).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("Wait for etcd signer pod to become Ready")
	_, err = exutil.WaitForPods(
		oc.AdminKubeClient().CoreV1().Pods("openshift-config"),
		exutil.ParseLabelsOrDie("k8s-app=etcd"),
		exutil.CheckPodIsReady,
		1,
		10*time.Minute,
	)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getPullSecret(oc *exutil.CLI) string {
	e2e.Logf("Saving image pull secret")
	//TODO: copy of test/extended/operators/images.go, move this to a common func
	imagePullSecret, err := oc.KubeFramework().ClientSet.CoreV1().Secrets("openshift-config").Get("pull-secret", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if err != nil {
		e2e.Failf("unable to get pull secret for cluster: %v", err)
	}

	// cache file to local temp location
	imagePullFile, err := ioutil.TempFile("", "image-pull-secret")
	if err != nil {
		e2e.Failf("unable to create a temporary file: %v", err)
	}

	// write the content
	imagePullSecretBytes := imagePullSecret.Data[".dockerconfigjson"]
	if _, err := imagePullFile.Write(imagePullSecretBytes); err != nil {
		e2e.Failf("unable to write pull secret to temp file: %v", err)
	}
	if err := imagePullFile.Close(); err != nil {
		e2e.Failf("unable to close file: %v", err)
	}
	e2e.Logf("Image pull secret: %s", imagePullFile.Name())
	return imagePullFile.Name()
}

func getImagePullSpecFromRelease(oc *exutil.CLI, imagePullSecretPath, imageName string) string {
	image, err := oc.Run("adm", "release", "info").Args("--image-for", imageName, "--registry-config", imagePullSecretPath).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return image
}

func updateDNS(domain string, etcdName, masterIP string) {
	//TODO vrutkovs: make a golang native version
	scriptPath := exutil.FixturePath("testdata", "disaster-recovery", "update_route_53.py")
	runCommandAndRetry(fmt.Sprintf(
		"python %s %s %s %s", scriptPath, domain, etcdName, masterIP))
}

func getMachineNameByNodeName(oc *exutil.CLI, name string) string {
	masterNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	annotations := masterNode.GetAnnotations()
	o.Expect(annotations).To(o.HaveKey(machineAnnotationName))
	return strings.Split(annotations[machineAnnotationName], "/")[1]
}
