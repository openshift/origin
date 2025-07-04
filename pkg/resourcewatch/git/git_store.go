package git

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/kube-openapi/pkg/util/sets"

	"gopkg.in/src-d/go-git.v4"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type GitStorage struct {
	repo *git.Repository
	path string
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
func NewGitStorage(path string) (*GitStorage, error) {
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

func (s *GitStorage) GC() error {
	return s.execGit("gc")
}

// handle handles different operations on git
func (s *GitStorage) handle(timestamp time.Time, gvr schema.GroupVersionResource, oldObj, obj *unstructured.Unstructured, delete bool) {
	filePath, content, err := decodeUnstructuredObject(gvr, obj)
	if err != nil {
		klog.Warningf("Decoding %q failed: %v", filePath, err)
		return
	}
	resourceName := ""
	if len(gvr.Group) == 0 {
		resourceName = gvr.Resource
	} else {
		resourceName = gvr.Resource + "." + gvr.Group
	}
	ocCommand := ""
	if len(obj.GetNamespace()) == 0 {
		ocCommand = fmt.Sprintf("%s/%s", resourceName, obj.GetName())
	} else {
		ocCommand = fmt.Sprintf("%s/%s -n %s", resourceName, obj.GetName(), obj.GetNamespace())
	}

	if delete {
		klog.Infof("Calling commitRemove for %s", filePath)
		err := os.Remove(path.Join(s.path, filePath))
		if err != nil {
			// If the file doesn't exist it means we're deleting a file we haven't previously observed.
			// That's probably a collection bug.
			// Add it first before removing it.
			if os.IsNotExist(err) {
				klog.Info("Observed delete of file we haven't previously observed. Adding it first.")
				s.handle(timestamp, gvr, nil, obj, false)
				s.handle(timestamp, gvr, nil, obj, true)
				return
			} else {
				klog.Errorf("Error removing %q: %v", filePath, err)
			}
		}
		if err := s.commitRemove(timestamp, filePath, "unknown", ocCommand); err != nil {
			klog.Error(err)
		}

		return
	}

	klog.Infof("Calling write for %s", filePath)
	operation, err := s.write(filePath, content)
	if err != nil {
		klog.Errorf("Writing file content failed %q: %v", filePath, err)
		return
	}

	modifyingUser, err := guessAtModifyingUsers(oldObj, obj)
	if err != nil {
		klog.Warningf("Guessing users failed %q: %v", filePath, err)
		modifyingUser = err.Error()
	}

	switch operation {
	case gitOpAdded:
		klog.Infof("Calling commitAdd for %s", filePath)
		if err := s.commitAdd(timestamp, filePath, modifyingUser, ocCommand); err != nil {
			klog.Error(err)
		}
	case gitOpModified:
		klog.Infof("Calling commitModify for %s", filePath)
		if err := s.commitModify(timestamp, filePath, modifyingUser, ocCommand); err != nil {
			klog.Error(err)
		}
	default:
		klog.Errorf("unhandled case for %s: %d", filePath, operation)
	}
}

func (s *GitStorage) OnAdd(timestamp time.Time, gvr schema.GroupVersionResource, obj interface{}) {
	objUnstructured := obj.(*unstructured.Unstructured)

	s.handle(timestamp, gvr, nil, objUnstructured, false)
}

func (s *GitStorage) OnUpdate(timestamp time.Time, gvr schema.GroupVersionResource, oldObj, obj interface{}) {
	objUnstructured := obj.(*unstructured.Unstructured)
	oldObjUnstructured := oldObj.(*unstructured.Unstructured)

	s.handle(timestamp, gvr, oldObjUnstructured, objUnstructured, false)
}

func (s *GitStorage) OnDelete(timestamp time.Time, gvr schema.GroupVersionResource, obj interface{}) {
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

	s.handle(timestamp, gvr, nil, objUnstructured, true)
}

// guessAtModifyingUsers tries to figure out who modified the resource
func guessAtModifyingUsers(oldObj, obj *unstructured.Unstructured) (string, error) {
	if oldObj == nil {
		allOwners := []string{}
		for _, managedField := range obj.GetManagedFields() {
			allOwners = append(allOwners, managedField.Manager)
		}
		if len(allOwners) == 0 {
			return "added-unknown", nil
		}
		return strings.Join(allOwners, " AND "), nil
	}

	allOwners := sets.NewString()
	modifiedFieldList, err := modifiedFields(oldObj, obj)
	if err != nil {
		return "unknown", err
	}
	modifiers, err := whichUsersOwnModifiedFields(obj, *modifiedFieldList)
	if err != nil {
		return "unknown", err
	}
	allOwners.Insert(modifiers...)

	if len(allOwners) == 0 {
		return "modified-unknown", nil
	}
	return strings.Join(allOwners.List(), " AND "), nil
}

// decodeUnstructuredObject decodes the unstructured object we get from informer into a YAML bytes
func decodeUnstructuredObject(gvr schema.GroupVersionResource, objUnstructured *unstructured.Unstructured) (string, []byte, error) {
	filename := resourceFilename(gvr, objUnstructured.GetNamespace(), objUnstructured.GetName())
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
func resourceFilename(gvr schema.GroupVersionResource, namespace, name string) string {
	groupStr := ""
	if len(gvr.Group) != 0 {
		groupStr = gvr.Group
	} else {
		groupStr = "core"
	}
	// do not toLower because these are case-sensitive fields.
	// these path prefixes match the structure of must-gather and oc adm inspect, so we can theoretically re-use tooling.
	if len(namespace) == 0 {
		return filepath.Join("cluster-scoped-resources", groupStr, gvr.Resource, name+".yaml")
	}
	return filepath.Join("namespaces", namespace, groupStr, gvr.Resource, name+".yaml")
}

func (s *GitStorage) execGit(args ...string) error {
	// Disable automatic garbage collection to avoid racing with other processes.
	args = append([]string{"-c", "gc.auto=0"}, args...)

	osCommand := exec.Command("git", args...)
	osCommand.Dir = s.path
	output, err := osCommand.CombinedOutput()
	if err != nil {
		klog.Errorf("Ran git %v\n%v\n\n", args, string(output))
		return err
	}
	return nil
}

func (s *GitStorage) commit(timestamp time.Time, path, author, commitMessage string) error {
	authorString := fmt.Sprintf("%s <ci-monitor@openshift.io>", author)
	dateString := timestamp.Format(time.RFC3339)

	return s.execGit("commit", "--author", authorString, "--date", dateString, "-m", commitMessage)
}

func (s *GitStorage) commitAdd(timestamp time.Time, path, author, ocCommand string) error {
	if err := s.execGit("add", path); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("added %s", ocCommand)
	if err := s.commit(timestamp, path, author, commitMessage); err != nil {
		return err
	}

	klog.Infof("Add: %v -- %v added %v", path, author, ocCommand)
	return nil
}

func (s *GitStorage) commitModify(timestamp time.Time, path, author, ocCommand string) error {
	if err := s.execGit("add", path); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("modifed %s", ocCommand)
	if err := s.commit(timestamp, path, author, commitMessage); err != nil {
		return err
	}

	klog.Infof("Modified: %v -- %v updated %v", path, author, ocCommand)
	return nil
}

func (s *GitStorage) commitRemove(timestamp time.Time, path, author, ocCommand string) error {
	if err := s.execGit("rm", path); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("removed %s", ocCommand)
	if err := s.commit(timestamp, path, author, commitMessage); err != nil {
		return err
	}

	klog.Infof("Removed: %v -- %v deleted %v", path, author, ocCommand)
	return nil
}

// write handle writing the content into git repository
func (s *GitStorage) write(name string, content []byte) (gitOperation, error) {
	fullPath := filepath.Join(s.path, name)

	fileMode := os.FileMode(0644)

	// If the file does not exist, create it and report it as new file
	// This will get reflected in the commit message
	if _, err := os.Lstat(fullPath); err != nil {
		if !os.IsNotExist(err) {
			return gitOpError, err
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), os.ModePerm); err != nil {
			return gitOpError, err
		}
		if err := os.WriteFile(fullPath, content, fileMode); err != nil {
			return gitOpError, err
		}
		return gitOpAdded, nil
	}

	// If the file exists, updated its content and report modified
	if err := os.WriteFile(fullPath, content, fileMode); err != nil {
		return gitOpError, err
	}
	return gitOpModified, nil
}
