package gitserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"

	"k8s.io/klog"
)

type key int

const (
	requestIDKey key = 0
)

var (
	listenAddr string
	ready      int32
)

func Run(directory string, addr string) {
	listenAddr = addr

	klog.Infof("Started serving GIT repository on %s/cluster-config ...", listenAddr)

	router := http.NewServeMux()

	fileserver := http.FileServer(http.Dir(filepath.Join(directory, ".git")))

	router.Handle("/", fileserver)
	router.Handle("/readyz", readyz())

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      tracing(nextRequestID)(logging()(router)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		klog.Infof("Git HTTP Server is shutting down...")
		atomic.StoreInt32(&ready, 0)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			klog.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		}
		close(done)
	}()

	klog.Infof("Server is ready to handle requests at %q", listenAddr)
	go func() {
		for {
			_, err := os.Stat(filepath.Join(directory, ".git"))
			if err == nil {
				atomic.StoreInt32(&ready, 1)
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	<-done
}

func readyz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&ready) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

func logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				klog.Infof("[%s][%s] %q [%s](%s)", requestID, r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
