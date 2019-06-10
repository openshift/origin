package dr

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

const (
	sshOpts               = "-o StrictHostKeyChecking=no -o LogLevel=error -o ServerAliveInterval=30 -o ConnectionAttempts=100 -o ConnectTimeout=30"
	proxyTemplate         = "ssh -A %s -W %%h:%%p core@%s 2>/dev/null"
	scpTemplate           = "scp %s -o ProxyCommand=\"%s\" %s core@%s:%s"
	sshTemplate           = "ssh %s -o ProxyCommand=\"%s\" core@%s \"%s\""
	rollBackMachineConfig = "99-rollback-test"
)

var _ = g.Describe("[Feature:DisasterRecovery][Disruptive]", func() {
	f := e2e.NewDefaultFramework("disaster-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("disaster-recovery")

	g.It("Cluster should restore itself from etcd snapshot", func() {
		config, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		dynamicClient := dynamic.NewForConfigOrDie(config)
		mcps := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigpools",
		})
		mc := dynamicClient.Resource(schema.GroupVersionResource{
			Group:    "machineconfiguration.openshift.io",
			Version:  "v1",
			Resource: "machineconfigs",
		})

		bastionHost := setupSSHBastion(oc)
		proxy := fmt.Sprintf(proxyTemplate, sshOpts, bastionHost)
		defer removeSSHBastion(oc)

		setMachineConfig("rollback-A.yaml", oc, mcps)

		masters := getAllMasters(oc)
		e2e.Logf("masters: %v", masters)
		o.Expect(masters).NotTo(o.BeEmpty())
		firstMaster := masters[0]
		e2e.Logf("first master: %v", firstMaster)

		e2e.Logf("Make etcd backup on first master")
		runViaBastionSSH(firstMaster, proxy,
			"sudo -i /bin/bash -x /usr/local/bin/etcd-snapshot-backup.sh /root/assets/backup/snapshot.db")
		runViaBastionSSH(firstMaster, proxy,
			"sudo -i install -o core -g core /root/assets/backup/snapshot.db /tmp/snapshot.db")
		setMachineConfig("rollback-B.yaml", oc, mcps)

		scpFileToHost(os.Getenv("KUBE_SSH_KEY_PATH"), proxy, "/home/core/.ssh/id_rsa", firstMaster)
		runViaBastionSSH(firstMaster, proxy, "chmod 0600 /home/core/.ssh/id_rsa")
		for _, master := range masters {
			if master == firstMaster {
				continue
			}
			runViaBastionSSH(firstMaster, proxy,
				fmt.Sprintf("scp -o StrictHostKeyChecking=no /tmp/snapshot.db core@%s:/tmp/snapshot.db", master))
		}

		etcdConnectionString := constructEtcdConnectionString(masters, proxy)
		e2e.Logf("etcd connstring: '%s'", etcdConnectionString)
		for _, master := range masters {
			runViaBastionSSH(master, proxy,
				fmt.Sprintf("sudo -i /bin/bash -x /usr/local/bin/etcd-snapshot-restore.sh /tmp/snapshot.db %s", etcdConnectionString))
		}

		waitForAPIServer(oc)
		waitForMastersToUpdate(oc, mcps)

		rollBackInMC := getRollbackContentsInMachineConfig(oc, mc, rollBackMachineConfig)
		o.Expect(rollBackInMC).To(o.BeEquivalentTo("data:,A"))

		for _, master := range masters {
			rollBackFile := fetchRollbackFileContents(master, proxy)
			o.Expect(rollBackFile).To(o.BeEquivalentTo("A"))
		}
	})
})

func setupSSHBastion(oc *exutil.CLI) string {
	e2e.Logf("Setting up ssh bastion host")
	const (
		ns = "ssh-bastion"
	)

	var (
		bastionHost       = ""
		sshBastionBaseDir = exutil.FixturePath("testdata", "disaster-recovery", "ssh-bastion")
		files             = []string{
			"service.yaml",
			"serviceaccount.yaml",
			"role.yaml",
			"rolebinding.yaml",
			"clusterrole.yaml",
			"clusterrolebinding.yaml",
			"deployment.yaml",
		}
		keyTypes = []string{"rsa", "ecdsa", "ed25519"}
		tmpFiles = make([]string, len(keyTypes))
	)

	_, err := oc.AdminProjectClient().Project().Projects().Get(ns, metav1.GetOptions{})
	if err != nil {
		err = oc.Run("new-project").Args(ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	e2e.Logf("Creating ssh keys")
	_, err = oc.AdminKubeClient().CoreV1().Secrets(ns).Get("ssh-host-keys", metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		tmpDir, err := ioutil.TempDir("/tmp", "ssh-keys")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpDir)

		for index, keyType := range keyTypes {
			keyPath := filepath.Join(tmpDir, keyType)
			e2e.Logf("Generating %s key in %s", keyType, keyPath)
			out, err := exec.Command(
				"ssh-keygen",
				"-q",          // silence
				"-t", keyType, // type
				"-f", keyPath, // output file
				"-C", "", // no comment
				"-N", "", // no passphrase
			).Output()
			if err != nil {
				e2e.Logf("ssh-keygen output:\n%s", out)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			tmpFiles[index] = keyPath
		}

		secretKeyArgs := fmt.Sprintf(
			"ssh_host_rsa_key=%s,ssh_host_ecdsa_key=%s,ssh_host_ed25519_key=%s,sshd_config=%s",
			tmpFiles[0], tmpFiles[1], tmpFiles[2], filepath.Join(sshBastionBaseDir, "sshd_config"),
		)
		_, err = oc.Run("create").Args("-n", ns, "secret", "generic", "ssh-host-keys", "--from-file", secretKeyArgs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	} else {
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	e2e.Logf("Deploying ssh bastion")
	for _, file := range files {
		testDataPath := filepath.Join(sshBastionBaseDir, file)
		err := oc.Run("apply").Args("-f", testDataPath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	e2e.Logf("Waiting for load balancer to be created")
	err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		svc, err := oc.AdminKubeClient().CoreV1().Services(ns).Get("ssh-bastion", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			return true, fmt.Errorf("Incorrect service type: %v", svc.Spec.Type)
		}
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return false, nil
		}
		bastionHost = svc.Status.LoadBalancer.Ingress[0].Hostname
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Bastion host: %s", bastionHost)

	e2e.Logf("Waiting for host to be resolvable")
	err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		_, err = exec.Command("nslookup", bastionHost).Output()
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	return bastionHost
}

func runCommandAndRetry(command string) string {
	const (
		maxRetries = 10
		pause      = 10
	)
	var (
		retryCount = 0
		out        []byte
		err        error
	)
	e2e.Logf("command '%s'", command)
	for retryCount = 0; retryCount <= maxRetries; retryCount++ {
		out, err = exec.Command("bash", "-c", command).CombinedOutput()
		e2e.Logf("output:\n%s", out)
		if err == nil {
			break
		}
		e2e.Logf("%v", err)
		time.Sleep(time.Second * pause)
	}
	o.Expect(retryCount).NotTo(o.Equal(maxRetries + 1))
	return string(out)
}

func scpFileToHost(src string, proxy string, dest string, destHost string) {
	e2e.Logf("Copying %s to %s at host '%s' via %s", src, dest, destHost, proxy)

	command := fmt.Sprintf(scpTemplate, sshOpts, proxy, src, destHost, dest)
	runCommandAndRetry(command)
}

func runViaBastionSSH(host string, proxy string, remoteCommand string) string {
	e2e.Logf("Running '%s' on host %s via %s", remoteCommand, host, proxy)

	command := fmt.Sprintf(sshTemplate, sshOpts, proxy, host, remoteCommand)
	return runCommandAndRetry(command)
}

func setMachineConfig(rollbackFileName string, oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2e.Logf("Update MachineConfig using %s file on masters", rollbackFileName)
	machineConfigTemplate := exutil.FixturePath("testdata", "disaster-recovery", rollbackFileName)
	err := oc.Run("apply").Args("-f", machineConfigTemplate).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	waitForMastersToUpdate(oc, mcps)
}

func getRollbackContentsInMachineConfig(oc *exutil.CLI, mcs dynamic.NamespaceableResourceInterface, mcName string) string {
	e2e.Logf("Reading contents of rollback MachineConfig")
	pool, err := mcs.Get(mcName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	files, found, err := unstructured.NestedSlice(pool.Object, "spec", "config", "storage", "files")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue())
	o.Expect(files).NotTo(o.BeEmpty())

	file := files[0].(map[string]interface{})
	actual, found, err := unstructured.NestedString(file, "contents", "source")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue())

	return actual
}

func waitForMastersToUpdate(oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2e.Logf("Waiting for MachineConfig master to finish rolling out")
	err := wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
		return upgrade.IsPoolUpdated(mcps, "master")
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func getAllMasters(oc *exutil.CLI) []string {
	nodeNames := sets.NewString()

	e2e.Logf("Fetching a list of masters")

	masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
	})
	for i := range masterNodes.Items {
		node := &masterNodes.Items[i]
		nodeNames.Insert(node.ObjectMeta.Name)
	}

	o.Expect(err).NotTo(o.HaveOccurred())

	return nodeNames.List()
}

func constructEtcdConnectionString(masters []string, proxy string) string {
	//TODO vrutkovs: replace this nonsense with `etcdctl member list -w json ...`
	etcdConnectionString := ""
	e2e.Logf("Construct etcd connection string")
	for _, master := range masters {
		hostname := runViaBastionSSH(master, proxy, "hostname -f")
		o.Expect(hostname).NotTo(o.BeEmpty())
		hostname = strings.TrimSpace(hostname)

		etcdEnv := runViaBastionSSH(master, proxy, "cat /run/etcd/environment")
		var entry string
		for _, entry = range strings.Split(etcdEnv, "\n") {
			if strings.HasPrefix(entry, "ETCD_DNS_NAME=") {
				break
			}
		}
		etcdDNSName := strings.Split(entry, "=")[1]
		o.Expect(etcdDNSName).NotTo(o.BeEmpty())
		etcdConnectionString = fmt.Sprintf("%setcd-member-%s=https://%s:2380,", etcdConnectionString, hostname, etcdDNSName)
	}
	return etcdConnectionString[:len(etcdConnectionString)-1]
}

func waitForAPIServer(oc *exutil.CLI) {
	e2e.Logf("Waiting for API server to restore")
	err := wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		_, err = oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func fetchRollbackFileContents(master string, proxy string) string {
	e2e.Logf("Fetching /etc/rollback-test file contents from %s", master)
	return runViaBastionSSH(master, proxy, "cat /etc/rollback-test")
}

func removeSSHBastion(oc *exutil.CLI) {
	e2e.Logf("Removing ssh bastion")
}
