package jenkins

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

// JobMon is a Jenkins job monitor
type JobMon struct {
	j               *JenkinsRef
	lastBuildNumber string
	buildNumber     string
	jobName         string
}

const (
	EnableJenkinsMemoryStats = "ENABLE_JENKINS_MEMORY_MONITORING"
	EnableJenkinsGCStats     = "ENABLE_JENKINS_GC_MONITORING"
)

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

func StartJenkinsGCTracking(oc *exutil.CLI, jenkinsNamespace string) *time.Ticker {
	jenkinsPod := FindJenkinsPod(oc)
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for t := range ticker.C {
			stats, err := oc.Run("rsh").Args("--namespace", jenkinsNamespace, jenkinsPod.Name, "jstat", "-gcutil", "1").Output()
			if err == nil {
				fmt.Fprintf(g.GinkgoWriter, "\n\nJenkins gc stats %v\n%s\n\n", t, stats)
			} else {
				fmt.Fprintf(g.GinkgoWriter, "Unable to acquire Jenkins gc stats: %v", err)
			}
		}
	}()
	return ticker
}

func StartJenkinsMemoryTracking(oc *exutil.CLI, jenkinsNamespace string) *time.Ticker {
	jenkinsPod := FindJenkinsPod(oc)
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for t := range ticker.C {
			memstats, err := oc.Run("exec").Args("--namespace", jenkinsNamespace, jenkinsPod.Name, "--", "cat", "/sys/fs/cgroup/memory/memory.stat").Output()
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "\nUnable to acquire Jenkins cgroup memory.stat")
			}
			ps, err := oc.Run("exec").Args("--namespace", jenkinsNamespace, jenkinsPod.Name, "--", "ps", "faux").Output()
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "\nUnable to acquire Jenkins ps information")
			}
			fmt.Fprintf(g.GinkgoWriter, "\nJenkins memory statistics at %v\n%s\n%s\n\n", t, ps, memstats)
			gcstats, err := oc.Run("rsh").Args("--namespace", jenkinsNamespace, jenkinsPod.Name, "jstat", "-gcutil", "1").Output()
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "Unable to acquire Jenkins gc stats: %v", err)
			}
			fmt.Fprintf(g.GinkgoWriter, "\n\nJenkins gc stats %v\n%s\n\n", t, gcstats)

			// This is likely a temporary measure in place to extract diagnostic information during unexpectedly
			// high memory utilization within the Jenkins image. If Jenkins is using
			// a large amount of RSS, extract JVM information from the pod.
			if memoryOverragePattern.MatchString(memstats) {
				histogram, err := oc.Run("rsh").Args("--namespace", jenkinsNamespace, jenkinsPod.Name, "jmap", "-histo", "1").Output()
				if err == nil {
					fmt.Fprintf(g.GinkgoWriter, "\n\nJenkins histogram:\n%s\n\n", histogram)
				} else {
					fmt.Fprintf(g.GinkgoWriter, "Unable to acquire Jenkins histogram: %v", err)
				}
				stack, err := oc.Run("exec").Args("--namespace", jenkinsNamespace, jenkinsPod.Name, "--", "jstack", "1").Output()
				if err == nil {
					fmt.Fprintf(g.GinkgoWriter, "\n\nJenkins thread dump:\n%s\n\n", stack)
				} else {
					fmt.Fprintf(g.GinkgoWriter, "Unable to acquire Jenkins thread dump: %v", err)
				}
			}

		}
	}()
	return ticker
}
