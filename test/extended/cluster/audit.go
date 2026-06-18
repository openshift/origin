package cluster

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

var _ = g.Describe("[sig-api-machinery][Feature:Audit] Basic audit", func() {
	f := framework.NewDefaultFramework("audit")

	g.It("should audit API calls", func() {
		namespace := f.Namespace.Name

		// Create & Delete pod
		pod := &apiv1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "audit-pod",
			},
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{{
					Name:  "pause",
					Image: imageutils.GetPauseImageName(),
				}},
			},
		}
		ctx := context.Background()
		e2epod.NewPodClient(f).CreateSync(ctx, pod)
		e2epod.NewPodClient(f).DeleteSync(ctx, pod.Name, metav1.DeleteOptions{}, e2epod.DefaultPodDeletionTimeout)

		// Create, Read, Delete secret
		secret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "audit-secret",
			},
			Data: map[string][]byte{
				"top-secret": []byte("foo-bar"),
			},
		}
		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(ctx, secret, metav1.CreateOptions{})
		framework.ExpectNoError(err, "failed to create audit-secret")
		_, err = f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Get(ctx, secret.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get audit-secret")
		err = f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Delete(ctx, secret.Name, metav1.DeleteOptions{})
		framework.ExpectNoError(err, "failed to delete audit-secret")

		// /version should not be audited
		_, err = f.ClientSet.CoreV1().RESTClient().Get().AbsPath("/version").DoRaw(ctx)
		framework.ExpectNoError(err, "failed to query version")

		expectedEvents := []auditEvent{{
			method:    "create",
			namespace: namespace,
			uri:       fmt.Sprintf("/api/v1/namespaces/%s/pods", namespace),
			response:  "201",
		}, {
			method:    "delete",
			namespace: namespace,
			uri:       fmt.Sprintf("/api/v1/namespaces/%s/pods/%s", namespace, pod.Name),
			response:  "200",
		}, {
			method:    "create",
			namespace: namespace,
			uri:       fmt.Sprintf("/api/v1/namespaces/%s/secrets", namespace),
			response:  "201",
		}, {
			method:    "get",
			namespace: namespace,
			uri:       fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, secret.Name),
			response:  "200",
		}, {
			method:    "delete",
			namespace: namespace,
			uri:       fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, secret.Name),
			response:  "200",
		}}
		expectAuditLines(f, expectedEvents)
	})
})

type auditEvent struct {
	method, namespace, uri, response string
}

// Search the audit log for the expected audit lines.
func expectAuditLines(f *framework.Framework, expected []auditEvent) {
	expectations := map[auditEvent]bool{}
	for _, event := range expected {
		expectations[event] = false
	}

	stream, err := os.Open(filepath.Join(os.Getenv("LOG_DIR"), "audit.log"))
	defer stream.Close()
	framework.ExpectNoError(err, "error opening audit log")
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		event, err := parseAuditLine(line)
		framework.ExpectNoError(err)

		// If the event was expected, mark it as found.
		if _, found := expectations[event]; found {
			expectations[event] = true
		}
	}
	framework.ExpectNoError(scanner.Err(), "error reading audit log")

	for event, found := range expectations {
		o.Expect(found).To(o.BeTrue(), "Event %#v not found!", event)
	}
}

func parseAuditLine(line string) (auditEvent, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return auditEvent{}, fmt.Errorf("could not parse audit line: %s", line)
	}
	// Ignore first field (timestamp)
	if fields[1] != "AUDIT:" {
		return auditEvent{}, fmt.Errorf("unexpected audit line format: %s", line)
	}
	fields = fields[2:]
	event := auditEvent{}
	for _, f := range fields {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			return auditEvent{}, fmt.Errorf("could not parse audit line (part: %q): %s", f, line)
		}
		value := strings.Trim(parts[1], "\"")
		switch parts[0] {
		case "method":
			event.method = value
		case "namespace":
			event.namespace = value
		case "uri":
			event.uri = value
		case "response":
			event.response = value
		}
	}
	return event, nil
}

// validAuditProfiles is the set of profile names accepted by the APIServer CR.
var validAuditProfiles = map[configv1.AuditProfileType]struct{}{
	configv1.DefaultAuditProfileType:            {},
	configv1.WriteRequestBodiesAuditProfileType: {},
	configv1.AllRequestBodiesAuditProfileType:   {},
	configv1.NoneAuditProfileType:               {},
	configv1.AuditProfileType(""):               {}, // unset is treated as Default
}

var _ = g.Describe("[sig-api-machinery] [Jira:apiserver-auth] Audit", func() {
	oc := exutil.NewCLIWithoutNamespace("apiserver-audit")

	// Read current audit profile and assert it is one of the four known valid values.
	g.It("[OTP] should report the current audit profile as a known valid value [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(
				ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, ok := validAuditProfiles[apiServer.Spec.Audit.Profile]
			o.Expect(ok).To(o.BeTrue(),
				"unexpected audit profile %q, expected one of Default, WriteRequestBodies, AllRequestBodies, None",
				apiServer.Spec.Audit.Profile)
		})

	// API server must reject an unrecognised audit profile name at admission time.
	g.It("[OTP] should reject an invalid audit profile name in the APIServer configuration [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver", "cluster",
				"--type=merge",
				`--patch={"spec":{"audit":{"profile":"InvalidAuditProfile"}}}`,
				"--dry-run=server",
			).Output()
			o.Expect(err).To(o.HaveOccurred(),
				"expected server-side validation to reject an invalid audit profile name")
		})

	// Custom audit rule must specify a non-empty group name; an empty value should be
	// rejected at admission time without altering the live configuration.
	g.It("[OTP] should reject a custom audit rule with an empty group name [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver", "cluster",
				"--type=merge",
				`--patch={"spec":{"audit":{"customRules":[{"group":"","profile":"Default"}]}}}`,
				"--dry-run=server",
			).Output()
			o.Expect(err).To(o.HaveOccurred(),
				"expected server-side validation to reject a custom audit rule with an empty group name")
		})

	// Custom audit rule that references an unknown profile name must be rejected at
	// admission time without altering the live configuration.
	g.It("[OTP] should reject a custom audit rule with an invalid profile name [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
				"apiserver", "cluster",
				"--type=merge",
				`--patch={"spec":{"audit":{"customRules":[{"group":"system:authenticated","profile":"BadProfile"}]}}}`,
				"--dry-run=server",
			).Output()
			o.Expect(err).To(o.HaveOccurred(),
				"expected server-side validation to reject a custom audit rule with an invalid profile name")
		})

	// Every documented profile name (Default, WriteRequestBodies, AllRequestBodies, None)
	// must be accepted by the API server.  A server-side dry-run is used so no rollout is triggered.
	g.It("[OTP] should accept all valid audit profile names via server-side dry-run [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			for _, profile := range []configv1.AuditProfileType{
				configv1.DefaultAuditProfileType,
				configv1.WriteRequestBodiesAuditProfileType,
				configv1.AllRequestBodiesAuditProfileType,
				configv1.NoneAuditProfileType,
			} {
				g.By(fmt.Sprintf("verifying profile %q is accepted by the API server", profile))
				patch := fmt.Sprintf(`{"spec":{"audit":{"profile":%q}}}`, profile)
				_, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args(
					"apiserver", "cluster",
					"--type=merge",
					"--patch="+patch,
					"--dry-run=server",
				).Output()
				o.Expect(err).NotTo(o.HaveOccurred(),
					"expected profile %q to be accepted by the API server", profile)
			}
		})

	// Audit log files must be present under /var/log/kube-apiserver/ on every master node.
	// Skipped on HyperShift where the control plane is hosted externally and master nodes are not
	// directly accessible through node-logs.
	g.It("[OTP] should have audit log files present on master nodes [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
				g.Skip("HyperShift hosts the control plane externally; master node logs are not accessible via node-logs")
			}

			masters, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(
				ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(masters.Items).NotTo(o.BeEmpty(), "expected at least one master node")

			for _, master := range masters.Items {
				g.By(fmt.Sprintf("checking for audit log files on master node %s", master.Name))
				output, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args(
					"node-logs", master.Name, "--path=kube-apiserver/",
				).Output()
				o.Expect(err).NotTo(o.HaveOccurred(),
					"failed to list kube-apiserver log files on master node %s", master.Name)
				o.Expect(output).To(o.MatchRegexp(`audit`),
					"expected audit log files to be present on master node %s", master.Name)
			}
		})

	// Each line in the kube-apiserver audit log must be valid JSON and contain the
	// mandatory fields defined by the audit.k8s.io/v1 event schema
	// (kind, apiVersion, level, requestURI, verb, user, stage).
	// Skipped on HyperShift where the control plane is hosted externally and master nodes are not
	// directly accessible through node-logs.
	g.It("[OTP] should write audit log entries in valid JSON format with required fields [apigroup:config.openshift.io]",
		ote.Informing(), func(ctx g.SpecContext) {
			if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
				g.Skip("HyperShift hosts the control plane externally; master node logs are not accessible via node-logs")
			}

			masters, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(
				ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(masters.Items).NotTo(o.BeEmpty(), "expected at least one master node")

			// Inspect the most recent audit log entries from the first master only to
			// keep the test fast while still validating the format is correct.
			master := masters.Items[0]
			g.By(fmt.Sprintf("reading recent audit log entries from master node %s", master.Name))
			output, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args(
				"node-logs", master.Name, "--path=kube-apiserver/audit.log", "--tail=20",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred(),
				"failed to read kube-apiserver audit log from master node %s", master.Name)

			checkedLines := 0
			for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				var entry map[string]interface{}
				o.Expect(json.Unmarshal([]byte(line), &entry)).NotTo(o.HaveOccurred(),
					"audit log entry is not valid JSON on node %s: %s", master.Name, line)

				for _, field := range []string{"kind", "apiVersion", "level", "requestURI", "verb", "user", "stage"} {
					o.Expect(entry).To(o.HaveKey(field),
						"audit log entry on node %s is missing required field %q: %s", master.Name, field, line)
				}
				checkedLines++
			}
			o.Expect(checkedLines).To(o.BeNumerically(">", 0),
				"expected at least one non-empty audit log line on node %s", master.Name)
		})
})
