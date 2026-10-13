package jenkins

import (
	"regexp"
	"strings"
	"time"

	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// JobMon is a Jenkins job monitor
type JobMon struct {
	j               *JenkinsRef
	lastBuildNumber string
	buildNumber     string
	jobName         string
}

// Designed to match if RSS memory is greater than 500000000  (i.e. > 476MB)
var memoryOverragePattern = regexp.MustCompile(`\s+rss\s+5\d\d\d\d\d\d\d\d`)

// Await waits for the timestamp on the Jenkins job to change. Returns
// and error if the timeout expires.
func (jmon *JobMon) Await(timeout time.Duration) error {
	err := wait.Poll(10*time.Second, timeout, func() (bool, error) {

		buildNumber, err := jmon.j.GetJobBuildNumber(jmon.jobName, time.Minute)
		o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

		e2e.Logf("Checking build number for job %q current[%v] vs last[%v]", jmon.jobName, buildNumber, jmon.lastBuildNumber)
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
			e2e.Logf("Jenkins job %q still building:\n%s\n\n", jmon.jobName, body)
			return false, nil
		}

		if strings.Contains(body, "\"result\":null") {
			e2e.Logf("Jenkins job %q still building result:\n%s\n\n", jmon.jobName, body)
			return false, nil
		}

		e2e.Logf("Jenkins job %q build complete:\n%s\n\n", jmon.jobName, body)
		// If Jenkins job has completed, output its log
		body, status, err = jmon.j.GetResource("job/%s/%s/consoleText", jmon.jobName, jmon.buildNumber)
		if err != nil || status != 200 {
			e2e.Logf("Unable to retrieve job log from Jenkins.\nStatus code: %d\nError: %v\nResponse Text: %s\n", status, err, body)
			return true, nil
		}
		e2e.Logf("Jenkins job %q log:\n%s\n\n", jmon.jobName, body)
		return true, nil
	})
	return err
}
