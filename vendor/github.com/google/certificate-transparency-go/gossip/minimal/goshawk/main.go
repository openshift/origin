// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// The goshawk binary scans a destination log for gossiped STH values
// and checks consistency against the source logs.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go/gossip/minimal"
	"github.com/google/certificate-transparency-go/scanner"
)

var (
	config        = flag.String("config", "", "File holding log configuration in text proto format")
	batchSize     = flag.Int("batch_size", 1000, "Max number of entries to request per call to get-entries")
	numWorkers    = flag.Int("num_workers", 2, "Number of concurrent matchers")
	parallelFetch = flag.Int("parallel_fetch", 2, "Number of concurrent GetEntries fetches")
	startIndex    = flag.Int64("start_index", 0, "Log index to start scanning at")
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scanOpts := scanner.ScannerOptions{
		FetcherOptions: scanner.FetcherOptions{
			BatchSize:     *batchSize,
			ParallelFetch: *parallelFetch,
			StartIndex:    *startIndex,
		},
		NumWorkers: *numWorkers,
	}

	hawk, err := minimal.NewGoshawkFromFile(ctx, *config, nil, scanOpts)
	if err != nil {
		glog.Exitf("failed to load --config: %v", err)
	}

	glog.CopyStandardLogTo("WARNING")
	glog.Info("**** Goshawk Starting ****")

	go awaitSignal(func() {
		cancel()
	})

	hawk.Fly(ctx)

	glog.Infof("Stopping server, about to exit")
	glog.Flush()

	// Give things a few seconds to tidy up
	time.Sleep(time.Second * 3)
}

// awaitSignal waits for standard termination signals, then runs the given
// function; it should be run as a separate goroutine.
func awaitSignal(doneFn func()) {
	// Arrange notification for the standard set of signals used to terminate a server
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Now block and wait for a signal
	sig := <-sigs
	glog.Warningf("Signal received: %v", sig)
	glog.Flush()

	doneFn()
}
