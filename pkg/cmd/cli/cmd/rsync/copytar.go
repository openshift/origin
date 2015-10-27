package rsync

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/spf13/cobra"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type tarStrategy struct {
	Quiet          bool
	Delete         bool
	Tar            tar.Tar
	RemoteExecutor executor
}

func newTarStrategy(f *clientcmd.Factory, c *cobra.Command, o *RsyncOptions) (copyStrategy, error) {

	tarHelper := tar.New()
	tarHelper.SetExclusionPattern(nil)

	remoteExec, err := newRemoteExecutor(f, o)
	if err != nil {
		return nil, err
	}

	return &tarStrategy{
		Quiet:          o.Quiet,
		Delete:         o.Delete,
		Tar:            tarHelper,
		RemoteExecutor: remoteExec,
	}, nil
}

func deleteContents(dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			err = os.RemoveAll(f.Name())
		} else {
			err = os.Remove(f.Name())
		}
		if err != nil {
			return err
		}
	}
	return nil

}

func deleteFiles(spec *pathSpec, remoteExecutor executor) error {
	if spec.Local() {
		return deleteContents(spec.Path)
	}
	deleteCmd := []string{"sh", "-c", fmt.Sprintf("rm -rf %s", filepath.Join(spec.Path, "*"))}
	return executeWithLogging(remoteExecutor, deleteCmd)
}

func (r *tarStrategy) Copy(source, destination *pathSpec, out, errOut io.Writer) error {

	glog.V(3).Infof("Copying files with tar")
	if r.Delete {
		// Implement the rsync --delete flag as a separate call to first delete directory contents
		err := deleteFiles(destination, r.RemoteExecutor)
		if err != nil {
			return fmt.Errorf("unable to delete files in destination: %v", err)
		}
	}
	tmp, err := ioutil.TempFile("", "rsync")
	if err != nil {
		return fmt.Errorf("cannot create local temporary file for tar: %v", err)
	}
	defer os.Remove(tmp.Name())

	// Create tar
	if source.Local() {
		glog.V(4).Infof("Creating local tar file %s from local path %s", tmp.Name(), source.Path)
		err = tarLocal(r.Tar, source.Path, tmp)
		if err != nil {
			return fmt.Errorf("error creating local tar of source directory: %v", err)
		}
	} else {
		glog.V(4).Infof("Creating local tar file %s from remote path %s", tmp.Name(), source.Path)
		err = tarRemote(r.RemoteExecutor, source.Path, tmp, errOut)
		if err != nil {
			return fmt.Errorf("error creating remote tar of source directory: %v", err)
		}
	}

	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("error closing temporary tar file %s: %v", tmp.Name(), err)
	}
	tmp, err = os.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("cannot open temporary tar file %s: %v", tmp.Name(), err)
	}
	defer tmp.Close()

	// Extract tar
	if destination.Local() {
		glog.V(4).Infof("Untarring temp file %s to local directory %s", tmp.Name(), destination.Path)
		err = untarLocal(r.Tar, destination.Path, tmp)
	} else {
		glog.V(4).Infof("Untarring temp file %s to remote directory %s", tmp.Name(), destination.Path)
		err = untarRemote(r.RemoteExecutor, destination.Path, r.Quiet, tmp, out, errOut)
	}
	if err != nil {
		return fmt.Errorf("error extracting tar at destination directory: %v", err)
	}
	return nil
}

func (r *tarStrategy) Validate() error {
	errs := []error{}
	if r.Tar == nil {
		errs = append(errs, errors.New("tar helper must be provided"))
	}
	if r.RemoteExecutor == nil {
		errs = append(errs, errors.New("remote executor must be provided"))
	}
	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}
	return nil
}

func (r *tarStrategy) String() string {
	return "tar"
}

func tarRemote(exec executor, sourceDir string, out, errOut io.Writer) error {
	glog.V(4).Infof("Tarring %s remotely", sourceDir)
	cmd := []string{"tar", "-C", sourceDir, "-c", "."}
	glog.V(4).Infof("Remote tar command: %s", strings.Join(cmd, " "))
	return exec.Execute(cmd, nil, out, errOut)
}

func tarLocal(tar tar.Tar, sourceDir string, w io.Writer) error {
	glog.V(4).Infof("Tarring %s locally", sourceDir)
	// includeParent mimics rsync's behavior. When the source path ends in a path
	// separator, then only the contents of the directory are copied. Otherwise,
	// the directory itself is copied.
	includeParent := true
	if strings.HasSuffix(sourceDir, string(filepath.Separator)) {
		includeParent = false
		sourceDir = sourceDir[:len(sourceDir)-1]
	}
	return tar.CreateTarStream(sourceDir, includeParent, w)
}

func untarLocal(tar tar.Tar, destinationDir string, r io.Reader) error {
	glog.V(4).Infof("Extracting tar locally to %s", destinationDir)
	return tar.ExtractTarStream(destinationDir, r)
}

func untarRemote(exec executor, destinationDir string, quiet bool, in io.Reader, out, errOut io.Writer) error {
	cmd := []string{"tar", "-C", destinationDir, "-x"}
	if !quiet {
		cmd = append(cmd, "-v")
	}
	glog.V(4).Infof("Extracting tar remotely with command: %s", strings.Join(cmd, " "))
	return exec.Execute(cmd, in, out, errOut)
}
