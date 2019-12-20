package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/gogo/protobuf/proto"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name: "delete",
	Usage: `
This command allows containerd to delete any container resources created, mounted, and/or run by a shim when containerd can no longer communicate over rpc. This happens if a shim is SIGKILL'd with a running container. These resources will need to be cleaned up when containerd loses the connection to a shim. This is also used when containerd boots and reconnects to shims. If a bundle is still on disk but containerd cannot connect to a shim, the delete command is invoked.
	
The delete command will be executed in the container's bundle as its cwd.
`,
	SkipArgReorder: true,
	Action: func(context *cli.Context) error {
		// We cant write anything to stdout for this cmd other than the
		// task.DeleteResponse by protcol. We can write to stderr which will be
		// warning logged in containerd.
		logrus.SetOutput(ioutil.Discard)

		bundleFlag := context.GlobalString("bundle")
		if bundleFlag == "" {
			return errors.New("bundle is required")
		}

		// Attempt to find the hcssystem for this bundle and terminate it.
		if sys, _ := hcs.OpenComputeSystem(idFlag); sys != nil {
			if err := sys.Terminate(); err != nil {
				if hcs.IsPending(err) {
					const terminateTimeout = time.Second * 30
					done := make(chan bool)
					go func() {
						if werr := sys.Wait(); err != nil {
							fmt.Fprintf(os.Stderr, "failed to wait for '%s' to terminate: %v", idFlag, werr)
						}
						done <- true
					}()
					select {
					case <-done:
					case <-time.After(terminateTimeout):
						sys.Close()
						return fmt.Errorf("timed out waiting for '%s' to terminate", idFlag)
					}
				} else {
					fmt.Fprintf(os.Stderr, "failed to terminate '%s': %v", idFlag, err)
				}
			}
			sys.Close()
		}

		// Determine if the config file was a POD and if so kill the whole POD.
		if s, err := getSpecAnnotations(bundleFlag); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			if containerType := s["io.kubernetes.cri.container-type"]; containerType == "container" {
				if sandboxID := s["io.kubernetes.cri.sandbox-id"]; sandboxID != "" {
					if sys, _ := hcs.OpenComputeSystem(sandboxID); sys != nil {
						if err := sys.Terminate(); err != nil {
							if hcs.IsPending(err) {
								if werr := sys.Wait(); err != nil {
									fmt.Fprintf(os.Stderr, "failed to wait for '%s' to terminate: %v", idFlag, werr)
								}
							} else {
								fmt.Fprintf(os.Stderr, "failed to terminate '%s': %v", idFlag, err)
							}
						}
						sys.Close()
					}
				}
			}
		}

		// Remove the bundle on disk
		if err := os.RemoveAll(bundleFlag); err != nil && !os.IsNotExist(err) {
			return err
		}

		if data, err := proto.Marshal(&task.DeleteResponse{
			ExitedAt:   time.Now(),
			ExitStatus: 255,
		}); err != nil {
			return err
		} else {
			if _, err := os.Stdout.Write(data); err != nil {
				return err
			}
		}
		return nil
	},
}
