package edge_topologies

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	etcdv1 "github.com/openshift/api/etcd/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	batchclient "k8s.io/client-go/kubernetes/typed/batch/v1"
	"k8s.io/client-go/rest"
	jobutil "k8s.io/kubernetes/pkg/controller/job/util"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

func testTNFUnresolvedCollectorRelease(oc *exutil.CLI, pcs *tnfPCSRunner, nodes []string) {
	ctx := context.Background()
	id := services.UntaintAlertID
	g.DeferCleanup(func() {
		o.Expect(waitForTNFAlertStatus(context.Background(), oc, nodes, time.Now())).To(o.Succeed())
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
	})
	backupTNFAlertRegistration(ctx, pcs, nodes[0], id)

	g.By("running a collector with a release absent from FeatureGate status")
	isolated, cancel := context.WithTimeout(ctx, tnfObservabilityTimeout)
	defer cancel()
	kube := oc.AdminKubeClient()
	cronJobs := kube.BatchV1().CronJobs(tnfCollectorNamespace)
	cron, err := cronJobs.Get(isolated, tnfCollectorName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	suffix := string(uuid.NewUUID())
	manualName := "tnf-observability-e2e-" + suffix
	policyName := "tnf-e2e-collector-" + suffix
	resume := func(ctx context.Context) error {
		return cleanupTNFCollectors(ctx, kube, cron.Namespace, manualName, policyName)
	}
	// Create responses can be lost, so register cleanup using client-generated names.
	g.DeferCleanup(func(ctx context.Context) error {
		return retryTNFOperation(ctx, time.Second, tnfCommandTimeout, resume)
	})
	policy := holdTNFScheduledCollectors(isolated, oc, cron, nodes, policyName)
	jobs := kube.BatchV1().Jobs(cron.Namespace)
	var baseline string
	o.Expect(retryTNFOperation(isolated, time.Second, tnfObservabilityTimeout, func(ctx context.Context) error {
		var err error
		baseline, err = tnfIdleCollectorVersion(ctx, kube, cron)
		return err
	})).To(o.Succeed())
	stream, err := jobs.Watch(isolated, metav1.ListOptions{ResourceVersion: baseline})
	o.Expect(err).NotTo(o.HaveOccurred())
	defer stream.Stop()
	isolated, stop := context.WithCancelCause(isolated)
	defer stop(nil)
	verified := make(chan error, 1)
	go func() {
		err := verifyTNFCollectorEvents(isolated, stream, tnfCollectorName, manualName)
		if err != nil && isolated.Err() == nil {
			g.GinkgoWriter.Printf("Collector isolation failed: %v\n", err)
			stop(err)
		}
		verified <- err
	}()

	before, err := apis.GetPacemakerCluster(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expectTNFAlertStatus(before, nodes)).To(o.Succeed())
	o.Expect(pcs.run(isolated, nodes[0], "alert", "delete", id)).To(o.Succeed())
	o.Expect(runTNFCollector(isolated, jobs, cron, policy, manualName, "0.0.0-tnf-e2e-unresolved")).To(o.Succeed())
	after, err := apis.GetPacemakerCluster(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(after.Status.LastUpdated.After(before.Status.LastUpdated.Time)).To(o.BeTrue(), "collector must publish fresh base status")
	o.Expect(apis.ExpectClusterHealthy(after)).To(o.Succeed())
	o.Expect(after.Status.Nodes).NotTo(o.BeNil())
	o.Expect(*after.Status.Nodes).To(o.HaveLen(len(nodes)))
	previousNodes := make(map[string]etcdv1.PacemakerClusterNodeStatus, len(nodes))
	for _, node := range *before.Status.Nodes {
		previousNodes[node.NodeName] = node
	}
	for _, node := range *after.Status.Nodes {
		o.Expect(apis.ExpectNodeCondition(after, node.NodeName, etcdv1.NodeHealthyConditionType, metav1.ConditionTrue)).To(o.Succeed())
		previous := previousNodes[node.NodeName]
		observed := meta.FindStatusCondition(node.Conditions, alertAgentsObserved)
		o.Expect(observed).NotTo(o.BeNil())
		o.Expect(observed.Status).To(o.Equal(metav1.ConditionFalse))
		o.Expect(observed.Reason).To(o.Equal(featureGateUnresolved))
		o.Expect(node.AlertAgents).To(o.Equal(previous.AlertAgents))
		o.Expect(meta.FindStatusCondition(node.Conditions, etcdv1.NodeAlertAgentsHealthyConditionType)).
			To(o.Equal(meta.FindStatusCondition(previous.Conditions, etcdv1.NodeAlertAgentsHealthyConditionType)))
	}
	expectTNFAlertUnregistered(isolated, pcs, nodes[0], id)
	current, err := cronJobs.Get(isolated, tnfCollectorName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(current.UID).To(o.Equal(cron.UID))
	// Deletion after the assertions provides an ordered end marker on the same Job watch.
	o.Expect(deleteTNFCollector(isolated, jobs, manualName)).To(o.Succeed())
	o.Expect(<-verified).To(o.Succeed())
	o.Expect(resume(isolated)).To(o.Succeed())

	g.By("letting the scheduled collector resolve its own release and repair the fault")
	o.Expect(waitForTNFAlertStatus(ctx, oc, nodes, after.Status.LastUpdated.Time)).To(o.Succeed())
	output, err := pcs.output(ctx, nodes[0], "alert", "config", "--output-format", "json")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expectTNFAlertRegistrations(output)).To(o.Succeed())
	o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
}

const tnfCollectorHoldMessage = "scheduled collector held for unresolved-release verification"

// The operator reconciles the CronJob spec. Hold only its scheduled Job creates
// so the one-off collector can finish independently of the minute boundary.
func holdTNFScheduledCollectors(ctx context.Context, oc *exutil.CLI, cron *batchv1.CronJob, nodes []string, policyName string) *admissionv1.ValidatingAdmissionPolicy {
	kube := oc.AdminKubeClient()
	admission := kube.AdmissionregistrationV1()
	policies := admission.ValidatingAdmissionPolicies()
	bindings := admission.ValidatingAdmissionPolicyBindings()
	deadline, _ := ctx.Deadline()
	expires := deadline.Add(tnfCommandTimeout)
	policy, err := policies.Create(ctx, &admissionv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: admissionv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: &admissionv1.MatchResources{
				ResourceRules: []admissionv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionv1.RuleWithOperations{
						Operations: []admissionv1.OperationType{admissionv1.Create},
						Rule: admissionv1.Rule{
							APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"jobs"},
						},
					},
				}},
			},
			MatchConditions: []admissionv1.MatchCondition{{
				Name: "scheduled-collector",
				Expression: fmt.Sprintf(`request.namespace == %q && has(object.metadata.ownerReferences) &&
					object.metadata.ownerReferences.exists(owner, owner.apiVersion == "batch/v1" && owner.kind == "CronJob" && owner.name == %q && has(owner.controller) && owner.controller)`,
					tnfCollectorNamespace, tnfCollectorName),
			}},
			// CronJob supplies the scheduled timestamp. Let later runs through even if the test process dies before cleanup.
			Validations: []admissionv1.Validation{{
				Expression: fmt.Sprintf(`has(object.metadata.annotations) && %[1]q in object.metadata.annotations &&
					timestamp(string(object.metadata.annotations[%[1]q])) >= timestamp(%[2]q)`,
					batchv1.CronJobScheduledTimestampAnnotation, expires.UTC().Format(time.RFC3339)),
				Message: tnfCollectorHoldMessage,
			}},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = bindings.Create(ctx, &admissionv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: policyName, ValidationActions: []admissionv1.ValidationAction{admissionv1.Deny},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	probe := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		GenerateName:    "tnf-probe-",
		OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(cron, batchv1.SchemeGroupVersion.WithKind("CronJob"))},
		Annotations:     map[string]string{batchv1.CronJobScheduledTimestampAnnotation: time.Now().UTC().Format(time.RFC3339)},
	}, Spec: cron.Spec.JobTemplate.Spec}
	// The load balancer may route consecutive creates to different admission caches.
	replicas := make(map[string]batchclient.JobInterface, len(nodes))
	for _, node := range nodes {
		cfg := oc.AdminConfig()
		dialer := e2epod.NewDialer(kube, rest.CopyConfig(cfg))
		cfg.Dial = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContainerPort(ctx, e2epod.Addr{Namespace: "openshift-kube-apiserver", PodName: "kube-apiserver-" + node, Port: 6443})
		}
		cfg.Proxy = func(*http.Request) (*url.URL, error) { return nil, nil }
		client, err := rest.HTTPClientFor(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer utilnet.CloseIdleConnectionsFor(client.Transport)
		batch, err := batchclient.NewForConfigAndClient(cfg, client)
		o.Expect(err).NotTo(o.HaveOccurred())
		replicas[node] = batch.Jobs(cron.Namespace)
	}
	o.Expect(retryTNFOperation(ctx, time.Second, tnfCommandTimeout, func(ctx context.Context) error {
		return checkTNFCollectorAdmission(ctx, replicas, probe, policyName)
	})).To(o.Succeed())
	return policy
}

func checkTNFCollectorAdmission(ctx context.Context, replicas map[string]batchclient.JobInterface, probe *batchv1.Job, policyName string) error {
	for node, jobs := range replicas {
		_, err := jobs.Create(ctx, probe, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
		if !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), policyName) || !strings.Contains(err.Error(), tnfCollectorHoldMessage) {
			return fmt.Errorf("waiting for policy %s on %s to deny the probe: %v", policyName, node, err)
		}
	}
	return nil
}

func tnfIdleCollectorVersion(ctx context.Context, kube kubernetes.Interface, cron *batchv1.CronJob) (string, error) {
	// Watch from the Job list, including changes made while checking Pods.
	list, err := kube.BatchV1().Jobs(cron.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for i := range list.Items {
		job := &list.Items[i]
		if tnfScheduledJob(job, cron.Name) && !jobutil.IsJobFinished(job) {
			return "", fmt.Errorf("waiting for collector Job %s to finish", job.Name)
		}
	}
	pods, err := kube.CoreV1().Pods(cron.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=" + cron.Name})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			return "", fmt.Errorf("waiting for collector Pod %s to finish", pod.Name)
		}
	}
	return list.ResourceVersion, nil
}

func tnfScheduledJob(job *batchv1.Job, cronName string) bool {
	owner := metav1.GetControllerOfNoCopy(job)
	return job.Labels["app.kubernetes.io/name"] == cronName ||
		(owner != nil && owner.APIVersion == "batch/v1" && owner.Kind == "CronJob" && owner.Name == cronName)
}

// The manual Job is deleted after the last assertion. Its deletion event proves
// the watch has delivered every earlier scheduled Job creation, even if the Job
// was already gone by the time the assertion finished.
func verifyTNFCollectorEvents(ctx context.Context, stream watch.Interface, cronName, manualName string) error {
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case event, ok := <-stream.ResultChan():
			if !ok {
				return fmt.Errorf("collector Job watch closed before manual Job deletion")
			}
			if event.Type == watch.Error {
				return apierrors.FromObject(event.Object)
			}
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				return fmt.Errorf("unexpected collector Job watch object %T", event.Object)
			}
			if event.Type == watch.Added && tnfScheduledJob(job, cronName) {
				return fmt.Errorf("scheduled collector %s started during verification", job.Name)
			}
			if event.Type == watch.Deleted && job.Name == manualName {
				return nil
			}
		}
	}
}

func runTNFCollector(ctx context.Context, jobs batchclient.JobInterface, cron *batchv1.CronJob, policy *admissionv1.ValidatingAdmissionPolicy, name, release string) error {
	ctx, cancel := context.WithTimeout(ctx, tnfCommandTimeout)
	defer cancel()
	spec := cron.Spec.JobTemplate.Spec.DeepCopy()
	env := spec.Template.Spec.Containers[0].Env
	found := false
	for i := range env {
		if env[i].Name == tnfCollectorVersionEnv {
			env[i] = corev1.EnvVar{Name: tnfCollectorVersionEnv, Value: release}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("collector template is missing %s", tnfCollectorVersionEnv)
	}

	// A CREATE can reach storage after its caller times out. Such a Job must
	// stay inert and be garbage-collected when cleanup removes the hold.
	spec.Suspend = ptr.To(true)
	_, err := jobs.Create(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, OwnerReferences: []metav1.OwnerReference{{
			APIVersion: admissionv1.SchemeGroupVersion.String(), Kind: "ValidatingAdmissionPolicy", Name: policy.Name, UID: policy.UID,
		}}},
		Spec: *spec,
	}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create suspended collector %s: %w", name, err)
	}
	_, err = jobs.Patch(ctx, name, types.MergePatchType, []byte(`{"spec":{"suspend":false}}`), metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("start collector %s: %w", name, err)
	}
	return waitForTNFManualJob(ctx, jobs, name)
}

func waitForTNFManualJob(ctx context.Context, jobs batchclient.JobInterface, name string) error {
	var lastStatusGetErr error
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		current, err := jobs.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) {
				return false, err
			}
			if !apierrors.IsTimeout(err) && !apierrors.IsServerTimeout(err) && !utils.IsTransientKubernetesAPIError(err) {
				return false, err
			}
			if ctx.Err() == nil {
				lastStatusGetErr = err
			}
			return false, nil
		}
		for _, condition := range current.Status.Conditions {
			if condition.Status != corev1.ConditionTrue {
				continue
			}
			switch condition.Type {
			case batchv1.JobComplete:
				return true, nil
			case batchv1.JobFailed:
				return false, fmt.Errorf("collector Job %s failed: %s", name, condition.Message)
			}
		}
		return false, nil
	})
	if wait.Interrupted(err) && lastStatusGetErr != nil {
		err = fmt.Errorf("%w (last Job status lookup error: %w)", err, lastStatusGetErr)
	}
	return err
}

func deleteTNFCollector(ctx context.Context, jobs batchclient.JobInterface, name string) error {
	policy := metav1.DeletePropagationForeground
	err := jobs.Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil {
		return err
	}
	var lastGetErr error
	err = wait.PollUntilContextTimeout(ctx, time.Second, tnfCommandTimeout, true, func(ctx context.Context) (bool, error) {
		_, err := jobs.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			lastGetErr = err
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("wait for collector Job %s to be deleted: %w", name, errors.Join(err, lastGetErr))
	}
	return nil
}

// A failed manual run can still have a running Pod. Keep the admission hold
// until foreground deletion has removed the Job and its dependents.
func cleanupTNFCollectors(ctx context.Context, kube kubernetes.Interface, namespace, manualName, policyName string) error {
	err := deleteTNFCollector(ctx, kube.BatchV1().Jobs(namespace), manualName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("remove manual collector %s before releasing admission hold: %w", manualName, err)
	}
	admission := kube.AdmissionregistrationV1()
	err = admission.ValidatingAdmissionPolicyBindings().Delete(ctx, policyName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	err = admission.ValidatingAdmissionPolicies().Delete(ctx, policyName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
