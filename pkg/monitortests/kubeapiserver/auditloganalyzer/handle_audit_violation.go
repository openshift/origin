package auditloganalyzer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func CheckForViolations() *auditViolations {
	return &auditViolations{}
}

type auditViolations struct {
	lock    sync.Mutex
	records []auditViolationRecord
}

type auditViolationRecord struct {
	auditId   string
	violation string
	resource  string
	namespace string
	name      string
	username  string
}

func (v *auditViolations) HandleAuditLogEvent(auditEvent *auditv1.Event, beginning, end *metav1.MicroTime, nodeName string) {
	if beginning != nil && auditEvent.RequestReceivedTimestamp.Before(beginning) || end != nil && end.Before(&auditEvent.RequestReceivedTimestamp) {
		return
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	if violation, ok := auditEvent.Annotations["pod-security.kubernetes.io/audit-violations"]; ok {
		v.records = append(v.records, auditViolationRecord{
			auditId:   string(auditEvent.AuditID),
			violation: violation,
			resource:  auditEvent.ObjectRef.Resource,
			namespace: auditEvent.ObjectRef.Namespace,
			name:      auditEvent.ObjectRef.Name,
			username:  auditEvent.User.Username,
		})
	}
}

func (v *auditViolations) CreateJunits(psaEnforcementDisabled bool) []*junitapi.JUnitTestCase {
	ret := []*junitapi.JUnitTestCase{}

	testName := " [bz-apiserver-auth][invariant] audit analysis PodSecurityViolation"

	// When the OpenShiftPodSecurityAdmission feature gate is disabled, the PSA label syncer stops
	// setting the pod-security.kubernetes.io/enforce label on namespaces and workloads that key
	// their security context off that label (e.g. OLM catalog registry pods) legitimately run
	// without a restricted-compatible security context. The global PodSecurity audit configuration
	// stays at restricted:latest, so those pod creations produce audit violations by design and
	// this invariant does not apply.
	if psaEnforcementDisabled {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				SkipMessage: &junitapi.SkipMessage{
					Message: fmt.Sprintf("OpenShiftPodSecurityAdmission is disabled: pod security is not enforced and audit-level violations are expected, %d violation(s) observed, details in audit log", len(v.records)),
				},
			},
		}
	}

	switch {
	case len(v.records) > 0:
		messages := []string{}
		for _, v := range v.records {
			messages = append(messages, fmt.Sprintf("%s: %s %s/%s: %s - %s", v.auditId, v.resource, v.namespace, v.name, v.username, v.violation))
		}
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("%s\ndetails from audit log", strings.Join(messages, "\n")),
				},
			},
		)
	default:
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	return ret
}

// isPodSecurityEnforcementDisabled reports whether the OpenShiftPodSecurityAdmission feature gate
// is explicitly disabled for the cluster's current version. On any error or ambiguity it returns
// false so that the PodSecurityViolation invariant keeps enforcing.
func isPodSecurityEnforcementDisabled(ctx context.Context, configClient configclient.Interface, clusterVersion *configv1.ClusterVersion) bool {
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false
	}

	desiredVersion := clusterVersion.Status.Desired.Version
	if len(desiredVersion) == 0 && len(clusterVersion.Status.History) > 0 {
		desiredVersion = clusterVersion.Status.History[0].Version
	}

	for _, featureGateValues := range featureGate.Status.FeatureGates {
		if featureGateValues.Version != desiredVersion {
			continue
		}
		for _, disabled := range featureGateValues.Disabled {
			if disabled.Name == features.FeatureGateOpenShiftPodSecurityAdmission {
				return true
			}
		}
		return false
	}

	return false
}
