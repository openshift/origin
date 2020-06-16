package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
	"k8s.io/klog"
)

func main() {
	terminationLog := flag.String("termination-log-file", "", "Write logs after SIGTERM to this file (in addition to stderr)")
	terminationLock := flag.String("termination-touch-file", "", "Touch this file on SIGTERM and delete on termination")

	klog.InitFlags(nil)
	flag.Set("v", "9")

	// never log to stderr, only through our termination log writer (which sends it also to stderr)
	flag.Set("logtostderr", "false")
	flag.Set("stderrthreshold", "99")

	flag.Parse()
	args := flag.CommandLine.Args()

	if len(args) == 0 {
		fmt.Println("Missing command line")
		os.Exit(1)
	}

	// use special tee-like writer when termination log is set
	termCh := make(chan struct{})
	var stderr io.Writer = os.Stderr
	if len(*terminationLog) > 0 {
		stderr = &terminationFileWriter{
			Writer:             os.Stderr,
			fn:                 *terminationLog,
			startFileLoggingCh: termCh,
		}

		// do the klog file writer dance: klog writes to all outputs of lower
		// severity. No idea why. So we discard for anything other than info.
		// Otherwise, we would see errors multiple times.
		klog.SetOutput(ioutil.Discard)
		klog.SetOutputBySeverity("INFO", stderr)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = stderr

	// forward SIGTERM and SIGINT to child
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for s := range sigCh {
			select {
			case <-termCh:
			default:
				close(termCh)
			}

			klog.Infof("Received signal %s. Forwarding to sub-process %q.", s, args[0])

			if len(*terminationLock) > 0 {
				klog.Infof("Touching termination lock file %q", *terminationLock)
				if err := touch(*terminationLock); err != nil {
					klog.Infof("error touching %s: %v", *terminationLock, err)
					// keep going
				}
			}

			cmd.Process.Signal(s)
		}
	}()

	klog.Infof("Launching sub-process %q", cmd)
	rc := 0
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			rc = exitError.ExitCode()
		} else {
			klog.Infof("Failed to launch %s: %v", args[0], err)
			os.Exit(255)
		}
	}

	// remove signal handling
	signal.Stop(sigCh)
	close(sigCh)
	wg.Wait()

	if len(*terminationLock) > 0 {
		klog.Infof("Deleting termination lock file %q", *terminationLock)
		os.Remove(*terminationLock)
	}

	klog.Infof("Termination finished with exit code %d", rc)
	os.Exit(rc)
}

// terminationFileWriter forwards everything to the embedded writer. When
// startFileLoggingCh is closed, everything is appended to the given file name
// in addition.
type terminationFileWriter struct {
	io.Writer
	fn                 string
	startFileLoggingCh <-chan struct{}

	logger io.Writer
}

func (w *terminationFileWriter) Write(bs []byte) (int, error) {
	select {
	case <-w.startFileLoggingCh:
		if w.logger == nil {
			l := &lumberjack.Logger{
				Filename:   w.fn,
				MaxSize:    100,
				MaxBackups: 3,
				MaxAge:     28,
				Compress:   false,
			}
			w.logger = l
			fmt.Fprintf(os.Stderr, "Copying termination logs to %q\n", w.fn)
		}
		if n, err := w.logger.Write(bs); err != nil {
			return n, err
		} else if n != len(bs) {
			return n, io.ErrShortWrite
		}
	default:
	}

	return w.Writer.Write(bs)
}

func touch(fn string) error {
	_, err := os.Stat(fn)
	if os.IsNotExist(err) {
		file, err := os.Create(fn)
		if err != nil {
			return err
		}
		defer file.Close()
		return nil
	}

	currentTime := time.Now().Local()
	return os.Chtimes(fn, currentTime, currentTime)
}
