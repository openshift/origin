package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	terminationLog := flag.String("termination-log-file", "", "Write logs after SIGTERM to this file (in addition to stderr)")
	terminationLock := flag.String("termination-touch-file", "", "Touch this file on SIGTERM and delete on termination")

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
			fmt.Fprintf(stderr, "Received signal %s\n", s)

			if len(*terminationLock) > 0 {
				fmt.Fprintln(stderr, "Touching", *terminationLock)
				if err := touch(*terminationLock); err != nil {
					fmt.Fprintln(stderr, fmt.Errorf("error touching %s: %v", *terminationLock, err))
					// keep going
				}
			}

			select {
			case <-termCh:
			default:
				close(termCh)
			}
			cmd.Process.Signal(s)
		}
	}()

	fmt.Printf("Launching %v\n", cmd)
	rc := 0
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); !ok {
			rc = exitError.ExitCode()
		} else {
			fmt.Fprintf(stderr, "Failed to launch %s: %v\n", args[0], err)
			os.Exit(255)
		}
	}

	// remove signal handling
	signal.Stop(sigCh)
	close(sigCh)
	wg.Wait()

	if len(*terminationLock) > 0 {
		os.Remove(*terminationLock)
	}

	fmt.Fprintf(stderr, "Exit code %d\n", rc)
	os.Exit(rc)
}

// terminationFileWriter forwards everything to the embedded writer. When
// startFileLoggingCh is closed, everything is appended to the given file name
// in addition.
type terminationFileWriter struct {
	io.Writer
	fn                 string
	startFileLoggingCh <-chan struct{}

	f *os.File
}

func (w *terminationFileWriter) Write(bs []byte) (int, error) {
	select {
	case <-w.startFileLoggingCh:
		if w.f == nil {
			f, err := os.OpenFile(w.fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatal(err)
			}
			w.f = f
			fmt.Println("Starting logging to", w.fn)
		}
		if n, err := w.f.Write(bs); err != nil {
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
