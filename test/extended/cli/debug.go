package cli

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	buildTimeout  = 10 * time.Minute
	deployTimeout = 2 * time.Minute
)

var _ = g.Describe("[sig-cli] oc debug", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("oc-debug", admissionapi.LevelBaseline)
	testCLIDebug := exutil.FixturePath("testdata", "test-cli-debug.yaml")
	testDeploymentConfig := exutil.FixturePath("testdata", "test-deployment-config.yaml")
	testDeployment := exutil.FixturePath("testdata", "test-deployment.yaml")
	testReplicationController := exutil.FixturePath("testdata", "test-replication-controller.yaml")
	helloPod := exutil.FixturePath("..", "..", "examples", "hello-openshift", "hello-pod.json")
	imageStreamsCentos := exutil.FixturePath("..", "..", "examples", "image-streams", "image-streams-centos7.json")

	g.It("deployment from a build [apigroup:image.openshift.io]", func() {
		projectName, err := oc.Run("project").Args("-q").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("-f", testCLIDebug).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		// wait for image stream to be present which means the build has completed
		err = wait.Poll(cliInterval, buildTimeout, func() (bool, error) {
			err := oc.Run("get").Args("imagestreamtags", "local-busybox:latest").Execute()
			return err == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// and for replicaset which means we can kick of debug session
		var rsName string
		err = wait.Poll(cliInterval, deployTimeout, func() (bool, error) {
			rsList, err := oc.AdminKubeClient().AppsV1().ReplicaSets(projectName).List(context.TODO(), metav1.ListOptions{LabelSelector: "deployment=local-busybox1"})
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, item := range rsList.Items {
				if item.Annotations["deployment.kubernetes.io/revision"] == "2" {
					rsName = rsList.Items[0].Name
				}
			}
			if rsName == "" {
				klog.Infof("Waiting for a replicaset with deployment.kubernetes.io/revision=2")
				return false, nil
			}
			rsName = rsList.Items[0].Name
			err = oc.Run("get").Args("replicasets", rsName).Execute()
			return err == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("should print the imagestream-based container entrypoint/command")
		var out string
		out, err = oc.Run("debug").Args("deployment/local-busybox1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp("Starting pod/local-busybox1-debug.* ...\n"))

		g.By("should print the overridden imagestream-based container entrypoint/command")
		out, err = oc.Run("debug").Args("deployment/local-busybox2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp("Starting pod/local-busybox2-debug.*, command was: foo bar baz qux\n"))

		g.By("should print the container image-based container entrypoint/command")
		out, err = oc.Run("debug").Args("deployment/busybox1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp("Starting pod/busybox1-debug.* ...\n"))

		g.By("should print the overridden container image-based container entrypoint/command")
		out, err = oc.Run("debug").Args("deployment/busybox2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp("Starting pod/busybox2-debug.*, command was: foo bar baz qux\n"))
	})

	g.It("dissect deployment config debug [apigroup:apps.openshift.io]", func() {
		err := oc.Run("create").Args("-f", testDeploymentConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var out string
		out, err = oc.Run("debug").Args("dc/test-deployment-config", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("- /bin/sh"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "--keep-annotations", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotations:"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "--as-root", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("runAsUser: 0"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "--as-root=false", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("runAsNonRoot: true"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "--as-user=1", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("runAsUser: 1"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "-t", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("stdinOnce"))
		o.Expect(out).To(o.ContainSubstring("tty"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "--tty=false", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("tty"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "-oyaml", "--", "/bin/env").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("- /bin/env"))
		o.Expect(out).NotTo(o.ContainSubstring("stdin"))
		o.Expect(out).NotTo(o.ContainSubstring("tty"))

		out, err = oc.Run("debug").Args("dc/test-deployment-config", "--node-name=invalid", "--", "/bin/env").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`on node "invalid"`))
	})

	g.It("dissect deployment debug", func() {
		err := oc.Run("create").Args("-f", testDeployment).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var out string
		out, err = oc.Run("debug").Args("deployment/test-deployment", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("- /bin/sh"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "--keep-annotations", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("annotations:"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "--as-root", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("runAsUser: 0"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "--as-root=false", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("runAsNonRoot: true"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "--as-user=1", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("runAsUser: 1"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "-t", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("stdinOnce"))
		o.Expect(out).To(o.ContainSubstring("tty"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "--tty=false", "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("tty"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "-oyaml", "--", "/bin/env").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("- /bin/env"))
		o.Expect(out).NotTo(o.ContainSubstring("stdin"))
		o.Expect(out).NotTo(o.ContainSubstring("tty"))

		out, err = oc.Run("debug").Args("deployment/test-deployment", "--node-name=invalid", "--", "/bin/env").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`on node "invalid"`))
	})

	g.It("does not require a real resource on the server", func() {
		out, err := oc.Run("debug").Args("-T", "-f", helloPod, "-oyaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("tty"))

		err = oc.Run("debug").Args("-f", helloPod, "--keep-liveness", "--keep-readiness", "-oyaml").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("debug").Args("-f", helloPod, "-oyaml", "--", "/bin/env").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("- /bin/env"))
		o.Expect(out).NotTo(o.ContainSubstring("stdin"))
		o.Expect(out).NotTo(o.ContainSubstring("tty"))
	})

	// TODO: write a test that emulates a TTY to verify the correct defaulting of what the pod is created

	g.It("ensure debug does not depend on a container actually existing for the selected resource [apigroup:apps.openshift.io]", func() {
		err := oc.Run("create").Args("-f", testReplicationController).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("create").Args("-f", testDeploymentConfig).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// The command should not hang waiting for an attachable pod. Timeout each cmd after 10s.
		err = oc.Run("scale").Args("--replicas=0", "rc/test-replication-controller").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		var out string
		out, err = oc.Run("debug").Args("--request-timeout=10s", "-c", "ruby-helloworld", "--one-container", "rc/test-replication-controller", "-o", "jsonpath='{.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-replication-controller-debug"))

		err = oc.Run("scale").Args("--replicas=0", "dc/test-deployment-config").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("debug").Args("--request-timeout=10s", "-c", "ruby-helloworld", "--one-container", "dc/test-deployment-config", "-o", "jsonpath='{.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-deployment-config"))
	})

	g.It("ensure debug does not depend on a container actually existing for the selected resource for deployment", func() {
		err := oc.Run("create").Args("-f", "-").InputString(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  labels:
    deployment: test-deployment
spec:
  replicas: 0
  selector:
    matchLabels:
      deployment: test-deployment
  template:
    metadata:
      labels:
        deployment: test-deployment
      name: test-deployment
    spec:
      containers:
      - name: ruby-helloworld
        image: openshift/origin-pod
        imagePullPolicy: IfNotPresent
`).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.Run("debug").Args("--request-timeout=10s", "-c", "ruby-helloworld", "--one-container", "deploy/test-deployment", "-o", "jsonpath='{.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-deployment-debug"))
	})

	g.It("ensure it works with image streams [apigroup:image.openshift.io]", func() {
		hasImageRegistry, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityImageRegistry)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("-f", imageStreamsCentos).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.Poll(cliInterval, cliTimeout, func() (bool, error) {
			err := oc.Run("get").Args("imagestreamtags", "wildfly:latest").Execute()
			return err == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		var out string
		var resolvedImageMatcher = o.MatchRegexp("image:.*oc-debug-.*/wildfly@sha256")
		if !hasImageRegistry {
			resolvedImageMatcher = o.ContainSubstring("image: quay.io/wildfly/wildfly-centos7")
		}

		out, err = oc.Run("debug").Args("istag/wildfly:latest", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(resolvedImageMatcher)

		var sha string
		sha, err = oc.Run("get").Args("istag/wildfly:latest", "--template", "{{ .image.metadata.name }}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err = oc.Run("debug").Args(fmt.Sprintf("isimage/wildfly@%s", sha), "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("image: quay.io/wildfly/wildfly-centos7"))
	})

	g.It("ensure that the label is set for node debug", func() {
		var err error

		ns := oc.Namespace()

		err = oc.AsAdmin().Run("label").Args("namespace", ns, "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.AsAdmin().WithoutNamespace().Run("auth").Args("can-i", "get", "nodes").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "User should be able to get nodes")

		nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodes.Items).NotTo(o.BeEmpty(), "No worker nodes found")

		var readyWorkerNode string

		for _, node := range nodes.Items {
			for _, nodeStatus := range node.Status.Conditions {
				if nodeStatus.Type == corev1.NodeReady && nodeStatus.Status == corev1.ConditionTrue {
					readyWorkerNode = node.Name
					break
				}
			}
		}
		o.Expect(readyWorkerNode).NotTo(o.BeEmpty(), "No ready worker node found")

		err = oc.AsAdmin().Run("debug").Args("node/"+readyWorkerNode, "--keep-labels=true", "--preserve-pod=true", "--", "sleep", "1").Execute()
		pods, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "debug.openshift.io/managed-by=oc-debug"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods.Items).To(o.HaveLen(1))
		o.Expect(pods.Items[0].Labels).To(o.HaveKeyWithValue("debug.openshift.io/managed-by", "oc-debug"))
		o.Expect(pods.Items[0].Labels).To(o.HaveLen(1))

		oc.AsAdmin().Run("delete").Args("pod", pods.Items[0].Name, "-n", ns).Output()

		// Wait for the pod to be deleted with 2 minute timeout and 10 second interval
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "debug.openshift.io/managed-by=oc-debug"})
			if err != nil {
				return false, err
			}
			return len(pods.Items) == 0, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Expected debug pod to be deleted")

		// Make sure the pod is deleted
		pods, err = oc.AdminKubeClient().CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "debug.openshift.io/managed-by=oc-debug"})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods.Items).To(o.HaveLen(0))

		err = oc.AsAdmin().Run("debug").Args("node/"+readyWorkerNode, "--preserve-pod=true", "--", "sleep", "1").Execute()

		// Tests the code fix in https://github.com/openshift/oc/pull/2074
		o.Expect(err).NotTo(o.HaveOccurred())
		pods, err = oc.AdminKubeClient().CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: "debug.openshift.io/managed-by=oc-debug"})
		o.Expect(pods.Items).To(o.HaveLen(1))
		o.Expect(pods.Items[0].Labels).To(o.HaveKeyWithValue("debug.openshift.io/managed-by", "oc-debug"))
		o.Expect(pods.Items[0].Labels).To(o.HaveLen(1))
	})
})
