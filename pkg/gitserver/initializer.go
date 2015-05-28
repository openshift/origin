package gitserver

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/generate/git"
)

var lazyInitMatch = regexp.MustCompile("^/([^\\/]+?)/info/refs$")

// lazyInitRepositoryHandler creates a handler that will initialize a Git repository
// if it does not yet exist.
func lazyInitRepositoryHandler(config *Config, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			handler.ServeHTTP(w, r)
			return
		}
		match := lazyInitMatch.FindStringSubmatch(r.URL.Path)
		if match == nil {
			handler.ServeHTTP(w, r)
			return
		}
		name := match[1]
		if name == "." || name == ".." {
			handler.ServeHTTP(w, r)
			return
		}
		path := filepath.Join(config.Home, name)
		_, err := os.Stat(path)
		if !os.IsNotExist(err) {
			handler.ServeHTTP(w, r)
			return
		}

		self := RepositoryURL(config, name, r)
		log.Printf("Lazily initializing bare repository %s", self.String())

		defaultHooks, err := loadHooks(config.HookDirectory)
		if err != nil {
			log.Printf("error: unable to load default hooks: %v", err)
			http.Error(w, fmt.Sprintf("unable to initialize repository: %v", err), http.StatusInternalServerError)
			return
		}

		// TODO: capture init hook output for Git
		if _, err := newRepository(config, path, defaultHooks, self, nil); err != nil {
			log.Printf("error: unable to initialize repo %s: %v", path, err)
			http.Error(w, fmt.Sprintf("unable to initialize repository: %v", err), http.StatusInternalServerError)
			os.RemoveAll(path)
			return
		}
		eventCounter.WithLabelValues(name, "init").Inc()

		handler.ServeHTTP(w, r)
	})
}

// RepositoryURL creates the public URL for the named git repo. If both config.URL and
// request are nil, the returned URL will be nil.
func RepositoryURL(config *Config, name string, r *http.Request) *url.URL {
	var url url.URL
	switch {
	case config.URL != nil:
		url = *config.URL
	case r != nil:
		url = *r.URL
		url.Host = r.Host
		url.Scheme = "http"
	default:
		return nil
	}
	url.Path = "/" + name
	url.RawQuery = ""
	url.Fragment = ""
	return &url
}

func newRepository(config *Config, path string, hooks map[string]string, self *url.URL, origin *url.URL) ([]byte, error) {
	var out []byte
	repo := git.NewRepositoryForBinary(config.GitBinary)

	if origin != nil {
		if err := repo.CloneMirror(path, origin.String()); err != nil {
			return out, err
		}
	} else {
		if err := repo.Init(path, true); err != nil {
			return out, err
		}
	}

	if self != nil {
		if err := repo.AddLocalConfig(path, "gitserver.self.url", self.String()); err != nil {
			return out, err
		}
	}

	// remove all sample hooks, ignore errors here
	if files, err := ioutil.ReadDir(filepath.Join(path, "hooks")); err == nil {
		for _, file := range files {
			os.Remove(filepath.Join(path, "hooks", file.Name()))
		}
	}

	for name, hook := range hooks {
		dest := filepath.Join(path, "hooks", name)
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			return out, err
		}
		if err := os.Symlink(hook, dest); err != nil {
			return out, err
		}
	}

	if initHook, ok := hooks["init"]; ok {
		cmd := exec.Command(initHook)
		cmd.Dir = path
		result, err := cmd.CombinedOutput()
		if err != nil {
			return out, fmt.Errorf("init hook failed: %v\n%s", err, string(result))
		}
		out = result
	}

	return out, nil
}

// clone clones the provided git repositories
func clone(config *Config) error {
	defaultHooks, err := loadHooks(config.HookDirectory)
	if err != nil {
		return err
	}

	errs := []error{}
	for name, v := range config.InitialClones {
		hooks := mergeHooks(defaultHooks, v.Hooks)
		url := v.URL
		url.Fragment = ""
		path := filepath.Join(config.Home, name)
		ok, err := git.IsBareRoot(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ok {
			if !config.CleanBeforeClone {
				continue
			}
			log.Printf("Removing %s", path)
			if err := os.RemoveAll(path); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		log.Printf("Cloning %s into %s", url.String(), path)

		self := RepositoryURL(config, name, nil)
		if _, err := newRepository(config, path, hooks, self, &url); err != nil {
			// TODO: tear this directory down
			errs = append(errs, err)
			continue
		}
	}
	if len(errs) > 0 {
		s := []string{}
		for _, err := range errs {
			s = append(s, err.Error())
		}
		return fmt.Errorf("initial clone failed:\n* %s", strings.Join(s, "\n* "))
	}
	return nil
}

func loadHooks(path string) (map[string]string, error) {
	hooks := make(map[string]string)
	if len(path) == 0 {
		return hooks, nil
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() || (file.Mode().Perm()&0111) == 0 {
			continue
		}
		hook := filepath.Join(path, file.Name())
		name := filepath.Base(hook)
		hooks[name] = hook
	}
	return hooks, nil
}

func mergeHooks(hooks ...map[string]string) map[string]string {
	hook := make(map[string]string)
	for _, m := range hooks {
		for k, v := range m {
			hook[k] = v
		}
	}
	return hook
}
