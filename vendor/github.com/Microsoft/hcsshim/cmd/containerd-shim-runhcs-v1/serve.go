package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/windows"
)

var serveCommand = cli.Command{
	Name:           "serve",
	Hidden:         true,
	SkipArgReorder: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "socket",
			Usage: "the socket path to serve",
		},
		cli.BoolFlag{
			Name:  "is-sandbox",
			Usage: "is the task id a Kubernetes sandbox id",
		},
	},
	Action: func(ctx *cli.Context) error {
		// On Windows the serve command is internally used to actually create
		// the process that hosts the containerd/ttrpc entrypoint to the Runtime
		// V2 API's. The model requires this 2nd invocation of the shim process
		// so that the 1st invocation can return the address via stdout on
		// success.
		//
		// The activation model for this shim is as follows:
		//
		// The public invocation of `shim start` is called which internally
		// decides to either return the address of an existing shim or serve a
		// new one. If serve is decided it execs this entry point `shim serve`.
		// The handoff logic is that this shim will serve the ttrpc entrypoint
		// with only stderr set by the caller. Once the shim has successfully
		// served the entrypoint it is required to close stderr to alert the
		// caller it has completed to the point of handoff. If it fails it will
		// write the error to stderr and the caller will forward the error on as
		// part of the `shim start` failure path. Once successfully served the
		// shim `MUST` not use any std handles. The shim can log any errors to
		// the upstream caller by listening for a log connection and streaming
		// the events.

		os.Stdin.Close()

		// Force the cli.ErrWriter to be os.Stdout for this. We use stderr for
		// the panic.log attached via start.
		cli.ErrWriter = os.Stdout

		socket := ctx.String("socket")
		if !strings.HasPrefix(socket, `\\.\pipe`) {
			return errors.New("socket is required to be pipe address")
		}

		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: log.RFC3339NanoFixed,
			FullTimestamp:   true,
		})

		// Setup the log listener
		//
		// TODO: JTERRY75 we need this to be the reconnect log listener or
		// switch to events
		// TODO: JTERRY75 switch containerd to use the protected path.
		//const logAddrFmt = "\\\\.\\pipe\\ProtectedPrefix\\Administrators\\containerd-shim-%s-%s-log"
		const logAddrFmt = "\\\\.\\pipe\\containerd-shim-%s-%s-log"
		logl, err := winio.ListenPipe(fmt.Sprintf(logAddrFmt, namespaceFlag, idFlag), nil)
		if err != nil {
			return err
		}
		defer logl.Close()

		lerrs := make(chan error, 1)
		defer close(lerrs)
		go func() {
			// Listen for log connections in the background
			a, err := logl.Accept()
			if err != nil {
				lerrs <- err
				return
			}
			// Switch the logrus output to here. Note: we wont get this
			// connection until the return from `shim start` so we still
			// havent transitioned the error model yet.
			logrus.SetOutput(a)
		}()

		// Setup the ttrpc server
		svc := &service{
			events:    publishEvent,
			tid:       idFlag,
			isSandbox: ctx.Bool("is-sandbox"),
		}
		s, err := ttrpc.NewServer()
		if err != nil {
			return err
		}
		defer s.Close()
		task.RegisterTaskService(s, svc)

		sl, err := winio.ListenPipe(socket, nil)
		if err != nil {
			return err
		}
		defer sl.Close()

		serrs := make(chan error, 1)
		defer close(serrs)
		go func() {
			// TODO: JTERRY75 We should use a real context with cancellation shared by
			// the service for shim shutdown gracefully.
			ctx := context.Background()
			if err := s.Serve(ctx, sl); err != nil &&
				!strings.Contains(err.Error(), "use of closed network connection") {
				logrus.WithError(err).Fatal("containerd-shim: ttrpc server failure")
				serrs <- err
				return
			}
			serrs <- nil
		}()

		select {
		case err := <-lerrs:
			return err
		case err := <-serrs:
			return err
		case <-time.After(2 * time.Millisecond):
			// TODO: JTERRY75 this is terrible code. Contribue a change to
			// ttrpc that you can:
			//
			// go func () { errs <- s.Serve() }
			// select {
			// case <-errs:
			// case <-s.Ready():
			// }

			// This is our best indication that we have not errored on creation
			// and are successfully serving the API.
			os.Stdout.Close()
		}

		// Wait for the serve API to be shut down.
		<-serrs
		return nil
	},
}

// createEvent creates a Windows event ACL'd to builtin administrator
// and local system. Can use docker-signal to signal the event.
func createEvent(event string) (windows.Handle, error) {
	ev, _ := windows.UTF16PtrFromString(event)
	sd, err := winio.SddlToSecurityDescriptor("D:P(A;;GA;;;BA)(A;;GA;;;SY)")
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get security descriptor for event '%s'", event)
	}
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	sa.SecurityDescriptor = uintptr(unsafe.Pointer(&sd[0]))
	h, err := windows.CreateEvent(&sa, 0, 0, ev)
	if h == 0 || err != nil {
		return 0, errors.Wrapf(err, "failed to create event '%s'", event)
	}
	return h, nil
}

// setupDebuggerEvent listens for an event to allow a debugger such as delve
// to attach for advanced debugging. It's called when handling a ContainerCreate
func setupDebuggerEvent() {
	if os.Getenv("CONTAINERD_SHIM_RUNHCS_V1_WAIT_DEBUGGER") == "" {
		return
	}
	event := "Global\\debugger-" + fmt.Sprint(os.Getpid())
	handle, err := createEvent(event)
	if err != nil {
		return
	}
	logrus.Infof("Halting until %s is signalled", event)
	windows.WaitForSingleObject(handle, windows.INFINITE)
	return
}

// setupDumpStacks listens for an event which when signalled dumps the
// stacks from this process to the log output.
func setupDumpStacks() {
	event := "Global\\stackdump-" + fmt.Sprint(os.Getpid())
	handle, err := createEvent(event)
	if err != nil {
		return
	}
	go func() {
		for {
			windows.WaitForSingleObject(handle, windows.INFINITE)
			dumpStacks(true)
		}
	}()
	return
}

func dumpStacks(writeToFile bool) {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	logrus.Infof("=== BEGIN goroutine stack dump ===\n%s\n=== END goroutine stack dump ===", buf)

	if writeToFile {
		// Also write to file to aid gathering diagnostics
		name := filepath.Join(os.TempDir(), fmt.Sprintf("containerd-shim-runhcs-v1.%d.stacks.log", os.Getpid()))
		f, err := os.Create(name)
		if err != nil {
			return
		}
		defer f.Close()
		f.WriteString(string(buf))
	}
}
