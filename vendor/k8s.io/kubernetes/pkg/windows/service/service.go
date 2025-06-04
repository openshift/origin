//go:build windows
// +build windows

/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"os"
	"time"

	"k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

type handler struct {
	tosvc   chan bool
	fromsvc chan error
}

// InitService is the entry point for running the daemon as a Windows
// service. It returns an indication of whether it is running as a service;
// and an error.
func InitService(serviceName string) error {
	h := &handler{
		tosvc:   make(chan bool),
		fromsvc: make(chan error),
	}

	var err error
	go func() {
		err = svc.Run(serviceName, h)
		h.fromsvc <- err
	}()

	// Wait for the first signal from the service handler.
	err = <-h.fromsvc
	if err != nil {
		return err
	}
	klog.Infof("Running %s as a Windows service!", serviceName)
	return nil
}

func (h *handler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending, Accepts: 0}
	// Unblock initService()
	h.fromsvc <- nil

	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.Accepted(windows.SERVICE_ACCEPT_PARAMCHANGE)}
	klog.Infof("Service running")
Loop:
	for {
		select {
		case <-h.tosvc:
			break Loop
		case c := <-r:
			switch c.Cmd {
			case svc.Cmd(windows.SERVICE_CONTROL_PARAMCHANGE):
				s <- c.CurrentStatus
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				klog.Infof("Service stopping")
				// We need to translate this request into a signal that can be handled by the signal handler
				// handling shutdowns normally (currently apiserver/pkg/server/signal.go).
				// If we do not do this, our main threads won't be notified of the upcoming shutdown.
				// Since Windows services do not use any console, we cannot simply generate a CTRL_BREAK_EVENT
				// but need a dedicated notification mechanism.
				graceful := server.RequestShutdown()

				// Free up the control handler and let us terminate as gracefully as possible.
				// If that takes too long, the service controller will kill the remaining threads.
				// As per https://docs.microsoft.com/en-us/windows/desktop/services/service-control-handler-function
				s <- svc.Status{State: svc.StopPending}

				// If we cannot exit gracefully, we really only can exit our process, so at least the
				// service manager will think that we gracefully exited. At the time of writing this comment this is
				// needed for applications that do not use signals (e.g. kube-proxy)
				if !graceful {
					go func() {
						// Ensure the SCM was notified (The operation above (send to s) was received and communicated to the
						// service control manager - so it doesn't look like the service crashes)
						time.Sleep(1 * time.Second)
						os.Exit(0)
					}()
				}
				break Loop
			}
		}
	}

	return false, 0
}
