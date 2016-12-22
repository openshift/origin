package jenkins

import (
	"strings"
	"time"

	o "github.com/onsi/gomega"

	"k8s.io/kubernetes/pkg/util/wait"
)

// JobMon is a Jenkins job monitor
type JobMon struct {
	j               *JenkinsRef
	lastBuildNumber string
	buildNumber     string
	jobName         string
}

// Await waits for the timestamp on the Jenkins job to change. Returns
// and error if the timeout expires.
func (jmon *JobMon) Await(timeout time.Duration) error {
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {

		buildNumber, err := jmon.j.GetJobBuildNumber(jmon.jobName, time.Minute)
		o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

		ginkgolog("Checking build number for job %q current[%v] vs last[%v]", jmon.jobName, buildNumber, jmon.lastBuildNumber)
		if buildNumber == jmon.lastBuildNumber {
			return false, nil
		}

		if jmon.buildNumber == "" {
			jmon.buildNumber = buildNumber
		}
		body, status, err := jmon.j.GetResource("job/%s/%s/api/json?depth=1", jmon.jobName, jmon.buildNumber)
		o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
		o.ExpectWithOffset(1, status).To(o.Equal(200))

		body = strings.ToLower(body)
		if strings.Contains(body, "\"building\":true") {
			ginkgolog("Jenkins job %q still building:\n%s\n\n", jmon.jobName, body)
			return false, nil
		}

		if strings.Contains(body, "\"result\":null") {
			ginkgolog("Jenkins job %q still building result:\n%s\n\n", jmon.jobName, body)
			return false, nil
		}

		ginkgolog("Jenkins job %q build complete:\n%s\n\n", jmon.jobName, body)
		return true, nil
	})
	return err
}
