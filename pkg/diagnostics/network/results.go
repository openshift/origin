package network

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/diagnostics/networkpod/util"
	"github.com/openshift/source-to-image/pkg/tar"
)

func (d *NetworkDiagnostic) CollectNetworkPodLogs() error {
	podList, err := d.getPodList(d.nsName1, util.NetworkDiagPodNamePrefix)
	if err != nil {
		return err
	}

	errList := []error{}
	for _, pod := range podList.Items {
		if err := d.getNetworkPodLogs(&pod); err != nil {
			errList = append(errList, err)
		}
	}
	return kerrs.NewAggregate(errList)
}

func (d *NetworkDiagnostic) CollectNetworkInfo(diagsFailed bool) error {
	if diagsFailed {
		// Collect useful info from master
		l := util.LogInterface{
			Result: d.res,
			Logdir: filepath.Join(d.LogDir, util.NetworkDiagMasterLogDirPrefix),
		}
		l.LogMaster()
	}

	podList, err := d.getPodList(d.nsName1, util.NetworkDiagPodNamePrefix)
	if err != nil {
		return err
	}

	errList := []error{}
	for _, pod := range podList.Items {
		if pod.Status.Phase != kapi.PodRunning {
			continue
		}

		if diagsFailed {
			if err := d.copyNetworkPodInfo(&pod); err != nil {
				errList = append(errList, err)
			}
		}

		if err := d.deleteRemoteNodeInfo(&pod); err != nil {
			errList = append(errList, err)
		}
	}
	return kerrs.NewAggregate(errList)
}

func (d *NetworkDiagnostic) copyNetworkPodInfo(pod *kapi.Pod) error {
	tmp, err := ioutil.TempFile("", "network-diags")
	if err != nil {
		return fmt.Errorf("Can not create local temporary file for tar: %v", err)
	}
	defer os.Remove(tmp.Name())

	// Tar logdir on the remote node and copy to a local temporary file
	errBuf := &bytes.Buffer{}
	nodeLogDir := filepath.Join(util.NetworkDiagDefaultLogDir, util.NetworkDiagNodeLogDirPrefix, pod.Spec.NodeName)
	cmd := []string{"chroot", util.NetworkDiagContainerMountPath, "tar", "-C", nodeLogDir, "-c", "."}
	if err = util.Execute(d.Factory, cmd, pod, nil, tmp, errBuf); err != nil {
		return fmt.Errorf("Creating remote tar locally failed: %v, %s", err, errBuf.String())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("Closing temporary tar file %s failed: %v", tmp.Name(), err)
	}

	// Extract copied temporary file locally
	tmp, err = os.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("Can not open temporary tar file %s: %v", tmp.Name(), err)
	}
	defer tmp.Close()

	tarHelper := tar.New()
	tarHelper.SetExclusionPattern(nil)
	logdir := filepath.Join(d.LogDir, util.NetworkDiagNodeLogDirPrefix, pod.Spec.NodeName)
	err = tarHelper.ExtractTarStream(logdir, tmp)
	if err != nil {
		return fmt.Errorf("Untar local directory failed: %v, %s", err, errBuf.String())
	}
	return nil
}

func (d *NetworkDiagnostic) deleteRemoteNodeInfo(pod *kapi.Pod) error {
	tmp, err := ioutil.TempFile("", "network-diags")
	if err != nil {
		return fmt.Errorf("Can not create local temporary file for tar: %v", err)
	}
	defer os.Remove(tmp.Name())

	errBuf := &bytes.Buffer{}
	nodeLogDir := filepath.Join(util.NetworkDiagDefaultLogDir, util.NetworkDiagNodeLogDirPrefix, pod.Spec.NodeName)
	cmd := []string{"chroot", util.NetworkDiagContainerMountPath, "sh", "-c", fmt.Sprintf("shopt -s dotglob && rm -rf %s", nodeLogDir)}
	if err = util.Execute(d.Factory, cmd, pod, nil, tmp, errBuf); err != nil {
		return fmt.Errorf("Deleting remote logdir %q on node %q failed: %v, %s", nodeLogDir, pod.Spec.NodeName, err, errBuf.String())
	}
	return nil
}

func (d *NetworkDiagnostic) getNetworkPodLogs(pod *kapi.Pod) error {
	bytelim := int64(1024000)
	opts := &kapi.PodLogOptions{
		TypeMeta:   pod.TypeMeta,
		Container:  pod.Name,
		Follow:     true,
		LimitBytes: &bytelim,
	}

	req, err := d.Factory.LogsForObject(pod, opts)
	if err != nil {
		return fmt.Errorf("Request for network diagnostic pod on node %q failed unexpectedly: %v", pod.Spec.NodeName, err)
	}
	readCloser, err := req.Stream()
	if err != nil {
		return fmt.Errorf("Logs for network diagnostic pod on node %q failed: %v", pod.Spec.NodeName, err)
	}
	defer readCloser.Close()

	scanner := bufio.NewScanner(readCloser)
	podLogs, nwarnings, nerrors := "", 0, 0
	errorRegex := regexp.MustCompile(`^\[Note\]\s+Errors\s+seen:\s+(\d+)`)
	warnRegex := regexp.MustCompile(`^\[Note\]\s+Warnings\s+seen:\s+(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()
		podLogs += line + "\n"
		if matches := errorRegex.FindStringSubmatch(line); matches != nil {
			nerrors, _ = strconv.Atoi(matches[1])
		} else if matches := warnRegex.FindStringSubmatch(line); matches != nil {
			nwarnings, _ = strconv.Atoi(matches[1])
		}
	}

	if err := scanner.Err(); err != nil { // Scan terminated abnormally
		return fmt.Errorf("Unexpected error reading network diagnostic pod on node %q: (%T) %[1]v\nLogs are:\n%[2]s", pod.Spec.NodeName, err, podLogs)
	} else {
		if nerrors > 0 {
			return fmt.Errorf("See the errors below in the output from the network diagnostic pod on node %q:\n%s", pod.Spec.NodeName, podLogs)
		} else if nwarnings > 0 {
			d.res.Warn("DNet4002", nil, fmt.Sprintf("See the warnings below in the output from the network diagnostic pod on node %q:\n%s", pod.Spec.NodeName, podLogs))
		} else {
			d.res.Info("DNet4003", fmt.Sprintf("Output from the network diagnostic pod on node %q:\n%s", pod.Spec.NodeName, podLogs))
		}
	}
	return nil
}
