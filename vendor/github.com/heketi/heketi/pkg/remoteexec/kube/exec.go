//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kube

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	rex "github.com/heketi/heketi/pkg/remoteexec"
	rexlog "github.com/heketi/heketi/pkg/remoteexec/log"
)

type TimeoutOptions struct {
	TimeoutMinutes   int
	UseTimeoutPrefix bool
}

// ExecCommands executes the given array of commands on the given
// target container using the given connection. The results type
// will contain both the success and failure results of the
// indvidual commands if run. Commands are only run if the previous
// command was successful. Any connection level error conditions
// are returned as the function's error condition.
// Using sudo is not supported by kubeexec.
func ExecCommands(
	k *KubeConn, t TargetContainer,
	commands rex.Cmds, topts TimeoutOptions) (rex.Results, error) {

	results := make(rex.Results, len(commands))
	cmdlog := rexlog.NewCommandLogger(k.logger)

	for index, cmd := range commands {
		cmdlog.Before(cmd, t.String())
		command := cmd.String()

		// Remove any whitespace
		command = strings.Trim(command, " ")

		// Create a buffer to trap session output
		var (
			b    bytes.Buffer
			berr bytes.Buffer
			cmdv []string
		)
		if topts.TimeoutMinutes > 0 && topts.UseTimeoutPrefix {
			cmdv = []string{
				"timeout",
				fmt.Sprintf("%vm", topts.TimeoutMinutes),
				"bash",
				"-c",
				command,
			}
		} else {
			cmdv = []string{"bash", "-c", command}
		}

		ccount := k.counter.get()
		k.logger.Debug("Current kube connection count: %v", ccount)
		if k.MaxConnThreshold > 0 && ccount >= k.MaxConnThreshold {
			k.logger.LogError("Too many existing kube connections (%v)", ccount)
			return results, NewMaxConnectionsErr(ccount)
		}

		errch := make(chan error)
		go func() {
			errch <- execOnKube(k, t, cmdv, &b, &berr)
		}()
		timeout := time.After(time.Minute * time.Duration(topts.TimeoutMinutes+1))

		select {
		case err := <-errch:
			r := rex.Result{
				Completed: true,
				Output:    b.String(),
				ErrOutput: berr.String(),
				Err:       err,
			}
			if err == nil {
				cmdlog.Success(cmd, t.String(), r.Output, r.ErrOutput)
			} else {
				r.ExitStatus = 1
				if cee, ok := err.(exec.CodeExitError); ok {
					r.ExitStatus = cee.Code
				}
				if topts.TimeoutMinutes > 0 && topts.UseTimeoutPrefix && r.ExitStatus == 124 {
					cmdlog.Timeout(cmd, err, t.String(), b.String(), berr.String())
					return results, fmt.Errorf("timeout")
				}
				cmdlog.Error(cmd, err, t.String(), r.Output, r.ErrOutput)
			}
			results[index] = r
			if r.ExitStatus != 0 {
				// stop running commands on error
				// TODO: make caller configurable?)
				return results, nil
			}

		case <-timeout:
			cmdlog.Timeout(cmd, nil, t.String(), b.String(), berr.String())
			return results, fmt.Errorf("timeout")
		}
	}

	return results, nil
}

func execOnKube(
	k *KubeConn, t TargetContainer, cmdv []string,
	b, berr *bytes.Buffer) error {

	k.counter.increment()
	defer k.counter.decrement()

	req := k.kube.CoreV1().RESTClient().Post().
		Resource(t.resourceName()).
		Name(t.PodName).
		Namespace(t.Namespace).
		SubResource("exec").
		Param("container", t.ContainerName)
	req.VersionedParams(&api.PodExecOptions{
		Container: t.ContainerName,
		Command:   cmdv,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	// Create SPDY connection
	exec, err := remotecommand.NewSPDYExecutor(k.kubeConfig, "POST", req.URL())
	if err != nil {
		k.logger.Err(err)
		return fmt.Errorf("Unable to setup a session with %v", t.PodName)
	}

	// Execute command
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: b,
		Stderr: berr,
	})
	return err
}
