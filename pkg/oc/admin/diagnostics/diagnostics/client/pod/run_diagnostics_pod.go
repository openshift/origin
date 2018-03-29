package pod

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	poddiag "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/client/pod/in_pod"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	DiagnosticPodName  = "DiagnosticPod"
	ImageTemplateParam = "images"
	LatestImageParam   = "latest-images"
)

// DiagnosticPod is a diagnostic that runs a diagnostic pod and relays the results.
type DiagnosticPod struct {
	KubeClient          kclientset.Interface
	Namespace           string
	Level               int
	Factory             *osclientcmd.Factory
	PreventModification bool
	ImageTemplate       variable.ImageTemplate
}

var _ types.ParameterizedDiagnostic = (*DiagnosticPod)(nil)

// Name is part of the Diagnostic interface and just returns name.
func (d *DiagnosticPod) Name() string {
	return DiagnosticPodName
}

// Description is part of the Diagnostic interface and provides a user-focused description of what the diagnostic does.
func (d *DiagnosticPod) Description() string {
	return "Create a pod to run diagnostics from the application standpoint"
}

func (d *DiagnosticPod) Requirements() (client bool, host bool) {
	return true, false
}

func (d *DiagnosticPod) AvailableParameters() []types.Parameter {
	return []types.Parameter{
		{ImageTemplateParam, "Image template to use in creating a pod", &d.ImageTemplate.Format, variable.NewDefaultImageTemplate().Format},
		{LatestImageParam, "If true, when expanding the image template, use latest version, not release version", &d.ImageTemplate.Latest, false},
	}
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d *DiagnosticPod) CanRun() (bool, error) {
	if d.PreventModification {
		return false, fmt.Errorf("running the diagnostic pod is an API change, which is prevented because the --prevent-modification flag was specified")
	}
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d *DiagnosticPod) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult("DiagnosticPod")
	d.runDiagnosticPod(r)
	return r
}

func (d *DiagnosticPod) runDiagnosticPod(r types.DiagnosticResult) {
	loglevel := d.Level
	if loglevel > 2 {
		loglevel = 2 // need to show summary at least
	}
	imageName := d.ImageTemplate.ExpandOrDie("deployer")
	pod, err := d.KubeClient.Core().Pods(d.Namespace).Create(&kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "pod-diagnostic-test-"},
		Spec: kapi.PodSpec{
			RestartPolicy: kapi.RestartPolicyNever,
			Containers: []kapi.Container{
				{
					Name:    "pod-diagnostics",
					Image:   imageName,
					Command: []string{"oc", "adm", "diagnostics", poddiag.InPodDiagnosticRecommendedName, "-l", strconv.Itoa(loglevel)},
				},
			},
		},
	})
	if err != nil {
		r.Error("DCli2001", err, fmt.Sprintf("Creating diagnostic pod with image %s failed. Error: (%[2]T) %[2]v", imageName, err))
		return
	}

	// Jump straight to clean up if there is an interrupt/terminate signal while running diagnostic
	done := make(chan bool, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		r.Warn("DCli2014", nil, "Interrupt received; aborting diagnostic.")
		done <- true
	}()
	go func() {
		d.processDiagnosticPodResults(pod, imageName, r)
		done <- true
	}()

	<-done
	signal.Stop(sig)
	// delete what we created, or notify that we couldn't
	zero := int64(0)
	delOpts := metav1.DeleteOptions{TypeMeta: pod.TypeMeta, GracePeriodSeconds: &zero}
	if err := d.KubeClient.Core().Pods(d.Namespace).Delete(pod.ObjectMeta.Name, &delOpts); err != nil {
		r.Error("DCl2002", err, fmt.Sprintf("Deleting diagnostic pod '%s' failed. Error: %s", pod.ObjectMeta.Name, fmt.Sprintf("(%T) %[1]s", err)))
	}
}

func (d *DiagnosticPod) processDiagnosticPodResults(protoPod *kapi.Pod, imageName string, r types.DiagnosticResult) {
	pod, err := d.KubeClient.Core().Pods(d.Namespace).Get(protoPod.ObjectMeta.Name, metav1.GetOptions{}) // status is filled in post-create
	if err != nil {
		r.Error("DCli2003", err, fmt.Sprintf("Retrieving the diagnostic pod definition failed. Error: (%T) %[1]v", err))
		return
	}
	r.Debug("DCli2004", fmt.Sprintf("Created diagnostic pod named %v running image %s.", pod.ObjectMeta.Name, imageName))

	bytelim := int64(1024000)
	podLogsOpts := &kapi.PodLogOptions{
		TypeMeta:   pod.TypeMeta,
		Container:  "pod-diagnostics",
		Follow:     true,
		LimitBytes: &bytelim,
	}
	req, err := d.Factory.LogsForObject(pod, podLogsOpts, 1*time.Minute)
	if err != nil {
		r.Error("DCli2005", err, fmt.Sprintf("The request for diagnostic pod logs failed unexpectedly. Error: (%T) %[1]v", err))
		return
	}

	// wait for pod to be started and logs available
	var scanner *bufio.Scanner
	var lastError error
	for times := 1; true; times++ {
		if times <= 50 {
			readCloser, err := req.Stream()
			if err != nil {
				lastError = err
				r.Debug("DCli2010", fmt.Sprintf("Could not get diagnostic pod logs (loop %d): (%T[2]) %[2]v", times, err))
				time.Sleep(time.Duration(times*100) * time.Millisecond)
				continue
			}
			defer readCloser.Close()
			// make sure we can actually get something from the stream before going on.
			// it seems the creation of docker logs can trail the container start a bit.
			lineScanner := bufio.NewScanner(readCloser)
			if lineScanner.Scan() {
				scanner = lineScanner
				break // success - drop down to reading the logs.
			}
			// no luck - try, try again
			lastError = fmt.Errorf("Diagnostics pod is ready but not its logs (loop %d). Retry.", times)
			r.Debug("DCli2010", lastError.Error())
			time.Sleep(time.Duration(times*100) * time.Millisecond)
			continue
		}
		// tries exhausted
		r.Warn("DCli2006", err, fmt.Sprintf("Timed out preparing diagnostic pod logs for streaming, so this diagnostic cannot run.\nIt is likely that the image '%s' was not pulled and running yet.\nLast error: (%T[2]) %[2]v", pod.Spec.Containers[0].Image, lastError))
		return
	}
	// then watch logs and wait until it exits
	podLogs, warnings, errors := "", 0, 0
	errorRegex := regexp.MustCompile(`^\[Note\]\s+Errors\s+seen:\s+(\d+)`)
	warnRegex := regexp.MustCompile(`^\[Note\]\s+Warnings\s+seen:\s+(\d+)`)
	// keep in mind one test line was already scanned, so scan after the loop runs once
	for scanned := true; scanned; scanned = scanner.Scan() {
		line := scanner.Text()
		podLogs += line + "\n"
		if matches := errorRegex.FindStringSubmatch(line); matches != nil {
			errors, _ = strconv.Atoi(matches[1])
		} else if matches := warnRegex.FindStringSubmatch(line); matches != nil {
			warnings, _ = strconv.Atoi(matches[1])
		}
	}
	if err := scanner.Err(); err != nil { // Scan terminated abnormally
		r.Error("DCli2009", err, fmt.Sprintf("Unexpected error reading diagnostic pod logs: (%T) %[1]v\nLogs are:\n%[2]s", err, podLogs))
	} else {
		if errors > 0 {
			r.Error("DCli2012", nil, "See the errors below in the output from the diagnostic pod:\n"+podLogs)
		} else if warnings > 0 {
			r.Warn("DCli2013", nil, "See the warnings below in the output from the diagnostic pod:\n"+podLogs)
		} else {
			r.Info("DCli2008", fmt.Sprintf("Output from the diagnostic pod (image %s):\n", imageName)+podLogs)
		}
	}
}
