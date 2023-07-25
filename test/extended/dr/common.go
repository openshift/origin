package dr

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/test/library"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	"github.com/stretchr/objx"
	xssh "golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	applyappsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applymetav1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/utils"
)

const (
	operatorWait           = 25 * time.Minute
	defaultSSHTimeout      = 5 * time.Minute
	openshiftEtcdNamespace = "openshift-etcd"
	sshPath                = "/tmp/ssh"
	// sshKeyDance is necessary because permissions on a secret mount can't be changed and ssh is very strict about this
	sshKeyDance = `
		CORE_SSH_BASE_DIR=$HOME/.ssh
		SSH_MOUNT_DIR=` + sshPath + `
		P_KEY=$SSH_MOUNT_DIR/privKey
		# we can't change the permissions on the secret mount, thus we copy it to HOME
		mkdir -p $CORE_SSH_BASE_DIR && chmod 700 $CORE_SSH_BASE_DIR
		cp $P_KEY $CORE_SSH_BASE_DIR/id_rsa
		P_KEY=$CORE_SSH_BASE_DIR/id_rsa
		chmod 600 $P_KEY
`
)

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

func internalIP(restoreNode *corev1.Node) string {
	internalIp := restoreNode.Name
	for _, address := range restoreNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			internalIp = address.Address
		}
	}
	return internalIp
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
	_ = wait.PollImmediate(30*time.Second, operatorWait, func() (bool, error) {
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

func waitForReadyEtcdStaticPods(client kubernetes.Interface, masterCount int) {
	g.By("Waiting for all etcd static pods to become ready")
	waitForPodsTolerateClientTimeout(
		client.CoreV1().Pods(openshiftEtcdNamespace),
		exutil.ParseLabelsOrDie("app=etcd"),
		exutil.CheckPodIsRunning,
		masterCount,
		40*time.Minute,
	)
}

func waitForPodsTolerateClientTimeout(c corev1client.PodInterface, label labels.Selector, predicate func(corev1.Pod) bool, count int, timeout time.Duration) {
	err := wait.Poll(60*time.Second, timeout, func() (bool, error) {
		p, e := exutil.GetPodNamesByFilter(c, label, predicate)
		if e != nil {
			framework.Logf("Saw an error waiting for etcd pods to become available: %v", e)
			// TODO tolerate transient etcd timeout only and fail other errors
			return false, nil
		}
		if len(p) != count {
			framework.Logf("Only %d of %d expected pods are ready", len(p), count)
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// InstallSSHKeyOnControlPlaneNodes will create a new private/public ssh keypair,
// create a new secret for both in the openshift-etcd namespace. Then it
// will append the public key on the host core user authorized_keys file with a daemon set.
func InstallSSHKeyOnControlPlaneNodes(oc *exutil.CLI) error {
	const name = "dr-ssh"

	err := createPrivatePublicSSHKeySecret(oc, name, openshiftEtcdNamespace)
	if err != nil {
		return err
	}

	err = createKeyInstallerDaemon(oc, name, openshiftEtcdNamespace)
	if err != nil {
		return err
	}

	err = ensureControlPlaneSSHAccess(oc, name, openshiftEtcdNamespace)
	if err != nil {
		return err
	}

	return nil
}

// ensureControlPlaneSSHAccess will test that the private key generated and installed with installSSHKeyOnControlPlaneNodes
// is working on all control plane nodes. This effectively polls until the pod executing ssh succeeds reaching all nodes.
func ensureControlPlaneSSHAccess(oc *exutil.CLI, name string, namespace string) error {
	const containerName = "ssh-key-tester"

	var cpNodeInternalIps []string
	for _, node := range masterNodes(oc) {
		framework.Logf("CP Node: %s, addresses: %v", node.ObjectMeta.Name, node.Status.Addresses)
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				cpNodeInternalIps = append(cpNodeInternalIps, address.Address)
			}
		}
	}

	framework.Logf("found internal IPs: %v", cpNodeInternalIps)

	testScript := fmt.Sprintf(`
		#!/bin/bash

		# ssh key dance
		%s
		
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
			`, sshKeyDance, strings.Join(cpNodeInternalIps, " "))

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
	return runPodAndWaitForSuccess(oc, pod)
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
            mkdir -p $FOLDER 

            PUB_FILE_NAME="${FOLDER}/authorized_keys"
			PRIV_FILE_NAME="${FOLDER}/id_rsa"

            # not idempotent, will always append the key on pod restarts - but ssh can deal with it
            echo "$PUBLIC_KEY" >> $PUB_FILE_NAME && chmod 0400 $PUB_FILE_NAME && echo "installed public key successfully"
            # this will also void all previously valid private keys (if any exists there)
			echo "$PRIVATE_KEY" > $PRIV_FILE_NAME && chmod 0600 $PRIV_FILE_NAME && echo "installed private key successfully"
			# need to chown for core (uid 1000), otherwise the user doesn't have access to the keys
			chown -R 1000:1000 $FOLDER 

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
			WithEnv(
				applycorev1.EnvVar().WithName("PRIVATE_KEY").WithValueFrom(applycorev1.EnvVarSource().WithSecretKeyRef(applycorev1.SecretKeySelector().WithName(name).WithKey("privKey"))),
				applycorev1.EnvVar().WithName("PUBLIC_KEY").WithValueFrom(applycorev1.EnvVarSource().WithSecretKeyRef(applycorev1.SecretKeySelector().WithName(name).WithKey("pubKey"))),
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
	).WithSelector(applymetav1.LabelSelector().WithMatchLabels(labels)).
		WithUpdateStrategy(applyappsv1.DaemonSetUpdateStrategy().WithRollingUpdate(applyappsv1.RollingUpdateDaemonSet().WithMaxUnavailable(intstr.FromInt(3)))))
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

// runClusterBackupScript will create a new pod to run on the backupNode and call the cluster-backup.sh script.
// This will get triggered through a CRD in CEO over the next couple of iterations.
func runClusterBackupScript(oc *exutil.CLI, backupNode *corev1.Node) error {
	const name = "etcd-backup-pod"
	framework.Logf("running backup script on node: %v", backupNode.Name)

	internalIp := internalIP(backupNode)

	backupScript := fmt.Sprintf(`
		#!/bin/bash
        set -exuo pipefail
		
		# ssh key dance
		%s

        TARGET_NODE_NAME=%s
 		ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${TARGET_NODE_NAME} <<EOF
		 sudo rm -rf /home/core/backup
		 sudo /usr/local/bin/cluster-backup.sh --force /home/core/backup
		 sudo chown -R core /home/core/backup
		 exit
EOF
        `, sshKeyDance, internalIp)

	podSpec := applycorev1.PodSpec().WithHostNetwork(true).WithRestartPolicy(corev1.RestartPolicyOnFailure)
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName("cluster-backup").
			WithSecurityContext(applycorev1.SecurityContext().WithPrivileged(true)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(
				applycorev1.VolumeMount().WithName("keys").WithMountPath(sshPath),
			).
			WithCommand("/bin/bash", "-c", backupScript),
	}
	podSpec.NodeSelector = map[string]string{"kubernetes.io/hostname": backupNode.Labels["kubernetes.io/hostname"]}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{
		*applycorev1.Toleration().WithKey("node-role.kubernetes.io/master").WithOperator(corev1.TolerationOpExists).WithEffect(corev1.TaintEffectNoSchedule),
	}

	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("keys").WithSecret(applycorev1.SecretVolumeSource().WithSecretName("dr-ssh")),
	}

	pod := applycorev1.Pod(name, openshiftEtcdNamespace).WithSpec(podSpec)
	return runPodAndWaitForSuccess(oc, pod)
}

func runClusterRestoreScript(oc *exutil.CLI, restoreNode *corev1.Node, backupNode *corev1.Node) error {
	const name = "etcd-restore-pod"
	framework.Logf("running restore script on node: %v", restoreNode.Name)

	backupInternalIp := internalIP(backupNode)
	restoreInternalIp := internalIP(restoreNode)

	restoreScript := fmt.Sprintf(`
		#!/bin/bash
        set -exuo pipefail
		
		# ssh key dance
		%s

        TARGET_NODE_NAME=%s 
        ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${TARGET_NODE_NAME} <<EOF
         sudo rm -rf /home/core/backup
         scp -o StrictHostKeyChecking=no -r core@%s:/home/core/backup .
         sudo chown -R core /home/core/backup
         SNAPSHOT=\$(ls -vd /home/core/backup/snapshot*.db | tail -1)
         mv \$SNAPSHOT /home/core/backup/snapshot.db

         sudo -s -- <<EOF
            rm -rf /var/lib/etcd
            mkdir -p /var/lib/etcd
            export ETCD_ETCDCTL_RESTORE=true
            export ETCD_RESTORE_SKIP_MOVE_CP_STATIC_PODS=true
            /usr/local/bin/cluster-restore.sh /home/core/backup
            # from here on out, the member should come back through the static pod installer in CEO
EOF
		 exit
EOF
        `, sshKeyDance, restoreInternalIp, backupInternalIp)

	podSpec := applycorev1.PodSpec().WithHostNetwork(true).WithRestartPolicy(corev1.RestartPolicyOnFailure)
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName("cluster-restore").
			WithSecurityContext(applycorev1.SecurityContext().WithPrivileged(true)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(
				applycorev1.VolumeMount().WithName("keys").WithMountPath(sshPath),
			).
			WithCommand("/bin/bash", "-c", restoreScript),
	}
	podSpec.NodeSelector = map[string]string{"kubernetes.io/hostname": restoreNode.Labels["kubernetes.io/hostname"]}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{
		*applycorev1.Toleration().WithKey("node-role.kubernetes.io/master").WithOperator(corev1.TolerationOpExists).WithEffect(corev1.TaintEffectNoSchedule),
	}

	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("keys").WithSecret(applycorev1.SecretVolumeSource().WithSecretName("dr-ssh")),
	}

	pod := applycorev1.Pod(name, openshiftEtcdNamespace).WithSpec(podSpec)
	return runPodAndWaitForSuccess(oc, pod)
}

func runDeleteAndRestoreScript(oc *exutil.CLI, restoreNode *corev1.Node, backupNode *corev1.Node, nonRecoveryNodes []*corev1.Node) error {
	const name = "bumping-etcd-restore-pod"
	framework.Logf("running deletes and restore script on node: %v", restoreNode.Name)

	backupInternalIp := internalIP(backupNode)
	restoreInternalIp := internalIP(restoreNode)

	var nonRecoveryIps []string
	for _, n := range nonRecoveryNodes {
		nonRecoveryIps = append(nonRecoveryIps, internalIP(n))
	}

	restoreScript := fmt.Sprintf(`
		#!/bin/bash
        set -exuo pipefail
		
		# ssh key dance
		%s

        NODE_IPS=( %s )
		for i in "${NODE_IPS[@]}"; do
		  echo "removing etcd static pod on [$i]"
		  ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${i} sudo rm -rf /etc/kubernetes/manifests/etcd-pod.yaml 
          echo "remove data dir on [$i]" 
          ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${i} sudo rm -rf /var/lib/etcd
          echo "restarting kubelet on [$i]"
		  ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${i} sudo systemctl restart kubelet.service
		done

        TARGET_NODE_NAME=%s 
        ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${TARGET_NODE_NAME} <<EOF
         sudo rm -rf /home/core/backup
         scp -o StrictHostKeyChecking=no -r core@%s:/home/core/backup .
         sudo chown -R core /home/core/backup
         SNAPSHOT=\$(ls -vd /home/core/backup/snapshot*.db | tail -1)
         mv \$SNAPSHOT /home/core/backup/snapshot.db
         sudo rm -rf /var/lib/etcd
         
         sudo ETCD_ETCDCTL_RESTORE_ENABLE_BUMP=true /usr/local/bin/cluster-restore.sh /home/core/backup
         # this will cause the pod to disappear effectively, must be the last statement
         sudo systemctl restart kubelet.service

EOF`, sshKeyDance, strings.Join(nonRecoveryIps, " "), restoreInternalIp, backupInternalIp)

	podSpec := applycorev1.PodSpec().WithHostNetwork(true).WithRestartPolicy(corev1.RestartPolicyOnFailure)
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName("cluster-restore").
			WithSecurityContext(applycorev1.SecurityContext().WithPrivileged(true)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(
				applycorev1.VolumeMount().WithName("keys").WithMountPath(sshPath),
			).
			WithCommand("/bin/bash", "-c", restoreScript),
	}

	podSpec.NodeSelector = map[string]string{"kubernetes.io/hostname": restoreNode.Labels["kubernetes.io/hostname"]}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{
		*applycorev1.Toleration().WithKey("node-role.kubernetes.io/master").WithOperator(corev1.TolerationOpExists).WithEffect(corev1.TaintEffectNoSchedule),
	}

	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("keys").WithSecret(applycorev1.SecretVolumeSource().WithSecretName("dr-ssh")),
	}

	pod := applycorev1.Pod(name, openshiftEtcdNamespace).WithSpec(podSpec)
	// we only run the pod and not wait for it, as it will not be tracked after the control plane comes back
	return runPod(oc, pod)
}

// TODO(thomas): this shouldn't be necessary once we can bump the etcd revisions
func runOVNRepairCommands(oc *exutil.CLI, restoreNode *corev1.Node, nonRecoveryNodes []*corev1.Node) error {
	const name = "etcd-ovn-repair-pod"
	framework.Logf("running ovn-repair script on node: %v", restoreNode.Name)

	var nodeIPs []string
	for _, n := range nonRecoveryNodes {
		nodeIPs = append(nodeIPs, internalIP(n))
	}
	// adding the restore node last is important for kubelet restarts, as the ovn-repair pod runs on top of that node
	nodeIPs = append(nodeIPs, internalIP(restoreNode))

	ovnRepairScript := fmt.Sprintf(`
		#!/bin/bash
        set -exuo pipefail
		
		# ssh key dance
		%s
        NODE_IPS=( %s )
		for i in "${NODE_IPS[@]}"; do
		  echo "removing ovn etc on [$i]"
		  ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${i} sudo rm -rf /var/lib/ovn/etc/ 
          echo "restarting kubelet on [$i]"
          # running this process async to get a chance to finish the pod gracefully
          ssh -i $P_KEY -o StrictHostKeyChecking=no -q core@${i} sudo systemctl restart kubelet.service &
        done
        exit`, sshKeyDance, strings.Join(nodeIPs, " "))

	podSpec := applycorev1.PodSpec().WithHostNetwork(true).WithRestartPolicy(corev1.RestartPolicyOnFailure)
	podSpec.Containers = []applycorev1.ContainerApplyConfiguration{
		*applycorev1.Container().
			WithName("ovn-repair").
			WithSecurityContext(applycorev1.SecurityContext().WithPrivileged(true)).
			WithImage(image.ShellImage()).
			WithVolumeMounts(
				applycorev1.VolumeMount().WithName("keys").WithMountPath(sshPath),
			).
			WithCommand("/bin/bash", "-c", ovnRepairScript),
	}
	podSpec.NodeSelector = map[string]string{"kubernetes.io/hostname": restoreNode.Labels["kubernetes.io/hostname"]}
	podSpec.Tolerations = []applycorev1.TolerationApplyConfiguration{
		*applycorev1.Toleration().WithKey("node-role.kubernetes.io/master").WithOperator(corev1.TolerationOpExists).WithEffect(corev1.TaintEffectNoSchedule),
	}

	podSpec.Volumes = []applycorev1.VolumeApplyConfiguration{
		*applycorev1.Volume().WithName("keys").WithSecret(applycorev1.SecretVolumeSource().WithSecretName("dr-ssh")),
	}

	pod := applycorev1.Pod(name, openshiftEtcdNamespace).WithSpec(podSpec)
	err := runPodAndWaitForSuccess(oc, pod)
	if err != nil {
		return err
	}

	return wait.PollImmediate(5*time.Minute, 30*time.Minute, func() (bool, error) {
		framework.Logf("attempting to delete all ovn pods...")
		pods, err := getAllOvnPods(oc)
		if err != nil {
			framework.Logf("error while attempting to get OVN pods: %v", err)
			return false, nil
		}

		numPods := len(pods)

		err = e2epod.DeletePodsWithGracePeriod(context.TODO(), oc.AdminKubeClient(), pods, 0)
		if err != nil {
			framework.Logf("error while attempting to delete OVN pods: %v", err)
			return false, nil
		}
		framework.Logf("all ovn pods were deleted successfully!")

		err = wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
			pods, err := getAllOvnPods(oc)
			if err != nil {
				framework.Logf("error while attempting to get OVN pods: %v", err)
				return false, nil
			}

			if len(pods) != numPods {
				framework.Logf("expected %d ovn pods, found only %d...", numPods, len(pods))
				return false, nil
			}

			// wait for all the ovn pods to be running, looping through all of them for better logging
			allRunning := true
			for _, pod := range pods {
				_, err := utils.PodRunningReady(&pod)
				if err != nil {
					framework.Logf("%v", err)
					allRunning = false
				}
			}

			return allRunning, nil
		})

		if err != nil {
			framework.Logf("error while waiting for OVN pods to become ready: %v", err)
			return false, nil
		}

		framework.Logf("all OVN pods are running, continuing")
		return true, nil
	})
}

func getAllOvnPods(oc *exutil.CLI) ([]corev1.Pod, error) {
	ovnKubeMasters, err := e2epod.GetPods(context.TODO(), oc.AdminKubeClient(), "openshift-ovn-kubernetes", map[string]string{"app": "ovnkube-master"})
	if err != nil {
		return nil, err
	}

	ovnNodes, err := e2epod.GetPods(context.TODO(), oc.AdminKubeClient(), "openshift-ovn-kubernetes", map[string]string{"app": "ovnkube-node"})
	if err != nil {
		return nil, err
	}

	return append(ovnKubeMasters, ovnNodes...), nil
}

func runPodAndWaitForSuccess(oc *exutil.CLI, pod *applycorev1.PodApplyConfiguration) error {
	err := runPod(oc, pod)
	if err != nil {
		return err
	}

	framework.Logf("Waiting for %s to complete...", *pod.Name)
	return e2epod.WaitForPodSuccessInNamespaceTimeout(context.TODO(), oc.AdminKubeClient(), *pod.Name, *pod.Namespace, 15*time.Minute)
}

func runPod(oc *exutil.CLI, pod *applycorev1.PodApplyConfiguration) error {
	err := wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
		err := e2epod.DeletePodWithGracePeriodByName(context.TODO(), oc.AdminKubeClient(), *pod.Name, *pod.Namespace, 0)
		if err != nil {
			framework.Logf("error while attempting to delete pod %s: %v", *pod.Name, err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	return wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		framework.Logf("Applying pod [%s] in namespace [%s]...", *pod.Name, *pod.Namespace)
		_, err = oc.AdminKubeClient().CoreV1().Pods(*pod.Namespace).Apply(ctx, pod, metav1.ApplyOptions{FieldManager: *pod.Name})
		if err != nil {
			framework.Logf("error while attempting to apply pod %s: %v", *pod.Name, err)
			return false, nil
		}
		return true, nil
	})
}

func removeMemberOfNode(oc *exutil.CLI, node *corev1.Node) error {
	etcdEndpointsConfigMap, err := oc.AdminKubeClient().CoreV1().ConfigMaps(openshiftEtcdNamespace).Get(context.Background(), "etcd-endpoints", metav1.GetOptions{})
	if err != nil {
		return err
	}

	memberIp := internalIP(node)
	for memberIdentifier, votingMemberIP := range etcdEndpointsConfigMap.Data {
		if votingMemberIP == memberIp {
			framework.Logf("found node with IP %s, has identifier [%v]. Removing from cluster...", memberIp, memberIdentifier)
			removeMember(oc, memberIdentifier)
			return nil
		}
	}

	return fmt.Errorf("removeMemberOfNode could not find any member with IP %s in current endpoint configmap: [%v]",
		memberIp, etcdEndpointsConfigMap.Data)
}

func removeMember(oc *exutil.CLI, memberID string) {
	pods, err := e2epod.GetPods(context.TODO(), oc.AdminKubeClient(), openshiftEtcdNamespace, map[string]string{"app": "etcd"})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, pod := range pods {
		_, err = utils.PodRunningReady(&pod)
		if err == nil {
			framework.Logf("found running etcd pod to exec member removal: %s", pod.Name)
			member, err := oc.AsAdmin().Run("exec").Args("-n", openshiftEtcdNamespace, pod.Name, "-c", "etcdctl", "etcdctl", "member", "remove", memberID).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(member).To(o.ContainSubstring("removed from cluster"))
			return
		}
	}
	framework.Logf("found no running etcd pod to exec member removal, failing test.")
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForEtcdToStabilizeOnTheSameRevision(t library.LoggingT, oc *exutil.CLI) error {
	podClient := oc.AdminKubeClient().CoreV1().Pods(openshiftEtcdNamespace)
	return library.WaitForPodsToStabilizeOnTheSameRevision(t, podClient, "app=etcd", 3, 10*time.Second, 5*time.Second, 30*time.Minute)
}

func waitForApiServerToStabilizeOnTheSameRevision(t library.LoggingT, oc *exutil.CLI) error {
	podClient := oc.AdminKubeClient().CoreV1().Pods("openshift-kube-apiserver")
	return library.WaitForPodsToStabilizeOnTheSameRevision(t, podClient, "apiserver=true", 3, 10*time.Second, 5*time.Second, 30*time.Minute)
}
