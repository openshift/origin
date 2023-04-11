package dr

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/openshift/origin/test/extended/util/image"
	xssh "golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"

	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2essh "k8s.io/kubernetes/test/e2e/framework/ssh"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/stretchr/objx"
)

const (
	operatorWait      = 15 * time.Minute
	defaultSSHTimeout = 5 * time.Minute
)

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

func masterNodes(oc *exutil.CLI) []*corev1.Node {
	masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	var nodes []*corev1.Node
	for i := range masterNodes.Items {
		node := &masterNodes.Items[i]
		nodes = append(nodes, node)
	}
	return nodes
}

func clusterNodes(oc *exutil.CLI) (masters, workers []*corev1.Node) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			masters = append(masters, node)
		} else {
			workers = append(workers, node)
		}
	}
	return
}

func waitForMastersToUpdate(oc *exutil.CLI, mcps dynamic.NamespaceableResourceInterface) {
	e2e.Logf("Waiting for MachineConfig master to finish rolling out")
	err := wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
		done, _ = upgrade.IsPoolUpdated(mcps, "master")
		return done, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForOperatorsToSettle() {
	g.By("Waiting for operators to settle before performing post-disruption testing")
	config, err := framework.LoadConfig()
	o.Expect(err).NotTo(o.HaveOccurred())
	dynamicClient := dynamic.NewForConfigOrDie(config)
	coc := dynamicClient.Resource(schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "clusteroperators",
	})

	var lastErr error
	// gate on all clusteroperators being ready
	available := make(map[string]struct{})
	lastErr = nil
	var lastCOs []objx.Map
	wait.PollImmediate(30*time.Second, operatorWait, func() (bool, error) {
		obj, err := coc.List(context.Background(), metav1.ListOptions{})
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
			if condition(co, "Degraded").Get("status").String() != "False" {
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
			condition(co, "Degraded").Get("message").String(),
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

	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-sdn").List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods.Items {
		e2e.Logf("Deleting pod %s", pod.Name)
		err := oc.AdminKubeClient().CoreV1().Pods("openshift-sdn").Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	err = wait.Poll(10*time.Second, 5*time.Minute, func() (done bool, err error) {
		sdnDaemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets("openshift-sdn").Get(context.Background(), "sdn", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return sdnDaemonset.Status.NumberReady == sdnDaemonset.Status.NumberAvailable, nil
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

func nodeConditionStatus(conditions []corev1.NodeCondition, conditionType corev1.NodeConditionType) corev1.ConditionStatus {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status
		}
	}
	return corev1.ConditionUnknown
}

func countReady(items []corev1.Node) int {
	ready := 0
	for _, item := range items {
		if nodeConditionStatus(item.Status.Conditions, corev1.NodeReady) == corev1.ConditionTrue {
			ready++
		}
	}
	return ready
}

func fetchFileContents(node *corev1.Node, path string) string {
	e2e.Logf("Fetching %s file contents from %s", path, node.Name)
	out := execOnNodeWithOutputOrFail(node, fmt.Sprintf("cat %q", path))
	return out.Stdout
}

// execOnNodeWithOutputOrFail executes a command via ssh against a
// node in a poll loop to ensure reliable execution in a disrupted
// environment. The calling test will be failed if the command cannot
// be executed successfully before the provided timeout.
func execOnNodeWithOutputOrFail(node *corev1.Node, cmd string) *e2essh.Result {
	var out *e2essh.Result
	var err error
	waitErr := wait.PollImmediate(5*time.Second, defaultSSHTimeout, func() (bool, error) {
		out, err = e2essh.IssueSSHCommandWithResult(cmd, e2e.TestContext.Provider, node)
		// IssueSSHCommandWithResult logs output
		if err != nil {
			e2e.Logf("Failed to exec cmd [%s] on node %s: %v", cmd, node.Name, err)
		}
		return err == nil, nil
	})
	o.Expect(waitErr).NotTo(o.HaveOccurred())
	return out
}

// execOnNodeOrFail executes a command via ssh against a node in a
// poll loop until success or timeout. The output is ignored. The
// calling test will be failed if the command cannot be executed
// successfully before the timeout.
func execOnNodeOrFail(node *corev1.Node, cmd string) {
	_ = execOnNodeWithOutputOrFail(node, cmd)
}

// sudoExecOnNodeOrFail executes a command under sudo with execOnNodeOrFail.
func sudoExecOnNodeOrFail(node *corev1.Node, cmd string) {
	sudoCmd := fmt.Sprintf(`sudo -i /bin/bash -cx "%s"`, cmd)
	execOnNodeOrFail(node, sudoCmd)
}

// checkSSH repeatedly attempts to establish an ssh connection to a
// node and fails the calling test if unable to establish the
// connection before the default timeout.
func checkSSH(node *corev1.Node) {
	_ = execOnNodeWithOutputOrFail(node, "true")
}

func ssh(cmd string, node *corev1.Node) (*e2essh.Result, error) {
	return e2essh.IssueSSHCommandWithResult(cmd, e2e.TestContext.Provider, node)
}

// InstallSSHKeyOnControlPlaneNodes will create a new private/public ssh keypair,
// create a new secret for both in the openshift-etcd-operator namespace. Then it
// will append the public key on the host core user authorized_keys file with a daemon set.
func InstallSSHKeyOnControlPlaneNodes(oc *exutil.CLI) error {
	const name = "dr-ssh"
	const namespace = "openshift-etcd"

	err := createPrivatePublicSSHKeySecret(oc, name, namespace)
	if err != nil {
		return err
	}

	err = createKeyInstallerDaemon(oc, name, namespace)
	if err != nil {
		return err
	}

	err = ensureControlPlaneSSHAccess(oc, name, namespace)
	if err != nil {
		return err
	}

	return nil
}

// ensureControlPlaneSSHAccess will test that the private key generated and installed with installSSHKeyOnControlPlaneNodes
// is working on all control plane nodes. This effectively polls until the pod executing ssh succeeds reaching all nodes.
func ensureControlPlaneSSHAccess(oc *exutil.CLI, name string, namespace string) error {
	const sshPath = "/tmp/ssh"
	const containerName = "ssh-key-tester"

	var cpNodeInternalIps []string
	for _, node := range masterNodes(oc) {
		framework.Logf("CP Node meta: %s, status: %s", node.ObjectMeta.String(), node.Status.String())
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				cpNodeInternalIps = append(cpNodeInternalIps, address.Address)
			}
		}
	}

	framework.Logf("found internal IPs: %v", cpNodeInternalIps)

	testScript := fmt.Sprintf(`
		#!/bin/bash
		CORE_SSH_BASE_DIR=$HOME/.ssh
		SSH_MOUNT_DIR=%s
		P_KEY=$SSH_MOUNT_DIR/privKey
		# we can't change the permissions on the secret mount, thus we copy it to HOME
		mkdir -p $CORE_SSH_BASE_DIR && chmod 700 $CORE_SSH_BASE_DIR
		cp $P_KEY $CORE_SSH_BASE_DIR/id_rsa
		P_KEY=$CORE_SSH_BASE_DIR/id_rsa
		chmod 600 $P_KEY

		ls -l $P_KEY
		
		NODE_IPS=( %s )
		for i in "${NODE_IPS[@]}"; do
		  echo "testing SSH to [$i]"
		  until ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${i} exit
		  do
			echo "Waiting for ssh connection on ${i}"
			sleep 5
		  done 
		  echo ""
		  echo "SSH to [$i] was successful!"
		done
			`, sshPath, strings.Join(cpNodeInternalIps, " "))

	podSpec := applycorev1.PodSpec().WithHostNetwork(true).WithRestartPolicy(corev1.RestartPolicyOnFailure)
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName(containerName).
			WithSecurityContext(applycorev1.SecurityContext().WithPrivileged(true)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(
				applycorev1.VolumeMount().WithName("keys").WithMountPath(sshPath),
			).
			WithCommand("/bin/bash", "-c", testScript),
	}
	podSpec.NodeSelector = map[string]string{"node-role.kubernetes.io/master": ""}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{
		*applycorev1.Toleration().WithKey("node-role.kubernetes.io/master").WithOperator(corev1.TolerationOpExists).WithEffect(corev1.TaintEffectNoSchedule),
	}

	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("keys").WithSecret(applycorev1.SecretVolumeSource().WithSecretName(name)),
	}

	pod := applycorev1.Pod(name, namespace).WithSpec(podSpec)

	// this is solely to ensure idempotency, especially in case we run it multiple times locally
	err := wait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err == nil || apierrors.IsNotFound(err) {
			return true, nil
		}
		framework.Logf("ssh pre-test pod deletion result [%v]", err)
		return false, err
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = oc.AdminKubeClient().CoreV1().Pods(namespace).Apply(ctx, pod, metav1.ApplyOptions{FieldManager: name, Force: true})
	if err != nil {
		return err
	}

	framework.Logf("Waiting for ssh test pod to complete...")
	err = wait.Poll(10*time.Second, 15*time.Minute, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		getResult, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Error while waiting for ssh test pod to complete: %v", err)
			return false, nil
		}
		framework.Logf("ssh test pod has status [%v]", getResult.Status.Phase)
		return getResult.Status.Phase == corev1.PodSucceeded, nil
	})

	return err
}

// createKeyInstallerDaemon will spawn a CP-only daemonset to add the publicKey created in
// createPrivatePublicSSHKeySecret to the core ssh server on the host machine.
func createKeyInstallerDaemon(oc *exutil.CLI, name string, namespace string) error {
	const sshPath = "/home/core/.ssh"
	labels := map[string]string{"name": "etcd-backup-server"}

	installScript := fmt.Sprintf(`
            #!/bin/bash

            echo "installing public key on host"
            FOLDER=%s
            FILE_NAME="${FOLDER}/authorized_keys"
            # not idempotent, will always append the key on pod restarts
            mkdir -p $FOLDER && echo "$PUBLIC_KEY" >> $FILE_NAME && chmod 0400 $FILE_NAME && echo "installed public key successfully"
			# need to chown for core, otherwise the user doesn't have access to the authorized_keys
			chown 1000:1000 $FILE_NAME 

			ls -l $FOLDER

            # work around the DS restart policy by never exiting the container
            sleep infinity`, sshPath)
	podSpec := applycorev1.PodSpec()
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName("ssh-key-installer").
			WithSecurityContext(applycorev1.SecurityContext().WithPrivileged(true)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(
				applycorev1.VolumeMount().WithName("ssh").WithMountPath(sshPath),
			).
			WithEnv(applycorev1.EnvVar().WithName("PUBLIC_KEY").
				WithValueFrom(applycorev1.EnvVarSource().WithSecretKeyRef(applycorev1.SecretKeySelector().WithName(name).WithKey("pubKey"))),
				// appending the time to ensure the DS updates if it already exists
				applycorev1.EnvVar().WithName("TIME").WithValue(time.Now().String())).
			WithCommand("/bin/bash", "-c", installScript),
	}
	podSpec.NodeSelector = map[string]string{"node-role.kubernetes.io/master": ""}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{
		*applycorev1.Toleration().WithKey("node-role.kubernetes.io/master").WithOperator(corev1.TolerationOpExists).WithEffect(corev1.TaintEffectNoSchedule),
	}

	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("ssh").WithHostPath(applycorev1.HostPathVolumeSource().WithPath(sshPath)),
		*applycorev1.Volume().WithName("keys").WithSecret(applycorev1.SecretVolumeSource().WithSecretName(name)),
	}

	ds := applyappsv1.DaemonSet(name, namespace).WithSpec(applyappsv1.DaemonSetSpec().WithTemplate(
		applycorev1.PodTemplateSpec().WithName(name).WithSpec(podSpec).WithLabels(labels),
	).WithSelector(applymetav1.LabelSelector().WithMatchLabels(labels)))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := oc.AdminKubeClient().AppsV1().DaemonSets(namespace).Apply(ctx, ds, metav1.ApplyOptions{FieldManager: name})
	if err != nil {
		return err
	}
	return nil
}

// createPrivatePublicSSHKeySecret will create a new private and public key as a
// secret with the given name in the given namespace
func createPrivatePublicSSHKeySecret(oc *exutil.CLI, name string, namespace string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4*1024)
	if err != nil {
		return fmt.Errorf("could not generate private key for CP nodes: %w", err)
	}

	if err := privateKey.Validate(); err != nil {
		return fmt.Errorf("could not validate private key for CP nodes: %w", err)
	}

	der := x509.MarshalPKCS1PrivateKey(privateKey)
	block := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   der,
	}
	pemBytes := pem.EncodeToMemory(&block)

	publicKey, err := xssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("could not generate public key for CP nodes: %w", err)
	}

	pubKey := xssh.MarshalAuthorizedKey(publicKey)
	framework.Logf("successfully created new public key: %s", string(pubKey))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	secret := applycorev1.Secret(name, namespace).WithData(map[string][]byte{
		"privKey": pemBytes,
		"pubKey":  pubKey,
	})
	_, err = oc.AdminKubeClient().CoreV1().Secrets(namespace).Apply(ctx, secret, metav1.ApplyOptions{FieldManager: name})
	if err != nil {
		return fmt.Errorf("could not save key secret for CP nodes: %w", err)
	}
	return nil
}
