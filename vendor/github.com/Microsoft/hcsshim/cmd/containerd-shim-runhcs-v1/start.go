package main

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/oci"
	"github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name: "start",
	Usage: `
This command will launch new shims.

The start command, as well as all binary calls to the shim, has the bundle for the container set as the cwd.

The start command MUST return an address to a shim for containerd to issue API requests for container operations.

The start command can either start a new shim or return an address to an existing shim based on the shim's logic.
`,
	SkipArgReorder: true,
	Action: func(context *cli.Context) (err error) {
		// We cant write anything to stdout/stderr for this cmd.
		logrus.SetOutput(ioutil.Discard)

		// On Windows there are two scenarios that will launch a shim.
		//
		// 1. The config.json in the bundle path contains the kubernetes
		// annotation `io.kubernetes.cri.container-type = sandbox`. This shim
		// will be served for the POD itself and all
		// `io.kubernetes.cri.container-type = container` with a matching
		// `io.kubernetes.cri.sandbox-id`. For any calls to start where the
		// config.json contains the `io.kubernetes.cri.container-type =
		// container` annotation a shim path to the
		// `io.kubernetes.cri.sandbox-id` will be returned.
		//
		// 2. The container does not have any kubernetes annotations and
		// therefore is a process isolated Windows Container, a hypervisor
		// isolated Windows Container, or a hypervisor isolated Linux Container
		// on Windows.

		const addrFmt = "\\\\.\\pipe\\ProtectedPrefix\\Administrators\\containerd-shim-%s-%s-pipe"

		var (
			address string
			pid     int
		)

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		a, err := getSpecAnnotations(cwd)
		if err != nil {
			return err
		}

		ct, sbid, err := oci.GetSandboxTypeAndID(a)
		if err != nil {
			return err
		}

		if ct == oci.KubernetesContainerTypeContainer {
			address = fmt.Sprintf(addrFmt, namespaceFlag, sbid)

			// Connect to the hosting shim and get the pid
			c, err := winio.DialPipe(address, nil)
			if err != nil {
				return errors.Wrap(err, "failed to connect to hosting shim")
			}
			cl := ttrpc.NewClient(c)
			cl.OnClose(func() { c.Close() })
			t := task.NewTaskClient(cl)
			ctx := gocontext.Background()
			req := &task.ConnectRequest{ID: sbid}
			cr, err := t.Connect(ctx, req)

			cl.Close()
			c.Close()
			if err != nil {
				return errors.Wrap(err, "failed to get shim pid from hosting shim")
			}
			pid = int(cr.ShimPid)
		}

		// We need to serve a new one.
		if address == "" {
			isSandbox := ct == oci.KubernetesContainerTypeSandbox
			if isSandbox && idFlag != sbid {
				return errors.Errorf(
					"'id' and '%s' must match for '%s=%s'",
					oci.KubernetesSandboxIDAnnotation,
					oci.KubernetesContainerTypeAnnotation,
					oci.KubernetesContainerTypeSandbox)
			}

			self, err := os.Executable()
			if err != nil {
				return err
			}

			r, w, err := os.Pipe()
			if err != nil {
				return err
			}
			defer r.Close()
			defer w.Close()

			f, err := os.Create(filepath.Join(cwd, "panic.log"))
			if err != nil {
				return err
			}
			defer f.Close()

			address = fmt.Sprintf(addrFmt, namespaceFlag, idFlag)
			args := []string{
				self,
				"--namespace", namespaceFlag,
				"--address", addressFlag,
				"--publish-binary", containerdBinaryFlag,
				"--id", idFlag,
				"serve",
				"--socket", address,
			}
			if isSandbox {
				args = append(args, "--is-sandbox")
			}
			cmd := &exec.Cmd{
				Path:   self,
				Args:   args,
				Env:    os.Environ(),
				Dir:    cwd,
				Stdout: w,
				Stderr: f,
			}

			if err := cmd.Start(); err != nil {
				return err
			}
			w.Close()
			defer func() {
				if err != nil {
					cmd.Process.Kill()
				}
			}()

			// Forward the invocation stderr until the serve command closes it.
			_, err = io.Copy(os.Stderr, r)
			if err != nil {
				return err
			}
			pid = cmd.Process.Pid
		}

		if err := shim.WritePidFile(filepath.Join(cwd, "shim.pid"), pid); err != nil {
			return err
		}
		if err := shim.WriteAddress(filepath.Join(cwd, "address"), address); err != nil {
			return err
		}

		// Write the address new or existing to stdout
		if _, err := fmt.Fprint(os.Stdout, address); err != nil {
			return err
		}
		return nil
	},
}

func getSpecAnnotations(bundlePath string) (map[string]string, error) {
	// specAnnotations is a minimal representation for oci.Spec that we need
	// to serve a shim.
	type specAnnotations struct {
		// Annotations contains arbitrary metadata for the container.
		Annotations map[string]string `json:"annotations,omitempty"`
	}
	f, err := os.OpenFile(filepath.Join(bundlePath, "config.json"), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var spec specAnnotations
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize valid OCI spec")
	}
	return spec.Annotations, nil
}
