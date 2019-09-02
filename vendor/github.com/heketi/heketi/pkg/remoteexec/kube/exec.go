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

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	kubeletcmd "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	rex "github.com/heketi/heketi/pkg/remoteexec"
)

// ExecCommands executes the given array of commands on the given
// target container using the given connection. The results type
// will contain both the success and failure results of the
// indvidual commands if run. Commands are only run if the previous
// command was successful. Any connection level error conditions
// are returned as the function's error condition.
func ExecCommands(
	k *KubeConn, t TargetContainer,
	commands []string, timeoutMinutes int) (rex.Results, error) {

	results := make(rex.Results, len(commands))

	for index, command := range commands {

		// Remove any whitespace
		command = strings.Trim(command, " ")

		// SUDO is *not* supported

		// Create REST command
		req := k.rest.Post().
			Resource(t.resourceName()).
			Name(t.PodName).
			Namespace(t.Namespace).
			SubResource("exec").
			Param("container", t.ContainerName)
		req.VersionedParams(&api.PodExecOptions{
			Container: t.ContainerName,
			Command:   []string{"/bin/bash", "-c", command},
			Stdout:    true,
			Stderr:    true,
		}, api.ParameterCodec)

		// Create SPDY connection
		exec, err := remotecommand.NewExecutor(k.kubeConfig, "POST", req.URL())
		if err != nil {
			k.logger.Err(err)
			return nil, fmt.Errorf("Unable to setup a session with %v", t.PodName)
		}

		// Create a buffer to trap session output
		var b bytes.Buffer
		var berr bytes.Buffer

		// Excute command
		err = exec.Stream(remotecommand.StreamOptions{
			SupportedProtocols: kubeletcmd.SupportedStreamingProtocols,
			Stdout:             &b,
			Stderr:             &berr,
		})
		r := rex.Result{
			Completed: true,
			Output:    b.String(),
			ErrOutput: berr.String(),
			Err:       err,
		}
		if err == nil {
			k.logger.Debug(
				"Ran command [%v] on [%v]: Stdout [%v]: Stderr [%v]",
				command, t.String(), r.Output, r.ErrOutput)
		} else {
			k.logger.LogError(
				"Failed to run command [%v] on [%v]: Err[%v]: Stdout [%v]: Stderr [%v]",
				command, t.String(), err, r.Output, r.ErrOutput)
			// TODO: extract the real error code if possible
			r.ExitStatus = 1
		}
		results[index] = r
		if r.ExitStatus != 0 {
			// stop running commands on error
			// TODO: make caller configurable?)
			return results, nil
		}
	}

	return results, nil
}
