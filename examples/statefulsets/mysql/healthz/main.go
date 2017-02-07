/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

// This is a simple health monitor for mysql. Only needed till #25456 is fixed.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	osExec "os/exec"
	"strings"

	flag "github.com/spf13/pflag"
)

var (
	flags   = flag.NewFlagSet("a http healthz server for mysql.", flag.ExitOnError)
	port    = flags.Int("port", 8080, "port to use for healthz server.")
	verbose = flags.Bool("verbose", false, "log verbose output?")
	pass    = flags.String("password", "", "mysql password.")
	host    = flags.String("host", "", "mysql host.")

	mysqlChecks = map[string]string{
		"/healthz": "show databases;",
	}
)

type mysqlManager struct {
	host, pass string
}

func (m *mysqlManager) exec(cmd string) ([]byte, error) {
	var password string
	if m.pass != "" {
		password = fmt.Sprintf("-p %v", m.pass)
	}
	mysqlCmd := fmt.Sprintf("/usr/bin/mysql -u root %v -h %v -B -e '%v'", password, m.host, cmd)
	return osExec.Command("sh", "-c", mysqlCmd).CombinedOutput()
}

func registerHandlers(verbose bool, m *mysqlManager) {
	var str string
	for endpoint, cmd := range mysqlChecks {
		str += fmt.Sprintf("\t%v: %q\n", endpoint, cmd)
		http.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
			output, err := m.exec(cmd)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			if verbose {
				log.Printf("Output of %v:\n%v\n", cmd, string(output))
			}
			w.Write(output)
		})
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Available handlers:\n%v", str)))
	})
}

func getHostnameOrDie() string {
	output, err := osExec.Command("hostname").CombinedOutput()
	if err != nil {
		log.Fatalf("%v", err)
	}
	return strings.Trim(string(output), "\n")
}

func main() {
	flags.Parse(os.Args)
	hostname := *host
	if hostname == "" {
		hostname = getHostnameOrDie()
	}
	registerHandlers(*verbose, &mysqlManager{pass: *pass, host: hostname})
	log.Printf("Starting mysql healthz server on port %v", *port)
	log.Fatalf(fmt.Sprintf("%v", http.ListenAndServe(fmt.Sprintf(":%v", *port), nil)))
}
