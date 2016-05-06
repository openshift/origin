package clusterquota

import (
	"io"
	"strings"
	"sync"
	"time"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/install"

	oclient "github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

func init() {
	admission.RegisterPlugin("ClusterResourceQuota",
		func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
			registry := install.NewRegistry(client)
			return NewResourceQuota(client, registry, 5)
		})
}

// quotaAdmission implements an admission controller that can enforce quota constraints
type quotaAdmission struct {
	*admission.Handler

	evaluator *quotaEvaluator

	startup            sync.Once
	clusterQuotaClient oclient.Interface
	registry           quota.Registry
	numEvaluators      int
}

var _ = oadmission.WantsOpenshiftClient(&quotaAdmission{})

type liveLookupEntry struct {
	expiry time.Time
	items  []*api.ResourceQuota
}

// NewResourceQuota configures an admission controller that can enforce quota constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func NewResourceQuota(client clientset.Interface, registry quota.Registry, numEvaluators int) (admission.Interface, error) {
	return &quotaAdmission{
		Handler:       admission.NewHandler(admission.Create, admission.Update),
		registry:      registry,
		numEvaluators: numEvaluators,
	}, nil
}

// Admit makes admission decisions while enforcing quota
func (q *quotaAdmission) Admit(a admission.Attributes) (err error) {
	q.startup.Do(func() {
		evaluator, err := newQuotaEvaluator(q.clusterQuotaClient, q.registry)
		if err != nil {
			// TODO always fail somehow if this fails
			panic(err)
		}
		evaluator.Run(q.numEvaluators)

		q.evaluator = evaluator
	})

	// ignore all operations that correspond to sub-resource actions
	if a.GetSubresource() != "" {
		return nil
	}

	// if we do not know how to evaluate use for this kind, just ignore
	evaluators := q.evaluator.registry.Evaluators()
	evaluator, found := evaluators[a.GetKind().GroupKind()]
	if !found {
		return nil
	}

	// for this kind, check if the operation could mutate any quota resources
	// if no resources tracked by quota are impacted, then just return
	op := a.GetOperation()
	operationResources := evaluator.OperationResources(op)
	if len(operationResources) == 0 {
		return nil
	}

	return q.evaluator.evaluate(a)
}

// prettyPrint formats a resource list for usage in errors
func prettyPrint(item api.ResourceList) string {
	parts := []string{}
	for key, value := range item {
		constraint := string(key) + "=" + value.String()
		parts = append(parts, constraint)
	}
	return strings.Join(parts, ",")
}

// hasUsageStats returns true if for each hard constraint there is a value for its current usage
func hasUsageStats(resourceQuota *api.ResourceQuota) bool {
	for resourceName := range resourceQuota.Status.Hard {
		if _, found := resourceQuota.Status.Used[resourceName]; !found {
			return false
		}
	}
	return true
}

func (a *quotaAdmission) SetOpenshiftClient(c oclient.Interface) {
	a.clusterQuotaClient = c
}
