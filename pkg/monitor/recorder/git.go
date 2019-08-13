package recorder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

type GitStorage struct {
	repo *git.Repository
	path string

	// Writing to Git repository must be synced otherwise Git will freak out
	sync.Mutex
}

type gitOperation int

const (
	gitOpAdded gitOperation = iota
	gitOpModified
	gitOpDeleted
	gitOpError
)

// NewGitStorage returns the resource event handler capable of storing changes observed on resource
// into a Git repository. Each change is stored as separate commit which means a full history of the
// resource lifecycle is preserved.
func NewGitStorage(path string) (cache.ResourceEventHandler, error) {
	// If the repo does not exists, do git init
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		_, err := git.PlainInit(path, false)
		if err != nil {
			return nil, err
		}
	}
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	storage := &GitStorage{path: path, repo: repo}

	return storage, nil
}

// handle handles different operations on git
func (s *GitStorage) handle(obj interface{}, delete bool) {
	s.Lock()
	defer s.Unlock()
	objUnstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Warningf("Object is not unstructured: %v", obj)
	}
	name, content, err := decodeUnstructuredObject(objUnstructured)
	if err != nil {
		klog.Warningf("Decoding %q failed: %v", name, err)
		return
	}
	defer s.updateRefsFile()
	if delete {
		if err := s.delete(name); err != nil {
			klog.Warningf("Unable to delete file %q: %v", name, err)
			return
		}
		if err := s.commit(name, "operator", gitOpDeleted); err != nil {
			klog.Warningf("Committing %q failed: %v", name, err)
		}
		return
	}
	operation, err := s.write(name, content)
	if err != nil {
		klog.Warningf("Writing file content failed %q: %v", name, err)
		return
	}

	if err := s.commit(name, "operator", operation); err != nil {
		klog.Warningf("Committing %q failed: %v", name, err)
	}
}

func (s *GitStorage) OnAdd(obj interface{}) {
	objUnstructured := obj.(*unstructured.Unstructured)
	s.handle(objUnstructured, false)
}

func (s *GitStorage) OnUpdate(_, obj interface{}) {
	objUnstructured := obj.(*unstructured.Unstructured)
	s.handle(objUnstructured, false)
}

func (s *GitStorage) OnDelete(obj interface{}) {
	s.Lock()
	defer s.Unlock()
	objUnstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		objUnstructured, ok = tombstone.Obj.(*unstructured.Unstructured)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Namespace %#v", obj))
			return
		}
	}
	s.handle(objUnstructured, true)
}

// decodeUnstructuredObject decodes the unstructured object we get from informer into a YAML bytes
func decodeUnstructuredObject(objUnstructured *unstructured.Unstructured) (string, []byte, error) {
	filename := resourceFilename(objUnstructured.GetName(), objUnstructured.GroupVersionKind())
	objectBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, objUnstructured)
	if err != nil {
		return filename, nil, err
	}
	objectYAML, err := yaml.JSONToYAML(objectBytes)
	if err != nil {
		return filename, nil, err
	}
	return filename, objectYAML, err
}

// resourceFilename extracts the filename out from the group version kind
func resourceFilename(name string, gvk schema.GroupVersionKind) string {
	return strings.ToLower(fmt.Sprintf("%s.%s.%s-%s.yaml", gvk.Kind, gvk.Version, gvk.Group, name))
}

// commit handle different git operators on repository
func (s *GitStorage) commit(name, component string, operation gitOperation) error {
	t, err := s.repo.Worktree()
	if err != nil {
		return err
	}
	status, err := t.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		return nil
	}
	if _, err := t.Add(name); err != nil {
		return err
	}
	message := ""
	switch operation {
	case gitOpAdded:
		message = fmt.Sprintf("added %s", name)
	case gitOpModified:
		message = fmt.Sprintf("modified %s", name)
	case gitOpDeleted:
		message = fmt.Sprintf("deleted %s", name)
	}
	hash, err := t.Commit(message, &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  "monitor",
			Email: "monitor@openshift.io",
			When:  time.Now(),
		},
		Committer: &object.Signature{
			Name:  component,
			Email: component + "@openshift.io",
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}
	klog.Infof("Committed %q tracking %s", hash.String(), message)
	return err
}

// delete handle removing the file in git repository
func (s *GitStorage) delete(name string) error {
	t, err := s.repo.Worktree()
	if err != nil {
		return err
	}
	return t.Filesystem.Remove(name)
}

// write handle writing the content into git repository
func (s *GitStorage) write(name string, content []byte) (gitOperation, error) {
	t, err := s.repo.Worktree()
	if err != nil {
		return 0, err
	}

	// If the file does not exists, create it and report it as new file
	// This will get reflected in the commit message
	if _, err := t.Filesystem.Lstat(name); err != nil {
		if !os.IsNotExist(err) {
			return gitOpError, err
		}
		f, err := t.Filesystem.Create(name)
		if err != nil {
			return gitOpError, err
		}
		defer f.Close()
		_, err = f.Write(content)
		if err != nil {
			return gitOpError, err
		}
		return gitOpAdded, nil
	}

	// If the file exists, updated its content and report modified
	f, err := t.Filesystem.OpenFile(name, os.O_RDWR, os.ModePerm)
	if err != nil {
		return gitOpError, err
	}
	defer f.Close()
	if _, err := f.Write(content); err != nil {
		return gitOpError, err
	}
	return gitOpModified, nil
}

// updateRefsFile populate .git/info/refs which is needed for git clone via HTTP server
func (s *GitStorage) updateRefsFile() {
	refs, _ := s.repo.References()
	var data []byte
	err := refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() == plumbing.HashReference {
			s := ref.Strings()
			data = append(data, []byte(fmt.Sprintf("%s\t%s\n", s[1], s[0]))...)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(s.path, ".git", "info"), os.ModePerm); err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(filepath.Join(s.path, ".git", "info", "refs"), data, os.ModePerm); err != nil {
		panic(err)
	}
}
