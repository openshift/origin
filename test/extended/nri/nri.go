package nri

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

// Exercises the NRI path from CRI-O to a plugin to a workload through the
// Kubernetes API: the plugin is a DaemonSet and the workloads are pods. The
// scenarios map to the cases in CRI-O's test/nri.bats, see README.md.
var _ = g.Describe("[Suite:openshift/nri][sig-node] Node Resource Interface", g.Ordered, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("nri").AsAdmin()

	var (
		nodeName        string
		namespace       string
		node            *corev1.Node
		podImage        = image.ShellImage()
		pluginImageName string
		cpuset          string
	)

	optIn := func(annotations map[string]string) map[string]string {
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[pluginOptIn] = "true"
		return annotations
	}

	g.BeforeAll(func(ctx context.Context) {
		nodeutils.SkipOnMicroShift(oc)

		// The e2e framework deletes namespaces created by SetupProject after
		// every spec, which would take the plugin down between our specs.
		var err error
		namespace, err = createTestNamespace(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create the test namespace")
		oc.SetNamespace(namespace)

		g.DeferCleanup(func(ctx context.Context) {
			deleteNamespace(ctx, oc, namespace)
		})

		node, err = pickNode(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeName = node.Name

		if cpus := node.Status.Capacity.Cpu().Value(); cpus >= 2 {
			cpuset = strconv.Itoa(int(cpus) - 1)
		}

		pluginImageName, err = pluginImage(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// The plugin needs a privileged pod to reach the root-only NRI socket.
		grantPrivilegedSCC(ctx, oc, namespace)

		deployPlugin(ctx, oc, node, pluginImageName, pluginOptions{name: pluginName, idx: "50"})
		if err := waitForPlugin(ctx, oc, pluginName); err != nil {
			if logs, logErr := pluginLogs(ctx, oc, pluginName); logErr == nil && strings.Contains(logs, "failed to connect to NRI service") {
				g.Skip(fmt.Sprintf("NRI is not enabled on node %s", nodeName))
			}
			dumpPluginState(ctx, oc, pluginName)
			o.Expect(err).NotTo(o.HaveOccurred(), "NRI test plugin did not configure with CRI-O")
		}
	})

	// The framework also clears the CLI namespace after every spec.
	g.JustBeforeEach(func() {
		oc.SetNamespace(namespace)
	})

	g.AfterAll(func(ctx context.Context) {
		deletePlugin(ctx, oc, pluginName)
	})

	g.It("enables NRI in CRI-O and registers the plugin", func(ctx context.Context) {
		events, err := readEvents(ctx, oc, pluginName)
		o.Expect(err).NotTo(o.HaveOccurred())

		var configured, synced bool
		for _, ev := range events {
			switch ev.Kind {
			case "configure":
				configured = true
			case "synchronize-pod", "synchronize-container":
				synced = true
			}
		}
		o.Expect(configured).To(o.BeTrue(), "plugin never received a Configure request from CRI-O")
		o.Expect(synced).To(o.BeTrue(), "plugin never synchronized with CRI-O")
	})

	g.It("relays pod and container lifecycle events to the plugin", func(ctx context.Context) {
		pod := createPod(ctx, oc, namespace, nodeName, "nri-events", podImage, optIn(nil), "/bin/bash", "-c", "sleep 3600")
		waitPodRunning(ctx, oc, namespace, pod.Name, 3*time.Minute)

		podRef := namespace + "/" + pod.Name
		for _, kind := range []string{"run-pod", "create-container", "start-container"} {
			_, err := waitForEvent(ctx, oc, pluginName, func(ev event) bool {
				return ev.Kind == kind && ev.Pod == podRef
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "did not observe %s for %s", kind, podRef)
		}

		deletePod(ctx, oc, namespace, pod.Name)

		for _, kind := range []string{"stop-container", "stop-pod", "remove-pod"} {
			_, err := waitForEvent(ctx, oc, pluginName, func(ev event) bool {
				return ev.Kind == kind && ev.Pod == podRef
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "did not observe %s for %s", kind, podRef)
		}
	})

	g.It("injects environment, annotations, mounts and cpuset into opted-in containers", func(ctx context.Context) {
		if cpuset == "" {
			g.Skip("node has a single online CPU, cannot observe a cpuset adjustment")
		}
		pod := createPod(ctx, oc, namespace, nodeName, "nri-adjust", podImage, optIn(map[string]string{
			annEnv:    "NRI_TEST_ENV=hello-from-nri,NRI_SECOND=2",
			annMounts: "/proc/version:/mnt/nri/version:ro,/dev/null:/mnt/nri/null",
			annAnnos:  "nri.openshift.io/injected=yes",
			annCpuset: cpuset,
		}), "/bin/bash", "-c", "sleep 3600")
		waitPodRunning(ctx, oc, namespace, pod.Name, 3*time.Minute)

		ev, err := waitForEvent(ctx, oc, pluginName, func(ev event) bool {
			return ev.Kind == "create-container" && ev.Pod == namespace+"/"+pod.Name
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ev.Adjustments).To(o.ContainElement("env"))
		o.Expect(ev.Adjustments).To(o.ContainElement("mounts"))
		o.Expect(ev.Adjustments).To(o.ContainElement("annotations"))
		o.Expect(ev.Adjustments).To(o.ContainElement("cpuset"))

		out, err := execInPod(ctx, oc, namespace, pod.Name, "env")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("NRI_TEST_ENV=hello-from-nri"))
		o.Expect(out).To(o.ContainSubstring("NRI_SECOND=2"))

		out, err = execInPod(ctx, oc, namespace, pod.Name, "cat", "/mnt/nri/version")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Linux version"))

		_, err = execInPod(ctx, oc, namespace, pod.Name, "sh", "-c", "echo nri > /mnt/nri/version")
		o.Expect(err).To(o.HaveOccurred(), "the injected mount is not read-only")

		out, err = execInPod(ctx, oc, namespace, pod.Name, "stat", "-c", "%F %t:%T", "/mnt/nri/null")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(out)).To(o.Equal("character special file 1:3"))
		_, err = execInPod(ctx, oc, namespace, pod.Name, "sh", "-c", "echo nri > /mnt/nri/null")
		o.Expect(err).NotTo(o.HaveOccurred(), "the read-write mount is not writable")

		out, err = execInPod(ctx, oc, namespace, pod.Name, "grep", "Cpus_allowed_list", "/proc/self/status")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(out)).To(o.Equal("Cpus_allowed_list:\t" + cpuset))
	})

	g.It("injects device nodes into opted-in containers", func(ctx context.Context) {
		pod := createPod(ctx, oc, namespace, nodeName, "nri-device", podImage, optIn(map[string]string{
			annDevices: "/dev/nri-null:c:1:3:666",
		}), "/bin/bash", "-c", "sleep 3600")
		waitPodRunning(ctx, oc, namespace, pod.Name, 3*time.Minute)

		out, err := execInPod(ctx, oc, namespace, pod.Name, "stat", "-c", "%F %a %t:%T", "/dev/nri-null")
		o.Expect(err).NotTo(o.HaveOccurred())
		// major 1, minor 3 in hex, mode 666.
		o.Expect(strings.TrimSpace(out)).To(o.Equal("character special file 666 1:3"))

		_, err = execInPod(ctx, oc, namespace, pod.Name, "sh", "-c", "dd if=/dev/nri-null of=/dev/null bs=1 count=1")
		o.Expect(err).NotTo(o.HaveOccurred(), "the injected device is not usable")
	})

	g.It("fails container creation when the plugin fails the adjustment", func(ctx context.Context) {
		// The pod has to reach a terminal phase: it can never start (the plugin
		// rejects it), and a pod deleted while Pending trips the pod-lifecycle
		// monitor.
		pod := createPodWithDeadline(ctx, oc, namespace, nodeName, "nri-fail", podImage, optIn(map[string]string{
			annFail: "true",
		}), 10, "/bin/bash", "-c", "sleep 3600")

		err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			events, err := oc.AdminKubeClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: "involvedObject.name=" + pod.Name,
			})
			if err != nil {
				return false, err
			}
			for _, ev := range events.Items {
				if ev.Reason == "Failed" && strings.Contains(ev.Message, "failing CreateContainer on request of") {
					return true, nil
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "the NRI plugin error did not fail container creation")

		logs, err := oc.AsAdmin().Run("logs").Args("-n", namespace, "ds/"+pluginName, "--tail=200").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(logs).To(o.ContainSubstring("failing CreateContainer on request of"), "the plugin did not log the failure")

		current, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(current.Status.Phase).NotTo(o.Equal(corev1.PodRunning))

		err = e2epod.WaitForPodCondition(ctx, oc.AdminKubeClient(), namespace, pod.Name, "failed", 2*time.Minute, func(p *corev1.Pod) (bool, error) {
			return p.Status.Phase == corev1.PodFailed, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "the failing pod did not reach a terminal phase")
	})

	g.It("does not adjust pods that do not opt in", func(ctx context.Context) {
		pod := createPod(ctx, oc, namespace, nodeName, "nri-noop", podImage, map[string]string{
			annMounts: "/proc/version:/mnt/nri/version:ro",
		}, "/bin/bash", "-c", "sleep 3600")
		waitPodRunning(ctx, oc, namespace, pod.Name, 3*time.Minute)

		out, err := execInPod(ctx, oc, namespace, pod.Name, "env")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("NRI_TEST_ENV"))

		_, err = execInPod(ctx, oc, namespace, pod.Name, "cat", "/mnt/nri/version")
		o.Expect(err).To(o.HaveOccurred(), "an unannotated pod received the injected mount")

		events, err := readEvents(ctx, oc, pluginName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hasCreateContainerEvent(events, namespace+"/"+pod.Name)).To(o.BeFalse(),
			"the plugin recorded an adjustment for a pod that did not opt in")
	})

	g.It("ignores pods in another namespace", func(ctx context.Context) {
		// Same privileges as the suite's namespace, so only the plugin's
		// namespace filter can keep it from adjusting the pod.
		other, err := createNamespace(ctx, oc, "e2e-test-nri-other-", map[string]string{
			"pod-security.kubernetes.io/enforce":             "privileged",
			"pod-security.kubernetes.io/audit":               "privileged",
			"pod-security.kubernetes.io/warn":                "privileged",
			"security.openshift.io/scc.podSecurityLabelSync": "false",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		grantPrivilegedSCC(ctx, oc, other)
		g.DeferCleanup(func(ctx context.Context) {
			deleteNamespace(ctx, oc, other)
		})

		pod := createPod(ctx, oc, other, nodeName, "nri-other", podImage, map[string]string{
			pluginOptIn: "true",
			annEnv:      "NRI_TEST_ENV=should-not-appear",
			annMounts:   "/proc/version:/mnt/nri/version:ro",
		}, "/bin/bash", "-c", "sleep 3600")
		waitPodRunning(ctx, oc, other, pod.Name, 3*time.Minute)

		out, err := execInPod(ctx, oc, other, pod.Name, "env")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("NRI_TEST_ENV"))

		_, err = execInPod(ctx, oc, other, pod.Name, "cat", "/mnt/nri/version")
		o.Expect(err).To(o.HaveOccurred(), "a pod outside the plugin's namespace received the injected mount")

		events, err := readEvents(ctx, oc, pluginName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(hasCreateContainerEvent(events, other+"/"+pod.Name)).To(o.BeFalse(),
			"the plugin adjusted a pod outside its namespace")
	})

	g.It("does not admit a privileged plugin pod without the suite's privileges", func(ctx context.Context) {
		noSCC, err := createNamespace(ctx, oc, "e2e-test-nri-noscc-", map[string]string{
			"pod-security.kubernetes.io/enforce":             "privileged",
			"pod-security.kubernetes.io/audit":               "privileged",
			"pod-security.kubernetes.io/warn":                "privileged",
			"security.openshift.io/scc.podSecurityLabelSync": "false",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func(ctx context.Context) {
			deleteNamespace(ctx, oc, noSCC)
		})

		name := createPrivilegedDeployment(ctx, oc, noSCC, pluginImageName)
		waitForDeploymentForbidden(ctx, oc, noSCC, name, "security context constraint")

		restricted, err := createNamespace(ctx, oc, "e2e-test-nri-restricted-", nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func(ctx context.Context) {
			deleteNamespace(ctx, oc, restricted)
		})

		name = createPrivilegedDeployment(ctx, oc, restricted, pluginImageName)
		waitForDeploymentForbidden(ctx, oc, restricted, name, "security context constraint|PodSecurity")
	})

	g.It("merges adjustments from two plugins with different indices", func(ctx context.Context) {
		second := pluginOptions{
			name:              secondPluginName,
			idx:               "60",
			extraEnv:          "NRI_PLUGIN_B=yes",
			ignoreAnnotations: true,
		}
		deployPlugin(ctx, oc, node, pluginImageName, second)
		g.DeferCleanup(func(ctx context.Context) {
			deletePlugin(ctx, oc, secondPluginName)
			waitNoPluginPods(ctx, oc, secondPluginName)
		})
		if err := waitForPlugin(ctx, oc, secondPluginName); err != nil {
			dumpPluginState(ctx, oc, secondPluginName)
			o.Expect(err).NotTo(o.HaveOccurred(), "second NRI test plugin did not configure with CRI-O")
		}

		pod := createPod(ctx, oc, namespace, nodeName, "nri-two-plugins", podImage, optIn(map[string]string{
			annEnv: "NRI_PLUGIN_A=yes",
		}), "/bin/bash", "-c", "sleep 3600")
		waitPodRunning(ctx, oc, namespace, pod.Name, 3*time.Minute)

		out, err := execInPod(ctx, oc, namespace, pod.Name, "env")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("NRI_PLUGIN_A=yes"))
		o.Expect(out).To(o.ContainSubstring("NRI_PLUGIN_B=yes"))

		matchPod := func(ev event) bool {
			return ev.Kind == "create-container" && ev.Pod == namespace+"/"+pod.Name
		}
		evIndex50, err := waitForEvent(ctx, oc, pluginName, matchPod)
		o.Expect(err).NotTo(o.HaveOccurred(), "index 50 plugin did not observe the container")
		evIndex60, err := waitForEvent(ctx, oc, secondPluginName, matchPod)
		o.Expect(err).NotTo(o.HaveOccurred(), "index 60 plugin did not observe the container")
		o.Expect(evIndex50.Time).To(o.BeNumerically(">", 0), "index 50 event has no timestamp")
		o.Expect(evIndex60.Time).To(o.BeNumerically(">", 0), "index 60 event has no timestamp")
		o.Expect(evIndex50.Time).To(o.BeNumerically("<=", evIndex60.Time), "plugins were not invoked in index order")
	})

	g.It("starts a batch of pods with a plugin registered", func(ctx context.Context) {
		const count = 10
		names := make([]string, 0, count)
		for i := 0; i < count; i++ {
			name := fmt.Sprintf("nri-churn-%d", i)
			createPod(ctx, oc, namespace, nodeName, name, podImage, optIn(nil), "/bin/bash", "-c", "sleep 3600")
			names = append(names, name)
		}
		for _, name := range names {
			waitPodRunning(ctx, oc, namespace, name, 5*time.Minute)
		}

		events, err := readEvents(ctx, oc, pluginName)
		o.Expect(err).NotTo(o.HaveOccurred())
		observed := createContainerPods(events)
		for _, name := range names {
			o.Expect(observed).To(o.HaveKey(namespace+"/"+name), "the plugin did not observe pod %s", name)
		}

		for _, name := range names {
			deletePod(ctx, oc, namespace, name)
		}
	})

	g.It("keeps creating pods while the plugin is down and resynchronizes when it returns", func(ctx context.Context) {
		g.By("stopping the plugin")
		resumePlugin := pausePlugin(ctx, oc)
		g.DeferCleanup(resumePlugin)

		g.By("creating a pod with no plugin connection")
		pod := createPod(ctx, oc, namespace, nodeName, "nri-noplugin", podImage, optIn(nil), "/bin/bash", "-c", "sleep 3600")
		waitPodRunning(ctx, oc, namespace, pod.Name, 3*time.Minute)

		g.By("restarting the plugin")
		resumePlugin(ctx)
		err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
			events, err := readEvents(ctx, oc, pluginName)
			if err != nil {
				return false, nil
			}
			configured, seenPod := false, false
			for _, ev := range events {
				if ev.Kind == "configure" {
					configured = true
				}
				if ev.Kind == "synchronize-pod" && ev.Pod == namespace+"/"+pod.Name {
					seenPod = true
				}
			}
			return configured && seenPod, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "plugin did not re-register and synchronize the existing pod")
	})
})
