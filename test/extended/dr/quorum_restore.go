package dr

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
	apps "k8s.io/kubernetes/test/e2e/upgrades/apps"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
)

const (
	machineAnnotationName = "machine.openshift.io/machine"
	localEtcdSignerYaml   = "/tmp/kube-etcd-cert-signer.yaml"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Disruptive]", func() {
	f := framework.NewDefaultFramework("disaster-recovery")
	f.SkipNamespaceCreation = true
	f.SkipPrivilegedPSPBinding = true

	oc := exutil.NewCLIWithoutNamespace("disaster-recovery")

	g.It("[Feature:EtcdRecovery] Cluster should restore itself after quorum loss", func() {
		framework.SkipUnlessProviderIs("aws")

		disruption.Run("Quorum Loss and Restore", "quorum_restore",
			disruption.TestData{},
			[]upgrades.Test{
				&upgrades.ServiceUpgradeTest{},
				&upgrades.SecretUpgradeTest{},
				&apps.ReplicaSetUpgradeTest{},
				&apps.StatefulSetUpgradeTest{},
				&apps.DeploymentUpgradeTest{},
				&apps.DaemonSetUpgradeTest{},
			},
			func() {

				config, err := framework.LoadConfig()
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

				framework.Logf("Verify SSH is available before restart")
				masters := masterNodes(oc)
				o.Expect(len(masters)).To(o.BeNumerically(">=", 1))
				survivingNode := masters[rand.Intn(len(masters))]
				survivingNodeName := survivingNode.Name
				expectSSH("true", survivingNode)

				err = scaleEtcdQuorum(oc.AdminKubeClient(), 0)
				o.Expect(err).NotTo(o.HaveOccurred())

				expectedNumberOfMasters := len(masters)
				survivingMachineName := getMachineNameByNodeName(oc, survivingNodeName)
				survivingMachine, err := ms.Get(survivingMachineName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				// Set etcd connection string before destroying masters, as ssh bastion may become unavailable
				etcdConnectionString := constructEtcdConnectionString([]string{survivingNodeName})

				framework.Logf("Destroy %d masters", len(masters)-1)
				var masterMachines []string
				for _, node := range masters {
					masterMachine := getMachineNameByNodeName(oc, node.Name)
					masterMachines = append(masterMachines, masterMachine)

					if node.Name == survivingNodeName {
						continue
					}

					framework.Logf("Destroying %s", masterMachine)
					err = ms.Delete(masterMachine, &metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				pollConfig := rest.CopyConfig(config)
				pollConfig.Timeout = 5 * time.Second
				pollClient, err := kubernetes.NewForConfig(pollConfig)
				o.Expect(err).NotTo(o.HaveOccurred())

				if len(masters) != 1 {
					framework.Logf("Wait for control plane to become unresponsive (may take several minutes)")
					failures := 0
					err = wait.Poll(5*time.Second, 30*time.Minute, func() (done bool, err error) {
						_, err = pollClient.CoreV1().Nodes().List(metav1.ListOptions{})
						if err != nil {
							failures++
						} else {
							failures = 0
						}

						// there is a small chance the cluster restores the default replica size during
						// this loop process, so keep forcing quorum guard to be zero, without failing on
						// errors
						scaleEtcdQuorum(pollClient, 0)

						// wait to see the control plane go down for good to avoid a transient failure
						return failures > 4, nil
					})
				}

				framework.Logf("Perform etcd backup on remaining machine %s (machine %s)", survivingNodeName, survivingMachineName)
				expectSSH("sudo -i /bin/bash -c 'rm -f /root/assets/backup/snapshot*'; sudo -i /bin/bash -x /usr/local/bin/etcd-snapshot-backup.sh /root/assets/backup", survivingNode)
				framework.Logf("Restore etcd on remaining node %s (machine %s)", survivingNodeName, survivingMachineName)
				expectSSH(fmt.Sprintf("sudo -i /bin/bash -c '/bin/bash -x /usr/local/bin/etcd-snapshot-restore.sh /root/assets/backup/snapshot* %s'", etcdConnectionString), survivingNode)

				framework.Logf("Wait for API server to come up")
				time.Sleep(30 * time.Second)
				err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
					nodes, err := pollClient.CoreV1().Nodes().List(metav1.ListOptions{Limit: 2})
					if err != nil || nodes.Items == nil {
						framework.Logf("return false - err %v nodes.Items %v", err, nodes.Items)
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				if expectedNumberOfMasters == 1 {
					framework.Logf("Cannot create new masters, you must manually create masters and update their DNS entries according to the docs")
				} else {
					framework.Logf("Create new masters")
					for _, master := range masterMachines {
						if master == survivingMachineName {
							continue
						}
						framework.Logf("Creating master %s", master)
						newMaster := survivingMachine.DeepCopy()
						// The providerID is relied upon by the machine controller to determine a machine
						// has been provisioned
						// https://github.com/openshift/cluster-api/blob/c4a461a19efb8a25b58c630bed0829512d244ba7/pkg/controller/machine/controller.go#L306-L308
						unstructured.SetNestedField(newMaster.Object, "", "spec", "providerID")
						newMaster.SetName(master)
						newMaster.SetResourceVersion("")
						newMaster.SetSelfLink("")
						newMaster.SetUID("")
						newMaster.SetCreationTimestamp(metav1.NewTime(time.Time{}))
						// retry until the machine gets created
						err := wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
							_, err := ms.Create(newMaster, metav1.CreateOptions{})
							if errors.IsAlreadyExists(err) {
								framework.Logf("Waiting for old machine object %s to be deleted so we can create a new one", master)
								return false, nil
							}
							if err != nil {
								return false, err
							}
							return true, nil
						})
						o.Expect(err).NotTo(o.HaveOccurred())
					}

					framework.Logf("Waiting for machines to be created")
					err = wait.Poll(30*time.Second, 10*time.Minute, func() (done bool, err error) {
						mastersList, err := ms.List(metav1.ListOptions{
							LabelSelector: "machine.openshift.io/cluster-api-machine-role=master",
						})
						if err != nil || mastersList.Items == nil {
							framework.Logf("return false - err %v mastersList.Items %v", err, mastersList.Items)
							return false, err
						}
						return len(mastersList.Items) == expectedNumberOfMasters, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())

					framework.Logf("Wait for masters to join as nodes and go ready")
					err = wait.Poll(30*time.Second, 30*time.Minute, func() (done bool, err error) {
						defer func() {
							if r := recover(); r != nil {
								fmt.Println("Recovered from panic", r)
							}
						}()
						nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
						if err != nil {
							return false, err
						}
						ready := countReady(nodes.Items)
						if ready != expectedNumberOfMasters {
							framework.Logf("%d nodes still unready", expectedNumberOfMasters-ready)
							return false, nil
						}
						return true, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				// reload all machine info
				masterMachines = nil
				masters = masterNodes(oc)
				for _, node := range masters {
					masterMachine := getMachineNameByNodeName(oc, node.Name)
					masterMachines = append(masterMachines, masterMachine)
				}

				infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get("cluster", metav1.GetOptions{})
				o.Expect(infra.Status.EtcdDiscoveryDomain).NotTo(o.BeEmpty())
				domain := infra.Status.EtcdDiscoveryDomain
				framework.Logf("Etcd recovery domain: %s", domain)

				var survivingMasterIP string
				var masterEtcdDomains []string
				var masterIPs []string
				dnsUpdates := make(map[string]string)
				for i, node := range masters {
					// IMPORTANT: by convention the index is the last segment of the machine, if this
					// changes these tests die horribly
					machine := masterMachines[i]
					last := strings.LastIndex(machine, "-")
					o.Expect(last).ToNot(o.Equal(-1))
					index := machine[last+1:]
					o.Expect(index).ToNot(o.BeEmpty())
					etcdName := fmt.Sprintf("etcd-%s.%s", index, domain)
					masterEtcdDomains = append(masterEtcdDomains, etcdName)

					var masterIP string
					for _, address := range node.Status.Addresses {
						if address.Type == "InternalIP" {
							masterIP = address.Address
							break
						}
					}
					o.Expect(masterIP).ToNot(o.BeEmpty())
					masterIPs = append(masterIPs, masterIP)

					if node.Name == survivingNodeName {
						survivingMasterIP = masterIP
						continue
					}
					framework.Logf("Update DNS records for %s to %s", etcdName, masterIP)
					dnsUpdates[etcdName] = masterIP
				}
				dnsErr := updateDNS(domain, dnsUpdates, framework.TestContext.Provider)
				if dnsErr != nil {
					framework.Logf("Could not complete DNS updates, continuing anyway: %v", dnsErr, err)
				}

				imagePullSecretPath := getPullSecret(oc)
				defer os.Remove(imagePullSecretPath)
				runPodSigner(oc, survivingNode, imagePullSecretPath)

				framework.Logf("Restore etcd on remaining masters")
				setupEtcdEnvImage := getImagePullSpecFromRelease(oc, imagePullSecretPath, "machine-config-operator")
				kubeClientAgent := getImagePullSpecFromRelease(oc, imagePullSecretPath, "kube-client-agent")
				for i, node := range masters {
					if node.Name == survivingNodeName {
						framework.Logf("Skipping node as its the surviving master")
						continue
					}
					err := wait.PollImmediate(15*time.Second, 5*time.Minute, func() (bool, error) {
						res, err := ssh(fmt.Sprintf(`dig +short "%s"`, masterEtcdDomains[i]), node)
						if err != nil {
							framework.Logf("Retrieving DNS name for %s failed: %v", node.Name, err)
							return false, nil
						}
						out := strings.TrimSpace(res.Stdout)
						if out != masterIPs[i] {
							framework.Logf("Expected DNS for %s to be %s, got %s", masterEtcdDomains[i], masterIPs[i], out)
							return false, nil
						}
						return true, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
					expectSSH(fmt.Sprintf("sudo -i env SETUP_ETCD_ENVIRONMENT=%s KUBE_CLIENT_AGENT=%s /bin/bash -x /usr/local/bin/etcd-member-recover.sh %s \"etcd-member-%s\"", setupEtcdEnvImage, kubeClientAgent, survivingMasterIP, node.GetName()), node)
				}

				framework.Logf("Wait for etcd pods to become available")
				_, err = exutil.WaitForPods(
					oc.AdminKubeClient().CoreV1().Pods("openshift-etcd"),
					exutil.ParseLabelsOrDie("k8s-app=etcd"),
					exutil.CheckPodIsReady,
					expectedNumberOfMasters,
					10*time.Minute,
				)
				o.Expect(err).NotTo(o.HaveOccurred())

				scaleEtcdQuorum(pollClient, expectedNumberOfMasters)

				framework.Logf("Remove etcd signer")
				err = oc.AdminKubeClient().CoreV1().Pods("openshift-config").Delete("etcd-signer", &metav1.DeleteOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=1707006#
				// SDN won't switch to Degraded mode when service is down after disaster recovery
				// restartSDNPods(oc)
				waitForMastersToUpdate(oc, mcps)
				waitForOperatorsToSettle(coc)

				// we shouldn't have errored, but try what we can
				o.Expect(dnsErr).ToNot(o.HaveOccurred())
			})
	},
	)
})

func scaleEtcdQuorum(client kubernetes.Interface, replicas int) error {
	etcdQGScale, err := client.AppsV1().Deployments("openshift-machine-config-operator").GetScale("etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	if etcdQGScale.Spec.Replicas == int32(replicas) {
		return nil
	}
	framework.Logf("Scale etcd-quorum-guard to %d replicas", replicas)
	etcdQGScale.Spec.Replicas = int32(replicas)
	_, err = client.AppsV1().Deployments("openshift-machine-config-operator").UpdateScale("etcd-quorum-guard", etcdQGScale)
	if err != nil {
		return err
	}

	etcdQGScale, err = client.AppsV1().Deployments("openshift-machine-config-operator").GetScale("etcd-quorum-guard", metav1.GetOptions{})
	if err != nil {
		return err
	}
	o.Expect(etcdQGScale.Spec.Replicas).To(o.Equal(int32(replicas)))
	return nil
}

func runPodSigner(oc *exutil.CLI, survivingNode *corev1.Node, imagePullSecretPath string) {
	framework.Logf("Run etcd signer pod")
	nodeHostname := strings.Split(survivingNode.Name, ".")[0]

	kubeEtcdSignerServerImage := getImagePullSpecFromRelease(oc, imagePullSecretPath, "kube-etcd-signer-server")
	expectSSH(fmt.Sprintf("sudo -i env KUBE_ETCD_SIGNER_SERVER=%s /bin/bash -x /usr/local/bin/tokenize-signer.sh %s && sudo -i install -o core -g core /root/assets/manifests/kube-etcd-cert-signer.yaml /tmp/kube-etcd-cert-signer.yaml", kubeEtcdSignerServerImage, nodeHostname), survivingNode)
	etcdSignerYaml := fetchFileContents(survivingNode, "/tmp/kube-etcd-cert-signer.yaml")
	err := oc.Run("apply").InputString(etcdSignerYaml).Args("-f", "-").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	framework.Logf("Wait for etcd signer pod to become Ready")
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
	framework.Logf("Saving image pull secret")
	//TODO: copy of test/extended/operators/images.go, move this to a common func
	imagePullSecret, err := oc.KubeFramework().ClientSet.CoreV1().Secrets("openshift-config").Get("pull-secret", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if err != nil {
		framework.Failf("unable to get pull secret for cluster: %v", err)
	}

	// cache file to local temp location
	imagePullFile, err := ioutil.TempFile("", "image-pull-secret")
	if err != nil {
		framework.Failf("unable to create a temporary file: %v", err)
	}

	// write the content
	imagePullSecretBytes := imagePullSecret.Data[".dockerconfigjson"]
	if _, err := imagePullFile.Write(imagePullSecretBytes); err != nil {
		framework.Failf("unable to write pull secret to temp file: %v", err)
	}
	if err := imagePullFile.Close(); err != nil {
		framework.Failf("unable to close file: %v", err)
	}
	framework.Logf("Image pull secret: %s", imagePullFile.Name())
	return imagePullFile.Name()
}

func getImagePullSpecFromRelease(oc *exutil.CLI, imagePullSecretPath, imageName string) string {
	var image string
	err := wait.PollImmediate(5*time.Second, 2*time.Minute, func() (bool, error) {
		location, err := oc.Run("adm", "release", "info").Args("--image-for", imageName, "--registry-config", imagePullSecretPath).Output()
		if err != nil {
			framework.Logf("Unable to find release info, retrying: %v", err)
			return false, nil
		}
		image = location
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return image
}

func updateDNS(domain string, dnsUpdates map[string]string, provider string) error {
	if len(dnsUpdates) == 0 {
		return nil
	}

	switch provider {
	case "aws":
		ssn := session.New()
		r53 := route53.New(ssn)
		zones, err := r53.ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
			DNSName:  aws.String(domain),
			MaxItems: aws.String("1"),
		})
		if err != nil {
			return err
		}
		if len(zones.HostedZones) == 0 {
			return fmt.Errorf("unable to find hosted zone for domain %q", domain)
		}

		// update route53 with the correct A records
		input := &route53.ChangeResourceRecordSetsInput{
			HostedZoneId: zones.HostedZones[0].Id,
			ChangeBatch: &route53.ChangeBatch{
				Comment: aws.String(fmt.Sprintf("update master addresses %v", dnsUpdates)),
			},
		}
		for name, ip := range dnsUpdates {
			input.ChangeBatch.Changes = append(input.ChangeBatch.Changes, &route53.Change{
				Action: aws.String(route53.ChangeActionUpsert),
				ResourceRecordSet: &route53.ResourceRecordSet{
					Name:            aws.String(name),
					Type:            aws.String("A"),
					TTL:             aws.Int64(60),
					ResourceRecords: []*route53.ResourceRecord{{Value: aws.String(ip)}},
				},
			})
		}
		change, err := r53.ChangeResourceRecordSets(input)
		if err != nil {
			return err
		}

		// wait until we sync
		var lastErr error
		if err := wait.PollImmediate(15*time.Second, 5*time.Minute, func() (bool, error) {
			status, err := r53.GetChange(&route53.GetChangeInput{Id: change.ChangeInfo.Id})
			if err != nil {
				return false, err
			}
			lastErr = fmt.Errorf("operation has status %s", *status.ChangeInfo.Status)
			return *status.ChangeInfo.Status == route53.ChangeStatusInsync, nil
		}); err != nil {
			if err == wait.ErrWaitTimeout && lastErr != nil {
				return lastErr
			}
			return err
		}
		return nil

	default:
		return fmt.Errorf("no DNS implementation available for provider %s, control plane cannot be restored", provider)
	}
}

func getMachineNameByNodeName(oc *exutil.CLI, name string) string {
	masterNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	annotations := masterNode.GetAnnotations()
	o.Expect(annotations).To(o.HaveKey(machineAnnotationName))
	return strings.Split(annotations[machineAnnotationName], "/")[1]
}
