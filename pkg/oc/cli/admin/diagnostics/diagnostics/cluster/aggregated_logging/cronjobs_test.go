package aggregated_logging

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapibatch "k8s.io/kubernetes/pkg/apis/batch"

	"github.com/openshift/origin/pkg/oc/cli/admin/diagnostics/diagnostics/log"
)

const (
	testCronKey = "cronjobs"
)

type fakeCronJobDiagnostic struct {
	fakeDiagnostic
	fakeCronJobs kapibatch.CronJobList
	clienterrors map[string]error
}

func newFakeCronJobDiagnostic(t *testing.T) *fakeCronJobDiagnostic {
	return &fakeCronJobDiagnostic{
		fakeDiagnostic: *newFakeDiagnostic(t),
		clienterrors:   map[string]error{},
	}
}

func (f *fakeCronJobDiagnostic) addCronJobWithLabel(key string, value string) {
	cron := kapibatch.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				key: value,
			},
		},
	}
	f.fakeCronJobs.Items = append(f.fakeCronJobs.Items, cron)
}

func (f *fakeCronJobDiagnostic) cronjobs(project string, options metav1.ListOptions) (*kapibatch.CronJobList, error) {
	value, ok := f.clienterrors[testDsKey]
	if ok {
		return nil, value
	}
	return &f.fakeCronJobs, nil
}

func TestCheckCronJobsWhenErrorResponseFromClientRetrievingList(t *testing.T) {
	d := newFakeCronJobDiagnostic(t)
	d.clienterrors[testDsKey] = errors.New("someerror")

	checkCronJobs(d, d, fakeProject)

	d.assertMessage("AGL0805", "Exp. error when client errors on retrieving Cronjobs", log.ErrorLevel)
}

func TestCheckCronjobsWhenNoneFound(t *testing.T) {
	d := newFakeCronJobDiagnostic(t)

	checkCronJobs(d, d, fakeProject)

	d.assertMessage("AGL0807", "Exp. error when client retrieves no Cronjobs", log.ErrorLevel)
}

func TestCheckCronjobsWhenNoneMatched(t *testing.T) {
	d := newFakeCronJobDiagnostic(t)
	d.addCronJobWithLabel("foo", "bar")

	checkCronJobs(d, d, fakeProject)

	d.assertMessage("AGL0865", "Exp. error when cronjobs do not match any cronjobs", log.ErrorLevel)
}
