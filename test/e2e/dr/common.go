package dr

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/test/e2e/upgrade"
	excl "github.com/openshift/origin/test/extended/cluster"
	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"
)

const (
	sshOpts             = "-o StrictHostKeyChecking=no -o LogLevel=error -o ServerAliveInterval=30 -o ConnectionAttempts=100 -o ConnectTimeout=30"
	proxyTemplate       = "ssh -A %s -W %%h:%%p core@%s 2>/dev/null"
	scpToHostTemplate   = "scp %s -o ProxyCommand=\"%s\" %s core@%s:%s"
	scpFromHostTemplate = "scp %s -o ProxyCommand=\"%s\" core@%s:%s %s"
	sshTemplate         = "ssh %s -o ProxyCommand=\"%s\" core@%s \"%s\""
	bastionNamespace    = "ssh-bastion"

	operatorWait = 15 * time.Minute
)

func createPasswdEntry(homeDir string) {
	e2e.Logf("Adding a fake user entry")
	userName := os.Getenv("USER_NAME")
	if len(userName) == 0 {
		userName = "default"
	}
	// User IDs are fake in openshift, so os/user would return nil
	uid := strings.TrimSuffix(runCommandAndRetry("id -u"), "\n")
	passwdEntry := fmt.Sprintf("%s:x:%s:0:%s user:%s:/sbin/nologin\n", userName, uid, userName, homeDir)

	f, err := os.OpenFile("/etc/passwd", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer f.Close()
	_, err = f.WriteString(passwdEntry)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func copyKubeSSHKeyToUser(homeDir string) {
	e2e.Logf("Copying kube's ssh key to %s", homeDir)
	var (
		sshDirPath = filepath.Join(homeDir, ".ssh")
		destPath   = filepath.Join(sshDirPath, "id_rsa")
	)

	os.MkdirAll(sshDirPath, os.ModePerm)

	kubeSSHPath := os.Getenv("KUBE_SSH_KEY_PATH")
	o.Expect(kubeSSHPath).NotTo(o.HaveLen(0))

	source, err := os.Open(kubeSSHPath)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer source.Close()

	destination, err := os.Create(destPath)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer destination.Close()

	_, err = io.Copy(destination, source)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = os.Chmod(destPath, 0600)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func createSSHKeys() ([]string, string) {
	var (
		keyTypes = []string{"rsa", "ecdsa", "ed25519"}
		tmpFiles = make([]string, len(keyTypes))
	)

	homeDir := os.Getenv("HOME")

	// Ensure user entry is created in the container
	createPasswdEntry(homeDir)

	// Copy test ssh key to user dir
	copyKubeSSHKeyToUser(homeDir)

	// Create temporary ssh keys
	tmpDir, err := ioutil.TempDir("/tmp", "ssh-keys")
	o.Expect(err).NotTo(o.HaveOccurred())

	for index, keyType := range keyTypes {
		keyPath := filepath.Join(tmpDir, keyType)
		e2e.Logf("Generating %s key in %s", keyType, keyPath)
		out, err := exec.Command(
			"/usr/bin/ssh-keygen",
			"-q",          // silence
			"-t", keyType, // type
			"-f", keyPath, // output file
			"-C", "", // no comment
			"-N", "", // no passphrase
		).CombinedOutput()
		if err != nil {
			e2e.Logf("ssh-keygen output:\n%s", out)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		tmpFiles[index] = keyPath
	}
	return tmpFiles, tmpDir
}

func setupSSHBastion(oc *exutil.CLI) string {
	e2e.Logf("Setting up ssh bastion host")

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
	)

	ok, err := excl.ProjectExists(oc, bastionNamespace)
	if err == nil && !ok {
		err := oc.Run("new-project").Args(bastionNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("Creating ssh keys")
	_, err = oc.AdminKubeClient().CoreV1().Secrets(bastionNamespace).Get("ssh-host-keys", metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		tmpFiles, tmpDir := createSSHKeys()
		defer os.RemoveAll(tmpDir)

		secretKeyArgs := fmt.Sprintf(
			"ssh_host_rsa_key=%s,ssh_host_ecdsa_key=%s,ssh_host_ed25519_key=%s,sshd_config=%s",
			tmpFiles[0], tmpFiles[1], tmpFiles[2], filepath.Join(sshBastionBaseDir, "sshd_config"),
		)
		_, err = oc.Run("create").Args("-n", bastionNamespace, "secret", "generic", "ssh-host-keys", "--from-file", secretKeyArgs).Output()
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
		svc, err := oc.AdminKubeClient().CoreV1().Services(bastionNamespace).Get("ssh-bastion", metav1.GetOptions{})
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

	command := fmt.Sprintf(scpToHostTemplate, sshOpts, proxy, src, destHost, dest)
	runCommandAndRetry(command)
}

func scpFileFromHost(src string, srcHost string, proxy string, dest string) {
	e2e.Logf("Copying %s from '%s' to %s via %s", src, srcHost, dest, proxy)

	command := fmt.Sprintf(scpFromHostTemplate, sshOpts, proxy, srcHost, src, dest)
	runCommandAndRetry(command)
}

func runViaBastionSSH(host string, proxy string, remoteCommand string) string {
	e2e.Logf("Running '%s' on host %s via %s", remoteCommand, host, proxy)

	command := fmt.Sprintf(sshTemplate, sshOpts, proxy, host, remoteCommand)
	return runCommandAndRetry(command)
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
		etcdEnv := runViaBastionSSH(master, proxy, "cat /run/etcd/environment")
		var entry string
		for _, entry = range strings.Split(etcdEnv, "\n") {
			if strings.HasPrefix(entry, "ETCD_DNS_NAME=") {
				break
			}
		}
		etcdDNSName := strings.Split(entry, "=")[1]
		o.Expect(etcdDNSName).NotTo(o.BeEmpty())
		etcdConnectionString = fmt.Sprintf("%setcd-member-%s=https://%s:2380,", etcdConnectionString, master, etcdDNSName)
	}
	return etcdConnectionString[:len(etcdConnectionString)-1]
}

func removeSSHBastion(oc *exutil.CLI) {
	e2e.Logf("Removing ssh bastion")
	err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(bastionNamespace, &metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForMastersToUpdate(oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2e.Logf("Waiting for MachineConfig master to finish rolling out")
	err := wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
		return upgrade.IsPoolUpdated(mcps, "master")
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForOperatorsToSettle(coc dynamic.NamespaceableResourceInterface) {
	var lastErr error
	// gate on all clusteroperators being ready
	available := make(map[string]struct{})
	lastErr = nil
	var lastCOs []objx.Map
	wait.PollImmediate(30*time.Second, operatorWait, func() (bool, error) {
		obj, err := coc.List(metav1.ListOptions{})
		if err != nil {
			lastErr = err
			e2e.Logf("Unable to check for cluster operators: %v", err)
			return false, nil
		}
		cv := objx.Map(obj.UnstructuredContent())
		lastErr = nil
		items := objects(cv.Get("items"))
		lastCOs = items

		if len(items) == 0 {
			return false, nil
		}

		var unavailable []objx.Map
		var unavailableNames []string
		for _, co := range items {
			if condition(co, "Available").Get("status").String() != "True" {
				ns := co.Get("metadata.namespace").String()
				name := co.Get("metadata.name").String()
				unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
				unavailable = append(unavailable, co)
				break
			}
			if condition(co, "Progressing").Get("status").String() != "False" {
				ns := co.Get("metadata.namespace").String()
				name := co.Get("metadata.name").String()
				unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
				unavailable = append(unavailable, co)
				break
			}
			if condition(co, "Failing").Get("status").String() != "False" {
				ns := co.Get("metadata.namespace").String()
				name := co.Get("metadata.name").String()
				unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
				unavailable = append(unavailable, co)
				break
			}
		}
		if len(unavailable) > 0 {
			e2e.Logf("Operators still doing work: %s", strings.Join(unavailableNames, ", "))
			return false, nil
		}
		return true, nil
	})

	o.Expect(lastErr).NotTo(o.HaveOccurred())
	var unavailable []string
	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
	fmt.Fprintf(w, "NAMESPACE\tNAME\tPROGRESSING\tAVAILABLE\tVERSION\tMESSAGE\n")
	for _, co := range lastCOs {
		ns := co.Get("metadata.namespace").String()
		name := co.Get("metadata.name").String()
		if condition(co, "Available").Get("status").String() != "True" {
			unavailable = append(unavailable, fmt.Sprintf("%s/%s", ns, name))
		} else {
			available[fmt.Sprintf("%s/%s", ns, name)] = struct{}{}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			ns,
			name,
			condition(co, "Progressing").Get("status").String(),
			condition(co, "Available").Get("status").String(),
			co.Get("status.version").String(),
			condition(co, "Failing").Get("message").String(),
		)
	}
	w.Flush()
	e2e.Logf("ClusterOperators:\n%s", buf.String())
	if len(unavailable) > 0 {
		e2e.Failf("Some cluster operators never became available %s", strings.Join(unavailable, ", "))
	}
	// Check at least one core operator is available
	if len(available) == 0 {
		e2e.Failf("There must be at least one cluster operator")
	}
}

func restartSDNPods(oc *exutil.CLI) {
	e2e.Logf("Restarting SDN")

	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-sdn").List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		e2e.Logf("Deleting pod %s", pod.Name)
		err := oc.AdminKubeClient().CoreV1().Pods("openshift-sdn").Delete(pod.Name, &metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		sdnDaemonset, err := oc.AdminKubeClient().ExtensionsV1beta1().DaemonSets("openshift-sdn").Get("sdn", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return sdnDaemonset.Status.NumberReady == sdnDaemonset.Status.NumberAvailable, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func restartOpenshiftAPIPods(oc *exutil.CLI) {
	e2e.Logf("Restarting Openshift API server")

	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-apiserver").List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		e2e.Logf("Deleting pod %s", pod.Name)
		err := oc.AdminKubeClient().CoreV1().Pods("openshift-apiserver").Delete(pod.Name, &metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		apiServerDS, err := oc.AdminKubeClient().ExtensionsV1beta1().DaemonSets("openshift-apiserver").Get("apiserver", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return apiServerDS.Status.NumberReady == apiServerDS.Status.NumberAvailable, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func restartMCDPods(oc *exutil.CLI) {
	e2e.Logf("Restarting MCD pods")

	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-machine-config-operator").List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		e2e.Logf("Deleting pod %s", pod.Name)
		err := oc.AdminKubeClient().CoreV1().Pods("openshift-machine-config-operator").Delete(pod.Name, &metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		mcDS, err := oc.AdminKubeClient().ExtensionsV1beta1().DaemonSets("openshift-machine-config-operator").Get("machine-config-daemon", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return mcDS.Status.NumberReady == mcDS.Status.NumberAvailable, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func objects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, objx.Map(msi))
			}
		}
	}
	return values
}

func condition(cv objx.Map, condition string) objx.Map {
	for _, obj := range objects(cv.Get("status.conditions")) {
		if obj.Get("type").String() == condition {
			return obj
		}
	}
	return objx.Map(nil)
}
