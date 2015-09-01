package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/openshift/origin/pkg/cmd/util"
)

type Server struct {
	Name     string
	Port     string
	Hostname string
	Data     map[string]string
}

var server Server

func init() {

	// name of the mock server
	name := util.Env("MOCK_NAME", "")

	// port of the mock server
	port := util.Env("MOCK_PORT", "8080")

	hostname, _ := os.Hostname()

	// initialize the new server
	server = newServer(name, hostname, port)
}

func newServer(name, hostname, port string) Server {

	if name == "" || len(strings.TrimSpace(name)) == 0 {
		glog.Fatalf("This server must have a name, found in MOCK_NAME env variable")
	}

	if port == "" || len(strings.TrimSpace(port)) == 0 {
		port = "8080"
	}

	data := make(map[string]string)

	_server := Server{Name: name, Hostname: hostname, Port: port, Data: data}

	return _server

}

func failHandler(w http.ResponseWriter, r *http.Request) {

	p := mux.Vars(r)

	// delay
	exitCodeStr := p["exitcode"]
	exitCode, err := strconv.Atoi(exitCodeStr)
	if err != nil {
		exitCode = -1
	}

	fmt.Fprintln(w, "ok")

	// flush before shutdown
	f, ok := w.(http.Flusher)
	if ok && f != nil {
		f.Flush()
	}

	timer := time.NewTimer(time.Second * time.Duration(0))
	glog.Infof("%s will fail with exit code \"%d\" now\n", server.Name, exitCode)
	go func(code int) {
		<-timer.C
		glog.Infof("%s stopping now (%d).\n", server.Name, code)
		os.Exit(code)
	}(exitCode)
}

func shutdownHandler(w http.ResponseWriter, r *http.Request) {

	p := mux.Vars(r)

	// delay
	delayStr := p["delay"]
	delay, err := strconv.Atoi(delayStr)
	if err != nil {
		delay = 0
	}

	fmt.Fprintln(w, "ok")

	// flush before shutdown
	f, ok := w.(http.Flusher)
	if ok && f != nil {
		f.Flush()
	}

	timer := time.NewTimer(time.Second * time.Duration(delay))
	glog.Infof("%s will shutdown in %d second(s)\n", server.Name, delay)
	go func() {
		<-timer.C
		glog.Infof("%s stopping now.\n", server.Name)
		os.Exit(0)
	}()
}

func nameHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, server.Name)
}

func hostnameHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, server.Hostname)
}

func envHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)

	key := p["var"]
	v := util.Env(key, "")

	fmt.Fprintln(w, v)
}

func putHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)

	key := p["key"]
	value := p["value"]

	server.Data[key] = value

	fmt.Fprintln(w, "ok")
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	key := p["key"]

	value, _ := server.Data[key]

	fmt.Fprintln(w, value)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	key := p["key"]

	_, z := server.Data[key]
	if z {
		delete(server.Data, key)
		fmt.Fprintln(w, "ok")
	} else {
		fmt.Fprintln(w, "ko")
	}
}

func containsHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)
	key := p["key"]

	_, z := server.Data[key]

	if z {
		fmt.Fprintln(w, "ok")
	} else {
		fmt.Fprintln(w, "ko")
	}
}

func keysHandler(w http.ResponseWriter, r *http.Request) {

	for k := range server.Data {
		fmt.Fprintln(w, k)
	}
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {

	p := mux.Vars(r)

	scheme := p["scheme"]
	host := p["host"]
	port := p["port"]
	path := p["path"]

	if scheme == "" {
		scheme = "http"
	}

	if port == "" {
		port = "8080"
	}

	if path == "" {
		path = ""
	} else {
		path = fmt.Sprintf("/%s", path)
	}

	doRedirect(scheme, host, port, path, w)
}

// http://localhost:8080/chainredirect/http/localhost_8080_name-localhost_8080_contains_k1
//                                     ^
//                                     |scheme
//
func chainRedirectHandler(w http.ResponseWriter, r *http.Request) {
	p := mux.Vars(r)

	chainPath := p["chainPath"]
	scheme := p["scheme"]

	chainPathArr := strings.Split(chainPath, "-")

	if len(chainPathArr) > 0 {

		currentLinkArr := strings.Split(chainPathArr[0], "_")

		if len(currentLinkArr) >= 3 {
			host := currentLinkArr[0]
			port := currentLinkArr[1]

			var pathArr = make([]string, len(currentLinkArr)-2)

			for i := 2; i < len(currentLinkArr); i++ {

				pathArr[i-2] = currentLinkArr[i]

			}

			path := strings.Join(pathArr, "/")

			if scheme == "" {
				scheme = "http"
			}
			if port == "" {
				port = "8080"
			}

			if path == "" {
				path = ""
			} else {
				path = fmt.Sprintf("/%s", path)
			}

			// call the action
			back := doCall(scheme, host, port, path)

			// then continue the chain
			if len(chainPathArr) > 1 {
				idx := strings.Index(chainPath, "-")
				nextLink := chainPath[idx+1:]
				chainRedirect := fmt.Sprintf("/chainredirect/%s/%s", scheme, nextLink)
				recursiveBack := doCall(scheme, host, port, chainRedirect)
				back = fmt.Sprintf("%s%s", back, recursiveBack)
			}

			fmt.Fprintf(w, back)
		}
	}
}

func doRedirect(scheme, host, port, path string, w http.ResponseWriter) {
	url := fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)

	response, err := http.Get(url)

	if err == nil {
		defer response.Body.Close()
		contents, _ := ioutil.ReadAll(response.Body)
		fmt.Fprintf(w, "Querying %s got response %s", url, string(contents))
	}
}

func doCall(scheme, host, port, path string) string {
	url := fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)
	response, err := http.Get(url)

	if err == nil {
		defer response.Body.Close()
		contents, _ := ioutil.ReadAll(response.Body)
		//fmt.Fprintf(w, "Querying %s got response %s", url, string(contents))
		return string(contents)
	} else {
		return "{error}"
	}
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/name", nameHandler)
	r.HandleFunc("/hostname", hostnameHandler)
	r.HandleFunc("/env/{var}", envHandler)
	r.HandleFunc("/shutdown", shutdownHandler)
	r.HandleFunc("/fail", failHandler)
	r.HandleFunc("/fail/{exitcode}", failHandler)
	r.HandleFunc("/shutdown/{delay}", shutdownHandler)
	r.HandleFunc("/redirect/{scheme}/{host}/{port}/{path}", redirectHandler)
	r.HandleFunc("/chainredirect/{scheme}/{chainPath}", chainRedirectHandler)
	r.HandleFunc("/get/{key}", getHandler)
	r.HandleFunc("/delete/{key}", deleteHandler)
	r.HandleFunc("/put/{key}/{value}", putHandler)
	r.HandleFunc("/contains/{key}", containsHandler)
	r.HandleFunc("/keys", keysHandler)

	// glog package need flag parsing.
	flag.Parse()

	glog.Infof("Started %s, serving at %s\n", server.Name, server.Port)

	http.Handle("/", r)
	err := http.ListenAndServe(fmt.Sprintf(":%s", server.Port), r)
	if err != nil {
		glog.Fatalf("ListenAndServe: " + err.Error())
	}
}
